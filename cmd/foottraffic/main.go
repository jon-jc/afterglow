// The standalone synthetic measurement service can use its own SQLite or PostgreSQL database.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/foottraffic"
)

func value(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	if err := run(); err != nil {
		slog.Error("foottraffic_failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	db, err := core.Open(startup, value("FOOT_TRAFFIC_DATABASE_URL", "foottraffic.db"))
	if err != nil {
		return err
	}
	defer db.DB.Close()
	store := &foottraffic.Store{DB: db.DB}
	if os.Getenv("FOOT_TRAFFIC_MIGRATE") == "true" {
		return store.Migrate(startup)
	}
	key := os.Getenv("FOOT_TRAFFIC_API_KEY")
	if len(key) < 24 {
		return fmt.Errorf("FOOT_TRAFFIC_API_KEY must have at least 24 characters; migrate the database explicitly before serving")
	}
	if _, err = db.DB.ExecContext(startup, `SELECT 1 FROM traffic_batches_v1 LIMIT 1`); err != nil {
		return fmt.Errorf("measurement schema missing; run with FOOT_TRAFFIC_MIGRATE=true: %w", err)
	}
	handler := store.Handler(value("FOOT_TRAFFIC_TENANT", "demo"))
	expected := sha256.Sum256([]byte("Bearer " + key))
	slots := make(chan struct{}, 32)
	server := &http.Server{Addr: value("FOOT_TRAFFIC_ADDR", "127.0.0.1:8092"), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "authentication required", 401)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host {
					http.Error(w, "origin refused", 403)
					return
				}
			}
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "busy", 503)
			return
		}
		handler.ServeHTTP(w, r)
	})
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case err = <-done:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	drain, finish := context.WithTimeout(context.Background(), 10*time.Second)
	defer finish()
	return server.Shutdown(drain)
}
