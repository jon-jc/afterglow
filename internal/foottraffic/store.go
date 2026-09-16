// Package foottraffic reconciles immutable, synthetic partner count windows.
// It accepts no device identifiers, coordinates, or individual trajectories.
package foottraffic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"
)

const Threshold = 20
const Source = "synthetic-partner-v1"

var ErrInvalid = errors.New("invalid aggregate batch")
var ErrConflict = errors.New("immutable batch or window conflicts with stored content")
var identity = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Context string `json:"context"`
}

var zones = []Zone{
	{"downtown", "Downtown corridor", "Synthetic street-level zone"},
	{"transit", "Transit concourse", "Synthetic transit zone"},
	{"retail", "Retail district", "Synthetic retail zone"},
	{"neighborhood", "Neighborhood plaza", "Synthetic low-volume zone"},
}

type Window struct {
	Zone  string `json:"zone_id"`
	Start int64  `json:"window_start"` // Unix milliseconds, aligned to a UTC hour.
	Count int64  `json:"observations"`
}
type Batch struct {
	ID      string   `json:"batch_id"`
	Source  string   `json:"source"`
	Windows []Window `json:"windows"`
}
type Result struct {
	ID       string `json:"batch_id"`
	Inserted int    `json:"inserted_windows"`
	Replayed bool   `json:"replayed"`
}
type Store struct{ DB *sql.DB }

// The measurement service owns separate tables; it does not alter settlement state.
// This initial additive schema can be installed by the demo or an explicit migrate command.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS traffic_scopes_v1 (tenant TEXT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS traffic_batches_v1 (tenant TEXT NOT NULL, id TEXT NOT NULL, fingerprint TEXT NOT NULL, inserted BIGINT NOT NULL, PRIMARY KEY(tenant,id))`,
		`CREATE TABLE IF NOT EXISTS traffic_windows_v1 (tenant TEXT NOT NULL, source TEXT NOT NULL, zone TEXT NOT NULL, start_ms BIGINT NOT NULL, observations BIGINT NOT NULL CHECK(observations >= 0 AND observations <= 1000000), PRIMARY KEY(tenant,source,zone,start_ms))`,
		`CREATE INDEX IF NOT EXISTS traffic_time_v1 ON traffic_windows_v1(tenant,start_ms)`,
	} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Ingest(ctx context.Context, tenant string, b Batch, now time.Time) (Result, error) {
	return s.ingest(ctx, tenant, b, now, false)
}

// ReplaceSample swaps a random sandbox dataset atomically. An exact retry of
// the current batch returns its result before any rows are removed.
func (s *Store) ReplaceSample(ctx context.Context, tenant string, b Batch, now time.Time) (Result, error) {
	return s.ingest(ctx, tenant, b, now, true)
}

