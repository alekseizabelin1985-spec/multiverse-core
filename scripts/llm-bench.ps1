#Requires -Version 7
<#
.SYNOPSIS
  The LLM measurement matrix (F-8): runs testdata/bench/prompts.jsonl against a
  running llama-server and writes the numbers the E / C / A decision rests on.

.DESCRIPTION
  infrastructure.md v0.3 §6.4, overview.md §18.1, ADR-005 add. 2 p. 5,
  nfr.md «Что измерить первым», metrics.md §7 B1-B5.

  Twin of scripts/llm-bench.sh. Both write the SAME CSV columns — a divergence
  is a review finding, not a detail. On the owner's machine PowerShell 7 is not
  installed yet (journal 2026-09-10, ОВ-43), so the .sh is the one that runs
  today; this file is here for a machine that has pwsh, and for the `make bench`
  branch that prefers it.

  Outputs:
    ops/metrics/bench-<date>.csv           — the columns of §6.4, one row per cell
    ops/metrics/bench-<config>-<date>.json — the same numbers plus every request
    a table on stdout                      — what the operator reads on the stand

  What this script deliberately does NOT do: it never starts or reconfigures
  llama-server (--ctx-size, the KV cache type and the model belong to the
  running process — §6.4 rule 1; start it with `make llm-up`), and it does not
  decide anything (the verdict of one run is not a decision; three runs and
  ops/metrics/baseline.md are).

.PARAMETER Configs
  Which configurations of the matrix to measure. Default E — the primary
  candidate; C and A are only run when E fails (decision_order).

.PARAMETER NumCtx
  The per-slot context the server was STARTED with. Recorded, not applied.

.PARAMETER KvCache
  The KV cache type the server was STARTED with (f16 | q8_0). Recorded.

.PARAMETER Repeats
  Passes over the whole prompt set inside one invocation. The matrix asks for
  runs_required runs at different moments; -Repeats is for the case when the
  operator wants several passes without leaving the machine.

.PARAMETER Limit
  Cap on prompts per phase; 0 means n_per_cell from the matrix.

.EXAMPLE
  pwsh scripts/llm-bench.ps1 -Configs E
  pwsh scripts/llm-bench.ps1 -Configs E -NumCtx 16384 -KvCache q8_0
  pwsh scripts/llm-bench.ps1 -Configs C,A -Placement native

.NOTES
  Exit codes: 0 — measured; 2 — cannot measure (bad arguments, missing files,
  no server on the URL, no phase left to run). A failing verdict is a result,
  not an error, and exits 0.
