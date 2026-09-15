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

	"github.com/jon-jc/afterglow/internal/console"
	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/httpapi"
	"github.com/jon-jc/afterglow/internal/pipeline"
	"github.com/jon-jc/afterglow/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	closeTelemetry, err := telemetry.Start(ctx)
	if err != nil {
		return err
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = closeTelemetry(c)
	}()
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
	api.Static = console.Handler()
	reg := prometheus.NewRegistry()
	worker := pipeline.New(store, reg)
	transport := env("TRANSPORT", "local")
	role := env("ROLE", "all")
	if role != "all" && role != "api" && role != "worker" {
		return core.ErrInvalid
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	workerDone := make(chan struct{})
	receiverDone := make(chan struct{})
	var bus *pipeline.PubSub
	if transport == "pubsub" {
		bus, err = pipeline.NewPubSub(ctx, os.Getenv("GCP_PROJECT_ID"), env("PUBSUB_TOPIC", "afterglow-receipts"), env("PUBSUB_SUBSCRIPTION", "afterglow-reconciler"))
		if err != nil {
			return err
		}
		defer bus.Close()
		worker.Publisher = bus
	} else if transport != "local" {
		return core.ErrInvalid
	}
	if role != "api" {
		go func() { defer close(workerDone); worker.Run(workerCtx) }()
		if bus != nil {
			go func() {
				defer close(receiverDone)
				if e := bus.Receive(workerCtx, worker); e != nil && workerCtx.Err() == nil {
					slog.Error("subscriber_failed", "error", e)
					cancel()
				}
			}()
		} else {
			close(receiverDone)
		}
	} else {
		close(workerDone)
		close(receiverDone)
	}
	api.Metrics = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	api.DemoHandler = httpapi.Demo(store, worker, tenant)
	api.Runtime = func() any {
		return map[string]any{"transport": transport, "storage": map[bool]string{true: "PostgreSQL", false: "SQLite WAL"}[store.Postgres], "demo": demo, "paused": worker.Paused.Load(), "breaker_open": worker.BreakerOpen(), "role": role}
	}
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
	// Keep serving briefly while upstream readiness checks remove this instance.
	if !demo {
		time.Sleep(2 * time.Second)
	}
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	err = srv.Shutdown(shutdown)
	stopWorker()
	<-workerDone
	<-receiverDone
	return err
}
