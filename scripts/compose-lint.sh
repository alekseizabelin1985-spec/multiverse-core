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
# The nine rules:
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
#      compose (ADR-005 add. 2 p. 7) — and MV_LLM_URL and MV_OLLAMA_URL of a
#      service are LOCAL addresses: the rule is not this file's, it is
#      llm_endpoint_classify of scripts/lib/llm-endpoint.sh, held to the table
#      testdata/llm/local-endpoints.tsv (T-450). A cloud address is refused
#      (SEC-15, ADR-005 add. 2 p. 3), and so is an invalid one — 0.0.0.0, a
#      local host without a port — which is a configuration error the platform
#      would refuse at start;
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
#        own default instead of ours. Nor is anything but its own interpolation
#        alone allowed for it — a literal (`OLLAMA_X: 4`, `- OLLAMA_X=4`), even
#        one equal to the default, the text of another variable, or its own
#        interpolation with text around it: .env could then never set it as
#        set. Of the interpolations, rule 8 accepts `${OLLAMA_X:-<the default
#        of infra.go>}` and refuses `${OLLAMA_X-d}`, where a .env line
#        `OLLAMA_X=` hands the container an empty value; a requirement with
#        `:?`/`?` is left to rule 7, which does not read the per-profile files
#        (contracts.md §16 p. 5, v0.9, T-432). An OLLAMA_* that infra.go does
#        not declare is not this rule's;
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
#        rejected too — that is also what makes a parse miss loud;
#   9. a variable a context reads reaches every service that runs the context
#      (T-469; the class of T-055, T-310 and T-305 — a variable declared in
#      the manifest and never passed to its container, whose process then
#      silently takes the manifest's default whatever .env says):
#      - what a service runs is read from the resolved model: the platform
#        process with the contexts of `--contexts=a,b` (or `--contexts a,b`,
#        `all` meaning every one) in the rest of an entrypoint /multiverse and
#        in its command — the last `--contexts` wins, as in the Go flag
#        package — and the Telegram bot by the entrypoint /telegram-bot. A
#        service with neither is not this rule's; a context the rule does not
#        know is refused by name;
#      - who reads a variable is the explicit table READERS below — contexts,
#        `process` for cmd/multiverse whatever its contexts, `telegram-bot` for
#        the bot. A variable that is read but never passed to a container is in
#        NOT_IN_CONTAINERS, with the reason; the variables the manifest marks
#        Tooling() are the host's and are left out by that mark. The table is
#        a list in this file and not a new mark of shared/env: that would be a
#        change of contracts.md §16 p. 5 (the orchestrator's decision, T-469);
#      - every variable the manifest declares is in exactly one of the two
#        tables or marked Tooling(), and every name of the tables is declared
#        and not retired: a new Declare without a row fails here, so the next
#        variable cannot be forgotten the way those three were;
#      - a service passes a variable when its `environment` has the key, as
#        written (with a value or without) or in the resolved model — which is
#        where compose puts the variables of an `env_file`; the linter never
#        opens an env file of a service itself.
#        The form of the value is the business of rules 3 and 8; the refusal
#        names the service, the variable, its reader and the form to add.
#
# Where the values come from. Rules 1-6 read the resolved model, `docker
# compose config --format json` over every file and profile at once. Rules 3, 7
# and 8 — and the source of an image for rule 1 — cannot: interpolation has
# already happened there, and a literal password looks exactly like a correct
# `${VAR:?}`. They read each file on its own through `docker compose config
# --no-interpolate --no-consistency --format json --profile '*'`: every value
# after YAML has parsed it and before compose has interpolated it. Quotes,
# `''`, `\x24`, block scalars, flow mappings, continuation lines, `KEY :` and
# comments are thereby compose's own reading, not a copy of it — the line
# reader this replaced missed each of them in turn (T-429; T-413 review #1
# N-1, review #2). What is reproduced here is the interpolation alone, and it
# follows compose there too:
# the message of `${VAR:?msg}` is evaluated only when compose is about to refuse
# anyway, so nothing in it is checked, and `${VAR:-$${X}}` closes at the second
# brace. Keys are not interpolated by compose and are not read as text. A place
# is named by its path in the model (`services.core.environment.MV_X`), and a
# finding repeated by an anchor merged into several services is reported once,
# with every place. Rule 9 reads both: what a service runs from the resolved
# model, the keys it passes from the uninterpolated reads — a key with no value
# that .env leaves unset is still a line of the file (T-469).
#
# Rule 8, the shape of the fix it asks for — the form of passing (T-469): write
# `${MV_X:-<the manifest's default>}`, `${MV_X:-}` for an optional secret and
# `${MV_X:?...}` for a required variable. Compose then hands the process the
# value of .env when .env holds one, and the manifest's default both when .env
# is silent and when its line is empty; the value is visible in `docker compose
# config`, and it is the form of the neighbours. `${MV_X}` is NOT that: it hands
# the process an empty value, and shared/env treats set-to-empty as a value —
# which is why rule 8 rejects it whenever the manifest's default is not empty.
# The one exception is the allow-lists of EMPTY_IS_NOBODY: for them an empty
# line in .env is a value, nobody, and `:-` would replace it with the default,
# so they stay a key with no value (`MV_X:` in a mapping, `- MV_X` in a list) —
# compose sets it from .env when .env has it, empty included, and leaves it out
# of the container when .env is silent (T-411). Rule 8 accepts both forms; only
# the advice differs. Inside a longer string — a `command:`, a quoted text —
# there is no key to leave without a value, so the advice there is different
# (N-5): repeat the manifest's default or require the variable with `:?`.
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
# first file in the set is the one rule 7 applies to. A relative -f is relative
# to the directory the linter is called from, as for docker compose itself
# (T-413 review #1 N-3); a file that does not exist is refused by name.
# --fixtures runs the linter over its own fixtures in testdata/compose-lint:
# every bad-*.yml must be rejected by exactly the rule its `# expect-rule: N`
# line names, and its message must contain every `# expect-text: ...` line of
# the fixture and none of its `# expect-absent: ...` lines, in any letter case
# (T-463); every good-*.yml must pass. A fixture rejected by another rule
# proves nothing about its own (T-411 acceptance). The fixtures run side by
# side and are judged in a fixed order afterwards (T-429).
# Environment:
#   COMPOSE_LINT_ENV_FILES  space separated env files (default:
#                           "build/versions.env .github/ci.env")
#   COMPOSE_LINT_PROFILES   space separated profiles to resolve
#                           (default: all of them)
#   COMPOSE_LINT_FIXTURES   the directory --fixtures reads
#                           (default: testdata/compose-lint)
#   COMPOSE_LINT_JOBS       how many fixtures --fixtures runs at once
#                           (default: the number of processors)
# =============================================================================
set -euo pipefail

# Builtins only on the way in, here and in the fixture loop: a fork costs tens
# of milliseconds in Git Bash, and --fixtures starts this script fifty times.
case ${BASH_SOURCE[0]} in
*/*) script_dir=${BASH_SOURCE[0]%/*} ;;
*) script_dir=. ;;
esac
caller_pwd=$PWD
CDPATH='' cd -- "$script_dir/.."
repo_root=$PWD
self="$repo_root/scripts/${BASH_SOURCE[0]##*/}"

# Python prints the refusals, and on Windows a pipe would otherwise get them in
# the console code page — `§` of a message would then never match the `§` of a
# fixture's `# expect-text:` line, which is UTF-8 like every file here.
export PYTHONIOENCODING=utf-8

