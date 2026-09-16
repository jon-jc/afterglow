# Milestone 14: campaign assurance

Added a Go full-history campaign report and an operations view connecting settled plays, held/available budget, screen-level delivery, duplicate decisions and explainable follow-up. Money is aggregated independently of repeated receipts. Reports expose a balance invariant and their scope; exported JSON retains exact integer amounts.

Company alignment and technical trade-offs are documented in campaign-assurance.md. This is operational delivery evidence, not audience measurement or attribution.

Validation: full Go suite and vet passed; PostgreSQL core race suite passed. New tests cover 105 receipts against one reservation, no multiplied spend, tenant isolation, deliberate balance drift, overdue held plays, unknown reservations and empty campaigns. HTTP tests cover success, authentication, invalid/missing campaign. OpenAPI validation passed. Eight new browser checks plus 27 existing UX checks passed. Desktop/mobile reviewed; no JavaScript errors observed.
