# Milestone 19: reservation failure recovery and regression review

Fixed a locked-price failure after definitive reservation rejection. Shared request errors now preserve HTTP status; intake discards rejected plans for input/auth/not-found/conflict responses, allowing correction and a fresh request. Timeouts, server errors and rate limits retain the original key and price to avoid duplicate holds.

A recovered expired or released reservation no longer generates a misleading ready-to-send example. It unlocks the price and asks for a fresh load. Invalid prices now have adjacent text and aria-invalid/aria-describedby feedback. The pricing regression selects its own route so it also runs from Overview.

Validation:
- Go build, go vet, go test ./... passed.
- All 13 isolated API smoke checks passed.
- Browser suites passed: general UI (10), product UX (17), intake (8), all six example scenarios, pricing, standalone proof (14), assurance (8), inline assurance proof (7), Overview proof (7), new failure recovery (6).
- Failure injection covered budget rejection, lost response, server failure, expired recovery and accessible validation; existing tests verified successful real settlement and duplicate suppression.
- Mobile screenshot reviewed at 390px, no JavaScript errors in the verification session.

Hosted Actions run 35045467789 still rejected both jobs before starting due to account payments/spending limits. This is not a passing hosted CI result. No billing settings or CI checks were changed.
