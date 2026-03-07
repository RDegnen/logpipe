# 2026-03-07 – Sink Writer Startup Wiring

## What We Covered

### Two-phase startup
- The sink writer needs two Kafka clients with different configurations — can't reuse one.
- Bootstrap client: consume `seen_events.v1` from start, no consumer group, read-only.
- Main client (not yet built): consume `processed_logs.v1` with a consumer group, produce to `seen_events.v1`.
- `ConsumeTopics` and consumer group are set at client creation time, so a single client can't serve both roles.

### Bootstrap wiring in main.go
- Load config, create bootstrap client with `kgo.ConsumeResetOffset(kgo.NewOffset().AtStart())`.
- Construct `Dedup` with an initialized map, call `Bootstrap`.
- Close the bootstrap client explicitly after bootstrap (not `defer`) — it's no longer needed and the main client hasn't been created yet.

### `ConsumeResetOffset` default
- franz-go defaults to consuming from the **end** (latest). Bootstrap needs `AtStart()` to read the full topic.

## Where to Pick Up Next
- Create the main Kafka client in `main.go`:
  - Consumer group, consume `processed_logs.v1`, disable auto commit, produce capability.
- Build the main consume loop:
  - Fetch records, check dedup, (placeholder for DB write), produce to `seen_events.v1`, manual offset commit.
  - Follow the processor's pattern for manual commit and error handling.
- DB write is a future piece — loop can log what it would write for now.
