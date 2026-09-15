package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

type Publisher interface {
	Publish(context.Context, *core.Job) error
}
type Worker struct {
	Store     *core.Store
	Publisher Publisher
	Paused    atomic.Bool
	FailNext  atomic.Int32
	DropAck   atomic.Bool
	outcomes  *prometheus.CounterVec
	latency   prometheus.Histogram
	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

func New(s *core.Store, reg *prometheus.Registry) *Worker {
	w := &Worker{Store: s}
	w.outcomes = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "afterglow_delivery_total", Help: "Delivery attempts by bounded outcome"}, []string{"outcome"})
	w.latency = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "afterglow_processing_seconds", Help: "Reconciliation transaction duration", Buckets: prometheus.DefBuckets})
	reg.MustRegister(w.outcomes, w.latency)
	return w
}
func (w *Worker) BreakerOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Now().Before(w.openUntil)
}
func (w *Worker) markFailure() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failures++
	if w.failures >= 3 {
		w.openUntil = time.Now().Add(2 * time.Second)
	}
}
func (w *Worker) markSuccess() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failures = 0
	w.openUntil = time.Time{}
}
func Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 6 {
		attempt = 6
	}
	base := time.Duration(1<<uint(attempt-1)) * 250 * time.Millisecond
	return base + time.Duration(rand.Int64N(int64(base/2)+1))
}
func (w *Worker) consumeFault() bool {
	for {
		n := w.FailNext.Load()
		if n <= 0 {
			return false
		}
		if w.FailNext.CompareAndSwap(n, n-1) {
			return true
		}
	}
}
func (w *Worker) Consume(ctx context.Context, tenant, id string) error {
	start := time.Now()
	defer func() { w.latency.Observe(time.Since(start).Seconds()) }()
	ctx, span := otel.Tracer("afterglow").Start(ctx, "reconcile.playback")
	defer span.End()
	span.SetAttributes(attribute.String("delivery.id", id))
	if w.consumeFault() {
		w.outcomes.WithLabelValues("injected_failure").Inc()
		return errors.New("demo transient storage failure")
	}
	status, e := w.Store.Process(ctx, tenant, id)
	if e != nil {
		span.RecordError(e)
		w.outcomes.WithLabelValues("retry").Inc()
		return e
	}
	if w.DropAck.CompareAndSwap(true, false) {
		w.outcomes.WithLabelValues("ack_lost").Inc()
		return errors.New("demo crash after commit before acknowledgement")
	}
	if e = w.Store.Complete(ctx, tenant, id); e != nil {
		return e
	}
	w.outcomes.WithLabelValues(status).Inc()
	return nil
}
func (w *Worker) Tick(ctx context.Context) error {
	if w.Paused.Load() || w.BreakerOpen() {
		return nil
	}
	j, e := w.Store.Claim(ctx, time.Now().UnixMilli())
	if e != nil {
		return e
	}
	if j == nil {
		return nil
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(j.Trace))
	ctx, span := otel.Tracer("afterglow").Start(ctx, "outbox.dispatch")
	defer span.End()
	op, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if w.Publisher != nil {
		e = w.Publisher.Publish(op, j)
		if e == nil {
			e = w.Store.Dispatched(op, j)
		}
	} else {
		e = w.Consume(op, j.Tenant, j.ID)
	}
	if e == nil {
		w.markSuccess()
		return nil
	}
	span.RecordError(e)
	w.markFailure()
	w.outcomes.WithLabelValues("dispatch_failed").Inc()
	if err := w.Store.FailJob(ctx, j, time.Now().Add(Backoff(j.Attempts)).UnixMilli()); err != nil {
		return err
	}
	return e
}
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(75 * time.Millisecond)
	defer ticker.Stop()
	sweep := time.NewTicker(30 * time.Second)
	defer sweep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e := w.Tick(ctx); e != nil {
				slog.Warn("dispatch_retry", "error", e)
			}
		case <-sweep.C:
			if e := w.Store.ReleaseExpired(ctx, time.Now().UnixMilli()); e != nil {
				slog.Warn("expiry_sweep_failed", "error", e)
			}
		}
	}
}
