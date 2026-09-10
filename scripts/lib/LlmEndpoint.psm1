#Requires -Version 7
<#
.SYNOPSIS
  The ONE rule that turns MV_LLM_URL into an address (T-404).

.DESCRIPTION
  infrastructure.md v0.3 §6.1-§6.4, ADR-005 add. 2 p. 2, p. 3. Imported by
  scripts/llm-server.ps1 and scripts/llm-bench.ps1. The bash twin is
  scripts/lib/llm-endpoint.sh and the two files are one rule written twice: a
  divergence between them is a review finding, not a detail. Every message
  printed from here exists in both, character for character.

  WHY THIS FILE EXISTS. T-404 made the ADDRESS have one source of truth,
  MV_LLM_URL, and review #1 showed that this was not enough: the RULE that
  turns the variable into an address lived in four scripts with three different
  behaviours (the bench trimmed a trailing /v1 and the server did not; the
  server rewrote host.docker.internal and the bench did not; the server fell
  back from /health to /v1/models and the bench did not). On the owner's stand
  that combination made `make bench` say "the LLM process is not running" to a
  server that was answering. One variable with four rules is still two sources
  of truth, so the rule moved here.

  WHAT THE RULE IS, in one paragraph. From MV_LLM_URL (MV_OLLAMA_URL when
  MV_LLM_PROVIDER=ollama) come three things: the address AS CONFIGURED (what is
  printed, and what a remote endpoint is probed at), the address REACHABLE FROM
  THIS MACHINE (the same, with the host.docker.internal alias rewritten to
  127.0.0.1 — that name resolves inside a container and not on the host), and
  the PORT the local llama-server binds. A trailing /v1 is cut off: the variable
  holds the BASE address, and vendor documentation usually hands out the address
  with /v1 on the end (orchestrator decision, journal 2026-09-11). Nothing else
  about the value is rewritten: a real remote address is probed as it is, or the
  check would be measuring the wrong host.

  The URL is parsed by hand rather than with [uri] on purpose. [uri] is close to
  the bash parser but not equal to it — it keeps the brackets of an IPv6 literal
  in .Host (which is how [::1] came to be classified as a cloud address, review
  M-3), it silently drops userinfo where the shell kept it, and it accepts
  shapes the platform's Go client would refuse. Parity is easier to hold than to
  prove, so both implementations run the same algorithm.

  EVERY string comparison here is ORDINAL, and that is not a style choice.
  String.EndsWith, StartsWith and IndexOf(string) in .NET compare by the rules
  of the culture by default, and a culture comparison SKIPS the characters
  Unicode calls ignorable — a soft hyphen U+00AD, a zero width space U+200B.
  With MV_LLM_URL=http://127.0.0.1:8888/v1<U+00AD> this module used to trim a
  /v1 that was not at the end and report "200 ok" for an address the variable
  did not hold, while the bash twin compared bytes and reported 404: one input,
  opposite exit codes, and the green one on the main machine of the project
  (review M-1). Values with such characters are refused outright as well; the
  ordinal comparisons are what keeps the refusal honest.
#>

Set-StrictMode -Version Latest

# Two console settings, both fixes of findings, both belonging to every script
# that imports this module:
#   * en-US messages — a localised .NET exception text came out of the Windows
#     console as mojibake and undid the "all output is English" decision of
#     T-403 (review Mi-5);
#   * UTF-8 output — the em dash and § of these messages are transliterated by
#     the OEM code page otherwise, and the two implementations then differ in
#     bytes while agreeing in words (review N-2).
[Threading.Thread]::CurrentThread.CurrentUICulture = 'en-US'
try { [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false) } catch { }

# --- reading the environment -------------------------------------------------

