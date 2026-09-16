package core

import (
	"context"
	"errors"
	"testing"
)

func TestEvidencePreservesIdentityAndTenantBoundary(t *testing.T) {
	s, tenant := fixture(t)
	ctx := context.Background()
	r := hold(t, s, tenant)
	id := accept(t, s, tenant, proof(r))
	before, err := s.Evidence(ctx, tenant, id)
	if err != nil || before.Delivery.Status != "accepted" || before.Reservation == nil || before.Reservation.State != "held" || before.OutboxState != "pending" {
		t.Fatalf("before: %+v %v", before, err)
	}
	if _, err = s.Evidence(ctx, ID(), id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant: %v", err)
	}
	if _, err = s.Process(ctx, tenant, id); err != nil {
		t.Fatal(err)
	}
	// Prove targeted reads remain available outside the console's recent window.
	for i := 0; i < 105; i++ {
		accept(t, s, tenant, proof(r))
	}
	after, err := s.Evidence(ctx, tenant, id)
	if err != nil || after.Delivery.Status != "settled" || after.Reservation.State != "settled" || after.Reservation.Cost != r.Cost || after.Delivery.Attempts != 1 {
		t.Fatalf("after: %+v %v", after, err)
	}
	if _, err = s.Evidence(ctx, tenant, "../bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid id: %v", err)
	}
}

func TestEvidenceForUnknownReservation(t *testing.T) {
	s, tenant := fixture(t)
	p := Receipt{Version: 1, EventID: ID(), ReservationID: ID(), ScreenID: "sea-01", PlayedAt: millis(), DurationMS: 10000}
	id := accept(t, s, tenant, p)
	if _, err := s.Process(context.Background(), tenant, id); err != nil {
		t.Fatal(err)
	}
	e, err := s.Evidence(context.Background(), tenant, id)
	if err != nil || e.Reservation != nil || e.Delivery.Status != "quarantined" {
		t.Fatalf("evidence: %+v %v", e, err)
	}
}
