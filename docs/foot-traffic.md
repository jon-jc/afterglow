# Foot-traffic measurement service

This module demonstrates a partner aggregate feed using **synthetic data only**. It is inspired by the public description of [CCO RADAR](https://clearchanneloutdoor.com/radar-data-solutions/), which says CCO licenses aggregated and/or anonymous mobile-location data from partners. It is not a CCO integration, mobile SDK, device-tracking system, or attribution model.

## Try it

Start Afterglow normally. Open **Foot traffic → Load sample feed → Replay last batch**. Counts stay unchanged on replay. Expand a zone to inspect hourly counts, missing data and suppression. The report covers six completed UTC hours and compares corresponding hours from the previous day. Percent change only uses pairs where both counts are reportable.

## API and guarantees

- `GET /api/v1/foot-traffic/example` generates a deterministic synthetic batch: four fixed zones, six hours, two days. The generated input deliberately includes small counts.
- `POST /api/v1/foot-traffic/batches` validates and commits synchronously. **201** means the batch transaction finished; **200** returns a previously committed batch result. It does not return 202 or imply an asynchronous pipeline that is not present.
- `GET /api/v1/foot-traffic/report` emits fixed windows. Counts below 20 become `null`, with `status: "suppressed"`. Missing windows are also `null`, with a distinct `missing` status. Published totals include only released cells; comparisons exclude unreportable pairs.
- A unique `(tenant, batch_id)` plus canonical fingerprint prevents changed-payload retries. A unique `(tenant, source, zone, hour)` prevents the same window being counted again under a new batch ID. Conflicting content rolls back the **whole** transaction.
- Inputs accept 1–96 distinct completed UTC-hour windows from the last seven days, counts from 0 through 1,000,000, a fixed synthetic provider, and known zones. The 32 KiB JSON parser rejects unknown fields, including device IDs and precise coordinates.
- The original settlement tables, budgets and receipt pipeline remain separate. This demo shares the database connection pool for easy local startup; production analytics should have an independently sized store and pool.

The threshold is a demo reporting rule, **not a proof of anonymity**. There are no device-level records to establish unique reach or deduplicate people across hours, zones or providers. Temporal changes are descriptive sample activity, not verified ad exposure, store visits, conversion or causal lift. Production corrections need an explicit revision contract; this version treats accepted windows as immutable. Stored history has no automatic expiry yet.

## Separate Go process

`cmd/foottraffic` runs the same service independently. It accepts SQLite or PostgreSQL through `FOOT_TRAFFIC_DATABASE_URL`. Set `FOOT_TRAFFIC_MIGRATE=true` once to install its additive version-1 tables, then unset that variable. Set `FOOT_TRAFFIC_API_KEY` to a strong secret of at least 24 characters before serving. Every request requires `Authorization: Bearer <key>`. The default address is `127.0.0.1:8092`; override `FOOT_TRAFFIC_ADDR` for your runtime. `FOOT_TRAFFIC_TENANT` fixes the service's tenant; clients cannot choose another tenant in their payload.

Run `go run ./cmd/foottraffic`. The integrated local demo mounts the same handler in-process and initializes these tables automatically. The production Afterglow runtime does not enable this synthetic module. No mobile-data provider credentials or cloud resources are provisioned.

## GCP deployment direction

Start with a Go ingestion service on Cloud Run and authorized aggregate feeds. For asynchronous acceptance, add durable dispatch intent or acknowledge only after Pub/Sub accepts publication; define exactly what 202 promises. Use Dataflow only when event-time windows, late arrivals, joins and state warrant a streaming engine. Keep settlement in Cloud SQL; use partitioned BigQuery tables for analytical history and approved reporting views. Provider/event identities and correction versions must survive replay independently of broker message IDs.

Use separate service identities, least-privilege IAM, restricted raw-data access, defined retention/deletion, monitored dead-letter handling and replay procedures. For Dataflow, implement bad-record side outputs rather than assuming Pub/Sub dead-letter behavior applies unchanged. These are design proposals, not deployed capabilities or a compliance claim.

## Verification

Store tests cover concurrent replay, overlapping batches, canonical order, all-or-nothing conflicts, suppression, paired comparisons and tenant isolation. HTTP tests cover replay status codes, rejected device fields and body limits. CI runs race-enabled tests against SQLite and PostgreSQL, plus an end-to-end smoke test confirming measurement ingestion does not move settlement spend.
