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
# The eight rules:
#   1. every image has an explicit tag, never `latest`, and every third-party
#      image comes from a variable declared in build/versions.env (NFR-071);
#   2. every published port is bound to 127.0.0.1 (SEC-13); the init containers,
#      chromadb and narrative-orchestrator publish nothing; a console port is
#      published only by a service in profile `dev` (SEC-33);
#   3. no default credentials: a secret in `environment` is an interpolation of
#      the variable of the same name, and the required ones use `${VAR:?}` —
#      a key without a value does not count, because an unset one is simply
#      left out of the container and the image falls back to its own account;
#      `minioadmin` appears nowhere outside Docs/ and services/_archive/
#      (SEC-14);
#   4. MV_TELEGRAM_BOT_TOKEN reaches the `telegram-bot` service only, through
#      `environment` and never through an `env_file` shared with others (D-10);
#   5. NEO4J_PLUGINS is absent (T-17, SEC-32); OLLAMA_ORIGINS is not a wildcard
#      and OLLAMA_HOST is not 0.0.0.0 (SEC-15);
#   6. there is no `llama-server` service — it is a native process outside
#      compose (ADR-005 add. 2 p. 7) — and MV_LLM_URL points at loopback,
#      host.docker.internal, a service of this file or an RFC1918 address, but
#      never at a public host (SEC-15, ADR-005 add. 2 p. 3);
#   7. the always-loaded compose file starts on a clean machine (T-397);
#   8. a platform variable has one default, and it is the manifest's (T-411):
#      `${MV_X:-d}` is allowed only when d equals the default declared for MV_X
#      in shared/env/vars.go, or — for a variable whose manifest default is
#      itself an address — when every item of d is `service:port` or
#      `scheme://service...` for a service of this compose network: the one
#      override the manifest documents ("compose overrides them with the names
#      of the services on its own network"). A bare word is never an address,
#      even when it happens to be a service name: `telegram-bot` is a service
#      AND a client id, and an allow-list defaulting to it is exactly the
#      defect T-411 closed (review #1 Mi-1). `${MV_X}` and `$MV_X` with no
#      modifier are rejected when the manifest default is not empty: a silent
#      .env then hands the process an empty value instead of it (Mi-2); `$$`
#      is compose's escape and is left alone. Anything else is a second source
#      of one value: a container and a process started by hand disagree, and
#      nothing tells the operator which one decides. A name the manifest does
#      not declare, or declares retired, is rejected too — that is also what
#      makes a parse miss loud.
#
# Rule 8, the shape of the fix it asks for: pass the variable through as a key
# with no value (`MV_X:` in a mapping, `- MV_X` in a list). Compose then sets it
# from .env when .env has it — empty included, which for an allow-list means
# nobody — and leaves it out of the container when .env is silent, so the
# manifest's default applies. `${MV_X}` and `${MV_X:-}` are NOT that: both hand
# the process an empty value, and shared/env treats set-to-empty as a value —
# which is why rule 8 rejects both whenever the manifest's default is not empty.
#
# Rule 7 in full, because it is the one that is easy to break by accident:
# `docker compose` interpolates a file WHOLE, before it filters by profile, so
# one `${VAR:?}` belonging to a service the operator never runs stops `config`,
# `up` and `ps` for everybody — which is exactly how MV_TELEGRAM_BOT_TOKEN and
# CHROMA_IMAGE (empty by decision D-3) killed `make up` on a clean machine
# before a single container existed. The rule reproduces that: it builds the
# environment of a machine that copied .env.example and filled what the README
# asks for — the variables marked `[required]` there — adds build/versions.env,
# and demands that the first compose file (the one compose loads by default)
# interpolate without a single error. A required variable that a clean machine
# has no reason to hold belongs in a per-profile file, loaded with its profile:
# docker-compose.bot.yml, docker-compose.legacy.yml. Those files are linted by
# rules 1–6 like any other, and are deliberately outside rule 7.
#
# Exit code is non-zero on any violation.
#
# Usage:
#   scripts/compose-lint.sh [-f <compose file>]...
# The first -f replaces the default set of files; further -f add to it. The
# first file in the set is the one rule 7 applies to.
# Environment:
#   COMPOSE_LINT_ENV_FILES  space separated env files (default:
#                           "build/versions.env .github/ci.env")
#   COMPOSE_LINT_PROFILES   space separated profiles to resolve
#                           (default: all of them)
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

