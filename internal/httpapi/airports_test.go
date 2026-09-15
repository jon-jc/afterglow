package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAirportProxyBoundsAndCredentials(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/airports" || r.URL.Query().Get("q") != "SEA" || r.URL.Query().Has("url") || r.Header.Get("Authorization") != "" {
			t.Error("invalid upstream request", r.URL, r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"airports":[]}`))
	}))
	defer up.Close()
	r := httptest.NewRequest("GET", "/api/v1/airports?q=SEA&url=http://untrusted", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	AirportCatalog(up.URL).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	up.Close()
	w = httptest.NewRecorder()
	AirportCatalog(up.URL).ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("unavailable dependency not handled")
	}
}
