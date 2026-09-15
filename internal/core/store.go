package core

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Store struct {
	DB       *sql.DB
	Postgres bool
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	pg := strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
	driver := "sqlite"
	if pg {
		driver = "pgx"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if pg {
		db.SetMaxOpenConns(12)
		db.SetMaxIdleConns(4)
	} else {
		db.SetMaxOpenConns(1)
	}
	s := &Store{DB: db, Postgres: pg}
	if !pg {
		for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA foreign_keys=ON", "PRAGMA busy_timeout=5000"} {
			if _, err = db.ExecContext(ctx, q); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.Postgres {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(114921873)`); err != nil {
			return err
		}
	}
	for _, q := range strings.Split(schema, ";") {
		if strings.TrimSpace(q) != "" {
			if _, err = tx.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func audit(ctx context.Context, tx *sql.Tx, tenant, kind, resource, detail string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit(id,tenant,kind,resource,detail,created) VALUES($1,$2,$3,$4,$5,$6)`, ID(), tenant, kind, resource, detail, millis())
	return err
}
func (s *Store) Seed(ctx context.Context, tenant string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range []Campaign{{ID: "cmp-cascade", Name: "Cascade / A little further", Budget: 250000000}, {ID: "cmp-northstar", Name: "Northstar / Next departure", Budget: 180000000}, {ID: "cmp-solstice", Name: "Solstice / Find your outside", Budget: 120000000}} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO campaigns(tenant,id,name,budget) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, tenant, c.ID, c.Name, c.Budget); err != nil {
			return err
		}
	}
	for _, v := range []Screen{{"sea-01", "SEA / Concourse A", "Seattle", "Airport", 47.45, -122.31}, {"sea-02", "I-5 / Downtown", "Seattle", "Roadside", 47.61, -122.33}, {"pdx-01", "PDX / North lobby", "Portland", "Airport", 45.59, -122.60}, {"sfo-01", "SFO / Terminal 2", "San Francisco", "Airport", 37.62, -122.38}, {"lax-01", "LAX / Arrivals", "Los Angeles", "Airport", 33.94, -118.40}, {"den-01", "DEN / Concourse B", "Denver", "Airport", 39.86, -104.67}, {"ord-01", "ORD / Terminal 1", "Chicago", "Airport", 41.98, -87.90}, {"jfk-01", "JFK / Departures", "New York", "Airport", 40.64, -73.78}} {
		if _, err = tx.ExecContext(ctx, `INSERT INTO screens(id,name,market,format,lat,lng) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, v.ID, v.Name, v.Market, v.Format, v.Lat, v.Lng); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func scanReservation(row interface{ Scan(...any) error }) (Reservation, error) {
	var r Reservation
	err := row.Scan(&r.ID, &r.CampaignID, &r.ScreenID, &r.Cost, &r.State, &r.Created, &r.Expires)
	return r, err
}

const reservationFields = "id,campaign,screen,cost,state,created,expires"

func (s *Store) Reserve(ctx context.Context, tenant, key string, in ReservationInput) (Reservation, bool, error) {
	if !validID(key) || !validID(in.CampaignID) || !validID(in.ScreenID) || in.Cost <= 0 || in.Cost > 1000000000 {
		return Reservation{}, false, invalid("valid IDs and cost_micros between 1 and 1000000000 required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Reservation{}, false, err
	}
	defer tx.Rollback()
	// Serialize on the campaign BEFORE reading its budget. On Postgres this is a row lock.
	// SQLite's single connection is only a portable demo substitute, not a scaling claim.
	res, err := tx.ExecContext(ctx, `UPDATE campaigns SET reserved=reserved WHERE tenant=$1 AND id=$2`, tenant, in.CampaignID)
	if err != nil {
		return Reservation{}, false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Reservation{}, false, ErrNotFound
	}
	var fp string
	err = tx.QueryRowContext(ctx, `SELECT fingerprint FROM reservations WHERE tenant=$1 AND idem_key=$2`, tenant, key).Scan(&fp)
	if err == nil {
		if fp != digest(in) {
			return Reservation{}, false, ErrConflict
		}
		r, e := scanReservation(tx.QueryRowContext(ctx, `SELECT `+reservationFields+` FROM reservations WHERE tenant=$1 AND idem_key=$2`, tenant, key))
		return r, true, e
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, false, err
	}
	var screen string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM screens WHERE id=$1`, in.ScreenID).Scan(&screen); errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, false, ErrNotFound
	} else if err != nil {
		return Reservation{}, false, err
	}
	res, err = tx.ExecContext(ctx, `UPDATE campaigns SET reserved=reserved+$1 WHERE tenant=$2 AND id=$3 AND budget-spent-reserved >= $1`, in.Cost, tenant, in.CampaignID)
	if err != nil {
		return Reservation{}, false, err
	}
	n, _ = res.RowsAffected()
	if n == 0 {
		return Reservation{}, false, ErrBudget
	}
	r := Reservation{ID: ID(), CampaignID: in.CampaignID, ScreenID: in.ScreenID, Cost: in.Cost, State: "held", Created: millis(), Expires: millis() + int64(15*time.Minute/time.Millisecond)}
	res, err = tx.ExecContext(ctx, `INSERT INTO reservations(tenant,id,campaign,screen,cost,state,idem_key,fingerprint,created,expires) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(tenant,idem_key) DO NOTHING`, tenant, r.ID, r.CampaignID, r.ScreenID, r.Cost, r.State, key, digest(in), r.Created, r.Expires)
	if err != nil {
		return Reservation{}, false, err
	}
	n, _ = res.RowsAffected()
	if n == 0 {
		return Reservation{}, false, ErrConflict
	}
	if err = audit(ctx, tx, tenant, "reserved", r.ID, fmt.Sprintf("%d micros held; %s", r.Cost, r.ScreenID)); err != nil {
		return Reservation{}, false, err
	}
	err = tx.Commit()
	return r, false, err
}

