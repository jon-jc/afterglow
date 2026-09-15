package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jon-jc/afterglow/internal/airports"
)

func main() {
	if err := run(); err != nil {
		slog.Error("airport_service_failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	addr := os.Getenv("AIRPORT_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8091"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	// This internal read service is intentionally loopback-only. A cloud deployment
	// needs authenticated service-to-service ingress rather than exposing this port.
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return &net.AddrError{Err: "airport service requires loopback", Addr: addr}
	}
	cache := os.Getenv("AIRPORT_CACHE")
	if cache == "" {
		cache = "data/airports.json"
	}
	c, err := airports.New(cache)
	if err != nil {
		return err
	}
	go func() {
		for {
			delay := 24 * time.Hour
			if e := c.Refresh(ctx); e != nil {
				slog.Warn("airport_refresh_failed", "error", e)
				delay = 5 * time.Minute
			} else {
				slog.Info("airport_refresh_completed")
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	srv := &http.Server{Addr: addr, Handler: c.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan error, 1)
	go func() { slog.Info("airport_service_started", "address", addr); done <- srv.ListenAndServe() }()
	select {
	case err = <-done:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	return srv.Shutdown(shutdown)
}
