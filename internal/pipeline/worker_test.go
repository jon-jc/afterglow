package pipeline

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/prometheus/client_golang/prometheus"
)

func setup(t *testing.T) (*Worker, string) {
	t.Helper()
	ctx := context.Background()
	s, e := core.Open(ctx, filepath.Join(t.TempDir(), "worker.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.Seed(ctx, "demo"); e != nil {
		t.Fatal(e)
	}
	r, _, e := s.Reserve(ctx, "demo", core.ID(), core.ReservationInput{CampaignID: "cmp-cascade", ScreenID: "sea-01", Cost: 100})
	if e != nil {
		t.Fatal(e)
	}
	id, _, e := s.Accept(ctx, "demo", core.Receipt{Version: 1, EventID: core.ID(), ReservationID: r.ID, ScreenID: r.ScreenID, PlayedAt: r.Created + 1, DurationMS: 10000})
	if e != nil {
		t.Fatal(e)
	}
	return New(s, prometheus.NewRegistry()), id
}
func TestLostAcknowledgementDoesNotDoubleCharge(t *testing.T) {
	w, id := setup(t)
	ctx := context.Background()
	w.DropAck.Store(true)
	if e := w.Consume(ctx, "demo", id); e == nil {
		t.Fatal("fault did not trigger")
	}
	if e := w.Consume(ctx, "demo", id); e != nil {
		t.Fatal(e)
	}
	v, e := w.Store.Snapshot(ctx, "demo")
	if e != nil || v.Campaigns[0].Spent != 100 {
		t.Fatal("double charge", e)
	}
}
func TestLeaseRecoveryAndFencing(t *testing.T) {
	w, _ := setup(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()
	first, e := w.Store.Claim(ctx, now)
	if e != nil || first == nil {
		t.Fatal(e)
	}
	second, e := w.Store.Claim(ctx, now+1)
	if e != nil || second != nil {
		t.Fatal("claimed live lease", e)
	}
	second, e = w.Store.Claim(ctx, now+30001)
	if e != nil || second == nil || second.Owner == first.Owner {
		t.Fatal("did not recover", e)
	}
	if e = w.Store.Dispatched(ctx, first); e != nil {
		t.Fatal(e)
	}
	var owner string
	if e = w.Store.DB.QueryRow(`SELECT owner FROM outbox WHERE id=$1`, second.ID).Scan(&owner); e != nil || owner != second.Owner {
		t.Fatal("stale dispatcher changed newer claim", e)
	}
}
func TestExhaustedRetryAndReplay(t *testing.T) {
	w, id := setup(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		j, e := w.Store.Claim(ctx, time.Now().UnixMilli()+int64(i)*40000)
		if e != nil || j == nil {
			t.Fatal(e)
		}
		if e = w.Store.FailJob(ctx, j, 0); e != nil {
			t.Fatal(e)
		}
	}
	v, e := w.Store.Snapshot(ctx, "demo")
	if e != nil || v.Deliveries[0].Status != "failed" {
		t.Fatal("not quarantined after retries", e)
	}
	if e = w.Store.Replay(ctx, "demo", id); e != nil {
		t.Fatal(e)
	}
	if e = w.Tick(ctx); e != nil {
		t.Fatal(e)
	}
	v, e = w.Store.Snapshot(ctx, "demo")
	if e != nil || v.Campaigns[0].Spent != 100 {
		t.Fatal("replay failed", e)
	}
}
func TestBackoffBounded(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := Backoff(i)
		if d <= 0 || d > 12*time.Second {
			t.Fatal(d)
		}
	}
}

func TestReplayRacingOldCompletionKeepsDispatchIntent(t *testing.T) {
	w, id := setup(t)
	ctx := context.Background()
	if _, e := w.Store.DB.Exec(`UPDATE deliveries SET status='quarantined' WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	if e := w.Store.Replay(ctx, "demo", id); e != nil {
		t.Fatal(e)
	}
	if e := w.Store.Complete(ctx, "demo", id); e != nil {
		t.Fatal(e)
	}
	var state string
	if e := w.Store.DB.QueryRow(`SELECT state FROM outbox WHERE id=$1`, id).Scan(&state); e != nil || state != "pending" {
		t.Fatal("old completion erased replay", state, e)
	}
}
