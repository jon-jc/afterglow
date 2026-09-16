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
	return s.handler(tenant, false)
}

// DemoHandler explicitly enables removal of synthetic sample observations.
// The standalone service's regular Handler never exposes this operation.
func (s *Store) DemoHandler(tenant string) http.Handler {
	return s.handler(tenant, true)
}

func (s *Store) handler(tenant string, demo bool) http.Handler {
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
			write(w, 400, map[string]string{"detail": "Choose a supported sample scenario: baseline, busy, gaps, quiet, commuter, retail, threshold, interruption or random."})
			return "", false
		}
		return values[0], true
	}
	if demo {
		mux.HandleFunc("POST /api/v1/foot-traffic/empty", func(w http.ResponseWriter, r *http.Request) {
			scenario, ok := scenarioFor(w, r)
			if !ok {
				return
			}
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				write(w, 415, map[string]string{"detail": "application/json required"})
				return
			}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			decoder.DisallowUnknownFields()
			var input *struct{}
			if err = decoder.Decode(&input); err != nil || input == nil {
				write(w, 400, map[string]string{"detail": "An empty JSON object is required"})
				return
			}
			if err = decoder.Decode(new(any)); err != io.EOF {
				write(w, 400, map[string]string{"detail": "One JSON object required"})
				return
			}
			result, err := s.EmptySample(r.Context(), scenarioTenant(tenant, scenario))
			if err != nil {
				fail(w, err)
				return
			}
			write(w, 200, result)
		})
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
		b, err := ExampleForScenario(time.Now(), scenario)
		if err != nil {
			fail(w, err)
			return
		}
		write(w, 200, b)
	})
	ingestBatch := func(replace bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			scenario, ok := scenarioFor(w, r)
			if !ok {
				return
			}
			if replace && scenario != "random" {
				write(w, 400, map[string]string{"detail": "Random generation can replace only the random sample."})
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
			var v Result
			var e error
			if replace {
				v, e = s.ReplaceSample(r.Context(), scenarioTenant(tenant, scenario), b, time.Now())
			} else {
				v, e = s.Ingest(r.Context(), scenarioTenant(tenant, scenario), b, time.Now())
			}
			if e != nil {
				fail(w, e)
				return
			}
			code := 201
			if v.Replayed {
				code = 200
			}
			write(w, code, v)
		}
	}
	mux.HandleFunc("POST /api/v1/foot-traffic/batches", ingestBatch(false))
	if demo {
		mux.HandleFunc("POST /api/v1/foot-traffic/random-batches", ingestBatch(true))
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
