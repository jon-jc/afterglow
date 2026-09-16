// Package localdemo owns destructive maintenance for the single-process SQLite
// sandbox. It is deliberately separate from production storage operations.
package localdemo

import (
	"context"
	"errors"

	"github.com/jon-jc/afterglow/internal/core"
)

var ErrUnavailable = errors.New("reset requires the local SQLite demo")

// Initialize persists a generation across restarts so only a real reset
// invalidates saved browser runs. Call after installing both demo schemas.
func Initialize(ctx context.Context, s *core.Store) (string, error) {
	if s.Postgres {
		return "", ErrUnavailable
	}
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS demo_reset_state (id INTEGER PRIMARY KEY CHECK(id=1), generation TEXT NOT NULL)`); err != nil {
		return "", err
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO demo_reset_state(id,generation) VALUES(1,$1) ON CONFLICT DO NOTHING`, core.ID()); err != nil {
		return "", err
	}
	var generation string
	err := s.DB.QueryRowContext(ctx, `SELECT generation FROM demo_reset_state WHERE id=1`).Scan(&generation)
	return generation, err
}

// Reset removes activity across the dedicated local database, preserving the
// inventory, campaign definitions and schema. The caller must exclude HTTP
// operations and wait for the local worker before calling this function.
func Reset(ctx context.Context, s *core.Store) (string, error) {
	if s.Postgres {
		return "", ErrUnavailable
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	for _, table := range []string{"dispatch_context", "outbox", "deliveries", "reservations", "audit", "transport_quarantine", "traffic_windows_v1", "traffic_batches_v1", "traffic_scopes_v1"} {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
			return "", err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE campaigns SET reserved=0, spent=0`); err != nil {
		return "", err
	}
	generation := core.ID()
	result, err := tx.ExecContext(ctx, `UPDATE demo_reset_state SET generation=$1 WHERE id=1`, generation)
	if err != nil {
		return "", err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return "", errors.New("demo reset state is not initialized")
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return generation, nil
}
