package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jon-jc/afterglow/internal/core"
)

const maxBatchReceipts = 50

type batchResult struct {
	Index     int    `json:"index"`
	Status    int    `json:"status"`
	ID        string `json:"id,omitempty"`
	Replayed  bool   `json:"replayed"`
	Retryable bool   `json:"retryable"`
	Detail    string `json:"detail"`
}

type batchResponse struct {
	Accepted int           `json:"accepted"`
	Rejected int           `json:"rejected"`
	Replayed int           `json:"replayed"`
	Results  []batchResult `json:"results"`
}

// Each receipt has its own transaction. An interrupted batch can be retried with
// the same event IDs and payloads; it is deliberately not an atomic batch commit.
func (s *Server) acceptBatch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Receipts []json.RawMessage `json:"receipts"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Receipts) < 1 || len(in.Receipts) > maxBatchReceipts {
		problem(w, 422, "receipts_must_contain_1_to_50_items")
		return
	}
	// The middleware already charged one token. Charge remaining items before
	// any writes, so batching cannot multiply the mutation admission budget.
	if !s.allowCost(float64(len(in.Receipts) - 1)) {
		w.Header().Set("Retry-After", "2")
		problem(w, 429, "batch_rate_limited")
		return
	}
	out := batchResponse{Results: make([]batchResult, 0, len(in.Receipts))}
	for i, raw := range in.Receipts {
		result := batchResult{Index: i, Status: 400, Detail: "invalid_receipt_object"}
		var receipt core.Receipt
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if len(raw) > 0 && bytes.TrimSpace(raw)[0] == '{' && dec.Decode(&receipt) == nil {
			id, replayed, err := s.Store.Accept(r.Context(), s.Tenant, receipt)
			switch {
			case err == nil:
				result.Status, result.ID, result.Replayed = 202, id, replayed
				result.Detail = "database_and_outbox_committed"
				out.Accepted++
				if replayed {
					out.Replayed++
				}
			case errors.Is(err, core.ErrInvalid):
				result.Status, result.Detail = 422, err.Error()
			case errors.Is(err, core.ErrConflict):
				result.Status, result.Detail = 409, "event_id_conflicts_with_prior_content"
			default:
				result.Status, result.Detail, result.Retryable = 503, "temporarily_unavailable", true
				slog.Error("batch_receipt_failed", "index", i, "error", err)
			}
		}
		if result.Status != 202 {
			out.Rejected++
		}
		out.Results = append(out.Results, result)
	}
	status := 202
	if out.Rejected > 0 {
		status = 207
	}
	respond(w, status, out)
}
