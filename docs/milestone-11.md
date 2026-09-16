# Milestone 11: live integration proof

Added a responsive six-stage integration demonstration with real API assertions, persisted evidence inspection, JSON export, and resumable request identities. Added a tenant-scoped exact receipt evidence endpoint independent of the recent dashboard window. Overview links to the demonstration.

Moved the interview study guide into docs as a standalone local HTML file, removed its sidebar entry and removed it from the embedded application assets.

Validation:
- Full Go suite and go vet passed.
- PostgreSQL core suite passed with race detector, including evidence tenant isolation and missing-reservation behavior.
- HTTP tests verify authentication, cross-tenant 404, missing and invalid IDs, and successful evidence shape.
- 14 live-proof browser checks passed, including a lost response after a committed batch, safe resumption, exact outcomes, exported records, and study-guide 404.
- Eight partner intake checks and ten existing UI checks passed.
- Desktop and mobile visual review; no viewport overflow or JavaScript errors.
- OpenAPI validation passed.

This is still a synthetic local demonstration. Managed GCP/Vistar integration and production release gates remain separate. GitHub Actions had an account billing blocker on previous milestones; local checks are recorded explicitly.