# The caller may be `make`, which exports COMPOSE_ENV_FILES, or a shell where
# the operator set COMPOSE_FILE/COMPOSE_PROFILES for their own stack. Every file
# and profile this linter looks at is passed explicitly below; inheriting any of
# them would make the result depend on whose terminal it ran in.
unset COMPOSE_ENV_FILES COMPOSE_FILE COMPOSE_PROFILES

compose_files=()
while [ $# -gt 0 ]; do
  case "$1" in
  -f | --file)
    compose_files+=("$2")
    shift 2
    ;;
  -h | --help)
    # The whole header, however long it grows: up to the first line of code.
    awk 'NR > 1 && /^set -euo pipefail/ { exit } NR > 1' "$0"
    exit 0
    ;;
  *)
    printf 'compose-lint: unknown argument %s\n' "$1" >&2
    exit 2
    ;;
  esac
done

# The whole target topology: the file compose loads by default plus the two
# per-profile files it does not (T-397). Rules 1-6 need all of them, or the bot
# token and the Chroma image would stop being checked the moment they moved.
if [ ${#compose_files[@]} -eq 0 ]; then
  compose_files=(docker-compose.yml docker-compose.bot.yml docker-compose.legacy.yml)
fi

read -r -a env_files <<<"${COMPOSE_LINT_ENV_FILES:-build/versions.env .github/ci.env}"
read -r -a profiles <<<"${COMPOSE_LINT_PROFILES:-gpu memory dev legacy bot}"

python_bin=$(command -v python3 || command -v python || true)
if [ -z "$python_bin" ]; then
  echo "compose-lint: python3 is required to read the compose model" >&2
  exit 2
fi

profile_args=()
for p in "${profiles[@]}"; do
  [ -n "$p" ] && profile_args+=(--profile "$p")
done

# --------------------------------------------------------------------------
# Rule 7 — the always-loaded file starts on a clean machine.
# Runs first and on its own: if this fails, nothing else about the file matters
# to an operator who cannot get past `docker compose config`.
# --------------------------------------------------------------------------
clean_env=$(mktemp)
trap 'rm -f "$clean_env"' EXIT

primary=${compose_files[0]}
# A fixture may bring its own example file: testdata/compose-lint/bad-x.yml is
# paired with bad-x.env when that file exists. That is how the half of rule 7
# that reads .env.example gets a negative fixture of its own without teaching
# the `bad-*.yml` loop of `make compose-lint` and of CI a second file pattern.
env_example=${primary%.yml}.env
[ -f "$env_example" ] || env_example=.env.example

if ! clean_out=$(CLEAN_ENV_PATH="$clean_env" ENV_EXAMPLE_PATH="$env_example" \
  "$python_bin" - <<'PY' 2>&1
import io
import os
import sys

# The environment of a machine that copied .env.example and filled what the
# README asks for: the file verbatim, with one substitution — the variables
# marked `[required]` in the comment above them. Everything else keeps the value
# the example ships, including the empty ones, which is the whole point.
DUMMY = "compose-lint-clean-machine"
SKIP = ("COMPOSE_ENV_FILES=", "COMPOSE_FILE=")

path = os.environ["ENV_EXAMPLE_PATH"]
out = []
comment = ""
problems = []

for lineno, line in enumerate(io.open(path, encoding="utf-8"), 1):
    line = line.rstrip("\n")
    stripped = line.strip()
    if not stripped:
        comment = ""
        continue
    if stripped.startswith("#"):
        # The whole contiguous block, not just its last line: the marker belongs
        # to the comment above the variable, and which line of that comment
        # carries it is the author's business. Keeping only the last line made
        # the marker invisible whenever an explanation followed it, and rule 7
        # then declared a filled clean machine unfilled (T-404).
        comment = f"{comment} {stripped}" if comment else stripped
        continue
    key, sep, rest = line.partition("=")
    if not sep:
        comment = ""
        continue
    value, hashmark, _ = rest.partition("#")
    # An inline comment after an EMPTY value is not a comment to compose: its
    # dotenv parser trims a trailing comment only when a value precedes it, so
    # `KEY=   # a secret, leave empty` arrives as the literal text of the
    # comment — non-empty, which silences ${KEY:?} and starts the container with
    # rubbish. `set -a; . ./.env` in the Makefile reads the same line as empty,
    # so compose and make disagree about one variable. Hence: no inline comment
    # after an empty value, ever.
    if hashmark and not value.strip():
        problems.append(f"{path}:{lineno}: {key.strip()} is empty and carries an "
                        f"inline comment, which compose reads as its value; move "
                        f"the comment to the line above")
    if not line.startswith(SKIP):
        out.append(f"{key}={DUMMY}" if ("[required]" in comment and not value.strip()) else line)
    comment = ""

if problems:
    print("\n".join(problems), file=sys.stderr)
    sys.exit(1)

io.open(os.environ["CLEAN_ENV_PATH"], "w", encoding="utf-8", newline="\n").write(
    "\n".join(out) + "\n"
)
PY
); then
  {
    echo "compose-lint: 1 violation(s) in $env_example"
    echo "  [rule 7] the environment of a clean machine cannot be read from $env_example:"
    printf '%s\n' "$clean_out" | sed 's/^/           /'
  } >&2
  exit 1
fi

# The env files in the order .env declares them (COMPOSE_ENV_FILES=.env,
# build/versions.env): the pins win over the example, as they do for `make up`.
if ! clean_out=$(docker compose -f "$primary" \
  --env-file "$clean_env" --env-file build/versions.env \
  "${profile_args[@]}" config -q 2>&1); then
  {
    echo "compose-lint: 1 violation(s) in $primary"
    echo "  [rule 7] $primary does not interpolate on a clean machine (.env.example"
    echo "           with its [required] variables filled + build/versions.env), so"
    echo "           \`make up\` stops before the first container is created (T-397):"
    printf '%s\n' "$clean_out" | sed 's/^/           /'
    echo "           A required variable of a service outside the default profile set"
    echo "           belongs in that profile's own file (docker-compose.bot.yml,"
    echo "           docker-compose.legacy.yml), where the refusal reaches the operator"
    echo "           who actually asked for the profile."
  } >&2
  exit 1
fi

# --------------------------------------------------------------------------
# Rules 1-6 — the resolved model, every file and every profile at once.
# --------------------------------------------------------------------------
args=(compose)
for f in "${compose_files[@]}"; do
  args+=(-f "$f")
done
for f in "${env_files[@]}"; do
  [ -n "$f" ] && args+=(--env-file "$f")
done
# Every profile at once: the resolved model keeps each service's `profiles`, so
# one pass covers the whole topology and rule 2 can still tell a `dev` console
# from a production port.
args+=("${profile_args[@]}")

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
minioadmin_hits=$(git grep -i -n -- minioadmin -- \
  'docker-compose*.yml' build scripts cmd shared .github .env.example \
  ':!scripts/compose-lint.sh' |
  grep -v -i -E '(never|not|не) minioadmin' || true)

MODEL_JSON="$model" MINIOADMIN_HITS="$minioadmin_hits" \
  COMPOSE_FILE_PATHS="${compose_files[*]}" \
  "$python_bin" - <<'PY'
import ipaddress
import json
import os
import re
import sys
from urllib.parse import urlsplit

model = json.loads(os.environ["MODEL_JSON"])
compose_paths = os.environ["COMPOSE_FILE_PATHS"].split()
raw_files = []
for path in compose_paths:
    with open(path, encoding="utf-8") as fh:
        raw_files.append((path, fh.read()))

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

# `image:` as written in the files, per service, so that rule 1 can see the
# variable and not only the value it interpolated to.
raw_images = {}
for _, raw in raw_files:
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

# Credentials as written, in one pass over every file: the resolved model has
# already substituted the values, so a literal password and a correct
# `${VAR:?}` look the same there. Layout is deliberately not part of the rule —
# `environment` may be a mapping (`KEY: value`) or a list (`- KEY=value`), and a
# secret may sit in any anchor (`x-platform-env` and whatever the next one is
# called), not only in a service block. Anchoring on either would leave exactly
# the hole this rule exists to close. The nearest enclosing key is tracked only
# to name the place in the message.
ENTRY = re.compile(r"^\s*-?\s*([A-Za-z_][A-Za-z0-9_]*)\s*[:=]\s*(\S.*?)\s*$")
# A key with no value: `KEY:` in a mapping, `- KEY` in a list. Compose fills it
# from .env when .env has it and otherwise leaves it out of the container — for
# a credential the stack must refuse to start without, that is a silent
# fallback to whatever the image does on its own (MinIO: minioadmin). Checked
# before BLOCK, because at indent 2 inside an anchor it looks exactly like the
# name of a block (T-411: rule 8 teaches this form, rule 3 must not be blind
# to it).
BARE = re.compile(r"^\s*-?\s*([A-Za-z_][A-Za-z0-9_]*)\s*:?\s*$")
TOP_LEVEL = re.compile(r"^([A-Za-z0-9_][A-Za-z0-9._-]*):")
BLOCK = re.compile(r"^  ([A-Za-z0-9][A-Za-z0-9._-]*):\s*$")

for compose_path, raw in raw_files:
    context = compose_path
    for lineno, line in enumerate(raw.splitlines(), 1):
        if line.lstrip().startswith("#"):
            continue
        m = BARE.match(line)
        if m and m.group(1) in MUST_BE_REQUIRED:
            fail(
                3,
                f"{compose_path} {context}:{lineno}: {m.group(1)} is passed through "
                "with no value; an unset credential would just be left out, so it "
                "needs ${VAR:?}",
            )
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
            fail(
                3,
                f"{compose_path} {context}:{lineno}: {key} carries the literal "
                f"{value!r}; use ${{{key}:?...}}",
            )
            continue
        if key in MUST_BE_REQUIRED and not any(
            (mod or "").startswith(":?") for _, mod in refs
        ):
            fail(
                3,
                f"{compose_path} {context}:{lineno}: {key} has a default ({value!r}); "
                "a missing credential must stop the stack, so it needs ${VAR:?}",
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
for compose_path, raw in raw_files:
    if re.search(r"OLLAMA_HOST\s*[:=]\s*[\"']?0\.0\.0\.0", raw):
        fail(5, f"the literal OLLAMA_HOST=0.0.0.0 appears in {compose_path} (SEC-15)")

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
# Rule 8 — a platform variable has one default, and it is the manifest's
# --------------------------------------------------------------------------
# The manifest is read as source text and not through mvctl: this job runs
# without Go. `\s*` spans newlines, so a Declare whose name and default sit on
# lines of their own parses as well as a one-line one. What the pattern cannot
# read — a constant or a raw string for the name, an escaped quote in the
# default, DeclareExternal("MV_...") — leaves the name unknown, and an unknown
# name fails below; a concatenated default is read up to its first quote and
# fails as a mismatch. Either way a parse miss is loud. Lines that are Go
# comments are dropped first: dict() keeps the LAST match, so a commented-out
# `// Declare("MV_A", "old", ...)` below the real one would otherwise become
# the default this rule compares with — the one silent case (review #1 N-2).
MANIFEST = "shared/env/vars.go"
with open(MANIFEST, encoding="utf-8") as fh:
    manifest_src = "\n".join(l for l in fh.read().splitlines() if not l.lstrip().startswith("//"))
manifest = dict(re.findall(r'\bDeclare\(\s*"(MV_[A-Z0-9_]+)"\s*,\s*"([^"\\]*)"', manifest_src))
retired = set(re.findall(r'\bDeclareDeprecated\(\s*"(MV_[A-Z0-9_]+)"', manifest_src))
if not manifest:
    fail(8, f"no Declare(\"MV_...\", ...) found in {MANIFEST}; nothing to compare the defaults with")

# `${MV_X:-d}` and `${MV_X-d}`; `${MV_X:?}` carries no default and is rule 7's.
DEFAULTED = re.compile(r"\$\{(MV_[A-Z0-9_]+):?-([^}]*)\}")
# `${MV_X}` and `$MV_X`: no modifier at all, so a silent .env hands the process
# an EMPTY value — set, and therefore beating the manifest's default (review #1
# Mi-2). `$$` is compose's escape for a literal dollar, not an interpolation.
BRACED = r"\{(MV_[A-Z0-9_]+)\}"
BARE_REF = r"(MV_[A-Z0-9_]+)(?![A-Za-z0-9_])"
UNMODIFIED = re.compile(r"(?<!\$)\$(?:" + BRACED + "|" + BARE_REF + ")")
# host:port or scheme://... — the only shapes an address takes in these files.
ADDRESS_ITEM = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*://\S+|[A-Za-z0-9._-]+:[0-9]+")


def is_address(value):
    """True when every comma separated item is host:port or scheme://..."""
    return all(ADDRESS_ITEM.fullmatch(i.strip()) for i in value.split(","))


def names_services(value):
    """True when every item names a service of this compose network AS AN
    ADDRESS: `redpanda:9092`, `http://core:8090`, `neo4j://neo4j:7687`. A bare
    `telegram-bot` is a word that happens to be a service name — and a client
    id (review #1 Mi-1)."""
    if not is_address(value):
        return False
    for item in (i.strip() for i in value.split(",")):
        host = urlsplit(item).hostname if "://" in item else item.rpartition(":")[0] or item
        if host not in service_names:
            return False
    return True


def unknown(where, name):
    """Retired and undeclared names fail whatever form the interpolation takes."""
    if name in retired:
        fail(8, f"{where}: {name} is declared retired in {MANIFEST}; nothing reads it")
        return True
    if name not in manifest:
        fail(
            8,
            f"{where}: {name} is not declared in {MANIFEST}, so its value here "
            "has nothing to agree with (and mvctl env check cannot see it)",
        )
        return True
    return False


for compose_path, raw in raw_files:
    for lineno, line in enumerate(raw.splitlines(), 1):
        if line.lstrip().startswith("#"):
            continue
        where = f"{compose_path}:{lineno}"
        for name, default in DEFAULTED.findall(line):
            if unknown(where, name):
                continue
            # The address exception holds only for a variable that IS an address.
            if default != manifest[name] and not (is_address(manifest[name]) and names_services(default)):
                fail(
                    8,
                    f"{where}: {name} defaults to {default!r} here and to "
                    f"{manifest[name]!r} in {MANIFEST} - two sources of one value "
                    "(T-411). Pass it through as a key with no value, so that .env "
                    "decides and the manifest's default applies when .env is silent. "
                    "Only an address variable (its manifest default is host:port or "
                    "scheme://) may differ, and only by naming a service of this "
                    "compose network with a port or a scheme",
                )
        for m in UNMODIFIED.finditer(line):
            name = next(g for g in m.groups() if g)
            if unknown(where, name):
                continue
            if manifest[name] != "":
                fail(
                    8,
                    f"{where}: {m.group(0)} has neither a default nor `:?`: when .env "
                    f"is silent the process gets an EMPTY {name}, not the manifest's "
                    f"{manifest[name]!r}, and shared/env reads set-to-empty as a value "
                    "- for an allow-list that is nobody (T-411 review #1 Mi-2). Pass "
                    "it through as a key with no value",
                )

# --------------------------------------------------------------------------
if failures:
    print(
        f"compose-lint: {len(failures)} violation(s) in {', '.join(compose_paths)}",
        file=sys.stderr,
    )
    for line in failures:
        print("  " + line, file=sys.stderr)
    sys.exit(1)

print(
    f"compose-lint: ok — {len(services)} services in {len(compose_paths)} file(s), 8 rules "
    f"(profiles resolved: {', '.join(sorted({p for s in services.values() for p in (s.get('profiles') or [])})) or 'none'})"
)
PY
