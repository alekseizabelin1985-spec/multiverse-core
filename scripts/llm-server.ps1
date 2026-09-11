#Requires -Version 7
<#
.SYNOPSIS
  LLM runtime of the platform: start, stop, health.

.DESCRIPTION
  infrastructure.md v0.3 §6.1-§6.3, ADR-005 add. 2 p. 2, p. 3, p. 7. The LLM is
  NOT a compose service: the default runtime is a native Windows process with
  CUDA and D:\Models\ paths, its life cycle is longer than the stack's, and
  keeping it out of docker-compose.yml is what makes the "every port on
  127.0.0.1, no default passwords" invariant easy to hold. Containers reach it
  through host.docker.internal (extra_hosts on `core`).

  THE ADDRESS HAS ONE SOURCE OF TRUTH, MV_LLM_URL, and — since T-404 review #1 —
  ONE RULE: scripts/lib/LlmEndpoint.psm1. This script, scripts/llm-bench.ps1 and
  their two bash twins all derive the address there, so that "what the probe
  knocks on", "what the bench measures" and "what llama-server binds" cannot
  drift apart again. Before that module existed the rule was copied into four
  files with three different behaviours, and on the owner's stand `make bench`
  called a living server dead.

  MV_LLM_HOST stays and is not a duplicate: it is the interface the local server
  listens on, loopback only (SEC-15) — a different question from where a client
  should knock. The port was the duplicate, and it is gone: a second declaration
  is what let the probe knock on 1234 while the server listened on 8888 (T-404).

  Not every provider is a process of this machine. openai_compat on a cloud
  address, anthropic, recorded and fake are not started or stopped from here,
  and up/down say so instead of pretending to have done something.

  Every value comes from .env (MV_LLM_*, §4.2) — nothing about this machine is
  hard-coded here, and nothing here belongs in build/versions.env except the
  pin LLAMACPP_BUILD, which `health` compares against the running binary when
  the runtime is the local llama.cpp.

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
  # The defaults of these four are NOT read here. `$env:X ?? 'default'` does not
  # react to an empty string, so MV_LLM_NUM_CTX= used to arrive as [int]'' = 0
  # and build `--ctx-size 0`, while the bash twin substituted 8192 (review Mi-9,
  # and the same root as C-1). They are filled from Get-LlmEnv below, which
  # treats blank as "not configured" in both implementations.
  [string]  $Model = '',
  [int]     $Ctx = 0,
  [int]     $Slots = 0,
  [string]  $Reason = '',
  [switch]  $WithUi
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$opsDir = Join-Path $repoRoot 'ops'
$pidFile = Join-Path $opsDir 'llm-server.pid'
$logFile = Join-Path $opsDir 'llm-server.log'

# The module is the ONLY source of the rule that reads MV_LLM_URL, so its
# absence is not a stack trace to decipher: it is one English line naming the
# file. Import-Module threw a localised error record instead — mojibake on a
# Cyrillic path — because the console settings that fix that live INSIDE the
# module, and on this one path the module is what is missing (review Mi-3).
$llmModule = Join-Path $PSScriptRoot 'lib/LlmEndpoint.psm1'
if (-not (Test-Path $llmModule)) {
  # The two console settings of the module, repeated here and only here: this is
  # the one path on which the module is not loaded, so without them the em dash
  # of the line below and a Cyrillic path both come out as mojibake.
  [Threading.Thread]::CurrentThread.CurrentUICulture = 'en-US'
  try { [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false) } catch { }
  [Console]::Error.WriteLine("llm: $llmModule is missing — it holds the only rule that turns MV_LLM_URL into an address, and no script of this project can read the variable without it")
  exit 1
}
Import-Module $llmModule -Force

New-Item -ItemType Directory -Force -Path $opsDir | Out-Null

if (-not $PSBoundParameters.ContainsKey('Model')) { $Model = Get-LlmEnv 'MV_LLM_MODEL_FILE' }
if (-not $PSBoundParameters.ContainsKey('Ctx')) { $Ctx = [int](Get-LlmEnv 'MV_LLM_NUM_CTX' '8192') }
if (-not $PSBoundParameters.ContainsKey('Slots')) { $Slots = [int](Get-LlmEnv 'MV_LLM_SLOTS' '1') }
if (-not $PSBoundParameters.ContainsKey('Reason')) { $Reason = Get-LlmEnv 'MV_LLM_REASONING' 'off' }

