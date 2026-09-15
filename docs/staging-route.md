# Chosen route: measured local rehearsal, then one-region GCP staging

Use PostgreSQL and the existing Go services. Keep the financial pipeline on the transactional outbox/Pub/Sub architecture; keep external airport refresh independent. Avoid adding more infrastructure until the current boundaries are measured and exercised.

## Region and cloud shape

The provisional staging region is **us-west1 (Oregon)**, supported by [Cloud Run](https://docs.cloud.google.com/run/docs/locations). This is a reasonable initial choice for the Seattle context, not a measured latency winner. Co-locate Cloud Run and Cloud SQL and revisit the choice if the employer's integration endpoint or data-residency constraints require another region.

Use separate API and worker identities, an explicit migration job, PostgreSQL, a Pub/Sub subscription plus dead-letter review, and secret-manager credentials. Keep IAM invocation private. Start with the existing modest instance caps and measure SQL connection/lock pressure before increasing them. Do not provision a project or invent credentials merely to make deployment appear complete.

## Repeatable local workload

Set `REHEARSAL_DATABASE_URL` to a **local PostgreSQL test database**, then run:

```sh
go run ./cmd/rehearsal
```

The command refuses remote hosts and unsupported connection query overrides. It creates a uniquely named schema, migrates/seeds that schema, starts a private loopback HTTP server with bearer authentication, and removes only its own schema when finished. Existing demo data and running application servers are untouched. A forcibly killed harness may leave its isolated schema for manual inspection/cleanup.

The fixed profile sends 120 plays from 12 concurrent clients. Each play reserves $0.05 and submits two event identities for the same reservation; every tenth batch is retransmitted unchanged. Admission limits remain enabled. Workers initially stay stopped so accepted work accumulates. The API and database pool close and the database reopens; four workers then reconcile the pending work, including one injected lost acknowledgement.

Pass conditions: 240 receipts survive reopen, exactly 120 reservations settle, 120 semantic duplicates are suppressed, at least one outbox row redelivers, all outbox work completes, held money returns to zero, and total spend is exactly $6.00. A failure exits nonzero. The entire run has a 90-second deadline.

Latency is measured per logical HTTP operation **including admission retry delays**. Rate-limited attempts are counted separately, not hidden as successes. This small bounded test is a correctness/recovery baseline, not a sustained capacity benchmark, an OS-crash test or an SLO claim. It uses SQL transport, not managed Pub/Sub.

The [recorded baseline](evidence/rehearsal-postgres-2026-09-15.json) passed with 240 receipts preserved, $6.00 settled and zero held. Four workers drained the backlog in 4.66 seconds. The 126 rate-limit responses are included in the report.

## Next gate

Once a real GCP project is configured, run the same business invariants through managed Pub/Sub and Cloud SQL, exercise IAM/DLQ/credential-rotation behavior, rehearse a backup restore, and establish latency/backlog targets from measured staging traffic. Required evidence is tracked in [production readiness](production-readiness.md). Cloud deployment and real production readiness remain unverified until those gates pass.