func (s *Store) Accept(ctx context.Context, tenant string, r Receipt) (string, bool, error) {
	if !validID(r.EventID) || !validID(r.ReservationID) || !validID(r.ScreenID) {
		return "", false, invalid("event, reservation and screen IDs required")
	}
	b, _ := json.Marshal(r)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	id := ID()
	res, err := tx.ExecContext(ctx, `INSERT INTO deliveries(tenant,id,event_id,fingerprint,payload,status,created) VALUES($1,$2,$3,$4,$5,'accepted',$6) ON CONFLICT(tenant,event_id) DO NOTHING`, tenant, id, r.EventID, digest(r), string(b), millis())
	if err != nil {
		return "", false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var fp string
		err = tx.QueryRowContext(ctx, `SELECT id,fingerprint FROM deliveries WHERE tenant=$1 AND event_id=$2`, tenant, r.EventID).Scan(&id, &fp)
		if err != nil {
			return "", false, err
		}
		if fp != digest(r) {
			return "", false, ErrConflict
		}
		return id, true, nil
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO outbox(tenant,id) VALUES($1,$2)`, tenant, id); err != nil {
		return "", false, err
	}
	if err = persistTrace(ctx, tx, tenant, id); err != nil {
		return "", false, err
	}
	if err = audit(ctx, tx, tenant, "accepted", id, "Receipt and dispatch intent committed together"); err != nil {
		return "", false, err
	}
	return id, false, tx.Commit()
}

// Process commits the decision, reservation transition, budget and audit as ONE transaction.
// Re-delivery after commit (including a lost broker ack) is a no-op.
func (s *Store) Process(ctx context.Context, tenant, id string) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE deliveries SET attempts=attempts+1 WHERE tenant=$1 AND id=$2`, tenant, id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", ErrNotFound
	}
	var body, status string
	if err = tx.QueryRowContext(ctx, `SELECT payload,status FROM deliveries WHERE tenant=$1 AND id=$2`, tenant, id).Scan(&body, &status); err != nil {
		return "", err
	}
	if status != "accepted" {
		return status, tx.Commit()
	}
	var p Receipt
	reason := ""
	if json.Unmarshal([]byte(body), &p) != nil {
		reason = "malformed_payload"
	} else if p.Version != 1 {
		reason = "unsupported_schema"
	} else if p.DurationMS < 1000 || p.DurationMS > 120000 {
		reason = "invalid_play_duration"
	} else if p.PlayedAt > millis()+60000 {
		reason = "future_play_time"
	}
	var r Reservation
	if reason == "" {
		// Every receipt for this reservation takes the same lock before changing state.
		_, err = tx.ExecContext(ctx, `UPDATE reservations SET state=state WHERE tenant=$1 AND id=$2`, tenant, p.ReservationID)
		if err != nil {
			return "", err
		}
		r, err = scanReservation(tx.QueryRowContext(ctx, `SELECT `+reservationFields+` FROM reservations WHERE tenant=$1 AND id=$2`, tenant, p.ReservationID))
		if errors.Is(err, sql.ErrNoRows) {
			reason = "unknown_reservation"
		} else if err != nil {
			return "", err
		} else if r.ScreenID != p.ScreenID {
			reason = "screen_mismatch"
		} else if p.PlayedAt < r.Created || p.PlayedAt > r.Expires {
			reason = "outside_reservation_window"
		} else if r.State == "released" {
			reason = "reservation_released"
		}
	}
	status = "quarantined"
	if reason == "" {
		status = "duplicate"
		reason = "reservation_already_settled"
		if r.State == "held" {
			if _, err = tx.ExecContext(ctx, `UPDATE reservations SET state='settled' WHERE tenant=$1 AND id=$2`, tenant, r.ID); err != nil {
				return "", err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE campaigns SET reserved=reserved-$1,spent=spent+$1 WHERE tenant=$2 AND id=$3`, r.Cost, tenant, r.CampaignID); err != nil {
				return "", err
			}
			status = "settled"
			reason = "matched_screen_window_and_reservation"
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE deliveries SET status=$1,reason=$2 WHERE tenant=$3 AND id=$4`, status, reason, tenant, id); err != nil {
		return "", err
	}
	if err = audit(ctx, tx, tenant, status, id, reason); err != nil {
		return "", err
	}
	return status, tx.Commit()
}

// ReleaseExpired waits an additional 15 minutes for delayed delivery before releasing a hold.
// A later proof is quarantined; it cannot resurrect money already allocated elsewhere.
func (s *Store) ReleaseExpired(ctx context.Context, now int64) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT tenant,id FROM reservations WHERE state='held' AND expires < $1 LIMIT 100`, now-900000)
	if err != nil {
		return err
	}
	var ids [][2]string
	for rows.Next() {
		var x [2]string
		if err = rows.Scan(&x[0], &x[1]); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, x := range ids {
		if err = s.release(ctx, x[0], x[1], now); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) release(ctx context.Context, tenant, id string, now int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE reservations SET state='released' WHERE tenant=$1 AND id=$2 AND state='held' AND expires<$3`, tenant, id, now-900000)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	r, err := scanReservation(tx.QueryRowContext(ctx, `SELECT `+reservationFields+` FROM reservations WHERE tenant=$1 AND id=$2`, tenant, id))
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE campaigns SET reserved=reserved-$1 WHERE tenant=$2 AND id=$3`, r.Cost, tenant, r.CampaignID); err != nil {
		return err
	}
	if err = audit(ctx, tx, tenant, "released", id, "Expired after receipt grace period; funds returned"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Snapshot(ctx context.Context, tenant string) (Snapshot, error) {
	out := Snapshot{Campaigns: []Campaign{}, Screens: []Screen{}, Reservations: []Reservation{}, Deliveries: []Delivery{}, Audit: []Audit{}, Counts: map[string]int64{}, Now: millis()}
	// A single read transaction avoids showing a budget from before a receipt with a status from after it.
	opts := &sql.TxOptions{ReadOnly: true}
	if s.Postgres {
		opts.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.DB.BeginTx(ctx, opts)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,name,budget,reserved,spent FROM campaigns WHERE tenant=$1 ORDER BY id`, tenant)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c Campaign
		if err = rows.Scan(&c.ID, &c.Name, &c.Budget, &c.Reserved, &c.Spent); err != nil {
			rows.Close()
			return out, err
		}
		out.Campaigns = append(out.Campaigns, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,name,market,format,lat,lng FROM screens ORDER BY id`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var v Screen
		if err = rows.Scan(&v.ID, &v.Name, &v.Market, &v.Format, &v.Lat, &v.Lng); err != nil {
			rows.Close()
			return out, err
		}
		out.Screens = append(out.Screens, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT `+reservationFields+` FROM reservations WHERE tenant=$1 ORDER BY created DESC,id LIMIT 100`, tenant)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		r, e := scanReservation(rows)
		if e != nil {
			rows.Close()
			return out, e
		}
		out.Reservations = append(out.Reservations, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,payload,status,reason,created,attempts FROM deliveries WHERE tenant=$1 ORDER BY created DESC,id LIMIT 100`, tenant)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var d Delivery
		var b string
		if err = rows.Scan(&d.ID, &b, &d.Status, &d.Reason, &d.Created, &d.Attempts); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal([]byte(b), &d.Receipt); err != nil {
			rows.Close()
			return out, err
		}
		out.Deliveries = append(out.Deliveries, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT id,kind,resource,detail,created FROM audit WHERE tenant=$1 ORDER BY created DESC,id LIMIT 80`, tenant)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var a Audit
		if err = rows.Scan(&a.ID, &a.Kind, &a.Resource, &a.Detail, &a.Created); err != nil {
			rows.Close()
			return out, err
		}
		out.Audit = append(out.Audit, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT status,COUNT(*) FROM deliveries WHERE tenant=$1 GROUP BY status`, tenant)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var k string
		var n int64
		if err = rows.Scan(&k, &n); err != nil {
			rows.Close()
			return out, err
		}
		out.Counts[k] = n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	var oldest sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT MIN(created) FROM deliveries WHERE tenant=$1 AND status='accepted'`, tenant).Scan(&oldest); err != nil {
		return out, err
	}
	if oldest.Valid {
		out.OldestPendingMS = out.Now - oldest.Int64
	}
	return out, tx.Commit()
}
