# Operations console and interview-ready delivery

Adds an embedded operations console backed by the Go APIs: screen map, campaign balances, manual reservation/playback, searchable receipt ledger, recovery details and six live failure scenarios. The console runs from the Go binary with no front-end build or external runtime assets.

Includes a GoLand run configuration, OpenAPI contract, Docker/emulator setup, validated Cloud Run/Pub/Sub/IAM Terraform, and an interview walkthrough with technical trade-offs and questions about the CCO/Vistar integration. Synthetic data and the absence of vendor certification are explicit.

## Validation

- Go tests and static checks pass; Linux race suite and PostgreSQL concurrency tests pass.
- 13 full HTTP end-to-end checks pass, including replay after retry exhaustion and exact financial balances.
- Browser tests confirm traffic simulation, duplicate suppression, quarantine/replay and manual reservation/playback. No browser errors reported; mobile layout has no document overflow.
- OpenAPI validates. Terraform provider-schema validation passes without cloud changes.
- GitHub Actions remains blocked before execution by the account billing/spending-limit setting.

Production limits and reproducible verification are documented in `docs/operations.md` and `docs/verification.md`.