function Get-LlmEnv {
  <#
    The only way this module reads a tooling variable. Blank means "not
    configured" and yields the default, in BOTH implementations: ?? does not
    react to an empty string, so `MV_LLM_NUM_CTX=` used to build `--ctx-size 0`
    here while bash substituted 8192 (review Mi-9, Mi-10, and the root of C-1).
  #>
  param([Parameter(Mandatory)][string] $Name, [string] $Default = '')
  $value = [Environment]::GetEnvironmentVariable($Name)
  if ([string]::IsNullOrWhiteSpace($value)) { return $Default }
  return $value.Trim()
}

# The closed set of MV_LLM_PROVIDER, as declared in shared/env/vars.go.
$script:LlmProviders = @('openai_compat', 'ollama', 'anthropic', 'recorded', 'fake')

# --- printing ----------------------------------------------------------------
#
# Parity rule, chosen here and obeyed by both implementations: everything a
# healthy run prints goes to stdout (including WARNING lines), and only a
# message that ends the run goes to stderr. The .sh used to send its SEC-15
# warning to stderr while the .ps1 sent it to stdout, which made the two outputs
# differ in ordering under 2>&1 for no reason (review Mi-12).

# --- character classes -------------------------------------------------------
#
# The same two questions the bash twin asks of bytes, asked here of UTF-16
# characters. For any value that is valid UTF-8 the answers agree.

function Test-LlmPrintableAscii {
  <#
    False when the value carries anything outside 0x20-0x7E. An invisible
    character — U+00AD, U+200B — is the normal result of copying an address out
    of vendor documentation, and it is what made the two implementations
    disagree (review M-1). See the ordinal note in the header of this file.
  #>
  param([string] $Value = '')
  return -not ($Value -cmatch '[^\x20-\x7E]')
}

function Test-LlmControlChars {
  <#
    True when the value carries a character below 0x20. -SkipHeaderValidation
    turns off exactly the .NET check that forbids CR and LF in a header value,
    so a key pasted across two lines of .env went to the server TRUNCATED, with
    its tail sent as a header of its own, and nobody was told (review M-3).
  #>
  param([string] $Value = '')
  return [bool]($Value -cmatch '[\x00-\x1F]')
}

# Protect-LlmSecret hides the key in anything printed. It only redacts a value
# of at least $script:LlmRedactMin characters: MV_LLM_API_KEY=8888 used to turn
# every line into `probing http://127.0.0.1:***`, so the diagnostic became
# unreadable exactly when it was needed (review Mi-10). A value that short is
# not a key of any endpoint, and the bash twin uses the same threshold.
$script:LlmRedactMin = 8

function Protect-LlmSecret {
  param([string] $Text = '')
  $key = Get-LlmEnv 'MV_LLM_API_KEY'
  if ($key -and $key.Length -ge $script:LlmRedactMin) {
    $Text = $Text.Replace($key, '***', [StringComparison]::Ordinal)
  }
  return $Text
}

function Write-LlmLine {
  param([string] $Message = '')
  [Console]::Out.WriteLine((Protect-LlmSecret $Message))
}

function Write-LlmFail {
  param([string] $Message = '')
  [Console]::Error.WriteLine((Protect-LlmSecret $Message))
}

function Write-LlmRetiredPortNote {
  <#
    Mi-1: the retirement of MV_LLM_PORT is declared in the manifest, but only
    `mvctl env check --env` ever printed it, and no make target passes that
    flag. The scripts run with .env already loaded, and this is the place where
    an operator with a stale .env is looking for an explanation.
  #>
  param([Parameter(Mandatory)][string] $Var)
  $retired = Get-LlmEnv 'MV_LLM_PORT'
  if ($retired) {
    Write-LlmLine "llm: MV_LLM_PORT=$retired is retired and ignored — the port comes from $Var (T-404); remove the line from .env"
  }
}

# --- the rule ----------------------------------------------------------------

function New-LlmEndpoint {
  # One shape, always all keys: a caller under Set-StrictMode must never have to
  # test for the existence of a field.
  return [ordered]@{
    Scheme       = ''
    Host         = ''  # no brackets, even for an IPv6 literal
    Port         = ''
    Path         = ''
    Url          = ''  # the address as configured, canonical: what is printed
    Probe        = ''  # the same address as reachable from THIS machine
    Raw          = ''  # the configured value, trimmed
    Var          = ''
    Provider     = ''
    Has          = $false
    Startable    = $false
    Trusted      = $false
    TrimmedV1    = $false
    PortExplicit = $false
    Error        = ''
  }
}

