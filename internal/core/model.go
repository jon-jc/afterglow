package core

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrConflict = errors.New("idempotency key reused with different content")
	ErrBudget   = errors.New("insufficient uncommitted campaign budget")
	ErrNotFound = errors.New("resource not found")
	ErrInvalid  = errors.New("invalid request")
)

type Campaign struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Budget   int64  `json:"budget_micros"`
	Reserved int64  `json:"reserved_micros"`
	Spent    int64  `json:"spent_micros"`
}
type Screen struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Market string  `json:"market"`
	Format string  `json:"format"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
}
type ReservationInput struct {
	CampaignID string `json:"campaign_id"`
	ScreenID   string `json:"screen_id"`
	Cost       int64  `json:"cost_micros"`
}
type Reservation struct {
	ID         string `json:"id"`
	CampaignID string `json:"campaign_id"`
	ScreenID   string `json:"screen_id"`
	Cost       int64  `json:"cost_micros"`
	State      string `json:"state"`
	Created    int64  `json:"created_at"`
	Expires    int64  `json:"expires_at"`
}

// Receipt is a deliberately small, versioned synthetic partner contract.
// It is not Vistar's wire format and makes no claims about viewer impressions.
type Receipt struct {
	Version       int    `json:"schema_version"`
	EventID       string `json:"event_id"`
	ReservationID string `json:"reservation_id"`
	ScreenID      string `json:"screen_id"`
	PlayedAt      int64  `json:"played_at"`
	DurationMS    int    `json:"duration_ms"`
}
type Delivery struct {
	ID string `json:"id"`
	Receipt
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Created  int64  `json:"received_at"`
	Attempts int    `json:"attempts"`
}
type Audit struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Resource string `json:"resource_id"`
	Detail   string `json:"detail"`
	Created  int64  `json:"created_at"`
}
type Snapshot struct {
	Campaigns       []Campaign       `json:"campaigns"`
	Screens         []Screen         `json:"screens"`
	Reservations    []Reservation    `json:"reservations"`
	Deliveries      []Delivery       `json:"deliveries"`
	Audit           []Audit          `json:"audit"`
	Counts          map[string]int64 `json:"counts"`
	OldestPendingMS int64            `json:"oldest_pending_ms"`
	Now             int64            `json:"now"`
}

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func validID(s string) bool {
	if len(s) < 1 || len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
func invalid(s string) error { return fmt.Errorf("%w: %s", ErrInvalid, s) }
func millis() int64          { return time.Now().UnixMilli() }
