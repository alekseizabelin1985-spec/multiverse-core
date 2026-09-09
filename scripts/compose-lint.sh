#!/usr/bin/env bash
# =============================================================================
# scripts/compose-lint.sh — the house rules of docker-compose.yml
# (infrastructure.md v0.3 §3.1.1; ADR-010 add. 4; SEC-13, SEC-14, SEC-15,
# SEC-33; NFR-071)
# =============================================================================
# Same script in CI (job `compose-lint`) and locally (`make compose-lint`), so
# it must run in Git Bash on Windows as well as on ubuntu-latest. It shells out
# to `docker compose` and to `python3` for the JSON — `jq` is not installed on
# the owner's machine and pulling a container to parse a document would be a
# strange dependency for a linter.
#
# The six rules:
#   1. every image has an explicit tag, never `latest`, and every third-party
#      image comes from a variable declared in build/versions.env (NFR-071);
#   2. every published port is bound to 127.0.0.1 (SEC-13); the init containers,
#      chromadb and narrative-orchestrator publish nothing; a console port is
#      published only by a service in profile `dev` (SEC-33);
#   3. no default credentials: a secret in `environment` is an interpolation of
#      the variable of the same name, and the required ones use `${VAR:?}`;
#      `minioadmin` appears nowhere outside Docs/ and services/_archive/
#      (SEC-14);
#   4. MV_TELEGRAM_BOT_TOKEN reaches the `telegram-bot` service only, through
#      `environment` and never through an `env_file` shared with others (D-10);
#   5. NEO4J_PLUGINS is absent (T-17, SEC-32); OLLAMA_ORIGINS is not a wildcard
#      and OLLAMA_HOST is not 0.0.0.0 (SEC-15);
#   6. there is no `llama-server` service — it is a native process outside
#      compose (ADR-005 add. 2 p. 7) — and MV_LLM_URL points at loopback,
#      host.docker.internal, a service of this file or an RFC1918 address, but
#      never at a public host (SEC-15, ADR-005 add. 2 p. 3).
#
# Exit code is non-zero on any violation.
#
# Usage:
#   scripts/compose-lint.sh [-f <compose file>]
# Environment:
#   COMPOSE_LINT_ENV_FILES  space separated env files (default:
#                           "build/versions.env .github/ci.env")
#   COMPOSE_LINT_PROFILES   space separated profiles to resolve
#                           (default: all of them)
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

compose_file=docker-compose.yml
while [ $# -gt 0 ]; do
  case "$1" in
  -f | --file)
    compose_file=$2
    shift 2
    ;;
  -h | --help)
    sed -n '2,50p' "$0"
    exit 0
    ;;
  *)
    printf 'compose-lint: unknown argument %s\n' "$1" >&2
    exit 2
    ;;
  esac
done

read -r -a env_files <<<"${COMPOSE_LINT_ENV_FILES:-build/versions.env .github/ci.env}"
read -r -a profiles <<<"${COMPOSE_LINT_PROFILES:-gpu memory dev legacy bot}"

python_bin=$(command -v python3 || command -v python || true)
if [ -z "$python_bin" ]; then
  echo "compose-lint: python3 is required to read the compose model" >&2
  exit 2
fi

args=(compose -f "$compose_file")
for f in "${env_files[@]}"; do
  [ -n "$f" ] && args+=(--env-file "$f")
done
# Every profile at once: the resolved model keeps each service's `profiles`, so
# one pass covers the whole file and rule 2 can still tell a `dev` console from
# a production port.
for p in "${profiles[@]}"; do
  [ -n "$p" ] && args+=(--profile "$p")
done

model=$(docker "${args[@]}" config --format json)

# Rule 3's second half needs the work tree, not the model. The scope is what
# the target platform is built from — compose, build/, scripts/, the single Go
# module and the environment files. Out of scope, and out of this linter's
# reach: Docs/ and services/_archive/ (named by §3.1.1), the frozen as-is
# services under services/<name>/ that no target profile builds (their removal
# is EPIC-003 I2 / T-019, not T-008) and the as-is documents at the repository
# root. A line that mentions the word in order to forbid it — `.env.example`,
# `shared/env/infra.go` — is not a credential and is filtered below.
# This file is excluded from its own scan: it necessarily spells the word out.
minioadmin_hits=$(git grep -i -n -- minioadmin --   docker-compose.yml build scripts cmd shared .github .env.example   ':!scripts/compose-lint.sh' |
  grep -v -i -E '(never|not|не) minioadmin' || true)

MODEL_JSON="$model" MINIOADMIN_HITS="$minioadmin_hits" \
  COMPOSE_FILE_PATH="$compose_file" \
  "$python_bin" - <<'PY'
import ipaddress
import json
import os
import re
import sys
from urllib.parse import urlsplit

model = json.loads(os.environ["MODEL_JSON"])
compose_path = os.environ["COMPOSE_FILE_PATH"]
with open(compose_path, encoding="utf-8") as fh:
    raw = fh.read()

services = model.get("services") or {}
service_names = set(services)

failures = []


def fail(rule, message):
    failures.append(f"[rule {rule}] {message}")


