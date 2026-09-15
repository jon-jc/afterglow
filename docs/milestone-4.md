# Interview field guide

Adds a self-contained HTML reading site comparing Fluxgate and Afterglow: concise interview answers, architecture and transaction boundaries, concrete improvements, failure examples, a live-demo script, technical follow-ups, teamwork/AI-assisted-development guidance, and questions for Stanford.

The comparison identifies Fluxgate's process-local HTTP idempotency cache and distinguishes it from its durable per-window aggregation ledger. It explains Afterglow's SQL-backed request identities without implying that the two applications share the same architecture or that the demo is a certified vendor integration.

The guide is embedded in the Go service, linked from the operations console, and also works as a standalone HTML file with native expandable sections and print styling. No new runtime dependencies or cloud deployment are introduced.

Validation: Go executable rebuilt successfully. Browser checks passed at 1440px and 390px, including section navigation, expandable answers, no broken internal anchors, no horizontal page overflow on mobile, and no reported browser errors. The same HTML opened successfully directly from disk. Git diff whitespace checks passed. GitHub Actions remains unavailable because of the previously observed account billing/spending-limit restriction; this content change was verified locally.
