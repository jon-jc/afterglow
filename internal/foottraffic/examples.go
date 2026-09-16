package foottraffic

import (
	"crypto/sha256"
	"fmt"
	"time"
)

func validScenario(id string) bool {
	switch id {
	case "baseline", "busy", "gaps", "quiet":
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
		}
		windows = append(windows, w)
	}
	b.Windows = windows
	return b, nil
}