function Convert-LlmUrl {
  <#
    Fills an endpoint object from one address string. Sets .Error and leaves
    .Has false when the value is not an address the platform could use; the
    caller decides whether that is fatal (it always is for `up`).
  #>
  param([Parameter(Mandatory)] $Endpoint, [string] $Value = '')

  $raw = ''
  if ($null -ne $Value) { $raw = $Value.Trim() }
  $var = $Endpoint.Var

  # Checked before anything is compared, because every comparison below is what
  # an invisible character breaks (review M-1).
  if ($raw -and -not (Test-LlmPrintableAscii $raw)) {
    $Endpoint.Error = "$var=$raw carries a character outside printable ASCII; an address copied from documentation often brings an invisible one with it (a soft hyphen U+00AD, a zero width space U+200B) — retype the value by hand"
    return $Endpoint
  }
  if ($raw.Contains('?', [StringComparison]::Ordinal) -or $raw.Contains('#', [StringComparison]::Ordinal)) {
    $Endpoint.Error = "$var=$raw carries a query or a fragment; the value must be scheme://host[:port][/path]"
    return $Endpoint
  }
  if (-not $raw) {
    $Endpoint.Error = "$var is empty — the platform has no LLM address (shared/env: a variable set to an empty value overrides the default)"
    return $Endpoint
  }

  # A value without a scheme is REFUSED in both implementations. The .sh used to
  # accept 127.0.0.1:8888 and the .ps1 threw a raw error record at it (review
  # Mi-11); accepting it is the worse of the two, because the platform's Go
  # client cannot: http.Client on such a value fails with "unsupported protocol
  # scheme". A wrapper that works where the platform breaks reports a green LLM
  # nobody can call.
  $split = $raw.IndexOf('://', [StringComparison]::Ordinal)
  if ($split -lt 0) {
    $Endpoint.Error = "$var=$raw has no scheme; write it as http://host:port (the platform's HTTP client refuses a value without one)"
    return $Endpoint
  }
  $scheme = $raw.Substring(0, $split).ToLowerInvariant()
  $rest = $raw.Substring($split + 3)
  if ($scheme -ne 'http' -and $scheme -ne 'https') {
    $Endpoint.Error = "$var=$raw uses scheme '$scheme'; only http and https are addresses of an OpenAI-compatible endpoint"
    return $Endpoint
  }

  $slash = $rest.IndexOf('/', [StringComparison]::Ordinal)
  if ($slash -ge 0) {
    $hostport = $rest.Substring(0, $slash)
    $path = $rest.Substring($slash)
  } else {
    $hostport = $rest
    $path = ''
  }

  # userinfo. The .sh used to keep `user:s3cr3t@` as part of the host, which
  # made a loopback address look remote AND printed the password to the console
  # and into every captured log (review M-4). Masking it was only half a fix:
  # the credentials were then DROPPED, so the probe went out without them while
  # Go's net/http builds a Basic header from exactly that userinfo — the probe
  # and the platform would ask different questions of the same endpoint (review
  # Mi-4). The value is refused instead, and it is not echoed back: it is the
  # one shape of this variable that carries a secret.
  if ($hostport.Contains('@', [StringComparison]::Ordinal)) {
    $Endpoint.Error = "$var carries user information in front of the host; the key of an endpoint lives in MV_LLM_API_KEY, not in the address (the value is not printed here because it holds a secret)"
    return $Endpoint
  }

  $hostName = ''
  $port = ''
  if ($hostport.StartsWith('[', [StringComparison]::Ordinal)) {
    $close = $hostport.IndexOf(']', [StringComparison]::Ordinal)
    if ($close -lt 0) {
      $Endpoint.Error = "$var=$raw has an unclosed IPv6 literal"
      return $Endpoint
    }
    $hostName = $hostport.Substring(1, $close - 1)
    $tail = $hostport.Substring($close + 1)
    if ($tail.StartsWith(':', [StringComparison]::Ordinal)) { $port = $tail.Substring(1) }
    elseif ($tail) {
      $Endpoint.Error = "$var=$raw has trailing characters after the IPv6 literal"
      return $Endpoint
    }
  } else {
    $colon = $hostport.LastIndexOf(':', [StringComparison]::Ordinal)
    if ($colon -ge 0) {
      $hostName = $hostport.Substring(0, $colon)
      $port = $hostport.Substring($colon + 1)
    } else {
      $hostName = $hostport
    }
    # An IPv6 literal keeps its brackets in the URL and loses them in .Host, so
    # that '::1' compares equal to the entry in the loopback list. [uri].Host in
    # .NET keeps the brackets, which is exactly why this script called [::1] a
    # cloud address, refused to start a local server on it and printed a false
    # gateway warning (review M-3).
    if ($hostName.Contains(':')) {
      $Endpoint.Error = "$var=$raw looks like a bare IPv6 literal; bracket it: http://[::1]:8888"
      return $Endpoint
    }
  }
  $hostName = $hostName.ToLowerInvariant()
  if (-not $hostName) {
    $Endpoint.Error = "$var=$raw has no host"
    return $Endpoint
  }

  $explicit = $false
  if ($port) {
    if ($port -notmatch '^\d+$') {
      $Endpoint.Error = "$var=$raw has a non-numeric port '$port'"
      return $Endpoint
    }
    if ([int]$port -lt 1 -or [int]$port -gt 65535) {
      $Endpoint.Error = "$var=$raw has port $port, which is outside 1-65535"
      return $Endpoint
    }
    $explicit = $true
  } else {
    $port = if ($scheme -eq 'https') { '443' } else { '80' }
  }
  # A port equal to the scheme default is not printed back, so that the two
  # implementations agree: .NET reports IsDefaultPort for http://host:80 too.
  if (($scheme -eq 'http' -and $port -eq '80') -or ($scheme -eq 'https' -and $port -eq '443')) {
    $explicit = $false
  }

  # The trailing /v1. MV_LLM_URL is the BASE address; vendors hand out the
  # address with /v1 on the end, and without this both scripts would ask for
  # /v1/v1/models and call a living endpoint dead (review Mi-2, orchestrator
  # decision). The rule already existed in llm-bench.*; here it is the only copy.
  $path = $path.TrimEnd('/')
  if ($path.EndsWith('/v1', [StringComparison]::Ordinal)) {
    $path = $path.Substring(0, $path.Length - 3)
    $Endpoint.TrimmedV1 = $true
  }
  $path = $path.TrimEnd('/')

  $shown = if ($hostName.Contains(':', [StringComparison]::Ordinal)) { "[$hostName]" } else { $hostName }
  $authority = if ($explicit) { "${shown}:$port" } else { $shown }
  # host.docker.internal is how a container reaches this machine; from the host
  # the name does not resolve. Only that alias is rewritten.
  $probeHost = if ($hostName -eq 'host.docker.internal') { '127.0.0.1' } else { $shown }
  $probeAuthority = if ($explicit) { "${probeHost}:$port" } else { $probeHost }

  $Endpoint.Scheme = $scheme
  $Endpoint.Host = $hostName
  $Endpoint.Port = $port
  $Endpoint.Path = $path
  $Endpoint.Url = "${scheme}://$authority$path"
  $Endpoint.Probe = "${scheme}://$probeAuthority$path"
  $Endpoint.Raw = $raw
  $Endpoint.PortExplicit = $explicit
  return $Endpoint
}

