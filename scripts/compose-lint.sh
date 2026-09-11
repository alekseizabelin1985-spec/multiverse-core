#!/usr/bin/env bash
# =============================================================================
# scripts/compose-lint.sh — the house rules of docker-compose.yml
# (infrastructure.md v0.3 §3.1.1; ADR-010 add. 4; SEC-13, SEC-14, SEC-15,
# SEC-33; NFR-071; contracts.md §16 p. 5)
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
#      a key is a key whether it is quoted or not (`"MINIO_ROOT_USER":`,
#      `- "KEY=value"`, T-413 N-3), and `$${VAR}` is compose's escape — the
#      literal text `${VAR}`, not an interpolation; `minioadmin` appears nowhere
#      outside Docs/ and services/_archive/ (SEC-14);
#   4. MV_TELEGRAM_BOT_TOKEN reaches the `telegram-bot` service only, through
#      `environment` and never through an `env_file` shared with others (D-10);
#   5. NEO4J_PLUGINS is absent (T-17, SEC-32); OLLAMA_ORIGINS is not a wildcard
#      and OLLAMA_HOST is not 0.0.0.0 (SEC-15);
#   6. there is no `llama-server` service — it is a native process outside
#      compose (ADR-005 add. 2 p. 7) — and MV_LLM_URL points at loopback,
#      host.docker.internal, a service of this file or an RFC1918 address, but
#      never at a public host (SEC-15, ADR-005 add. 2 p. 3);
#   7. the always-loaded compose file starts on a clean machine (T-397), and
#      every variable it requires with `${VAR:?}` or `${VAR?}` — other than an
#      image pin of build/versions.env — is marked `[required]` in
#      .env.example: the README tells an operator to fill the marked ones and
#      nothing else (T-412 acceptance, T-413);
#   8. a variable has one default, and it is the manifest's (T-411, T-413;
#      contracts.md §16 p. 5):
#      - `${MV_X:-d}` is allowed when d equals the default declared for MV_X in
#        shared/env/vars.go. The one exception is the explicit set of network
#        addresses below (NETWORK_ADDRESSES: MV_KAFKA_BROKERS,
#        MV_MINIO_ENDPOINT, MV_CORE_URL, MV_QDRANT_ADDR, MV_NEO4J_URI,
#        MV_TELEGRAM_GATEWAY_URL): compose may name a service of its own
#        network instead — the value of another launch context, not a second
#        source — provided every item names a service of this file AND has the
#        shape of the manifest's default (scheme ↔ scheme, host:port ↔
#        host:port). The set is a list and not a guess from the shape of a
#        default: the guess took MV_CORE_ADDR, a listen address, for a network
#        one and refused MV_LLM_URL for no reason (review #2 T-411, N-4). A
#        network variable outside the set is refused with the hint to add it
#        — through the system architect, it is part of the contract;
#      - the same equality holds for a third-party variable that the manifest
#        declares WITH a default (DeclareExternal in shared/env/infra.go —
#        OLLAMA_*): compose must repeat that default, and a key with no value
#        is refused for it, because it would hand the container the image's
#        own default instead of ours;
#      - `${MV_X}` and `$MV_X` with no modifier are rejected when the manifest
#        default is not empty: a silent .env then hands the process an empty
#        value instead of it (Mi-2). So are `${MV_X:+x}` and `${MV_X+x}`, which
#        substitute compose's own text when the variable is set and an empty
#        one when it is not (N-6), and a default that is itself another
#        interpolation, `${MV_X:-${MV_Y}}` — a second source by construction;
#        the inner one is checked on its own (N-1);
#      - `$$` is compose's escape for a literal dollar: `$${MV_X:-d}` is text
#        for a shell in the container and is left alone, while `$$$MV_X` is
#        that escape followed by a real `$MV_X` (N-1, N-6). A YAML comment is
#        not interpolated and is not read (N-5);
#      - a name the manifest does not declare, or declares retired, is
#        rejected too — that is also what makes a parse miss loud.
#
# Rule 8, the shape of the fix it asks for: pass the variable through as a key
# with no value (`MV_X:` in a mapping, `- MV_X` in a list). Compose then sets it
# from .env when .env has it — empty included, which for an allow-list means
# nobody — and leaves it out of the container when .env is silent, so the
# manifest's default applies. `${MV_X}` and `${MV_X:-}` are NOT that: both hand
# the process an empty value, and shared/env treats set-to-empty as a value —
# which is why rule 8 rejects both whenever the manifest's default is not empty.
# Inside a longer string — a `command:`, a quoted text — there is no key to
# leave without a value, so the advice there is different (N-5): repeat the
# manifest's default or require the variable with `:?`.
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
# rules 1–6 and 8 like any other, and are deliberately outside rule 7 — both
# halves of it, the interpolation and the `[required]` marker.
#
# Exit code is non-zero on any violation.
#
# Usage:
#   scripts/compose-lint.sh [-f <compose file>]...
#   scripts/compose-lint.sh --fixtures
# The first -f replaces the default set of files; further -f add to it. The
# first file in the set is the one rule 7 applies to.
# --fixtures runs the linter over its own fixtures in testdata/compose-lint:
# every bad-*.yml must be rejected by exactly the rule its `# expect-rule: N`
# line names, and its message must contain every `# expect-text: ...` line of
# the fixture; every good-*.yml must pass. A fixture rejected by another rule
# proves nothing about its own (T-411 acceptance).
# Environment:
#   COMPOSE_LINT_ENV_FILES  space separated env files (default:
#                           "build/versions.env .github/ci.env")
#   COMPOSE_LINT_PROFILES   space separated profiles to resolve
#                           (default: all of them)
#   COMPOSE_LINT_FIXTURES   the directory --fixtures reads
#                           (default: testdata/compose-lint)
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
self="$repo_root/scripts/$(basename -- "${BASH_SOURCE[0]}")"
cd "$repo_root"

