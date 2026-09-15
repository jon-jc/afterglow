# Architecture decisions

## The business invariant

`spent + reserved <= budget`, all in integer USD micros. A micros value of 1,000,000 is one dollar. No floating point money. A playback receipt cannot choose its price: settlement reads the immutable reservation price.

## Reservation lifecycle

`held -> settled` on a matching proof of play, or `held -> released` after a 15-minute play window and 15-minute receipt grace period. Terminal states never go backwards. Late evidence after release is quarantined for review; it cannot consume funds already reallocated to another play.

The campaign row is locked before reservation budget is inspected. A conditional update and a database check constraint independently enforce the cap. Database uniqueness scopes idempotency to the tenant. A reused key with different content is a conflict.

## Receipt lifecycle

An HTTP 202 means the receipt **and outbox intent committed to the database**. It does not claim the broker already has it, or that billing succeeded. The asynchronous consumer validates the version, display, play duration and event-time window. It locks the delivery and reservation, then atomically writes the reservation transition, spend and audit decision. A duplicate broker delivery is harmless; a new event ID for an already settled reservation is also harmless.

The receipt contract is synthetic. Vistar's real ad-serving contract and certification process require a separate adapter and vendor testing; this project does not claim compatibility.

## Portable execution

SQLite with WAL and FULL synchronous writes is the zero-dependency local demo. Its single database connection serializes local operations. PostgreSQL uses row locks and a connection pool; CI runs the same business-invariant suite on PostgreSQL. SQLite results are not evidence of multi-instance database concurrency or GCP durability.

## Tenant boundary

The configured API credential selects the tenant; request bodies cannot select it. This version runs one configured tenant per API deployment. SQL keys, uniqueness constraints and all private reads include that tenant. Screen inventory is shared reference data. Multi-tenant credential provisioning and rotating secrets are deployment work, not UI controls.