# The caller may be `make`, which exports COMPOSE_ENV_FILES, or a shell where
# the operator set COMPOSE_FILE/COMPOSE_PROFILES for their own stack. Every file
# and profile this linter looks at is passed explicitly below; inheriting any of
# them would make the result depend on whose terminal it ran in.
unset COMPOSE_ENV_FILES COMPOSE_FILE COMPOSE_PROFILES

# The names the given env files declare — `KEY=...`, `export KEY=...` and the
# YAML form `KEY: value`, the lines compose's dotenv parser reads as variables —
# into the array `names`. A comment never matches: `#` does not start a name.
# Builtins only, for the same reason as above. A missing file declares nothing:
# compose refuses a missing --env-file by itself.
declared_names() {
  local file line re='^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*[=:]'
  names=()
  for file in "$@"; do
    [ -f "$file" ] || continue
    while IFS= read -r line || [ -n "$line" ]; do
      if [[ $line =~ $re ]]; then
        names+=("${BASH_REMATCH[2]}")
      fi
    done <"$file"
  done
  return 0
}

# The env files the model of rules 1-6 is resolved with unless
# COMPOSE_LINT_ENV_FILES says otherwise. One place: the hostile environment of
# --fixtures must be made of exactly the files the linter reads.
default_env_files='build/versions.env .github/ci.env'

