# Add a Go airport catalog microservice and explorer

Afterglow can now explore all source-listed US scheduled-service airports, including territories, instead of only the eight fictional advertising screens. A separate Go process fetches OurAirports public-domain CSV data, validates records, persists a last-good snapshot and refreshes daily. Failed source refreshes retain existing data, surface a stale warning and retry after five minutes.

The operations API uses a fixed upstream with bounded timeouts and response size; it does not forward bearer credentials. Financial APIs continue independently if the airport service is unavailable. The service is loopback-only pending a deliberate authenticated cloud deployment.

The explorer plots every matching airport and provides search, region filters, paginated rows, source details and freshness. Mainland, Alaska, Hawaii and territories use explicitly marked separate scales. The first live fetch selected 720 records. This is the source's scheduled-service definition, not an FAA certification list, CCO advertising inventory, live flight feed or passenger-volume estimate.

Adds a GoLand run configuration, Windows launch script, architecture/operations documentation, API documentation and interview notes for both the airport service and milestone 6 batch intake. Corrects inline YAML description quoting in the batch API contract.

## Validation

- Actual OurAirports source download succeeded; live service API returned 720 records.
- Go tests and vet passed. Race tests passed on Linux for airport and HTTP packages.
- Import tests cover coverage filtering, invalid coordinates, duplicates, failed refresh retaining the in-memory/disk snapshot, restart loading, successful recovery, search, pagination and empty-state availability.
- Proxy tests cover a fixed path, allowed query parameters, credential isolation and unavailable-service responses.
- Six read-only browser checks passed: full-map coverage, pagination, regional filtering, preserved search focus, airport detail and empty results. Desktop and 390px mobile reviewed with no horizontal page overflow or browser errors.
- All 13 isolated financial pipeline smoke checks passed, final spend 43,650,000 micros.
- OpenAPI lint passed with the existing advisory license/health-response exceptions.

GitHub Actions has the previously observed account billing/spending-limit restriction. No billing settings or cloud deployments were changed.
