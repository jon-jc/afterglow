package core

import (
	"context"
	"errors"
	"testing"
)

func TestAssuranceFullHistoryWithoutMultiplyingSpend(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	r := hold(t, s, tenant)
	for i := 0; i < 105; i++ {
		id := accept(t, s, tenant, proof(r))
		if _, err := s.Process(ctx, tenant, id); err != nil {
			t.Fatal(err)
		}
	}
	report, err := s.Assurance(ctx, tenant, r.CampaignID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.BudgetConsistent || report.SettledPlays != 1 || report.Campaign.Spent != r.Cost || report.Decisions["duplicate"] != 104 || report.Screens[0].SettledMicros != r.Cost {
		t.Fatalf("bad aggregate: %+v", report)
	}
	if _, err = s.Assurance(ctx, ID(), r.CampaignID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("tenant isolation: %v", err)
	}
	if _, err = s.DB.ExecContext(ctx, `UPDATE campaigns SET spent=spent+1 WHERE tenant=$1 AND id=$2`, tenant, r.CampaignID); err != nil {
		t.Fatal(err)
	}
	report, err = s.Assurance(ctx, tenant, r.CampaignID)
	if err != nil || report.BudgetConsistent {
		t.Fatalf("drift must be detected: %+v %v", report, err)
	}
}
func TestAssuranceUnplayedAndUnknownEvidence(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	r := hold(t, s, tenant)
	if _, err := s.DB.ExecContext(ctx, `UPDATE reservations SET expires=$1 WHERE tenant=$2 AND id=$3`, millis()-1, tenant, r.ID); err != nil {
		t.Fatal(err)
	}
	p := proof(r)
	p.ReservationID = ID()
	id := accept(t, s, tenant, p)
	if _, err := s.Process(ctx, tenant, id); err != nil {
		t.Fatal(err)
	}
	report, err := s.Assurance(ctx, tenant, r.CampaignID)
	if err != nil || !report.BudgetConsistent || report.HeldPlays != 1 || report.AwaitingEvidence != 1 || len(report.Decisions) != 0 {
		t.Fatalf("unexpected report: %+v %v", report, err)
	}
	empty, err := s.Assurance(ctx, tenant, "cmp-northstar")
	if err != nil || len(empty.Screens) != 0 || !empty.BudgetConsistent {
		t.Fatalf("empty: %+v %v", empty, err)
	}
}
