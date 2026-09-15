package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestCredentialRotationWindow(t *testing.T) {
	current, previous := "current-key-test-at-least-24-chars", "previous-key-test-at-least-24-chars"
	for _, old := range []string{previous, ""} {
		h := (&Server{APIKey: current, PreviousAPIKey: old}).Handler()
		for _, key := range []string{current, previous, "wrong", ""} {
			r := httptest.NewRequest("GET", "/api/v1/runtime", nil)
			if key != "" {
				r.Header.Set("Authorization", "Bearer "+key)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			want := 401
			if key == current || (old != "" && key == old) {
				want = 200
			}
			if w.Code != want {
				t.Fatalf("rotation status got %d want %d", w.Code, want)
			}
		}
	}
}
