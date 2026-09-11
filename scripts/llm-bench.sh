#!/usr/bin/env bash
# =============================================================================
# scripts/llm-bench.sh — the LLM measurement matrix (F-8)
# (infrastructure.md v0.3 §6.4, overview.md §18.1, nfr.md «Что измерить первым»,
#  metrics.md §7 B1-B5, ADR-005 add. 2 p. 5)
# =============================================================================
# Runs testdata/bench/prompts.jsonl against a llama-server (or any other
# OpenAI-compatible endpoint) that is ALREADY RUNNING, and writes what the
# choice between the E / C / A configurations rests on:
#
#   ops/metrics/bench-<date>.csv           — the columns of §6.4, one row per cell
#   ops/metrics/bench-<config>-<date>.json — the same numbers plus every single
#                                            request, for a second look later
#   a table on stdout                      — what the operator reads on the stand
#
# This is the twin of scripts/llm-bench.ps1 and the one that runs on the owner's
# machine today: PowerShell 7 is not installed there, Git Bash is (journal
# 2026-09-10, ОВ-43). The two scripts write the SAME columns — a divergence is
# a review finding, not a detail.
#
# What this script deliberately does NOT do:
#   * it never starts, restarts or reconfigures llama-server. --ctx-size, the KV
#     cache type and the model are properties of the running process; a script
#     that restarted the server would measure its own start-up parameters and
#     depend on paths and privileges it has no business knowing (§6.4 rule 1).
#     Start it first: `make llm-up` (or `make llm-up ROUTER=1` for E+ / A).
#   * it does not decide anything. The verdict column applies the thresholds of
#     ops/metrics/bench-matrix.json to one run; the decision needs three runs at
#     different moments and is written down by hand in ops/metrics/baseline.md.
#
# Dependencies: curl and python3. There is no jq on the target machine, so JSON
# is handled by the embedded python helper below (deviation from §6.4 rule 6,
# which assumed jq + awk). python3 is used as a JSON tool only: every HTTP call
# is still curl, so the timings measured are the timings of the same client the
# platform uses on the stand.
#
# Usage:
#   scripts/llm-bench.sh [--configs E,C] [--num-ctx 8192] [--kv-cache f16]
#                        [--repeats 1] [--placement native] [--limit N]
#                        [--matrix PATH] [--prompts PATH] [--out-dir DIR]
#
# Exit codes: 0 — measured; 2 — cannot measure (bad arguments, missing files or
# tools, no server on the URL, no phase left to run). A failing verdict is a
# result, not an error, and exits 0.
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)

# The address of the endpoint is derived in one place for the whole project —
# the same file scripts/llm-server.sh uses. Before T-404 review #1 this script
# had its own rule (it trimmed a trailing /v1, it did NOT rewrite
# host.docker.internal and it did NOT fall back from /health to /v1/models), and
# on the owner's stand that made `make bench` refuse to run against a server
# that was answering (review M-1).
#
# Its absence is reported as one English line naming the file, not as a shell
# error about a path (review Mi-3).
llm_lib="$repo_root/scripts/lib/llm-endpoint.sh"
if [ ! -f "$llm_lib" ]; then
  echo "bench: $llm_lib is missing — it holds the only rule that turns MV_LLM_URL into an address, and no script of this project can read the variable without it" >&2
  exit 2
fi
# shellcheck source=scripts/lib/llm-endpoint.sh
. "$llm_lib"

matrix="$repo_root/ops/metrics/bench-matrix.json"
prompts="$repo_root/testdata/bench/prompts.jsonl"
out_dir="$repo_root/ops/metrics"
configs=E
num_ctx=${MV_LLM_NUM_CTX:-8192}
kv_cache=f16
placement=native
repeats=1
limit=0 # 0 = n_per_cell from the matrix

die() {
  echo "bench: $1" >&2
  exit 2
}

usage() {
  # The header of this file is the only description of the arguments; printing
  # it beats a second copy that drifts away from it.
  sed -n '/^# Usage:/,/^# =\{10,\}$/p' "$0"
}

while [ $# -gt 0 ]; do
  case "$1" in
  --configs)
    configs=${2:?--configs needs a value}
    shift
    ;;
  --matrix)
    matrix=${2:?--matrix needs a path}
    shift
    ;;
  --prompts)
    prompts=${2:?--prompts needs a path}
    shift
    ;;
  --out-dir)
    out_dir=${2:?--out-dir needs a path}
    shift
    ;;
  --num-ctx)
    num_ctx=${2:?--num-ctx needs a value}
    shift
    ;;
  --kv-cache)
    kv_cache=${2:?--kv-cache needs a value}
    shift
    ;;
  --placement)
    placement=${2:?--placement needs a value}
    shift
    ;;
  --repeats)
    repeats=${2:?--repeats needs a value}
    shift
    ;;
  --limit)
    limit=${2:?--limit needs a value}
    shift
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  *) die "unknown argument $1 (try --help)" ;;
  esac
  shift