# --------------------------------------------------------------------------
# build/versions.env — the only place a third-party tag may come from.
# --------------------------------------------------------------------------
versions = {}
with open("build/versions.env", encoding="utf-8") as fh:
    for line in fh:
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        name, _, value = line.partition("=")
        versions[name.strip()] = value.strip()

# Images built from this repository: their tag is ours to choose, so rule 1
# only asks that the tag is explicit.
LOCAL_IMAGE_PREFIXES = ("multiverse-core:", "multiverse-core-legacy:", "multiverse-core/")

# `image:` as written in the file, per service, so that rule 1 can see the
# variable and not only the value it interpolated to.
raw_images = {}
current = None
for line in raw.splitlines():
    m = re.match(r"^  ([A-Za-z0-9][A-Za-z0-9._-]*):\s*$", line)
    if m:
        current = m.group(1)
        continue
    m = re.match(r"^    image:\s*(\S.*?)\s*$", line)
    if m and current:
        raw_images[current] = m.group(1)

# --------------------------------------------------------------------------
# Rule 1 — pinned images
# --------------------------------------------------------------------------
for name, svc in sorted(services.items()):
    image = svc.get("image")
    if not image:
        fail(1, f"{name}: no image, so nothing is pinned")
        continue
    repo, sep, tag = image.rpartition(":")
    if not sep or "/" in tag:
        fail(1, f"{name}: image {image!r} has no explicit tag")
        continue
    if not tag or tag == "latest":
        fail(1, f"{name}: image {image!r} must carry a version, not {tag or 'an empty tag'!r}")
    if image.startswith(LOCAL_IMAGE_PREFIXES):
        continue
    source = raw_images.get(name, "")
    used = set(re.findall(r"\$\{([A-Za-z_][A-Za-z0-9_]*)", source))
    if not used & set(versions):
        fail(
            1,
            f"{name}: image {image!r} does not come from build/versions.env "
            f"(source line: {source!r})",
        )

# --------------------------------------------------------------------------
# Rule 2 — ports
# --------------------------------------------------------------------------
NO_PUBLICATION = {"redpanda-init", "minio-init", "chromadb", "narrative-orchestrator"}
# A console is an operator UI: it must not be reachable outside profile `dev`.
CONSOLE_TARGETS = {9001: "MinIO console", 7474: "Neo4j Browser"}

for name, svc in sorted(services.items()):
    published = svc.get("ports") or []
    profiles = set(svc.get("profiles") or [])
    if name in NO_PUBLICATION and published:
        fail(2, f"{name}: publishes {len(published)} port(s); this service must publish none")
    for port in published:
        host_ip = port.get("host_ip") or ""
        target = int(port.get("target") or 0)
        pub = port.get("published") or ""
        if host_ip != "127.0.0.1":
            fail(
                2,
                f"{name}: port {pub}->{target} is published on "
                f"{host_ip or 'every interface'}; SEC-13 wants 127.0.0.1",
            )
        console = CONSOLE_TARGETS.get(target) or ("console" if name.endswith("-console") else None)
        if console and "dev" not in profiles:
            fail(
                2,
                f"{name}: {console} ({target}) is published outside profile `dev` "
                f"(profiles: {sorted(profiles) or 'none'}); SEC-33",
            )

# --------------------------------------------------------------------------
# Rule 3 — no default credentials
# --------------------------------------------------------------------------
SECRET_KEY = re.compile(
    r"^(MINIO_ROOT_USER|MINIO_ROOT_PASSWORD|NEO4J_AUTH|NEO4J_PASSWORD"
    r"|MV_.*_KEY|MV_.*_PASSWORD|MV_TELEGRAM_BOT_TOKEN)$"
)
# The ones the stack must refuse to start without: they need `${VAR:?}`, not a
# default. MV_LLM_API_KEY and MV_ANTHROPIC_API_KEY are absent on purpose — an
# empty cloud key is the normal, local case.
MUST_BE_REQUIRED = {
    "MINIO_ROOT_USER",
    "MINIO_ROOT_PASSWORD",
    "NEO4J_AUTH",
    "NEO4J_PASSWORD",
    "MV_MINIO_ACCESS_KEY",
    "MV_MINIO_SECRET_KEY",
    "MV_NEO4J_PASSWORD",
    "MV_TELEGRAM_BOT_TOKEN",
}

# Credentials as written, in one pass over the whole file: the resolved model
# has already substituted the values, so a literal password and a correct
# `${VAR:?}` look the same there. Layout is deliberately not part of the rule —
# `environment` may be a mapping (`KEY: value`) or a list (`- KEY=value`), and a
# secret may sit in any anchor (`x-platform-env` and whatever the next one is
# called), not only in a service block. Anchoring on either would leave exactly
# the hole this rule exists to close. The nearest enclosing key is tracked only
# to name the place in the message.
ENTRY = re.compile(r"^\s*-?\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:=]\s*(\S.*?)\s*$")
TOP_LEVEL = re.compile(r"^([A-Za-z0-9_][A-Za-z0-9._-]*):")
BLOCK = re.compile(r"^  ([A-Za-z0-9][A-Za-z0-9._-]*):\s*$")

