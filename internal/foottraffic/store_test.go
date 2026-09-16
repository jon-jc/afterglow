package foottraffic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
)

var testNow = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

func fixture(t *testing.T) (*Store, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = filepath.Join(t.TempDir(), "traffic.db")
	}
	db, err := core.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Close() })
	s := &Store{DB: db.DB}
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s, core.ID()
}
func ingest(t *testing.T, s *Store, tenant string, b Batch) Result {
	t.Helper()
	r, e := s.Ingest(context.Background(), tenant, b, testNow)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestConcurrentReplayCountsWindowOnce(t *testing.T) {
	s, tenant := fixture(t)
	b := Batch{ID: "same-batch", Source: Source, Windows: []Window{{Zone: "downtown", Start: testNow.Add(-time.Hour).UnixMilli(), Count: 42}}}
	var first atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Ingest(context.Background(), tenant, b, testNow)
			if e != nil {
				t.Error(e)
				return
			}
			if !r.Replayed {
				first.Add(1)
			}
		}()
	}
	wg.Wait()
	if first.Load() != 1 {
		t.Fatalf("new batches: %d", first.Load())
	}
	r, e := s.Report(context.Background(), tenant, testNow)
	if e != nil {
		t.Fatal(e)
	}
	if r.Published != 42 || r.Released != 1 {
		t.Fatalf("double counted: %+v", r)
	}
}
func TestConflictsRollbackWholeBatch(t *testing.T) {
	s, tenant := fixture(t)
	w := Window{Zone: "downtown", Start: testNow.Add(-time.Hour).UnixMilli(), Count: 42}
	b := Batch{ID: "original", Source: Source, Windows: []Window{w}}
	ingest(t, s, tenant, b)
	b.Windows[0].Count = 43
	if _, e := s.Ingest(context.Background(), tenant, b, testNow); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	b.ID = "conflict"
	b.Windows = append([]Window{{Zone: "retail", Start: testNow.Add(-2 * time.Hour).UnixMilli(), Count: 55}}, b.Windows...)
	if _, e := s.Ingest(context.Background(), tenant, b, testNow); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	var count int
	if e := s.DB.QueryRow(`SELECT COUNT(*) FROM traffic_windows_v1 WHERE tenant=$1`, tenant).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if count != 1 {
		t.Fatalf("partial write: %d", count)
	}
	b.Windows = []Window{w}
	result := ingest(t, s, tenant, b)
	if result.Inserted != 0 || result.Replayed {
		t.Fatalf("rollback did not release batch identity: %+v", result)
	}
}
func TestCanonicalOrderAndOverlappingExamples(t *testing.T) {
	s, tenant := fixture(t)
	b := Example(testNow)
	first := ingest(t, s, tenant, b)
	if first.Inserted != 48 {
		t.Fatal(first)
	}
	for i, j := 0, len(b.Windows)-1; i < j; i, j = i+1, j-1 {
		b.Windows[i], b.Windows[j] = b.Windows[j], b.Windows[i]
	}
	if !ingest(t, s, tenant, b).Replayed {
		t.Fatal("order-dependent identity")
	}
	later := testNow.Add(time.Hour)
	result, e := s.Ingest(context.Background(), tenant, Example(later), later)
	if e != nil {
		t.Fatal(e)
	}
	if result.Inserted != 8 {
		t.Fatalf("overlap changed or counted twice: %+v", result)
	}
}
func TestReportSuppressesCountsAndUsesOnlyComparablePairs(t *testing.T) {
	s, tenant := fixture(t)
	a := testNow.Add(-time.Hour).UnixMilli()
	hour := int64(time.Hour / time.Millisecond)
	ingest(t, s, tenant, Batch{ID: "privacy", Source: Source, Windows: []Window{{"downtown", a, 19}, {"retail", a, 50}, {"retail", a - 24*hour, 25}, {"transit", a, 20}, {"transit", a - 24*hour, 19}}})
	r, e := s.Report(context.Background(), tenant, testNow)
	if e != nil {
		t.Fatal(e)
	}
	if r.Published != 70 || r.Suppressed != 1 || r.Released != 2 || r.Missing != 21 {
		t.Fatalf("wrong publication counts: %+v", r)
	}
	if r.Zones[0].Current[5].Count != nil || r.Zones[0].Current[5].Status != "suppressed" {
		t.Fatal("small count exposed")
	}
	if r.Zones[1].Change != nil {
		t.Fatal("suppressed baseline used in comparison")
	}
	if r.Zones[2].Comparable != 1 || r.Zones[2].Change == nil || *r.Zones[2].Change != 100 {
		t.Fatal("wrong paired comparison")
	}
	encoded, _ := json.Marshal(r)
	if bytes.Contains(encoded, []byte(`"observations":19`)) {
		t.Fatal("suppressed number serialized")
	}
	other, e := s.Report(context.Background(), "other-tenant", testNow)
	if e != nil || other.Published != 0 || other.Missing != 24 {
		t.Fatal("tenant isolation", e)
	}
}
func TestInvalidWindowsCannotEnterStore(t *testing.T) {
	s, tenant := fixture(t)
	for _, w := range []Window{{"unknown", testNow.Add(-time.Hour).UnixMilli(), 40}, {"retail", testNow.UnixMilli(), 40}, {"retail", testNow.Add(-8 * 24 * time.Hour).UnixMilli(), 40}, {"retail", testNow.Add(-time.Hour).UnixMilli() + 1, 40}, {"retail", testNow.Add(-time.Hour).UnixMilli(), -1}} {
		if _, e := s.Ingest(context.Background(), tenant, Batch{ID: core.ID(), Source: Source, Windows: []Window{w}}, testNow); !errors.Is(e, ErrInvalid) {
			t.Fatal(w, e)
		}
	}
}
func TestHTTPRejectsIdentifiersAndReturnsCommittedResults(t *testing.T) {
	s, tenant := fixture(t)
	h := s.Handler(tenant)
	post := func(body string, media string) int {
		r := httptest.NewRequest("POST", "/api/v1/foot-traffic/batches", strings.NewReader(body))
		r.Header.Set("Content-Type", media)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	b := Example(time.Now())
	encoded, _ := json.Marshal(b)
	if got := post(string(encoded), "application/json"); got != 201 {
		t.Fatal(got)
	}
	if got := post(string(encoded), "application/json"); got != 200 {
		t.Fatal(got)
	}
	for _, extra := range []string{`"device_id":"phone",`, `"latitude":47.61,`, `"longitude":-122.33,`} {
		body := "{" + extra + string(encoded[1:])
		if got := post(body, "application/json"); got != 400 {
			t.Fatal(got)
		}
	}
	if got := post(string(encoded)+`{}`, "application/json"); got != 400 {
		t.Fatal(got)
	}
	if got := post(string(encoded), "text/plain"); got != 415 {
		t.Fatal(got)
	}
	if got := post(strings.Repeat(" ", 40000), "application/json"); got != 413 {
		t.Fatal(got)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/foot-traffic/report", nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code)
	}
}
