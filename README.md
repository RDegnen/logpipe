# LogPipe

LogPipe is a distributed log processing system built in Go.

This project exists to build real-world distributed systems intuition:

- Partitioning
- Consumer groups
- Offset commit correctness
- At-least-once delivery
- DLQs
- Crash recovery
- Replay
- Storage commit protocols

This is NOT an interview-prep project.

---

# Architecture Overview

Clients
↓
Ingest Gateway (HTTP)
↓
Kafka Topic: raw_logs.v1
↓
Processor (consumer group)
↓
Kafka Topic: processed_logs.v1.default
↓
Sink Writer
↓
MinIO (S3-compatible storage)

See:

- SYSTEM_ARCHITECTURE.md
- LOGPIPE_BUILD_PLAN.md
- RUNBOOK.md
- CLAUDE.md

---

# Prerequisites

- Go 1.21+
- Docker
- Docker Compose
- curl

---

# 1. Infrastructure Startup

From repo root:

make up

This starts:

- Redpanda (Kafka API compatible)
- MinIO (S3-compatible object store)

Verify Redpanda is up:

docker exec -it redpanda rpk cluster info

---

# 2. Create Kafka Topics

docker exec -it redpanda rpk topic create raw_logs.v1 -p 6
docker exec -it redpanda rpk topic create processed_logs.v1.default -p 6
docker exec -it redpanda rpk topic create dlq_logs.v1 -p 3

Verify:

docker exec -it redpanda rpk topic list

---

# 3. Run Services

Run each service in a separate terminal.

### Ingest Gateway

go run ./cmd/ingest-gateway

Runs on:
http://localhost:8082

---

### Processor

go run ./cmd/processor

Default consumer group:
processors-v1

---

### Sink Writer (if implemented)

go run ./cmd/sink-writer

---

# 4. Test Ingest

Create test file:

echo '{"level":"info","msg":"hello"}' > test.ndjson
echo '{"level":"error","msg":"boom"}' >> test.ndjson

Send request:

curl -X POST http://localhost:8082/ingest \
 -H 'Content-Type: application/x-ndjson' \
 -H 'X-Tenant-Id: t1' \
 -H 'X-Source-Service: api' \
 --data-binary @test.ndjson

Expected response:

{
"request_id": "...",
"accepted": 2,
"rejected": 0
}

---

# 5. Verify Kafka Flow

Check raw topic:

docker exec -it redpanda rpk topic consume raw_logs.v1 -n 5

Check processed topic:

docker exec -it redpanda rpk topic consume processed_logs.v1.default -n 5

Check DLQ:

docker exec -it redpanda rpk topic consume dlq_logs.v1 -n 5

---

# 6. Failure Testing

### Crash Processor

Ctrl+C processor
Restart it:

go run ./cmd/processor

Verify:

- No accepted messages lost
- Duplicates acceptable

---

### Broker Restart

docker restart redpanda

Verify:

- Gateway resumes producing
- Processor resumes consuming

---

# 7. Replay Procedure

Stop processor.

Reset offsets:

docker exec -it redpanda rpk group seek processors-v1 --to-start raw_logs.v1

Restart processor:

go run ./cmd/processor

---

# 8. Load Testing

Generate 1000 test logs:

seq 1 1000 | awk '{print "{\"level\":\"info\",\"n\":"$1"}"}' > test.ndjson

Send:

curl -X POST http://localhost:8082/ingest \
 -H 'Content-Type: application/x-ndjson' \
 -H 'X-Tenant-Id: t1' \
 -H 'X-Source-Service: api' \
 --data-binary @test.ndjson

Measure:

- accepted count
- latency
- processor lag:

docker exec -it redpanda rpk group describe processors-v1

---

# 9. Environment Variables

All services load from `.env` in the repo root.

KAFKA_BROKERS=localhost:9093
KAFKA_GROUP=processors-v1
RAW_LOGS_TOPIC=raw_logs.v1
PROCESSED_LOGS_TOPIC=processed_logs.v1.default
DLQ_TOPIC=dlq_logs.v1
COMMIT_EVERY=200
PORT=8082

---

# 10. Delivery Guarantees

Ingest:

- accepted == Kafka acked

Processor:

- At-least-once
- Offset commit after downstream produce
- DLQ for invalid events

Sink:

- Atomic file visibility (temp → rename)

System guarantees:

- No accepted message loss
- Duplicates possible

---

# 11. Development Guidelines

- Keep services minimal.
- Do not introduce heavy frameworks.
- Do not change offset semantics casually.
- Document architectural changes.

---

# 12. Reset Everything

docker compose -f infra/docker-compose.yml down -v

Recreate topics after reset.

---

# 13. Future Improvements

- Prometheus metrics
- Idempotency strategy
- Replay API
- Config-driven routing
- ClickHouse integration
- Multi-tenant throttling

---

# Project Philosophy

This repository is intentionally built to:

- Experience distributed system pain
- Learn offset correctness
- Learn replay semantics
- Understand partition scaling

It is not optimized for elegance.
It is optimized for learning.