done

command -v curl >/dev/null 2>&1 || die 'curl not found in PATH'
command -v python3 >/dev/null 2>&1 || die 'python3 not found in PATH (it is the JSON tool of this script; jq is not used)'
[ -f "$matrix" ] || die "matrix $matrix not found"
[ -f "$prompts" ] || die "prompt set $prompts not found"
case "$repeats" in '' | *[!0-9]*) die "--repeats must be a whole number, got $repeats" ;; esac
[ "$repeats" -ge 1 ] || die '--repeats must be at least 1'

tmpdir=$(mktemp -d)
cleanup() {
  rm -rf "$tmpdir"
  # A header-only CSV is litter in a directory the architect reads as results;
  # it appears whenever the run stopped before the first cell was measured.
  if [ -n "${csv:-}" ] && [ -f "$csv" ] && [ "$(wc -l <"$csv")" -le 1 ]; then
    rm -f "$csv"
  fi
}
trap cleanup EXIT

# -----------------------------------------------------------------------------
# The JSON half of the script. Everything that has to look inside a JSON
# document lives here; the shell only orchestrates curl and prints the table.
# -----------------------------------------------------------------------------
cat >"$tmpdir/bench.py" <<'PY'
"""JSON helper of scripts/llm-bench.sh (and nothing else).

Sub-commands print `key=value` lines that the shell reads with `read -r`, or
write files. Values never contain spaces or newlines, which is what keeps the
shell side free of quoting games.
"""
import csv
import json
import os
import re
import sys

# Git Bash runs a native Windows python, whose stdout translates a line feed
# into CRLF. The shell reads these lines back as variable names and compares
# model names with grep -Fx, and a trailing CR turns both into silent nonsense
# ("MV_LLM_URL<CR>: invalid variable name"), so the translation is switched off
# here instead of being stripped in ten places on the shell side.
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", newline="\n")

CJK = re.compile("[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff]")
LETTER = re.compile(r"[^\W\d_]", re.UNICODE)
LATIN = re.compile(r"[A-Za-z]")


def texts_at(value, paths):
    """The strings the player would actually read, by dotted path.

    NFR-090 is about narrative text, not about an envelope: `kind`, `player-A`
    and `region.event_occurred` are identifiers, and counting their letters
    would report every structured answer as 90 % Latin. The gateway checks the
    language of Call.TextPaths only (swarm-llm-laws.md 9.2) and the bench keeps
    the same rule, so that a verdict here means the same thing as a rejection
    there. `a[].b` walks a list.
    """
    found = []

    def walk(node, rest):
        if not rest:
            if isinstance(node, str):
                found.append(node)
            return
        head, tail = rest[0], rest[1:]
        if head.endswith("[]"):
            head = head[:-2]
            branch = node.get(head) if isinstance(node, dict) else None
            for item in branch or []:
                walk(item, tail)
            return
        if isinstance(node, dict) and head in node:
            walk(node[head], tail)

    for path in paths:
        walk(value, path.split("."))
    return found


def fail(message):
    sys.stderr.write("bench: %s\n" % message)
    raise SystemExit(2)


