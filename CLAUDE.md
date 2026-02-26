# LogPipe – Agent Operating Constraints

This document defines rules for AI coding agents modifying this repository.

See [LOGPIPE_BUILD_PLAN.md](LOGPIPE_BUILD_PLAN.md) for the full project architecture, schema, delivery semantics, and 12-week build plan.

---

# 1. Delivery Guarantees MUST NOT Change

Processor must:

- Use manual offset commit.
- Commit only after downstream produce success.
- Preserve at-least-once semantics.

Ingest must:

- Only report accepted after Kafka ack.

---

# 2. Do NOT Introduce Heavy Frameworks

No:

- ORMs
- Dependency injection frameworks
- Massive abstractions
- Distributed transaction libraries

Keep services minimal and explicit.

---

# 3. No Silent Behavior Changes

If modifying:

- Offset commit logic
- Partition key
- Schema
- Delivery guarantees

You MUST update:

- SYSTEM_ARCHITECTURE.md
- LOGPIPE_BUILD_PLAN.md

---

# 4. Code Style Guidelines

- Prefer clarity over cleverness.
- Avoid premature abstraction.
- Keep concurrency explicit.
- Avoid global mutable state.
- Log errors explicitly.

---

# 5. Testing Requirements

When modifying processing logic:

- Validate no message loss.
- Validate DLQ behavior.
- Validate crash recovery.

---

# 6. Performance Tuning Rules

Before adding concurrency:

- Measure baseline.
- Document bottleneck.
- Avoid speculative optimization.

---

# 7. Architectural Stability

Agent must not:

- Change partition key without justification.
- Switch delivery semantics.
- Introduce exactly-once illusions.

---

# 8. Change Control

Major changes require:

- Documentation update.
- Reasoning section added.
- Tradeoff explanation.

---

# 9. Purpose Reminder

This project is for:

- Learning distributed systems by building.
- Experiencing failure modes.
- Practicing operational thinking.

This is NOT a tutorial scaffold or interview prep exercise.
