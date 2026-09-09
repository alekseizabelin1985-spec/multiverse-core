#!/usr/bin/env bash
# =============================================================================
# build/redpanda-init.sh — the 8 topics of the platform (infrastructure.md v0.3
# §5.1, ADR-007 p. 4-6 and add. 1-2, D-9, U-3)
# =============================================================================
# Runs in the `redpanda-init` container, on the Redpanda image pinned in
# build/versions.env, after the broker reports `rpk cluster health` healthy.
# It publishes no port and holds no state: on every `make up` it creates the
# topics that are missing and rewrites the config of the ones that are there,
# so a changed retention in this file reaches an existing cluster on the next
# start. That is the whole reason `alter-config` runs unconditionally instead
# of only in the "create failed" branch.
#
# Not created here on purpose: `scope_management` and the other phantom topics
# of the as-is layout (ADR-007), and no Schema Registry — the registry of
# schemas is the repository (shared/contracts, schemas/events/**).
# =============================================================================
set -euo pipefail

BROKERS="${RPK_BROKERS:-redpanda:9092}"

# One day. Without it retention never fires at this traffic: a segment is only
# eligible for deletion once it is closed, and the default segment is 1 GiB
# while the whole platform writes a few megabytes a month (D-9).
SEGMENT_MS=86400000

RETENTION_30D=2592000000
RETENTION_90D=7776000000
RETENTION_180D=15552000000

# 4 MiB: llm_records carries whole prompts and completions (§5.1).
LLM_MAX_MESSAGE_BYTES=4194304

# topic:retention.ms[:extra config]... — the table of §5.1, in one place.
TOPICS=(
  "player_events:${RETENTION_30D}"
  "game_events:${RETENTION_30D}"
  "world_events:${RETENTION_30D}"
  "system_events:${RETENTION_30D}"
  "narrative_output:${RETENTION_30D}"
  "llm_records:${RETENTION_90D}:max.message.bytes=${LLM_MAX_MESSAGE_BYTES}"
  "analytics_events:${RETENTION_180D}"
  "dead_letters:${RETENTION_30D}"
)

log() { printf '%s redpanda-init: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

# `rpk cluster health` talks to the admin API on 9644 and takes no --brokers;
# `cluster info` goes through the Kafka listener — the same path the topic
# commands below use, which is what we actually need to be up. Note the flag
# form: this rpk (v26) takes `-X brokers=`, not `--brokers`.
rpk_ready() {
  rpk cluster info -X brokers="$BROKERS" >/dev/null 2>&1
}

# The compose healthcheck already gates this container, but a broker can lose
# its leader between the probe and the first command; 30 s of patience costs
# nothing and turns a flaky `make up` into a slow one.
for _ in $(seq 1 30); do
  if rpk_ready; then break; fi
  log "waiting for ${BROKERS}"
  sleep 1
done
rpk_ready || { log "broker ${BROKERS} is not reachable"; exit 1; }

for entry in "${TOPICS[@]}"; do
  IFS=':' read -r topic retention extra <<<"$entry"

  create_args=(-p 1 -r 1
    -c cleanup.policy=delete
    -c "retention.ms=${retention}"
    -c "segment.ms=${SEGMENT_MS}")
  alter_args=(--set cleanup.policy=delete
    --set "retention.ms=${retention}"
    --set "segment.ms=${SEGMENT_MS}")
  if [ -n "${extra:-}" ]; then
    create_args+=(-c "${extra}")
    alter_args+=(--set "${extra}")
  fi

  if rpk topic create "$topic" -X brokers="$BROKERS" "${create_args[@]}" >/dev/null 2>&1; then
    log "created ${topic} (retention.ms=${retention}, segment.ms=${SEGMENT_MS}${extra:+, ${extra}})"
  else
    log "${topic} exists"
  fi

  # Always converge: the topic may predate a change to the table above.
  rpk topic alter-config "$topic" -X brokers="$BROKERS" "${alter_args[@]}" >/dev/null
  log "config of ${topic} is up to date"
done

log "done: ${#TOPICS[@]} topics"
