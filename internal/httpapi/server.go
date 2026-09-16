package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type Server struct {
	Store          *core.Store
	Tenant         string
	APIKey         string
	PreviousAPIKey string
	Demo           bool
	Static         http.Handler
	DemoHandler    http.Handler
	Metrics        http.Handler
	Airports       http.Handler
	FootTraffic    http.Handler
	Runtime        func() any
	Ready          func() bool
	ResetDemo      func(context.Context) (string, error)
	DemoGeneration string
	demoGate       sync.RWMutex
	mu             sync.Mutex
	tokens         float64
	last           time.Time
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/demo/reset", func(w http.ResponseWriter, r *http.Request) {
		if !s.Demo || s.ResetDemo == nil {
			problem(w, 404, "not_found")
			return
		}
		var in struct {
			Confirm bool `json:"confirm"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !in.Confirm {
			problem(w, 422, "confirm_demo_data_reset")
			return
		}
		generation, err := s.ResetDemo(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		s.DemoGeneration = generation
		w.Header().Set("X-Demo-Generation", generation)
		respond(w, 200, map[string]any{"cleared": true, "generation": generation})
	})
	mux.HandleFunc("/api/v1/foot-traffic/", func(w http.ResponseWriter, r *http.Request) {
		if s.FootTraffic == nil {
			problem(w, 503, "foot_traffic_service_not_configured")
			return
		}
		s.FootTraffic.ServeHTTP(w, r)
	})
	mux.HandleFunc("GET /api/v1/airports", func(w http.ResponseWriter, r *http.Request) {
		if s.Airports == nil {
			problem(w, 503, "airport_service_not_configured")
			return
		}
		s.Airports.ServeHTTP(w, r)
	})
	mux.HandleFunc("POST /api/v1/receipts/batch", s.acceptBatch)
	mux.HandleFunc("GET /api/v1/campaigns/{id}/assurance", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Store.Assurance(r.Context(), s.Tenant, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, v)
	})
	mux.HandleFunc("GET /api/v1/deliveries/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		v, err := s.Store.Evidence(ctx, s.Tenant, r.PathValue("id"))
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, v)
	})
	mux.HandleFunc("GET /api/v1/runtime", func(w http.ResponseWriter, r *http.Request) {
		if s.Runtime != nil {
			respond(w, 200, s.Runtime())
			return
		}
		respond(w, 200, map[string]string{"transport": "local"})
	})
	mux.HandleFunc("GET /api/v1/metrics", func(w http.ResponseWriter, r *http.Request) {
		if s.Metrics != nil {
			s.Metrics.ServeHTTP(w, r)
			return
		}
		problem(w, 404, "not_found")
	})
	mux.HandleFunc("POST /api/v1/deliveries/{id}/replay", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.Replay(r.Context(), s.Tenant, r.PathValue("id")); err != nil {
			fail(w, err)
			return
		}
		respond(w, 202, map[string]string{"status": "requeued"})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "alive"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if (s.Ready != nil && !s.Ready()) || s.Store.DB.PingContext(ctx) != nil {
			problem(w, 503, "not_ready")
			return
		}
		respond(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /api/v1/snapshot", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Store.Snapshot(r.Context(), s.Tenant)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 200, v)
	})
	mux.HandleFunc("POST /api/v1/reservations", func(w http.ResponseWriter, r *http.Request) {
		var in core.ReservationInput
		if !decode(w, r, &in) {
			return
		}
		v, replay, err := s.Store.Reserve(r.Context(), s.Tenant, r.Header.Get("Idempotency-Key"), in)
		if err != nil {
			fail(w, err)
			return
		}
		code := 201
		if replay {
			code = 200
			w.Header().Set("Idempotent-Replayed", "true")
		}
		respond(w, code, v)
	})
	mux.HandleFunc("POST /api/v1/receipts", func(w http.ResponseWriter, r *http.Request) {
		var in core.Receipt
		if !decode(w, r, &in) {
			return
		}
		id, replay, err := s.Store.Accept(r.Context(), s.Tenant, in)
		if err != nil {
			fail(w, err)
			return
		}
		respond(w, 202, map[string]any{"id": id, "replayed": replay, "durability": "database_and_outbox_committed"})
	})
	mux.HandleFunc("/api/demo/", func(w http.ResponseWriter, r *http.Request) {
		if !s.Demo || s.DemoHandler == nil {
			problem(w, 404, "not_found")
			return
		}
		s.DemoHandler.ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			problem(w, 404, "not_found")
			return
		}
		if s.Static != nil {
			s.Static.ServeHTTP(w, r)
			return
		}
		respond(w, 200, map[string]string{"name": "Afterglow", "status": "Reconciliation engine online"})
	})
	s.tokens = 60
	s.last = time.Now()
	slots := make(chan struct{}, 64)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := core.ID()
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		defer func() {
			if v := recover(); v != nil {
				slog.Error("handler panic", "request_id", id)
				problem(w, 500, "internal_error")
			}
		}()
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.Demo {
				got := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
				want := sha256.Sum256([]byte(s.APIKey))
				previous := sha256.Sum256([]byte(s.PreviousAPIKey))
				currentMatch := subtle.ConstantTimeCompare(got[:], want[:])
				previousMatch := subtle.ConstantTimeCompare(got[:], previous[:])
				if s.APIKey == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || !(currentMatch == 1 || (len(s.PreviousAPIKey) >= 24 && previousMatch == 1)) {
					problem(w, 401, "unauthorized")
					return
				}
			}
			// No CORS. Reject browser cross-origin mutations even in local demo mode.
			if r.Method != "GET" && r.Method != "HEAD" {
				origin := r.Header.Get("Origin")
				if origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
					problem(w, 403, "cross_origin_request")
					return
				}
				if !s.allow() {
					w.Header().Set("Retry-After", "1")
					problem(w, 429, "rate_limited")
					return
				}
			}
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				w.Header().Set("Retry-After", "1")
				problem(w, 503, "overloaded")
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if s.Demo && s.ResetDemo != nil && strings.HasPrefix(r.URL.Path, "/api/") {
			// Existing requests finish before resetting. New requests see the new
			// generation; stale browser mutations cannot recreate old work.
			if r.Method == "POST" && r.URL.Path == "/api/demo/reset" {
				s.demoGate.Lock()
				defer s.demoGate.Unlock()
			} else {
				s.demoGate.RLock()
				defer s.demoGate.RUnlock()
			}
			w.Header().Set("X-Demo-Generation", s.DemoGeneration)
			if g := r.Header.Get("X-Demo-Generation"); r.Method != "GET" && r.Method != "HEAD" && g != "" && g != s.DemoGeneration {
				problem(w, 409, "demo_was_reset_reload_page")
				return
			}
		}
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("afterglow").Start(ctx, "http.request")
		defer span.End()
		mux.ServeHTTP(w, r.WithContext(ctx))
		slog.Debug("http_request", "request_id", id, "method", r.Method, "elapsed_ms", time.Since(started).Milliseconds())
	})
}
func (s *Server) allow() bool {
	return s.allowCost(1)
}
func (s *Server) allowCost(cost float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.tokens += now.Sub(s.last).Seconds() * 30
	if s.tokens > 60 {
		s.tokens = 60
	}
	s.last = now
	if s.tokens < cost {
		return false
	}
	s.tokens -= cost
	return true
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		problem(w, 415, "application_json_required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		problem(w, 400, "invalid_json")
		return false
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		problem(w, 400, "single_json_object_required")
		return false
	}
	return true
}
func respond(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, code int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(code), "status": code, "detail": detail})
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrConflict), errors.Is(err, core.ErrBudget):
		problem(w, 409, err.Error())
	case errors.Is(err, core.ErrNotFound):
		problem(w, 404, "not_found")
	case errors.Is(err, core.ErrInvalid):
		problem(w, 422, err.Error())
	default:
		slog.Error("operation_failed", "error", err)
		problem(w, 503, "temporarily_unavailable")
	}
}
