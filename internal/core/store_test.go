package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = filepath.Join(t.TempDir(), "test.db")
	}
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}
func fixture(t *testing.T) (*Store, string) {
	t.Helper()
	s := testStore(t)
	tenant := ID()
	if err := s.Seed(context.Background(), tenant); err != nil {
		t.Fatal(err)
	}
	return s, tenant
}
func hold(t *testing.T, s *Store, tenant string) Reservation {
	t.Helper()
	r, _, err := s.Reserve(context.Background(), tenant, ID(), ReservationInput{"cmp-cascade", "sea-01", 2500000})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func proof(r Reservation) Receipt { return Receipt{1, ID(), r.ID, r.ScreenID, r.Created + 1, 10000} }
func accept(t *testing.T, s *Store, tenant string, p Receipt) string {
	t.Helper()
	id, _, err := s.Accept(context.Background(), tenant, p)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func snap(t *testing.T, s *Store, tenant string) Snapshot {
	t.Helper()
	v, e := s.Snapshot(context.Background(), tenant)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestConcurrentReservationsCannotOverspend(t *testing.T) {
	s, tenant := fixture(t)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.Reserve(context.Background(), tenant, ID(), ReservationInput{"cmp-cascade", "sea-01", 10000000})
			if err == nil {
				ok.Add(1)
			} else if !errors.Is(err, ErrBudget) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 25 {
		t.Fatalf("accepted %d, want 25", ok.Load())
	}
	c := snap(t, s, tenant).Campaigns[0]
	if c.Reserved != c.Budget || c.Spent != 0 {
		t.Fatalf("bad balance %+v", c)
	}
}
func TestIdempotencyAndConflict(t *testing.T) {
	s, tenant := fixture(t)
	in := ReservationInput{"cmp-cascade", "sea-01", 100}
	key := ID()
	r, _, err := s.Reserve(context.Background(), tenant, key, in)
	if err != nil {
		t.Fatal(err)
	}
	rr, replay, err := s.Reserve(context.Background(), tenant, key, in)
	if err != nil || !replay || r.ID != rr.ID {
		t.Fatal("reservation not replayed", err)
	}
	in.Cost++
	if _, _, err = s.Reserve(context.Background(), tenant, key, in); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	p := proof(r)
	id := accept(t, s, tenant, p)
	again, replay, err := s.Accept(context.Background(), tenant, p)
	if err != nil || !replay || id != again {
		t.Fatal("receipt not replayed", err)
	}
	p.DurationMS++
	if _, _, err = s.Accept(context.Background(), tenant, p); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestConcurrentDuplicateReceiptsChargeOnce(t *testing.T) {
	s, tenant := fixture(t)
	r := hold(t, s, tenant)
	var ids []string
	for i := 0; i < 30; i++ {
		ids = append(ids, accept(t, s, tenant, proof(r)))
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := s.Process(context.Background(), tenant, id); err != nil {
				t.Error(err)
			}
		}(id)
	}
	wg.Wait()
	v := snap(t, s, tenant)
	if v.Counts["settled"] != 1 || v.Counts["duplicate"] != 29 || v.Campaigns[0].Spent != r.Cost || v.Campaigns[0].Reserved != 0 {
		t.Fatalf("double charge: %+v", v.Counts)
	}
}
func TestCommitBeforeAckSurvivesRedelivery(t *testing.T) {
	s, tenant := fixture(t)
	r := hold(t, s, tenant)
	id := accept(t, s, tenant, proof(r))
	for i := 0; i < 5; i++ {
		if status, err := s.Process(context.Background(), tenant, id); err != nil || status != "settled" {
			t.Fatal(status, err)
		}
	}
	v := snap(t, s, tenant)
	if v.Campaigns[0].Spent != r.Cost {
		t.Fatal("redelivery changed balance")
	}
	n := 0
	for _, a := range v.Audit {
		if a.Kind == "settled" {
			n++
		}
	}
	if n != 1 {
		t.Fatal("audit duplicated", n)
	}
}
func TestPoisonAndMismatchAreQuarantined(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*Receipt)
		reason string
	}{{"schema", func(p *Receipt) { p.Version = 99 }, "unsupported_schema"}, {"screen", func(p *Receipt) { p.ScreenID = "jfk-01" }, "screen_mismatch"}, {"duration", func(p *Receipt) { p.DurationMS = -1 }, "invalid_play_duration"}, {"old", func(p *Receipt) { p.PlayedAt = 1 }, "outside_reservation_window"}, {"future", func(p *Receipt) { p.PlayedAt = time.Now().Add(time.Hour).UnixMilli() }, "future_play_time"}} {
		t.Run(tc.name, func(t *testing.T) {
			s, tenant := fixture(t)
			r := hold(t, s, tenant)
			p := proof(r)
			tc.edit(&p)
			id := accept(t, s, tenant, p)
			if status, err := s.Process(context.Background(), tenant, id); err != nil || status != "quarantined" {
				t.Fatal(status, err)
			}
			v := snap(t, s, tenant)
			if v.Deliveries[0].Reason != tc.reason || v.Campaigns[0].Spent != 0 || v.Campaigns[0].Reserved != r.Cost {
				t.Fatalf("bad quarantine %+v", v.Deliveries)
			}
		})
	}
}
func TestTenantIsolation(t *testing.T) {
	s, tenant := fixture(t)
	other := ID()
	if err := s.Seed(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	r := hold(t, s, tenant)
	id := accept(t, s, other, proof(r))
	status, err := s.Process(context.Background(), other, id)
	if err != nil || status != "quarantined" {
		t.Fatal(status, err)
	}
	if len(snap(t, s, other).Reservations) != 0 {
		t.Fatal("tenant data exposed")
	}
	if snap(t, s, tenant).Campaigns[0].Reserved != r.Cost {
		t.Fatal("foreign receipt modified budget")
	}
}
func TestExpiredHoldReleasesExactlyOnce(t *testing.T) {
	s, tenant := fixture(t)
	r := hold(t, s, tenant)
	for i := 0; i < 2; i++ {
		if err := s.ReleaseExpired(context.Background(), r.Expires+900001); err != nil {
			t.Fatal(err)
		}
	}
	v := snap(t, s, tenant)
	if v.Campaigns[0].Reserved != 0 || v.Reservations[0].State != "released" {
		t.Fatal("hold not released")
	}
	id := accept(t, s, tenant, proof(r))
	status, err := s.Process(context.Background(), tenant, id)
	if err != nil || status != "quarantined" {
		t.Fatal("late receipt resurrected released funds", err)
	}
}
func TestReceiptAndOutboxAtomic(t *testing.T) {
	s, tenant := fixture(t)
	r := hold(t, s, tenant)
	p := proof(r)
	id := accept(t, s, tenant, p)
	accept(t, s, tenant, p)
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM outbox WHERE tenant=$1 AND id=$2`, tenant, id).Scan(&n); err != nil || n != 1 {
		t.Fatal("outbox missing or duplicated", n, err)
	}
}
func TestDurableRestart(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") != "" {
		t.Skip("file restart test only")
	}
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "restart.db")
	s, e := Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.Seed(ctx, "demo"); e != nil {
		t.Fatal(e)
	}
	r := hold(t, s, "demo")
	id := accept(t, s, "demo", proof(r))
	s.DB.Close()
	s, e = Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	if _, e = s.Process(ctx, "demo", id); e != nil {
		t.Fatal(e)
	}
	if snap(t, s, "demo").Campaigns[0].Spent != r.Cost {
		t.Fatal("lost accepted receipt")
	}
}

func TestAuditWriteFailureRollsBackCharge(t *testing.T) {
	s, tenant := fixture(t)
	if s.Postgres {
		t.Skip("SQLite fault injection; invariants also exercised on Postgres")
	}
	r := hold(t, s, tenant)
	id := accept(t, s, tenant, proof(r))
	if _, e := s.DB.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON audit WHEN NEW.kind='settled' BEGIN SELECT RAISE(ABORT,'injected audit failure'); END`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Process(context.Background(), tenant, id); e == nil {
		t.Fatal("fault did not trigger")
	}
	v := snap(t, s, tenant)
	if v.Campaigns[0].Spent != 0 || v.Reservations[0].State != "held" || v.Deliveries[0].Status != "accepted" {
		t.Fatal("partial commit")
	}
	if _, e := s.DB.Exec(`DROP TRIGGER reject_audit`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Process(context.Background(), tenant, id); e != nil {
		t.Fatal(e)
	}
}
