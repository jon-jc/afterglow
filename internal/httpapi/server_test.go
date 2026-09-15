package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jon-jc/afterglow/internal/core"
)

func handler(t *testing.T) http.Handler {
	t.Helper()
	s, e := core.Open(context.Background(), filepath.Join(t.TempDir(), "http.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.DB.Close() })
	if e = s.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = s.Seed(context.Background(), "demo"); e != nil {
		t.Fatal(e)
	}
	return (&Server{Store: s, Tenant: "demo", APIKey: "test-secret"}).Handler()
}
func TestBoundaryValidation(t *testing.T) {
	h := handler(t)
	for _, tc := range []struct {
		name, method, path, body, key string
		status                        int
	}{{"auth", "GET", "/api/v1/snapshot", "", "", 401}, {"snapshot", "GET", "/api/v1/snapshot", "", "test-secret", 200}, {"unknown_field", "POST", "/api/v1/reservations", `{"campaign_id":"cmp-cascade","screen_id":"sea-01","cost_micros":1,"tenant":"other"}`, "test-secret", 400}, {"missing_key", "POST", "/api/v1/reservations", `{"campaign_id":"cmp-cascade","screen_id":"sea-01","cost_micros":1}`, "test-secret", 422}, {"trailing_json", "POST", "/api/v1/receipts", `{} {}`, "test-secret", 400}, {"demo_disabled", "POST", "/api/demo/scenario", "{}", "test-secret", 404}, {"negative_cost", "POST", "/api/v1/reservations", `{"campaign_id":"cmp-cascade","screen_id":"sea-01","cost_micros":-1}`, "test-secret", 422}} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			if tc.key != "" {
				r.Header.Set("Authorization", "Bearer "+tc.key)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestCrossOriginMutationRejected(t *testing.T) {
	h := handler(t)
	r := httptest.NewRequest("POST", "http://localhost/api/v1/receipts", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
