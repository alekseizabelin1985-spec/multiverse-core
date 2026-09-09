#!/usr/bin/env bash
# =============================================================================
# build/minio-init.sh — buckets, service user, versioning and ILM safety net
# (infrastructure.md v0.3 §5.2, ADR-004 add. 1-3, ADR-021 p. 1, SEC-22)
# =============================================================================
# Runs in the `minio-init` container on the pinned `mc` image, after MinIO
# reports healthy. The image ships bash and coreutils and NOTHING else — no
# sed, no grep, no awk — so every bit of text handling below is a bash builtin.
#
# Division of labour with the platform (review T-007, Mi-10). The per-world
# buckets are created by the code — objstore.EnsureBucket, called from
# `mvctl world init` — and the code is the source of the rules
# (shared/objstore/buckets.go: BucketOptionsFor). This container is the safety
# net for buckets that already exist and the one place that creates the service
# user, because `mc admin` has no equivalent in the objstore API.
#
# Convergence is the point: EnsureBucket calls SetBucketLifecycle, which
# REPLACES the whole configuration with a single rule whose id is
# `multiverse-retention-<bucket>`. So this script writes the same single rule
# with the same id and the same body, through `mc ilm rule import`, which also
# replaces the configuration. Whichever of the two ran last, the bucket ends up
# in the same state, and neither leaves a second, conflicting rule behind.
#
# Everything here is idempotent — it runs on every `make up`.
# =============================================================================
set -euo pipefail

ENDPOINT="${MINIO_ENDPOINT_URL:-http://minio:9000}"
ALIAS=local

# The single retention of the platform, mirroring shared/objstore/buckets.go.
# Changing it means changing both; the integration test of T-007 is what
# notices if only one of them moved.
RETENTION_DAYS=30

# ops-artifacts holds what the operator and CI produce: recordings, reports,
# exports. No versioning and no rule — an artifact goes when somebody decides.
OPS_BUCKET=ops-artifacts

log() { printf '%s minio-init: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

# ---------------------------------------------------------------------------
# 1. Alias. The root credentials are used here and only here; the platform
#    itself runs under the service user created in step 3 (§5.2).
#
#    The keys are piped, not passed as arguments: an argument is readable in
#    /proc/<pid>/cmdline and in `docker top` (review T-008, Mi-3). The other
#    documented way, MC_HOST_<alias>, was tried first and rejected — it is a
#    URL, and on the pinned mc image it neither survives a password containing
#    `/`, `@` or `#` nor percent-decodes the userinfo, so it would trade a
#    visible secret for a stack that refuses to start on a strong password.
# ---------------------------------------------------------------------------
for attempt in $(seq 1 30); do
  if printf '%s\n%s\n' "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" |
    mc alias set "$ALIAS" "$ENDPOINT" >/dev/null 2>&1; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    log "cannot reach ${ENDPOINT} as ${MINIO_ROOT_USER} after ${attempt} attempts"
    exit 1
  fi
  log "waiting for ${ENDPOINT}"
  sleep 1
done
log "alias ${ALIAS} -> ${ENDPOINT}"

# ---------------------------------------------------------------------------
# 2. ops-artifacts. The only bucket that does not belong to a world, so no
#    world-init run would ever create it.
# ---------------------------------------------------------------------------
mc mb --ignore-existing "${ALIAS}/${OPS_BUCKET}" >/dev/null
log "bucket ${OPS_BUCKET} is present"

# ---------------------------------------------------------------------------
# 3. Service user. In prod the platform must not hold the root key (§5.2); in
#    dev the two are often the same value, and then there is nothing to add.
# ---------------------------------------------------------------------------
if [ "$MV_MINIO_ACCESS_KEY" = "$MINIO_ROOT_USER" ]; then
  log "MV_MINIO_ACCESS_KEY is the root user: no service user to add"
else
  user_info() { mc admin user info "$ALIAS" "$MV_MINIO_ACCESS_KEY" 2>/dev/null || true; }

  # The keys are piped, not passed as arguments, for the same reason as the
  # alias above: `mc admin user add TARGET` reads them from stdin.
  if printf '%s\n%s\n' "$MV_MINIO_ACCESS_KEY" "$MV_MINIO_SECRET_KEY" |
    mc admin user add "$ALIAS" >/dev/null 2>&1; then
    log "service user added"
  elif [ -z "$(user_info)" ]; then
    # `add` fails both for a user that is already there and for a credential
    # MinIO refuses (access key shorter than 3 characters, secret shorter than
    # 8, admin API trouble). Only the first is benign; taking the second for
    # the first would let the platform start against a user that does not
    # exist and meet a 403 on its first call — from the very container the
    # rest of the stack waits for with service_completed_successfully.
    log "cannot create service user ${MV_MINIO_ACCESS_KEY} (access key >= 3 and secret >= 8 characters?)"
    exit 1
  else
    # Already there — the usual case on the second run. Its secret is not
    # rotated here: rotating it silently would lock out a running platform.
    log "service user exists"
  fi

  if mc admin policy attach "$ALIAS" readwrite --user "$MV_MINIO_ACCESS_KEY" >/dev/null 2>&1; then
    log "policy readwrite attached"
  else
    # Same distinction: `attach` refuses a policy that is already in effect and
    # it refuses a policy it could not apply. `mc admin user info` names the
    # policies of the user, and bash pattern matching is all this image has to
    # read them with.
    case "$(user_info)" in
    *readwrite*) log "policy readwrite already attached" ;;
    *)
      log "cannot attach policy readwrite to ${MV_MINIO_ACCESS_KEY}"
      exit 1
      ;;
    esac
  fi
