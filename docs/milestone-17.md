# Milestone 17: working partner intake examples

Load example now prepares its own one-cent synthetic reservation instead of requiring an existing hold. Six scenarios cover valid playback, mixed acceptance, multiple event IDs for one play, identical event replay, unsupported schema and playback outside the reservation window. The UI explains expected outcomes and keeps sending explicit.

Repeated loads reuse the unsent hold. Reservation requests retain their idempotency key after an uncertain response. Once a batch is submitted, a later example gets a new reservation; unchanged batch retries preserve their original payload. Scenario selection leaves the editor intact and explicitly asks the user to load the new scenario.

Validation: eight existing intake browser checks passed. New browser regression verified all six scenarios against the real Go API and worker, repeated-load hold reuse and recovery from a simulated lost committed reservation response. Browser screenshot reviewed; no JavaScript errors. Go build and tests passed.

Scope: synthetic local demo data. Loading reserves budget; sending can settle it. The example reservation retry plan survives navigation within the page, not a browser reload. Hosted Actions remain subject to the previously documented account billing restriction.
