package httpapi

import (
	"encoding/json"
	"github.com/jon-jc/afterglow/internal/core"
	"net/http/httptest"
	"testing"
)

func TestCampaignAssuranceHTTP(t *testing.T) {
	h := handler(t)
	for _, tc := range []struct {
		id, key string
		status  int
	}{{"cmp-cascade", "test-secret", 200}, {"cmp-cascade", "", 401}, {"missing", "test-secret", 404}, {"invalid.id", "test-secret", 422}} {
		r := httptest.NewRequest("GET", "/api/v1/campaigns/"+tc.id+"/assurance", nil)
		if tc.key != "" {
			r.Header.Set("Authorization", "Bearer "+tc.key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.id, w.Code, w.Body.String())
		}
		if w.Code == 200 {
			var report core.CampaignAssurance
			if err := json.Unmarshal(w.Body.Bytes(), &report); err != nil || report.Campaign.ID != tc.id || !report.BudgetConsistent {
				t.Fatalf("invalid report: %+v %v", report, err)
			}
		}
	}
}
