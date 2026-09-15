package main

import (
	"errors"
	"strings"
)

func validateProduction(demo bool, dsn, tenant, key string) error {
	if demo {
		return nil
	}
	if !(strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")) {
		return errors.New("protected mode requires an explicit PostgreSQL DATABASE_URL")
	}
	if strings.TrimSpace(tenant) == "" {
		return errors.New("protected mode requires an explicit TENANT_ID")
	}
	if len(key) < 24 {
		return errors.New("protected mode requires an API_KEY of at least 24 characters")
	}
	return nil
}
