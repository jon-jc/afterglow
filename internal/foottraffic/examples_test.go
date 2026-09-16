package foottraffic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSampleScenariosKeepIndependentImmutableWindows(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	for _, scenario := range []string{"baseline", "busy", "gaps", "quiet"} {
		scope := scenarioTenant(tenant, scenario)
		before, err := s.Report(ctx, scope, testNow)
		if err != nil || before.Missing != 24 {
			t.Fatalf("scenario %s leaked another sample: %+v, %v", scenario, before, err)
		}
		batch, err := ExampleForScenario(testNow, scenario)
		if err != nil {
			t.Fatal(err)
		}
		first := ingest(t, s, scope, batch)
		if first.Inserted != len(batch.Windows) {
			t.Fatal("did not insert full sample", scenario, first)
		}
		if !ingest(t, s, scope, batch).Replayed {
			t.Fatal("sample replay not recognized", scenario)
		}
		r, err := s.Report(ctx, scope, testNow)
		if err != nil {
			t.Fatal(err)
		}
		switch scenario {
		case "baseline":
			if r.Missing != 0 || !reflect.DeepEqual(batch, Example(testNow)) {
				t.Fatal("existing default changed")
			}
		case "busy":
			if r.Released != 24 || r.Suppressed != 0 || r.Published < 24*600 {
				t.Fatal("high-activity sample is not fully reportable", r)
			}
		case "gaps":
			if r.Missing != 9 || r.Zones[1].Change != nil {
				t.Fatal("missing coverage became a comparison or zero", r)
			}
		case "quiet":
			if r.Suppressed != 18 || r.Released != 6 || r.Missing != 0 {
				t.Fatal("low counts were not suppressed", r)
			}
		}
		// Advancing the report period must not change immutable overlapping hours.
		later := testNow.Add(time.Hour)
		next, _ := ExampleForScenario(later, scenario)
		if _, err = s.Ingest(ctx, scope, next, later); err != nil {
			t.Fatalf("overlapping %s feed conflicts: %v", scenario, err)
		}
	}
	other, err := s.Report(ctx, scenarioTenant(tenant+"-other", "busy"), testNow)
	if err != nil || other.Missing != 24 {
		t.Fatal("scenario selection crossed tenant boundary", err)
	}
}

func TestHTTPScenarioSelectionAndRefreshAreConsistent(t *testing.T) {
	s, tenant := fixture(t)
	handler := s.Handler(tenant)
	call := func(method, route string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/v1/foot-traffic/"+route, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, scenario := range []string{"baseline", "busy", "gaps", "quiet"} {
		query := "?scenario=" + scenario
		feed := call("GET", "example"+query, nil)
		if feed.Code != 200 {
			t.Fatal(feed.Code, feed.Body.String())
		}
		for i, status := range []int{201, 200} {
			got := call("POST", "batches"+query, feed.Body.Bytes())
			if got.Code != status {
				t.Fatalf("%s attempt %d: %d %s", scenario, i, got.Code, got.Body)
			}
		}
		var previous Report
		for i := 0; i < 2; i++ {
			got := call("GET", "report"+query, nil)
			if got.Code != 200 || got.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(got.Code, got.Body.String())
			}
			var report Report
			if err := json.Unmarshal(got.Body.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if i > 0 && report.WindowEnd == previous.WindowEnd && !reflect.DeepEqual(report.Zones, previous.Zones) {
				t.Fatal("refresh changed saved observations")
			}
			previous = report
		}
	}
	for _, query := range []string{"?scenario=unknown", "?scenario=", "?scenario=busy&scenario=quiet"} {
		for _, endpoint := range []string{"example", "report", "batches"} {
			method := "GET"
			if endpoint == "batches" {
				method = "POST"
			}
			if got := call(method, endpoint+query, nil); got.Code != 400 {
				t.Fatal("invalid sample did not fail closed", endpoint, query, got.Code)
			}
		}
	}
	if !identity.MatchString(scenarioTenant(strings.Repeat("x", 100), "busy")) {
		t.Fatal("long authorized tenant creates an invalid sample identity")
	}
}
