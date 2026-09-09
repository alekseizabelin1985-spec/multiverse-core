#!/usr/bin/env bash
# =============================================================================
# scripts/llm-server.sh — native llama-server (llama.cpp): start, stop, health
# (infrastructure.md v0.3 §6.1-§6.3, §2.1; ADR-005 add. 2 p. 7)
# =============================================================================
# The WSL/Linux twin of scripts/llm-server.ps1, with the same argument set. On
# the owner's machine the PowerShell script is the one that runs: llama-server
# is a native Windows process with CUDA and D:\Models\ paths, and moving it into
# WSL would mean setting CUDA up twice for no gain (§6.1). This file exists for
# a Linux stand and for CI-like environments that have a Linux build.
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

action=up
router=0
with_ui=0
while [ $# -gt 0 ]; do
  case "$1" in
  up | down | health) action=$1 ;;
  --router) router=1 ;;
  --with-ui) with_ui=1 ;;
  -h | --help)
    sed -n '2,22p' "$0"
    exit 0
    ;;
  *)
    echo "llm: unknown argument $1" >&2
    exit 2
    ;;
  esac
  shift
done

llm_host=${MV_LLM_HOST:-127.0.0.1} # loopback ONLY (SEC-15)
llm_port=${MV_LLM_PORT:-1234}
base_url="http://${llm_host}:${llm_port}"

mkdir -p "$ops_dir"

# 000 when nothing answers; 503 while the model loads is a state, not a failure.
# curl both prints 000 through -w and exits 7 on a refused connection, so the
# `|| echo` shorthand would append a second value and yield 0000 — a code that
# matches no branch below.
health_code() {
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$base_url/health" 2>/dev/null) || true
  printf '%s' "${code:-000}"
}

running_pid() {
  [ -f "$pid_file" ] || return 1
  local recorded
  recorded=$(tr -d '[:space:]' <"$pid_file")
  [ -n "$recorded" ] || return 1
  kill -0 "$recorded" 2>/dev/null || return 1
  printf '%s' "$recorded"
}

pinned_build() {
  sed -n 's/^[[:space:]]*LLAMACPP_BUILD[[:space:]]*=[[:space:]]*\(.*\)$/\1/p' \
    "$repo_root/build/versions.env" | head -n 1
}

case "$action" in

down)
  if pid=$(running_pid); then
    echo "llm: stopping llama-server (pid $pid)"
    kill "$pid" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    echo 'llm: stopped; models and volumes are untouched'
  else
    echo 'llm: no live server in ops/llm-server.pid — nothing to stop'
    rm -f "$pid_file"
  fi
  ;;

