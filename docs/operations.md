# Operating Afterglow

## Local interview demo

Open the repository folder in GoLand, allow its Go module index to finish, and choose the checked-in **Afterglow** run configuration. Alternatively run `go run ./cmd/afterglow` or `./scripts/start.ps1`. Visit http://127.0.0.1:8090.

The binary embeds the entire console. No Node installation, front-end build, cloud credentials or Docker are required to run it. Node is only used by the optional end-to-end verification script. The local database persists at `data/afterglow.db`.

For a fresh demo without deleting existing evidence, stop the server and set `DATABASE_URL` to a new filename, such as `data/interview.db`. Do not run two instances against one SQLite file: the portable demo intentionally uses one writer. Use PostgreSQL for multiple processes.

## PostgreSQL + official Pub/Sub emulator

`docker compose up -d` provisions local PostgreSQL, the official Google Pub/Sub emulator and Jaeger. Start the Go application on the host. In PowerShell:

```powershell
$env:DATABASE_URL='postgres://afterglow:local-demo-only@127.0.0.1:55432/afterglow?sslmode=disable'
$env:PUBSUB_EMULATOR_HOST='127.0.0.1:8085'
$env:GCP_PROJECT_ID='afterglow-local'
$env:TRANSPORT='pubsub'
$env:OTEL_EXPORTER_OTLP_ENDPOINT='http://127.0.0.1:4318'
go run ./cmd/bootstrap
go run ./cmd/afterglow
```

The emulator does not reproduce managed-service IAM, regional failure modes or all delivery policies. The bootstrap is deliberately emulator-only. View sampled traces at http://127.0.0.1:16686. Trace sampling is parent-based at 10%; propagation remains installed when exporting is off.

To exercise the transport test against the official emulator:

```powershell
$env:TEST_PUBSUB_EMULATOR_HOST='127.0.0.1:8085'
go test -count=1 ./internal/pipeline
```

Without this variable, the transport test uses Google's `pstest` gRPC service, not the official emulator.

## Configuration

| Setting | Default | Purpose |
|---|---|---|
| `DEMO_MODE` | `true` | Seeds synthetic data; permits local unauthenticated use and fault injection. Restricted to loopback listeners. |
| `ADDR` | `127.0.0.1:8090` | HTTP listener. Container sets `0.0.0.0:8080`. |
| `DATABASE_URL` | `data/afterglow.db` | SQLite filename or PostgreSQL DSN. |
| `API_KEY` | none | At least 24 characters required outside demo mode. Send `Authorization: Bearer ...`. |
| `TENANT_ID` | `demo` | Tenant selected by this API deployment's credential. |
| `ROLE` | `all` | `api`, `worker`, or `all`. |
| `TRANSPORT` | `local` | `local` durable SQL queue, or `pubsub`. |
| `GCP_PROJECT_ID` | none | Required for Pub/Sub. Uses Application Default Credentials. |
| `PUBSUB_TOPIC` | `afterglow-receipts` | Pre-provisioned topic. |
| `PUBSUB_SUBSCRIPTION` | `afterglow-reconciler` | Pre-provisioned pull subscription. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | none | Optional trace exporter. |

## Cloud deployment template

`deploy/terraform` defines separate API and worker Cloud Run services, service identities, topic/subscriptions, a transport dead-letter queue and secret access. It references an **existing** Cloud SQL instance, container image and Secret Manager secrets. It does not create cloud infrastructure until explicitly applied.

The pull worker has minimum one instance and request-independent CPU. API and workers have separate service accounts; only workers publish/subscribe. No service is granted public invocation. The API invoker needs both an IAM identity token and the application's API credential. Workers share PostgreSQL; never put SQLite on Cloud Run's ephemeral filesystem.

The database secret should contain a PostgreSQL DSN using `/cloudsql/PROJECT:REGION:INSTANCE` as its host socket. Store actual passwords only in Secret Manager. Before deploying, configure an appropriate Cloud SQL backup/PITR policy, connection limits, a migration owner and service-specific runtime database roles. Automatic startup DDL in this demo is serialized with a PostgreSQL advisory lock; a production rollout should run versioned migrations as a dedicated deployment step.

`terraform init` / `terraform validate` checks syntax and provider schema without deploying. No cloud plan or apply is included in the verification claim. Review costs and parameters before applying. Console fault controls are intentionally unavailable in protected mode; use the authenticated REST API.

## Diagnosis and recovery

- **Growing accepted backlog:** inspect `/api/v1/snapshot` for `oldest_pending_ms`, the dispatch breaker and worker logs. Check database access and broker credentials. Accepted receipts remain durable. Admission still depends on database capacity.
- **Dispatch failures:** up to five attempts with exponential backoff/jitter. Three consecutive failures trigger a two-second local cooldown. Each dispatcher has its own breaker; this is not a fleet-wide limiter.
- **Recovery queue (`failed`):** restore the dependency, then replay the receipt from the console or `POST /api/v1/deliveries/{id}/replay`. This preserves the event ID and payload.
- **Quarantined schema/screen/time:** investigate the evidence. Replay does not magically repair a bad payload. A corrected proof needs a new event ID. Existing financial effects remain protected by the reservation state.
- **Managed Pub/Sub dead-letter subscription:** sustained consumer failures that cannot be recorded in SQL may eventually be forwarded by Pub/Sub. Recover the database, inspect the dead-letter messages and republish their original envelope after diagnosis. The console recovery queue and Pub/Sub transport DLQ are different surfaces.
- **Expired hold:** funds are released after the play window plus receipt grace period. A later proof is quarantined. There is intentionally no automatic retrospective charge.
- **Shutdown:** readiness fails, HTTP gets a short routing grace period in protected mode, in-flight HTTP drains, and workers/subscriber stop. Unacknowledged or expired leased work can be retried.

## Important limits

This is a production-oriented engineering demo, not a production certification. It has no real Vistar credentials or integration certification, no audience attribution, no device trust/receipt signing, no real billing, no availability SLO evidence, and no fleet-scale load result. The local console is designed for one trusted demonstrator. Shared inventory is synthetic; real catalog synchronization and campaign lifecycle APIs are outside this version.

The row lock on a hot campaign is an intentional throughput limit in exchange for a hard budget cap. The application has bounded request size/concurrency and a per-process rate limiter, but no tenant-wide quota service or database admission watermark. Audit writes are append-only in application code, not tamper-proof storage. Retention, data archival, alert policies, backup restore drills, authentication rotation and independent security review are still required before production use.
