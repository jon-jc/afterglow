package httpapi

import (
	"net/http"
	"sync"

	"github.com/jon-jc/afterglow/internal/core"
	"github.com/jon-jc/afterglow/internal/pipeline"
)

func Demo(store *core.Store, worker *pipeline.Worker, tenant string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/demo/control", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Paused bool `json:"paused"`
		}
		if !decode(w, r, &in) {
			return
		}
		worker.Paused.Store(in.Paused)
		respond(w, 200, map[string]any{"paused": in.Paused})
	})
	mux.HandleFunc("POST /api/demo/scenario", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Kind string `json:"kind"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.Kind == "budget-race" {
			var wg sync.WaitGroup
			var mu sync.Mutex
			accepted, rejected := 0, 0
			for i := 0; i < 32; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _, e := store.Reserve(r.Context(), tenant, core.ID(), core.ReservationInput{CampaignID: "cmp-solstice", ScreenID: "sea-01", Cost: 10000000})
					mu.Lock()
					defer mu.Unlock()
					if e == nil {
						accepted++
					} else {
						rejected++
					}
				}()
			}
			wg.Wait()
			respond(w, 200, map[string]any{"message": "32 concurrent $10 reservations competed for remaining Solstice budget. Holds remain visible and expire after the grace period.", "accepted": accepted, "rejected": rejected})
			return
		}
		n := 1
		switch in.Kind {
		case "traffic":
			n = 12
		case "duplicate", "poison":
		case "crash":
			worker.DropAck.Store(true)
		case "retry":
			worker.FailNext.Store(5)
		default:
			problem(w, 422, "unknown_scenario")
			return
		}
		screens := []string{"sea-01", "sea-02", "pdx-01", "sfo-01", "lax-01", "den-01", "ord-01", "jfk-01"}
		campaigns := []string{"cmp-cascade", "cmp-northstar", "cmp-solstice"}
		ids := []string{}
		for i := 0; i < n; i++ {
			reservation, _, e := store.Reserve(r.Context(), tenant, core.ID(), core.ReservationInput{CampaignID: campaigns[i%3], ScreenID: screens[i%len(screens)], Cost: int64(1250000 + i*75000)})
			if e != nil {
				fail(w, e)
				return
			}
			p := core.Receipt{Version: 1, EventID: core.ID(), ReservationID: reservation.ID, ScreenID: reservation.ScreenID, PlayedAt: reservation.Created + 1, DurationMS: 10000}
			if in.Kind == "poison" {
				p.Version = 99
			}
			id, _, e := store.Accept(r.Context(), tenant, p)
			if e != nil {
				fail(w, e)
				return
			}
			ids = append(ids, id)
			if in.Kind == "duplicate" {
				for k := 0; k < 4; k++ {
					p.EventID = core.ID()
					id, _, e = store.Accept(r.Context(), tenant, p)
					if e != nil {
						fail(w, e)
						return
					}
					ids = append(ids, id)
				}
			}
		}
		messages := map[string]string{"traffic": "12 playback receipts accepted. Follow the pipeline as reservations settle.", "duplicate": "5 distinct receipts reference one reservation. Only one can move money.", "poison": "Schema v99 accepted durably. The consumer will quarantine it without charging.", "crash": "Next receipt will commit, then lose its acknowledgement. Redelivery must preserve the balance.", "retry": "Next five processing attempts will fail. With no other traffic, the receipt reaches the recovery queue; replay it after recovery."}
		respond(w, 202, map[string]any{"message": messages[in.Kind], "delivery_ids": ids})
	})
	return mux
}