# --------------------------------------------------------------------------
# --fixtures — the linter's own fixtures, each held to the rule it plants.
# --------------------------------------------------------------------------
run_fixtures() {
  local dir=${COMPOSE_LINT_FIXTURES:-testdata/compose-lint} failed=0 bad good expected got out text
  local ok stray line n n_bad=0 n_good=0 work limit i running rc near rest rule root
  local -a bads=() wants=() goods=()
  local -A fired=()
  # The loop reads with builtins — `read`, `[[ =~ ]]`, pattern matching — and
  # forks only to run the linter. With fifty fixtures, the `grep`, `sed` and
  # `cat` of each one cost Git Bash more than the runs themselves (T-429).
  local re_rule='^# expect-rule:' re_num='^# expect-rule:[[:space:]]*([0-9]+)[[:space:]]*$'
  local re_text='^# expect-text:[[:space:]]*(.*)$' re_fired='\[rule ([0-9]+)\]'
  local re_absent='^# expect-absent:[[:space:]]*(.*)$' folded
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
    n=0
    expected=""
    while IFS= read -r line || [ -n "$line" ]; do
      line=${line%$'\r'}
      [[ $line =~ $re_rule ]] || continue
      n=$((n + 1))
      if [ -z "$expected" ] && [[ $line =~ $re_num ]]; then
        expected=${BASH_REMATCH[1]}
      fi
    done <"$bad"
    if [ "$n" -gt 1 ]; then
      echo "compose-lint: $bad has $n '# expect-rule:' lines; a fixture plants exactly one rule" >&2
      failed=1
      continue
    fi
    if [ -z "$expected" ]; then
      echo "compose-lint: $bad has no '# expect-rule: N' line, so nothing says which rule it plants" >&2
      failed=1
      continue
    fi
    bads+=("$bad")
    wants+=("$expected")
  done
  for good in "$dir"/good-*.yml; do
    [ -e "$good" ] || continue
    n_good=$((n_good + 1))
    goods+=("$good")
  done

  # Each fixture is a run of its own — three calls of `docker compose config`
  # and two of Python, about a second — and nothing is shared between them, so
  # they run side by side (T-429: one after another they took 90 s). The
  # verdicts are read afterwards in the order of the files, so the output does
  # not depend on which run finished first. One more run takes the first good
  # fixture by a path relative to its own directory, from that directory: a
  # relative -f is the caller's, not the repository root's (T-413 review #1 N-3).
  work=$(mktemp -d)
  # shellcheck disable=SC2064 # the directory is known now and must go whatever happens
  trap "rm -rf '$work'" EXIT
  limit=${COMPOSE_LINT_JOBS:-$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 4)}
  # Every run below starts in a hostile environment: each name the env files of
  # the linter, build/versions.env and .env.example declare is exported EMPTY —
  # what the CI job did to CHROMA_IMAGE by exporting build/versions.env through
  # $GITHUB_ENV (T-455). The linter takes its values from the files alone, so no
  # verdict changes; a linter that lets the caller's environment shadow its
  # --env-file fails the fixtures that take a value from those files, the good
  # ones included, on the owner's machine and not only in CI. The names of
  # .env.example are exported too, but nothing here can catch a linter that
  # keeps them: see the note where the linter drops them.
  local name
  local -a lint_env_files
  read -r -a lint_env_files <<<"${COMPOSE_LINT_ENV_FILES:-$default_env_files}"
  declared_names "${lint_env_files[@]}" build/versions.env .env.example
  for name in "${names[@]}"; do
    export "$name=" 2>/dev/null || true
  done
  i=0
  running=0
  for bad in "${bads[@]}" "${goods[@]}"; do
    if [ "$running" -ge "$limit" ]; then
      wait -n || true
      running=$((running - 1))
    fi
    (
      set +e
      bash "$self" -f "$bad" >"$work/$i.out" 2>&1
      echo $? >"$work/$i.rc"
    ) &
    running=$((running + 1))
    i=$((i + 1))
  done
  if [ ${#goods[@]} -gt 0 ]; then
    good=${goods[0]}
    (
      set +e
      cd -- "${good%/*}" || exit
      bash "$self" -f "${good##*/}" >"$work/near.out" 2>&1
      echo $? >"$work/near.rc"
    ) &
  fi
  wait

  i=0
  for bad in "${bads[@]}"; do
    expected=${wants[$i]}
    out=""
    rc=2
    [ -f "$work/$i.out" ] && { IFS= read -r -d '' out || true; } <"$work/$i.out"
    [ -f "$work/$i.rc" ] && { read -r rc || true; } <"$work/$i.rc"
    i=$((i + 1))
    if [ "$rc" = 0 ]; then
      echo "compose-lint: $bad broke rule $expected but passed the linter" >&2
      failed=1
      continue
    fi
    # Every rule that fired, not only the expected one: a fixture that is red
    # for two reasons stops proving either the moment one of them is fixed.
    fired=()
    rest=$out
    while [[ $rest =~ $re_fired ]]; do
      fired[${BASH_REMATCH[1]}]=1
      rest=${rest#*"${BASH_REMATCH[0]}"}
    done
    if [ "${#fired[@]}" -ne 1 ] || [ -z "${fired[$expected]:-}" ]; then
      got=""
      for rule in "${!fired[@]}"; do
        got+="[rule $rule] "
      done
      {
        echo "compose-lint: $bad plants rule $expected but was rejected by: ${got:-no rule at all}"
        printf '%s\n' "$out" | sed 's/^/    /'
      } >&2
      failed=1
      continue
    fi
    ok=1
    folded=${out,,}
    while IFS= read -r line || [ -n "$line" ]; do
      line=${line%$'\r'}
      # `# expect-absent: ...` is a fragment the refusal must NOT contain, in
      # any letter case: a fake password planted in the value (T-463). This
      # failure does not echo the report — it holds the fragment. An
      # `expect-text` failure of the same fixture below still echoes it, and a
      # broken mask fails both, so the fragment does reach the output then; it
      # is the fake value of a committed fixture (T-463 review #2 N-8).
      if [[ $line =~ $re_absent ]]; then
        text=${BASH_REMATCH[1]}
        [ -n "$text" ] || continue
        if [[ $folded == *"${text,,}"* ]]; then
          echo "compose-lint: $bad was rejected by rule $expected, but the report prints '$text', which its '# expect-absent:' line forbids" >&2
          ok=0
          failed=1
        fi
        continue
      fi
      [[ $line =~ $re_text ]] || continue
      text=${BASH_REMATCH[1]}
      [ -n "$text" ] || continue
      if [[ $out != *"$text"* ]]; then
        {
          echo "compose-lint: $bad was rejected by rule $expected, but not for the reason it plants — no '$text' in:"
          printf '%s\n' "$out" | sed 's/^/    /'
        } >&2
        ok=0
        failed=1
      fi
    done <"$bad"
    # Said only on a full match: a wrong reason is not "as it must be".
    [ "$ok" = 1 ] && echo "compose-lint: $bad rejected by rule $expected, as it must be"
  done
  for good in "${goods[@]}"; do
    out=""
    rc=2
    [ -f "$work/$i.out" ] && { IFS= read -r -d '' out || true; } <"$work/$i.out"
    [ -f "$work/$i.rc" ] && { read -r rc || true; } <"$work/$i.rc"
    i=$((i + 1))
    if [ "$rc" != 0 ]; then
      {
        echo "compose-lint: $good is clean but the linter rejected it:"
        printf '%s\n' "$out" | sed 's/^/    /'
      } >&2
      failed=1
      continue
    fi
    echo "compose-lint: $good passed, as it must"
  done
  if [ ${#goods[@]} -gt 0 ]; then
    near=${goods[0]##*/}
    # The run of the same fixture from the root is the one right after the bad
    # ones; only when it passed does a failure here say anything about -f.
    root=2
    [ -f "$work/${#bads[@]}.rc" ] && { read -r root || true; } <"$work/${#bads[@]}.rc"
    rc=2
    [ -f "$work/near.rc" ] && { read -r rc || true; } <"$work/near.rc"
    if [ "$root" != 0 ]; then
      echo "compose-lint: $near was not checked as -f $near from its own directory: it does not pass from the repository root either (see above)" >&2
      failed=1
    elif [ "$rc" != 0 ]; then
      {
        echo "compose-lint: $near passes from the repository root but not as -f $near from its own directory:"
        sed 's/^/    /' "$work/near.out"
      } >&2
      failed=1
    else
      echo "compose-lint: $near passed as -f $near from its own directory, as it must"
    fi
  fi
  if [ "$n_bad" -eq 0 ] || [ "$n_good" -eq 0 ]; then
    echo "compose-lint: $dir holds $n_bad bad and $n_good good fixtures; the self-test needs both kinds, or it proves nothing" >&2
    failed=1
  fi
  [ "$failed" = 0 ] && echo "compose-lint: fixtures ok — $n_bad bad, $n_good good"
  return "$failed"
}

# A relative -f is relative to the directory the linter was called from, as it
# is for docker compose; resolving it against the repository root, where this
# script works, turned a mistyped path into a refusal of rule 7 that spoke of a
# clean machine (T-413 review #1 N-3). A path inside the repository is shown
# relative to its root, as it always was.
resolve_file() {
  local path=${1//\\//}
  case "$path" in
  /* | [A-Za-z]:/*) ;;
  *) path="$caller_pwd/$path" ;;
  esac
  if [ ! -f "$path" ]; then
    echo "compose-lint: no such compose file: $1 (looked for $path)" >&2
    return 1
  fi
  # Normalised by the shell itself (`..`, `.`, a drive letter), without a fork.
  CDPATH='' cd -- "${path%/*}"
  resolved="$PWD/${path##*/}"
  cd -- "$repo_root"
  case "$resolved" in
  "$repo_root"/*) resolved=${resolved#"$repo_root"/} ;;
  esac
}

compose_files=()
while [ $# -gt 0 ]; do
  case "$1" in
  -f | --file)
    if [ $# -lt 2 ]; then
      echo "compose-lint: $1 needs a compose file" >&2
      exit 2
    fi
    resolve_file "$2" || exit 2
    compose_files+=("$resolved")
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

read -r -a env_files <<<"${COMPOSE_LINT_ENV_FILES:-$default_env_files}"
read -r -a profiles <<<"${COMPOSE_LINT_PROFILES:-gpu memory dev legacy bot}"

if command -v python3 >/dev/null 2>&1; then
  python_bin=python3
elif command -v python >/dev/null 2>&1; then
  python_bin=python
else
  echo "compose-lint: python3 is required to read the compose model" >&2
  exit 2
fi

profile_args=()
for p in "${profiles[@]}"; do
  [ -n "$p" ] && profile_args+=(--profile "$p")
done
env_args=()
for f in "${env_files[@]}"; do
  [ -n "$f" ] && env_args+=(--env-file "$f")
done

primary=${compose_files[0]}
# A fixture may bring its own example file: testdata/compose-lint/bad-x.yml is
# paired with bad-x.env when that file exists. That is how the half of rule 7
# that reads .env.example gets a negative fixture of its own without teaching
# the fixture loop a second file pattern.
env_example=${primary%.yml}.env
[ -f "$env_example" ] || env_example=.env.example

# The values come from the files, never from the caller (T-455). Compose takes a
# variable from the environment of its own process before any --env-file, so a
# name the caller exports shadows the file — empty included. The CI job exports
# build/versions.env through $GITHUB_ENV, its CHROMA_IMAGE is empty by decision
# D-3, and that empty value hid the placeholder of .github/ci.env: the model
# refused docker-compose.legacy.yml in CI and resolved on every machine that had
# not exported the pins. Rule 7's clean machine is made of files as well. Every
# name the files of this run declare is therefore dropped before the first
# `docker compose`. `--fixtures` holds the linter to it for the names of its
# --env-file. Dropping the names of the example file is a precaution no fixture
# can check: a fixture's `${VAR:?}` must resolve in the model of rules 1-6
# first, and that model reads the --env-file alone.
declared_names "${env_files[@]}" build/versions.env "$env_example"
for name in "${names[@]}"; do
  unset -v "$name" 2>/dev/null || true
done

work=$(mktemp -d)
clean_env="$work/clean.env"
marks="$work/marks.tsv"
# `wait` first: a model still being written when a rule refuses early must not
# race the removal of its directory.
trap 'wait; rm -rf "$work"' EXIT

# --------------------------------------------------------------------------
# The models, started now and read later: none of them depends on rule 7, and
# each `docker compose config` costs half a second, so they run while it does.
#   model.json  — every file and every profile of the list, interpolated with
#                 the env files: what the containers get (rules 1-6);
#   raw.N.json  — file N on its own, every profile ('*'), NOT interpolated: the
#                 values as written, after YAML (rules 3, 7, 8 and the source of
#                 an image for rule 1). One file at a time, so that a finding
#                 names its file. A per-profile file is not a whole project
#                 on its own — telegram-bot depends on `gateway` of the main
#                 file — so the consistency check is off: compose v5.2 skips
#                 it for an uninterpolated read anyway (and does not filter
#                 such a read by profile either), but a compose that builds
#                 the project first would refuse the file, and would drop
#                 the services outside the profiles it enables — hence
#                 --no-consistency and '*'. Neither changes a byte of the
#                 v5.2 output.
#   raw.N.yml   — the file as text, for the one textual check of rule 5.
# The env files are passed to the uninterpolated reads as well: without them
# compose would load the .env of the project directory, the operator's own.
# --------------------------------------------------------------------------
args=(compose)
for f in "${compose_files[@]}"; do
  args+=(-f "$f")
done
# Every profile at once: the resolved model keeps each service's `profiles`, so
# one pass covers the whole topology and rule 2 can still tell a `dev` console
# from a production port.
args+=("${env_args[@]}" "${profile_args[@]}")
docker "${args[@]}" config --format json >"$work/model.json" 2>"$work/model.err" &
model_pid=$!
raw_pids=()
: >"$work/names"
for i in "${!compose_files[@]}"; do
  docker compose -f "${compose_files[$i]}" "${env_args[@]}" --profile '*' \
    config --no-interpolate --no-consistency --format json >"$work/raw.$i.json" 2>"$work/raw.$i.err" &
  raw_pids+=($!)
  cp -- "${compose_files[$i]}" "$work/raw.$i.yml"
  printf '%s\n' "${compose_files[$i]}" >>"$work/names"
done

# --------------------------------------------------------------------------
# Rule 7 — the always-loaded file starts on a clean machine.
# Judged first and on its own: if this fails, nothing else about the file
# matters to an operator who cannot get past `docker compose config`.
# --------------------------------------------------------------------------

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
# Rules 1-6 and 8, and the marker half of rule 7 — the models started above.
# --------------------------------------------------------------------------
if ! wait "$model_pid"; then
  cat "$work/model.err" >&2
  exit 1
fi
for i in "${!raw_pids[@]}"; do
  if ! wait "${raw_pids[$i]}"; then
    echo "compose-lint: docker compose cannot read ${compose_files[$i]} as written (config --no-interpolate):" >&2
    cat "$work/raw.$i.err" >&2
    exit 1
  fi
done

# Rule 6 needs the ONE rule of a local address, and that rule is a bash function
# of scripts/lib/llm-endpoint.sh — the same one llm-server and llm-bench call,
# held to testdata/llm/local-endpoints.tsv (T-450). This file used to carry its
# own copy in Python (a service of this file, loopback or whatever `ipaddress`
# calls private), and the copies disagreed: http://ollama:11434 passed here and
# stopped the platform. So the addresses are taken out of the model by Python,
# judged here by the function, and the verdicts handed back to rule 6.
# NUL-separated both ways: a value of compose may hold any character but NUL.
WORK_DIR="$work" "$python_bin" - <<'PY'
import json
import os

work = os.environ["WORK_DIR"]
with open(os.path.join(work, "model.json"), encoding="utf-8") as fh:
    services = json.load(fh).get("services") or {}
with open(os.path.join(work, "llm-urls.bin"), "wb") as out:
    for name in sorted(services):
        env = services[name].get("environment") or {}
        for key in ("MV_LLM_URL", "MV_OLLAMA_URL"):
            url = env.get(key)
            if url:
                out.write(f"{name}\0{key}\0{url}\0".encode("utf-8"))
PY
# shellcheck source=scripts/lib/llm-endpoint.sh
. "$repo_root/scripts/lib/llm-endpoint.sh"
: >"$work/llm-verdicts.bin"
while IFS= read -r -d '' llm_service && IFS= read -r -d '' llm_key && IFS= read -r -d '' llm_url; do
  llm_endpoint_classify "$llm_url" "$llm_key"
  # An @ anywhere in the value may end a password with a bare / in it, so rule
  # 6 names the host of such a value in NO sentence — its own sentence of the
  # cloud included, which does not come from the judge (C-15 v1.5, T-463 review
  # #1 Ma-1). The flag travels with the verdict; the host itself is not blanked,
  # so the verdict stays what the function said.
  llm_masked=0
  case "$llm_url" in
  *@*) llm_masked=1 ;;
  esac
  printf '%s\0%s\0%s\0%s\0%s\0%s\0%s\0' "$llm_service" "$llm_key" "$LLM_CLASS" "$LLM_CLASS_KIND" \
    "$LLM_CLASS_HOST" "$LLM_CLASS_ERROR" "$llm_masked" >>"$work/llm-verdicts.bin"
done <"$work/llm-urls.bin"

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

WORK_DIR="$work" MINIOADMIN_HITS="$minioadmin_hits" \
  MARKS_PATH="$marks" ENV_EXAMPLE_PATH="$env_example" \
  "$python_bin" - <<'PY'
import json
import os
import re
import sys
from urllib.parse import urlsplit

work = os.environ["WORK_DIR"]


def read(name, parse=False):
    with open(os.path.join(work, name), encoding="utf-8") as fh:
        return json.load(fh) if parse else fh.read()


model = read("model.json", parse=True)
compose_paths = read("names").splitlines()
# (file, the file as compose reads it before interpolation): rules 1, 3, 7, 8.
raw_models = [(path, read(f"raw.{i}.json", parse=True) or {}) for i, path in enumerate(compose_paths)]
# (file, its text): the one textual check of rule 5.
raw_files = [(path, read(f"raw.{i}.yml")) for i, path in enumerate(compose_paths)]

services = model.get("services") or {}
service_names = set(services)

# The uninterpolated reads enable every profile with '*'. Compose v5.2 does not
# filter such a read by profile anyway, but a compose that does, and does not
# know the wildcard, would enable a profile literally named '*' and drop every
# service that has a profile — rules 3, 7 and 8 would then pass them in
# silence. Every service of the resolved model must be in one of the reads.
written = set()
for _, raw in raw_models:
    written |= set(raw.get("services") or {})
if service_names - written:
    print(
        f"compose-lint: {', '.join(sorted(service_names - written))} missing from the "
        "uninterpolated read (config --no-interpolate --profile '*'); this docker compose "
        "does not enable every profile with '*', so rules 3, 7 and 8 cannot see them",
        file=sys.stderr,
    )
    sys.exit(2)

failures = []


def fail(rule, message):
    failures.append(f"[rule {rule}] {message}")


# A finding of rules 3 and 8 at one place of a model. The model repeats an
# anchor in every service that merges it, so one finding is reported once,
# with all of its places, in the order they were found.
noted = {}


def note(rule, where, message):
    noted.setdefault((rule, message), []).append(where)


def flush():
    for (rule, message), places in noted.items():
        shown = ", ".join(places[:3])
        if len(places) > 3:
            shown += f" and {len(places) - 3} more"
        fail(rule, f"{shown}: {message}")
    noted.clear()


# --------------------------------------------------------------------------
# The values as written. Rules 3, 7 and 8 read raw_models, not the model: the
# model has already substituted every value, so a literal password and a
# correct `${VAR:?}` look the same there. YAML is compose's business (config
# --no-interpolate, see the header); what compose does to a value after that —
# the interpolation — is reproduced here and nowhere else, so that the three
# rules cannot disagree about it.
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
            # The closing brace is found as compose finds it: every `${` opens
            # a level, the one inside `$${` included — that escape is undone
            # only when the default itself is interpolated, so `${U:-$${Q}}`
            # ends at the second brace. This loop used to skip `$$` and close
            # at the first (T-413 review #1 N-1).
            j, depth = i + 2, 1
            while j < n:
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
    """Every interpolation compose may perform, nested ones included — but not
    inside the message of `:?` or `?`. Compose evaluates that message only on
    its way to a refusal, so nothing in it ever reaches a container or asks
    anything more of .env (T-413 review #1 N-1: `${MV_LLM_URL:?need
    ${MV_WORLD_ID}}` was refused, and compose starts with it)."""
    for ref in refs:
        yield ref
        if not ref.op.endswith("?"):
            yield from walk(ref.inner)


LISTED = re.compile(r"([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?", re.S)


class Item:
    """One scalar of a model, at `where` (`services.core.environment.MV_X`).
    An entry — a mapping key with its value, or a list item `NAME` or
    `NAME=value` as `environment` and `args` write them — has a `key`, and its
    `value` is None for a key with no value (`KEY:`, `KEY: null`, `- KEY`).
    `text` is the whole string compose interpolates, None for a null.
    `parent` is the path of the mapping or list that holds it."""

    def __init__(self, where, key, value, text, parent):
        self.where, self.key, self.value, self.text = where, key, value, text
        self.parent = parent


def text_of(value):
    """A scalar as compose hands it on: `8080` and `true` reach a container as
    text. Only a string can hold an interpolation."""
    if value is None or isinstance(value, str):
        return value
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def scalars(node, where=""):
    """Every scalar of node, depth first, in the order the file has them. A
    list item is an entry whenever it reads as one, whatever the list is: the
    rules are about values, and a credential in a list outside `environment` is
    no less a credential. Keys are never text — compose does not interpolate
    them."""
    if isinstance(node, dict):
        for key, value in node.items():
            here = f"{where}.{key}" if where else str(key)
            if isinstance(value, (dict, list)):
                yield from scalars(value, here)
            else:
                yield Item(here, key, text_of(value), text_of(value), where)
    elif isinstance(node, list):
        for n, value in enumerate(node):
            here = f"{where}[{n}]"
            if isinstance(value, (dict, list)):
                yield from scalars(value, here)
                continue
            text = text_of(value)
            m = LISTED.fullmatch(text) if text is not None else None
            if m:
                yield Item(here, m.group(1), m.group(2), text, where)
            else:
                yield Item(here, None, None, text, where)


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

# `image:` as written, per service, so that rule 1 can see the variable and not
# only the value it interpolated to. From the uninterpolated read, like rules
# 3, 7 and 8: an image merged from an anchor or written `image :` is found as
# compose finds it (the line reader refused the latter, T-413 review #2).
raw_images = {}
for _, raw in raw_models:
    for name, svc in (raw.get("services") or {}).items():
        image = (svc or {}).get("image")
        if isinstance(image, str):
            raw_images[name] = image

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
            f"(as written: {source!r})",
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
# `MV_.*_SALT`: MV_TELEGRAM_ACTION_KEY_SALT is a Secret() of the manifest whose
# name ends past `_KEY`, and a literal salt in compose passed every rule until
# it was named here (T-464).
SECRET_KEY = re.compile(
    r"^(MINIO_ROOT_USER|MINIO_ROOT_PASSWORD|NEO4J_AUTH|NEO4J_PASSWORD"
    r"|MV_.*_KEY|MV_.*_PASSWORD|MV_.*_SALT|MV_TELEGRAM_BOT_TOKEN)$"
)
# The ones the stack must refuse to start without: they need `${VAR:?}`, not a
# default. The secrets that are legitimately empty are absent on purpose, and
# for them the advice is `${VAR:-}`, their manifest's empty default: an empty
# MV_LLM_API_KEY or MV_ANTHROPIC_API_KEY is the normal, local case, and an
# empty MV_TELEGRAM_ACTION_KEY_SALT derives the HMAC key of action_key from
# the token (ADR-018, T-310). A `:?` on any of them would stop a stack that is
# right to start (T-464 review #1 Mi-1, N-1).
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
# What a refusal prints in place of a value that belongs to a secret: the value
# sits in the file under review, but not in the log of CI (C-15 v1.5 "a refusal
# does not print the value"; T-464 review #1 N-2). Rule 8 uses it as well.
WITHHELD = "<withheld>"


def is_secret(name):
    return bool(name) and SECRET_KEY.match(name) is not None


def advice(key):
    """The form rule 3 asks a secret to take instead of what it found."""
    if key in MUST_BE_REQUIRED:
        return f"use ${{{key}:?...}}"
    return (f"use ${{{key}:-}}: an optional secret, empty by the manifest's "
            "default (rule 8), and no `:?`, which would stop a stack that is "
            "right to start")


# Credentials as written, in every file and wherever they sit. Layout is not
# part of the rule — `environment` may be a mapping or a list, a key may be
# quoted, and a secret may sit in any anchor (`x-platform-env` and whatever the
# next one is called), not only in a service. After YAML none of that is left
# to get wrong: a flow mapping, a continuation line and `KEY :` are entries like
# any other (T-429).
for compose_path, raw in raw_models:
    for item in scalars(raw):
        key, value = item.key, item.value
        if key is None:
            continue
        where = f"{compose_path} {item.where}"
        # A key with no value: compose fills it from .env when .env has it and
        # otherwise leaves it out of the container — for a credential the
        # stack must refuse to start without, that is a silent fallback to
        # whatever the image does on its own (MinIO: minioadmin; T-411).
        if value is None:
            if key in MUST_BE_REQUIRED:
                note(3, where, f"{key} is passed through with no value; an unset "
                               "credential would just be left out, so it needs ${VAR:?}")
            continue
        if not SECRET_KEY.match(key):
            continue
        # Real interpolations only: `$${KEY:?}` hands the container the literal
        # text, which is a default credential like any other.
        refs = [r for r in interpolations(value) if r.name]
        if not refs:
            note(3, where, f"{key} carries the literal {WITHHELD}; {advice(key)}")
        elif key in MUST_BE_REQUIRED and not any(r.op == ":?" for r in refs):
            note(3, where, f"{key} has a default ({WITHHELD}); a missing credential must "
                           "stop the stack, so it needs ${VAR:?}")
flush()

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

# The verdicts of llm_endpoint_classify (scripts/lib/llm-endpoint.sh), taken
# in bash before this program started: service, key, class, kind, host, error,
# and 1 when the value holds an @ (then no sentence names the host, T-463).
# No second copy of the rule lives here (T-450). The error is printed as the
# function wrote it: llm_parse_url cuts the query and the fragment off the value
# it echoes, and the user information in front of the last @ as well — before
# the first refusal, the one about the query included (T-450 review #1 N-2,
# review #2 Mi-R2-1). A key lives there when it is put into the address, so a
# COMPOSE_LINT_ENV_FILES pointed at a real env file does not carry one into the
# log; the fixture bad-llm-url-userinfo holds that with a fake password.
with open(os.path.join(work, "llm-verdicts.bin"), "rb") as fh:
    fields = [f.decode("utf-8", errors="replace") for f in fh.read().split(b"\0")[:-1]]
if len(fields) % 7:
    print("compose-lint: the verdicts of llm_endpoint_classify are malformed "
          f"({len(fields)} fields, want groups of 7)", file=sys.stderr)
    sys.exit(2)
for i in range(0, len(fields), 7):
    name, key, verdict, kind, llm_host, reason, masked = fields[i:i + 7]
    if verdict == "local":
        continue
    if verdict == "cloud" and masked == "1":
        # What the parser calls the host may be the head of a password
        # (http://pw.example/x@10.0.0.5:8080), so neither the value nor the
        # host is printed (C-15 v1.5, T-463 review #1 Ma-1).
        fail(
            6,
            f"{name}: {key} points at a host which is not a local address "
            f"(testdata/llm/local-endpoints.tsv); a cloud endpoint needs "
            "MV_LLM_CLOUD_ENABLED=true and never a compose default "
            "(SEC-15, ADR-005 add. 2 p. 3) (the value and its host are not "
            "printed: the value holds an @, and what stands in front of it "
            "may be a key)",
        )
    elif verdict == "cloud":
        fail(
            6,
            f"{name}: {key} points at {llm_host!r}, which is not a local address "
            f"(testdata/llm/local-endpoints.tsv); a cloud endpoint needs "
            "MV_LLM_CLOUD_ENABLED=true and never a compose default "
            "(SEC-15, ADR-005 add. 2 p. 3)",
        )
    else:
        fail(
            6,
            f"{name}: {key} is not an address the platform can use — a "
            f"configuration error, not the cloud: {reason} "
            "(testdata/llm/local-endpoints.tsv)",
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

primary_path, primary_raw = raw_models[0]
asked = set()
for item in scalars(primary_raw):
    if item.text is None:
        continue
    for ref in walk(interpolations(item.text)):
        if not ref.name or not ref.op.endswith("?") or ref.name in versions or ref.name in asked:
            continue
        asked.add(ref.name)
        where = f"{primary_path} {item.where}"
        if ref.name not in marked:
            fail(
                7,
                f"{where}: {ref.name} is required here (`{ref.op}`) and "
                f"absent from {EXAMPLE}; a clean machine never gets it",
            )
        elif not marked[ref.name]:
            fail(
                7,
                f"{where}: {ref.name} is required here (`{ref.op}`) but "
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

GO_STRING = re.compile(r'"(?:[^"\\\n]|\\.)*"|`[^`]*`')


def declarations(src):
    """name -> the options of its Declare(...) call, the string literals
    blanked: `Tooling()` or `Required()` inside a description is text, not a
    mark. The call ends at its balanced closing parenthesis, counted outside
    the strings. A call that does not close leaves its tail to the end of the
    source — a mark after it is then misread, and `go build` has refused the
    file long before this."""
    blank = GO_STRING.sub('""', src)
    found = {}
    for m in re.finditer(r'\bDeclare\(\s*""', blank):
        depth, i = 1, m.end()
        while i < len(blank) and depth:
            depth += {"(": 1, ")": -1}.get(blank[i], 0)
            i += 1
        found[m.start()] = blank[m.end():i]
    # The names come from the source with its strings intact, in the same order.
    names = re.findall(r'\bDeclare\(\s*"(MV_[A-Z0-9_]+)"', src)
    calls = [found[k] for k in sorted(found)]
    return dict(zip(names, calls)) if len(names) == len(calls) else {}


declared_calls = declarations(manifest_src)
if manifest and not declared_calls:
    fail(9, f"the Declare calls of {MANIFEST} cannot be told apart; the marks Tooling() "
            "and Required() are unknown, so rule 9 cannot tell a host variable from a "
            "variable of a container")
TOOLING = {n for n, call in declared_calls.items() if re.search(r"\bTooling\(\)", call)}
REQUIRED = {n for n, call in declared_calls.items() if re.search(r"\bRequired\(\)", call)}
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
        note(8, where, f"{name} is declared retired in {MANIFEST}; nothing reads it")
        return True
    if name not in manifest:
        note(
            8,
            where,
            f"{name} is not declared in {MANIFEST}, so its value here "
            "has nothing to agree with (and mvctl env check cannot see it)",
        )
        return True
    return False


# The allow-lists whose EMPTY value is a value — nobody (T-411). For them, and
# only them, the form of passing is a key with no value: `${MV_X:-d}` would
# replace the empty line of .env with the default and admit the default list.
# Whether they move to the common form as well is the system architect's to
# decide (T-469); until then the advice keeps them as they are.
EMPTY_IS_NOBODY = {
    "MV_GATEWAY_CLIENT_IDS",
    "MV_GATEWAY_ACTOR_KIND_CLIENTS",
    "MV_CORE_ADMIN_CLIENTS",
}


def passing_form(name, default):
    """The form of passing a platform variable to a container (T-469): the
    manifest's default behind `:-`, empty for an optional secret, `:?` for a
    required variable, and a key with no value for an allow-list of
    EMPTY_IS_NOBODY."""
    if name in EMPTY_IS_NOBODY:
        return f"{name}: (a key with no value)"
    if name in REQUIRED or name in MUST_BE_REQUIRED:
        return f"${{{name}:?set {name} in .env}}"
    if is_secret(name):
        return f"${{{name}:-}}"
    return f"${{{name}:-{default}}}"


def fix_for(name, default, whole):
    """What to write instead. A key with no value exists only where the
    interpolation IS the value of an environment key; inside a command or a
    longer string there is no key to leave empty (review #2 T-411, N-5). For a
    third-party variable a key with no value hands the container the image's
    default, not ours (contracts.md §16 p. 5)."""
    if not name.startswith("MV_"):
        return f"write ${{{name}:-{default}}}, the default of {EXTERNALS}"
    if whole and name in EMPTY_IS_NOBODY:
        return ("pass it through as a key with no value: for this allow-list an empty "
                "line in .env is a value, nobody, which `:-` would replace with the "
                "default, and with .env silent the manifest's default applies (T-411)")
    if whole and (name in REQUIRED or name in MUST_BE_REQUIRED):
        return (f"write {passing_form(name, default)}, the form of passing of a required "
                "variable (T-469): compose refuses to start without it")
    if whole:
        return (f"write {passing_form(name, default)}, the form of passing (T-469): the "
                "value of .env when it holds one, the manifest's default when .env is "
                "silent or its line is empty")
    return (f"it is part of a larger string here (a command, a quoted text), where "
            f"a key with no value is not available: repeat the manifest's default, "
            f"${{{name}:-{default}}}, or require it with ${{{name}:?...}}")


def check(where, ref, whole, hidden=False):
    # `hidden`: the interpolation is the value of a secret's key, or of a
    # secret's own interpolation around it — then no refusal prints what was
    # written (WITHHELD of rule 3; T-464 review #1 N-2).
    hidden = hidden or is_secret(ref.name)
    # Not inside the message of `:?`/`?`: compose evaluates it only on its way
    # to a refusal, so nothing in it reaches a container (see walk).
    if not ref.op.endswith("?"):
        for inner in ref.inner:
            check(where, inner, False, hidden)
    name = ref.name
    if name is None:
        return

    def shown(text):
        return WITHHELD if hidden else repr(text)

    written = f"${{{name}{ref.op}{WITHHELD}}}" if hidden and ref.arg else ref.text
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
    if op == "-" and not name.startswith("MV_"):
        # `-` without the colon falls back only when the variable is UNSET: a
        # `.env` line `OLLAMA_X=` hands the container an empty value instead of
        # ours (review #1 T-432, Mi-2; decision of the orchestrator). MV_* keep
        # their old reading — this task does not widen the rule for them.
        note(
            8,
            where,
            f"{written} falls back to the default only when {name} is unset: a .env "
            f"line `{name}=` hands the container an EMPTY {name} instead of the default "
            f"of {source} ({default!r}); " + fix_for(name, default, whole),
        )
        return
    if op.endswith("+"):
        note(
            8,
            where,
            f"{written} hands the process compose's own text when {name} "
            "is set and an EMPTY value when .env is silent - never the operator's "
            f"value and never the default of {source} (T-411 review #2 N-6); "
            + fix_for(name, default, whole),
        )
        return
    if op == "":
        if default != "" and not name.startswith("MV_"):
            # No process of the platform reads it, so the allow-list text below
            # does not apply; what goes wrong is the container (review #1 Mi-1).
            note(
                8,
                where,
                f"{written} has no default: when .env is silent the container "
                f"gets an EMPTY {name} instead of the default of {source} ({default!r}); "
                + fix_for(name, default, whole),
            )
        elif default != "":
            note(
                8,
                where,
                f"{written} has neither a default nor `:?`: when .env is "
                f"silent the process gets an EMPTY {name}, not the default of {source} "
                f"({default!r}), and shared/env reads set-to-empty as a value - for an "
                "allow-list that is nobody (T-411 review #1 Mi-2); "
                + fix_for(name, default, whole),
            )
        return
    value = ref.arg
    if ref.inner:
        note(
            8,
            where,
            f"{name} falls back to another variable here ({shown(value)}), and "
            f"{source} declares {default!r}: one value with two sources by "
            "construction (T-411 review #1 N-1); " + fix_for(name, default, whole),
        )
        return
    if value == default:
        return
    if not name.startswith("MV_"):
        note(
            8,
            where,
            f"{name} defaults to {shown(value)} here and to {default!r} in "
            f"{source} - two sources of one value (contracts.md §16 p. 5); "
            + fix_for(name, default, whole),
        )
    elif name in NETWORK_ADDRESSES:
        want = shape(default)
        misshapen = [i for i in items(value) if shape(i) != want]
        foreign = [i for i in items(value) if host(i) not in service_names]
        if misshapen:
            note(
                8,
                where,
                f"{name} defaults to {shown(value)} here; {shown(misshapen[0])} does not "
                f"have the same shape as the manifest's default {default!r} ({want}) - "
                "a network address may name a service of this network, but in the "
                "shape the process reads (contracts.md §16 p. 5)",
            )
        elif foreign:
            note(
                8,
                where,
                f"{name} defaults to {shown(value)} here and to {default!r} in "
                f"{source}; {shown(foreign[0])} is not a service of this compose network, so "
                "it is a second, invented source of the value (T-411). Every item "
                "must name a service of this file",
            )
    elif name in NOT_NETWORK_ADDRESSES:
        note(
            8,
            where,
            f"{name} defaults to {shown(value)} here and to {default!r} in "
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
        note(
            8,
            where,
            f"{name} defaults to {shown(value)} here and to {default!r} in "
            f"{source} - two sources of one value (T-411); {name} is not in the "
            "explicit set of network addresses, the only variables compose may "
            "point at a service of its own network; "
            + fix_for(name, default, whole) + "." + hint,
        )


for compose_path, raw in raw_models:
    for item in scalars(raw):
        where = f"{compose_path} {item.where}"
        if item.key is not None and item.value is None and item.key in external:
            note(
                8,
                where,
                f"{item.key} is passed through with no value; with .env silent "
                "the container then runs on the image's own default, not on the "
                f"default of {EXTERNALS} ({external[item.key]!r}); "
                + fix_for(item.key, external[item.key], True),
            )
        # A value for it that is anything but its own interpolation: a literal,
        # even one equal to the manifest's default, or text taken from another
        # variable. Either way the OLLAMA_X of .env never reaches the container
        # — the operator tunes it there, and the setting lands nowhere
        # (contracts.md §16 p. 5, v0.9, T-431 variant (b); the class of T-408).
        # `$${OLLAMA_X:-d}` is compose's escape, the literal text, and counts as
        # a literal. The default of the one allowed form is `check`'s business.
        if item.key is not None and item.value is not None and item.key in external:
            own = interpolations(item.value)
            if not (len(own) == 1 and own[0].name == item.key and own[0].text == item.value):
                mine = next((r for r in own if r.name == item.key), None)
                lost = (f": .env cannot override it, so the {item.key} an operator sets "
                        "there never reaches the container, whatever the value")
                if not own:
                    what = f"carries the literal {item.value!r}{lost}"
                elif mine is not None:
                    # Its own interpolation with text around it (review #1 T-432,
                    # N-1): the value of .env arrives, but not as it was set.
                    what = (f"wraps {mine.text} in other text ({item.value!r}): the "
                            f"{item.key} an operator sets in .env reaches the container "
                            "with compose's text around it; the value must be the "
                            "interpolation alone")
                else:
                    what = f"takes its value from {item.value!r}, not from {item.key} itself{lost}"
                note(
                    8,
                    where,
                    f"{item.key} {what} (contracts.md §16 p. 5, v0.9); rule 8 accepts "
                    f"${{{item.key}:-{external[item.key]}}}, the default of {EXTERNALS} "
                    "(a requirement with `:?` is rule 7's)",
                )
        if item.text is None:
            continue
        # `whole`: the interpolation IS the value of an `environment` entry — of
        # a service, or of a top-level anchor that is merged into one — so a
        # key with no value is a fix on offer. `KEY=${X}` in a command, a label
        # or a build arg is an entry too, but leaving its value out passes
        # nothing through (T-429 review #1 N-4).
        in_env = item.parent.endswith(".environment") or (
            item.parent.startswith("x-") and "." not in item.parent and "[" not in item.parent)
        for ref in interpolations(item.text):
            check(where, ref, whole=in_env and item.value is not None and ref.text == item.value,
                  hidden=is_secret(item.key))
flush()

# --------------------------------------------------------------------------
# Rule 9 — a variable a context reads reaches every service that runs it
# --------------------------------------------------------------------------
# Who reads a platform variable inside a container (T-469). A reader is a
# context of cmd/multiverse/contexts.go, PROCESS — cmd/multiverse itself, which
# reads the variable whatever --contexts it runs — or BOT, the binary of
# cmd/telegram-bot. The source of a row is the code where the reader exists
# (`env.<Var>` in internal/<context>, cmd/multiverse, shared/runtime,
# shared/objstore, shared/logging, cmd/telegram-bot) and the component
# document where the context is still the /health stub of contexts.go: llm,
# laws and swarm read nothing yet, and their rows are what their components
# assign them (swarm-llm-laws.md, C-15). A row lists every reader, not only the
# one whose service happens to carry the line today.
PROCESS = "process"
BOT = "telegram-bot"
CONTEXTS = {"state", "mechanics", "laws", "llm", "swarm", "gateway", "memory"}
READERS = {
    # cmd/multiverse and shared/logging, whatever the contexts.
    "MV_ENV": (PROCESS, BOT),
    "MV_LOG_LEVEL": (PROCESS, BOT),
    "MV_LOG_FORMAT": (PROCESS, BOT),
    "MV_MODE": (PROCESS,),
    "MV_BUS": (PROCESS,),
    "MV_BUS_VALIDATE_ON_READ": (PROCESS,),
    "MV_KAFKA_BROKERS": (PROCESS,),
    "MV_CORE_ADDR": (PROCESS,),
    # shared/runtime.AdminOnly: /v1/admin/* of any context, the replay clock.
    "MV_CORE_ADMIN_CLIENTS": (PROCESS,),
    # shared/objstore: the snapshots of state, the backups of the gateway.
    "MV_MINIO_ENDPOINT": ("state", "gateway"),
    "MV_MINIO_ACCESS_KEY": ("state", "gateway"),
    "MV_MINIO_SECRET_KEY": ("state", "gateway"),
    "MV_MINIO_USE_SSL": ("state", "gateway"),
    "MV_WORLD_ID": ("gateway",),
    # internal/gateway (context.go).
    "MV_GATEWAY_DATA_DIR": ("gateway",),
    "MV_GATEWAY_CLIENT_IDS": ("gateway",),
    "MV_GATEWAY_ACTOR_KIND_CLIENTS": ("gateway",),
    "MV_CORE_URL": ("gateway",),
    "MV_GATEWAY_RATE_ACTIONS_PER_MIN": ("gateway",),
    "MV_GATEWAY_RATE_ACTIONS_BURST": ("gateway",),
    "MV_GATEWAY_INPUT_FILTER": ("gateway",),
    "MV_GATEWAY_ENCOUNTER_GRACE": ("gateway",),
    "MV_GATEWAY_SESSION_IDLE": ("gateway",),
    "MV_GATEWAY_TURN_TIMEOUT": ("gateway",),
    "MV_GATEWAY_CHARACTER_WAIT": ("gateway",),
    "MV_GATEWAY_CHARACTER_DEADLINE": ("gateway",),
    "MV_GATEWAY_DELIVERY_LEASE": ("gateway",),
    "MV_GATEWAY_DELIVERY_TTL": ("gateway",),
    "MV_GM_PATH": ("gateway", "swarm"),
    # internal/state, laws.
    "MV_SNAPSHOT_EVERY_FACTS": ("state",),
    "MV_STATE_WORLDS": ("state",),
    "MV_LAWS_BREACH_PHASE": ("laws",),
    "MV_MEMORY_URL": ("swarm",),
    # the context llm (C-15).
    "MV_LLM_PROVIDER": ("llm",),
    "MV_LLM_URL": ("llm",),
    "MV_LLM_API_KEY": ("llm",),
    "MV_LLM_NUM_CTX": ("llm",),
    "MV_LLM_STORE_PROMPTS": ("llm",),
    "MV_LLM_CLOUD_ENABLED": ("llm",),
    "MV_LLM_CLOUD_ALLOW_EXTERNAL_PLAYERS": ("llm",),
    "MV_LLM_CLOUD_BUDGET_USD_PER_DAY": ("llm",),
    "MV_ANTHROPIC_API_KEY": ("llm",),
    "MV_OLLAMA_URL": ("llm",),
    # the context memory (EPIC-005).
    "MV_QDRANT_ADDR": ("memory",),
    "MV_NEO4J_URI": ("memory",),
    "MV_NEO4J_USER": ("memory",),
    "MV_NEO4J_PASSWORD": ("memory",),
    "MV_EMBED_MODEL": ("memory",),
    # cmd/telegram-bot/internal/config.
    "MV_TELEGRAM_BOT_TOKEN": (BOT,),
    "MV_TELEGRAM_ALLOWED_USER_IDS": (BOT,),
    "MV_TELEGRAM_GATEWAY_URL": (BOT,),
    "MV_TELEGRAM_POLL_TIMEOUT_S": (BOT,),
    "MV_TELEGRAM_HEALTH_ADDR": (BOT,),
    "MV_TELEGRAM_ACTION_KEY_SALT": (BOT,),
    "MV_TELEGRAM_COMMANDS_PER_MIN": (BOT,),
}
# Declared, not Tooling(), and still never passed to a container — each with
# the reason. Whether these become a mark of the manifest is the system
# architect's question (contracts.md §16 p. 5, T-469).
NOT_IN_CONTAINERS = {
    "MV_BACKUP_AGE_RECIPIENT": "the public key `age` encrypts links.db to on the host, "
                               "after `docker cp` (infrastructure.md §9); no process reads it",
    "MV_SWARM_FAKE": "the temporary hook of I1-α (cmd/multiverse/fake_contexts.go, removed "
                     "by T-256) for a process started by hand; the stub reads rules/ under "
                     "the working directory, which the image does not carry, and "
                     "infrastructure.md §4.2 says compose does not pass the flag",
}

rows = set(READERS) | set(NOT_IN_CONTAINERS)
for name in sorted(set(manifest) - TOOLING - rows):
    fail(9, f"{name} is declared in {MANIFEST} but rule 9 does not know who reads it: add "
            "it to READERS of scripts/compose-lint.sh with the contexts (or `process`, "
            "`telegram-bot`) that read it, or to NOT_IN_CONTAINERS with the reason it "
            "never reaches a container")
for name in sorted(rows - set(manifest)):
    what = "declared retired" if name in retired else "not declared"
    fail(9, f"{name} is in the table of rule 9 but {what} in {MANIFEST}; the table and "
            "the manifest have drifted apart")
for name in sorted(rows & TOOLING):
    fail(9, f"{name} is marked Tooling() in {MANIFEST}, a variable of the host, and is "
            "also in the table of rule 9; one of the two is wrong")
for name in sorted(set(READERS) & set(NOT_IN_CONTAINERS)):
    fail(9, f"{name} is both in READERS and in NOT_IN_CONTAINERS of rule 9")
for name, readers in sorted(READERS.items()):
    for reader in readers:
        if reader not in CONTEXTS | {PROCESS, BOT}:
            fail(9, f"{name}: the reader {reader!r} of rule 9 is neither a context "
                    f"({', '.join(sorted(CONTEXTS))}) nor `{PROCESS}` or `{BOT}`")

CONTEXTS_FLAG = re.compile(r"--?contexts(?:=(.*))?", re.S)


def argv(value):
    if value is None:
        return []
    if isinstance(value, str):
        return value.split()
    return [str(v) for v in value]


def runs(svc):
    """(the readers a service runs, how the file says so, unknown contexts).

    The arguments of the platform process are what compose hands it: the rest
    of an `entrypoint` whose program is /multiverse, then `command` (T-469
    review #1 N-2). A flag given twice is read as the Go flag package reads it,
    the last one wins — so the contexts a service runs are the ones of its last
    `--contexts`. Another program in `entrypoint` has its `command` read as
    before: the platform image starts /multiverse only when nothing replaces
    it."""
    command = argv(svc.get("command"))
    entrypoint = argv(svc.get("entrypoint"))
    program = entrypoint[0].rsplit("/", 1)[-1] if entrypoint else ""
    if program == BOT:
        return {BOT}, f"entrypoint {entrypoint[0]}", []
    args = (entrypoint[1:] if program == "multiverse" else []) + command
    value = None
    for n, arg in enumerate(args):
        m = CONTEXTS_FLAG.fullmatch(arg)
        if not m:
            continue
        value = m.group(1)
        if value is None:
            value = args[n + 1] if n + 1 < len(args) else ""
    if value is None:
        return set(), "", []
    names = [c.strip() for c in value.split(",") if c.strip()]
    shown = f"--contexts={value}"
    if "all" in names:
        return {PROCESS} | CONTEXTS, shown, []
    return {PROCESS} | (set(names) & CONTEXTS), shown, [c for c in names if c not in CONTEXTS]


def passed(name, svc):
    """The keys a service passes: every key of its `environment` as written in
    every file that defines the service — a key with no value that .env leaves
    unset included — and every key of its `environment` in the resolved model.
    The resolved model is where an `env_file` shows up: `docker compose config`
    moves the variables of the file into `environment` and drops the key
    `env_file` from the service (compose v5.2.0), so a variable passed that way
    is found there and nowhere else. The linter never opens an env file of a
    service itself: its path may be the operator's own .env (T-469 review #1
    Mi-1)."""
    resolved = svc.get("environment") or {}
    if isinstance(resolved, list):
        resolved = [m.group(1) for m in map(LISTED.fullmatch, map(str, resolved)) if m]
    keys = {str(k) for k in resolved}
    for _, raw in raw_models:
        env = ((raw.get("services") or {}).get(name) or {}).get("environment")
        if isinstance(env, dict):
            keys |= {str(k) for k in env}
        elif isinstance(env, list):
            for entry_ in env:
                m = LISTED.fullmatch(str(entry_))
                if m:
                    keys.add(m.group(1))
    return keys


for name, svc in sorted(services.items()):
    readers, how, strangers = runs(svc)
    for stranger in strangers:
        fail(9, f"{name}: {how} names the context {stranger!r}, which rule 9 does not know "
                f"({', '.join(sorted(CONTEXTS))}); a new context comes with its rows in READERS")
    if not readers:
        continue
    have = passed(name, svc)
    for var in sorted(READERS):
        if var not in manifest or var in TOOLING:
            continue  # the drift is reported above
        whose = sorted(set(READERS[var]) & readers)
        if not whose or var in have:
            continue
        whose = ["the process itself (cmd/multiverse)" if r == PROCESS else
                 f"the binary {r}" if r == BOT else f"the context {r}" for r in whose]
        fail(9, f"{name}: {var} is read by {', '.join(whose)}, which this service runs "
                f"({how}), but its environment does not pass it: the value of .env never "
                "reaches the container and the process silently takes the manifest's "
                f"default; add {passing_form(var, manifest[var])}")

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
    f"compose-lint: ok — {len(services)} services in {len(compose_paths)} file(s), 9 rules "
    f"(profiles resolved: {', '.join(sorted({p for s in services.values() for p in (s.get('profiles') or [])})) or 'none'})"
)
PY
