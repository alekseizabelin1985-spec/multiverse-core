#Requires -Version 7
<#
.SYNOPSIS
  Native llama-server (llama.cpp) of the platform: start, stop, health.

.DESCRIPTION
  infrastructure.md v0.3 §6.1-§6.3, ADR-005 add. 2 p. 7. The LLM is NOT a
  compose service: it is a native Windows process with CUDA and D:\Models\
  paths, its life cycle is longer than the stack's, and keeping it out of
  docker-compose.yml is what makes the "every port on 127.0.0.1, no default
  passwords" invariant easy to hold. Containers reach it through
  host.docker.internal (extra_hosts on `core`).

  Every value comes from .env (MV_LLM_*, §4.2) — nothing about this machine is
  hard-coded here, and nothing here belongs in build/versions.env except the
  pin LLAMACPP_BUILD, which `health` compares against the running binary.

  Sampling parameters (--temp, --top-p, --top-k, --min-p) are deliberately NOT
  passed: the gateway sets them per request from the blueprint of the phase
  (ADR-005 add. 2 p. 1). Two sets of defaults in two places is how they drift.

.PARAMETER Action
  up (default) | down | health.

.PARAMETER Router
  Router mode: --models-dir without -m (§6.2.1). Needed by variants A and E+;
  the two models must fit in VRAM at the same time.

.PARAMETER WithUi
  Turn the web UI, --tools all and --ui-mcp-proxy back on for manual work.
  Loopback only, and the port is never published (SEC-15).

.EXAMPLE
  pwsh scripts/llm-server.ps1                 # make llm-up
  pwsh scripts/llm-server.ps1 -Router         # make llm-up ROUTER=1
  pwsh scripts/llm-server.ps1 -Action health  # make llm-health
  pwsh scripts/llm-server.ps1 -Action down    # make llm-down