#>
param(
  [string[]] $Configs = @('E'),
  [string]   $Matrix = 'ops/metrics/bench-matrix.json',
  [string]   $Prompts = 'testdata/bench/prompts.jsonl',
  [string]   $OutDir = 'ops/metrics',
  # Filled below from Get-LlmEnv: `??` does not react to an empty string, so
  # MV_LLM_NUM_CTX= used to arrive here as 0 (review Mi-9).
  [int]      $NumCtx = 0,
  [ValidateSet('f16', 'q8_0')]
  [string]   $KvCache = 'f16',
  [string]   $Placement = 'native',
  [int]      $Repeats = 1,
  [int]      $Limit = 0
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Numbers are rendered by the INVARIANT culture, not by the culture of the
# machine. On a ru-RU Windows [double]66.7 renders as "66,7", and this script
# joins the columns of the CSV with a comma: tps = 66,7 made the row 25 columns
# wide, so verdict, repeat and started_at were read one column to the left of
# where they are, while the bash twin wrote 66.7 on the same input (found by the
# parity run of T-434; the owner's machine is ru-RU and `make bench` prefers
# pwsh when it is in PATH).
#
# This line is the SECOND of two defences and on its own it changes nothing
# today: every number that reaches the CSV or the table goes through
# Format-BenchNumber, which names the invariant culture itself (a mutant that
# deletes this line leaves both outputs correct, while a mutant that deletes the
# formatting breaks them). It is kept for the number that will one day be
# printed without that helper. The UI culture is set in
# scripts/lib/LlmEndpoint.psm1 for another reason — English messages.
[Threading.Thread]::CurrentThread.CurrentCulture = [Globalization.CultureInfo]::InvariantCulture

$repoRoot = Split-Path -Parent $PSScriptRoot

# The address of the endpoint is derived in one place for the whole project -
# the same module scripts/llm-server.ps1 uses. Before T-404 review #1 this
# script had its own rule (it trimmed a trailing /v1, it did NOT rewrite
# host.docker.internal and it did NOT fall back from /health to /v1/models), and
# on the owner's stand that made `make bench` refuse to run against a server
# that was answering (review M-1).
#
# Its absence is reported as one English line naming the file, not as a
# localised .NET error record — the console settings that would have made that
# record readable live INSIDE the module (review Mi-3).
$llmModule = Join-Path $PSScriptRoot 'lib/LlmEndpoint.psm1'
if (-not (Test-Path $llmModule)) {
  # The two console settings of the module, repeated here and only here: this is
  # the one path on which the module is not loaded, so without them the em dash
  # of the line below and a Cyrillic path both come out as mojibake.
  [Threading.Thread]::CurrentThread.CurrentUICulture = 'en-US'
  try { [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false) } catch { }
  [Console]::Error.WriteLine("bench: $llmModule is missing — it holds the only rule that turns MV_LLM_URL into an address, and no script of this project can read the variable without it")
  exit 2
}
Import-Module $llmModule -Force

function Resolve-RepoPath([string]$Path) {
  if ([IO.Path]::IsPathRooted($Path)) { return $Path }
  return (Join-Path $repoRoot $Path)
}

function Get-ReportPath([string]$Path) {
  <#
    How a file the report names is written into it (T-434), the same rule as
    `locate` of the bash twin: inside the repository — the path relative to its
    root, as before; outside it — the file NAME only, because the absolute path
    of a scratch copy is a path of the user's profile, and T-402 put exactly
    that into two reports meant for the repository (review #1, C-1). The sha256
    of the content goes next to it either way, which is also what makes a copy
    of the matrix checkable.
  #>
  $full = [IO.Path]::GetFullPath($Path)
  $shown = Get-RepoRelativePath $full
  if ($null -eq $shown) { $shown = [IO.Path]::GetFileName($full) }
  return [ordered]@{
    Path   = $shown
    Sha256 = (Get-FileHash -LiteralPath $full -Algorithm SHA256).Hash.ToLowerInvariant()
  }
}

function Get-RepoRelativePath([string]$Path) {
  <#
    The ONE rule of this script for "is this file inside the repository": the
    path relative to the root with forward slashes when it is, $null when it is
    not. Both the report (Get-ReportPath) and the console (Get-ShownPath) ask it,
    so the two cannot answer the same question differently (T-405 review #1,
    N-3). The path is made absolute first, and the comparison is ordinal and
    case-insensitive only where the file system is: every other comparison of
    this project is ordinal for the reason the module header of
    scripts/lib/LlmEndpoint.psm1 gives. The bash twin prints ${path#"$repo_root/"}
    without resolving `..` in the argument, so for an -OutDir written with `..`
    this script prints the resolved relative path and bash the path as given —
    the one shape in which the console lines of the two may still differ.
  #>
  $full = [IO.Path]::GetFullPath($Path)
  $root = [IO.Path]::GetFullPath($repoRoot).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar) +
          [IO.Path]::DirectorySeparatorChar
  $comparison = if ($IsWindows) { [StringComparison]::OrdinalIgnoreCase } else { [StringComparison]::Ordinal }
  if ($full.StartsWith($root, $comparison)) {
    return [IO.Path]::GetRelativePath($root, $full).Replace('\', '/')
  }
  return $null
}

function Get-ShownPath([string]$Path) {
  <#
    A file this run wrote, the way the bash twin prints it: ${path#"$repo_root/"}
    — relative to the repository when the file is inside it, as it is otherwise.
    This script printed the absolute path, so `make bench` said
    `bench: C:\Users\...\ops\metrics\bench-E-....json` here and
    `bench: ops/metrics/bench-E-....json` in bash (found by the parity stand,
    T-405).
  #>
  $relative = Get-RepoRelativePath $Path
  if ($null -ne $relative) { return $relative }
  return $Path
}

# Assigned before the first Stop-Bench can run: the CSV is created further down,
# and Set-StrictMode refuses to read a variable that was never set.
$csvPath = ''

function Remove-HeaderOnlyCsv {
  # A header-only CSV is litter in a directory that is read as results. The bash
  # twin removes it in its EXIT trap, on every way out; this script removed it
  # only when the run reached the end, so a configuration the matrix does not
  # have, or a server that does not answer, left one behind (found by the parity
  # stand, T-405). It is called from Stop-Bench and from the finally around the
  # measurement, which also runs on an unhandled error and on Ctrl+C (review #1,
  # N-2).
  if ($script:csvPath -and (Test-Path -LiteralPath $script:csvPath) -and
    @([IO.File]::ReadAllLines($script:csvPath)).Count -le 1) {
    Remove-Item -LiteralPath $script:csvPath -ErrorAction SilentlyContinue
  }
}

function Stop-Bench([string]$Message) {
  # Everything that makes a measurement impossible ends here, with exit 2: the
  # caller (make bench, or the operator) must be able to tell "did not run"
  # from "ran and failed the thresholds", which is exit 0 with verdict=fail.
  Write-Host "bench: $Message" -ForegroundColor Red
  Remove-HeaderOnlyCsv
  exit 2
}

if (-not $PSBoundParameters.ContainsKey('NumCtx')) { $NumCtx = [int](Get-LlmEnv 'MV_LLM_NUM_CTX' '8192') }

$matrixPath = Resolve-RepoPath $Matrix
$promptsPath = Resolve-RepoPath $Prompts
$outPath = Resolve-RepoPath $OutDir
if (-not (Test-Path $matrixPath)) { Stop-Bench "matrix $matrixPath not found" }
if (-not (Test-Path $promptsPath)) { Stop-Bench "prompt set $promptsPath not found" }
if ($Repeats -lt 1) { Stop-Bench '-Repeats must be at least 1' }

$matrixDoc = Get-Content $matrixPath -Raw -Encoding utf8 | ConvertFrom-Json
$promptSet = @(
  Get-Content $promptsPath -Encoding utf8 |
    Where-Object { $_.Trim() } |
    ForEach-Object { $_ | ConvertFrom-Json }
)
if ($promptSet.Count -eq 0) { Stop-Bench "$promptsPath has no prompts" }

# CJK by escape rather than by literal character: this file is opened in
# terminals with different code pages, and a mangled character class would
# quietly stop detecting the very thing NFR-090 is about.
$cjkPattern = "[`u{3400}-`u{4DBF}`u{4E00}-`u{9FFF}`u{F900}-`u{FAFF}]"

function Get-Prop($Object, [string]$Name, $Default = 0) {
  # Set-StrictMode turns a missing property into a terminating error, and an
  # answer from a provider is exactly the place where a field may legitimately
  # be absent (Ollama reports no `timings`, an error envelope has no `usage`).
  if ($null -eq $Object) { return $Default }
  $property = $Object.PSObject.Properties[$Name]
  if (-not $property -or $null -eq $property.Value) { return $Default }
  return $property.Value
}

function Get-TextAtPaths($Value, [string[]]$Paths) {
  # The strings a player would actually read. NFR-090 is about narrative text,
  # not about an envelope: `kind`, `player-A` and `region.event_occurred` are
  # identifiers, and counting their letters would report every structured
  # answer as mostly Latin. The gateway checks Call.TextPaths only
  # (swarm-llm-laws.md §9.2); the bench keeps the same rule so that a verdict
  # here means what a rejection there means. `a[].b` walks a list.
  $found = [Collections.Generic.List[string]]::new()
  function Walk($node, [string[]]$rest) {
    if ($rest.Count -eq 0) {
      if ($node -is [string]) { $found.Add($node) }
      return
    }
    $head = $rest[0]
    $tail = @($rest | Select-Object -Skip 1)
    if ($head.EndsWith('[]')) {
      $head = $head.Substring(0, $head.Length - 2)
      $branch = $null
      if ($node -and $node.PSObject.Properties[$head]) { $branch = $node.PSObject.Properties[$head].Value }
      foreach ($item in @($branch)) { if ($null -ne $item) { Walk $item $tail } }
      return
    }
    if ($node -and $node.PSObject.Properties[$head]) { Walk $node.PSObject.Properties[$head].Value $tail }
  }
  # @(...) on purpose: -split of a single-segment path yields a scalar, and
  # indexing a scalar string would walk its characters instead of its fields.
  foreach ($path in $Paths) { Walk $Value ([string[]]@($path -split '\.')) }
  return $found
}

function Get-Percentile([int[]]$Sorted, [double]$Fraction) {
  # The same rule as scripts/llm-bench.sh: nearest rank, clamped. With n = 10
  # the p95 is the largest sample — on purpose: interpolating a 95th percentile
  # out of ten points would look more precise than it is. The decision rests on
  # runs_required runs of n_per_cell, not on one.
  if ($Sorted.Count -eq 0) { return 0 }
  $index = [math]::Floor($Sorted.Count * $Fraction)
  $index = [math]::Min($index, $Sorted.Count - 1)
  return $Sorted[$index]
}

function Format-BenchNumber([double]$Value) {
  <#
    A number as the bash twin writes it: a fixed point, at least one digit after
    it, the rest as they come — `1.0`, `0.0`, `0.034`, `66.7`, `200.0`. Python
    prints a rounded float exactly that way, and the two CSVs are supposed to be
    the same file. .NET prints [math]::Round(1.0, 3) as "1", which is where the
    two started to differ once the separator was fixed.
  #>
  return $Value.ToString('0.0###', [Globalization.CultureInfo]::InvariantCulture)
}

function Get-BenchRound([double]$Value, [int]$Digits) {
  <#
    round() of Python, which is what the bash twin computes every rounded number
    of the CSV, the JSON and the table with: the EXACT binary value of the double
    rounded half to even. [math]::Round is a different rule — it scales the value
    by 10^digits in floating point first, and that product lands on a midpoint
    the double itself is not on. 348/800 is 0.43499999999999999778 as a double:
    Python says 0.43, [math]::Round said 0.44 (T-434 review #1, N-1); 1333/20.0
    is 66.650000000000005684: Python says 66.7, [math]::Round said 66.6, and that
    one is the tps column of the CSV (found while the parity stand of T-405 was
    being built).

    'F40' prints the exact digits of the double (.NET Core 3.0 and later), and a
    decimal keeps 28 of them — more than any double of this script needs to be
    placed on the right side of a midpoint. Checked against Python on 190 425
    values (every k/n up to n = 399 at 2 and 3 digits, 20 000 tps values, 10 000
    values at 4 digits): no difference, where [math]::Round differed on 636.
  #>
  $invariant = [Globalization.CultureInfo]::InvariantCulture
  $exact = [decimal]::Parse($Value.ToString('F40', $invariant), [Globalization.NumberStyles]::Float, $invariant)
  return [double][decimal]::Round($exact, $Digits, [MidpointRounding]::ToEven)
}

function Get-LlamaBuild([string]$Base) {
  # The build the numbers belong to (§6.4 rule 5), in order of trust:
  #   1. MV_LLM_BIN --version — the binary itself, printed as bNNNN;
  #   2. GET <base>/props build_info — the RUNNING server, printed as
  #      bNNNN(props). Without it T-402 wrote the pin b10441 into every row
  #      while the stand ran b10840 and then b10878;
  #   3. the pin of build/versions.env, printed with (pin), so that a stale pin
  #      cannot be read as a fact.
  # The suffix is the source. Whether the stand matches the pin is a question of
  # `make llm-health` (T-404 p. 2) and is deliberately not repeated here.
  $versionsFile = Join-Path $repoRoot 'build/versions.env'
  $pin = ''
  if (Test-Path $versionsFile) {
    $line = Select-String -Path $versionsFile -Pattern '^\s*LLAMACPP_BUILD\s*=\s*(.+)$' |
      Select-Object -First 1
    if ($line) { $pin = $line.Matches[0].Groups[1].Value.Trim() }
  }
  if ($env:MV_LLM_BIN -and (Test-Path $env:MV_LLM_BIN)) {
    try {
      $reported = & $env:MV_LLM_BIN --version 2>&1 | Out-String
      $match = [regex]::Match($reported, 'build[:\s]*(\d+)')
      if ($match.Success) { return "b$($match.Groups[1].Value)" }
    } catch {
      # A binary that cannot be asked is not a reason to abandon the run.
    }
  }
  if ($Base) {
    try {
      $props = Invoke-RestMethod -Uri "$Base/props" -Headers (Get-LlmHeaders) -TimeoutSec 5 `
        -NoProxy -SkipHeaderValidation
      $info = ''
      if ($props -and $props.PSObject.Properties['build_info']) {
        $info = [string]$props.PSObject.Properties['build_info'].Value
      }
      # Only the build number is kept, the same shape `--version` yields, so
      # that the column compares with the pin at a glance: llama-server reports
      # e.g. "b10878-4850c7727".
      $match = [regex]::Match($info, '^\s*b?(\d+)')
      if ($match.Success) { return "b$($match.Groups[1].Value)(props)" }
    } catch {
      # An endpoint without /props (Ollama, a cloud vendor) is measurable; the
      # pin below is then the only thing that can be said about the build.
    }
  }
  if ($pin) { return "$pin(pin)" }
  return 'unknown'
}

function Get-VramUsedMb {
  if (-not (Get-Command nvidia-smi -ErrorAction SilentlyContinue)) { return '' }
  try {
    # All of the output first and the exit code second: a driver that does not
    # answer makes nvidia-smi exit 9 with its complaint on stdout, and that text
    # is not a VRAM figure — the bash twin reads the same status (T-405).
    $lines = @(& nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits 2>$null)
    if ($LASTEXITCODE -ne 0 -or $lines.Count -eq 0) { return '' }
    return ([string]$lines[0]).Trim()
  } catch {
    return ''
  }
}

# One rule for the key, shared with scripts/llm-server.ps1: sent only when there
# is one, never printed (ADR-009 p. 4).
$headers = Get-LlmHeaders

New-Item -ItemType Directory -Force -Path $outPath | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmm'
$startedAt = (Get-Date -Format 'yyyy-MM-ddTHH:mm:ss')
$csvPath = Join-Path $outPath "bench-$stamp.csv"

# The columns of infrastructure.md §6.4 in that order, plus `repeat` and
# `started_at` appended at the end: one invocation may hold several passes, and
# without them two rows of the same cell are indistinguishable. Appending keeps
# every reader of the documented prefix working.
$csvHeader = 'config,provider,placement,model,phase,num_ctx,kv_cache,n,first_call_ms,p50_ms,p95_ms,prompt_tok,completion_tok,cached_tok,tps,valid_json_ratio,cjk_ratio,latin_ratio,lang_pass_ratio,vram_used_mb,llamacpp_build,verdict,repeat,started_at'
# LF, not CRLF: the two implementations are supposed to write the SAME file,
# and Set-Content on Windows ends every line with CR LF while the bash twin
# ends it with LF. A byte comparison of two stands failed on that alone.
[IO.File]::WriteAllText($csvPath, "$csvHeader`n", [Text.UTF8Encoding]::new($false))

# Everything after the CSV exists runs inside try/finally: whatever way the run
# ends — Stop-Bench, an unhandled error, Ctrl+C — a CSV that holds nothing but
# its header does not stay behind (T-405 review #1, N-2; the bash twin does the
# same in its EXIT trap).
try {

  Write-Host "bench: prompts $promptsPath, matrix $matrixPath"

  # How the two input files are named in the report (T-434). Resolved once — the
  # files do not change during a run.
  $matrixRef = Get-ReportPath $matrixPath
  $promptsRef = Get-ReportPath $promptsPath

  $thresholds = $matrixDoc.thresholds
  $request = $matrixDoc.request
  $sampling = $request.sampling_non_thinking
  $latinMax = [double]$thresholds.'NFR-090'.latin_ratio_max
  $langPassMin = [double]$thresholds.'NFR-090'.pass_ratio_min
  $validJsonMin = [double]$thresholds.B3.valid_json_ratio_min
  $p95Max = [double]$thresholds.'NFR-002'.p95_ms_max
  $groupP95Max = [double]$thresholds.'NFR-002-group3'.p95_ms_max
  $groupAdvisory = [bool]$thresholds.'NFR-002-group3'.advisory
  $nPerCell = [int]$matrixDoc.n_per_cell
  if ($Limit -gt 0) { $nPerCell = $Limit }
  $runsRequired = [int]$matrixDoc.runs_required

  $rowsWritten = 0
  $headerPrinted = $false

  function Write-Row($Config, $Phase, $N, $P50, $P95, $Tps, $Json, $Lang, $Cache, $Verdict) {
    # The table header waits for the first measured cell: printed up front, it
    # would stand above an error message and read as "a run found nothing".
    if (-not $script:headerPrinted) {
      '{0,-4} {1,-14} {2,4} {3,8} {4,8} {5,6} {6,6} {7,6} {8,6} {9}' -f `
        'CFG', 'PHASE', 'N', 'P50', 'P95', 'TPS', 'JSON', 'LANG', 'CACHE', 'VERDICT' | Write-Host
      $script:headerPrinted = $true
    }
    '{0,-4} {1,-14} {2,4} {3,8} {4,8} {5,6} {6,6} {7,6} {8,6} {9}' -f `
      $Config, $Phase, $N, $P50, $P95, $Tps, $Json, $Lang, $Cache, $Verdict | Write-Host
  }

  foreach ($configName in $Configs) {
    if (-not $configName) { continue }
    $property = $matrixDoc.configs.PSObject.Properties[$configName]
    if (-not $property) {
      $known = ($matrixDoc.configs.PSObject.Properties.Name | Sort-Object) -join ', '
      Stop-Bench "configuration $configName is not in $matrixPath (have: $known)"
    }
    $cfg = $property.Value
    $serverMode = [string](Get-Prop $cfg 'server_mode' '')
    $urlEnv = $cfg.url_env
    $ep = Resolve-LlmEndpoint -Var $urlEnv
    if ($ep.Error) {
      Stop-Bench "$($ep.Error -replace '^llm: ', '') (configuration $configName expects the endpoint there; see .env.example and ops/metrics/README.md)"
    }
    # $url is the address as configured (what the report shows), $base is the same
    # address as reachable from this machine (host.docker.internal rewritten to
    # 127.0.0.1). Both http://host:1234 and http://host:11434/v1 are legal values
    # of the variable - llama-server serves /v1/... at the root, the OpenAI layer
    # of Ollama lives under /v1 - and the module trims the trailing /v1 for both.
    # Both go into the report: `url` as the variable was set and `probe_url` as it
    # was knocked on. Writing only the probe made two stands incomparable - from
    # `url=http://127.0.0.1:18993` nobody can tell whether the machine was
    # configured with the container alias or with loopback (review Mi-6).
    $url = $ep.Raw
    $base = $ep.Probe

    # --- the server has to be there, and loaded (§6.3) --------------------------
    # /health belongs to llama.cpp and not to the contract: the module falls back
    # to /v1/models, the endpoint the platform actually needs, exactly as
    # scripts/llm-server.ps1 does. Without that fallback this gate declared the
    # owner's running server dead, because it answers /health with 404 (M-1).
    $health = Invoke-LlmHealthProbe $base
    $waited = 0
    while ($health.Code -eq 503 -and $waited -lt 180) {
      # 503 is "loading the model", a state and not a failure: a 27B on a cold
      # page cache takes tens of seconds even with mmap.
      Write-Host "bench: $base/health = 503 (model loading), waited ${waited}s"
      Start-Sleep -Seconds 5
      $waited += 5
      $health = Invoke-LlmHealthProbe $base
    }
    # if/elseif rather than switch: `break` inside a switch that sits inside a
    # loop is a well-known trap, and the difference would only show up on a stand
    # with several configurations.
    if ($health.Code -eq 0) {
      Stop-Bench "no answer from $base — the LLM process is not running. Start it: make llm-up (config $configName expects $urlEnv=$url)"
    } elseif ($health.Code -eq 503) {
      Stop-Bench "$base/health is still 503 after 180 s — the model has not finished loading; check ops/llm-server.log"
    } elseif ($health.Code -ne 200) {
      Stop-Bench "$base$($health.Via) answered $($health.Code) — this is not a llama-server / OpenAI-compatible endpoint (config $configName, $urlEnv=$url)"
    }

    $models = @()
    try {
      $models = @((Invoke-RestMethod -Uri "$base/v1/models" -Headers $headers -TimeoutSec 10 -NoProxy -SkipHeaderValidation).data.id)
    } catch {
      # An endpoint without /v1/models is measurable; it only costs the check
      # below, which is a convenience and not a requirement.
      $models = @()
    }

    # Per configuration, after the gate: /props belongs to the server of THIS
    # configuration, and C / A may be another process than E.
    $build = Get-LlamaBuild $base
    Write-Host "bench: $configName build $build"

    $vram = Get-VramUsedMb
    $configRows = 0
    $requestLog = [Collections.Generic.List[object]]::new()
    $cellLog = [Collections.Generic.List[object]]::new()
    $firstCall = $null

    foreach ($repeat in 1..$Repeats) {
      # --- B2: the cold first call, kept out of p50/p95 -------------------------
      # The first request after a start allocates the compute buffers and warms
      # the CUDA kernels; folding it into the percentiles would make every run
      # depend on how long ago the server came up.
      $warmBody = @{
        model                = $cfg.narrative
        max_tokens           = 1
        stream               = $false
        chat_template_kwargs = @{ enable_thinking = $false }
        messages             = @(@{ role = 'user'; content = 'ping' })
      } | ConvertTo-Json -Depth 10
      $watch = [Diagnostics.Stopwatch]::StartNew()
      # The STATUS CODE is judged, not just the exception: -SkipHttpErrorCheck
      # means a 404 from a server with another set of endpoints never reaches the
      # catch, so this call used to stay silent where the bash twin reported the
      # code (review Mi-5 — the same defect as Mi-7 of review #1, which was fixed
      # in llm-server.* and not carried into the bench). Same sentence as the twin.
      $warmCode = '000'
      try {
        $warmResponse = Invoke-WebRequest -Uri "$base/v1/chat/completions" -Method Post -Headers $headers `
          -ContentType 'application/json; charset=utf-8' `
          -Body ([Text.Encoding]::UTF8.GetBytes($warmBody)) -TimeoutSec 120 -SkipHttpErrorCheck `
          -NoProxy -SkipHeaderValidation
        $warmCode = [string][int]$warmResponse.StatusCode
      } catch {
        $warmCode = '000'
      }
      $firstCall = [int]$watch.ElapsedMilliseconds
      # Only a 200 is a cold call. Anything else measured how fast the server
      # refuses, so the number is not written at all — an empty first_call_ms in
      # the CSV, null in the JSON — rather than written and explained elsewhere
      # (T-402 review #1; the sentence is the same in the bash twin).
      if ($warmCode -ne '200') {
        $firstCall = $null
        [Console]::Error.WriteLine("bench: warm-up call answered $warmCode — first_call_ms left empty, the cold call was not measured")
      }

      foreach ($phase in @('tick', 'phase2', 'phase2-group3')) {
        $model = if ($phase -eq 'tick') { $cfg.tick } else { $cfg.narrative }
        $timeout = [int]$request.timeout_s.$phase
        $threshold = if ($phase -eq 'phase2-group3') { $groupP95Max } else { $p95Max }
        $advisory = ($phase -eq 'phase2-group3' -and $groupAdvisory)

        # A model the endpoint does not serve is an operator error (wrong server
        # mode, wrong pull), not a measurement — skipping loudly beats measuring
        # whatever the server substitutes.
        if ($models.Count -gt 0 -and $models -notcontains $model) {
          # ${serverMode}, not $serverMode: PowerShell reads a question mark as
          # part of a variable name, so this line used to THROW under Set-StrictMode
          # ("the variable '$serverMode?' has not been set") on the one path where
          # it is printed — a skipped phase ended the run with a stack trace and
          # exit 1, while the bash twin printed the sentence and exited 2.
          Write-Host "bench: model $model is not in $base/v1/models — phase $phase skipped (server mode ${serverMode}?)"
          continue
        }

        $cellPrompts = @($promptSet | Where-Object { $_.phase -eq $phase } | Select-Object -First $nPerCell)
        if ($cellPrompts.Count -eq 0) {
          Write-Host "bench: no prompts with phase=$phase in $promptsPath — skipped"
          continue
        }

        $latencies = [Collections.Generic.List[int]]::new()
        $okJson = 0; $cjkHits = 0; $latinSum = 0.0; $langPass = 0
        $promptTok = 0; $completionTok = 0; $cachedTok = 0; $predictedMs = 0.0; $errors = 0

        # The position of the prompt inside the cell, 1-based: the bash twin
        # writes it into requests[] as `index`, and the two reports are supposed
        # to carry the same fields.
        $index = 0
        foreach ($prompt in $cellPrompts) {
          $index++
          $body = [ordered]@{
            model            = $model
            stream           = $false
            max_tokens       = $prompt.max_tokens
            response_format  = @{
              type        = 'json_schema'
              json_schema = @{
                name   = $prompt.schema_name
                schema = $prompt.schema
                strict = $true
              }
            }
            # Not a preference: with thinking on, llama.cpp does not apply the
            # grammar of the schema (#20345), so the run would measure the defect
            # instead of the model.
            chat_template_kwargs = @{ enable_thinking = $false }
            messages         = @(
              @{ role = 'system'; content = $prompt.system },
              @{ role = 'user'; content = $prompt.user }
            )
          }
          foreach ($key in @('temperature', 'top_p', 'top_k', 'min_p', 'presence_penalty')) {
            if ($sampling.PSObject.Properties[$key]) { $body[$key] = $sampling.PSObject.Properties[$key].Value }
          }
          $json = $body | ConvertTo-Json -Depth 40

          $status = 0
          $content = ''
          $answer = $null
          $watch = [Diagnostics.Stopwatch]::StartNew()
          try {
            $response = Invoke-WebRequest -Uri "$base/v1/chat/completions" -Method Post -Headers $headers `
              -ContentType 'application/json; charset=utf-8' `
              -Body ([Text.Encoding]::UTF8.GetBytes($json)) -TimeoutSec $timeout -SkipHttpErrorCheck `
              -NoProxy -SkipHeaderValidation
            $status = [int]$response.StatusCode
            if ($status -eq 200) { $answer = $response.Content | ConvertFrom-Json }
          } catch {
            $status = 0
          }
          $latency = [int]$watch.ElapsedMilliseconds

          $recordError = ''
          $rowPromptTok = 0; $rowCompletionTok = 0; $rowCachedTok = 0; $rowPredicted = 0.0
          # Empty, not 0, when the server did not report it: Ollama has no
          # `timings`, and a zero would read as "the prompt was free".
          $rowPromptMs = $null
          $rowOkJson = 0; $rowCjk = 0; $rowLatin = 0.0; $rowLangPass = 0

          if ($status -ne 200 -or $null -eq $answer) {
            $recordError = "http_$status"
            $errors++
          } else {
            $usage = Get-Prop $answer 'usage' $null
            $rowPromptTok = [int](Get-Prop $usage 'prompt_tokens' 0)
            $rowCompletionTok = [int](Get-Prop $usage 'completion_tokens' 0)
            $rowCachedTok = [int](Get-Prop (Get-Prop $usage 'prompt_tokens_details' $null) 'cached_tokens' 0)
            $timings = Get-Prop $answer 'timings' $null
            $rowPredicted = [double](Get-Prop $timings 'predicted_ms' 0)
            # Prompt processing as the server measured it (T-434). Latency minus
            # predicted_ms is "everything else", and without this number a spike
            # there could not be split into prompt processing and the rest from
            # the report alone (baseline.md §2.1, §2.5 of T-402).
            $rawPromptMs = Get-Prop $timings 'prompt_ms' $null
            if ($null -ne $rawPromptMs) { $rowPromptMs = [double]$rawPromptMs }
            $choices = @(Get-Prop $answer 'choices' @())
            if ($choices.Count -gt 0) {
              $content = [string](Get-Prop (Get-Prop $choices[0] 'message' $null) 'content' '')
            }
            if (-not $content) { $recordError = 'empty_content' }

            $parsed = $null
            try {
              $parsed = $content | ConvertFrom-Json
              $rowOkJson = 1
            } catch {
              $rowOkJson = 0
            }

            # A valid answer is measured on its text fields; a broken one is
            # measured as it arrived — there is nothing else to look at.
            $narrative = $content
            $paths = @()
            if ($prompt.PSObject.Properties['text_paths']) { $paths = @($prompt.text_paths) }
            if ($rowOkJson -eq 1 -and $paths.Count -gt 0) {
              $narrative = (Get-TextAtPaths $parsed $paths) -join ' '
            }
            $rowCjk = if ($narrative -match $cjkPattern) { 1 } else { 0 }
            $letters = ([regex]::Matches($narrative, '\p{L}')).Count
            $rowLatin = if ($letters -gt 0) { ([regex]::Matches($narrative, '[A-Za-z]')).Count / $letters } else { 0.0 }
            $rowLangPass = if ($rowCjk -eq 0 -and $rowLatin -le $latinMax) { 1 } else { 0 }
            # The verdict above reads the unrounded share, the log and the cell read
            # the share rounded to four digits — in that order, as the bash twin
            # does: it writes round(latin, 4) into its request log and averages the
            # cell over what it wrote, so the unrounded sum here was a second rule.
            $rowLatin = Get-BenchRound $rowLatin 4

            if (-not $recordError) {
              $latencies.Add($latency)
              $okJson += $rowOkJson
              $cjkHits += $rowCjk
              $latinSum += $rowLatin
              $langPass += $rowLangPass
              $promptTok += $rowPromptTok
              $completionTok += $rowCompletionTok
              $cachedTok += $rowCachedTok
              $predictedMs += $rowPredicted
            } else {
              $errors++
            }
          }

          $requestLog.Add([ordered]@{
              config            = $configName
              repeat            = $repeat
              phase             = $phase
              index             = $index
              prompt_id         = $prompt.id
              http              = $status
              latency_ms        = $latency
              ok_json           = $rowOkJson
              cjk               = $rowCjk
              latin             = $rowLatin
              lang_pass         = $rowLangPass
              prompt_tokens     = $rowPromptTok
              completion_tokens = $rowCompletionTok
              cached_tokens     = $rowCachedTok
              predicted_ms      = $rowPredicted
              prompt_ms         = $rowPromptMs
              error             = $recordError
            })
          Write-Host '.' -NoNewline
        }
        Write-Host ''

        $n = $latencies.Count
        if ($n -eq 0) {
          Write-Host "bench: $configName/$phase produced no usable answer ($errors errors) — no row written"
          continue
        }
        if ($errors -gt 0) { Write-Host "bench: $configName/$phase had $errors failed requests" }

        $sorted = [int[]]($latencies | Sort-Object)
        $p50 = Get-Percentile $sorted 0.5
        $p95 = Get-Percentile $sorted 0.95
        # Tokens per second of generation, from the server's own timings: the wall
        # clock of the client also contains prompt processing and the HTTP hop,
        # and mixing the two makes runs incomparable.
        $tps = if ($predictedMs -gt 0) { Get-BenchRound ($completionTok / ($predictedMs / 1000.0)) 1 } else { 0 }
        # The CSV text of tps: a bare `0` exactly when the server reported no
        # timings (Ollama), a fixed-point number otherwise — the two cases the
        # bash twin distinguishes as an int and a float.
        $tpsText = if ($predictedMs -gt 0) { Format-BenchNumber $tps } else { '0' }
        $validJsonRatio = Get-BenchRound ($okJson / $n) 3
        $cjkRatio = Get-BenchRound ($cjkHits / $n) 3
        $latinRatio = Get-BenchRound ($latinSum / $n) 3
        $langPassRatio = Get-BenchRound ($langPass / $n) 3

        $verdict = 'n/a'
        if ($phase.StartsWith('phase2')) {
          # An error that never produced an answer fails the cell too: a
          # configuration that times out on a fifth of the prompts has not passed
          # NFR-002, whatever the p95 of the answers that did arrive says.
          $passed = ($p95 -le $threshold) -and ($langPassRatio -ge $langPassMin) `
            -and ($validJsonRatio -ge $validJsonMin) -and ($errors -eq 0)
          $verdict = if ($passed) { 'pass' } else { 'fail' }
          if ($advisory) { $verdict += '*' }
        }

        # Every fractional column goes through Format-BenchNumber: the CSV is the
        # file two implementations are supposed to write identically, and a
        # number is where they used to stop doing that.
        $row = @(
          $configName, $cfg.provider, $Placement, $model, $phase, $NumCtx, $KvCache,
          $n, $firstCall, $p50, $p95, $promptTok, $completionTok, $cachedTok, $tpsText,
          (Format-BenchNumber $validJsonRatio), (Format-BenchNumber $cjkRatio),
          (Format-BenchNumber $latinRatio), (Format-BenchNumber $langPassRatio),
          $vram, $build, $verdict, $repeat, $startedAt
        ) -join ','
        [IO.File]::AppendAllText($csvPath, "$row`n", [Text.UTF8Encoding]::new($false))
        $cellLog.Add([ordered]@{
            config           = $configName
            provider         = $cfg.provider
            placement        = $Placement
            model            = $model
            phase            = $phase
            num_ctx          = $NumCtx
            kv_cache         = $KvCache
            n                = $n
            first_call_ms    = $firstCall
            p50_ms           = $p50
            p95_ms           = $p95
            prompt_tok       = $promptTok
            completion_tok   = $completionTok
            cached_tok       = $cachedTok
            tps              = $tps
            valid_json_ratio = $validJsonRatio
            cjk_ratio        = $cjkRatio
            latin_ratio      = $latinRatio
            lang_pass_ratio  = $langPassRatio
            vram_used_mb     = $vram
            llamacpp_build   = $build
            verdict          = $verdict
            repeat           = $repeat
            started_at       = $startedAt
          })

        # The share of the prompt served from the cache of llama.cpp. The system
        # half of an agent prompt is stable, so a low share on the stand means the
        # slots were reset between calls — a reason for a slow p95 that has
        # nothing to do with the model (§6.3).
        $cacheShare = if ($promptTok -gt 0) { Get-BenchRound ($cachedTok / $promptTok) 2 } else { 0 }
        $cacheShareText = if ($promptTok -gt 0) { Format-BenchNumber $cacheShare } else { '0' }
        Write-Row $configName $phase $n $p50 $p95 $tpsText (Format-BenchNumber $validJsonRatio) `
          (Format-BenchNumber $langPassRatio) $cacheShareText $verdict
        $rowsWritten++
        $configRows++
      }
    }

    # --- the per-configuration JSON report ---------------------------------------
    # A report with no cells in it would look like a run that found nothing, which
    # is not the same thing as a run that never happened.
    if ($configRows -eq 0) {
      Write-Host "bench: $configName measured nothing — no report written"
      continue
    }
    $safeConfig = ($configName -replace '\+', 'p') -replace '[^A-Za-z0-9._-]', ''
    $reportPath = Join-Path $outPath "bench-$safeConfig-$stamp.json"
    $document = [ordered]@{
      schema       = 'multiverse.bench.v1'
      generated_by = 'scripts/llm-bench.ps1'
      meta         = [ordered]@{
        config         = $configName
        provider       = $cfg.provider
        placement      = $Placement
        server_mode    = $serverMode
        url            = $url
        probe_url      = $base
        num_ctx        = $NumCtx
        kv_cache       = $KvCache
        llamacpp_build = $build
        vram_used_mb   = $vram
        first_call_ms  = $firstCall
        n_per_cell     = $nPerCell
        repeats        = $Repeats
        runs_required  = $runsRequired
        started_at     = $startedAt
        matrix         = $matrixRef.Path
        matrix_sha256  = $matrixRef.Sha256
        prompts        = $promptsRef.Path
        prompts_sha256 = $promptsRef.Sha256
        script         = 'scripts/llm-bench.ps1'
      }
      cells        = $cellLog
      requests     = $requestLog
    }
    $document | ConvertTo-Json -Depth 8 | Set-Content -Path $reportPath -Encoding utf8
    Write-Host "bench: $(Get-ShownPath $reportPath)"
  }

  if ($rowsWritten -eq 0) {
    # The header-only CSV is litter in a directory that is read as results.
    Remove-Item $csvPath -ErrorAction SilentlyContinue
    Stop-Bench 'nothing was measured — every phase was skipped or failed; see the messages above'
  }

  Write-Host "bench: $(Get-ShownPath $csvPath) ($rowsWritten rows)"
  Write-Host "bench: this is ONE run. The matrix asks for $runsRequired runs per configuration at different moments (restart llama-server between them); the decision goes into ops/metrics/baseline.md"
} finally {
  Remove-HeaderOnlyCsv
}