fi

# ---------------------------------------------------------------------------
# 4. Versioning and ILM for the buckets that already exist.
#
#    entities-*, snapshots-* : versioned, non-current versions expire after 30 d
#    prompts-*               : NOT versioned, objects expire after 30 d (SEC-22)
#    everything else         : left alone — an unknown bucket is somebody's
#                              scratch space, not something to start expiring
#                              (the default branch of BucketOptionsFor)
# ---------------------------------------------------------------------------

# import_ilm <bucket> <rule-body-json>
# `mc ilm rule import` reads STDIN and replaces the whole configuration,
# exactly like SetBucketLifecycle does on the Go side.
import_ilm() {
  local bucket=$1 body=$2
  printf '{"Rules":[{"ID":"multiverse-retention-%s","Status":"Enabled","Filter":{"Prefix":""},%s}]}\n' \
    "$bucket" "$body" |
    mc ilm rule import "${ALIAS}/${bucket}" >/dev/null
}

# `mc ls <alias>` prints one line per bucket, the name last and slash
# terminated. Captured into a variable first so that a listing failure aborts
# the container instead of quietly iterating over nothing.
listing=$(mc ls "$ALIAS")

while IFS= read -r line; do
  [ -n "$line" ] || continue
  bucket=${line##* }
  bucket=${bucket%/}
  [ -n "$bucket" ] || continue

  case "$bucket" in
  entities-* | snapshots-*)
    mc version enable "${ALIAS}/${bucket}" >/dev/null
    import_ilm "$bucket" "\"NoncurrentVersionExpiration\":{\"NoncurrentDays\":${RETENTION_DAYS}}"
    log "${bucket}: versioning on, non-current versions expire in ${RETENTION_DAYS} d"
    ;;
  prompts-*)
    # Versioning is neither enabled nor suspended: suspending a bucket that was
    # never versioned is an error, and one that somehow got versioned is a
    # finding for the operator, not something to paper over here.
    import_ilm "$bucket" "\"Expiration\":{\"Days\":${RETENTION_DAYS}}"
    log "${bucket}: objects expire in ${RETENTION_DAYS} d, versioning untouched (SEC-22)"
    ;;
  *)
    log "${bucket}: no rules"
    ;;
  esac
done <<<"$listing"

log "done"