health)
  code=$(health_code)
  case "$code" in
  200) echo "llm: $base_url/health = 200 ok" ;;
  503) echo "llm: $base_url/health = 503 loading (the model is still being read)" ;;
  000) echo "llm: $base_url/health unreachable — the process is not running (make llm-up)" ;;
  *) echo "llm: $base_url/health = $code" ;;
  esac

  if [ "$code" = "200" ]; then
    models=$(curl -s --max-time 5 "$base_url/v1/models" |
      sed -n 's/.*"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | paste -sd', ' -)
    echo "llm: models = ${models:-<none reported>}"
  fi

  pin=$(pinned_build)
  if [ -n "${MV_LLM_BIN:-}" ] && [ -x "${MV_LLM_BIN}" ]; then
    actual=$("$MV_LLM_BIN" --version 2>&1 | sed -n 's/.*build[:[:space:]]*\([0-9]\{1,\}\).*/b\1/p' | head -n 1)
    if [ -n "$actual" ] && [ -n "$pin" ] && [ "$actual" != "$pin" ]; then
      # A warning, not an error (§6.2): the pin only becomes true after the
      # golden set has been re-run against the new build (§9.3).
      echo "llm: WARNING build $actual differs from the pin $pin in build/versions.env (infrastructure.md §9.5)"
    elif [ -n "$actual" ]; then
      echo "llm: build $actual matches the pin"
    fi
  elif [ -n "$pin" ]; then
    echo "llm: pinned build $pin; MV_LLM_BIN is not set, cannot compare"
  fi

  if command -v nvidia-smi >/dev/null 2>&1; then
    echo "llm: VRAM $(nvidia-smi --query-gpu=memory.used,memory.total --format=csv,noheader | paste -sd'; ' -)"
  fi

  [ "$code" = "200" ] || exit 1
  ;;

up)
  if pid=$(running_pid) && [ "$(health_code)" != "000" ]; then
    echo "llm: already running (pid $pid) on $base_url — nothing to do"
    exit 0
  fi
  if pid=$(running_pid); then
    echo "llm: pid $pid is recorded but $base_url does not answer; restarting"
    kill -9 "$pid" 2>/dev/null || true
  fi

  : "${MV_LLM_BIN:?MV_LLM_BIN is not set (see .env.example, §4.2)}"
  [ -x "$MV_LLM_BIN" ] || {
    echo "llm: MV_LLM_BIN points at $MV_LLM_BIN, which is not executable" >&2
    exit 1
  }

  ctx=${MV_LLM_NUM_CTX:-8192}
  slots=${MV_LLM_SLOTS:-1}

  server_args=(
    --host "$llm_host"
    --port "$llm_port"
    # llama.cpp splits --ctx-size between the slots, so the per-slot context the
    # platform asks for has to be multiplied back up.
    --ctx-size "$((ctx * slots))"
    --parallel "$slots"
    --n-gpu-layers "${MV_LLM_NGL:-99}"
    --threads "${MV_LLM_THREADS:-32}"
    --batch-size "${MV_LLM_BATCH_SIZE:-16000}"
    --flash-attn on
    --kv-offload
    --kv-unified
    # Mandatory: without it the Qwen3 chat template (tools, thinking) is not
    # applied and chat_template_kwargs is ignored.
    --jinja
    --reasoning "${MV_LLM_REASONING:-off}"
    --load-mode mmap+mlock
  )
  [ -n "${MV_LLM_SLOT_SAVE_PATH:-}" ] && server_args+=(--slot-save-path "$MV_LLM_SLOT_SAVE_PATH")

  if [ "$router" = 1 ]; then
    # -m and --models-dir are mutually exclusive: with -m the server is in
    # single-model mode and the `model` field of a request is ignored
    # (ADR-005 add. 2 p. 4).
    : "${MV_LLM_MODELS_DIR:?ROUTER mode needs MV_LLM_MODELS_DIR}"
    server_args+=(--models-dir "$MV_LLM_MODELS_DIR" --models-max 2)
  else
    : "${MV_LLM_MODEL_FILE:?MV_LLM_MODEL_FILE is not set (single mode needs -m)}"
    server_args+=(-m "$MV_LLM_MODEL_FILE")
  fi

  if [ "$with_ui" = 1 ]; then
    # Extra attack surface: the UI and the MCP proxy are reachable by any
    # process on this machine. Loopback only, port never published (SEC-15).
    server_args+=(--tools all --ui-mcp-proxy)
    echo 'llm: WARNING web UI and MCP proxy are on (--with-ui); do not publish the port'
  else
    # The server serves a web UI on the same port by default; a service runtime
    # has no use for it.
    server_args+=(--no-webui)
  fi

  mode=single
  [ "$router" = 1 ] && mode=router
  echo "llm: starting $MV_LLM_BIN on $base_url ($mode mode)"
  rm -f "$log_file"
  nohup "$MV_LLM_BIN" "${server_args[@]}" >"$log_file" 2>&1 &
  server_pid=$!
  echo "$server_pid" >"$pid_file"

  # 503 (loading) -> 200 (ok), up to 180 s: a 27B model on a cold page cache
  # takes tens of seconds even with mmap (§6.3).
  code=0
  for _ in $(seq 1 90); do
    if ! kill -0 "$server_pid" 2>/dev/null; then
      rm -f "$pid_file"
      echo "llm: llama-server exited during start; see $log_file" >&2
      exit 1
    fi
    code=$(health_code)
    [ "$code" = "200" ] && break
    sleep 2
  done
  if [ "$code" != "200" ]; then
    echo "llm: did not become healthy in 180 s (last /health = $code); see $log_file" >&2
    exit 1
  fi
  echo "llm: /health = 200 (pid $server_pid, log $log_file)"

  # One short call: the first request after a start allocates the compute
  # buffers and warms the CUDA kernels, so it is both the warm-up and a rough
  # "the server answers" indicator. `make bench` measures it separately (B2)
  # and it never enters p50/p95.
  model_name=""
  [ "$router" = 0 ] && model_name=$(basename "${MV_LLM_MODEL_FILE%.gguf}")
  started=$(date +%s%3N 2>/dev/null || date +%s000)
  if curl -s -o /dev/null --max-time 120 -X POST "$base_url/v1/chat/completions" \
    -H 'Content-Type: application/json' \
    -d "{\"model\":\"${model_name}\",\"messages\":[{\"role\":\"user\",\"content\":\"ok\"}],\"max_tokens\":1,\"chat_template_kwargs\":{\"enable_thinking\":false}}"; then
    finished=$(date +%s%3N 2>/dev/null || date +%s000)
    echo "llm: first call $((finished - started)) ms"
  else
    echo 'llm: warm-up call failed; the server is up, check the blueprint model name'
  fi
  ;;
esac
