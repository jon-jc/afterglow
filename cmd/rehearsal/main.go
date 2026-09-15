// rehearsal runs a bounded, authenticated HTTP workload against an isolated
// PostgreSQL schema. It refuses remote database targets and never targets a
// running application. The schema it creates is removed when the run ends.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/httpapi"
	"github.com/jon-jc/afterglow/internal/pipeline"
	"github.com/prometheus/client_golang/prometheus"
)

const plays = 120
const clients = 12
const workers = 4
const price = 50000 // $0.05 per synthetic play

type measurements struct {
	sync.Mutex
	Latencies   []float64
	RateLimited int
	Requests    int
}

func (m *measurements) post(ctx context.Context, client *http.Client, base, key, path, identity string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	started := time.Now()
	for attempt := 0; attempt < 60; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", base+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Idempotency-Key", identity)
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		payload, readErr := io.ReadAll(io.LimitReader(res.Body, 65536))
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		m.Lock()
		m.Requests++
		if res.StatusCode == 429 {
			m.RateLimited++
		}
		m.Unlock()
		if res.StatusCode == 429 {
			// Honor the server's one/two-second Retry-After admission guidance.
			delay := time.Second
			if res.Header.Get("Retry-After") == "2" {
				delay = 2 * time.Second
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("%s returned %d: %s", path, res.StatusCode, payload)
		}
		if output != nil {
			if err = json.Unmarshal(payload, output); err != nil {
				return err
			}
		}
		m.Lock()
		m.Latencies = append(m.Latencies, float64(time.Since(started).Microseconds())/1000)
		m.Unlock()
		return nil
	}
	return errors.New("admission retry budget exhausted")
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rehearsal failed:", err)
		os.Exit(1)
	}
}
func run() error {
	u, err := url.Parse(os.Getenv("REHEARSAL_DATABASE_URL"))
	if err != nil {
		return errors.New("invalid rehearsal database URL")
	}
	if (u.Scheme != "postgres" && u.Scheme != "postgresql") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1") {
		return errors.New("REHEARSAL_DATABASE_URL must name a local PostgreSQL test database")
	}
	// Disallow query overrides that could redirect pgx away from the checked host.
	for k := range u.Query() {
		if k != "sslmode" {
			return fmt.Errorf("unsupported database query option %q", k)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := core.Open(ctx, u.String())
	if err != nil {
		return errors.New("cannot connect to local rehearsal database")
	}
	defer admin.DB.Close()
	schema := "rehearsal_" + core.ID()
	if _, err = admin.DB.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, e := admin.DB.ExecContext(cleanup, `DROP SCHEMA "`+schema+`" CASCADE`); e != nil {
			fmt.Fprintln(os.Stderr, "isolated rehearsal schema cleanup failed:", schema)
		}
	}()
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	store, err := core.Open(ctx, u.String())
	if err != nil {
		return err
	}
	defer func() {
		if store != nil {
			store.DB.Close()
		}
	}()
	if err = store.Migrate(ctx); err != nil {
		return err
	}
	if err = store.CheckSchema(ctx); err != nil {
		return err
	}
	tenant := core.ID()
	if err = store.Seed(ctx, tenant); err != nil {
		return err
	}
	key := core.ID() + core.ID()
	api := httptest.NewServer((&httpapi.Server{Store: store, Tenant: tenant, APIKey: key}).Handler())
	defer api.Close()
	client := &http.Client{Timeout: 12 * time.Second}
	stats := &measurements{}
	started := time.Now()
	tasks := make(chan int)
	failures := make(chan error, plays)
	var group sync.WaitGroup
	for i := 0; i < clients; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for job := range tasks {
				var hold core.Reservation
				err := stats.post(ctx, client, api.URL, key, "/api/v1/reservations", core.ID(), core.ReservationInput{CampaignID: "cmp-cascade", ScreenID: "sea-01", Cost: price}, &hold)
				if err != nil {
					failures <- err
					continue
				}
				first := core.Receipt{Version: 1, EventID: core.ID(), ReservationID: hold.ID, ScreenID: hold.ScreenID, PlayedAt: hold.Created, DurationMS: 10000}
				second := first
				second.EventID = core.ID()
				batch := map[string]any{"receipts": []core.Receipt{first, second}}
				var result struct{ Accepted, Rejected, Replayed int }
				if err = stats.post(ctx, client, api.URL, key, "/api/v1/receipts/batch", "", batch, &result); err != nil {
					failures <- err
					continue
				}
				if result.Accepted != 2 || result.Rejected != 0 {
					failures <- errors.New("unexpected batch outcome")
					continue
				}
				if job%10 == 0 {
					if err = stats.post(ctx, client, api.URL, key, "/api/v1/receipts/batch", "", batch, &result); err != nil {
						failures <- err
						continue
					}
					if result.Replayed != 2 {
						failures <- errors.New("batch retry failed to preserve identity")
					}
				}
			}
		}()
	}
	for i := 0; i < plays; i++ {
		tasks <- i
	}
	close(tasks)
	group.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			return e
		}
	}
	ingestDuration := time.Since(started)
	before, err := store.Snapshot(ctx, tenant)
	if err != nil {
		return err
	}
	if before.Counts["accepted"] != 2*plays {
		return fmt.Errorf("pending receipts: %d", before.Counts["accepted"])
	}
	// Close the API and database pool with a durable backlog, then reopen. This
	// tests connection lifecycle recovery, not an OS kill or managed SQL outage.
	api.Close()
	if err = store.DB.Close(); err != nil {
		return err
	}
	store, err = core.Open(ctx, u.String())
	if err != nil {
		return err
	}
	if err = store.CheckSchema(ctx); err != nil {
		return err
	}
	after, err := store.Snapshot(ctx, tenant)
	if err != nil {
		return err
	}
	if after.Counts["accepted"] != before.Counts["accepted"] {
		return errors.New("backlog changed after reopen")
	}
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workerGroup sync.WaitGroup
	defer func() { stopWorkers(); workerGroup.Wait() }()
	drainStarted := time.Now()
	for i := 0; i < workers; i++ {
		w := pipeline.New(store, prometheus.NewRegistry())
		if i == 0 {
			w.DropAck.Store(true)
		}
		workerGroup.Add(1)
		go func() { defer workerGroup.Done(); w.Run(workerCtx) }()
	}
	var snapshot core.Snapshot
	var unfinished, redeliveries int
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox WHERE state <> 'done'`).Scan(&unfinished); err != nil {
			return err
		}
		if unfinished == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if snapshot, err = store.Snapshot(ctx, tenant); err != nil {
		return err
	}
	if err = store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox WHERE attempts > 1`).Scan(&redeliveries); err != nil {
		return err
	}
	var spent, held int64
	for _, c := range snapshot.Campaigns {
		spent += c.Spent
		held += c.Reserved
		if c.Spent+c.Reserved > c.Budget {
			return errors.New("budget cap violated")
		}
	}
	if snapshot.Counts["settled"] != plays || snapshot.Counts["duplicate"] != plays || spent != plays*price || held != 0 || redeliveries < 1 {
		return fmt.Errorf("reconciliation failed: counts=%v spent=%d held=%d retries=%d", snapshot.Counts, spent, held, redeliveries)
	}
	sort.Float64s(stats.Latencies)
	percentile := func(p float64) float64 { return stats.Latencies[int(math.Ceil(float64(len(stats.Latencies))*p))-1] }
	report := map[string]any{"passed": true, "profile": "local PostgreSQL + authenticated loopback HTTP + SQL transport (not managed Pub/Sub)", "plays": plays, "clients": clients, "workers": workers, "http_attempts": stats.Requests, "rate_limited_responses": stats.RateLimited, "logical_operations": len(stats.Latencies), "operation_latency_including_admission_retries_ms": map[string]float64{"p50": percentile(.5), "p95": percentile(.95), "p99": percentile(.99)}, "ingest_seconds": ingestDuration.Seconds(), "drain_seconds": time.Since(drainStarted).Seconds(), "backlog_preserved_on_reopen": after.Counts["accepted"], "redelivered_outbox_rows": redeliveries, "counts": snapshot.Counts, "spent_micros": spent, "held_micros": held, "recorded_at": time.Now().UTC()}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