# --- the address ------------------------------------------------------------
#
# One call, one rule. .Url is the address as configured (what is printed, and
# what a remote endpoint is probed at), .Probe is the same address as reachable
# from this machine, .Port is what llama-server binds.

$llmHost = Get-LlmEnv 'MV_LLM_HOST' '127.0.0.1'   # what the local server binds, loopback ONLY (SEC-15)
$ep = Resolve-LlmEndpoint

function Write-NotStartable {
  switch ($ep.Provider) {
    { $_ -in 'recorded', 'fake' } {
      Write-LlmLine "llm: MV_LLM_PROVIDER=$($ep.Provider) answers inside the process — no endpoint to probe, nothing to start or stop (ADR-005 add. 2 p. 2)"
    }
    'anthropic' {
      Write-LlmLine 'llm: MV_LLM_PROVIDER=anthropic is the vendor cloud API — not started from here, and this wrapper has no address to probe; core reports it as deps.llm in /health'
    }
    'ollama' {
      Write-LlmLine 'llm: MV_LLM_PROVIDER=ollama is a runtime of its own — start it with `ollama serve` or the compose profile gpu, not from here; models are pulled by make models'
    }
    default {
      Write-LlmLine "llm: $($ep.Var)=$($ep.Raw) is not an address of this machine — provider $($ep.Provider) is not started or stopped from here"
    }
  }
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
# Everything in this block exists in scripts/llm-server.sh as well, message for
# message: this file has $proc.Path where the twin has /proc/<pid>/exe.

$script:PidRecorded = 0
$script:PidRecordedBin = ''
$script:PidRecordedStarted = ''
$script:PidStranger = $false      # the recorded number is provably another process
$script:PidUnverifiable = $false  # the recorded number cannot be judged either way
$script:PidOwnerReason = ''

function Read-PidFile {
  # A pid file with junk in it is a recovery path, not a place to fall over
  # (review Mi-8): anything that is not a number is treated as no pid at all.
  $script:PidRecorded = 0
  $script:PidRecordedBin = ''
  $script:PidRecordedStarted = ''
  if (-not (Test-Path $pidFile)) { return $false }
  $lines = @(Get-Content $pidFile -ErrorAction SilentlyContinue)
  if ($lines.Count -eq 0) {
    Write-LlmLine 'llm: ops/llm-server.pid does not contain a process id — ignoring it'
    return $false
  }
  $id = 0
  if (-not [int]::TryParse($lines[0].Trim(), [ref]$id)) {
    Write-LlmLine 'llm: ops/llm-server.pid does not contain a process id — ignoring it'
    return $false
  }
  $script:PidRecorded = $id
  # A file written before T-404 review #2 holds the number and nothing else, and
  # $lines[1..0] on such a file does not yield an empty range in PowerShell — it
  # counts DOWN and reads past the end. A pid file is a recovery path; it must
  # not throw.
  if ($lines.Count -lt 2) { return $true }
  foreach ($line in $lines[1..($lines.Count - 1)]) {
    if ($null -eq $line) { continue }
    if ($line.StartsWith('bin=', [StringComparison]::Ordinal)) {
      $script:PidRecordedBin = $line.Substring(4).Trim()
    } elseif ($line.StartsWith('started=', [StringComparison]::Ordinal)) {
      $script:PidRecordedStarted = $line.Substring(8).Trim()
    }
  }
  return $true
}

function Get-PathKey {
  # Separators one way, the .exe suffix off (the twin reads /proc/<pid>/exe,
  # which drops it) and the case folded where the file system folds it.
  param([string] $Path = '')
  $key = $Path.Replace('\', '/')
  if ($IsWindows) {
    $key = $key.ToLowerInvariant()
    if ($key.EndsWith('.exe', [StringComparison]::Ordinal)) {
      $key = $key.Substring(0, $key.Length - 4)
    }
  }
  return $key
}

function Get-ProcessImage {
  # "What program is that number running". $proc.Path is the answer; it throws
  # or returns nothing for a process of another user or another subsystem, and
  # that case is "cannot tell", not "not ours".
  param([Parameter(Mandatory)] $Process)
  try {
    if ($Process.Path) { return [string]$Process.Path }
  } catch {
    return ''
  }
  return ''
}

function Get-ProcessStarted {
  # The start time in the shape `up` records, or nothing when the platform will
  # not say.
  param([Parameter(Mandatory)] $Process)
  try {
    return $Process.StartTime.ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
  } catch {
    return ''
  }
}

function Test-TimesAgree {
  # Two seconds: the two platforms round the start time differently, and a
  # reused process number differs by minutes, not by ticks.
  param([string] $Recorded = '', [string] $Actual = '')
  try {
    $a = [datetimeoffset]::Parse($Recorded, [cultureinfo]::InvariantCulture).ToUnixTimeSeconds()
    $b = [datetimeoffset]::Parse($Actual, [cultureinfo]::InvariantCulture).ToUnixTimeSeconds()
  } catch {
    return $true
  }
  return ([math]::Abs($a - $b) -le 2)
}

function Test-PidIsOurs {
  # The check the incident of review #2 was missing.
  param([Parameter(Mandatory)] $Process)
  $script:PidOwnerReason = ''
  $processId = $Process.Id
  $expected = $script:PidRecordedBin
  if (-not $expected) { $expected = Get-LlmEnv 'MV_LLM_BIN' }
  if (-not $expected) {
    $script:PidOwnerReason = "llm: ops/llm-server.pid records pid $processId and nothing to compare it against (the file carries no program path and MV_LLM_BIN is not set) — refusing to stop a process this run cannot recognise; check it by hand and delete ops/llm-server.pid"
    return $false
  }
  $actual = Get-ProcessImage $Process
  if (-not $actual) {
    $script:PidOwnerReason = "llm: ops/llm-server.pid records pid $processId, but this run cannot read the program of that process — refusing to stop a process this run cannot recognise; check it by hand and delete ops/llm-server.pid"
    return $false
  }
  # Equal path, or — when the two sides name the same file differently — equal
  # file name. That happens routinely and is not laxity: /proc/<pid>/exe under
  # Git Bash answers /tmp/... where MV_LLM_BIN says C:\..., and Windows hands
  # out short 8.3 names for a path a human wrote in full. What the check has to
  # separate is llama-server from python.exe, and a file name does that; the
  # start time below separates two runs of the same program.
  $actualKey = Get-PathKey $actual
  $expectedKey = Get-PathKey $expected
  if ($actualKey -cne $expectedKey -and
    $actualKey.Substring($actualKey.LastIndexOf('/') + 1) -cne $expectedKey.Substring($expectedKey.LastIndexOf('/') + 1)) {
    $script:PidOwnerReason = "llm: ops/llm-server.pid records pid $processId, which runs $actual and not $expected — the number belongs to another process now; nothing is stopped and the file is removed"
    return $false
  }
  if ($script:PidRecordedStarted) {
    $actualStarted = Get-ProcessStarted $Process
    if ($actualStarted -and -not (Test-TimesAgree $script:PidRecordedStarted $actualStarted)) {
      $script:PidOwnerReason = "llm: ops/llm-server.pid records pid $processId started at $($script:PidRecordedStarted), but that process started at $actualStarted — the number has been reused; nothing is stopped and the file is removed"
      return $false
    }
  }
  return $true
}

function Get-ServerProcess {
  # A live server OF OURS, or $null with the reason said out loud. A stale file
  # (machine rebooted, process killed by hand) must not make `up` believe the
  # server is running, and a damaged one must not make `down` throw: stopping a
  # server is a recovery path.
  $script:PidStranger = $false
  $script:PidUnverifiable = $false
  if (-not (Read-PidFile)) { return $null }
  $proc = Get-Process -Id $script:PidRecorded -ErrorAction SilentlyContinue
  if (-not $proc) { return $null }
  if (-not (Test-PidIsOurs $proc)) {
    Write-LlmLine $script:PidOwnerReason
    if ($script:PidOwnerReason.Contains('the number belongs to another process now', [StringComparison]::Ordinal) -or
      $script:PidOwnerReason.Contains('the number has been reused', [StringComparison]::Ordinal)) {
      $script:PidStranger = $true
    } else {
      $script:PidUnverifiable = $true
    }
    return $null
  }
  return $proc
}

function Stop-ServerIfOurs {
  # Stops the recorded process when it is ours, and says why it did not
  # otherwise. $Lead picks which of the two lead lines to print.
  param([ValidateSet('still-recorded', 'stopping')][string] $Lead = 'stopping')
  $proc = Get-ServerProcess
  if ($proc) {
    if ($Lead -eq 'still-recorded') {
      Write-LlmLine "llm: ops/llm-server.pid still records a live process (pid $($proc.Id)) started here — stopping it"
    } else {
      Write-LlmLine "llm: stopping llama-server (pid $($proc.Id))"
    }
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    Remove-Item $pidFile -ErrorAction SilentlyContinue
    return $true
  }
  # A stranger's number is removed from the file; a number this run could not
  # judge is left alone, because the message above tells the operator to look.
  if ($script:PidStranger) { Remove-Item $pidFile -ErrorAction SilentlyContinue }
  return $false
}

function Get-PinnedBuild {
  $versions = Join-Path $repoRoot 'build/versions.env'
  if (-not (Test-Path $versions)) { return '' }
  foreach ($line in Get-Content $versions) {
    if ($line -match '^\s*LLAMACPP_BUILD\s*=\s*(\S+)') { return $Matches[1] }
  }
  return ''
}

function Test-CloudGateShut {
  # The gate is the address, not the name of the provider (ADR-005 add. 2 p. 3):
  # on an address outside the trusted network the provider refuses to start
  # unless the operator opened the gate.
  if (-not $ep.Has) { return $false }
  if ($ep.Trusted) { return $false }
  return (Get-LlmEnv 'MV_LLM_CLOUD_ENABLED' 'false') -ne 'true'
}

function Show-Health {
  # The question this answers is "can the platform use the LLM", not "does
  # something answer at that address" — which is why a shut cloud gate is a
  # failure even when the endpoint says 200 (orchestrator decision on review
  # question 3: the same false green as C-1, by another route).
  if ($ep.Error) {
    Write-LlmFail $ep.Error
    return 1
  }
  if (-not $ep.Has) {
    Write-NotStartable
    return 0
  }

  Write-LlmRetiredPortNote $ep.Var
  Write-LlmLine "llm: probing $($ep.Probe) (from $($ep.Var)=$($ep.Raw))"
  if ($ep.TrimmedV1) {
    Write-LlmLine "llm: $($ep.Var) ends with /v1; the variable holds the base address, so the probe uses $($ep.Url) (T-404)"
  }

  $health = Invoke-LlmHealthProbe $ep.Probe
  switch ($health.Code) {
    200 {
      if ($health.Via -eq '/health') {
        Write-LlmLine "llm: $($ep.Probe)/health = 200 ok"
      } else {
        Write-LlmLine "llm: $($ep.Probe)$($health.Via) = 200 ok (this server has no /health; judged by the endpoint the contract needs)"
      }
    }
    503 { Write-LlmLine "llm: $($ep.Probe)/health = 503 loading (the model is still being read)" }
    0 {
      if ($ep.Startable -and (Get-LlmEnv 'MV_LLM_BIN')) {
        Write-LlmLine "llm: $($ep.Probe) unreachable — the process is not running (make llm-up)"
      } elseif ($ep.Startable) {
        # `make llm-up` here would stop at "MV_LLM_BIN is not set", and the very
        # next line of this report says so: two neighbouring lines contradicting
        # each other is worse than no advice (review Mi-1).
        Write-LlmLine "llm: $($ep.Probe) unreachable — the process is not running; start it the way it was started before, because make llm-up needs MV_LLM_BIN (.env.example, §4.2)"
      } else {
        Write-LlmLine "llm: $($ep.Probe) unreachable — the endpoint does not answer from this machine"
      }
    }
    { $_ -in 401, 403 } {
      $key = if (Get-LlmEnv 'MV_LLM_API_KEY') { 'set and rejected' } else { 'empty' }
      Write-LlmLine "llm: $($ep.Probe)$($health.Via) = $($health.Code) — the endpoint needs a key; MV_LLM_API_KEY is $key"
    }
    default { Write-LlmLine "llm: $($ep.Probe)$($health.Via) = $($health.Code)" }
  }
  if ($health.Fault) {
    Write-LlmLine 'llm: the request itself failed — this is not "the process is not running"; check the address, the key and TLS'
  }

  $modelsPrinted = $false
  if ($health.Code -eq 200) {
    $models = Get-LlmModels $ep.Probe
    if (-not $models) { $models = '<none reported>' }
    Write-LlmLine "llm: models = $models"
    $modelsPrinted = $true
  }

  # The build pin and the VRAM report describe a local llama.cpp process. For
  # any other runtime they answer a question nobody asked (T-404); the way to
  # pin the version of a remote runtime is the model of /v1/models, above.
  if ($ep.Startable) {
    $pinned = Get-PinnedBuild
    $bin = Get-LlmEnv 'MV_LLM_BIN'
    if ($bin -and (Test-Path $bin)) {
      $reported = (& $bin --version 2>&1 | Out-String)
      if ($reported -match 'build[:\s]+(\d+)') {
        $actual = "b$($Matches[1])"
        if ($pinned -and $actual -ne $pinned) {
          # A warning, not an error (§6.2): the pin only becomes true after the
          # golden set has been re-run against the new build (§9.3).
          Write-LlmLine "llm: WARNING build $actual differs from the pin $pinned in build/versions.env (infrastructure.md §9.5)"
        } else {
          Write-LlmLine "llm: build $actual matches the pin"
        }
      }
    } elseif ($pinned) {
      # Card p. 2: comparing LLAMACPP_BUILD against a binary is meaningless for a
      # runtime that is not llama.cpp, and on this stand that is exactly the case
      # (the address is local, the server was started by hand, MV_LLM_BIN is
      # unset). Saying "cannot compare" invited the operator to fix the wrong
      # thing; the version of a runtime we did not start is the model list above.
      # "Above" only when there IS a list above: at an endpoint that does not
      # answer, this sentence used to point at nothing (review Mi-2).
      if ($modelsPrinted) {
        Write-LlmLine "llm: MV_LLM_BIN is not set — this runtime was not started from here, so the pin $pinned in build/versions.env does not describe it; the version of any other runtime is the model list above (T-404)"
      } else {
        Write-LlmLine "llm: MV_LLM_BIN is not set — this runtime was not started from here, so the pin $pinned in build/versions.env does not describe it; the version of any other runtime is its GET /v1/models, which this run could not read (T-404)"
      }
    }

    if (Get-Command nvidia-smi -ErrorAction SilentlyContinue) {
      $vram = (& nvidia-smi --query-gpu=memory.used,memory.total --format=csv,noheader) -join '; '
      Write-LlmLine "llm: VRAM $vram"
    }
  }

  if (Test-CloudGateShut) {
    Write-LlmLine "llm: WARNING $($ep.Var) points outside this machine and MV_LLM_CLOUD_ENABLED is not true — the provider will refuse this address (ADR-005 add. 2 p. 3)"
    if ($health.Code -eq 200) {
      Write-LlmLine 'llm: the endpoint answers, but the platform will not call it while the gate is shut — reported as a failure, not as a green LLM'
    }
    return 1
  }

  if ($health.Code -ne 200) { return 1 }
  return 0
}

switch ($Action) {

  'down' {
    if ($ep.Error) {
      Write-LlmFail $ep.Error
      $null = Stop-ServerIfOurs -Lead 'still-recorded'
      exit 1
    }
    if (-not $ep.Startable) {
      Write-NotStartable
      # Read after the explanation, not before it: the twin says the same two
      # things in the same order, damaged-pid-file notice included. The
      # configuration moved on, the process did not: if it was started by this
      # script it is still ours to stop.
      $null = Stop-ServerIfOurs -Lead 'still-recorded'
      exit 0
    }
    if (Stop-ServerIfOurs -Lead 'stopping') {
      Write-LlmLine 'llm: stopped; models and volumes are untouched'
      exit 0
    }
    if (-not $script:PidUnverifiable -and -not $script:PidStranger) {
      Write-LlmLine 'llm: no live server in ops/llm-server.pid — nothing to stop'
      Remove-Item $pidFile -ErrorAction SilentlyContinue
    }
    exit 0
  }

  'health' {
    exit (Show-Health)
  }

  'up' {
    if ($ep.Error) {
      Write-LlmFail $ep.Error
      exit 1
    }
    if (-not $ep.Startable) {
      Write-NotStartable
      if (-not $ep.Has) { exit 0 }
      # Nothing to start, but the operator still asked "is the LLM usable?", and
      # for a remote endpoint that is the only answer this script can give.
      Write-LlmLine 'llm: checking whether it answers instead'
      exit (Show-Health)
    }

    # SEC-15 is fail-closed. Warning about a non-loopback interface and then
    # binding it anyway is not a control, it is the imitation of one (review
    # Mi-12, orchestrator decision): a published llama-server has no
    # authentication in front of it, and the operator who set the variable will
    # never see the warning in a scripted run.
    if ($llmHost -notin '127.0.0.1', 'localhost', '::1', '[::1]') {
      Write-LlmFail "llm: MV_LLM_HOST=$llmHost is not loopback; llama-server would be reachable beyond this machine and it has no authentication (SEC-15, infrastructure.md §6.3). Refusing to start — set MV_LLM_HOST=127.0.0.1, and publish the port through a reverse proxy if it really has to leave the host"
      exit 1
    }

    Write-LlmRetiredPortNote $ep.Var
    $health = Invoke-LlmHealthProbe $ep.Probe
    $proc = Get-ServerProcess
    if ($proc) {
      if ($health.Code -ne 0) {
        Write-LlmLine "llm: already running (pid $($proc.Id)) on $($ep.Probe) — nothing to do"
        exit 0
      }
      # Only reached for a process that has just been proved ours: before review
      # #2 this line force-stopped whatever the number had become (M-5).
      Write-LlmLine "llm: pid $($proc.Id) is recorded but $($ep.Probe) does not answer; restarting"
      Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
      Remove-Item $pidFile -ErrorAction SilentlyContinue
    } elseif ($script:PidUnverifiable) {
      Write-LlmFail 'llm: ops/llm-server.pid names a process this run cannot recognise (see the line above) — refusing to start a second server while that is unresolved'
      exit 1
    } elseif ($health.Code -ne 0) {
      # Somebody already holds the address and it is not a process of ours: on
      # the owner's stand llama-server is started by hand and ops/llm-server.pid
      # does not exist, so `make llm-up` used to start a SECOND server on a taken
      # port and then report the stranger's 200 as its own success (review M-6).
      Write-LlmFail "llm: $($ep.Probe)$($health.Via) already answers $($health.Code) and ops/llm-server.pid records no process of ours — refusing to start a second server on the same address. It is already usable (make llm-health); stop it by hand if you need to replace it"
      exit 1
    }

    $bin = Get-LlmEnv 'MV_LLM_BIN'
    if (-not $bin) {
      Write-LlmFail 'llm: MV_LLM_BIN is not set (see .env.example, §4.2)'
      exit 1
    }
    if (-not (Test-Path $bin)) {
      Write-LlmFail "llm: MV_LLM_BIN points at $bin, which is not executable"
      exit 1
    }

    # The order of these arguments is the order of llm-server.sh, to the letter:
    # two stands whose logs cannot be diffed are two stands nobody compares (N-1).
    $serverArgs = @(
      '--host', $llmHost
      # The port is the one the platform will call: it is read out of
      # $ep.Var, never declared a second time (T-404).
      '--port', $ep.Port
      # llama.cpp splits --ctx-size between the slots, so the per-slot context
      # the platform asks for has to be multiplied back up.
      '--ctx-size', ($Ctx * $Slots)
      '--parallel', $Slots
      '--n-gpu-layers', (Get-LlmEnv 'MV_LLM_NGL' '99')
      '--threads', (Get-LlmEnv 'MV_LLM_THREADS' '32')
      '--batch-size', (Get-LlmEnv 'MV_LLM_BATCH_SIZE' '16000')
      '--flash-attn', 'on'
      '--kv-offload'
      '--kv-unified'
      # Mandatory: without it the Qwen3 chat template (tools, thinking) is not
      # applied and chat_template_kwargs is ignored.
      '--jinja'
      '--reasoning', $Reason
      '--load-mode', 'mmap+mlock'
    )
    $slotSavePath = Get-LlmEnv 'MV_LLM_SLOT_SAVE_PATH'
    if ($slotSavePath) { $serverArgs += @('--slot-save-path', $slotSavePath) }

    if ($Router) {
      # -m and --models-dir are mutually exclusive: with -m the server is in
      # single-model mode and the `model` field of a request is ignored
      # (ADR-005 add. 2 p. 4).
      $modelsDir = Get-LlmEnv 'MV_LLM_MODELS_DIR'
      if (-not $modelsDir) {
        Write-LlmFail 'llm: ROUTER mode needs MV_LLM_MODELS_DIR'
        exit 1
      }
      $serverArgs += @('--models-dir', $modelsDir, '--models-max', 2)
    } else {
      if (-not $Model) {
        Write-LlmFail 'llm: MV_LLM_MODEL_FILE is not set (single mode needs -m)'
        exit 1
      }
      $serverArgs += @('-m', $Model)
    }

    if ($WithUi) {
      # Extra attack surface: the UI and the MCP proxy are reachable by any
      # process on this machine. Loopback only, port never published (SEC-15).
      $serverArgs += @('--tools', 'all', '--ui-mcp-proxy')
      Write-LlmLine 'llm: WARNING web UI and MCP proxy are on (--with-ui); do not publish the port'
    } else {
      # The server serves a web UI on the same port by default; a service
      # runtime has no use for it.
      $serverArgs += '--no-webui'
    }

    $mode = if ($Router) { 'router' } else { 'single' }
    Write-LlmLine "llm: starting $bin on ${llmHost}:$($ep.Port) ($mode mode); the platform calls it at $($ep.Url)"
    Remove-Item $logFile -ErrorAction SilentlyContinue
    $proc = Start-Process -FilePath $bin -ArgumentList $serverArgs -PassThru `
      -RedirectStandardOutput $logFile -RedirectStandardError "$logFile.err" -WindowStyle Hidden
    # Three lines, not one: the number alone is not evidence that the process is
    # ours when it is read back (review M-5). The resolved path is recorded, so
    # the check survives a later change of MV_LLM_BIN.
    $recordedBin = Get-ProcessImage $proc
    if (-not $recordedBin) { $recordedBin = $bin }
    @("$($proc.Id)", "bin=$recordedBin", "started=$(Get-ProcessStarted $proc)") |
      Set-Content -Path $pidFile -Encoding ascii

    # 503 (loading) -> 200 (ok), up to 180 s: a 27B model on a cold page cache
    # takes tens of seconds even with mmap (§6.3).
    $deadline = (Get-Date).AddSeconds(180)
    $health = [ordered]@{ Code = 0; Via = '/health'; Fault = $false }
    while ((Get-Date) -lt $deadline) {
      if ($proc.HasExited) {
        Remove-Item $pidFile -ErrorAction SilentlyContinue
        Write-LlmFail "llm: llama-server exited during start; see $logFile"
        exit 1
      }
      $health = Invoke-LlmHealthProbe $ep.Probe
      if ($health.Code -eq 200) { break }
      Start-Sleep -Seconds 2
    }
    if ($health.Code -ne 200) {
      Write-LlmFail "llm: did not become healthy in 180 s (last $($health.Via) = $($health.Code)); see $logFile"
      exit 1
    }
    Write-LlmLine "llm: $($ep.Probe)$($health.Via) = 200 (pid $($proc.Id), log $logFile)"

    # One short call: the first request after a start allocates the compute
    # buffers and warms the CUDA kernels, so it is both the warm-up and a rough
    # "the server answers" indicator. `make bench` measures it separately (B2)
    # and it never enters p50/p95.
    $modelName = ''
    if (-not $Router) { $modelName = [System.IO.Path]::GetFileNameWithoutExtension($Model) }
    $body = @{
      model                = $modelName
      messages             = @(@{ role = 'user'; content = 'ok' })
      max_tokens           = 1
      chat_template_kwargs = @{ enable_thinking = $false }
    } | ConvertTo-Json -Depth 6
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    # The status code is checked, and the exception text is NOT printed: .NET
    # puts the whole header value into it, key included (review Mi-4, Mi-7).
    $warmCode = '000'
    try {
      $response = Invoke-WebRequest -Uri "$($ep.Probe)/v1/chat/completions" -Method Post `
        -Headers (Get-LlmHeaders) -ContentType 'application/json' -Body $body -TimeoutSec 120 `
        -SkipHttpErrorCheck -NoProxy -SkipHeaderValidation
      $warmCode = [string][int]$response.StatusCode
    } catch {
      $warmCode = '000'
    }
    $stopwatch.Stop()
    if ($warmCode -eq '200') {
      Write-LlmLine "llm: first call $($stopwatch.ElapsedMilliseconds) ms"
    } else {
      Write-LlmLine "llm: warm-up call answered $warmCode; the server is up, check the blueprint model name"
    }
    exit 0
  }
}
