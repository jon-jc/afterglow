package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// DeliveryEvidence is a consistent read of a receipt and its financial context.
// Dispatch state may lag a committed decision until acknowledgement completes.
type DeliveryEvidence struct {
	Delivery         Delivery     `json:"delivery"`
	Reservation      *Reservation `json:"reservation,omitempty"`
	OutboxState      string       `json:"outbox_state"`
	DispatchAttempts int          `json:"dispatch_attempts"`
	ObservedAt       int64        `json:"observed_at"`
}

func (s *Store) Evidence(ctx context.Context, tenant, id string) (DeliveryEvidence, error) {
	out := DeliveryEvidence{ObservedAt: millis()}
	if !validID(id) {
		return out, invalid("invalid_delivery_id")
	}
	opts := &sql.TxOptions{ReadOnly: true}
	if s.Postgres {
		opts.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.DB.BeginTx(ctx, opts)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	var payload string
	d := &out.Delivery
	err = tx.QueryRowContext(ctx, `SELECT d.id,d.payload,d.status,d.reason,d.created,d.attempts,o.state,o.attempts FROM deliveries d JOIN outbox o ON o.tenant=d.tenant AND o.id=d.id WHERE d.tenant=$1 AND d.id=$2`, tenant, id).Scan(&d.ID, &payload, &d.Status, &d.Reason, &d.Created, &d.Attempts, &out.OutboxState, &out.DispatchAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal([]byte(payload), &d.Receipt); err != nil {
		return out, err
	}
	r, err := scanReservation(tx.QueryRowContext(ctx, `SELECT `+reservationFields+` FROM reservations WHERE tenant=$1 AND id=$2`, tenant, d.ReservationID))
	if err == nil {
		out.Reservation = &r
	} else if !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	return out, tx.Commit()
}