#>
param(
  [ValidateSet('up', 'down', 'health')]
  [string]  $Action = 'up',
  [switch]  $Router,
  [string]  $Model = $env:MV_LLM_MODEL_FILE,
  [int]     $Ctx = [int]($env:MV_LLM_NUM_CTX ?? 8192),
  [int]     $Slots = [int]($env:MV_LLM_SLOTS ?? 1),
  [string]  $Reason = ($env:MV_LLM_REASONING ?? 'off'),
  [switch]  $WithUi
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$opsDir = Join-Path $repoRoot 'ops'
$pidFile = Join-Path $opsDir 'llm-server.pid'
$logFile = Join-Path $opsDir 'llm-server.log'

$llmHost = ($env:MV_LLM_HOST ?? '127.0.0.1')   # loopback ONLY (SEC-15)
$llmPort = [int]($env:MV_LLM_PORT ?? 1234)
$baseUrl = "http://${llmHost}:${llmPort}"

New-Item -ItemType Directory -Force -Path $opsDir | Out-Null

function Get-ServerProcess {
  # The PID file is the fast path; a stale file (machine rebooted, process
  # killed by hand) must not make `up` believe the server is running.
  if (-not (Test-Path $pidFile)) { return $null }
  $recorded = (Get-Content $pidFile -Raw).Trim()
  if (-not $recorded) { return $null }
  return Get-Process -Id ([int]$recorded) -ErrorAction SilentlyContinue
}

function Invoke-Health {
  # 503 while the model loads is a state, not a failure (§6.3), so the status
  # code is returned rather than thrown.
  try {
    $r = Invoke-WebRequest -Uri "$baseUrl/health" -Method Get -TimeoutSec 5 -SkipHttpErrorCheck
    return [int]$r.StatusCode
  } catch {
    return 0
  }
}

function Get-PinnedBuild {
  $versions = Join-Path $repoRoot 'build/versions.env'
  if (-not (Test-Path $versions)) { return $null }
  foreach ($line in Get-Content $versions) {
    if ($line -match '^\s*LLAMACPP_BUILD\s*=\s*(\S+)') { return $Matches[1] }
  }
  return $null
}

switch ($Action) {

  'down' {
    $proc = Get-ServerProcess
    if (-not $proc) {
      Write-Host 'llm: no server recorded in ops/llm-server.pid — nothing to stop'
      Remove-Item $pidFile -ErrorAction SilentlyContinue
      exit 0
    }
    Write-Host "llm: stopping llama-server (pid $($proc.Id))"
    Stop-Process -Id $proc.Id -Force
    Remove-Item $pidFile -ErrorAction SilentlyContinue
    Write-Host 'llm: stopped; models and volumes are untouched'
    exit 0
  }

  'health' {
    $code = Invoke-Health
    switch ($code) {
      200 { Write-Host "llm: $baseUrl/health = 200 ok" }
      503 { Write-Host "llm: $baseUrl/health = 503 loading (the model is still being read)" }
      0 { Write-Host "llm: $baseUrl/health unreachable — the process is not running (make llm-up)" }
      default { Write-Host "llm: $baseUrl/health = $code" }
    }

    if ($code -eq 200) {
      try {
        $models = Invoke-RestMethod -Uri "$baseUrl/v1/models" -TimeoutSec 5
        $names = ($models.data | ForEach-Object { $_.id }) -join ', '
        Write-Host "llm: models = $names"
      } catch {
        Write-Host "llm: /v1/models did not answer: $($_.Exception.Message)"
      }
    }

    $pinned = Get-PinnedBuild
    $bin = $env:MV_LLM_BIN
    if ($bin -and (Test-Path $bin)) {
      $reported = (& $bin --version 2>&1 | Out-String)
      if ($reported -match 'build[:\s]+(\d+)') {
        $actual = "b$($Matches[1])"
        if ($pinned -and $actual -ne $pinned) {
          # A warning, not an error (§6.2): the pin is advisory until the
          # golden set has been re-run against the new build (§9.3).
          Write-Host "llm: WARNING build $actual differs from the pin $pinned in build/versions.env (see infrastructure.md §9.5)"
        } else {
          Write-Host "llm: build $actual matches the pin"
        }
      }
    } elseif ($pinned) {
      Write-Host "llm: pinned build $pinned; MV_LLM_BIN is not set, cannot compare"
    }

    if (Get-Command nvidia-smi -ErrorAction SilentlyContinue) {
      $vram = (& nvidia-smi --query-gpu=memory.used,memory.total --format=csv,noheader) -join '; '
      Write-Host "llm: VRAM $vram"
    }

    if ($code -ne 200) { exit 1 }
    exit 0
  }

  'up' {
    $proc = Get-ServerProcess
    if ($proc -and (Invoke-Health) -ne 0) {
      Write-Host "llm: already running (pid $($proc.Id)) on $baseUrl — nothing to do"
      exit 0
    }
    if ($proc) {
      Write-Host "llm: pid $($proc.Id) is recorded but $baseUrl does not answer; restarting"
      Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }

    $bin = $env:MV_LLM_BIN
    if (-not $bin) { throw 'MV_LLM_BIN is not set (see .env.example, §4.2)' }
    if (-not (Test-Path $bin)) { throw "MV_LLM_BIN points at $bin, which does not exist" }

    $serverArgs = @(
      '--host', $llmHost
      '--port', $llmPort
      # llama.cpp splits --ctx-size between the slots, so the per-slot context
      # the platform asks for has to be multiplied back up.
      '--ctx-size', ($Ctx * $Slots)
      '--parallel', $Slots
      '--n-gpu-layers', ($env:MV_LLM_NGL ?? 99)
      '--threads', ($env:MV_LLM_THREADS ?? 32)
      '--batch-size', ($env:MV_LLM_BATCH_SIZE ?? 16000)
      '--flash-attn', 'on'
      '--kv-offload'
      '--kv-unified'
      # Mandatory: without it the Qwen3 chat template (tools, thinking) is not
      # applied and chat_template_kwargs is ignored.
      '--jinja'
      '--reasoning', $Reason
      '--load-mode', 'mmap+mlock'
      # The server serves a web UI on the same port by default; a service
      # runtime has no use for it.
      '--no-webui'
    )
    if ($env:MV_LLM_SLOT_SAVE_PATH) {
      $serverArgs += @('--slot-save-path', $env:MV_LLM_SLOT_SAVE_PATH)
    }
    if ($Router) {
      # -m and --models-dir are mutually exclusive: with -m the server is in
      # single-model mode and the `model` field of a request is ignored
      # (ADR-005 add. 2 p. 4).
      if (-not $env:MV_LLM_MODELS_DIR) { throw 'ROUTER mode needs MV_LLM_MODELS_DIR' }
      $serverArgs += @('--models-dir', $env:MV_LLM_MODELS_DIR, '--models-max', 2)
    } else {
      if (-not $Model) { throw 'MV_LLM_MODEL_FILE is not set (single mode needs -m)' }
      $serverArgs += @('-m', $Model)
    }
    if ($WithUi) {
      # Extra attack surface: the UI and the MCP proxy are reachable by any
      # process on this machine. Loopback only, port never published (SEC-15).
      $serverArgs = $serverArgs | Where-Object { $_ -ne '--no-webui' }
      $serverArgs += @('--tools', 'all', '--ui-mcp-proxy')
      Write-Host 'llm: WARNING web UI and MCP proxy are on (-WithUi); do not publish the port'
    }

    Write-Host "llm: starting $bin on $baseUrl ($(if ($Router) { 'router' } else { 'single' }) mode)"
    Remove-Item $logFile -ErrorAction SilentlyContinue
    $proc = Start-Process -FilePath $bin -ArgumentList $serverArgs -PassThru `
      -RedirectStandardOutput $logFile -RedirectStandardError "$logFile.err" -WindowStyle Hidden
    $proc.Id | Set-Content -Path $pidFile -Encoding ascii

    # 503 (loading) -> 200 (ok), up to 180 s: a 27B model on a cold page cache
    # takes tens of seconds even with mmap (§6.3).
    $deadline = (Get-Date).AddSeconds(180)
    $code = 0
    while ((Get-Date) -lt $deadline) {
      if ($proc.HasExited) {
        Remove-Item $pidFile -ErrorAction SilentlyContinue
        throw "llama-server exited with code $($proc.ExitCode); see $logFile"
      }
      $code = Invoke-Health
      if ($code -eq 200) { break }
      Start-Sleep -Seconds 2
    }
    if ($code -ne 200) {
      throw "llama-server did not become healthy in 180 s (last /health = $code); see $logFile"
    }
    Write-Host "llm: /health = 200 (pid $($proc.Id), log $logFile)"

    # One short call: the first request after a start allocates the compute
    # buffers and warms the CUDA kernels, so it is both the warm-up and a rough
    # "the server answers" indicator. It is measured separately in `make bench`
    # (B2) and never enters p50/p95.
    $body = @{
      model                = if ($Router) { $null } else { [System.IO.Path]::GetFileNameWithoutExtension($Model) }
      messages             = @(@{ role = 'user'; content = 'ok' })
      max_tokens           = 1
      chat_template_kwargs = @{ enable_thinking = $false }
    } | ConvertTo-Json -Depth 6
    try {
      $sw = [System.Diagnostics.Stopwatch]::StartNew()
      Invoke-RestMethod -Uri "$baseUrl/v1/chat/completions" -Method Post `
        -ContentType 'application/json' -Body $body -TimeoutSec 120 | Out-Null
      $sw.Stop()
      Write-Host "llm: first call $($sw.ElapsedMilliseconds) ms"
    } catch {
      Write-Host "llm: warm-up call failed ($($_.Exception.Message)); the server is up, check the blueprint model name"
    }
    exit 0
  }
}