function Test-LlmHostLocal {
  <#
    An address this machine answers on. It is the question "would llama-server
    started here own this address", so it is loopback, the any-address and the
    container alias — and nothing else.
  #>
  param([string] $HostName = '')
  if ($HostName -in 'localhost', '0.0.0.0', '::', '::1', 'host.docker.internal') { return $true }
  return $HostName -match '^127\.'
}

function Test-LlmHostTrusted {
  <#
    A DIFFERENT question — "is this address outside the trusted network", the one
    the cloud gate of ADR-005 add. 2 p. 3 asks — and therefore also counts the
    private ranges: a server on the LAN is not a cloud vendor, even though it is
    not startable from here.
  #>
  param([string] $HostName = '')
  if (Test-LlmHostLocal $HostName) { return $true }
  return $HostName -match '^(10\.|192\.168\.|172\.(1[6-9]|2[0-9]|3[01])\.|fd..:|fe80:)'
}

function Resolve-LlmEndpoint {
  <#
    Picks the variable by provider and parses it. -Var overrides the choice: the
    bench reads the variable name out of the measurement matrix (url_env), one
    per configuration.

      .Has = $false, .Error = ''      — this provider has no address on this side
      .Has = $false, .Error = '...'   — it should have had one; the caller goes red
  #>
  param([string] $Var = '')

  $ep = New-LlmEndpoint
  $ep.Provider = Get-LlmEnv 'MV_LLM_PROVIDER' 'openai_compat'

  # The same closed set the manifest declares for MV_LLM_PROVIDER
  # (shared/env/vars.go, OneOf). An unlisted value used to pass through both
  # wrappers in silence, which cost the pin and the VRAM report and made `up`
  # explain itself with the wrong reason, while `mvctl env check --env` refused
  # the very same value (review Mi-11).
  if ($ep.Provider -notin $script:LlmProviders) {
    $ep.Error = "llm: MV_LLM_PROVIDER=$($ep.Provider) is not one of $($script:LlmProviders -join ', '); the set is declared in shared/env and mvctl env check refuses the same value"
    return $ep
  }

  if ($Var) {
    $ep.Var = $Var
    $value = Get-LlmEnv $Var
    if (-not $value) {
      $ep.Error = "llm: $Var is empty — no address to probe"
      return $ep
    }
  } else {
    switch ($ep.Provider) {
      { $_ -in 'anthropic', 'recorded', 'fake' } { return $ep }
      'ollama' { $ep.Var = 'MV_OLLAMA_URL' }
      default { $ep.Var = 'MV_LLM_URL' }
    }
    $value = Get-LlmEnv $ep.Var
    if (-not $value) {
      if ($ep.Var -eq 'MV_OLLAMA_URL') {
        $ep.Error = 'llm: MV_LLM_PROVIDER=ollama but MV_OLLAMA_URL is empty — no address to probe (ADR-005 add. 2 p. 2)'
        return $ep
      }
      # No address, no green light. This is review C-1: `??` does not react to
      # an empty string, so an MV_LLM_URL that had been cleared but not removed
      # made this script print "nothing to check" and return 0 — a fully
      # unconfigured LLM passing the strict `make health` on the main machine of
      # the project.
      #
      # Blank and unset are the same answer here, and since review #2 they are
      # the same answer on the other side too: MV_LLM_URL has NO default in
      # shared/env/vars.go any more, it is Required(). It used to declare
      # http://127.0.0.1:1234, so a machine with the line deleted from .env had
      # the platform quietly talking to 1234 while this wrapper said there was
      # no address — two sources of truth for the one case the task exists to
      # close (review M-4, orchestrator decision). An address cannot be guessed.
      $ep.Error = "llm: $($ep.Var) has no value — the platform has no LLM address (the variable is required and has no default; an empty value is not configuration either). Put the address in .env"
      return $ep
    }
  }

  $ep = Convert-LlmUrl $ep $value
  if ($ep.Error) {
    $ep.Error = "llm: $($ep.Error)"
    return $ep
  }
  return (Complete-LlmEndpoint $ep)
}

