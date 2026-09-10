#!/usr/bin/env bash
# =============================================================================
# scripts/llm-server.sh — LLM runtime of the platform: start, stop, health
# (infrastructure.md v0.3 §6.1-§6.3, §2.1; ADR-005 add. 2 p. 2, p. 3, p. 7)
# =============================================================================
# The WSL/Linux twin of scripts/llm-server.ps1, with the same argument set. On
# the owner's machine the PowerShell script is the one that runs: llama-server
# is a native Windows process with CUDA and D:\Models\ paths, and moving it into
# WSL would mean setting CUDA up twice for no gain (§6.1). This file exists for
# a Linux stand and for CI-like environments that have a Linux build.
#
# THE ADDRESS HAS ONE SOURCE OF TRUTH, and — since T-404 review #1 — ONE RULE:
# scripts/lib/llm-endpoint.sh. Both this script and scripts/llm-bench.sh derive
# the address from it, so that "what the probe knocks on", "what the bench
# measures" and "what llama-server binds" cannot drift again. MV_LLM_HOST stays
# and is not a duplicate: it is the interface the local server listens on
# (loopback only, SEC-15), which is a different question from where a client
# should knock. The port was the duplicate, and it is gone.
#
# Not every provider is a process of this machine: openai_compat on a cloud
# address, anthropic, recorded and fake are not started or stopped from here,
# and up/down say so instead of pretending (ADR-005 add. 2 p. 2).
#
# The LLM is not a compose service. `make up` does not start it and `make down`
# does not stop it; the two life cycles are deliberately separate.
#
# Every value comes from .env (MV_LLM_*, §4.2). Sampling parameters (--temp,
# --top-p, --top-k, --min-p) are deliberately NOT passed: the gateway sets them
# per request from the blueprint of the phase (ADR-005 add. 2 p. 1).
#
# Usage:
#   scripts/llm-server.sh [up|down|health] [--router] [--with-ui]
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
ops_dir="$repo_root/ops"
pid_file="$ops_dir/llm-server.pid"
log_file="$ops_dir/llm-server.log"

# The module is the ONLY source of the rule that reads MV_LLM_URL, so its
# absence is not a stack trace to decipher: it is one English line naming the
# file. Without this check the twin printed a localised .NET error record —
# mojibake on a Cyrillic path — because the console settings that fix that live
# INSIDE the module (review Mi-3).
llm_lib="$repo_root/scripts/lib/llm-endpoint.sh"
if [ ! -f "$llm_lib" ]; then
  echo "llm: $llm_lib is missing — it holds the only rule that turns MV_LLM_URL into an address, and no script of this project can read the variable without it" >&2
  exit 1
fi
# shellcheck source=scripts/lib/llm-endpoint.sh
. "$llm_lib"

action=up
router=0
with_ui=0
while [ $# -gt 0 ]; do
  case "$1" in
  up | down | health) action=$1 ;;
  --router) router=1 ;;
  --with-ui) with_ui=1 ;;
  -h | --help)
    sed -n '2,32p' "$0"
    exit 0
    ;;
  *)
    echo "llm: unknown argument $1" >&2
    exit 2
    ;;
  esac
  shift
done

mkdir -p "$ops_dir"

# --- the address ------------------------------------------------------------
#
# One call, one rule. LLM_EP_URL is the address as configured (what is printed,
# and what a remote endpoint is probed at), LLM_EP_PROBE is the same address as
# reachable from this machine, LLM_EP_PORT is what llama-server binds.

llm_host=$(llm_env_or MV_LLM_HOST 127.0.0.1) # what the local server binds, loopback ONLY (SEC-15)
resolve_status=0
llm_endpoint_resolve || resolve_status=1

# not_startable explains, in one line, why this provider is not a process here.
not_startable() {
  case "$LLM_EP_PROVIDER" in
  recorded | fake)
    llm_say "llm: MV_LLM_PROVIDER=$LLM_EP_PROVIDER answers inside the process — no endpoint to probe, nothing to start or stop (ADR-005 add. 2 p. 2)"
    ;;
  anthropic)
    llm_say 'llm: MV_LLM_PROVIDER=anthropic is the vendor cloud API — not started from here, and this wrapper has no address to probe; core reports it as deps.llm in /health'
    ;;
  ollama)
    llm_say 'llm: MV_LLM_PROVIDER=ollama is a runtime of its own — start it with `ollama serve` or the compose profile gpu, not from here; models are pulled by make models'
    ;;
  *)
    llm_say "llm: $LLM_EP_VAR=$LLM_EP_RAW is not an address of this machine — provider $LLM_EP_PROVIDER is not started or stopped from here"
    ;;
  esac
}

