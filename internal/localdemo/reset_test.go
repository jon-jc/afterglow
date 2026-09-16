package localdemo

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/foottraffic"
)

func populated(t *testing.T) (*core.Store, string) {
	t.Helper()
	ctx := context.Background()
	s, err := core.Open(ctx, filepath.Join(t.TempDir(), "reset.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	for _, install := range []func(context.Context) error{s.Migrate, (&foottraffic.Store{DB: s.DB}).Migrate} {
		if err = install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, tenant := range []string{"demo", "sample-other"} {
		if err = s.Seed(ctx, tenant); err != nil {
			t.Fatal(err)
		}
		r, _, err := s.Reserve(ctx, tenant, "reservation-key", core.ReservationInput{CampaignID: "cmp-cascade", ScreenID: "sea-01", Cost: 10000})
		if err != nil {
			t.Fatal(err)
		}
		id, _, err := s.Accept(ctx, tenant, core.Receipt{Version: 1, EventID: "event", ReservationID: r.ID, ScreenID: r.ScreenID, PlayedAt: r.Created, DurationMS: 10000})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Process(ctx, tenant, id); err != nil {
			t.Fatal(err)
		}
		_, err = (&foottraffic.Store{DB: s.DB}).Ingest(ctx, tenant, foottraffic.Batch{ID: "batch", Source: foottraffic.Source, Windows: []foottraffic.Window{{Zone: "downtown", Start: time.Now().UTC().Truncate(time.Hour).Add(-time.Hour).UnixMilli(), Count: 42}}}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.DB.Exec(`INSERT INTO transport_quarantine VALUES('transport','bad payload','invalid',1)`); err != nil {
		t.Fatal(err)
	}
	generation, err := Initialize(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	return s, generation
}

func TestResetClearsAllActivityAndRetainsReadyInventory(t *testing.T) {
	s, before := populated(t)
	ctx := context.Background()
	after, err := Reset(ctx, s)
	if err != nil || after == before || after == "" {
		t.Fatal(after, err)
	}
	for _, table := range []string{"outbox", "dispatch_context", "deliveries", "reservations", "audit", "transport_quarantine", "traffic_windows_v1", "traffic_batches_v1", "traffic_scopes_v1"} {
		var n int
		if err = s.DB.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d, %v", table, n, err)
		}
	}
	for _, tenant := range []string{"demo", "sample-other"} {
		v, err := s.Snapshot(ctx, tenant)
		if err != nil || len(v.Campaigns) != 3 || len(v.Screens) != 8 {
			t.Fatal(v, err)
		}
		for _, c := range v.Campaigns {
			if c.Reserved != 0 || c.Spent != 0 || c.Budget <= 0 {
				t.Fatal(c)
			}
		}
		if _, _, err = s.Reserve(ctx, tenant, "reservation-key", core.ReservationInput{CampaignID: "cmp-cascade", ScreenID: "sea-01", Cost: 10000}); err != nil {
			t.Fatal("fresh run cannot reuse a cleared key", err)
		}
	}
	reopened, err := Initialize(ctx, s)
	if err != nil || reopened != after {
		t.Fatal("restart changed the generation", err)
	}
}

func TestResetFailureRollsBackDataBudgetsAndGeneration(t *testing.T) {
	s, before := populated(t)
	if _, err := s.DB.Exec(`CREATE TRIGGER block_demo_reset BEFORE DELETE ON traffic_batches_v1 BEGIN SELECT RAISE(ABORT, 'injected reset failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := Reset(context.Background(), s); err == nil {
		t.Fatal("expected injected failure")
	}
	var n int
	if err := s.DB.QueryRow(`SELECT count(*) FROM deliveries`).Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	v, err := s.Snapshot(context.Background(), "demo")
	if err != nil || v.Campaigns[0].Spent != 10000 {
		t.Fatal("budget did not roll back", v, err)
	}
	after, err := Initialize(context.Background(), s)
	if err != nil || after != before {
		t.Fatal("generation changed on rollback", after, err)
	}
}

func TestPostgresResetIsRejectedBeforeDatabaseAccess(t *testing.T) {
	s := &core.Store{Postgres: true}
	if _, err := Initialize(context.Background(), s); err != ErrUnavailable {
		t.Fatal(err)
	}
	if _, err := Reset(context.Background(), s); err != ErrUnavailable {
		t.Fatal(err)
	}
}