function Complete-LlmEndpoint {
  <#
    The tail both resolution paths share, so that the rule cannot differ
    between `make llm-health` and `make bench`. The bash twin is
    llm_endpoint_finish.
  #>
  param([Parameter(Mandatory)] $Endpoint)

  $Endpoint.Has = $true
  $Endpoint.Startable = (Test-LlmHostLocal $Endpoint.Host) -and ($Endpoint.Provider -eq 'openai_compat')
  $Endpoint.Trusted = Test-LlmHostTrusted $Endpoint.Host

  # A local endpoint without a port is not "the port is 80": it is a port
  # nobody chose. The scheme default was passed to llama-server as --port 80
  # and knocked on by the probe, and the operator who wrote the value meant
  # something else (review Mi-12). A remote address without a port is left
  # alone — there 443 and 80 are what the vendor documents.
  if ($Endpoint.Startable -and -not $Endpoint.PortExplicit) {
    $Endpoint.Has = $false
    $Endpoint.Startable = $false
    $Endpoint.Error = "llm: $($Endpoint.Var)=$($Endpoint.Raw) has no port; the local runtime would bind $($Endpoint.Port), the default of the scheme, and the probe would knock there — write the port you mean"
    return $Endpoint
  }

  # The key is checked here, once, for every script: it is part of how the
  # endpoint is reached, and the caller already knows what to do with an error.
  if (Test-LlmControlChars (Get-LlmEnv 'MV_LLM_API_KEY')) {
    $Endpoint.Has = $false
    $Endpoint.Startable = $false
    $Endpoint.Error = 'llm: MV_LLM_API_KEY carries a control character (a byte below 0x20 — a key pasted across two lines of .env is the usual cause); an HTTP header cannot hold it, and sending it truncated would look like a wrong key. Put the value on one line'
    return $Endpoint
  }
  return $Endpoint
}

