package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jon-jc/afterglow/internal/core"
)

func TestEvidenceHTTPBoundary(t *testing.T) {
	ctx := context.Background()
	s, err := core.Open(ctx, filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Seed(ctx, "owner"); err != nil {
		t.Fatal(err)
	}
	r, _, err := s.Reserve(ctx, "owner", core.ID(), core.ReservationInput{CampaignID: "cmp-cascade", ScreenID: "sea-01", Cost: 10000})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.Accept(ctx, "owner", core.Receipt{Version: 1, EventID: core.ID(), ReservationID: r.ID, ScreenID: r.ScreenID, PlayedAt: r.Created + 1, DurationMS: 10000})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, tenant, key, id string
		status                int
	}{
		{"authorized", "owner", "secret", id, 200},
		{"no credential", "owner", "", id, 401},
		{"other tenant", "other", "secret", id, 404},
		{"missing", "owner", "secret", core.ID(), 404},
		{"invalid", "owner", "secret", "bad.id", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := (&Server{Store: s, Tenant: tc.tenant, APIKey: "secret"}).Handler()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/deliveries/"+tc.id, nil)
			if tc.key != "" {
				req.Header.Set("Authorization", "Bearer "+tc.key)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Code == 200 {
				var e core.DeliveryEvidence
				if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e.Delivery.ID != id || e.Reservation == nil || e.Reservation.ID != r.ID {
					t.Fatalf("wrong evidence: %+v %v", e, err)
				}
			}
		})
	}
}
