# 2026-03-25 – First E2E Run and Sink Writer Dedup Bugs

## What We Covered

### End-to-end test run
First time running the full pipeline: ingest gateway → Kafka (raw) → processor → Kafka (processed) → sink writer → Postgres. Hit two bugs along the way: messages not flowing to the processed topic, and only 2 out of 4 messages landing in Postgres.

---

### Bug 1: Messages not appearing in processed topic

**Symptoms:**
- `rpk topic consume raw_logs.v1` showed messages.
- `rpk topic consume processed_logs.v1.default` showed nothing.
- DLQ was empty.
- `rpk group describe processors-v1` showed `TOTAL-LAG=2`, `MEMBERS=2`, `STATE=Stable`.
- Processor terminal showed no output after the startup line.

**Diagnosis:**
- `MEMBERS=2` was the key clue — only one processor should have been running.
- `lsof -i :19093` revealed two processor PIDs connected to Redpanda.
- With two members, Kafka's cooperative-sticky balancer split the 6 partitions between them. All messages landed on partition 1 (keyed by tenant ID `t1`). Partition 1 was assigned to the *other* member.
- The visible processor had partitions with no data, so `PollFetches` blocked forever waiting for records that would never arrive on its assigned partitions.

**Key lessons:**
- Consumer group members each own a subset of partitions. If your data is on a partition you don't own, you never see it.
- `CURRENT-OFFSET = -` means no offset has ever been committed for that partition — not the same as lag=0.
- `PollFetches` blocks during rebalance and also blocks indefinitely if there's nothing on your assigned partitions.
- Silence in the processor terminal is not always healthy — add a "processed N records" log if you want visibility.
- `lsof -i :<port>` is a useful tool for spotting stale or duplicate processes.

**Fix:** Kill all processor instances, verify with `ps aux | grep processor`, restart a single one.

---

### Bug 2: Only 2 of 4 messages in Postgres

**Symptoms:**
- Processed topic had all 4 messages.
- `SELECT * FROM log_events` returned only 2 rows.

**Root cause: dedup key was tenant ID, not event ID.**

The sink writer's dedup check:
```go
if dedup.IsDuplicate(string(rec.Key)) {
```
`rec.Key` is the Kafka partition key — the tenant ID (`t1`). After the first message was processed and `t1` was marked as seen, every subsequent message from the same tenant was silently dropped as a "duplicate".

**Fix:** Unmarshal the record value first, then dedup on `evt.EventID`:
```go
var evt logevent.LogEvent
if err := json.Unmarshal(rec.Value, &evt); err != nil { ... }

if dedup.IsDuplicate(evt.EventID) { return }
```

---

### Bug 3: Bootstrap dedup was also broken

**Root cause:** `produceSeenEvent` wrote records to `seen_events.v1` with no `Value`:
```go
seenRec := &kgo.Record{
    Topic: cfg.SeenEventsTopic,
    Key:   rec.Key,
    // Value: nil
}
```
On restart, `Bootstrap` tried to unmarshal `record.Value` to get the event ID — but it was always nil. The unmarshal failed every time, so the in-memory dedup map was never rebuilt. Events already written to Postgres could be re-inserted after a restart.

**Fix:** Pass the full processed event as the value:
```go
seenRec := &kgo.Record{
    Topic: cfg.SeenEventsTopic,
    Key:   []byte(eventId),
    Value: rec.Value,
}
```

---

### Bug 4: Compaction key was tenant ID

**Root cause:** `seen_events.v1` is a compacted topic. Log compaction keeps only the latest record per key. With tenant ID as the key, compaction would collapse all events for a tenant down to one record — the most recent one. After compaction, bootstrap could only reconstruct one event ID per tenant, allowing all older events to be re-inserted.

**Fix:** Use event ID as the key. Each event gets its own compaction slot, so the full seen set survives compaction and restarts.

---

## Files Changed
- `cmd/sink-writer/main.go` — unmarshal before dedup, use `evt.EventID`, pass `eventId` to `produceSeenEvent`
- `cmd/sink-writer/dedup.go` — unmarshal record value in `Bootstrap`, mark on `evt.EventID`

## Where to Pick Up Next
- Failure injection: crash the processor mid-batch and verify no message loss.
- Crash recovery: restart the sink writer and verify the bootstrap dedup prevents duplicate DB inserts.