# The caller may be `make`, which exports COMPOSE_ENV_FILES, or a shell where
# the operator set COMPOSE_FILE/COMPOSE_PROFILES for their own stack. Every file
# and profile this linter looks at is passed explicitly below; inheriting any of
# them would make the result depend on whose terminal it ran in.
unset COMPOSE_ENV_FILES COMPOSE_FILE COMPOSE_PROFILES

# --------------------------------------------------------------------------
# --fixtures — the linter's own fixtures, each held to the rule it plants.
# --------------------------------------------------------------------------
run_fixtures() {
  local dir=${COMPOSE_LINT_FIXTURES:-testdata/compose-lint} failed=0 bad good expected got out text
  local ok stray lines n_bad=0 n_good=0
  # A moved, renamed or mistyped directory must not turn the self-test into a
  # silent pass (T-413 review #1 Mi-3): no fixtures is a failure, not a clean run.
  if [ ! -d "$dir" ]; then
    echo "compose-lint: the fixture directory $dir does not exist, so nothing checks the linter" >&2
    return 1
  fi
  # Only *.yml is read; a fixture spelled *.yaml would be skipped without a word.
  for stray in "$dir"/bad-*.yaml "$dir"/good-*.yaml; do
    [ -e "$stray" ] || continue
    echo "compose-lint: $stray is not read — fixtures end in .yml; rename it" >&2
    failed=1
  done
  for bad in "$dir"/bad-*.yml; do
    # No match leaves the pattern itself, and `set -e` would end the run there.
    [ -e "$bad" ] || continue
    n_bad=$((n_bad + 1))
    lines=$(grep -c '^# expect-rule:' "$bad" || true)
    if [ "$lines" -gt 1 ]; then
      echo "compose-lint: $bad has $lines '# expect-rule:' lines; a fixture plants exactly one rule" >&2
      failed=1
      continue
    fi
    expected=$(sed -n 's/^# expect-rule:[[:space:]]*\([0-9][0-9]*\)[[:space:]]*$/\1/p' "$bad" | head -n 1)
    if [ -z "$expected" ]; then
      echo "compose-lint: $bad has no '# expect-rule: N' line, so nothing says which rule it plants" >&2
      failed=1
      continue
    fi
    if out=$(bash "$self" -f "$bad" 2>&1); then
      echo "compose-lint: $bad broke rule $expected but passed the linter" >&2
      failed=1
      continue
    fi
    # Every rule that fired, not only the expected one: a fixture that is red
    # for two reasons stops proving either the moment one of them is fixed.
    got=$(printf '%s\n' "$out" | grep -oE '\[rule [0-9]+\]' | sort -u | tr '\n' ' ' || true)
    if [ "$got" != "[rule $expected] " ]; then
      {
        echo "compose-lint: $bad plants rule $expected but was rejected by: ${got:-no rule at all}"
        printf '%s\n' "$out" | sed 's/^/    /'
      } >&2
      failed=1
      continue
    fi
    ok=1
    while IFS= read -r text; do
      [ -n "$text" ] || continue
      if ! printf '%s\n' "$out" | grep -qF -- "$text"; then
        {
          echo "compose-lint: $bad was rejected by rule $expected, but not for the reason it plants — no '$text' in:"
          printf '%s\n' "$out" | sed 's/^/    /'
        } >&2
        ok=0
        failed=1
      fi
    done < <(sed -n 's/^# expect-text:[[:space:]]*//p' "$bad")
    # Said only on a full match: a wrong reason is not "as it must be".
    [ "$ok" = 1 ] && echo "compose-lint: $bad rejected by rule $expected, as it must be"
  done
  for good in "$dir"/good-*.yml; do
    [ -e "$good" ] || continue
    n_good=$((n_good + 1))
    if ! out=$(bash "$self" -f "$good" 2>&1); then
      {
        echo "compose-lint: $good is clean but the linter rejected it:"
        printf '%s\n' "$out" | sed 's/^/    /'
      } >&2
      failed=1
      continue
    fi
    echo "compose-lint: $good passed, as it must"
  done
  if [ "$n_bad" -eq 0 ] || [ "$n_good" -eq 0 ]; then
    echo "compose-lint: $dir holds $n_bad bad and $n_good good fixtures; the self-test needs both kinds, or it proves nothing" >&2
    failed=1
  fi
  [ "$failed" = 0 ] && echo "compose-lint: fixtures ok — $n_bad bad, $n_good good"
  return "$failed"
}

