package foottraffic

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

func validScenario(id string) bool {
	switch id {
	case "baseline", "busy", "gaps", "quiet", "commuter", "retail", "threshold", "interruption", "random":
		return true
	}
	return false
}

// Each illustrative feed has an isolated namespace derived from the trusted
// tenant. Switching examples never replaces immutable windows or chooses a tenant.
func scenarioTenant(tenant, scenario string) string {
	if scenario == "baseline" {
		return tenant // Preserve existing default imports.
	}
	hash := sha256.Sum256([]byte(tenant + "\x00" + scenario))
	return fmt.Sprintf("traffic-sample-%x", hash)
}

// ExampleForScenario remains deterministic for a source/zone/hour across calls
// and overlapping report periods. Scenarios illustrate data quality, not ad lift.
func ExampleForScenario(now time.Time, scenario string) (Batch, error) {
	if !validScenario(scenario) {
		return Batch{}, ErrInvalid
	}
	if scenario == "random" {
		return randomExample(now)
	}
	b := Example(now)
	if scenario == "baseline" {
		return b, nil
	}
	b.ID = fmt.Sprintf("sample-%s-%d", scenario, now.UTC().Truncate(time.Hour).Unix())
	windows := make([]Window, 0, len(b.Windows))
	for _, w := range b.Windows {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%d", scenario, w.Zone, w.Start)))
		switch scenario {
		case "busy":
			w.Count = 600 + int64(hash[0])*4
		case "gaps":
			// Omit the transit zone and alternate retail hours consistently.
			if w.Zone == "transit" || (w.Zone == "retail" && (w.Start/3600000)%2 == 0) {
				continue
			}
		case "quiet":
			w.Count = 3 + int64(hash[0])%17
			// One zone remains reportable to contrast released and hidden cells.
			if w.Zone == "downtown" {
				w.Count = 20 + int64(hash[0])%15
			}
		case "commuter":
			w.Count = 50 + int64(hash[0])%100
			if w.Zone == "transit" {
				hour := time.UnixMilli(w.Start).UTC().Hour()
				w.Count = 150 + int64(hash[0])
				if (hour >= 7 && hour <= 9) || (hour >= 16 && hour <= 18) {
					w.Count *= 4
				}
			}
		case "retail":
			w.Count = 35 + int64(hash[0])%90
			if w.Zone == "retail" {
				w.Count = 350 + int64(hash[0])*3
			}
		case "threshold":
			// Alternate exactly below/at the publication boundary, not a
			// random privacy guarantee. Stable across overlapping batches.
			w.Count = 19 + (w.Start/3600000+int64(len(w.Zone)))%2
		case "interruption":
			if (w.Start/3600000)%6 < 2 {
				continue
			}
		}
		windows = append(windows, w)
	}
	b.Windows = windows
	return b, nil
}

func randomExample(now time.Time) (Batch, error) {
	var entropy [16 + 48*2]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return Batch{}, err
	}
	b := Example(now)
	b.ID = fmt.Sprintf("random-%x", entropy[:16])
	windows := make([]Window, 0, len(b.Windows))
	for i, w := range b.Windows {
		n := int64(binary.LittleEndian.Uint16(entropy[16+i*2:]))
		// Always include one reported, one suppressed and one missing cell;
		// the rest vary on each generation. Every value remains synthetic.
		kind := n % 10
		if i == 0 {
			kind = 5
		} else if i == 1 {
			kind = 1
		} else if i == 2 {
			kind = 0
		}
		if kind == 0 {
			continue
		}
		if kind < 3 {
			w.Count = 2 + n%18
		} else {
			w.Count = 20 + n%981
		}
		windows = append(windows, w)
	}
	b.Windows = windows
	return b, nil
}