func (s *Store) ingest(ctx context.Context, tenant string, b Batch, now time.Time, replace bool) (Result, error) {
	result := Result{ID: b.ID}
	if !identity.MatchString(tenant) || !identity.MatchString(b.ID) || b.Source != Source || len(b.Windows) < 1 || len(b.Windows) > 96 {
		return result, ErrInvalid
	}
	// Sort a private copy, making retries independent of window order.
	b.Windows = append([]Window(nil), b.Windows...)
	sort.Slice(b.Windows, func(i, j int) bool {
		if b.Windows[i].Start == b.Windows[j].Start {
			return b.Windows[i].Zone < b.Windows[j].Zone
		}
		return b.Windows[i].Start < b.Windows[j].Start
	})
	end := now.UTC().Truncate(time.Hour).UnixMilli()
	for i, w := range b.Windows {
		known := false
		for _, z := range zones {
			if z.ID == w.Zone {
				known = true
			}
		}
		if !known || w.Count < 0 || w.Count > 1000000 || w.Start%3600000 != 0 || w.Start < end-int64(7*24*time.Hour/time.Millisecond) || w.Start >= end {
			return result, ErrInvalid
		}
		if i > 0 && b.Windows[i-1].Zone == w.Zone && b.Windows[i-1].Start == w.Start {
			return result, ErrInvalid
		}
	}
	encoded, _ := json.Marshal(b)
	hash := sha256.Sum256(encoded)
	fp := hex.EncodeToString(hash[:])
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = lockScope(ctx, tx, tenant); err != nil {
		return result, err
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO traffic_batches_v1(tenant,id,fingerprint,inserted) VALUES($1,$2,$3,0) ON CONFLICT DO NOTHING`, tenant, b.ID, fp)
	if err != nil {
		return result, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return result, err
	}
	if n == 0 {
		var previous string
		if err = tx.QueryRowContext(ctx, `SELECT fingerprint,inserted FROM traffic_batches_v1 WHERE tenant=$1 AND id=$2`, tenant, b.ID).Scan(&previous, &result.Inserted); err != nil {
			return result, err
		}
		if previous != fp {
			return result, ErrConflict
		}
		result.Replayed = true
		return result, nil
	}
	if replace {
		if _, err = tx.ExecContext(ctx, `DELETE FROM traffic_windows_v1 WHERE tenant=$1`, tenant); err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM traffic_batches_v1 WHERE tenant=$1 AND id<>$2`, tenant, b.ID); err != nil {
			return result, err
		}
	}
	for _, w := range b.Windows {
		r, err = tx.ExecContext(ctx, `INSERT INTO traffic_windows_v1(tenant,source,zone,start_ms,observations) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, tenant, b.Source, w.Zone, w.Start, w.Count)
		if err != nil {
			return result, err
		}
		n, err = r.RowsAffected()
		if err != nil {
			return result, err
		}
		if n == 0 {
			var count int64
			if err = tx.QueryRowContext(ctx, `SELECT observations FROM traffic_windows_v1 WHERE tenant=$1 AND source=$2 AND zone=$3 AND start_ms=$4`, tenant, b.Source, w.Zone, w.Start).Scan(&count); err != nil {
				return result, err
			}
			if count != w.Count {
				return result, ErrConflict
			}
		} else {
			result.Inserted++
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE traffic_batches_v1 SET inserted=$1 WHERE tenant=$2 AND id=$3`, result.Inserted, tenant, b.ID); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

// Both imports and demo clearing acquire this database-backed lock. A reset
// cannot remove a concurrent import's windows while leaving its replay ledger.
// The short transaction serializes writes only within one sample namespace.
func lockScope(ctx context.Context, tx *sql.Tx, tenant string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO traffic_scopes_v1(tenant) VALUES($1) ON CONFLICT(tenant) DO UPDATE SET tenant=excluded.tenant`, tenant)
	return err
}

type EmptyResult struct {
	Batches int64 `json:"removed_batches"`
	Windows int64 `json:"removed_windows"`
}

// EmptySample removes this sample's synthetic observations and replay records
// together. HTTP access is enabled only by the explicit demo handler.
func (s *Store) EmptySample(ctx context.Context, tenant string) (EmptyResult, error) {
	var result EmptyResult
	if !identity.MatchString(tenant) {
		return result, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = lockScope(ctx, tx, tenant); err != nil {
		return result, err
	}
	for _, operation := range []struct {
		query string
		count *int64
	}{
		{`DELETE FROM traffic_windows_v1 WHERE tenant=$1`, &result.Windows},
		{`DELETE FROM traffic_batches_v1 WHERE tenant=$1`, &result.Batches},
	} {
		r, err := tx.ExecContext(ctx, operation.query, tenant)
		if err != nil {
			return EmptyResult{}, err
		}
		if *operation.count, err = r.RowsAffected(); err != nil {
			return EmptyResult{}, err
		}
	}
	return result, tx.Commit()
}

type Cell struct {
	Start  int64  `json:"window_start"`
	Status string `json:"status"`
	Count  *int64 `json:"observations"` // Null for missing and suppressed windows.
}
type ZoneReport struct {
	Zone
	Current    []Cell   `json:"current"`
	Previous   []Cell   `json:"previous"`
	Published  int64    `json:"published_observations"`
	Comparable int      `json:"comparable_windows"`
	Change     *float64 `json:"change_percent"`
}
type Report struct {
	Source      string       `json:"source"`
	Synthetic   bool         `json:"synthetic"`
	AsOf        int64        `json:"as_of"`
	WindowStart int64        `json:"window_start"`
	WindowEnd   int64        `json:"window_end"`
	Minimum     int          `json:"minimum_reportable_count"`
	Published   int64        `json:"published_observations"`
	Suppressed  int          `json:"suppressed_windows"`
	Missing     int          `json:"missing_windows"`
	Released    int          `json:"released_windows"`
	Zones       []ZoneReport `json:"zones"`
	Scope       string       `json:"scope"`
}

func (s *Store) Report(ctx context.Context, tenant string, now time.Time) (Report, error) {
	end := now.UTC().Truncate(time.Hour)
	start := end.Add(-6 * time.Hour)
	r := Report{Source: Source, Synthetic: true, AsOf: now.UnixMilli(), WindowStart: start.UnixMilli(), WindowEnd: end.UnixMilli(), Minimum: Threshold, Zones: []ZoneReport{}, Scope: "Synthetic observations in fixed hourly zones. Not unique people, verified ad exposure, store visits or causal campaign lift. Comparisons use only paired reportable windows, 24 hours apart. Small-count suppression alone is not an anonymity guarantee."}
	rows, err := s.DB.QueryContext(ctx, `SELECT zone,start_ms,observations FROM traffic_windows_v1 WHERE tenant=$1 AND source=$2 AND start_ms >= $3 AND start_ms < $4`, tenant, Source, start.Add(-24*time.Hour).UnixMilli(), end.UnixMilli())
	if err != nil {
		return r, err
	}
	defer rows.Close()
	type key struct {
		zone  string
		start int64
	}
	values := map[key]int64{}
	for rows.Next() {
		var k key
		var count int64
		if err = rows.Scan(&k.zone, &k.start, &count); err != nil {
			return r, err
		}
		values[k] = count
	}
	if err = rows.Err(); err != nil {
		return r, err
	}
	cell := func(zone string, t time.Time) Cell {
		c := Cell{Start: t.UnixMilli(), Status: "missing"}
		if n, ok := values[key{zone, c.Start}]; ok {
			c.Status = "suppressed"
			if n >= Threshold {
				c.Status = "reported"
				c.Count = &n
			}
		}
		return c
	}
	for _, z := range zones {
		zr := ZoneReport{Zone: z, Current: []Cell{}, Previous: []Cell{}}
		var current, previous int64
		for i := 0; i < 6; i++ {
			t := start.Add(time.Duration(i) * time.Hour)
			a, b := cell(z.ID, t), cell(z.ID, t.Add(-24*time.Hour))
			zr.Current = append(zr.Current, a)
			zr.Previous = append(zr.Previous, b)
			switch a.Status {
			case "reported":
				zr.Published += *a.Count
				r.Published += *a.Count
				r.Released++
			case "suppressed":
				r.Suppressed++
			case "missing":
				r.Missing++
			}
			if a.Count != nil && b.Count != nil {
				zr.Comparable++
				current += *a.Count
				previous += *b.Count
			}
		}
		if zr.Comparable > 0 && previous > 0 {
			v := 100 * float64(current-previous) / float64(previous)
			zr.Change = &v
		}
		r.Zones = append(r.Zones, zr)
	}
	return r, nil
}

// Example is deterministic per zone/hour, so overlapping demo batches agree.
func Example(now time.Time) Batch {
	end := now.UTC().Truncate(time.Hour)
	b := Batch{ID: fmt.Sprintf("sample-%d", end.Unix()), Source: Source, Windows: []Window{}}
	for day := 0; day < 2; day++ {
		for h := 1; h <= 6; h++ {
			t := end.Add(-time.Duration(h+day*24) * time.Hour)
			for i, z := range zones {
				hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", z.ID, t.Unix())))
				count := int64(35 + i*18 + int(hash[0])%95)
				if z.ID == "neighborhood" {
					count = int64(6 + int(hash[0])%25)
				}
				b.Windows = append(b.Windows, Window{Zone: z.ID, Start: t.UnixMilli(), Count: count})
			}
		}
	}
	return b
}
