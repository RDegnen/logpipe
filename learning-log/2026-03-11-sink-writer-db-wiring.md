# 2026-03-11 – Sink Writer DB Wiring

## What We Covered

### Why a database at all
- Kafka is a transport, not a queryable store. The sink writer's job is to persist processed log events into Postgres so they can be queried (e.g. "all ERROR logs for tenant X in the last hour").
- MinIO is in the compose file but not wired yet — that's future cold storage / archival, not the primary store.

### Table schema design
- Columns: `event_id` (TEXT PRIMARY KEY), `tenant_id` (TEXT), `observed_at` (TIMESTAMPTZ), `level` (TEXT), `body` (JSONB), `inserted_at` (TIMESTAMPTZ DEFAULT now())
- `body` is JSONB — binary JSON, queryable with `->` and `->>` operators, indexable.
- `inserted_at` vs `observed_at`: observed_at is when the event happened in the source system; inserted_at is when we wrote it to the DB. Useful for diagnosing pipeline lag.
- Composite index on `(tenant_id, observed_at DESC)` — better than two separate indexes because most queries filter by tenant and time range together.
- `ON CONFLICT (event_id) DO NOTHING` — second line of defense against duplicate writes (dedup set in memory is the first).

### AllISRAcks
- Replicas are broker-side (fault tolerance), not consumers. ISR = In-Sync Replicas.
- `AllISRAcks` means the broker waits for all ISR replicas to write before acking the producer.
- Without it: broker acks with leader only, leader crashes before replication → event ID lost from `seen_events.v1` → dedup guarantee breaks on restart.
- Added to the sink writer's Kafka client (the processor already had it).

### DB wiring in the sink writer
- `pgxpool.New` for connection pooling, DSN assembled from config fields.
- DB write happens **before** `produceSeenEvent`. If DB succeeds but Kafka produce fails, we retry — `ON CONFLICT DO NOTHING` handles the duplicate insert safely.
- Unmarshal errors skip the record without setting `stopProcessing` — treats malformed records as poison, avoids infinite blocking.
- DB errors set `stopProcessing = true` — same pattern as transient Kafka errors, retry on next poll.

### Infrastructure
- Added Postgres to `docker-compose.yml` with a named volume for persistence.
- Migration at `infra/migrations/001_create_log_events.sql` — run manually via psql against the container.
- Added DB config fields to `internal/config/config.go` (Host, DBPort, DB, DBUser, DBPassword).

## Where to Pick Up Next
- Run the migration against the Postgres container.
- End-to-end test: bring up infra, run ingest gateway + processor + sink writer, send logs via HTTP, verify rows in Postgres.
