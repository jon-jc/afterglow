# Choose and verify the staging-first route

Adds a repeatable Go rehearsal for authenticated PostgreSQL HTTP intake and asynchronous recovery. It uses a uniquely created schema in a local test database, refuses remote hostnames and connection overrides, runs concurrent clients, retries rate-limited requests without changing identities, reconnects with a durable backlog, and drains work with four workers including an injected lost acknowledgement. Only the schema created by the harness is removed afterward.

The provisional Terraform region is us-west1 (Oregon); no project ID, credentials or cloud resources are invented. The route and remaining managed-service tests are documented in `docs/staging-route.md`. CI now includes the rehearsal, though hosted runs remain blocked by the account billing/spending-limit setting.

## Actual local PostgreSQL result — September 15, 2026

- 12 clients, 120 plays, 252 logical HTTP operations, 378 total HTTP attempts.
- 126 rate-limit responses retried; application admission controls stayed enabled.
- 240 accepted receipts survived closing/reopening the database pool.
- 120 settled, 120 duplicates, exactly 6,000,000 micros spent, zero held.
- Four workers drained the backlog in 4.66 seconds; one outbox row redelivered after the injected lost acknowledgement.
- Ingestion: 11.17 seconds. Logical-operation latency including admission retry waits: p50 18.27 ms, p95 2004.74 ms, p99 3005.04 ms.

This is one bounded local run using SQL transport, not a sustained benchmark, OS-crash test, managed Pub/Sub result or production SLO. The report contains no credentials or private business data. Go tests/vet and target-guard tests passed. A second full PostgreSQL rehearsal passed with the Go race detector enabled. Cloud deployment remains pending real project configuration and the release gates documented earlier.