# --- the pid file ------------------------------------------------------------
#
# ops/llm-server.pid is written by `up` and read by `up` and `down`. Since
# T-404 review #2 it holds three lines instead of one:
#
#   <pid>
#   bin=<the program that was started, resolved at start>
#   started=<when that process started, UTC, second precision>
#
# because a number on its own is not evidence that the process is ours. A pid
# file survives a reboot, Windows reuses process numbers aggressively, and both
# wrappers used to take the number on trust and force-stop whatever it had
# become. That is not hypothetical: during review #2 this code stopped a live
# process of the owner's environment which happened to carry the recorded
# number (review M-5). Nothing is stopped now until the program behind the
# number matches — and the recorded path is used in preference to MV_LLM_BIN,
# so the check still works after the variable has been changed or unset.
#
# Everything in this block exists in scripts/llm-server.ps1 as well, message for
# message: the twin has $proc.Path where this file has /proc/<pid>/exe.

pid_recorded=''
pid_recorded_bin=''
pid_recorded_started=''
pid_stranger=0      # the recorded number is provably another process
pid_unverifiable=0  # the recorded number cannot be judged either way
pid_owner_reason=''

read_pid_file() {
  pid_recorded='' pid_recorded_bin='' pid_recorded_started=''
  [ -f "$pid_file" ] || return 1
  local line first=1
  while IFS= read -r line || [ -n "$line" ]; do
    if [ "$first" = 1 ]; then
      first=0
      pid_recorded=$(printf '%s' "$line" | tr -d '[:space:]')
      continue
    fi
    case "$line" in
    bin=*) pid_recorded_bin=$(llm_trim "${line#bin=}") ;;
    started=*) pid_recorded_started=$(llm_trim "${line#started=}") ;;
    esac
  done <"$pid_file"
  # A pid file with junk in it is a recovery path, not a place to fall over
  # (review Mi-8): anything that is not a number is treated as no pid at all.
  case "$pid_recorded" in
  '' | *[!0-9]*)
    llm_say 'llm: ops/llm-server.pid does not contain a process id — ignoring it'
    return 1
    ;;
  esac
  return 0
}

# path_key normalises a program path for comparison: separators one way, the
# .exe suffix off (Git Bash reports /proc/<pid>/exe without it) and the case
# folded where the file system folds it.
path_key() {
  local p=${1-}
  p=${p//\\//}
  case "$(uname -s 2>/dev/null)" in
  MINGW* | MSYS* | CYGWIN*)
    p=$(printf '%s' "$p" | tr '[:upper:]' '[:lower:]')
    p=${p%.exe}
    ;;
  esac
  printf '%s' "$p"
}

# process_image answers "what program is that number running". /proc/<pid>/exe
# is the answer where it exists — including Git Bash, which keeps it for the
# native Windows processes it started; `ps` is the fallback and gives whatever
# that ps prints for the command, which is why the comparison below accepts a
# bare file name as well as a full path.
process_image() {
  local pid=$1 image=''
  if [ -r "/proc/$pid/exe" ]; then
    image=$(readlink "/proc/$pid/exe" 2>/dev/null) || image=''
  fi
  if [ -z "$image" ] && command -v ps >/dev/null 2>&1; then
    image=$(llm_trim "$(ps -p "$pid" -o comm= 2>/dev/null | tail -n 1)")
    # The ps of MSYS knows neither -o nor comm; there the command is the last
    # column of the last line.
    if [ -z "$image" ]; then
      image=$(ps -p "$pid" 2>/dev/null | awk 'END { print $NF }')
      case "$image" in COMMAND | '') image='' ;; esac
    fi
  fi
  printf '%s' "$image"
}

