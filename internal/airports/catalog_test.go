package airports

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const header = "ident,iata_code,name,municipality,iso_region,iso_country,type,latitude_deg,longitude_deg,scheduled_service\n"

func TestScopeAndCoordinates(t *testing.T) {
	data := header + "KSEA,SEA,Seattle Tacoma,Seattle,US-WA,US,large_airport,47.45,-122.31,yes\n" +
		"TJSJ,SJU,San Juan,San Juan,PR-U-A,PR,large_airport,18.4,-66.0,yes\n" +
		"CYVR,YVR,Vancouver,Vancouver,CA-BC,CA,large_airport,49,-123,yes\n" +
		"PRIV,,Private,Small,US-WA,US,small_airport,48,-123,no\n" +
		"OLD,,Closed,Old,US-WA,US,closed_airport,48,-123,yes\n"
	a, err := parse(strings.NewReader(data))
	if err != nil || len(a) != 2 {
		t.Fatalf("scope: %+v %v", a, err)
	}
	if _, err = parse(strings.NewReader(strings.Replace(data, "47.45", "NaN", 1))); err == nil {
		t.Fatal("accepted NaN coordinates")
	}
	if _, err = parse(strings.NewReader(data + "KSEA,SEA,Duplicate,Seattle,US-WA,US,large_airport,47,-122,yes\n")); err == nil {
		t.Fatal("accepted duplicate airport")
	}
}
func TestRefreshRetainsLastGoodSnapshot(t *testing.T) {
	var failed atomic.Bool
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failed.Load() {
			fmt.Fprint(w, "invalid,csv\n")
			return
		}
		fmt.Fprint(w, header)
		for i := 0; i < 110; i++ {
			fmt.Fprintf(w, "K%03d,,Airport %d,Seattle,US-WA,US,small_airport,47,-122,yes\n", i, i)
		}
	}))
	defer up.Close()
	path := filepath.Join(t.TempDir(), "cache.json")
	c, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Source = up.URL
	if err = c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, _ := c.read()
	failed.Store(true)
	if err = c.Refresh(context.Background()); err == nil {
		t.Fatal("malformed refresh succeeded")
	}
	a, warning := c.read()
	if len(a.Airports) != 110 || a.SHA256 != before.SHA256 || warning == "" {
		t.Fatal("failed refresh replaced last good data")
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := reopened.read()
	if saved.SHA256 != before.SHA256 {
		t.Fatal("disk snapshot changed")
	}
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/airports?q=Airport+10&limit=2", nil))
	var result struct {
		Airports []Airport `json:"airports"`
		Matched  int       `json:"matched"`
		Stale    bool      `json:"stale"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(result.Airports) != 2 || result.Matched != 11 || !result.Stale {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, query := range []string{"limit=201", "offset=-1", "limit=abc"} {
		w = httptest.NewRecorder()
		c.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/airports?"+query, nil))
		if w.Code != 400 {
			t.Fatal("invalid pagination accepted")
		}
	}
	failed.Store(false)
	if err = c.Refresh(context.Background()); err != nil {
		t.Fatal("refresh could not replace cache:", err)
	}
	if _, warning = c.read(); warning != "" {
		t.Fatal("recovery did not clear stale warning")
	}
}
func TestEmptyCatalogUnavailable(t *testing.T) {
	c, _ := New(filepath.Join(t.TempDir(), "missing.json"))
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/airports", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
