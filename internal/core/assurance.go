package core

import (
	"context"
	"database/sql"
	"errors"
)

type ScreenAssurance struct {
	ScreenID         string           `json:"screen_id"`
	Name             string           `json:"name"`
	Market           string           `json:"market"`
	Format           string           `json:"format"`
	ReservedPlays    int64            `json:"reserved_plays"`
	SettledPlays     int64            `json:"settled_plays"`
	ReleasedPlays    int64            `json:"released_plays"`
	HeldMicros       int64            `json:"held_micros"`
	SettledMicros    int64            `json:"settled_micros"`
	AwaitingEvidence int64            `json:"awaiting_evidence"`
	Decisions        map[string]int64 `json:"decisions"`
}
type CampaignAssurance struct {
	Campaign         Campaign          `json:"campaign"`
	Screens          []ScreenAssurance `json:"screens"`
	ObservedAt       int64             `json:"observed_at"`
	Scope            string            `json:"scope"`
	BudgetConsistent bool              `json:"budget_consistent"`
	SettledPlays     int64             `json:"settled_plays"`
	HeldPlays        int64             `json:"held_plays"`
	AwaitingEvidence int64             `json:"awaiting_evidence"`
	Decisions        map[string]int64  `json:"decisions"`
}

// Assurance aggregates reservation money separately from receipt decisions:
// joining money to many receipts would multiply spend under redelivery.
func (s *Store) Assurance(ctx context.Context, tenant, campaignID string) (CampaignAssurance, error) {
	out := CampaignAssurance{ObservedAt: millis(), Screens: []ScreenAssurance{}, Decisions: map[string]int64{}, Scope: "All retained campaign reservations and linked receipts. Operational playback evidence, not audience measurement. Unknown-reservation receipts cannot be attributed to a campaign."}
	if !validID(campaignID) {
		return out, invalid("invalid_campaign_id")
	}
	opts := &sql.TxOptions{ReadOnly: true}
	if s.Postgres {
		opts.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.DB.BeginTx(ctx, opts)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	c := &out.Campaign
	err = tx.QueryRowContext(ctx, `SELECT id,name,budget,reserved,spent FROM campaigns WHERE tenant=$1 AND id=$2`, tenant, campaignID).Scan(&c.ID, &c.Name, &c.Budget, &c.Reserved, &c.Spent)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id,s.name,s.market,s.format,
 SUM(CASE WHEN r.state='held' THEN 1 ELSE 0 END),SUM(CASE WHEN r.state='settled' THEN 1 ELSE 0 END),SUM(CASE WHEN r.state='released' THEN 1 ELSE 0 END),
 SUM(CASE WHEN r.state='held' THEN r.cost ELSE 0 END),SUM(CASE WHEN r.state='settled' THEN r.cost ELSE 0 END),
 SUM(CASE WHEN r.state='held' AND r.expires<$3 THEN 1 ELSE 0 END)
 FROM reservations r JOIN screens s ON s.id=r.screen WHERE r.tenant=$1 AND r.campaign=$2 GROUP BY s.id,s.name,s.market,s.format ORDER BY s.market,s.id`, tenant, campaignID, out.ObservedAt)
	if err != nil {
		return out, err
	}
	index := map[string]int{}
	var held, spent int64
	for rows.Next() {
		var v ScreenAssurance
		v.Decisions = map[string]int64{}
		if err = rows.Scan(&v.ScreenID, &v.Name, &v.Market, &v.Format, &v.ReservedPlays, &v.SettledPlays, &v.ReleasedPlays, &v.HeldMicros, &v.SettledMicros, &v.AwaitingEvidence); err != nil {
			rows.Close()
			return out, err
		}
		index[v.ScreenID] = len(out.Screens)
		out.Screens = append(out.Screens, v)
		held += v.HeldMicros
		spent += v.SettledMicros
		out.SettledPlays += v.SettledPlays
		out.HeldPlays += v.ReservedPlays
		out.AwaitingEvidence += v.AwaitingEvidence
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	reservationExpr := `json_extract(d.payload,'$.reservation_id')`
	if s.Postgres {
		reservationExpr = `(d.payload::jsonb ->> 'reservation_id')`
	}
	rows, err = tx.QueryContext(ctx, `SELECT r.screen,d.status,COUNT(*) FROM deliveries d JOIN reservations r ON r.tenant=d.tenant AND r.id=`+reservationExpr+` WHERE d.tenant=$1 AND r.campaign=$2 GROUP BY r.screen,d.status`, tenant, campaignID)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var screen, status string
		var n int64
		if err = rows.Scan(&screen, &status, &n); err != nil {
			rows.Close()
			return out, err
		}
		if i, ok := index[screen]; ok {
			out.Screens[i].Decisions[status] = n
		}
		out.Decisions[status] += n
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.BudgetConsistent = held == c.Reserved && spent == c.Spent && held+spent <= c.Budget
	return out, tx.Commit()
}