# process_started prints the start time of a process in the same shape `up`
# records, or nothing when this platform will not say. Field 22 of
# /proc/<pid>/stat counts clock ticks since boot; /proc/stat carries the boot
# time. Where either is missing (Git Bash against a native Windows process) the
# time is simply not compared and the path check stands alone.
process_started() {
  local pid=$1 stat rest ticks hz btime epoch
  [ -r "/proc/$pid/stat" ] || return 0
  stat=$(cat "/proc/$pid/stat" 2>/dev/null) || return 0
  rest=${stat#*") "}
  ticks=$(printf '%s' "$rest" | awk '{ print $20 }')
  case "$ticks" in '' | *[!0-9]*) return 0 ;; esac
  hz=$(getconf CLK_TCK 2>/dev/null) || return 0
  case "$hz" in '' | 0 | *[!0-9]*) return 0 ;; esac
  btime=$(awk '/^btime /{ print $2 }' /proc/stat 2>/dev/null)
  case "$btime" in '' | *[!0-9]*) return 0 ;; esac
  epoch=$((btime + ticks / hz))
  date -u -d "@$epoch" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || true
}

# times_agree allows two seconds: the two platforms round the start time
# differently, and a reused process number differs by minutes, not by ticks.
times_agree() {
  local a b delta
  a=$(date -u -d "$1" +%s 2>/dev/null) || return 0
  b=$(date -u -d "$2" +%s 2>/dev/null) || return 0
  [ -n "$a" ] && [ -n "$b" ] || return 0
  delta=$((a - b))
  [ "$delta" -lt 0 ] && delta=$((-delta))
  [ "$delta" -le 2 ]
}

# pid_is_ours is the check the incident of review #2 was missing.
pid_is_ours() {
  local pid=$1 expected actual actual_started
  pid_owner_reason=''
  expected=$pid_recorded_bin
  [ -n "$expected" ] || expected=$(llm_env_or MV_LLM_BIN '')
  if [ -z "$expected" ]; then
    pid_owner_reason="llm: ops/llm-server.pid records pid $pid and nothing to compare it against (the file carries no program path and MV_LLM_BIN is not set) — refusing to stop a process this run cannot recognise; check it by hand and delete ops/llm-server.pid"
    return 1
  fi
  actual=$(process_image "$pid")
  if [ -z "$actual" ]; then
    pid_owner_reason="llm: ops/llm-server.pid records pid $pid, but this run cannot read the program of that process — refusing to stop a process this run cannot recognise; check it by hand and delete ops/llm-server.pid"
    return 1
  fi
  # Equal path, or — when the two sides name the same file differently — equal
  # file name. That happens routinely and is not laxity: /proc/<pid>/exe under
  # Git Bash answers /tmp/... where MV_LLM_BIN says C:\..., and Windows hands
  # out short 8.3 names for a path a human wrote in full. What the check has to
  # separate is llama-server from python.exe, and a file name does that; the
  # start time below separates two runs of the same program.
  local actual_key expected_key
  actual_key=$(path_key "$actual")
  expected_key=$(path_key "$expected")
  if [ "$actual_key" != "$expected_key" ] && [ "${actual_key##*/}" != "${expected_key##*/}" ]; then
    pid_owner_reason="llm: ops/llm-server.pid records pid $pid, which runs $actual and not $expected — the number belongs to another process now; nothing is stopped and the file is removed"
    return 1
  fi
  if [ -n "$pid_recorded_started" ]; then
    actual_started=$(process_started "$pid")
    if [ -n "$actual_started" ] && ! times_agree "$pid_recorded_started" "$actual_started"; then
      pid_owner_reason="llm: ops/llm-server.pid records pid $pid started at $pid_recorded_started, but that process started at $actual_started — the number has been reused; nothing is stopped and the file is removed"
      return 1
    fi
  fi
  return 0
}

# running_pid reports a live server OF OURS and leaves its id in $server_pid.
# It sets variables instead of printing one, because it also has lines to say
# about a damaged file and about a stranger, and a `pid=$(running_pid)` would
# swallow them into the variable.
server_pid=''
running_pid() {
  server_pid=''
  pid_stranger=0
  pid_unverifiable=0
  read_pid_file || return 1
  kill -0 "$pid_recorded" 2>/dev/null || return 1
  if ! pid_is_ours "$pid_recorded"; then
    llm_say "$pid_owner_reason"
    case "$pid_owner_reason" in
    *'the number belongs to another process now'* | *'the number has been reused'*) pid_stranger=1 ;;
    *) pid_unverifiable=1 ;;
    esac
    return 1
  fi
  server_pid=$pid_recorded
  return 0
}

