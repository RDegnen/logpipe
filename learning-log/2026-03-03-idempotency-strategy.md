# 2026-03-03 – Idempotency Strategy (Week 5)

## What We Covered

### Why idempotency matters
- The processor has at-least-once delivery: if it crashes between producing downstream and committing offsets, it replays records on restart.
- Duplicates end up in `processed_logs.v1.default`, which the sink writer will consume.
- Without dedup, crash-and-replay cycles cause unbounded duplication in object storage.

### Where dedup belongs
- The processor's job is to process and pass messages on — it stays simple.
- The **sink writer** is responsible for dedup before writing to storage.
- This keeps separation of concerns clean.

### Chose compacted Kafka topic approach
Evaluated three options:

| Option | Survives restart | New infra | Complexity |
|--------|-----------------|-----------|------------|
| Compacted topic | Yes | No | Medium |
| Postgres unique constraint | Yes | Yes | Low conceptually, high operationally |
| In-memory window | No | No | Very low |

Chose **compacted topic** (`seen_events.v1`) because:
- Teaches Kafka compaction — a concept worth learning directly.
- No new infrastructure beyond what we already run.
- Provides bounded (not perfect) dedup, which matches our delivery guarantees.

### How the pattern works
1. **On startup**: sink writer consumes `seen_events.v1` from beginning to end, building an in-memory set of seen `event_id`s (bootstrap step).
2. **At runtime**: for each record from `processed_logs.v1.default`, check the in-memory set. Skip if seen.
3. **After writing**: produce the `event_id` to `seen_events.v1` so it survives restarts.

Key insight: the compacted topic is a **durable backup** of the in-memory set, not a queryable database. You materialize it into memory on startup.

### Infrastructure changes
- Created `scripts/create-topics.sh` — idempotent script that creates all topics including `seen_events.v1` with `cleanup.policy=compact`.
- Added `make topics` target.
- Updated README section 2.

## Where to Pick Up Next
- Start building the **sink writer** (week 6) — it consumes `processed_logs.v1.default`, buffers by tenant + hour, and writes to MinIO.
- The dedup logic (bootstrap consumer + in-memory set + produce-on-write) gets built into the sink writer.
- Alternatively, could write the dedup module in isolation first and integrate it into the sink writer after.
