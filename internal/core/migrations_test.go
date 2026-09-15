package core

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaReleaseGate(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "migrations.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	if !errors.Is(s.CheckSchema(ctx), ErrSchemaMismatch) {
		t.Fatal("unmigrated database accepted")
	}
	// Adopt the existing demo schema without destroying records.
	if _, err = s.DB.ExecContext(ctx, schema); err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(ctx, "test"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.DB.QueryRow(`SELECT COUNT(*) FROM campaigns`).Scan(&count); err != nil || count != 3 {
		t.Fatal("migration changed business data", err, count)
	}
	if _, err = s.DB.Exec(`UPDATE schema_migrations SET checksum='tampered'`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.CheckSchema(ctx), ErrSchemaMismatch) || !errors.Is(s.Migrate(ctx), ErrSchemaMismatch) {
		t.Fatal("checksum mismatch accepted")
	}
	if _, err = s.DB.Exec(`UPDATE schema_migrations SET version=2`); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(s.CheckSchema(ctx), ErrSchemaMismatch) {
		t.Fatal("newer schema accepted by old binary")
	}
}

func TestSchemaCheckWithReadOnlyPostgresConnection(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PostgreSQL integration target not configured")
	}
	s := testStore(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("default_transaction_read_only", "on")
	u.RawQuery = q.Encode()
	ro, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer ro.DB.Close()
	if err = ro.CheckSchema(ctx); err != nil {
		t.Fatal("runtime schema verification attempted a write:", err)
	}
	if err = ro.Migrate(ctx); err == nil {
		t.Fatal("read-only migration unexpectedly succeeded")
	}
}
