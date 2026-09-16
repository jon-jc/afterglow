package foottraffic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEmptySampleClearsReplayHistoryAndKeepsOtherSamples(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	b := Example(testNow)
	selected := scenarioTenant(tenant, "busy")
	ingest(t, s, selected, b)
	ingest(t, s, tenant, b)
	ingest(t, s, scenarioTenant(tenant+"-other", "busy"), b)
	removed, err := s.EmptySample(ctx, selected)
	if err != nil || removed.Batches != 1 || removed.Windows != 48 {
		t.Fatal(removed, err)
	}
	r, err := s.Report(ctx, selected, testNow)
	if err != nil || r.Missing != 24 || r.Published != 0 || r.Suppressed != 0 {
		t.Fatal("not empty", r, err)
	}
	for _, z := range r.Zones {
		for _, c := range append(z.Current, z.Previous...) {
			if c.Status != "missing" || c.Count != nil {
				t.Fatal("a current or previous window survived emptying")
			}
		}
	}
	for _, scope := range []string{tenant, scenarioTenant(tenant+"-other", "busy")} {
		other, err := s.Report(ctx, scope, testNow)
		if err != nil || other.Missing != 0 {
			t.Fatal("cleared another sample or tenant", err)
		}
	}
	if again, err := s.EmptySample(ctx, selected); err != nil || again != (EmptyResult{}) {
		t.Fatal("repeated empty must be safe", again, err)
	}
	if got := ingest(t, s, selected, b); got.Replayed || got.Inserted != 48 {
		t.Fatal("old replay ledger blocked a fresh import", got)
	}
}

func TestConcurrentEmptyAndImportKeepWindowsAndLedgerConsistent(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	b := Example(testNow)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				_, err = s.EmptySample(ctx, tenant)
			} else {
				_, err = s.Ingest(ctx, tenant, b, testNow)
			}
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	var windows, batches int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM traffic_windows_v1 WHERE tenant=$1`, tenant).Scan(&windows); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM traffic_batches_v1 WHERE tenant=$1`, tenant).Scan(&batches); err != nil {
		t.Fatal(err)
	}
	if !((batches == 0 && windows == 0) || (batches == 1 && windows == 48)) {
		t.Fatalf("partially erased import: batches=%d windows=%d", batches, windows)
	}
}

func TestRandomGenerationReplacesAtomicallyAndReplaysSafely(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	var previous string
	for i := 0; i < 5; i++ {
		batch, err := ExampleForScenario(testNow, "random")
		if err != nil || batch.ID == previous {
			t.Fatal("generation did not receive a fresh identity", err)
		}
		result, err := s.ReplaceSample(ctx, tenant, batch, testNow)
		if err != nil || result.Inserted != len(batch.Windows) || result.Replayed {
			t.Fatal(result, err)
		}
		r, err := s.Report(ctx, tenant, testNow)
		if err != nil || r.Missing == 0 || r.Suppressed == 0 || r.Released == 0 {
			t.Fatal("random sample should include missing, hidden and released cells", r, err)
		}
		if replay, err := s.ReplaceSample(ctx, tenant, batch, testNow); err != nil || !replay.Replayed {
			t.Fatal("lost-response retry must not regenerate", replay, err)
		}
		bad := batch
		bad.Windows = append([]Window(nil), batch.Windows...)
		bad.Windows[0].Count++
		if _, err := s.ReplaceSample(ctx, tenant, bad, testNow); !errors.Is(err, ErrConflict) {
			t.Fatal("same-identity changed content was accepted", err)
		}
		bad.ID = "invalid-replacement"
		bad.Windows[0].Count = -1
		if _, err := s.ReplaceSample(ctx, tenant, bad, testNow); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid replacement accepted", err)
		}
		after, err := s.Report(ctx, tenant, testNow)
		if err != nil || !reflect.DeepEqual(r, after) {
			t.Fatal("replay or rejected replacement changed the report", err)
		}
		var count int
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM traffic_batches_v1 WHERE tenant=$1`, tenant).Scan(&count); err != nil || count != 1 {
			t.Fatal("old generation retained", count, err)
		}
		previous = batch.ID
	}
}

func TestHTTPDemoOnlyEmptyAndRandomReplacement(t *testing.T) {
	s, tenant := fixture(t)
	call := func(h http.Handler, route, body string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("POST", "/api/v1/foot-traffic/"+route, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	demo, normal := s.DemoHandler(tenant), s.Handler(tenant)
	for _, route := range []string{"empty?scenario=busy", "random-batches?scenario=random"} {
		if got := call(normal, route, "{}"); got.Code != 404 {
			t.Fatal("regular service exposes demo mutation", route, got.Code)
		}
	}
	for _, body := range []string{`{"tenant":"other"}`, `null`, `{} {}`, `[]`} {
		if got := call(demo, "empty?scenario=busy", body); got.Code != 400 {
			t.Fatal("unexpected reset body accepted", body, got.Code)
		}
	}
	if got := call(demo, "empty?scenario=unknown", "{}"); got.Code != 400 {
		t.Fatal("invalid reset sample accepted", got.Code)
	}
	batch, err := ExampleForScenario(time.Now(), "random")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(batch)
	if got := call(demo, "random-batches?scenario=baseline", string(body)); got.Code != 400 {
		t.Fatal("generator replaced a fixed sample", got.Code)
	}
	for _, status := range []int{201, 200} {
		if got := call(demo, "random-batches?scenario=random", string(body)); got.Code != status {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	if got := call(demo, "empty?scenario=random", "{}"); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := call(demo, "random-batches?scenario=random", string(body)); got.Code != 201 {
		t.Fatal("empty did not release replay identity", got.Code, got.Body.String())
	}
}