# stop_server_if_ours stops the recorded process when it is ours, and says why
# it did not otherwise. $1 is which of the two lead lines to print.
stop_server_if_ours() {
  if running_pid; then
    local pid=$server_pid
    case "$1" in
    still-recorded) llm_say "llm: ops/llm-server.pid still records a live process (pid $pid) started here — stopping it" ;;
    *) llm_say "llm: stopping llama-server (pid $pid)" ;;
    esac
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    return 0
  fi
  # A stranger's number is removed from the file; a number this run could not
  # judge is left alone, because the message above tells the operator to look.
  [ "$pid_stranger" = 1 ] && rm -f "$pid_file"
  return 1
}

pinned_build() {
  sed -n 's/^[[:space:]]*LLAMACPP_BUILD[[:space:]]*=[[:space:]]*\(.*\)$/\1/p' \
    "$repo_root/build/versions.env" | head -n 1
}

# cloud_gate_shut: the address is outside the trusted network and the gate is
# closed, so the provider will REFUSE to start on it (ADR-005 add. 2 p. 3).
cloud_gate_shut() {
  [ "$LLM_EP_HAS" = 1 ] || return 1
  [ "$LLM_EP_TRUSTED" = 1 ] && return 1
  [ "$(llm_env_or MV_LLM_CLOUD_ENABLED false)" = true ] && return 1
  return 0
}

# show_health prints the report and returns the exit code of the check. The
# question it answers is "can the platform use the LLM", not "does something
# answer at that address" — which is why a shut cloud gate is a failure even
# when the endpoint says 200 (orchestrator decision on review question 3: the
# same false green as C-1, by another route).
show_health() {
  local models pin bin actual models_printed=0
  if [ "$resolve_status" != 0 ]; then
    llm_fail "$LLM_EP_ERROR"
    return 1
  fi
  if [ "$LLM_EP_HAS" != 1 ]; then
    not_startable
    return 0
  fi

  llm_note_retired_port "$LLM_EP_VAR"
  llm_say "llm: probing $LLM_EP_PROBE (from $LLM_EP_VAR=$LLM_EP_RAW)"
  if [ "$LLM_EP_TRIMMED_V1" = 1 ]; then
    llm_say "llm: $LLM_EP_VAR ends with /v1; the variable holds the base address, so the probe uses $LLM_EP_URL (T-404)"
  fi

  llm_health_probe "$LLM_EP_PROBE"
  case "$LLM_HEALTH_CODE" in
  200)
    if [ "$LLM_HEALTH_VIA" = /health ]; then
      llm_say "llm: $LLM_EP_PROBE/health = 200 ok"
    else
      llm_say "llm: $LLM_EP_PROBE$LLM_HEALTH_VIA = 200 ok (this server has no /health; judged by the endpoint the contract needs)"
    fi
    ;;
  503) llm_say "llm: $LLM_EP_PROBE/health = 503 loading (the model is still being read)" ;;
  000)
    if [ "$LLM_EP_STARTABLE" = 1 ] && [ -n "$(llm_env_or MV_LLM_BIN '')" ]; then
      llm_say "llm: $LLM_EP_PROBE unreachable — the process is not running (make llm-up)"
    elif [ "$LLM_EP_STARTABLE" = 1 ]; then
      # `make llm-up` here would stop at "MV_LLM_BIN is not set", and the very
      # next line of this report says so: two neighbouring lines contradicting
      # each other is worse than no advice (review Mi-1).
      llm_say "llm: $LLM_EP_PROBE unreachable — the process is not running; start it the way it was started before, because make llm-up needs MV_LLM_BIN (.env.example, §4.2)"
    else
      llm_say "llm: $LLM_EP_PROBE unreachable — the endpoint does not answer from this machine"
    fi
    ;;
  401 | 403)
    llm_say "llm: $LLM_EP_PROBE$LLM_HEALTH_VIA = $LLM_HEALTH_CODE — the endpoint needs a key; MV_LLM_API_KEY is $([ -n "$(llm_env_or MV_LLM_API_KEY '')" ] && echo 'set and rejected' || echo 'empty')"
    ;;
  *) llm_say "llm: $LLM_EP_PROBE$LLM_HEALTH_VIA = $LLM_HEALTH_CODE" ;;
  esac
  if [ "$LLM_HEALTH_FAULT" = 1 ]; then
    llm_say 'llm: the request itself failed — this is not "the process is not running"; check the address, the key and TLS'
  fi

  if [ "$LLM_HEALTH_CODE" = 200 ]; then
    models=$(llm_models "$LLM_EP_PROBE")
    llm_say "llm: models = ${models:-<none reported>}"
    models_printed=1
  fi

  # The build pin and the VRAM report describe a local llama.cpp process. For
  # any other runtime they answer a question nobody asked (T-404); the way to
  # pin the version of a remote runtime is the model of /v1/models, above.
  if [ "$LLM_EP_STARTABLE" = 1 ]; then
    pin=$(pinned_build)
    bin=$(llm_env_or MV_LLM_BIN '')
    if [ -n "$bin" ] && [ -x "$bin" ]; then
      actual=$("$bin" --version 2>&1 | sed -n 's/.*build[:[:space:]]*\([0-9]\{1,\}\).*/b\1/p' | head -n 1)
      if [ -n "$actual" ] && [ -n "$pin" ] && [ "$actual" != "$pin" ]; then
        # A warning, not an error (§6.2): the pin only becomes true after the
        # golden set has been re-run against the new build (§9.3).
        llm_say "llm: WARNING build $actual differs from the pin $pin in build/versions.env (infrastructure.md §9.5)"
      elif [ -n "$actual" ]; then
        llm_say "llm: build $actual matches the pin"
      fi
    elif [ -n "$pin" ]; then
      # Card p. 2: comparing LLAMACPP_BUILD against a binary is meaningless for a
      # runtime that is not llama.cpp, and on this stand that is exactly the case
      # (the address is local, the server was started by hand, MV_LLM_BIN is
      # unset). Saying "cannot compare" invited the operator to fix the wrong
      # thing; the version of a runtime we did not start is the model list above.
      # "Above" only when there IS a list above: at an endpoint that does not
      # answer, this sentence used to point at nothing (review Mi-2).
      if [ "$models_printed" = 1 ]; then
        llm_say "llm: MV_LLM_BIN is not set — this runtime was not started from here, so the pin $pin in build/versions.env does not describe it; the version of any other runtime is the model list above (T-404)"
      else
        llm_say "llm: MV_LLM_BIN is not set — this runtime was not started from here, so the pin $pin in build/versions.env does not describe it; the version of any other runtime is its GET /v1/models, which this run could not read (T-404)"
      fi
    fi

    if command -v nvidia-smi >/dev/null 2>&1; then
      llm_say "llm: VRAM $(nvidia-smi --query-gpu=memory.used,memory.total --format=csv,noheader | paste -sd'; ' -)"
    fi
  fi

  if cloud_gate_shut; then
    llm_say "llm: WARNING $LLM_EP_VAR points outside this machine and MV_LLM_CLOUD_ENABLED is not true — the provider will refuse this address (ADR-005 add. 2 p. 3)"
    if [ "$LLM_HEALTH_CODE" = 200 ]; then
      llm_say 'llm: the endpoint answers, but the platform will not call it while the gate is shut — reported as a failure, not as a green LLM'
    fi
    return 1
  fi

  [ "$LLM_HEALTH_CODE" = 200 ] || return 1
  return 0
}

