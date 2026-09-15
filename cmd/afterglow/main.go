package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/httpapi"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	if err := run(); err != nil {
		slog.Error("startup_failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	demo := env("DEMO_MODE", "true") == "true"
	addr := env("ADDR", "127.0.0.1:8090")
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if demo && host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return core.ErrInvalid
	}
	if !demo && len(os.Getenv("API_KEY")) < 24 {
		return core.ErrInvalid
	}
	dsn := env("DATABASE_URL", filepath.Join("data", "afterglow.db"))
	if os.Getenv("DATABASE_URL") == "" {
		if err = os.MkdirAll("data", 0700); err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store, err := core.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer store.DB.Close()
	if err = store.Migrate(ctx); err != nil {
		return err
	}
	tenant := env("TENANT_ID", "demo")
	if demo {
		if err = store.Seed(ctx, tenant); err != nil {
			return err
		}
	}
	var draining atomic.Bool
	api := &httpapi.Server{Store: store, Tenant: tenant, APIKey: os.Getenv("API_KEY"), Demo: demo, Ready: func() bool { return !draining.Load() }}
	srv := &http.Server{Addr: addr, Handler: api.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan error, 1)
	go func() { slog.Info("afterglow_started", "address", addr, "demo", demo); done <- srv.ListenAndServe() }()
	select {
	case err = <-done:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	draining.Store(true)
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	return srv.Shutdown(shutdown)
}