context = compose_path
for lineno, line in enumerate(raw.splitlines(), 1):
    if line.lstrip().startswith("#"):
        continue
    m = TOP_LEVEL.match(line)
    if m:
        context = m.group(1)
        continue
    m = BLOCK.match(line)
    if m:
        context = m.group(1)
        continue
    m = ENTRY.match(line)
    if not m:
        continue
    key, value = m.group(1), m.group(2)
    if not SECRET_KEY.match(key):
        continue
    refs = re.findall(r"\$\{([A-Za-z_][A-Za-z0-9_]*)(:[-?][^}]*)?\}", value)
    if not refs:
        fail(3, f"{context}:{lineno}: {key} carries the literal {value!r}; use ${{{key}:?...}}")
        continue
    if key in MUST_BE_REQUIRED and not any(
        (mod or "").startswith(":?") for _, mod in refs
    ):
        fail(
            3,
            f"{context}:{lineno}: {key} has a default ({value!r}); a missing credential "
            "must stop the stack, so it needs ${VAR:?}",
        )

hits = [h for h in os.environ.get("MINIOADMIN_HITS", "").splitlines() if h.strip()]
for hit in hits:
    fail(3, f"the as-is credential `minioadmin` is still in {hit}")

# --------------------------------------------------------------------------
# Rule 4 — the bot token belongs to the bot
# --------------------------------------------------------------------------
TOKEN = "MV_TELEGRAM_BOT_TOKEN"
for name, svc in sorted(services.items()):
    env = svc.get("environment") or {}
    if TOKEN in env and name != "telegram-bot":
        fail(4, f"{name}: reads {TOKEN}; only telegram-bot may (D-10)")
    for entry in svc.get("env_file") or []:
        path = entry.get("path") if isinstance(entry, dict) else entry
        if not path or not os.path.exists(path):
            continue
        with open(path, encoding="utf-8", errors="replace") as fh:
            if any(line.lstrip().startswith(TOKEN + "=") for line in fh):
                fail(4, f"{name}: env_file {path} carries {TOKEN}")

# --------------------------------------------------------------------------
# Rule 5 — Neo4j plugins and Ollama exposure
# --------------------------------------------------------------------------
for name, svc in sorted(services.items()):
    env = svc.get("environment") or {}
    if "NEO4J_PLUGINS" in env:
        fail(5, f"{name}: NEO4J_PLUGINS is set; APOC reaches the file system (T-17, SEC-32)")
    origins = env.get("OLLAMA_ORIGINS")
    if origins is not None and "*" in origins:
        fail(5, f"{name}: OLLAMA_ORIGINS={origins!r} is a wildcard (SEC-15)")
    host = env.get("OLLAMA_HOST")
    if host is not None and "0.0.0.0" in host:
        fail(5, f"{name}: OLLAMA_HOST={host!r} exposes the model API (SEC-15)")
if re.search(r"OLLAMA_HOST\s*[:=]\s*[\"']?0\.0\.0\.0", raw):
    fail(5, "the literal OLLAMA_HOST=0.0.0.0 appears in the compose file (SEC-15)")

# --------------------------------------------------------------------------
# Rule 6 — the LLM is a native process, and MV_LLM_URL stays local
# --------------------------------------------------------------------------
# `ollama` is a different runtime and has its own, optional service (profile
# `gpu`); what must not appear is llama.cpp's own server.
LLAMA_SERVER = re.compile(r"llama[._-]?(server|cpp)", re.IGNORECASE)
for name, svc in sorted(services.items()):
    if LLAMA_SERVER.search(name) or LLAMA_SERVER.search(svc.get("image") or ""):
        fail(
            6,
            f"{name}: llama-server runs natively on the host, outside compose "
            "(ADR-005 add. 2 p. 7)",
        )

ALLOWED_HOSTS = {"host.docker.internal", "localhost"} | service_names


def local_llm_host(url):
    host = urlsplit(url).hostname
    if not host:
        return False, "no host"
    if host in ALLOWED_HOSTS:
        return True, host
    try:
        addr = ipaddress.ip_address(host)
    except ValueError:
        return False, host
    return (addr.is_loopback or addr.is_private), host


for name, svc in sorted(services.items()):
    for key in ("MV_LLM_URL", "MV_OLLAMA_URL"):
        url = (svc.get("environment") or {}).get(key)
        if not url:
            continue
        ok, host = local_llm_host(url)
        if not ok:
            fail(
                6,
                f"{name}: {key} points at the public host {host!r}; a cloud endpoint "
                "needs MV_LLM_CLOUD_ENABLED=true and never a compose default "
                "(SEC-15, ADR-005 add. 2 p. 3)",
            )

# --------------------------------------------------------------------------
if failures:
    print(f"compose-lint: {len(failures)} violation(s) in {compose_path}", file=sys.stderr)
    for line in failures:
        print("  " + line, file=sys.stderr)
    sys.exit(1)

print(
    f"compose-lint: ok — {len(services)} services, 6 rules "
    f"(profiles resolved: {', '.join(sorted({p for s in services.values() for p in (s.get('profiles') or [])})) or 'none'})"
)
PY