case "$action" in

down)
  if [ "$resolve_status" != 0 ]; then
    llm_fail "$LLM_EP_ERROR"
    stop_server_if_ours still-recorded || true
    exit 1
  fi
  if [ "$LLM_EP_STARTABLE" != 1 ]; then
    not_startable
    # The configuration moved on, the process did not: if it was started by
    # this script it is still ours to stop.
    stop_server_if_ours still-recorded || true
    exit 0
  fi
  if stop_server_if_ours stopping; then
    llm_say 'llm: stopped; models and volumes are untouched'
  elif [ "$pid_unverifiable" = 0 ] && [ "$pid_stranger" = 0 ]; then
    llm_say 'llm: no live server in ops/llm-server.pid — nothing to stop'
    rm -f "$pid_file"
  fi
  ;;

health)
  show_health || exit 1
  ;;

up)
  if [ "$resolve_status" != 0 ]; then
    llm_fail "$LLM_EP_ERROR"
    exit 1
  fi
  if [ "$LLM_EP_STARTABLE" != 1 ]; then
    not_startable
    if [ "$LLM_EP_HAS" != 1 ]; then
      exit 0
    fi
    # Nothing to start, but the operator still asked "is the LLM usable?", and
    # for a remote endpoint that is the only answer this script can give.
    llm_say 'llm: checking whether it answers instead'
    show_health || exit 1
    exit 0
  fi

  # SEC-15 is fail-closed. Warning about a non-loopback interface and then
  # binding it anyway is not a control, it is the imitation of one (review
  # Mi-12, orchestrator decision): a published llama-server has no
  # authentication in front of it, and the operator who set the variable will
  # never see the warning in a scripted run.
  case "$llm_host" in
  127.0.0.1 | localhost | '::1' | '[::1]') ;;
  *)
    llm_fail "llm: MV_LLM_HOST=$llm_host is not loopback; llama-server would be reachable beyond this machine and it has no authentication (SEC-15, infrastructure.md §6.3). Refusing to start — set MV_LLM_HOST=127.0.0.1, and publish the port through a reverse proxy if it really has to leave the host"
    exit 1
    ;;
  esac

  llm_note_retired_port "$LLM_EP_VAR"
  llm_health_probe "$LLM_EP_PROBE"
  if running_pid; then
    pid=$server_pid
    if [ "$LLM_HEALTH_CODE" != 000 ]; then
      llm_say "llm: already running (pid $pid) on $LLM_EP_PROBE — nothing to do"
      exit 0
    fi
    # Only reached for a process that has just been proved ours: before review
    # #2 this line force-stopped whatever the number had become (M-5).
    llm_say "llm: pid $pid is recorded but $LLM_EP_PROBE does not answer; restarting"
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$pid_file"
  elif [ "$pid_unverifiable" = 1 ]; then
    llm_fail 'llm: ops/llm-server.pid names a process this run cannot recognise (see the line above) — refusing to start a second server while that is unresolved'
    exit 1
  elif [ "$LLM_HEALTH_CODE" != 000 ]; then
    # Somebody already holds the address and it is not a process of ours: on the
    # owner's stand llama-server is started by hand and ops/llm-server.pid does
    # not exist, so `make llm-up` used to start a SECOND server on a taken port
    # and then report the stranger's 200 as its own success (review M-6).
    llm_fail "llm: $LLM_EP_PROBE$LLM_HEALTH_VIA already answers $LLM_HEALTH_CODE and ops/llm-server.pid records no process of ours — refusing to start a second server on the same address. It is already usable (make llm-health); stop it by hand if you need to replace it"
    exit 1
  fi

  bin=$(llm_env_or MV_LLM_BIN '')
  [ -n "$bin" ] || {
    llm_fail 'llm: MV_LLM_BIN is not set (see .env.example, §4.2)'
    exit 1
  }
  [ -x "$bin" ] || {
    llm_fail "llm: MV_LLM_BIN points at $bin, which is not executable"
    exit 1
  }

  ctx=$(llm_env_or MV_LLM_NUM_CTX 8192)
  slots=$(llm_env_or MV_LLM_SLOTS 1)
  model_file=$(llm_env_or MV_LLM_MODEL_FILE '')
  models_dir=$(llm_env_or MV_LLM_MODELS_DIR '')
  slot_save_path=$(llm_env_or MV_LLM_SLOT_SAVE_PATH '')

  # The order of these arguments is the order of llm-server.ps1, to the letter:
  # two stands whose logs cannot be diffed are two stands nobody compares (N-1).
  server_args=(
    --host "$llm_host"
    # The port is the one the platform will call: it is read out of
    # $LLM_EP_VAR, never declared a second time (T-404).
    --port "$LLM_EP_PORT"
    # llama.cpp splits --ctx-size between the slots, so the per-slot context the
    # platform asks for has to be multiplied back up.
    --ctx-size "$((ctx * slots))"
    --parallel "$slots"
    --n-gpu-layers "$(llm_env_or MV_LLM_NGL 99)"
    --threads "$(llm_env_or MV_LLM_THREADS 32)"
    --batch-size "$(llm_env_or MV_LLM_BATCH_SIZE 16000)"
    --flash-attn on
    --kv-offload
    --kv-unified
    # Mandatory: without it the Qwen3 chat template (tools, thinking) is not
    # applied and chat_template_kwargs is ignored.
    --jinja
    --reasoning "$(llm_env_or MV_LLM_REASONING off)"
    --load-mode mmap+mlock
  )
  [ -n "$slot_save_path" ] && server_args+=(--slot-save-path "$slot_save_path")

  if [ "$router" = 1 ]; then
    # -m and --models-dir are mutually exclusive: with -m the server is in
    # single-model mode and the `model` field of a request is ignored
    # (ADR-005 add. 2 p. 4).
    [ -n "$models_dir" ] || {
      llm_fail 'llm: ROUTER mode needs MV_LLM_MODELS_DIR'
      exit 1
    }
    server_args+=(--models-dir "$models_dir" --models-max 2)
  else
    [ -n "$model_file" ] || {
      llm_fail 'llm: MV_LLM_MODEL_FILE is not set (single mode needs -m)'
      exit 1
    }
    server_args+=(-m "$model_file")
  fi

  if [ "$with_ui" = 1 ]; then
    # Extra attack surface: the UI and the MCP proxy are reachable by any
    # process on this machine. Loopback only, port never published (SEC-15).
    server_args+=(--tools all --ui-mcp-proxy)
    llm_say 'llm: WARNING web UI and MCP proxy are on (--with-ui); do not publish the port'
  else
    # The server serves a web UI on the same port by default; a service runtime
    # has no use for it.
    server_args+=(--no-webui)
  fi

  mode=single
  [ "$router" = 1 ] && mode=router
  llm_say "llm: starting $bin on $llm_host:$LLM_EP_PORT ($mode mode); the platform calls it at $LLM_EP_URL"
  rm -f "$log_file"
  nohup "$bin" "${server_args[@]}" >"$log_file" 2>&1 &
  server_pid=$!
  # Three lines, not one: the number alone is not evidence that the process is
  # ours when it is read back (review M-5). The resolved path is recorded, so
  # the check survives a later change of MV_LLM_BIN.
  {
    echo "$server_pid"
    echo "bin=$(readlink -f "$bin" 2>/dev/null || printf '%s' "$bin")"
    echo "started=$(process_started "$server_pid")"
  } >"$pid_file"

  # 503 (loading) -> 200 (ok), up to 180 s: a 27B model on a cold page cache
  # takes tens of seconds even with mmap (§6.3).
  #
  # The deadline is counted from the clock, not in iterations: 90 rounds of
  # "probe plus sleep 2" is 180 s only when the probe is instant, and against an
  # address that answers nothing each round costs up to two timeouts of 5 s, so
  # the loop used to run for a quarter of an hour while the message promised
  # 180 s (review Mi-7). The twin has had an honest deadline all along.
  deadline=$(($(date +%s) + 180))
  while [ "$(date +%s)" -lt "$deadline" ]; do
    if ! kill -0 "$server_pid" 2>/dev/null; then
      rm -f "$pid_file"
      llm_fail "llm: llama-server exited during start; see $log_file"
      exit 1
    fi
    llm_health_probe "$LLM_EP_PROBE"
    [ "$LLM_HEALTH_CODE" = 200 ] && break
    sleep 2
  done
  if [ "$LLM_HEALTH_CODE" != 200 ]; then
    llm_fail "llm: did not become healthy in 180 s (last $LLM_HEALTH_VIA = $LLM_HEALTH_CODE); see $log_file"
    exit 1
  fi
  llm_say "llm: $LLM_EP_PROBE$LLM_HEALTH_VIA = 200 (pid $server_pid, log $log_file)"

  # One short call: the first request after a start allocates the compute
  # buffers and warms the CUDA kernels, so it is both the warm-up and a rough
  # "the server answers" indicator. `make bench` measures it separately (B2)
  # and it never enters p50/p95.
  model_name=""
  [ "$router" = 0 ] && model_name=$(basename "${model_file%.gguf}")
  started=$(date +%s%3N 2>/dev/null || date +%s000)
  # The status code is checked, not just the exit code of curl: without it a 404
  # from a server with another set of endpoints was reported as a successful
  # first call here while the PowerShell twin reported a failure (review Mi-7).
  warm_code=$(llm_curl -o /dev/null -w '%{http_code}' --max-time 120 \
    -X POST "$LLM_EP_PROBE/v1/chat/completions" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"${model_name}\",\"messages\":[{\"role\":\"user\",\"content\":\"ok\"}],\"max_tokens\":1,\"chat_template_kwargs\":{\"enable_thinking\":false}}" 2>/dev/null) || warm_code=000
  finished=$(date +%s%3N 2>/dev/null || date +%s000)
  if [ "$warm_code" = 200 ]; then
    llm_say "llm: first call $((finished - started)) ms"
  else
    llm_say "llm: warm-up call answered $warm_code; the server is up, check the blueprint model name"
  fi
  ;;
esac
