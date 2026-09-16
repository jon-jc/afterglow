# Milestone 12: product usability

- Reordered navigation around campaign operations; engineering demonstration tools are grouped separately.
- Added a searchable page switcher with Ctrl/Cmd+K, arrow navigation, Enter selection, native modal focus management, and full mobile labels.
- Increased reading sizes and contrast, refined spacing and panel surfaces, and added a contextual overview next step.
- Added 20-row receipt pagination, newest/oldest ordering, reason search, clear filters, per-decision counts scoped to the latest 100 receipts, and export of matching records with explicit scope.
- Recovery separates invalid evidence from operational failures and displays the reason inline. Receipt tables become labeled cards on phones. Navigation stays available in the sticky header.
- Added pause/resume for background snapshots, manual refresh while paused, last-update timestamps and an explicit stale-data banner on connection loss. Pausing the view does not pause the worker.
- Study material remains outside the application.

Validation: Go embedded application build passed. 49 browser checks passed (17 new product UX checks, 10 existing UI checks, 8 intake checks, 14 live-proof checks). Reviewed overview, recovery, mobile navigation and mobile receipt cards. A clean browser session reported no JavaScript errors. Existing regression selectors now target the receipt dialog explicitly because the app also has a navigation dialog.

No financial engine or database schema changes. UI search, counts, sorting, pagination and export operate within the latest 100-receipt snapshot; they do not claim full-history coverage.
