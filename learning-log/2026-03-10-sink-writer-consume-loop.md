# 2026-03-10 – Sink Writer Consume Loop

## What We Covered

### Consumer groups
- A **consumer group** is a named logical consumer. Kafka tracks committed offsets per group.
- Multiple instances with the same group ID share the work — Kafka assigns each partition to exactly one member. This is how you scale consumers horizontally.
- Different groups are independent — processor and sink writer each track their own position, even on the same topic.
- Scaling is tied to partition count: more instances than partitions means some sit idle.

### Two Kafka clients in the sink writer
- Can't reuse the bootstrap client for the main loop. `ConsumeTopics` and consumer group are set at client creation time.
- Bootstrap client: consume `seen_events.v1` from start, no group. Closed explicitly after bootstrap.
- Main client: consume `processed_logs.v1` with group `sink-writers-v1`, manual commits, producer capability.

### Consume loop structure
- Follows the processor's pattern: `PollFetches` in a `for ctx.Err() == nil` loop, `EachRecord` callback, `commitMap` for manual offset tracking, `stopProcessing` flag for transient errors.
- Per record: check `IsDuplicate` (skip if seen), placeholder DB write, produce event ID to `seen_events.v1`, then `Mark` in dedup set and update commit map.
- `dedup.Mark` happens **after** successful produce — if produce fails, the event isn't marked, so it'll be retried.

### Producing to seen_events.v1
- Only the **key** (event ID) matters. Bootstrap reads keys to rebuild the seen set. No need for a value payload.
- Initial attempt had a `sinkMessage` struct with base64-encoded data — unnecessary complexity. Simplified to just a record with a key.

### Shared utilities
- Extracted `ProduceSync` and `CommitOffsets` into `internal/utilities/` since both the processor and sink writer need them.
- Updated processor to use shared utilities. Renamed `KafkaGroup` to `KafkaProcessorGroup`, added `KafkaSinkGroup`.

## Where to Pick Up Next
- Replace the DB write placeholder with an actual database write (Postgres or similar).
- Add a `RequiredAcks(kgo.AllISRAcks())` to the sink writer client for durability.
- Consider adding a startup log line (like the processor has) for operational visibility.
- End-to-end test: run ingest -> processor -> sink writer and verify the full pipeline.
