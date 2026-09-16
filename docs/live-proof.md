# Live integration proof

Open `/#proof` in the local console. The run creates two one-cent synthetic reservations and exercises the existing Go APIs. It is a functional demonstration, not a capacity or availability benchmark.

## Verified sequence

1. Reserve two plays using stable request keys.
2. Submit four batch items: two valid events for play A, schema v99 for play B, and a malformed neighbor. Require HTTP 207, three accepted and one rejected.
3. Resubmit the exact batch. Require all three accepted IDs to replay unchanged.
4. Read each receipt by exact ID. Require one settlement and one duplicate for A; require B's unsupported receipt to be quarantined while its reservation remains held.
5. Submit a corrected schema v1 event for B with a new identity.
6. Verify B settled, A still has one settlement, the original v99 remains quarantined, and the two reservations each cost 10,000 USD micros.

The two events for A are concurrent work: either may win settlement. The verification intentionally does not assume ordering.

## Evidence and resumption

`GET /api/v1/deliveries/{id}` reads a receipt, its outbox state and optional reservation in a consistent read transaction. PostgreSQL uses repeatable-read isolation; SQLite uses its read snapshot. Authentication binds the tenant. An inaccessible receipt has the same 404 as a nonexistent one. Exact reads do not depend on the newest-100 dashboard window.

Outbox acknowledgement can lag the financial decision. A settled receipt with pending dispatch is not proof of another charge. The endpoint deliberately omits internal lease-owner and credential information.

Browser session storage saves the plan before mutations and verified stages afterward. Reloading the same tab allows resumption with the original keys and payloads. A lost response is retried as the same logical request. Closing the browser session can discard that resume state; the server's accepted records remain durable. JSON export includes the observed records, timestamps, transport, results and scope. It is not a signed audit artifact.

Runs do not inject global worker faults or reset the database. A paused worker causes a bounded wait and a resumable message. Run one scenario at a time. Reservations still obey the existing expiry rules; this does not bypass stale evidence validation.

## Separate study material

`docs/interview-notes.html` remains a standalone local HTML document. It is no longer embedded, served, or linked in product navigation. `/interview-notes.html` returns 404.
