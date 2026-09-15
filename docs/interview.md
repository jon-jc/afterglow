# Tomorrow's interview: explain the system, then break it

## A 45-second introduction

“I built Afterglow to explore the reliability problems around digital out-of-home delivery. A campaign reserves budget for a screen, a playback receipt arrives asynchronously, and the system decides whether that evidence can settle the reservation. The interesting part is what happens when receipts are duplicated, workers crash, or a partner changes its schema. It uses Go, SQL transactions, a transactional outbox and a GCP Pub/Sub adapter. I also built a small operations console so I can demonstrate those failure cases instead of just describing them. The inventory is synthetic and this is not a certified Vistar connector.”

Use that only after walking through the code yourself. Describe this as a recent engineering demo, not commercial production experience. AI-assisted development is compatible with ownership when you can explain the design, review the code and show meaningful verification.

## A five-minute demonstration

1. **Overview:** explain that a proof of play is evidence that an ad ran. It is not a verified viewer impression or a conversion. Point out that the console explicitly distinguishes those ideas.
2. **Simulate traffic:** 12 reservations and receipts move through the real local database and worker. Held budget becomes settled spend. The local run uses SQLite; the cloud path uses PostgreSQL and Pub/Sub.
3. **Failure lab → Duplicate delivery:** five different event IDs refer to one reservation. One charges; four are suppressed. Message-level deduplication alone would miss this case.
4. **Failure lab → Crash after commit:** let the queue empty first. Lose the acknowledgement after commit. The delivery is retried, its attempts increase, and the charge remains unchanged.
5. **Failure lab → Unsupported schema:** inspect the quarantined receipt. The payload is preserved and the campaign is not charged. Replay validates the same evidence; it does not bypass the error.
6. **Campaigns:** show a held reservation, its immutable price and the remaining budget. Mention the concurrent budget test and explain its invariant.

If time is short, do steps 1–3 and pause for questions. The strongest demo is the one the interviewer can follow.

## Three examples worth preparing

### 1. Correctness under ambiguous delivery

**Problem:** A worker can commit a charge and die before acknowledging. The broker must redeliver, but billing must not repeat.

**Decision:** Make settlement a state transition on the reservation. The consumer locks the receipt and reservation and commits the state, spend and audit in one transaction. A second event ID for the same reservation is still harmless.

**Trade-off:** This is exactly-once **financial effect for a reservation**, not a claim of exactly-once message delivery. It relies on a trusted reservation ID and durable database constraints. A real integration must establish that identity contract with the partner.

**Evidence:** `TestConcurrentDuplicateReceiptsChargeOnce`, `TestCommitBeforeAckSurvivesRedelivery`, and `TestAuditWriteFailureRollsBackCharge`.

**Read:** [Process](../internal/core/store.go), [worker acknowledgement boundary](../internal/pipeline/worker.go).

### 2. Atomic API acceptance without a distributed transaction

**Problem:** Writing a receipt to SQL and publishing a message are two independent operations. A crash between them can strand the receipt or publish work that was never committed.

**Decision:** Commit the receipt and an outbox row together. A dispatcher later leases that row and publishes it. The HTTP contract says 202 means database acceptance, not completed billing or broker acknowledgement.

**Trade-off:** Eventual processing latency and an outbox table to operate. Publishing may happen twice if the dispatcher dies after the broker accepts but before its SQL update. The consumer must therefore remain idempotent.

**Evidence:** Atomic-outbox, lease-expiry and stale-owner tests; end-to-end pause/resume scenario.

**Read:** [Accept](../internal/core/store.go), [Claim and owner fencing](../internal/core/queue.go), [Pub/Sub publisher](../internal/pipeline/pubsub.go).

### 3. A hard budget cap under concurrency

**Problem:** Independent API instances can both read the same available budget and overspend it.

**Decision:** Serialize allocation on the campaign row and conditionally update its held amount. A database check constraint provides a second guard. Money uses integer micros; the receipt cannot choose its price.

**Trade-off:** A hot campaign becomes a contention point. That is a deliberate correctness choice. At higher scale I would measure lock contention before considering reservation buckets or partitioned budget allocation, because those change the overspend/availability trade-off.

**Evidence:** 80 concurrent $10 reservations against a $250 campaign admit exactly 25. The test passes on real PostgreSQL with the race detector enabled.

**Read:** [Reserve](../internal/core/store.go), [database constraints](../internal/core/schema.sql), [concurrency test](../internal/core/store_test.go).

## Go and GCP fundamentals to be ready for

- **Go contexts:** HTTP/database calls have deadlines; cancellation stops unnecessary work. A lease outlives the dispatch timeout so another worker does not immediately overlap a healthy dispatch.
- **Goroutines:** the local loop intentionally processes one dispatch at a time. Pub/Sub receive concurrency is bounded. PostgreSQL leases allow independent dispatcher replicas. Do not describe the local loop as a high-throughput benchmark.
- **Pub/Sub acknowledgement:** acknowledge after the durable decision. Transient database failure means retry. Permanent invalid evidence means persist quarantine, then acknowledge. A managed transport DLQ is separate from the application's recovery queue.
- **Event time:** validate when the ad played, not only when the server received the receipt. Hold funds through a documented receipt grace period. After release, late evidence needs reconciliation; it cannot silently reclaim money.
- **Observability:** bounded metric labels, transaction duration, persisted audit reasons, structured retry logs, and trace context carried through the outbox. Do not put request IDs in metric labels.
- **Cloud Run:** a pull worker needs CPU outside HTTP requests and a minimum live instance. API instances can scale differently. Persistent state belongs in SQL, not a local container disk.

## Ownership and teamwork

Prepare a **real example from your own experience** for each of these; this repository cannot supply a genuine team history:

- A code review where you explained a risk and helped agree on a simpler change.
- A problem you owned from reproduction through verification and documentation.
- A time you changed your mind after feedback or helped someone understand a difficult issue.

For this demo, a concrete engineering-review example is the replay/completion race: an old worker must not erase a newly queued replay. The fix uses consistent lock ordering and checks the delivery state under lock. You can explain that review without inventing a colleague or a production incident.

## Four questions for Stanford

1. “For the Vistar integration, which boundaries does this team own—campaign booking, inventory synchronization, ad serving, proof-of-play reconciliation, or some combination?”
2. “Where do Pub/Sub events enter the workflow today, and which failures or replay cases take the most engineering time?”
3. “What would make you say the successful person had had a strong first six months—shipping an integration milestone, improving reliability, or taking ownership of a service?”
4. “How do you review AI-assisted changes, and what do strong code reviews and learning from feedback look like on this team?”

## Tonight: 30-minute preparation

- **10 minutes:** read `Reserve`, `Accept`, `Process`, and the outbox claim/complete methods. Draw the transaction boundaries yourself.
- **10 minutes:** run duplicate, crash, quarantine and replay scenarios. Explain why each balance changes or does not change.
- **5 minutes:** read the production limits in [operations](operations.md). Be comfortable saying what you would investigate next.
- **5 minutes:** practice the introduction and prepare one genuine teamwork story. Keep the first answer short; let Stanford choose the technical depth.