def load_matrix(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def cmd_config(argv):
    """Everything the shell needs to know about one configuration."""
    matrix_path, name = argv[0], argv[1]
    matrix = load_matrix(matrix_path)
    configs = matrix.get("configs", {})
    if name not in configs:
        fail("configuration %s is not in %s (have: %s)"
             % (name, matrix_path, ", ".join(sorted(configs))))
    cfg = configs[name]
    request = matrix.get("request", {})
    timeouts = request.get("timeout_s", {})
    thresholds = matrix.get("thresholds", {})
    nfr002 = thresholds.get("NFR-002", {})
    group = thresholds.get("NFR-002-group3", {})
    nfr090 = thresholds.get("NFR-090", {})
    b3 = thresholds.get("B3", {})
    out = {
        "provider": cfg.get("provider", ""),
        "url_env": cfg.get("url_env", "MV_LLM_URL"),
        "tick_model": cfg.get("tick", ""),
        "narrative_model": cfg.get("narrative", ""),
        "server_mode": cfg.get("server_mode", ""),
        "n_per_cell": matrix.get("n_per_cell", 10),
        "runs_required": matrix.get("runs_required", 3),
        "timeout_tick": timeouts.get("tick", 120),
        "timeout_phase2": timeouts.get("phase2", 120),
        "timeout_group": timeouts.get("phase2-group3", 180),
        "p95_max": nfr002.get("p95_ms_max", 5000),
        "group_p95_max": group.get("p95_ms_max", 5000),
        "group_advisory": 1 if group.get("advisory") else 0,
        "latin_max": nfr090.get("latin_ratio_max", 0.10),
        # cjk_ratio_max is not passed on: NFR-090 sets it to zero, and the
        # answer-level rule is "a single CJK character fails the answer". A
        # non-zero value in the matrix would need a different rule, not a
        # different number, so it is better to notice its absence here.
        "lang_pass_min": nfr090.get("pass_ratio_min", 0.98),
        "valid_json_min": b3.get("valid_json_ratio_min", 0.98),
    }
    for key in sorted(out):
        print("%s=%s" % (key, out[key]))


def cmd_bodies(argv):
    """Turn the prompts of one phase into ready request bodies.

    Written as files rather than piped through the shell: a prompt carries its
    own JSON Schema, and passing that through a command line would be one
    quoting accident away from measuring the wrong request.
    """
    matrix_path, prompts_path, phase, model, limit, out_dir = argv[:6]
    matrix = load_matrix(matrix_path)
    request = matrix.get("request", {})
    sampling = request.get("sampling_non_thinking", {})
    limit = int(limit)

    written = 0
    with open(prompts_path, encoding="utf-8") as handle:
        for lineno, line in enumerate(handle, 1):
            line = line.strip()
            if not line:
                continue
            try:
                prompt = json.loads(line)
            except ValueError as err:
                fail("%s line %d is not JSON: %s" % (prompts_path, lineno, err))
            if prompt.get("phase") != phase:
                continue
            for key in ("id", "schema_name", "schema", "max_tokens", "system", "user"):
                if key not in prompt:
                    fail("%s line %d has no %r" % (prompts_path, lineno, key))
            body = {
                "model": model,
                "stream": request.get("stream", False),
                "max_tokens": prompt["max_tokens"],
                "response_format": {
                    "type": "json_schema",
                    "json_schema": {
                        "name": prompt["schema_name"],
                        "schema": prompt["schema"],
                        "strict": bool(request.get("strict", True)),
                    },
                },
                # Not a preference: with thinking on, llama.cpp does not apply
                # the grammar of the schema (#20345), so the measurement would
                # be about the defect instead of about the model.
                "chat_template_kwargs": request.get(
                    "chat_template_kwargs", {"enable_thinking": False}
                ),
                "messages": [
                    {"role": "system", "content": prompt["system"]},
                    {"role": "user", "content": prompt["user"]},
                ],
            }
            for key in ("temperature", "top_p", "top_k", "min_p", "presence_penalty"):
                if key in sampling:
                    body[key] = sampling[key]
            written += 1
            target = os.path.join(out_dir, "body-%03d.json" % written)
            with open(target, "w", encoding="utf-8") as out:
                json.dump(body, out, ensure_ascii=False)
            # The shell passes these back to `parse`: the prompt knows which
            # of its fields hold narrative text, the script does not.
            print("%03d\t%s\t%s" % (written, prompt["id"],
                                   ",".join(prompt.get("text_paths") or [])))
            if limit and written >= limit:
                break
    if written == 0:
        fail("no prompts with phase=%s in %s" % (phase, prompts_path))


def cmd_parse(argv):
    """One answer -> the numbers of one row of the request log."""
    resp_path, latin_max = argv[0], float(argv[1])
    text_paths = [p for p in (argv[2] if len(argv) > 2 else "").split(",") if p]
    values = {
        "ok_json": 0,
        "cjk": 0,
        "latin": 0.0,
        "lang_pass": 0,
        "prompt_tokens": 0,
        "completion_tokens": 0,
        "cached_tokens": 0,
        "predicted_ms": 0.0,
        "error": "",
    }
    try:
        with open(resp_path, encoding="utf-8") as handle:
            payload = json.load(handle)
    except (IOError, ValueError):
        values["error"] = "unreadable_response"
        for key in sorted(values):
            print("%s=%s" % (key, values[key]))
        return

    if isinstance(payload, dict) and "error" in payload and "choices" not in payload:
        # llama-server reports its own failures in the same envelope; keeping the
        # code makes "the model is not loaded" distinguishable from "the model
        # answered badly", which are different problems on the stand.
        error = payload["error"]
        code = error.get("code") if isinstance(error, dict) else None
        values["error"] = "server_error_%s" % (code if code is not None else "unknown")
        for key in sorted(values):
            print("%s=%s" % (key, values[key]))
        return

    usage = payload.get("usage") or {}
    details = usage.get("prompt_tokens_details") or {}
    timings = payload.get("timings") or {}
    values["prompt_tokens"] = int(usage.get("prompt_tokens") or 0)
    values["completion_tokens"] = int(usage.get("completion_tokens") or 0)
    values["cached_tokens"] = int(details.get("cached_tokens") or 0)
    values["predicted_ms"] = float(timings.get("predicted_ms") or 0.0)

    choices = payload.get("choices") or []
    content = ""
    if choices:
        content = (choices[0].get("message") or {}).get("content") or ""
    if not content:
        values["error"] = "empty_content"
    try:
        json.loads(content)
        values["ok_json"] = 1
    except ValueError:
        pass

    # A valid answer is measured on its text fields; a broken one is measured
    # as it arrived — there is nothing else to look at, and a wall of Latin in a
    # non-JSON answer is a language finding as much as a format one.
    if values["ok_json"] and text_paths:
        narrative = " ".join(texts_at(json.loads(content), text_paths))
    else:
        narrative = content
    has_cjk = 1 if CJK.search(narrative) else 0
    letters = len(LETTER.findall(narrative))
    latin = float(len(LATIN.findall(narrative))) / letters if letters else 0.0
    values["cjk"] = has_cjk
    values["latin"] = round(latin, 4)
    values["lang_pass"] = 1 if (not has_cjk and latin <= latin_max) else 0
    for key in sorted(values):
        print("%s=%s" % (key, values[key]))


REQUEST_FIELDS = [
    "config", "repeat", "phase", "index", "prompt_id", "http", "latency_ms",
    "ok_json", "cjk", "latin", "lang_pass", "prompt_tokens", "completion_tokens",
    "cached_tokens", "predicted_ms", "error",
]


def read_requests(path):
    with open(path, encoding="utf-8", newline="") as handle:
        return list(csv.DictReader(handle, fieldnames=REQUEST_FIELDS))


def percentile(sorted_values, fraction):
    """The same rule as scripts/llm-bench.ps1: nearest rank, clamped.

    With n = 10 the p95 is the largest sample. That is on purpose — the number
    of prompts per cell is what it is, and pretending to interpolate a 95th
    percentile out of ten points would look more precise than it is. Three runs
    of ten are what the decision rests on (bench-matrix.json runs_required).
    """
    if not sorted_values:
        return 0
    index = int(len(sorted_values) * fraction)
    index = min(index, len(sorted_values) - 1)
    return sorted_values[index]


def cmd_cell(argv):
    """Aggregate one (config, repeat, phase) cell and apply the thresholds."""
    (requests_path, config, repeat, phase, p95_max, lang_pass_min,
     valid_json_min, advisory) = argv[:8]
    rows = [
        row for row in read_requests(requests_path)
        if row["config"] == config and row["repeat"] == repeat and row["phase"] == phase
    ]
    answered = [row for row in rows if row["http"] == "200" and not row["error"]]
    n = len(answered)
    out = {
        "n": n,
        "attempted": len(rows),
        "errors": len(rows) - n,
        "p50_ms": 0,
        "p95_ms": 0,
        "prompt_tok": 0,
        "completion_tok": 0,
        "cached_tok": 0,
        "tps": 0,
        "valid_json_ratio": 0,
        "cjk_ratio": 0,
        "latin_ratio": 0,
        "lang_pass_ratio": 0,
        "verdict": "n/a",
    }
    if n == 0:
        for key in sorted(out):
            print("%s=%s" % (key, out[key]))
        return

    latencies = sorted(int(row["latency_ms"]) for row in answered)
    completion = sum(int(row["completion_tokens"]) for row in answered)
    predicted_ms = sum(float(row["predicted_ms"]) for row in answered)
    out["p50_ms"] = percentile(latencies, 0.5)
    out["p95_ms"] = percentile(latencies, 0.95)
    out["prompt_tok"] = sum(int(row["prompt_tokens"]) for row in answered)
    out["completion_tok"] = completion
    out["cached_tok"] = sum(int(row["cached_tokens"]) for row in answered)
    # Tokens per second of generation, from the server's own timings: the wall
    # clock of the client also contains prompt processing and the HTTP hop, and
    # mixing the two would make the number incomparable between runs.
    if predicted_ms > 0:
        out["tps"] = round(completion / (predicted_ms / 1000.0), 1)
    valid_json = sum(int(row["ok_json"]) for row in answered)
    lang_pass = sum(int(row["lang_pass"]) for row in answered)
    out["valid_json_ratio"] = round(float(valid_json) / n, 3)
    out["cjk_ratio"] = round(float(sum(int(row["cjk"]) for row in answered)) / n, 3)
    out["latin_ratio"] = round(
        sum(float(row["latin"]) for row in answered) / n, 3)
    out["lang_pass_ratio"] = round(float(lang_pass) / n, 3)

    if phase.startswith("phase2"):
        passed = (
            out["p95_ms"] <= float(p95_max)
            and out["lang_pass_ratio"] >= float(lang_pass_min)
            and out["valid_json_ratio"] >= float(valid_json_min)
        )
        # An error that never produced an answer is a failure of the cell too:
        # a configuration that times out on a fifth of the prompts has not
        # passed NFR-002, whatever the p95 of the answers that did arrive says.
        if out["errors"] > 0:
            passed = False
        out["verdict"] = "pass" if passed else "fail"
        if advisory == "1":
            out["verdict"] += "*"
    for key in sorted(out):
        print("%s=%s" % (key, out[key]))


def cmd_report(argv):
    """The per-configuration JSON report next to the CSV.

    The CSV is the comparable summary the architect reads; this file keeps every
    request behind it, so a p95 that looks wrong can be traced to the prompt
    that produced it without running the matrix again.
    """
    out_path, requests_path, cells_path = argv[0], argv[1], argv[2]
    meta = {}
    for pair in argv[3:]:
        key, _, value = pair.partition("=")
        meta[key] = value

    cells = []
    with open(cells_path, encoding="utf-8", newline="") as handle:
        for row in csv.DictReader(handle):
            for key, value in list(row.items()):
                if key in ("phase", "model", "verdict"):
                    continue
                try:
                    row[key] = int(value)
                except (TypeError, ValueError):
                    try:
                        row[key] = float(value)
                    except (TypeError, ValueError):
                        pass
            cells.append(row)

    requests = []
    for row in read_requests(requests_path):
        for key in ("repeat", "index", "latency_ms", "ok_json", "cjk", "lang_pass",
                    "prompt_tokens", "completion_tokens", "cached_tokens"):
            row[key] = int(row[key])
        row["latin"] = float(row["latin"])
        row["predicted_ms"] = float(row["predicted_ms"])
        requests.append(row)

    document = {
        "schema": "multiverse.bench.v1",
        "generated_by": "scripts/llm-bench.sh",
        "meta": meta,
        "cells": cells,
        "requests": requests,
    }
    with open(out_path, "w", encoding="utf-8", newline="\n") as out:
        json.dump(document, out, ensure_ascii=False, indent=2, sort_keys=False)
        out.write("\n")


def cmd_models(argv):
    """The model names of GET /v1/models, one per line (empty file = unknown)."""
    try:
        with open(argv[0], encoding="utf-8") as handle:
            payload = json.load(handle)
    except (IOError, ValueError):
        return
    for item in payload.get("data") or []:
        name = item.get("id")
        if name:
            print(name)


COMMANDS = {
    "config": cmd_config,
    "bodies": cmd_bodies,
    "parse": cmd_parse,
    "cell": cmd_cell,
    "report": cmd_report,
    "models": cmd_models,
}

if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        fail("internal: unknown helper command %r" % (sys.argv[1:2],))
    COMMANDS[sys.argv[1]](sys.argv[2:])
PY

py() { python3 "$tmpdir/bench.py" "$@"; }

now_ms() { date +%s%3N 2>/dev/null || date +%s000; }

# The build the numbers belong to (§6.4 rule 5). The running binary is the truth;
# the pin in build/versions.env is what we fall back to when MV_LLM_BIN is not
# set, and it is marked as a pin so that a stale pin cannot be read as a fact.
resolve_build() {
  local pin actual
  pin=$(sed -n 's/^[[:space:]]*LLAMACPP_BUILD[[:space:]]*=[[:space:]]*\(.*\)$/\1/p' \
    "$repo_root/build/versions.env" 2>/dev/null | head -n 1)
  if [ -n "${MV_LLM_BIN:-}" ] && [ -x "${MV_LLM_BIN}" ]; then
    actual=$("$MV_LLM_BIN" --version 2>&1 |
      sed -n 's/.*build[:[:space:]]*\([0-9]\{1,\}\).*/b\1/p' | head -n 1)
    [ -n "$actual" ] && {
      printf '%s' "$actual"
      return
    }
  fi
  printf '%s' "${pin:-unknown}(pin)"
}

read_vram() {
  if command -v nvidia-smi >/dev/null 2>&1; then
    nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits 2>/dev/null |
      head -n 1 | tr -d '[:space:]'
  fi
}

mkdir -p "$out_dir"
stamp=$(date +%Y%m%d-%H%M)
started_at=$(date +%Y-%m-%dT%H:%M:%S)
csv="$out_dir/bench-$stamp.csv"
requests_csv="$tmpdir/requests.csv"
: >"$requests_csv"

# The columns of infrastructure.md §6.4, in that order, plus `repeat` and
# `started_at` appended at the end: one invocation may hold several runs, and
# without them two rows of the same cell are indistinguishable. Appending keeps
# every reader of the documented prefix working.
echo 'config,provider,placement,model,phase,num_ctx,kv_cache,n,first_call_ms,p50_ms,p95_ms,prompt_tok,completion_tok,cached_tok,tps,valid_json_ratio,cjk_ratio,latin_ratio,lang_pass_ratio,vram_used_mb,llamacpp_build,verdict,repeat,started_at' >"$csv"

build=$(resolve_build)
printf 'bench: build %s, prompts %s, matrix %s\n' "$build" "$prompts" "$matrix" >&2

rows_written=0

# The table header waits for the first measured cell: printed up front, it would
# stand above an error message and read as "a run happened and found nothing".
print_row() {
  if [ "$rows_written" -eq 0 ]; then
    printf '%-4s %-14s %4s %8s %8s %6s %6s %6s %6s %s\n' \
      CFG PHASE N P50 P95 TPS JSON LANG CACHE VERDICT
  fi
  printf '%-4s %-14s %4s %8s %8s %6s %6s %6s %6s %s\n' "$@"
}

IFS=',' read -r -a config_list <<<"$configs"
for config in "${config_list[@]}"; do
  [ -n "$config" ] || continue
  config_rows=0

  # --- the configuration and the endpoint ------------------------------------
  unset provider url_env tick_model narrative_model server_mode n_per_cell
  unset runs_required timeout_tick timeout_phase2 timeout_group
  unset p95_max group_p95_max group_advisory latin_max lang_pass_min valid_json_min
  # Through a file rather than <(...): a failing process substitution leaves the
  # variables unset, and the script would carry on with an empty endpoint.
  py config "$matrix" "$config" >"$tmpdir/config.env" ||
    die "configuration $config could not be read from $matrix"
  while IFS='=' read -r key value; do
    printf -v "$key" '%s' "$value"
  done <"$tmpdir/config.env"

  llm_endpoint_resolve_var "$url_env" ||
    die "${LLM_EP_ERROR#llm: } (configuration $config expects the endpoint there; see .env.example and ops/metrics/README.md)"
  # $url is the address as configured (what the report shows), $base is the same
  # address as reachable from this machine (host.docker.internal rewritten to
  # 127.0.0.1). Both http://host:1234 and http://host:11434/v1 are legal values
  # of the variable — llama-server serves /v1/... at the root, Ollama's OpenAI
  # layer lives under /v1 — and the module trims the trailing /v1 for both.
  # Both go into the report: `url` as the variable was set and `probe_url` as it
  # was knocked on. Writing only the probe made two stands incomparable — from
  # `url=http://127.0.0.1:18993` nobody can tell whether the machine was
  # configured with the container alias or with loopback (review Mi-6).
  url=$LLM_EP_RAW
  base=$LLM_EP_PROBE

  cell_n=${n_per_cell:-10}
  [ "$limit" -gt 0 ] && cell_n=$limit

  # --- the server has to be there, and loaded (§6.3) --------------------------
  # /health belongs to llama.cpp and not to the contract: the module falls back
  # to /v1/models, the endpoint the platform actually needs, exactly as
  # scripts/llm-server.sh does. Without that fallback this gate declared the
  # owner's running server dead, because it answers /health with 404 (M-1).
  llm_health_probe "$base"
  waited=0
  while [ "$LLM_HEALTH_CODE" = "503" ] && [ "$waited" -lt 180 ]; do
    # 503 is "loading the model", a state and not a failure; a 27B on a cold
    # page cache takes tens of seconds even with mmap.
    printf 'bench: %s/health = 503 (model loading), waited %ss\n' "$base" "$waited" >&2
    sleep 5
    waited=$((waited + 5))
    llm_health_probe "$base"
  done
  case "$LLM_HEALTH_CODE" in
  200) : ;;
  000) die "no answer from $base — the LLM process is not running. Start it: make llm-up (config $config expects $url_env=$url)" ;;
  503) die "$base/health is still 503 after 180 s — the model has not finished loading; check ops/llm-server.log" ;;
  *) die "$base$LLM_HEALTH_VIA answered $LLM_HEALTH_CODE — this is not a llama-server / OpenAI-compatible endpoint (config $config, $url_env=$url)" ;;
  esac

  llm_curl -o "$tmpdir/models.json" --max-time 10 "$base/v1/models" >/dev/null 2>&1 || true
  py models "$tmpdir/models.json" >"$tmpdir/models.txt" || : >"$tmpdir/models.txt"

  first_call=0
  vram=$(read_vram)

  for repeat in $(seq 1 "$repeats"); do
    # --- B2: the cold first call, kept out of p50/p95 ------------------------
    # The first request after a start allocates the compute buffers and warms
    # the CUDA kernels; folding it into the percentiles would make every run
    # depend on how long ago the server came up.
    first_start=$(now_ms)
    # llm_curl, not curl: the key travels in a curl config on stdin instead of
    # the argument list, where every process on the machine can read it (SEC-22,
    # ADR-009 p. 4), and a key containing a quote or a backslash is escaped
    # rather than silently mangled (review Mi-3).
    first_code=$(llm_curl -o "$tmpdir/first.json" -w '%{http_code}' --max-time 120 \
      -X POST "$base/v1/chat/completions" \
      -H 'Content-Type: application/json; charset=utf-8' \
      --data-binary "{\"model\":\"$narrative_model\",\"max_tokens\":1,\"stream\":false,\"chat_template_kwargs\":{\"enable_thinking\":false},\"messages\":[{\"role\":\"user\",\"content\":\"ping\"}]}" 2>/dev/null) || first_code=000
    first_call=$(($(now_ms) - first_start))
    [ "$first_code" = "200" ] || printf 'bench: warm-up call answered %s (first_call_ms is still recorded)\n' "$first_code" >&2

    for phase in tick phase2 phase2-group3; do
      case "$phase" in
      tick)
        model=$tick_model
        timeout=$timeout_tick
        threshold=$p95_max
        advisory=0
        ;;
      phase2)
        model=$narrative_model
        timeout=$timeout_phase2
        threshold=$p95_max
        advisory=0
        ;;
      phase2-group3)
        model=$narrative_model
        timeout=$timeout_group
        threshold=$group_p95_max
        advisory=$group_advisory
        ;;
      esac

      # A model the endpoint does not serve is an operator error (wrong server
      # mode, wrong pull), not a measurement — skipping loudly beats measuring
      # whatever the server substitutes.
      if [ -s "$tmpdir/models.txt" ] && ! grep -Fxq "$model" "$tmpdir/models.txt"; then
        printf 'bench: model %s is not in %s/v1/models — phase %s skipped (server mode %s?)\n' \
          "$model" "$base" "$phase" "${server_mode:-single}" >&2
        continue
      fi

      bodies_dir="$tmpdir/bodies-$config-$repeat-$phase"
      mkdir -p "$bodies_dir"
      py bodies "$matrix" "$prompts" "$phase" "$model" "$cell_n" "$bodies_dir" >"$tmpdir/index.txt"

      while IFS=$'\t' read -r idx prompt_id text_paths; do
        [ -n "$idx" ] || continue
        start=$(now_ms)
        http=$(llm_curl -o "$tmpdir/resp.json" -w '%{http_code}' --max-time "$timeout" \
          -X POST "$base/v1/chat/completions" \
          -H 'Content-Type: application/json; charset=utf-8' \
          --data-binary "@$bodies_dir/body-$idx.json" 2>/dev/null) || http=000
        latency=$(($(now_ms) - start))

        unset ok_json cjk latin lang_pass prompt_tokens completion_tokens cached_tokens predicted_ms error
        py parse "$tmpdir/resp.json" "$latin_max" "${text_paths:-}" >"$tmpdir/parsed.env" ||
          die "internal: the answer parser failed on $prompt_id"
        while IFS='=' read -r key value; do
          printf -v "$key" '%s' "$value"
        done <"$tmpdir/parsed.env"
        [ "$http" = "200" ] || error="http_$http"

        printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
          "$config" "$repeat" "$phase" "$idx" "$prompt_id" "$http" "$latency" \
          "$ok_json" "$cjk" "$latin" "$lang_pass" "$prompt_tokens" \
          "$completion_tokens" "$cached_tokens" "$predicted_ms" "$error" \
          >>"$requests_csv"
        printf '.' >&2
      done <"$tmpdir/index.txt"
      printf '\n' >&2

      unset n attempted errors p50_ms p95_ms prompt_tok completion_tok cached_tok
      unset tps valid_json_ratio cjk_ratio latin_ratio lang_pass_ratio verdict
      py cell "$requests_csv" "$config" "$repeat" "$phase" \
        "$threshold" "$lang_pass_min" "$valid_json_min" "$advisory" >"$tmpdir/cell.env" ||
        die "internal: the aggregator failed on $config/$phase"
      while IFS='=' read -r key value; do
        printf -v "$key" '%s' "$value"
      done <"$tmpdir/cell.env"

      if [ "${n:-0}" -eq 0 ]; then
        printf 'bench: %s/%s produced no usable answer (%s errors) — no row written\n' \
          "$config" "$phase" "${errors:-0}" >&2
        continue
      fi
      [ "${errors:-0}" -eq 0 ] || printf 'bench: %s/%s had %s failed requests\n' \
        "$config" "$phase" "$errors" >&2

      printf '%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
        "$config" "$provider" "$placement" "$model" "$phase" "$num_ctx" "$kv_cache" \
        "$n" "$first_call" "$p50_ms" "$p95_ms" "$prompt_tok" "$completion_tok" \
        "$cached_tok" "$tps" "$valid_json_ratio" "$cjk_ratio" "$latin_ratio" \
        "$lang_pass_ratio" "${vram:-}" "$build" "$verdict" "$repeat" "$started_at" \
        >>"$csv"

      # The share of the prompt served from the cache of llama.cpp. The system
      # half of an agent prompt is stable, so a low share on the stand means the
      # slots were reset between calls — a reason for a slow p95 that has
      # nothing to do with the model (§6.3).
      cache_share=0
      [ "${prompt_tok:-0}" -gt 0 ] && cache_share=$(python3 -c \
        "print(round($cached_tok / $prompt_tok, 2))")
      print_row "$config" "$phase" "$n" "$p50_ms" "$p95_ms" "$tps" \
        "$valid_json_ratio" "$lang_pass_ratio" "$cache_share" "$verdict"
      rows_written=$((rows_written + 1))
      config_rows=$((config_rows + 1))
    done
  done

  # --- the per-configuration JSON report -------------------------------------
  # A report with no cells in it would look like a run that found nothing,
  # which is not the same thing as a run that never happened.
  if [ "$config_rows" -eq 0 ]; then
    printf 'bench: %s measured nothing — no report written\n' "$config" >&2
    continue
  fi
  safe_config=$(printf '%s' "$config" | tr '+' 'p' | tr -cd 'A-Za-z0-9._-')
  cells_csv="$tmpdir/cells-$safe_config.csv"
  head -n 1 "$csv" >"$cells_csv"
  grep "^$config," "$csv" >>"$cells_csv" || true
  report="$out_dir/bench-$safe_config-$stamp.json"
  py report "$report" "$requests_csv" "$cells_csv" \
    "config=$config" "provider=$provider" "placement=$placement" \
    "server_mode=${server_mode:-}" "url=$url" "probe_url=$base" "num_ctx=$num_ctx" \
    "kv_cache=$kv_cache" "llamacpp_build=$build" "vram_used_mb=${vram:-}" \
    "first_call_ms=$first_call" "n_per_cell=$cell_n" "repeats=$repeats" \
    "runs_required=${runs_required:-3}" "started_at=$started_at" \
    "matrix=${matrix#"$repo_root/"}" "prompts=${prompts#"$repo_root/"}" \
    "script=scripts/llm-bench.sh"
  printf 'bench: %s\n' "${report#"$repo_root/"}" >&2
done

if [ "$rows_written" -eq 0 ]; then
  die 'nothing was measured — every phase was skipped or failed; see the messages above'
fi

printf 'bench: %s (%s rows)\n' "${csv#"$repo_root/"}" "$rows_written" >&2
printf 'bench: this is ONE run. The matrix asks for %s runs per configuration at different moments (restart llama-server between them); the decision goes into ops/metrics/baseline.md\n' \
  "${runs_required:-3}" >&2
