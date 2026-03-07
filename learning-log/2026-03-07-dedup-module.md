# 2026-03-07 – Dedup Module (Week 6)

## What We Covered

### Dedup module design
- Created `cmd/sink-writer/dedup.go` with a `Dedup` struct holding `map[string]bool`.
- `IsDuplicate(eventId)` — checks membership in the seen set. Learned that a Go map lookup on a missing key returns the zero value (`false`), so `return d.SeenEventsMap[eventId]` is sufficient.
- `Mark(eventId)` — records an event_id after successful write.

### Bootstrap strategy
- On startup, the sink writer must rebuild the in-memory set from the `seen_events.v1` compacted topic.
- Key insight: snapshot the **high water marks** (end offsets) for each partition *before* starting to consume. This gives a concrete finish line — consume from the beginning until you reach those offsets, then bootstrap is done.
- Why not timeout-based ("consume until nothing comes back"): new messages keep arriving, and network delays don't mean you're caught up. A snapshotted offset is a deterministic stop point.

### No consumer group for bootstrap
- Consumer groups track committed offsets so the consumer resumes where it left off. That's wrong for bootstrap — you need to read from the beginning every time.
- Use `kgo.ConsumeResetOffset(kgo.NewOffset().AtStart())` without a consumer group.
- Consumer groups will be covered when building the main sink writer consume loop.

### Client ownership
- `Bootstrap` should receive the Kafka client and context as parameters, not create them internally. `main.go` owns the client lifecycle.
- Go convention: `context.Context` is always the first parameter.
- Topic name passed as a parameter too — Bootstrap doesn't load config.

### franz-go admin client
- `kadm.NewClient(kafkaClient)` wraps a `kgo.Client` with admin operations.
- `admin.ListEndOffsets(ctx, "seen_events.v1")` returns the high water mark per partition — this is the bootstrap finish line.
- End offset = next offset that will be written. A partition with 5 messages (offsets 0–4) has end offset 5. Empty partition has end offset 0.

### Unbounded memory
- The `map[string]bool` grows forever. In practice, duplicates only come from crash-and-replay, so the replay window is short (minutes).
- Future fix: bounded window (evict entries older than N hours). Not needed yet — revisit when observability is in place and memory can be measured.

### Bootstrap loop implementation
- Build a `trackingMap` (`map[int32]int64`) from end offsets, skipping partitions with offset 0 (empty — nothing to consume).
- Loop while `len(trackingMap) > 0`: call `PollFetches` each iteration for a fresh batch, `Mark` each record's key, delete from tracking map when `record.Offset >= highWaterMark - 1`.
- `PollFetches` must be **inside** the loop — calling it once outside just replays the same batch forever.
- `EachRecord`'s callback `return` only exits that one callback invocation, not the outer loop.

### Error handling
- `ListEndOffsets` failure: `log.Fatalf` — can't bootstrap without offsets, running without the seen set would silently allow duplicates.
- Context cancellation: check `ctx.Err() != nil` at the top of each loop iteration, `Fatalf` if cancelled — bootstrap is a startup requirement, not optional.
- Fetch errors: log and continue — transient errors are expected.

## Where to Pick Up Next
- Wire Bootstrap into the sink writer's `main.go` startup path.
- Create the Kafka client for bootstrap (consume `seen_events.v1` from start, no consumer group).
- After bootstrap completes, start the main sink writer consume loop.