# --- probing -----------------------------------------------------------------

function Get-LlmHeaders {
  <#
    The key is only sent when there is one: the local server needs none, a cloud
    endpoint answers 401 without it. The value is never printed (ADR-009 p. 4).
  #>
  $key = Get-LlmEnv 'MV_LLM_API_KEY'
  if ($key) { return @{ Authorization = "Bearer $key" } }
  return @{}
}

$script:LlmProbeFault = $false

function Get-LlmProbeFault {
  # True when the LAST probe failed for a reason other than "nothing answered at
  # that address": a TLS failure, a refused proxy, a header .NET will not build.
  # `catch { return 0 }` used to fold all of those into "the process is not
  # running (make llm-up)", advice that is wrong in every one of them (Mi-6).
  return $script:LlmProbeFault
}

function Invoke-LlmProbeCode {
  <#
    The status of one path, 0 when nothing answered. Three decisions live here
    and in the bash twin:

    1. -SkipHeaderValidation: a key with a quote in it is sent as it is, exactly
       as curl sends it. Without it .NET refuses to build the request and throws
       a message that CONTAINS THE KEY (review Mi-3, Mi-4).
    2. -NoProxy: the probe measures the ENDPOINT, not the proxy in front of it.
       On the owner's machine HTTP_PROXY is set, Invoke-WebRequest honoured it
       and curl did not, and the proxy answered 503 "the model is loading" for an
       address where nothing listens at all (review M-2). Both implementations
       now go direct. This is deliberately NOT what the platform does — Go's
       http.ProxyFromEnvironment honours the variables — and the direction is
       chosen on purpose: a probe through a proxy can invent readiness, a direct
       probe can only fail to see an endpoint that needs one, and a false red is
       survivable where a false green is not. Noted in Docs/ops/runbook.md §3.
    3. 503 while the model loads is a state, not a failure (§6.3), so the status
       code is returned rather than thrown.
  #>
  param([Parameter(Mandatory)][string] $Base, [string] $Path = '', [int] $TimeoutSec = 5)
  $script:LlmProbeFault = $false
  try {
    $response = Invoke-WebRequest -Uri "$Base$Path" -Method Get -TimeoutSec $TimeoutSec `
      -Headers (Get-LlmHeaders) -SkipHttpErrorCheck -NoProxy -SkipHeaderValidation
    return [int]$response.StatusCode
  } catch {
    $exception = $_.Exception
    $isNoAnswer = $false
    if ($exception -is [System.Net.Http.HttpRequestException]) {
      # A socket that never connected is "nothing answered"; a TLS handshake that
      # failed is not, and curl agrees (exit 35/60 versus 6/7/28).
      $isNoAnswer = ($null -eq $exception.InnerException) -or
                    ($exception.InnerException -is [System.Net.Sockets.SocketException])
    } elseif ($exception -is [System.Threading.Tasks.TaskCanceledException] -or
              $exception -is [System.TimeoutException] -or
              $exception -is [System.OperationCanceledException]) {
      $isNoAnswer = $true
    }
    $script:LlmProbeFault = -not $isNoAnswer
    return 0
  }
}

function Invoke-LlmHealthProbe {
  <#
    The readiness code of one base URL, and the path it was decided by.

    /health belongs to llama.cpp, not to the contract. What the platform depends
    on is the OpenAI-compatible surface (ADR-005), and a server can serve that
    without /health: on the owner's stand a running server answers /v1/models
    with 200 and /health with 404, and the stack was reported down while it was
    up. A code that means "this is not an endpoint of mine" therefore falls back
    to the endpoint the contract names — a cloud endpoint answers 404, 401 or
    405 there for the same reason. 503 keeps meaning "the model is still
    loading", every other code keeps its own meaning (T-403; carried into the
    bench by T-404 review M-1, which is where its absence produced a false "not
    running").
  #>
  param([Parameter(Mandatory)][string] $Base, [int] $TimeoutSec = 5)
  $code = Invoke-LlmProbeCode -Base $Base -Path '/health' -TimeoutSec $TimeoutSec
  $fault = $script:LlmProbeFault
  $via = '/health'
  if ($code -in 401, 403, 404, 405, 501) {
    if ((Invoke-LlmProbeCode -Base $Base -Path '/v1/models' -TimeoutSec $TimeoutSec) -eq 200) {
      $via = '/v1/models'
      $code = 200
      $fault = $false
    }
  }
  $script:LlmProbeFault = $fault
  return [ordered]@{ Code = $code; Via = $via; Fault = $fault }
}

function Get-LlmModels {
  <#
    The model ids of /v1/models, comma separated.

    Read out of the raw body with the same regular expression the bash twin
    uses, rather than through ConvertFrom-Json: the two must return the same
    string for the same body, and "every id in the document" is a rule both can
    hold. The greedy sed of the old .sh collapsed the list to its LAST entry
    (review M-5) — which matters more than it looks, because T-404 made the
    model list the way to pin the version of a runtime that is not llama.cpp.
  #>
  param([Parameter(Mandatory)][string] $Base, [int] $TimeoutSec = 5)
  try {
    $response = Invoke-WebRequest -Uri "$Base/v1/models" -Method Get -TimeoutSec $TimeoutSec `
      -Headers (Get-LlmHeaders) -SkipHttpErrorCheck -NoProxy -SkipHeaderValidation
    $body = [string]$response.Content
  } catch {
    return ''
  }
  $ids = [regex]::Matches($body, '"id"\s*:\s*"([^"]*)"') | ForEach-Object { $_.Groups[1].Value }
  return ($ids -join ', ')
}

Export-ModuleMember -Function Get-LlmEnv, Protect-LlmSecret, Write-LlmLine,
Write-LlmFail, Write-LlmRetiredPortNote, Resolve-LlmEndpoint, Convert-LlmUrl, New-LlmEndpoint,
Complete-LlmEndpoint, Test-LlmHostLocal, Test-LlmHostTrusted, Test-LlmPrintableAscii,
Test-LlmControlChars, Get-LlmHeaders, Invoke-LlmProbeCode, Invoke-LlmHealthProbe,
Get-LlmModels, Get-LlmProbeFault
