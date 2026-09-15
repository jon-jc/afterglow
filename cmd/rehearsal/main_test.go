package main

import "testing"

func TestRehearsalRefusesUnsafeTargetsBeforeConnecting(t *testing.T) {
	for _, dsn := range []string{"", "postgres://production.example/db", "https://localhost/db", "postgres://localhost/db?host=production.example", "postgres://localhost/db?service=production"} {
		t.Run(dsn, func(t *testing.T) {
			t.Setenv("REHEARSAL_DATABASE_URL", dsn)
			if err := run(); err == nil {
				t.Fatal("unsafe rehearsal target accepted")
			}
		})
	}
}
