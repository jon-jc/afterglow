package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func batchCall(h http.Handler, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/receipts/batch", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-secret")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestBatchPartialSuccessAndRetry(t *testing.T) {
	h := handler(t)
	valid := `{"schema_version":1,"event_id":"event-one","reservation_id":"hold-one","screen_id":"sea-01","played_at":1,"duration_ms":10000}`
	body := `{"receipts":[` + valid + `,null,{"event_id":"bad id"},` + valid + `,` + strings.Replace(valid, "10000", "9000", 1) + `,{"tenant":"injected"}]}`
	for attempt := 0; attempt < 2; attempt++ {
		w := batchCall(h, body)
		if w.Code != 207 {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
		var out batchResponse
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Accepted != 2 || out.Rejected != 4 || out.Replayed != attempt+1 {
			t.Fatalf("unexpected counts: %+v", out)
		}
		for i, code := range []int{202, 400, 422, 202, 409, 400} {
			if out.Results[i].Index != i || out.Results[i].Status != code {
				t.Fatalf("item %d: %+v", i, out.Results[i])
			}
		}
		if out.Results[0].ID != out.Results[3].ID {
			t.Fatal("replay created another delivery")
		}
	}
}

func TestBatchEnvelopeAndAdmission(t *testing.T) {
	for _, body := range []string{`{}`, `{"receipts":[]}`, `{"receipts":null}`, `{"receipts":[{}],"tenant":"other"}`, `{"receipts":[{}]} {}`} {
		w := batchCall(handler(t), body)
		if w.Code != 400 && w.Code != 422 {
			t.Fatalf("unexpected status: %d", w.Code)
		}
	}
	h := handler(t)
	items := strings.TrimSuffix(strings.Repeat(`{"event_id":"e","reservation_id":"r","screen_id":"s"},`, 50), ",")
	if w := batchCall(h, `{"receipts":[`+items+`]}`); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := batchCall(h, `{"receipts":[`+items+`]}`); w.Code != 429 {
		t.Fatalf("batch bypassed admission budget: %d", w.Code)
	}
	if w := batchCall(handler(t), `{"receipts":[`+items+`,{}]}`); w.Code != 422 {
		t.Fatalf("batch cap ignored: %d", w.Code)
	}
}
