# Processor Walkthrough

**Date:** 2026-02-26
**File:** `cmd/processor/main.go`

---

## Topics Covered

### Offset Commits — What They Are and Why They Matter

- Kafka tracks a **cursor position** (offset) per partition per consumer group.
- `commitOffsets` tells Kafka: "I've handled everything up to here — start me from here if I restart."
- Commits use `offset + 1` because Kafka's contract is "start reading *from* the committed offset."

### At-Least-Once Delivery

- **Core rule:** Never commit an offset until the downstream produce (to `processed_logs` or DLQ) has been acknowledged.
- If you commit before the produce succeeds and then crash, that message is **lost**.
- If you produce successfully but crash before committing, the message gets **reprocessed** on restart — a duplicate, but not lost.
- This is the at-least-once tradeoff: no loss, but duplicates are possible.

### Why Auto-Commit Is Disabled

- Auto-commit would commit offsets on a timer regardless of whether processing finished.
- That creates the same race condition as committing before produce success — message loss.

### How Retries Work (Without a Retry Loop)

- `processOne` returns an error only on **transient** failures (e.g., broker unavailable).
- Permanent failures (bad JSON, validation errors) go to the DLQ and return `(true, nil)` — handled successfully.
- When a transient error occurs, the offset is not committed, so the next `PollFetches` delivers the record again.
- **Retry = don't commit + let the broker redeliver.** No explicit retry loop needed.

### Bug Found: Committing Past a Gap

- `EachRecord` uses a callback — `return` exits the callback, not the loop.
- If offset 11 fails but offset 12 succeeds, the commitMap would store offset 13, silently skipping 11.
- **That's message loss.**

### Fix Applied: Stop Processing on Error

- Added a `stopProcessing` flag that short-circuits the `EachRecord` callback after a transient failure.
- Only offsets up to the failure get committed.
- On next poll, the failed record is delivered again.
- Tradeoff: **head-of-line blocking** — one stuck record holds up the partition. Acceptable for v1, revisit if it becomes a bottleneck.

---

## Key Takeaways

1. Offset commits are your checkpoint. Withholding a commit is your retry mechanism.
2. At-least-once = never lose, accept duplicates. Dedup is the sink's job (Week 5).
3. Always think about what happens between "produce succeeds" and "commit succeeds" — that gap is where duplicates come from.
4. Always think about what happens when you commit — are you sure everything up to that point actually succeeded?
