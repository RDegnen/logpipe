#!/usr/bin/env bash
set -euo pipefail

CONTAINER="redpanda"

topics=(
  "raw_logs.v1|-p 6"
  "processed_logs.v1.default|-p 6"
  "dlq_logs.v1|-p 3"
  "seen_events.v1|-p 3 --topic-config cleanup.policy=compact"
)

for entry in "${topics[@]}"; do
  name="${entry%%|*}"
  flags="${entry#*|}"

  if docker exec "$CONTAINER" rpk topic describe "$name" &>/dev/null; then
    echo "Topic $name already exists, skipping."
  else
    echo "Creating topic $name..."
    eval docker exec "$CONTAINER" rpk topic create "$name" $flags
  fi
done

echo ""
echo "All topics:"
docker exec "$CONTAINER" rpk topic list
