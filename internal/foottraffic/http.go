package foottraffic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"
)

func (s *Store) Handler(tenant string) http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	fail := func(w http.ResponseWriter, e error) {
		code := 503
		detail := "measurement service temporarily unavailable"
		if errors.Is(e, ErrInvalid) {
			code = 422
			detail = "Use synthetic-partner-v1, known zones and 1–96 distinct, completed UTC-hour windows from the last seven days; counts must be between 0 and 1000000."
		}
		if errors.Is(e, ErrConflict) {
			code = 409
			detail = ErrConflict.Error()
		}
		write(w, code, map[string]any{"status": code, "detail": detail})
	}
	// Fixed synthetic scenarios are separate views of one authorized tenant.
	// Reject invalid selection before any database operation.
	scenarioFor := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		values, present := r.URL.Query()["scenario"]
		if !present {
			return "baseline", true
		}
		if len(values) != 1 || !validScenario(values[0]) {
			write(w, 400, map[string]string{"detail": "Choose baseline, busy, gaps or quiet as the sample scenario."})
			return "", false
		}
		return values[0], true
	}
	mux.HandleFunc("GET /api/v1/foot-traffic/report", func(w http.ResponseWriter, r *http.Request) {
		scenario, ok := scenarioFor(w, r)
		if !ok {
			return
		}
		v, e := s.Report(r.Context(), scenarioTenant(tenant, scenario), time.Now())
		if e != nil {
			fail(w, e)
			return
		}
		write(w, 200, v)
	})
	mux.HandleFunc("GET /api/v1/foot-traffic/example", func(w http.ResponseWriter, r *http.Request) {
		scenario, ok := scenarioFor(w, r)
		if !ok {
			return
		}
		b, _ := ExampleForScenario(time.Now(), scenario)
		write(w, 200, b)
	})
	mux.HandleFunc("POST /api/v1/foot-traffic/batches", func(w http.ResponseWriter, r *http.Request) {
		scenario, ok := scenarioFor(w, r)
		if !ok {
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" {
			write(w, 415, map[string]string{"detail": "application/json required"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32768)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var b Batch
		if err = dec.Decode(&b); err != nil {
			var large *http.MaxBytesError
			code := 400
			if errors.As(err, &large) {
				code = 413
			}
			write(w, code, map[string]string{"detail": "Invalid aggregate JSON; unknown fields, including device identifiers and coordinates, are rejected."})
			return
		}
		if err = dec.Decode(new(any)); err != io.EOF {
			write(w, 400, map[string]string{"detail": "One JSON object required"})
			return
		}
		v, e := s.Ingest(r.Context(), scenarioTenant(tenant, scenario), b, time.Now())
		if e != nil {
			fail(w, e)
			return
		}
		code := 201
		if v.Replayed {
			code = 200
		}
		write(w, code, v)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
