# GitHub Actions investigation — September 15, 2026 (Pacific)

## Hosted-run blocker

Inspected [PR 12 run 35041349778](https://github.com/jon-jc/afterglow/actions/runs/35041349778) and [main run 35040462941](https://github.com/jon-jc/afterglow/actions/runs/35040462941). Both the Go and Terraform jobs were rejected before starting. GitHub's annotation reads:

> The job was not started because recent account payments have failed or your spending limit needs to be increased. Please check the 'Billing & plans' section in your settings

No runner test log exists for these jobs. This is a GitHub account execution blocker, not an observed Go assertion failure. No tests were removed, skipped, or marked successful to hide the blocker.

The account owner must resolve the payment issue or review the Actions spending limit in GitHub Billing & plans. Then rerun the failed jobs. The repository stays private. No billing settings or paid limits were changed during this investigation.

## Local verification of the current revision

- Linux/WSL Ubuntu: Go vet, full Go race suite, PostgreSQL core race suite, and application build passed.
- Authenticated PostgreSQL recovery rehearsal passed: 240 retained receipts, 120 settled, 120 duplicates, one redelivered outbox row, 6,000,000 settled micros, zero held micros.
- Linux Node 22.23.2 smoke test passed all 13 checks, including worker interruption, lost acknowledgement, retry exhaustion/replay and concurrent budget pressure. Official Node archive checksum verified before use.
- Go vulnerability scan v1.8.0 found no affected code paths or imported-package advisories. One advisory exists in an uncalled module component.
- Terraform 1.12.2 initialized and validated with locked Google providers 6.50.0 on Windows.
- 49 browser checks passed for the UI milestone.

These are local results, not a replacement green GitHub check. The local PostgreSQL version is 18; Actions config uses 17. Terraform validation used Windows rather than a GitHub Linux runner. Managed GCP behavior remains outside these local checks.
