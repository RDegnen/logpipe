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

### franz-go admin client
- `kadm.NewClient(kafkaClient)` wraps a `kgo.Client` with admin operations.
- `admin.ListEndOffsets(ctx, "seen_events.v1")` returns the high water mark per partition — this is the bootstrap finish line.

### Unbounded memory
- The `map[string]bool` grows forever. In practice, duplicates only come from crash-and-replay, so the replay window is short (minutes).
- Future fix: bounded window (evict entries older than N hours). Not needed yet — revisit when observability is in place and memory can be measured.

## Where to Pick Up Next
- Update `Bootstrap` method signature to `(ctx context.Context, kafkaClient *kgo.Client)`.
- Remove config loading, client creation, producer options, and signal handling from `Bootstrap`.
- Implement the full bootstrap loop:
  1. Snapshot end offsets with `kadm.ListEndOffsets`.
  2. Consume from the beginning, adding event_ids to the map.
  3. Track progress per partition, break when all partitions reach their target offsets.
- After bootstrap works, integrate it into the sink writer's startup path.
