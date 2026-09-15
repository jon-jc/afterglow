# Partner batch intake

Adds `POST /api/v1/receipts/batch` in Go. A partner can submit up to 50 receipts and receive an ordered result for every item, including durable acceptance, replay, malformed input, identifier validation, identity conflict and retryable storage failures. Each item retains the existing SQL receipt/outbox transaction; a batch is intentionally not all-or-nothing. The caller retries ambiguous responses with unchanged event identities and payloads.

Limits apply before writes: the existing 32 KiB body cap, a 50-item cap and per-item admission tokens. This prevents batching from multiplying the request rate budget. Unknown fields inside one item reject that item without discarding valid neighbors; a malformed envelope rejects the whole request.

This extends Fluxgate's partial-success lesson into advertising integration. The response acknowledges storage, not settlement or Vistar compatibility. Worker validation and duplicate financial-effect protection remain unchanged.

Validation: Go HTTP tests cover mixed outcomes, whole-batch retry, stable delivery IDs, conflicts, strict envelopes, item limits and per-item rate limiting. Full local Go tests and vet passed.
