# Durable delivery and recovery

Playback receipt ingestion now writes a transactional outbox. A leased dispatcher delivers each record through the real Google Pub/Sub v2 client, or a local SQL-backed transport for a self-contained demo. Publish success is recorded only after the broker acknowledges; retries retain the same delivery ID.

The consumer commits a reconciliation decision before acknowledging. A lost acknowledgement can cause another delivery, but cannot repeat the spend. Permanent payload failures are durably quarantined; dispatch failures use exponential backoff with jitter, a short circuit-breaker cooldown, and a five-attempt recovery queue. Replaying preserves the original payload and reruns validation. Malformed Pub/Sub envelopes are retained separately before acknowledgement.

Trace context survives the HTTP-to-outbox-to-consumer boundary. Prometheus metrics use bounded outcome labels. Shutdown removes readiness, drains HTTP work and cancels workers.

## Validation

- Go tests and `go vet` pass locally.
- Real Google Pub/Sub client exercised over gRPC against Google's `pstest` service, including repeated delivery and poison payload quarantine.
- Lease expiry, stale-owner fencing, retry exhaustion, replay, lost acknowledgement and replay-versus-completion races have explicit tests.
- An injected audit failure rolls back the entire settlement transaction.
- Business-invariant tests pass against a real PostgreSQL 18.6 instance with Go's race detector enabled.
- GitHub Actions cannot start because of the account billing/spending-limit setting. This is an infrastructure block, not a passing CI result.

Cloud IAM, managed Pub/Sub delivery policies and Cloud Run behavior require deployment validation. The default transport is a local demo; it does not claim to be GCP.
