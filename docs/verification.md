# Verification evidence

Verified locally on September 15, 2026. These results apply to the demo implementation; they are not evidence of a deployed production service.

| Check | Result | Scope |
|---|---|---|
| `go test ./...` | Pass | Windows Go 1.27, SQLite persistence, domain and HTTP tests, worker recovery, Pub/Sub SDK integration test |
| `go vet ./...` | Pass | Go static checks |
| `go test -race -count=1 ./...` | Pass | Linux Go 1.26 under WSL; all packages |
| PostgreSQL core tests with `-race` | Pass | Real PostgreSQL 18.6; concurrent reservation cap and duplicate-play settlement |
| Google Pub/Sub SDK integration | Pass | Real v2 client publishing/receiving over gRPC against Google's `pstest` service; duplicate delivery and poison envelope |
| `node scripts/smoke.mjs` | Pass | 13 full HTTP-to-database-to-worker-to-query checks on an isolated database |
| Browser workflow | Pass | Simulation, duplicate receipts, schema quarantine/replay, manual $2.50 reservation and playback |
| Responsive layout | Pass | Desktop 1440px and mobile 390px; mobile document width remains 390px |
| Browser errors | None reported | Agent-browser error collection during exercised workflows |
| OpenAPI validation | Valid | Redocly validation; optional license/health-probe 4xx guidance is advisory |
| Terraform validation | Pass | Terraform 1.12.2, Google/Google Beta providers 6.50.0; no plan/apply |
| GitHub Actions | Blocked before execution | Account billing/spending-limit error, not a code-test result |
| Managed GCP / actual Vistar | Not exercised | No cloud deployment, production traffic, vendor credentials or certification |

## HTTP end-to-end sequence

1. Start an isolated service and seed its demo catalog.
2. Reject a body that tries to inject a tenant field.
3. Settle 12 receipts through the asynchronous worker: exactly $19.95.
4. Send five unique event IDs for one play: one $1.25 charge, four duplicates.
5. Quarantine schema v99 without changing spend.
6. Replay that receipt: it remains quarantined and its attempt count increases.
7. Lose an acknowledgement after commit: redelivery leaves one charge.
8. Pause the dispatcher and accept 12 new receipts durably.
9. Resume and settle the retained backlog.
10. Inject five transient processing failures; observe the recovery queue.
11. Replay after recovery; settle the receipt once.
12. Race 32 reservation requests; assert every campaign remains inside its cap.
13. Verify HTTP idempotency returns the same reservation and conflicts on changed content.

The final sequence had 27 settled plays, four duplicate receipts, one quarantined receipt and **$43.65** settled spend. The tests assert exact integer-micros balances, not just successful HTTP codes.

## Stronger failure evidence

- A failing audit write rolls back reservation state, receipt status and budget movement.
- Reopening the local database preserves accepted receipts for later processing.
- An expired lease can be reclaimed; the previous owner cannot mark the new claim as dispatched.
- Completion from an earlier processing attempt cannot erase an operator's requeued work.
- A receipt from another tenant cannot settle the original tenant's reservation.
- Expired/released reservations cannot be resurrected by late evidence.

## Reproduce

See [README](../README.md) for the commands and [operations](operations.md) for PostgreSQL/emulator configuration. The smoke script writes `artifacts/smoke.json`; artifacts and local databases are intentionally excluded from source control.
