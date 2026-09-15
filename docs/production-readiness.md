# Production release gates

Afterglow has tested reliability mechanisms; it is not yet a verified production deployment. This document distinguishes implemented controls from evidence still required for a real launch.

## Implemented deployment controls

- Protected startup requires explicit PostgreSQL, tenant identity and a sufficiently long API credential. SQLite and automatic schema changes are restricted to local demo mode.
- `go run ./cmd/migrate` is the explicit deployment migration step. It requires `DATABASE_URL`, has a 30-second deadline, records a schema version and normalized checksum, and serializes PostgreSQL migration ownership with a transaction-scoped advisory lock.
- Version 1 adopts the prior schema without clearing data. It does not inspect every existing column/constraint for manual drift. Baseline adoption is for the known earlier Afterglow schema; inspect any independently modified database before adopting it.
- Runtime startup only reads schema metadata. Missing metadata, an edited checksum or a newer schema prevents startup. Future schema changes require explicit additional migration versions; never edit the version-1 schema after deployment.
- Optional `API_KEY_PREVIOUS` supports a controlled two-key transition. Removing the previous key revokes it on that deployment. Keys remain deployment-scoped, not per-user RBAC or partner identity.
- The application image contains `/migrate` as well as `/afterglow`; the migration job must use a separate database-owner credential. Runtime roles should receive only the required DML and SELECT-on-metadata grants, not table ownership or DDL privileges.

## Deployment sequence

1. Restore a recent backup into an isolated staging database and verify business invariants.
2. Build a pinned image; run local/CI checks and vulnerability analysis. Do not promote when any required check fails or cannot execute.
3. Run `/migrate` from that image with the migration-owner DSN as a one-off deployment job.
4. Start the matching runtime image using the restricted runtime DSN. Verify readiness and authenticated API access; check that demo controls and unauthenticated mutations fail.
5. Run a staging canary through reservation, Pub/Sub, settlement and replay, checking both ledger state and traces. Exercise secret rotation, broker/SQL outages and shutdown.
6. Promote gradually only after the deployment owner confirms all gates below. Avoid automatic schema downgrades; use reviewed forward migrations and compatible application rollback plans.

## Key rotation

Generate a new random secret through your secret manager. Roll out `API_KEY=new` and `API_KEY_PREVIOUS=old`. Update callers, verify they use the new key, then roll out with `API_KEY_PREVIOUS` removed. Keep the overlap short and documented. Never write real secrets into this repository or command examples. Terraform currently configures the current key only; configure an explicit temporary previous-secret binding in a reviewed deployment change when rotating.

## Required evidence before real use

| Gate | Current evidence | Still required |
|---|---|---|
| Transaction correctness | SQLite and PostgreSQL concurrency tests; exact-balance end-to-end checks | Staging tests with actual partner traffic contracts |
| Migration safety | Version/checksum tests, data preservation, PostgreSQL read-only runtime check | Restore rehearsal, restricted-role rollout and upgrade/rollback exercise |
| Authentication | Bearer validation and rotation-window tests; Cloud Run IAM template | Real service identities, partner authorization, rotation drill, security review |
| Transport | Google SDK with test server; outbox, retries and fencing | Managed Pub/Sub IAM/DLQ behavior and regional dependency failures |
| Capacity | Bounded admission and concurrency invariants | Agreed request rate, payload mix, p95/p99 and backlog SLOs; sustained and burst load tests |
| Operations | Health/readiness, traces, metrics, recovery controls | Dashboards, alert routing, on-call ownership and runbooks exercised by another operator |
| Recovery | Persistent database/outbox and airport-cache recovery tests | Cloud SQL backups/PITR, restore time and data-loss objectives proven |
| Airport microservice | Live reference import, cached outage behavior and read API | Authenticated cloud ingress, durable shared storage and refresh ownership |
| Supply chain | Pinned Go vulnerability-check command in CI | Passing hosted CI, image digest/signing, container scan and dependency update policy |
| Business integration | Explicit synthetic reservation/receipt contract | Vendor certification, trusted playback evidence, campaign lifecycle and retention policy |

GitHub Actions has been unable to start due to the account billing/spending-limit restriction. Local tests are evidence for the tested behavior, not a substitute for a passing production release pipeline. No cloud resources have been deployed or billing settings changed.