compose_files=()
while [ $# -gt 0 ]; do
  case "$1" in
  -f | --file)
    compose_files+=("$2")
    shift 2
    ;;
  --fixtures)
    run_fixtures
    exit $?
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
marks=$(mktemp)
trap 'rm -f "$clean_env" "$marks"' EXIT

primary=${compose_files[0]}
# A fixture may bring its own example file: testdata/compose-lint/bad-x.yml is
# paired with bad-x.env when that file exists. That is how the half of rule 7
# that reads .env.example gets a negative fixture of its own without teaching
# the fixture loop a second file pattern.
env_example=${primary%.yml}.env
[ -f "$env_example" ] || env_example=.env.example

if ! clean_out=$(CLEAN_ENV_PATH="$clean_env" MARKS_PATH="$marks" ENV_EXAMPLE_PATH="$env_example" \
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
marks = []
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
    required = "[required]" in comment
    # One reading of the marker for both halves of rule 7: the clean machine
    # below and the `${VAR:?}` ↔ `[required]` check of the model half.
    marks.append(f"{key.strip()}\t{'required' if required else 'optional'}")
    if not line.startswith(SKIP):
        out.append(f"{key}={DUMMY}" if (required and not value.strip()) else line)
    comment = ""

if problems:
    print("\n".join(problems), file=sys.stderr)
    sys.exit(1)

io.open(os.environ["CLEAN_ENV_PATH"], "w", encoding="utf-8", newline="\n").write(
    "\n".join(out) + "\n"
)
io.open(os.environ["MARKS_PATH"], "w", encoding="utf-8", newline="\n").write(
    "\n".join(marks) + "\n"
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

# The env files in the order the Makefile exports them (COMPOSE_ENV_FILES:
# .env, then build/versions.env): the pins win over the example, as they do
# for `make up`.
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
# Rules 1-6 and 8, and the marker half of rule 7 — the resolved model and the
# files as written, every file and every profile at once.
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
  COMPOSE_FILE_PATHS="${compose_files[*]}" MARKS_PATH="$marks" ENV_EXAMPLE_PATH="$env_example" \
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
# Reading the files as written. Rules 3, 7 and 8 look at the text, not at the
# model — the model has already substituted every value, so a literal password
# and a correct `${VAR:?}` look the same there. What compose itself does to a
# line is reproduced here and nowhere else, so that the three rules cannot
# disagree about it.
# --------------------------------------------------------------------------
NAME = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
BODY = re.compile(r"([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-?+])(.*))?", re.S)


class Ref:
    """One interpolation compose performs: `$NAME` or `${NAME<op><arg>}`, op
    being one of '', ':-', '-', ':?', '?', ':+', '+'. `inner` holds the
    interpolations nested in arg — compose resolves those too."""

    def __init__(self, name, op, arg, text, start, end, inner):
        self.name, self.op, self.arg, self.text = name, op, arg, text
        self.start, self.end, self.inner = start, end, inner


def interpolations(text):
    """Every interpolation of text, outermost first, each with its nested ones.
    `$$` is compose's escape for a literal dollar and yields nothing:
    `$${MV_X:-d}` is text for a shell in the container (review #1 T-411, N-1),
    and `$$$MV_X` is that escape followed by a real `$MV_X` (review #2, N-6)."""
    refs = []
    i, n = 0, len(text)
    while i < n:
        if text[i] != "$":
            i += 1
            continue
        if text.startswith("$$", i):
            i += 2
            continue
        if text.startswith("${", i):
            j, depth = i + 2, 1
            while j < n:
                if text.startswith("$$", j):
                    j += 2
                    continue
                if text.startswith("${", j):
                    depth += 1
                    j += 2
                    continue
                if text[j] == "}":
                    depth -= 1
                    if depth == 0:
                        break
                j += 1
            if j >= n:
                # Unclosed: `docker compose config` has refused the file already.
                break
            m = BODY.fullmatch(text, i + 2, j)
            if m:
                arg = m.group(3) or ""
                refs.append(Ref(m.group(1), m.group(2) or "", arg, text[i:j + 1],
                                i, j + 1, interpolations(arg)))
            i = j + 1
            continue
        m = NAME.match(text, i + 1)
        if m:
            refs.append(Ref(m.group(0), "", "", text[i:m.end()], i, m.end(), []))
            i = m.end()
        else:
            i += 1
    return refs


def walk(refs):
    for ref in refs:
        yield ref
        yield from walk(ref.inner)


def strip_comment(line):
    """The line without its YAML comment: a `#` at the start of the line or
    after whitespace, outside quotes (review #2 T-411, N-5). Compose does not
    interpolate a comment, so neither rule may read one."""
    quote = None
    i = 0
    while i < len(line):
        c = line[i]
        if quote:
            if quote == '"' and c == "\\":
                i += 2
                continue
            if c == quote:
                quote = None
        elif c in "\"'" and (i == 0 or line[i - 1] in " \t:-[{,"):
            quote = c
        elif c == "#" and (i == 0 or line[i - 1] in " \t"):
            return line[:i].rstrip()
        i += 1
    return line.rstrip()


def unquote(value):
    v = value.strip()
    if len(v) >= 2 and v[0] in "\"'" and v[-1] == v[0]:
        return v[1:-1]
    return v


LISTED = re.compile(r"([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?", re.S)
# `[ \t]*` before the colon: `KEY : value` is a key to YAML and to compose (v5.2
# reads `A7 : spaced` as A7), and a linter that wants the colon flush lets a
# literal password through in a file that aligns its colons (T-413 review #1
# Ma-1 — the old pattern allowed the space, the first T-413 one did not).
MAPPED = re.compile(r"""(["']?)([A-Za-z_][A-Za-z0-9_]*)\1[ \t]*:(?:[ \t]+(.*))?""", re.S)


def entry(code):
    """(key, value) of a line of an `environment` block, whatever its layout:
    `KEY: value`, `"KEY": value` (N-3), `- KEY=value`, `- "KEY=value"`. The
    value is None for a key with no value (`KEY:`, `- KEY`), and keeps its
    quotes otherwise. None for a line that is not a key at all. `code` is a
    line with its comment already stripped."""
    body = code.strip()
    if body == "-" or body.startswith("- "):
        body = body[1:].strip()
        # `- "KEY=value"`: in a list the quotes wrap the whole item.
        body = unquote(body)
        m = LISTED.fullmatch(body)
        return (m.group(1), m.group(2)) if m else None
    m = MAPPED.fullmatch(body)
    if not m:
        return None
    return m.group(2), (m.group(3) if m.group(3) else None)


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

# Credentials as written, in one pass over every file. Layout is deliberately
# not part of the rule — `environment` may be a mapping (`KEY: value`) or a
# list (`- KEY=value`), a key may be quoted, and a secret may sit in any anchor
# (`x-platform-env` and whatever the next one is called), not only in a service
# block. Anchoring on any of them would leave exactly the hole this rule exists
# to close. The nearest enclosing key is tracked only to name the place in the
# message.
TOP_LEVEL = re.compile(r"^([A-Za-z0-9_][A-Za-z0-9._-]*):")
BLOCK = re.compile(r"^  ([A-Za-z0-9][A-Za-z0-9._-]*):\s*$")

for compose_path, raw in raw_files:
    context = compose_path
    for lineno, line in enumerate(raw.splitlines(), 1):
        code = strip_comment(line)
        if not code.strip():
            continue
        e = entry(code)
        # A key with no value: compose fills it from .env when .env has it and
        # otherwise leaves it out of the container — for a credential the
        # stack must refuse to start without, that is a silent fallback to
        # whatever the image does on its own (MinIO: minioadmin). Checked
        # before BLOCK, because at indent 2 inside an anchor it looks exactly
        # like the name of a block (T-411).
        if e and e[1] is None and e[0] in MUST_BE_REQUIRED:
            fail(
                3,
                f"{compose_path} {context}:{lineno}: {e[0]} is passed through "
                "with no value; an unset credential would just be left out, so it "
                "needs ${VAR:?}",
            )
            continue
        m = TOP_LEVEL.match(code) or BLOCK.match(code)
        if m:
            context = m.group(1)
            continue
        if not e or e[1] is None or not SECRET_KEY.match(e[0]):
            continue
        key, value = e
        # Real interpolations only: `$${KEY:?}` hands the container the literal
        # text, which is a default credential like any other.
        refs = [r for r in interpolations(value) if r.name]
        if not refs:
            fail(
                3,
                f"{compose_path} {context}:{lineno}: {key} carries the literal "
                f"{value!r}; use ${{{key}:?...}}",
            )
            continue
        if key in MUST_BE_REQUIRED and not any(r.op == ":?" for r in refs):
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
    for entry_ in svc.get("env_file") or []:
        path = entry_.get("path") if isinstance(entry_, dict) else entry_
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
# Rule 7, the marker half — every `${VAR:?}` of the always-loaded file is a
# variable the README tells the operator to fill
# --------------------------------------------------------------------------
# The first half of rule 7 (above, in bash) proves the file interpolates on a
# clean machine. This half proves the clean machine is the one the README
# describes: a variable compose requires and .env.example does not mark
# `[required]` is one an operator following the instructions skips — silently,
# when the example ships a value or when the requirement is `?` (set, even to
# empty, is enough). The marker is read by the example half of rule 7 and handed
# over in MARKS_PATH, so the two halves cannot read it differently. Image pins
# come from build/versions.env and are not an operator's to fill. The
# per-profile files are outside, exactly as for the first half: their required
# variables are the ones a clean machine has no reason to hold (T-397).
EXAMPLE = os.environ["ENV_EXAMPLE_PATH"]
marked = {}
with open(os.environ["MARKS_PATH"], encoding="utf-8") as fh:
    for row in fh:
        name, sep, flag = row.rstrip("\n").partition("\t")
        if sep:
            marked[name] = flag == "required"

primary_path, primary_raw = raw_files[0]
asked = set()
for lineno, line in enumerate(primary_raw.splitlines(), 1):
    for ref in walk(interpolations(strip_comment(line))):
        if not ref.name or not ref.op.endswith("?") or ref.name in versions or ref.name in asked:
            continue
        asked.add(ref.name)
        if ref.name not in marked:
            fail(
                7,
                f"{primary_path}:{lineno}: {ref.name} is required here (`{ref.op}`) and "
                f"absent from {EXAMPLE}; a clean machine never gets it",
            )
        elif not marked[ref.name]:
            fail(
                7,
                f"{primary_path}:{lineno}: {ref.name} is required here (`{ref.op}`) but "
                f"{EXAMPLE} does not mark it [required]: the README asks an operator to "
                "fill the marked variables and nothing else, so this one is skipped "
                "until `make up` refuses (T-412 acceptance). Put [required] in the "
                "comment right above it",
            )

# --------------------------------------------------------------------------
# Rule 8 — a variable has one default, and it is the manifest's
# --------------------------------------------------------------------------
# The manifest is read as source text and not through mvctl: this job runs
# without Go. `\s*` spans newlines, so a Declare whose name and default sit on
# lines of their own parses as well as a one-line one. What the pattern cannot
# read — a constant or a raw string for the name, an escaped quote in the
# default — leaves the name unknown, and an unknown name fails below; a
# concatenated default is read up to its first quote and fails as a mismatch.
# Either way a parse miss is loud. Lines that are Go comments are dropped first:
# dict() keeps the LAST match, so a commented-out `// Declare("MV_A", "old",
# ...)` below the real one would otherwise become the default this rule
# compares with — the one silent case (review #1 N-2).
MANIFEST = "shared/env/vars.go"
EXTERNALS = "shared/env/infra.go"


def go_source(path):
    with open(path, encoding="utf-8") as fh:
        return "\n".join(l for l in fh.read().splitlines() if not l.lstrip().startswith("//"))


manifest_src = go_source(MANIFEST)
manifest = dict(re.findall(r'\bDeclare\(\s*"(MV_[A-Z0-9_]+)"\s*,\s*"([^"\\]*)"', manifest_src))
retired = set(re.findall(r'\bDeclareDeprecated\(\s*"(MV_[A-Z0-9_]+)"', manifest_src))
if not manifest:
    fail(8, f"no Declare(\"MV_...\", ...) found in {MANIFEST}; nothing to compare the defaults with")
# Third-party variables the manifest gives a default of its own, and only the
# ones the contract names: contracts.md §16 p. 5 speaks of OLLAMA_*. COMPOSE_*
# carry defaults in infra.go too, but they are compose's own settings — compose
# sets COMPOSE_PROJECT_NAME itself, so `${COMPOSE_PROJECT_NAME}` is never empty
# and no process of the platform reads it (T-413 review #1 Mi-1). One without a
# default (MINIO_ROOT_USER, NEO4J_PASSWORD) has nothing to agree with and is
# rule 3's and rule 7's business.
EXTERNAL_PREFIX = "OLLAMA_"
external = {
    name: default
    for name, default in re.findall(
        r'\bDeclareExternal\(\s*"([A-Z][A-Z0-9_]*)"\s*,\s*"([^"\\]*)"', go_source(EXTERNALS))
    if default != "" and name.startswith(EXTERNAL_PREFIX)
}
if not external:
    fail(8, f"no DeclareExternal with a default found in {EXTERNALS}; the OLLAMA_* "
            "defaults of compose have nothing to be compared with")

# The explicit set of network addresses (contracts.md §16 p. 5, v0.7). For
# these, and only these, compose may name a service of its own network instead
# of the manifest's host address: the value of another launch context, not a
# second source. It is a list because the guess it replaced — "a variable whose
# manifest default looks like an address" — erred both ways (review #2 T-411,
# N-4). A new network variable is added here AND to the contract, by the
# system architect.
NETWORK_ADDRESSES = {
    "MV_KAFKA_BROKERS",
    "MV_MINIO_ENDPOINT",
    "MV_CORE_URL",
    "MV_QDRANT_ADDR",
    "MV_NEO4J_URI",
    "MV_TELEGRAM_GATEWAY_URL",
}
# Out of the set by the same decision, with the reason the refusal names.
NOT_NETWORK_ADDRESSES = {
    "MV_CORE_ADDR": "a listen address of the process itself, not the address of "
                    "a service; inside compose each service carries the literal its "
                    "published port and its healthcheck agree on (T-408)",
    "MV_MEMORY_URL": "its empty default means memory is off (FR-035), and compose "
                     "passes it as is, ${MV_MEMORY_URL:-}",
}

SCHEMED = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*://\S+")
HOST_PORT = re.compile(r"[A-Za-z0-9._-]+:[0-9]+")


def shape(item):
    """`scheme://` or `host:port`, the two shapes an address takes here."""
    if SCHEMED.fullmatch(item):
        return "scheme://"
    if HOST_PORT.fullmatch(item):
        return "host:port"
    return None


def items(value):
    return [i.strip() for i in value.split(",")]


def host(item):
    # A bare word has no host: `core` is a word that happens to be a service
    # name, and `telegram-bot` is a service AND a client id (review #1 Mi-1).
    return urlsplit(item).hostname if "://" in item else item.rpartition(":")[0]


def names_services(value):
    return all(shape(i) and host(i) in service_names for i in items(value))


for name in sorted(NETWORK_ADDRESSES):
    if name not in manifest:
        fail(8, f"{name} is in the set of network addresses of this rule but not "
                f"declared in {MANIFEST}; the set and the manifest have drifted apart")
    elif shape(manifest[name]) is None:
        fail(8, f"{name} is in the set of network addresses, but its manifest default "
                f"{manifest[name]!r} is neither host:port nor scheme://; there is no "
                "shape to hold compose to")


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


def fix_for(name, default, whole):
    """What to write instead. A key with no value exists only where the
    interpolation IS the value of an environment key; inside a command or a
    longer string there is no key to leave empty (review #2 T-411, N-5). For a
    third-party variable a key with no value hands the container the image's
    default, not ours (contracts.md §16 p. 5)."""
    if not name.startswith("MV_"):
        return f"write ${{{name}:-{default}}}, the default of {EXTERNALS}"
    if whole:
        return ("pass it through as a key with no value, so that .env decides and "
                "the manifest's default applies when .env is silent")
    return (f"it is part of a larger string here (a command, a quoted text), where "
            f"a key with no value is not available: repeat the manifest's default, "
            f"${{{name}:-{default}}}, or require it with ${{{name}:?...}}")


def check(where, ref, whole):
    for inner in ref.inner:
        check(where, inner, False)
    name = ref.name
    if name is None:
        return
    if name.startswith("MV_"):
        if unknown(where, name):
            return
        default, source = manifest[name], MANIFEST
    elif name in external:
        default, source = external[name], EXTERNALS
    else:
        return
    op = ref.op
    if op.endswith("?"):
        return  # no default at all: rule 7's
    if op.endswith("+"):
        fail(
            8,
            f"{where}: {ref.text} hands the process compose's own text when {name} "
            "is set and an EMPTY value when .env is silent - never the operator's "
            f"value and never the default of {source} (T-411 review #2 N-6); "
            + fix_for(name, default, whole),
        )
        return
    if op == "":
        if default != "" and not name.startswith("MV_"):
            # No process of the platform reads it, so the allow-list text below
            # does not apply; what goes wrong is the container (review #1 Mi-1).
            fail(
                8,
                f"{where}: {ref.text} has no default: when .env is silent the container "
                f"gets an EMPTY {name} instead of the default of {source} ({default!r}); "
                + fix_for(name, default, whole),
            )
        elif default != "":
            fail(
                8,
                f"{where}: {ref.text} has neither a default nor `:?`: when .env is "
                f"silent the process gets an EMPTY {name}, not the default of {source} "
                f"({default!r}), and shared/env reads set-to-empty as a value - for an "
                "allow-list that is nobody (T-411 review #1 Mi-2); "
                + fix_for(name, default, whole),
            )
        return
    value = ref.arg
    if ref.inner:
        fail(
            8,
            f"{where}: {name} falls back to another variable here ({value!r}), and "
            f"{source} declares {default!r}: one value with two sources by "
            "construction (T-411 review #1 N-1); " + fix_for(name, default, whole),
        )
        return
    if value == default:
        return
    if not name.startswith("MV_"):
        fail(
            8,
            f"{where}: {name} defaults to {value!r} here and to {default!r} in "
            f"{source} - two sources of one value (contracts.md §16 p. 5); "
            + fix_for(name, default, whole),
        )
    elif name in NETWORK_ADDRESSES:
        want = shape(default)
        misshapen = [i for i in items(value) if shape(i) != want]
        foreign = [i for i in items(value) if host(i) not in service_names]
        if misshapen:
            fail(
                8,
                f"{where}: {name} defaults to {value!r} here; {misshapen[0]!r} does not "
                f"have the same shape as the manifest's default {default!r} ({want}) - "
                "a network address may name a service of this network, but in the "
                "shape the process reads (contracts.md §16 p. 5)",
            )
        elif foreign:
            fail(
                8,
                f"{where}: {name} defaults to {value!r} here and to {default!r} in "
                f"{source}; {foreign[0]!r} is not a service of this compose network, so "
                "it is a second, invented source of the value (T-411). Every item "
                "must name a service of this file",
            )
    elif name in NOT_NETWORK_ADDRESSES:
        fail(
            8,
            f"{where}: {name} defaults to {value!r} here and to {default!r} in "
            f"{source}; it is {NOT_NETWORK_ADDRESSES[name]}, and contracts.md §16 "
            "p. 5 keeps it out of the set of network addresses; "
            + fix_for(name, default, whole),
        )
    else:
        hint = ""
        if names_services(value):
            hint = (f" If {name} really is the address of a service of this network, "
                    "add it to the explicit set NETWORK_ADDRESSES of rule 8 and to "
                    "contracts.md §16 p. 5 - through the system architect, it is a "
                    "change of the contract.")
        fail(
            8,
            f"{where}: {name} defaults to {value!r} here and to {default!r} in "
            f"{source} - two sources of one value (T-411); {name} is not in the "
            "explicit set of network addresses, the only variables compose may "
            "point at a service of its own network; "
            + fix_for(name, default, whole) + "." + hint,
        )


for compose_path, raw in raw_files:
    for lineno, line in enumerate(raw.splitlines(), 1):
        code = strip_comment(line)
        if not code.strip():
            continue
        where = f"{compose_path}:{lineno}"
        e = entry(code)
        if e and e[1] is None and e[0] in external:
            fail(
                8,
                f"{where}: {e[0]} is passed through with no value; with .env silent "
                "the container then runs on the image's own default, not on the "
                f"default of {EXTERNALS} ({external[e[0]]!r}); "
                + fix_for(e[0], external[e[0]], True),
            )
        value = unquote(e[1]) if e and e[1] is not None else None
        for ref in interpolations(code):
            check(where, ref, whole=value is not None and ref.text == value)

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
