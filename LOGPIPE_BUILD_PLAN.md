# LogPipe – Distributed Log Processing System

Author: Ross Degnen  
Status: Active Build  
Primary Language: Go  
Broker: Redpanda (Kafka API compatible)  
Storage: MinIO (S3-compatible)  
Goal: Build a production-shaped distributed log processor to gain practical system design experience.

---

# 1. High-Level Objective

LogPipe is a distributed log processing pipeline with the following flow:

    HTTP Ingest Gateway
            ↓
        raw_logs.v1 (Kafka topic)
            ↓
        Processor Service
            ↓
    processed_logs.v1.<route>
            ↓
        Sink Writer
            ↓
    Object Storage (MinIO / S3)

Primary learning goals:

- Partitioning & key strategy
- Consumer group semantics
- Offset commit correctness
- At-least-once delivery
- Idempotent sinks
- Failure recovery
- Observability
- Replay and reprocessing

This is NOT an interview-prep project.
This is a practical distributed systems training system.

---

# 2. Core Technologies

## Language

- Go (all services)

## Messaging

- Redpanda (Kafka protocol compatible)
- Topics:
  - raw_logs.v1
  - processed_logs.v1.default
  - dlq_logs.v1

## Storage

- MinIO (S3-compatible object store)
- Parquet files (later stage)

## Libraries

- franz-go (kgo) for Kafka client
- chi for HTTP routing
- ULID for time-sortable IDs
- Standard library for most logic

---

# 3. Log Event Schema (v1)

Envelope:

{
"event_id": "ulid",
"tenant_id": "string",
"source": {
"service": "string",
"instance": "string?",
"env": "dev|staging|prod?",
"region": "string?"
},
"observed_at": "RFC3339 timestamp",
"emitted_at": "RFC3339 timestamp?",
"level": "debug|info|warn|error",
"trace": {
"trace_id": "string?",
"span_id": "string?",
"request_id": "string?"
},
"schema_version": 1,
"body": { arbitrary JSON }
}

Key = tenant_id  
Partitioning strategy = per-tenant ordering guarantee

---

# 4. Delivery Semantics (Important)

## Ingest Gateway

- "accepted" = message successfully produced to Kafka with acks=all

## Processor

- At-least-once semantics
- Offset committed ONLY after:
  - Successful produce to processed topic
    OR
  - Successful produce to DLQ

## DLQ

- Used for:
  - Envelope unmarshal failures
  - Schema validation failures
  - Marshal errors
- DLQ events must contain original record and metadata

Duplicates are allowed.
Loss of accepted events is NOT allowed.

---

# 5. 12-Week Build Plan

---

## Week 1 – Infrastructure Setup

Goals:

- Docker compose: Redpanda + MinIO
- Topic creation
- Simple producer + consumer smoke test
- Makefile automation

Deliverable:

- Working raw_logs.v1 topic
- Verified produce + consume

---

## Week 2 – Ingest Gateway (MVP)

Implement:

- POST /ingest
- application/x-ndjson support
- Optional gzip support
- ULID event_id generation
- Sync Kafka produce
- Basic validation (level required)
- Response counts: accepted / rejected

Success Criteria:

- NDJSON batch ingestion works
- accepted == Kafka acked
- Key = tenant_id

---

## Week 3 – Partitioning & Backpressure

Implement:

- Token bucket per tenant (simple in-memory)
- 429 when overloaded
- Increase topic partitions
- Validate ordering guarantees

Learning Focus:

- Partition scaling
- Backpressure patterns

---

## Week 4 – Processor v1

Implement:

- Consumer group processors-v1
- Manual offset commit
- Validate envelope
- Produce to processed_logs.v1.default
- DLQ routing
- Commit only after successful downstream produce

Success Criteria:

- raw_logs → processed_logs
- Poison messages → dlq_logs
- Crash does not lose accepted events

---

## Week 5 – Idempotency Strategy

Choose one:

1. Compact Kafka topic storing seen event_ids
2. Postgres unique constraint on event_id
3. In-memory dedupe window (learning version)

Goal:

- Crash + replay does not create unbounded duplication in sink

---

## Week 6 – Sink Writer v1 (MinIO + Parquet)

Implement:

- Consume processed topic
- Buffer by tenant + hour
- Write Parquet files
- Rolling by size/time
- Temp file + atomic rename commit pattern

Success Criteria:

- Files appear under:
  /tenant=<id>/dt=YYYY-MM-DD/hour=HH/

---

## Week 7 – Observability

Add:

- Structured logs
- Prometheus metrics endpoint
- Metrics:
  - ingest_rate
  - processing_rate
  - consumer_lag
  - dlq_rate
  - sink_failures

Goal:

- Be able to reason about system health

---

## Week 8 – Routing Rules

Implement:

- Config-driven routing
- processed_logs.v1.<route>
- Dynamic reload of rules

Learning:

- Config consistency
- Multi-topic fanout

---

## Week 9 – Failure Injection

Simulate:

- Kill processor mid-batch
- Kill broker
- Add network delay
- Replay duplicates

Validate invariants:

- No accepted message lost
- Duplicates acceptable but bounded

---

## Week 10 – Replay / Reprocessing

Implement:

- Offset rewind by timestamp
- DLQ re-drive
- Schema version compatibility handling

---

## Week 11 – Query Layer (Optional but Valuable)

Options:

- Simple object listing API
- ClickHouse ingestion
- OpenSearch indexing

Purpose:

- Force storage design tradeoffs

---

## Week 12 – Performance & Documentation

Add:

- Load test harness
- Throughput measurement
- Bottleneck analysis
- Scaling plan
- Operational runbook

Final deliverable:

- Architecture doc
- Failure model doc
- Scaling model doc

---

# 6. Non-Goals

- Perfect exactly-once semantics
- Multi-region replication
- Full schema registry (v1)

---

# 7. Definition of Done (System Level)

The system is considered complete when:

- Ingest → Process → Sink works end-to-end
- Crash scenarios tested
- Reprocessing supported
- Observability implemented
- Storage layout documented
- Tradeoffs documented

---

# 8. Agent Instructions

When modifying this repository:

- Do NOT introduce frameworks unless necessary.
- Maintain at-least-once semantics.
- Offset commits must remain explicit.
- Do not silently change delivery guarantees.
- Keep services small and independent.
- Prefer clarity over abstraction.

Before major changes:

- Update this file.
