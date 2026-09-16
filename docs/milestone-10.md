# Milestone 10: partner intake

Replaced the airport directory with a working partner playback intake console. It uses the existing Go batch endpoint, shows per-item results and preserves event IDs and payloads for retry. Drafts and responses survive navigation; background polling does not replace the editor. The optional airport backend remains available separately, but its UI and UI-specific regression script were removed.

Interview guide now explains the connection from Fluxgate durability and partial success to Afterglow transactional settlement. This remains a synthetic partner contract, not a live Vistar integration.

Validation:
- Go HTTP API tests passed; embedded console built successfully.
- Eight browser checks passed, including a real $0.01 hold, mixed batch, unchanged retry, one settlement and one duplicate, and draft/focus preservation.
- Ten existing UI regression checks passed.
- Desktop/mobile visual inspection passed; no browser errors.
- GitHub Actions has previously been blocked by account billing. Local verification does not substitute for managed GCP staging validation.
