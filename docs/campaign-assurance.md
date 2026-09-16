# Campaign assurance

## Business fit

CCO's public [programmatic advertising page](https://clearchanneloutdoor.com/programmatic-advertising/) emphasizes partner-based buying and measurable campaign impact. Its [RADAR data solutions](https://clearchanneloutdoor.com/radar-data-solutions/) describe planning and campaign measurement, including in-flight insight. Reviewed September 15, 2026 (Pacific).

Afterglow contributes a narrower operational foundation: know which reserved plays settled, where money remains held, and which partner reports need investigation. This is an independent design inference from public material, not a description of CCO's internal systems or a replacement for RADAR attribution.

## API and correctness

`GET /api/v1/campaigns/{id}/assurance` is authenticated and tenant-bound. It returns a consistent read of campaign balances, reservation-derived spend and play counts, per-screen totals and independently grouped receipt decisions. PostgreSQL uses repeatable-read; SQLite uses a read transaction. The handler inherits the ten-second request deadline.

Money is summed from reservations, never from a receipt join. Many reports may refer to one reservation; joining and summing prices would multiply spend. Receipt counts are aggregated separately, then combined by reserved screen. A mismatched screen in a receipt does not change the inventory attribution.

`budget_consistent` checks campaign held and settled balances against reservation totals and verifies they stay within budget. It is an explicit invariant check, not an independent payment audit. `awaiting_evidence` counts held reservations past the play window, including those whose existing evidence is invalid. Receipt grace and release rules remain unchanged.

All retained history is included, independent of the console's latest-100 window. Unknown-reservation receipts cannot be attributed to a campaign and are excluded. Historical quarantines remain visible after a corrected event settles. Follow-up links open workspace-wide investigation pages and are labeled accordingly.

## Product behavior

Campaign selection triggers a report fetch. Older in-flight responses cannot overwrite a newer selection. A failed refresh disables export, rather than offering stale evidence. Reports show observation time and require an explicit refresh. Exported JSON preserves exact integer USD micros; displayed dollars round to cents.

Operational follow-up is deterministic and explainable: investigate inconsistent balances, follow up on overdue held plays, review quarantined evidence, recover exhausted retries, or watch accepted work. No audience reach, store visits, conversion lift, real CCO inventory rights or Vistar certification is inferred.

## Scale boundary

This implementation reads the authoritative store directly, avoiding a second eventually-consistent balance source. Receipt-to-reservation linkage is extracted from the immutable JSON payload, with syntax for both supported databases. At larger scale this needs query-plan/load evidence, a normalized indexed link or incremental reporting projection, and projection freshness monitoring. The ten-second deadline bounds requests; no production throughput claim is made. No schema migration is introduced in this milestone.
