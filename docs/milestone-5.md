# Fix retry behavior and improve navigation

Retrying a reservation after an ambiguous response previously generated another idempotency key, allowing a second hold. The open form now retains a key for the same normalized payload; changed input starts a different operation. This identity is scoped to the open form, not persisted across page reloads. Errors are shown inside the dialog, decimal input validation is corrected, and requests have a bounded client timeout. A completed request cannot close a different dialog opened while it was pending.

Automatic refreshes preserve focused controls and keep search results fresh. Invalid URL fragments, including object-prototype names, fall back to Overview. Search misses have a specific empty state; pending scenario controls show their disabled state. Dialogs have accessible names, current navigation is announced, and the skip link preserves the selected view.

The interview guide gains persistent section navigation, a rehearsal shortcut, back-to-top access and keyboard-scrollable tables. The app's guide link is available on mobile and the sidebar can scroll on short screens. The guide remains standalone HTML without JavaScript or external assets. Its formatting was expanded for maintainability.

## Validation

- Go tests, vet and build passed locally.
- Ten browser regression checks passed, including mocked lost-response retries, changed-input identity, decimal validation, visible errors, dialog replacement during a pending request, focus across polling, empty search, invalid routes and skip navigation. The reservation responses in these browser checks were mocked; they did not modify the demo database.
- All 13 HTTP/SQL/worker smoke checks passed against an isolated database, with the expected final spend of 43,650,000 micros.
- Desktop and 390px mobile layouts inspected; no page overflow or browser errors observed. Guide anchors and offline file opening checked.
- GitHub Actions is still subject to the previously observed account billing/spending-limit restriction. No billing changes were made.

Browser regression runner: open the local demo with agent-browser, then pass `scripts/ui-regression.js` to `agent-browser --session <session> eval --stdin`. Run it in a dedicated test browser session.
