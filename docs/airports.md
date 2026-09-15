# Airport reference service

Run `go run ./cmd/airports` in a second terminal, or run **Airport Catalog** in GoLand. Then start Afterglow normally and choose **Airports**. The service listens on `127.0.0.1:8091`; Afterglow forwards its authenticated `/api/v1/airports` reads to that service. No browser CORS exception or third-party browser request is needed.

## Coverage and source

The [OurAirports public-domain CSV](https://ourairports.com/data/) is refreshed nightly by its publisher. Our service selects records with `scheduled_service=yes`, excludes closed facilities, and includes US plus PR, VI, GU, AS, MP and UM country codes. See the [source dictionary](https://ourairports.com/help/data-dictionary.html).

This is an explicit proxy for “US commercial airports,” not the FAA's passenger-enplanement classification. It includes smaller scheduled-service facilities and seaplane bases when marked in the source. It excludes charter-only/private airports without scheduled service. Data can contain errors and changes over time. No CCO inventory ownership, flight status or audience measurement is inferred.

The first live import on September 15, 2026 selected **720 records**. The UI reports the current total, fetch time, source timestamp and stale status rather than fixing that count in the interface.

## Data flow and failure behavior

1. The Go service downloads the CSV with a 30-second timeout and a 32 MiB cap.
2. It checks required columns, CSV structure, unique identifiers and finite, valid coordinates; it rejects implausibly small imports.
3. It writes a temporary JSON snapshot, syncs and closes it, then renames it over the prior cache before publishing the immutable in-memory snapshot. Readers see a complete old or new catalog.
4. Successful refreshes run every 24 hours. A failed attempt preserves the last snapshot and retries after five minutes. These timers run only while the service is running.
5. Cache is loaded on restart. API responses mark data stale after a failed refresh or after 48 hours. With no usable snapshot, reads return 503. Main campaign/receipt operations do not depend on this service.

`GET /v1/airports` accepts `q` (100 characters), `region` (20), `limit` (1–200, default 50), and `offset`. Results are sorted by source identifier. The response contains the current page, all matching map points, matching and total counts, region choices, fetch timestamp, CSV SHA-256 version, scope and freshness. Offset pagination operates on the current snapshot; a refresh between requests can change page boundaries. This is a small reference catalog, not an unbounded event stream.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `AIRPORT_ADDR` | `127.0.0.1:8091` | Separate service address; loopback required |
| `AIRPORT_CACHE` | `data/airports.json` | Service-owned snapshot; excluded from Git |
| `AIRPORT_SERVICE_URL` | `http://127.0.0.1:8091` | Afterglow's fixed upstream; never taken from browser input |

The catalog's source URL is fixed in code. The service never fetches browser-provided URLs or forwards the Afterglow bearer credential upstream. Cloud deployment still requires authenticated service ingress, durable shared storage and a deliberate refresh owner. No cloud service has been deployed for this addition.

## Interview explanation

“I split external reference-data ingestion from the financial path. A source outage should not prevent a playback receipt from being accepted or settled. The catalog validates a complete replacement before publication, serves the last known good snapshot, and tells the caller when it is stale. The trade-off is eventual freshness and a second process to operate.”

In GoLand, place breakpoints in `Catalog.Refresh`, `parse`, and `AirportCatalog`. Compare this snapshot-refresh pattern with the receipt pipeline's transactional outbox: the consistency contract depends on the business operation.
