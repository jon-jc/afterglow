package main

import "testing"

func TestProtectedStartupRequiresExplicitProductionSettings(t *testing.T) {
	key := "test-only-credential-long-enough"
	for _, tc := range []struct {
		dsn, tenant, key string
		valid            bool
	}{
		{"", "tenant", key, false}, {"data/local.db", "tenant", key, false}, {"postgres://localhost/db", "", key, false}, {"postgres://localhost/db", "tenant", "short", false}, {"postgres://localhost/db", "tenant", key, true},
	} {
		if (validateProduction(false, tc.dsn, tc.tenant, tc.key) == nil) != tc.valid {
			t.Fatalf("unexpected config result for %q", tc.dsn)
		}
	}
	if validateProduction(true, "", "", "") != nil {
		t.Fatal("local demo must remain easy to run")
	}
}
