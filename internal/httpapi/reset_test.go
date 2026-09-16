package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDemoResetBoundaryAndStaleTab(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		calls := 0
		s := &Server{Demo: enabled, APIKey: "secret", DemoGeneration: "before", ResetDemo: func(context.Context) (string, error) { calls++; return "after", nil }}
		h := s.Handler()
		call := func(body, origin, generation string) *httptest.ResponseRecorder {
			r := httptest.NewRequest("POST", "/api/demo/reset", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer secret")
			r.Header.Set("Origin", origin)
			r.Header.Set("X-Demo-Generation", generation)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			return w
		}
		if !enabled {
			if w := call(`{"confirm":true}`, "", "before"); w.Code != 404 || calls != 0 {
				t.Fatal(w.Code, calls)
			}
			continue
		}
		for _, body := range []string{`{}`, `{"confirm":false}`, `null`, `{"confirm":true,"tenant":"other"}`} {
			if w := call(body, "", "before"); w.Code < 400 || calls != 0 {
				t.Fatal(w.Code, calls)
			}
		}
		if w := call(`{"confirm":true}`, "https://another-site.test", "before"); w.Code != 403 || calls != 0 {
			t.Fatal(w.Code, calls)
		}
		if w := call(`{"confirm":true}`, "", "before"); w.Code != 200 || calls != 1 || w.Header().Get("X-Demo-Generation") != "after" {
			t.Fatal(w.Code, calls)
		}
		// The old generation cannot repeat the reset or start new work. A missing
		// store would panic if this stale reservation reached its handler.
		if w := call(`{"confirm":true}`, "", "before"); w.Code != 409 || calls != 1 {
			t.Fatal(w.Code, calls)
		}
		r := httptest.NewRequest("POST", "/api/v1/reservations", strings.NewReader(`{}`))
		r.Header.Set("X-Demo-Generation", "before")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 409 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestFailedDemoResetPreservesGeneration(t *testing.T) {
	s := &Server{Demo: true, DemoGeneration: "before", ResetDemo: func(context.Context) (string, error) { return "", errors.New("database unavailable") }}
	r := httptest.NewRequest("POST", "/api/demo/reset", strings.NewReader(`{"confirm":true}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable || s.DemoGeneration != "before" {
		t.Fatal(w.Code, s.DemoGeneration)
	}
}
