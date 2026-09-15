package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Job struct {
	Tenant   string            `json:"tenant"`
	ID       string            `json:"id"`
	Owner    string            `json:"-"`
	Attempts int               `json:"-"`
	Trace    map[string]string `json:"trace,omitempty"`
}

func persistTrace(ctx context.Context, tx *sql.Tx, tenant, id string) error {
	c := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, c)
	b, _ := json.Marshal(c)
	_, e := tx.ExecContext(ctx, `INSERT INTO dispatch_context(tenant,id,carrier) VALUES($1,$2,$3)`, tenant, id, string(b))
	return e
}

// Claim leases work in a transaction. SKIP LOCKED allows Postgres dispatcher replicas
// to progress independently. The owner token fences a stale dispatcher after expiry.
func (s *Store) Claim(ctx context.Context, now int64) (*Job, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	q := `SELECT tenant,id,attempts FROM outbox WHERE state IN ('pending','leased') AND next_at <= $1 AND lease_until <= $1 ORDER BY next_at,id LIMIT 1`
	if s.Postgres {
		q += ` FOR UPDATE SKIP LOCKED`
	}
	j := &Job{Owner: ID()}
	e = tx.QueryRowContext(ctx, q, now).Scan(&j.Tenant, &j.ID, &j.Attempts)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	j.Attempts++
	_, e = tx.ExecContext(ctx, `UPDATE outbox SET state='leased',owner=$1,lease_until=$2,attempts=attempts+1 WHERE tenant=$3 AND id=$4`, j.Owner, now+30000, j.Tenant, j.ID)
	if e != nil {
		return nil, e
	}
	var carrier string
	e = tx.QueryRowContext(ctx, `SELECT carrier FROM dispatch_context WHERE tenant=$1 AND id=$2`, j.Tenant, j.ID).Scan(&carrier)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	if carrier != "" {
		if e = json.Unmarshal([]byte(carrier), &j.Trace); e != nil {
			return nil, e
		}
	}
	return j, tx.Commit()
}
func (s *Store) Dispatched(ctx context.Context, j *Job) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE outbox SET state='dispatched',owner='',lease_until=0 WHERE tenant=$1 AND id=$2 AND owner=$3 AND state='leased'`, j.Tenant, j.ID, j.Owner)
	return e
}
func (s *Store) Complete(ctx context.Context, tenant, id string) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	// Match replay's lock order. A subquery without this lock could see a stale
	// delivery status while waiting behind replay's outbox update on PostgreSQL.
	if _, e = tx.ExecContext(ctx, `UPDATE deliveries SET status=status WHERE tenant=$1 AND id=$2`, tenant, id); e != nil {
		return e
	}
	var status string
	if e = tx.QueryRowContext(ctx, `SELECT status FROM deliveries WHERE tenant=$1 AND id=$2`, tenant, id).Scan(&status); e != nil {
		return e
	}
	if status == "accepted" {
		return nil
	}
	if _, e = tx.ExecContext(ctx, `UPDATE outbox SET state='done',owner='',lease_until=0 WHERE tenant=$1 AND id=$2`, tenant, id); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) FailJob(ctx context.Context, j *Job, next int64) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `UPDATE deliveries SET status=status WHERE tenant=$1 AND id=$2`, j.Tenant, j.ID); e != nil {
		return e
	}
	state := "pending"
	if j.Attempts >= 5 {
		state = "dead"
	}
	result, e := tx.ExecContext(ctx, `UPDATE outbox SET state=$1,owner='',lease_until=0,next_at=$2 WHERE tenant=$3 AND id=$4 AND owner=$5 AND state='leased'`, state, next, j.Tenant, j.ID, j.Owner)
	if e != nil {
		return e
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil
	}
	if state == "dead" {
		if _, e = tx.ExecContext(ctx, `UPDATE deliveries SET status='failed',reason='dispatch_retries_exhausted' WHERE tenant=$1 AND id=$2 AND status='accepted'`, j.Tenant, j.ID); e != nil {
			return e
		}
	}
	if e = audit(ctx, tx, j.Tenant, "retry", j.ID, state); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Replay(ctx context.Context, tenant, id string) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.ExecContext(ctx, `UPDATE deliveries SET status='accepted',reason='' WHERE tenant=$1 AND id=$2 AND status IN ('failed','quarantined')`, tenant, id)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `UPDATE outbox SET state='pending',owner='',lease_until=0,next_at=0,attempts=0 WHERE tenant=$1 AND id=$2`, tenant, id); e != nil {
		return e
	}
	if e = audit(ctx, tx, tenant, "replayed", id, "Original immutable payload requeued; validation still applies"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) QuarantineTransport(ctx context.Context, id string, payload []byte, reason string) error {
	// Bound poison-message retention per entry; payloads are never rendered as HTML.
	if len(payload) > 32768 {
		payload = payload[:32768]
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO transport_quarantine(id,payload,reason,created) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, id, string(payload), reason, time.Now().UnixMilli())
	return e
}
