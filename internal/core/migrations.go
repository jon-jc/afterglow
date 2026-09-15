package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const schemaVersion = 1

var ErrSchemaMismatch = errors.New("database schema does not match this application; run the matching migration release")

func schemaChecksum() string {
	sum := sha256.Sum256([]byte(strings.ReplaceAll(schema, "\r\n", "\n")))
	return hex.EncodeToString(sum[:])
}

// CheckSchema only reads metadata. Runtime database roles do not need DDL grants.
// The checksum detects changed migration sources, not arbitrary manual DDL drift.
func (s *Store) CheckSchema(ctx context.Context) error {
	var version int64
	var checksum string
	if err := s.DB.QueryRowContext(ctx, `SELECT version,checksum FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &checksum); err != nil {
		return fmt.Errorf("%w: migration metadata unavailable", ErrSchemaMismatch)
	}
	if version != schemaVersion || checksum != schemaChecksum() {
		return ErrSchemaMismatch
	}
	return nil
}
