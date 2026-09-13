#!/usr/bin/env bash
# =============================================================================
# scripts/lib/llm-endpoint.sh — the ONE rule that turns MV_LLM_URL into an
# address, shared by scripts/llm-server.sh and scripts/llm-bench.sh
# (infrastructure.md v0.3 §6.1-§6.4, ADR-005 add. 2 p. 2, p. 3; T-404)
# =============================================================================
# Sourced, never executed. The PowerShell twin is scripts/lib/LlmEndpoint.psm1
# and the two files are one rule written twice: a divergence between them is a
# review finding, not a detail. Every message printed from here exists in both.
#
# WHY THIS FILE EXISTS. T-404 made the ADDRESS have one source of truth,
# MV_LLM_URL, and review #1 showed that this was not enough: the RULE that
# turns the variable into an address lived in four scripts with three different
# behaviours (the bench trimmed a trailing /v1 and the server did not; the
# server rewrote host.docker.internal and the bench did not; the server fell
# back from /health to /v1/models and the bench did not). On the owner's stand
# that combination made `make bench` say "the LLM process is not running" to a
# server that was answering. One variable with four rules is still two sources
# of truth, so the rule moved here.
#
# WHAT THE RULE IS, in one paragraph. From MV_LLM_URL (MV_OLLAMA_URL when
# MV_LLM_PROVIDER=ollama) come three things: the address AS CONFIGURED (what is
# printed, and what a remote endpoint is probed at), the address REACHABLE FROM
# THIS MACHINE (the same, with the host.docker.internal alias rewritten to
# 127.0.0.1 — that name resolves inside a container and not on the host), and
# the PORT the local llama-server binds. A trailing /v1 is cut off: the variable
# holds the BASE address, and vendor documentation usually hands out the address
# with /v1 on the end (orchestrator decision, journal 2026-09-11). Nothing else
# about the value is rewritten: a real remote address is probed as it is, or the
# check would be measuring the wrong host.
# =============================================================================

# --- reading the environment -------------------------------------------------

# llm_trim strips leading and trailing whitespace, and nothing else: a Windows
# path in MV_LLM_BIN may legitimately contain spaces in the middle.
#
# ASCII whitespace only — space, TAB, LF, VT, FF, CR — which is what LC_ALL=C
# makes of [:space:]. In a UTF-8 locale the class follows the locale, and .NET's
# String.Trim() strips every Unicode space: MV_LLM_URL ending in a no-break space
# U+00A0 was refused here as "outside printable ASCII" while the PowerShell twin
# trimmed it away and reported 200 (found by the parity stand, T-405). Anything
# beyond ASCII is left in place for the printable-ASCII refusal of llm_parse_url.
llm_trim() {
  local LC_ALL=C
  local s=${1-}
  s=${s#"${s%%[![:space:]]*}"}
  s=${s%"${s##*[![:space:]]}"}
  printf '%s' "$s"
}

# llm_env_or is the only way this file reads a tooling variable. Blank means
# "not configured" and yields the default, in BOTH implementations: PowerShell's
# ?? operator does not react to an empty string, so `MV_LLM_NUM_CTX=` used to
# build `--ctx-size 0` there while bash substituted 8192 (review Mi-9, Mi-10).
llm_env_or() {
  local value
  value=$(llm_trim "${!1-}")
  if [ -z "$value" ]; then printf '%s' "${2-}"; else printf '%s' "$value"; fi
}

# --- character classes -------------------------------------------------------
#
# Both predicates run under LC_ALL=C on purpose: a bracket range is a range of
# BYTES there, and the answer no longer depends on the locale of whoever ran
# make. The PowerShell twin asks the same two questions of UTF-16 characters,
# and for any value that is valid UTF-8 the two answers agree.

# llm_not_printable_ascii is true when the value carries anything outside
# 0x20-0x7E. An invisible character — a soft hyphen U+00AD, a zero width space
# U+200B — is the normal result of copying an address out of vendor
# documentation, out of a PDF or out of a wiki, and it used to give the two
# implementations OPPOSITE verdicts: .NET compares strings by the rules of the
# culture, which SKIPS such characters, so the PowerShell twin trimmed a /v1
# that was not at the end and reported 200 for an address the variable did not
# hold, while bash compared bytes and reported 404 (review M-1). Both
# implementations now refuse the value instead: the platform's HTTP client
# would encode it in a third way, and a wrapper that works where the platform
# breaks is the whole defect this file exists to prevent.
llm_not_printable_ascii() {
  local LC_ALL=C
  case "${1-}" in
  *[!\ -~]*) return 0 ;;
  esac
  return 1
}

# llm_has_control_char is true when the value carries a byte below 0x20 (CR, LF,
# TAB in the middle, NUL is impossible in a shell variable). A key pasted across
# two lines of .env is a normal Windows accident, and it used to arrive at the
# server TRUNCATED, with the tail of the value sent as a header of its own,
# while bash could not build the request at all (review M-3).
llm_has_control_char() {
  local LC_ALL=C
  case "${1-}" in
  *[$'\001'-$'\037']*) return 0 ;;
  esac
  return 1
}

# --- printing ----------------------------------------------------------------
#
# Parity rule, chosen here and obeyed by both implementations: everything a
# healthy run prints goes to stdout (including WARNING lines), and only a
# message that ends the run goes to stderr. The .sh used to send its SEC-15
# warning to stderr while the .ps1 sent it to stdout, which made the two outputs
# differ in ordering under 2>&1 for no reason (review Mi-12).

# llm_redact hides the key in anything printed. It only redacts a value of at
# least LLM_REDACT_MIN characters: MV_LLM_API_KEY=8888 used to turn every line
# into `probing http://127.0.0.1:***`, so the diagnostic became unreadable
# exactly when it was needed (review Mi-10). A value that short is not a key of
# any endpoint, and the same threshold is used by the PowerShell twin.
LLM_REDACT_MIN=8
llm_redact() {
  local text=${1-} key
  key=$(llm_trim "${MV_LLM_API_KEY-}")
  if [ -n "$key" ] && [ "${#key}" -ge "$LLM_REDACT_MIN" ]; then
    text=${text//"$key"/'***'}
  fi
  printf '%s' "$text"
}

llm_say() { printf '%s\n' "$(llm_redact "$*")"; }
llm_fail() { printf '%s\n' "$(llm_redact "$*")" >&2; }

# llm_note_retired_port is Mi-1: the retirement of MV_LLM_PORT is declared in
# the manifest, but only `mvctl env check --env` ever printed it, and no make
# target passes that flag. The scripts run with .env already loaded, and this is
# the place where an operator with a stale .env is looking for an explanation.
llm_note_retired_port() {
  if [ -n "$(llm_env_or MV_LLM_PORT '')" ]; then
    llm_say "llm: MV_LLM_PORT=$(llm_env_or MV_LLM_PORT '') is retired and ignored — the port comes from $1 (T-404); remove the line from .env"
  fi
}

# --- the rule ----------------------------------------------------------------
#
# llm_parse_url fills the LLM_EP_* variables from one address string. Returns 1
# and sets LLM_EP_ERROR when the value is not an address the platform could use;
# the caller decides whether that is fatal (it always is for `up`).
#
# Deliberately narrow: this is the address of an OpenAI-compatible endpoint, not
# an arbitrary URI.

LLM_EP_SCHEME=''
LLM_EP_HOST=''   # no brackets, even for an IPv6 literal
LLM_EP_PORT=''
LLM_EP_PATH=''
LLM_EP_URL=''    # the address as configured, canonical: what is printed
LLM_EP_PROBE=''  # the same address as reachable from THIS machine
LLM_EP_RAW=''    # the configured value, trimmed, NOT masked: an accepted value may hold an @ after the first / (llm_endpoint_judge does not print it then)
LLM_EP_VAR=''
LLM_EP_PROVIDER=''
LLM_EP_HAS=0
LLM_EP_STARTABLE=0
LLM_EP_TRUSTED=0
LLM_EP_TRIMMED_V1=0
LLM_EP_PORT_EXPLICIT=0 # the port is printed back (it differs from the scheme default)
LLM_EP_PORT_WRITTEN=0  # the value carries a port at all, :80 included (T-450)
LLM_EP_CLASS=''        # local | cloud | invalid — llm_endpoint_judge (T-450)
LLM_EP_KIND=''         # why: loopback, localhost, docker-host, service, private, ...
LLM_EP_ERROR=''

llm_parse_url() {
  local raw=$1 scheme rest hostport path host port explicit=0 written=0
  LLM_EP_SCHEME='' LLM_EP_HOST='' LLM_EP_PORT='' LLM_EP_PATH=''
  LLM_EP_URL='' LLM_EP_PROBE='' LLM_EP_RAW='' LLM_EP_TRIMMED_V1=0
  LLM_EP_PORT_EXPLICIT=0 LLM_EP_PORT_WRITTEN=0 LLM_EP_CLASS='' LLM_EP_KIND=''
  LLM_EP_ERROR=''

  raw=$(llm_trim "$raw")
  # $shown is the value as every refusal of this function prints it. Four of
  # them — the query, a character outside printable ASCII, no scheme, a scheme
  # other than http/https — come BEFORE the check of the @ and used to print
  # user:pass@ with the rest, into the console, the log of CI and the report of
  # compose-lint (T-450 review #2 Mi-R2-1). The later ones print $shown too: a
  # password with a bare / (u:x/y@h) leaves no @ in front of the first /, and
  # the sentence about the port would print all of it. What such a sentence
  # still names — the port 'x' or the host — is the host and the port of that
  # value to every URL parser, Go's included, not user information.
  # Deliberately wider than the parser: a sentence that hides too much costs
  # nothing, one that shows a password is a leak. So:
  #   - the value is cut at the first ? or #, as the query refusal prints it;
  #   - the scheme is kept only when the part in front of the first :// holds
  #     no @; everything else up to the LAST @ of what is left becomes …@,
  #     whatever / or :// stands in front of that @;
  #   - when the cut-off query or fragment holds an @, the password may have
  #     had a bare ? or # in it: only the scheme is printed.
  # Plain parameter expansion, no fork — the table runs this for every row.
  # The .psm1 does the same, statement for statement.
  local shown shown_scheme='' shown_rest unqueried=$raw
  case "$raw" in
  *'?'* | *'#'*) unqueried=${raw%%[?#]*} ;;
  esac
  shown_rest=$unqueried
  case "$unqueried" in
  *://*)
    shown_scheme=${unqueried%%://*}
    case "$shown_scheme" in
    *@*) shown_scheme='' ;;
    *)
      shown_scheme="$shown_scheme://"
      shown_rest=${unqueried#*://}
      ;;
    esac
    ;;
  esac
  case "${raw#"$unqueried"}" in
  *@*) shown=$shown_scheme ;;
  *)
    case "$shown_rest" in
    *@*) shown="$shown_scheme…@${shown_rest##*@}" ;;
    *) shown=$unqueried ;;
    esac
    ;;
  esac
  # A query or a fragment is refused FIRST, and nothing after the ? or the # is
  # echoed back by this or any later sentence: `?api_key=…` is how some vendors
  # document their address, and the sentence reaches the console, the log of CI
  # and the report of compose-lint (T-450 review #1 N-2). Finding a ? or a # is
  # a comparison of one ASCII character, which an invisible one cannot break.
  case "$raw" in
  *'?'* | *'#'*)
    LLM_EP_ERROR="$LLM_EP_VAR=$shown… carries a query or a fragment (the rest of the value is not printed: it may hold a key); the value must be scheme://host[:port][/path]"
    return 1
    ;;
  esac
  # Checked before anything else is compared, because every comparison below is
  # what an invisible character breaks (review M-1).
  if llm_not_printable_ascii "$raw"; then
    LLM_EP_ERROR="$LLM_EP_VAR=$shown carries a character outside printable ASCII; an address copied from documentation often brings an invisible one with it (a soft hyphen U+00AD, a zero width space U+200B) — retype the value by hand"
    return 1
  fi

  # A value without a scheme is REFUSED in both implementations. The .sh used to
  # accept 127.0.0.1:8888 and the .ps1 threw a raw error record at it (review
  # Mi-11); accepting it is the worse of the two, because the platform's Go
  # client cannot: http.Client on such a value fails with "unsupported protocol
  # scheme". A wrapper that works where the platform breaks reports a green LLM
  # nobody can call.
  case "$raw" in
  *://*)
    scheme=${raw%%://*}
    rest=${raw#*://}
    ;;
  '')
    LLM_EP_ERROR="$LLM_EP_VAR is empty — the platform has no LLM address (shared/env: a variable set to an empty value overrides the default)"
    return 1
    ;;
  *)
    LLM_EP_ERROR="$LLM_EP_VAR=$shown has no scheme; write it as http://host:port (the platform's HTTP client refuses a value without one)"
    return 1
    ;;
  esac

  scheme=$(printf '%s' "$scheme" | tr '[:upper:]' '[:lower:]')
  case "$scheme" in
  http | https) ;;
  *)
    # The part in front of the first :// is printed too: an @ in it is
    # user information as well (u:p@h://x), cut the same way as in $shown.
    case "$scheme" in
    *@*) scheme="…@${scheme##*@}" ;;
    esac
    LLM_EP_ERROR="$LLM_EP_VAR=$shown uses scheme '$scheme'; only http and https are addresses of an OpenAI-compatible endpoint"
    return 1
    ;;
  esac

  hostport=${rest%%/*}
  if [ "$rest" != "$hostport" ]; then path="/${rest#*/}"; else path=''; fi

  # userinfo. The .sh used to keep `user:s3cr3t@` as part of the host, which
  # made a loopback address look remote AND printed the password to the console
  # and into every captured log (review M-4). Masking it was only half a fix:
  # the credentials were then DROPPED, so the probe went out without them while
  # Go's net/http builds a Basic header from exactly that userinfo — the probe
  # and the platform would ask different questions of the same endpoint (review
  # Mi-4). The value is refused instead, and it is not echoed back: it is the
  # one shape of this variable that carries a secret.
  case "$hostport" in
  *@*)
    LLM_EP_ERROR="$LLM_EP_VAR carries user information in front of the host; the key of an endpoint lives in MV_LLM_API_KEY, not in the address (the value is not printed here because it holds a secret)"
    return 1
    ;;
  esac

  # A percent sign in the host is an escaped character or the zone of an IPv6
  # literal (fe80::1%25eth0). Go's url.Parse unescapes the first, so ol%61ma
  # would be `ollama` to the platform and a name with a % to this parser; the
  # zone names an interface of one machine. Neither is how an address of an
  # endpoint is written, and both are refused rather than classified (T-450).
  # Checked after the user information, which may hold a secret with a %.
  case "$hostport" in
  *%*)
    LLM_EP_ERROR="$LLM_EP_VAR=$shown carries a percent sign in the host (an escaped character or the zone of an IPv6 literal); write the host as it is"
    return 1
    ;;
  esac

  # An IPv6 literal keeps its brackets in the URL and loses them in $host, so
  # that '::1' compares equal to the entry in the loopback list. [uri].Host in
  # .NET keeps the brackets, which is exactly why the .ps1 called [::1] a cloud
  # address, refused to start a local server on it and printed a false gateway
  # warning (review M-3). DnsSafeHost — the bracket-less form — is the one both
  # implementations classify on.
  #
  # The shape of this block follows the .ps1 statement for statement (the FIRST
  # closing bracket, then the tail), because the two used to answer
  # http://[::1]x with two different sentences (review Mi-9).
  local tail bracketed=0
  case "$hostport" in
  \[*)
    bracketed=1
    case "$hostport" in
    *\]*) ;;
    *)
      LLM_EP_ERROR="$LLM_EP_VAR=$shown has an unclosed IPv6 literal"
      return 1
      ;;
    esac
    host=${hostport%%\]*}
    host=${host#\[}
    tail=${hostport#*\]}
    case "$tail" in
    :*) port=${tail#:} written=1 ;;
    '') port='' ;;
    *)
      LLM_EP_ERROR="$LLM_EP_VAR=$shown has trailing characters after the IPv6 literal"
      return 1
      ;;
    esac
    ;;
  *:*)
    host=${hostport%:*}
    port=${hostport##*:}
    written=1
    ;;
  *)
    host=$hostport
    port=''
    ;;
  esac
  if [ "$bracketed" = 0 ]; then
    case "$host" in
    *:*)
      LLM_EP_ERROR="$LLM_EP_VAR=$shown looks like a bare IPv6 literal; bracket it: http://[::1]:8888"
      return 1
      ;;
    esac
    # The printable characters Go's url.Parse refuses in a host (`invalid
    # character in host name`) and the checks above have not already taken: a
    # space, a backslash, ^, `, {, | and }. Classified, http://localhost\evil.com
    # was the cloud here and a refusal on the platform, and the operator read
    # "not a local address" where the value was simply mistyped (T-450 review #1
    # Mi-2). The set is Go's, not a stricter one: ~ ! $ & ' ( ) * + , ; = < > "
    # pass url.Parse and stay names — of the cloud, having no service shape.
    case "$host" in
    *[' \^`{|}']*)
      LLM_EP_ERROR="$LLM_EP_VAR=$shown has a character in the host that an address cannot hold (a space, a backslash, a caret, a backtick, a brace or a vertical bar); the platform's URL parser refuses it"
      return 1
      ;;
    esac
  fi
  host=$(printf '%s' "$host" | tr '[:upper:]' '[:lower:]')

  if [ -z "$host" ]; then
    LLM_EP_ERROR="$LLM_EP_VAR=$shown has no host"
    return 1
  fi

  # Brackets hold an IPv6 address and nothing else: [localhost] used to pass
  # here as the name localhost, while Go's client refuses it (T-450).
  if [ "$bracketed" = 1 ] && ! llm_ipv6_hex "$host"; then
    LLM_EP_ERROR="$LLM_EP_VAR=$shown has a malformed IPv6 literal [$host]"
    return 1
  fi

  # `host:` with nothing after the colon is not "no port": Go refuses it, and
  # a cloud address written that way used to pass here with the default of the
  # scheme (T-450).
  if [ "$written" = 1 ] && [ -z "$port" ]; then
    LLM_EP_ERROR="$LLM_EP_VAR=$shown has an empty port after the colon"
    return 1
  fi

  if [ -n "$port" ]; then
    case "$port" in
    '' | *[!0-9]*)
      LLM_EP_ERROR="$LLM_EP_VAR=$shown has a non-numeric port '$port'"
      return 1
      ;;
    esac
    # Leading zeros go first: Go reads :008888 as 8888, and a port of twenty
    # digits used to overflow `[ -gt ]` here instead of being out of range.
    local digits=$port
    while [ "${#digits}" -gt 1 ] && [ "${digits#0}" != "$digits" ]; do
      digits=${digits#0}
    done
    if [ "${#digits}" -gt 5 ] || [ "$digits" -lt 1 ] || [ "$digits" -gt 65535 ]; then
      LLM_EP_ERROR="$LLM_EP_VAR=$shown has port $port, which is outside 1-65535"
      return 1
    fi
    port=$digits
    explicit=1
  fi
  if [ -z "$port" ]; then
    case "$scheme" in
    https) port=443 ;;
    *) port=80 ;;
    esac
  fi
  # A port equal to the scheme default is not printed back, so that the two
  # implementations agree: .NET reports IsDefaultPort for http://host:80 too.
  if { [ "$scheme" = http ] && [ "$port" = 80 ]; } ||
    { [ "$scheme" = https ] && [ "$port" = 443 ]; }; then
    explicit=0
  fi

  # The trailing /v1. MV_LLM_URL is the BASE address; vendors hand out the
  # address with /v1 on the end, and without this both scripts would ask for
  # /v1/v1/models and call a living endpoint dead (review Mi-2, orchestrator
  # decision). The rule already existed in llm-bench.*; here it is the only copy.
  path=${path%/}
  case "$path" in
  */v1)
    path=${path%/v1}
    LLM_EP_TRIMMED_V1=1
    ;;
  esac
  path=${path%/}

  local shown probe_host authority probe_authority
  shown=$host
  case "$host" in
  *:*) shown="[$host]" ;;
  esac
  authority=$shown
  [ "$explicit" = 1 ] && authority="$shown:$port"

  # host.docker.internal is how a container reaches this machine; from the host
  # the name does not resolve. Only that alias is rewritten.
  probe_host=$shown
  [ "$host" = host.docker.internal ] && probe_host=127.0.0.1
  probe_authority=$probe_host
  [ "$explicit" = 1 ] && probe_authority="$probe_host:$port"

  LLM_EP_SCHEME=$scheme
  LLM_EP_HOST=$host
  LLM_EP_PORT=$port
  LLM_EP_PATH=$path
  LLM_EP_URL="$scheme://$authority$path"
  LLM_EP_PROBE="$scheme://$probe_authority$path"
  LLM_EP_RAW=$raw
  LLM_EP_PORT_EXPLICIT=$explicit
  LLM_EP_PORT_WRITTEN=$written
  return 0
}

# --- the local address (T-450) ----------------------------------------------
#
# ONE answer to "is this address local", for the scripts, for compose-lint and
# for the cloud gate of the platform (internal/llm, IsLocalEndpoint). It used to
# be written three times, three ways: this file trusted RFC 1918 but not a
# compose service, compose-lint trusted a service of its own file and whatever
# Python calls private (documentation ranges included), and the platform
# neither — so MV_OLLAMA_URL=http://ollama:11434 passed the linter and stopped
# the platform. The rule is the decision of system-architect#1, and its cases
# live in testdata/llm/local-endpoints.tsv: every implementation is tested
# against that file, and a case is added there, not here.
#
#   local   — loopback 127.0.0.0/8 and ::1; localhost and *.localhost;
#             host.docker.internal; a one-word name (a compose service:
#             ollama, core); RFC 1918; link-local 169.254.0.0/16 and
#             fe80::/10; IPv6 ULA fc00::/7; the IPv4-mapped form of each;
#   invalid — the any-address 0.0.0.0 and :: (a server listens there, a client
#             cannot call it); a local host without a port; a port outside
#             1-65535; anything llm_parse_url refuses;
#   cloud   — everything else: a public address, any name with a dot (.local
#             included), CGNAT 100.64.0.0/10, the documentation ranges, the
#             numeric spellings 2130706433 and 0x7f000001, and a dotted quad
#             with the root dot (127.0.0.1.), which is a name to DNS. Names are
#             never resolved: the CLASS must not depend on the DNS of the
#             moment. Where a request to a one-word name or to *.localhost ends
#             up still does — Docs/ops/runbook.md §3 names that risk.
#
# Every helper below runs under LC_ALL=C (byte ranges, not the locale) and
# returns 0 whatever the answer, so that a caller under `set -e` —
# compose-lint is one — cannot be stopped by a verdict.

# The strict dotted quad: four decimal octets, no leading zero. 127.1, 0177.0.0.1
# and 127.0.0.01 are not addresses to Go's netip, and here they are names with
# dots — the cloud.
LLM_IPV4_OCTET='(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])'
LLM_IPV4_RE="^$LLM_IPV4_OCTET\\.$LLM_IPV4_OCTET\\.$LLM_IPV4_OCTET\\.$LLM_IPV4_OCTET\$"

# llm_ipv4_class A B C D sets LLM_HOST_CLASS and LLM_HOST_KIND for an address
# given as four decimal octets.
llm_ipv4_class() {
  local a=$1 b=$2 c=$3 d=$4
  LLM_HOST_CLASS=cloud LLM_HOST_KIND=public
  if [ "$a" = 0 ] && [ "$b" = 0 ] && [ "$c" = 0 ] && [ "$d" = 0 ]; then
    LLM_HOST_CLASS=invalid LLM_HOST_KIND=unspecified
  elif [ "$a" = 127 ]; then
    LLM_HOST_CLASS=local LLM_HOST_KIND=loopback
  elif [ "$a" = 10 ] || { [ "$a" = 172 ] && [ "$b" -ge 16 ] && [ "$b" -le 31 ]; } ||
    { [ "$a" = 192 ] && [ "$b" = 168 ]; }; then
    LLM_HOST_CLASS=local LLM_HOST_KIND=private
  elif [ "$a" = 169 ] && [ "$b" = 254 ]; then
    LLM_HOST_CLASS=local LLM_HOST_KIND=link-local
  fi
  return 0
}

# llm_ipv6_groups appends the colon-separated groups of $1 to LLM_IP6_GROUPS;
# returns 1 on a group that is not 1-4 hex digits. An empty $1 has no groups.
LLM_IP6_GROUPS=()
llm_ipv6_groups() {
  local LC_ALL=C s=${1-} g
  [ -z "$s" ] && return 0
  while :; do
    g=${s%%:*}
    [[ $g =~ ^[0-9a-f]{1,4}$ ]] || return 1
    LLM_IP6_GROUPS+=("$g")
    [ "$s" = "$g" ] && return 0
    s=${s#*:}
  done
}

# llm_ipv6_hex sets LLM_IP6_HEX to the 32 hex digits of an IPv6 address written
# in lower case without brackets, or returns 1 when $1 is not one. An IPv4 tail
# (::ffff:127.0.0.1) is accepted in the last 32 bits, as netip accepts it.
LLM_IP6_HEX=''
llm_ipv6_hex() {
  local LC_ALL=C s=${1-} head tail v4 n i zeros hex='' g
  local -a all=()
  LLM_IP6_HEX=''
  case "$s" in
  '' | *[!0-9a-f:.]*) return 1 ;;
  *:*) ;;
  *) return 1 ;;
  esac
  case "$s" in
  *.*)
    v4=${s##*:}
    [[ $v4 =~ $LLM_IPV4_RE ]] || return 1
    printf -v g '%02x%02x:%02x%02x' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}" "${BASH_REMATCH[4]}"
    s="${s%:*}:$g"
    ;;
  esac
  case "$s" in
  *:::*) return 1 ;;
  esac
  LLM_IP6_GROUPS=()
  if [ "${s#*::}" != "$s" ]; then
    head=${s%%::*}
    tail=${s#*::}
    case "$tail" in
    *::*) return 1 ;;
    esac
    llm_ipv6_groups "$head" || return 1
    n=${#LLM_IP6_GROUPS[@]}
    all=("${LLM_IP6_GROUPS[@]}")
    LLM_IP6_GROUPS=()
    llm_ipv6_groups "$tail" || return 1
    zeros=$((8 - n - ${#LLM_IP6_GROUPS[@]}))
    [ "$zeros" -ge 1 ] || return 1
    for ((i = 0; i < zeros; i++)); do all+=(0); done
    all+=("${LLM_IP6_GROUPS[@]}")
  else
    llm_ipv6_groups "$s" || return 1
    [ "${#LLM_IP6_GROUPS[@]}" = 8 ] || return 1
    all=("${LLM_IP6_GROUPS[@]}")
  fi
  for g in "${all[@]}"; do
    g="000$g"
    hex+=${g: -4}
  done
  LLM_IP6_HEX=$hex
  return 0
}

# llm_host_class sets LLM_HOST_CLASS (local | cloud | invalid) and LLM_HOST_KIND
# for a host as llm_parse_url leaves it: lower case, no brackets, no port.
#
# A trailing dot is the root of DNS, and it is dropped for the reserved names
# only: localhost. is localhost (RFC 6761), api.localhost. is *.localhost and
# host.docker.internal. is the alias. It is NOT dropped
#   - for a dotted quad: 127.0.0.1. is not an address to Go's netip.ParseAddr,
#     so the platform's client, curl and Invoke-WebRequest all hand it to DNS as
#     a name, and a resolver that answers for a name it does not know would take
#     the request wherever it likes — the cloud, like 127.1 (decision of
#     system-architect#1 on T-450 review #1 M-1);
#   - for the rule of one word: `ollama.` is an absolute name that skips the
#     search list of the container, a top-level domain and not a service.
LLM_HOST_CLASS=''
LLM_HOST_KIND=''
llm_host_class() {
  local LC_ALL=C host=${1-} bare hex
  LLM_HOST_CLASS=cloud LLM_HOST_KIND=name
  case "$host" in
  *:*)
    if ! llm_ipv6_hex "$host"; then
      LLM_HOST_CLASS=invalid LLM_HOST_KIND=malformed
      return 0
    fi
    hex=$LLM_IP6_HEX
    LLM_HOST_KIND=public
    case "$hex" in
    00000000000000000000000000000000) LLM_HOST_CLASS=invalid LLM_HOST_KIND=unspecified ;;
    00000000000000000000000000000001) LLM_HOST_CLASS=local LLM_HOST_KIND=loopback ;;
    00000000000000000000ffff*)
      llm_ipv4_class "$((16#${hex:24:2}))" "$((16#${hex:26:2}))" "$((16#${hex:28:2}))" "$((16#${hex:30:2}))"
      ;;
    fe[89ab]*) LLM_HOST_CLASS=local LLM_HOST_KIND=link-local ;;
    f[cd]*) LLM_HOST_CLASS=local LLM_HOST_KIND=ula ;;
    esac
    return 0
    ;;
  esac
  bare=${host%.}
  if [[ $host =~ $LLM_IPV4_RE ]]; then
    llm_ipv4_class "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}" "${BASH_REMATCH[4]}"
    return 0
  fi
  case "$bare" in
  localhost) LLM_HOST_CLASS=local LLM_HOST_KIND=localhost ;;
  host.docker.internal) LLM_HOST_CLASS=local LLM_HOST_KIND=docker-host ;;
  *)
    if [[ $bare =~ ^([a-z0-9_-]+\.)+localhost$ ]]; then
      LLM_HOST_CLASS=local LLM_HOST_KIND=localhost-sub
    elif [ "$bare" = "$host" ] && [[ $host =~ ^[a-z][a-z0-9_-]*$ ]]; then
      # One word, starting with a letter: 2130706433 and 0x7f000001 are the
      # numeric spellings of an address, not names of a service.
      LLM_HOST_CLASS=local LLM_HOST_KIND=service
    fi
    ;;
  esac
  return 0
}

# llm_endpoint_judge classifies the address llm_parse_url has just accepted:
# LLM_EP_CLASS, LLM_EP_KIND and, for invalid, LLM_EP_ERROR. Returns 1 when the
# address is invalid. The one place both llm_endpoint_finish and
# llm_endpoint_classify take the answer from.
llm_endpoint_judge() {
  llm_host_class "$LLM_EP_HOST"
  LLM_EP_CLASS=$LLM_HOST_CLASS
  LLM_EP_KIND=$LLM_HOST_KIND
  # An @ anywhere in the value — not only in front of the host, which
  # llm_parse_url refuses — may be the end of a password with a bare / in it:
  # http://pw/x@127.0.0.1:8888 parses as the host pw without a port, and the
  # sentence about the port used to print the whole value, pw included. Such a
  # value is printed by its scheme alone and its host is not named, in every
  # sentence below (C-15 v1.5 "Печать значения", the rule of aboutTheHost in
  # internal/llm, T-463). Without an @ the sentences are what they were.
  local masked=0
  case "$LLM_EP_RAW" in
  *@*) masked=1 ;;
  esac
  case "$LLM_EP_CLASS" in
  invalid)
    # The sentence follows the kind, not the class (T-450 review #1 N-5): today
    # llm_parse_url refuses a malformed host before it gets here, and a change
    # of that order must not turn its refusal into talk of the any-address.
    case "$LLM_EP_KIND" in
    unspecified)
      if [ "$masked" = 1 ]; then
        LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_SCHEME://… names the any-address: a server listens there, a client cannot call it — write 127.0.0.1 or the address of the host, with the port (the rest of the value and its host are not printed: the value holds an @, and what stands in front of it may be a key)"
      else
        LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_RAW names $LLM_EP_HOST, the any-address: a server listens there, a client cannot call it — write 127.0.0.1 or the address of the host, with the port"
      fi
      ;;
    *)
      if [ "$masked" = 1 ]; then
        LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_SCHEME://… names a host which is not an address a client can call ($LLM_EP_KIND) (the rest of the value and its host are not printed: the value holds an @, and what stands in front of it may be a key)"
      else
        LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_RAW names $LLM_EP_HOST, which is not an address a client can call ($LLM_EP_KIND)"
      fi
      ;;
    esac
    return 1
    ;;
  local)
    # A local endpoint without a port is not "the port is 80": it is a port
    # nobody chose. The scheme default was passed to llama-server as --port 80
    # and knocked on by the probe, and the operator who wrote the value meant
    # something else (review Mi-12). Since T-450 this holds for every local
    # address, not only for the startable ones; a cloud address keeps the
    # default of the scheme — there 443 and 80 are what the vendor documents.
    if [ "$LLM_EP_PORT_WRITTEN" != 1 ]; then
      LLM_EP_CLASS=invalid
      # $LLM_EP_PORT here is the default of the scheme, never a part of the
      # value, so the masked sentences keep it.
      case "$LLM_EP_KIND" in
      loopback | localhost | docker-host)
        if [ "$masked" = 1 ]; then
          LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_SCHEME://… has no port; the local runtime would bind $LLM_EP_PORT, the default of the scheme, and the probe would knock there — write the port you mean (the rest of the value and its host are not printed: the value holds an @, and what stands in front of it may be a key)"
        else
          LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_RAW has no port; the local runtime would bind $LLM_EP_PORT, the default of the scheme, and the probe would knock there — write the port you mean"
        fi
        ;;
      *)
        if [ "$masked" = 1 ]; then
          LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_SCHEME://… has no port; a local address is written with the port its runtime listens on, and $LLM_EP_PORT, the default of the scheme, is a port nobody chose (the rest of the value and its host are not printed: the value holds an @, and what stands in front of it may be a key)"
        else
          LLM_EP_ERROR="$LLM_EP_VAR=$LLM_EP_RAW has no port; a local address is written with the port its runtime listens on, and $LLM_EP_PORT, the default of the scheme, is a port nobody chose"
        fi
        ;;
      esac
      return 1
    fi
    ;;
  esac
  return 0
}

# llm_endpoint_classify URL [VAR] is the rule as a function of one value: it
# sets LLM_CLASS (local | cloud | invalid), LLM_CLASS_KIND, LLM_CLASS_HOST and,
# for invalid, LLM_CLASS_ERROR (a sentence naming VAR, URL by default). No other
# variable of this file changes. What testdata/llm/local-endpoints.tsv is
# checked against, and what scripts/compose-lint.sh calls for rule 6.
LLM_CLASS=''
LLM_CLASS_KIND=''
LLM_CLASS_HOST=''
LLM_CLASS_ERROR=''
llm_endpoint_classify() {
  local LLM_EP_VAR=${2:-URL}
  local LLM_EP_SCHEME='' LLM_EP_HOST='' LLM_EP_PORT='' LLM_EP_PATH='' LLM_EP_URL=''
  local LLM_EP_PROBE='' LLM_EP_RAW='' LLM_EP_TRIMMED_V1=0 LLM_EP_PORT_EXPLICIT=0
  local LLM_EP_PORT_WRITTEN=0 LLM_EP_CLASS='' LLM_EP_KIND='' LLM_EP_ERROR=''
  LLM_CLASS=invalid LLM_CLASS_KIND=unparsed LLM_CLASS_HOST='' LLM_CLASS_ERROR=''
  if ! llm_parse_url "${1-}"; then
    LLM_CLASS_ERROR=$LLM_EP_ERROR
    return 0
  fi
  llm_endpoint_judge || :
  LLM_CLASS=$LLM_EP_CLASS
  LLM_CLASS_KIND=$LLM_EP_KIND
  LLM_CLASS_HOST=$LLM_EP_HOST
  LLM_CLASS_ERROR=$LLM_EP_ERROR
  return 0
}

# llm_provider_ok holds the same closed set as the manifest declares for
# MV_LLM_PROVIDER (shared/env/vars.go, OneOf). An unlisted value used to pass
# through both wrappers in silence, which cost the pin and the VRAM report and
# made `up` explain itself with the wrong reason, while `mvctl env check --env`
# refused the very same value (review Mi-11): two readings of one variable is
# the defect this file exists to remove.
llm_provider_ok() {
  case "$1" in
  openai_compat | ollama | anthropic | recorded | fake) return 0 ;;
  esac
  return 1
}

# llm_endpoint_finish is the tail both resolvers share, so that the rule cannot
# differ between `make llm-health` and `make bench`.
llm_endpoint_finish() {
  LLM_EP_HAS=1
  if ! llm_endpoint_judge; then
    LLM_EP_HAS=0
    LLM_EP_ERROR="llm: $LLM_EP_ERROR"
    return 1
  fi

  # Two questions, one answer each (T-450):
  #   trusted   — "is this address outside the trusted network", the question
  #               of the cloud gate (ADR-005 add. 2 p. 3): every local address;
  #   startable — "would llama-server started here own this address": loopback,
  #               localhost and the container alias only, written without the
  #               root dot (the probe must reach them from this machine), and
  #               only for the runtime this script starts. A LAN server or a
  #               service of compose is trusted and not ours to start.
  # The any-address 0.0.0.0 and :: used to be startable; they are invalid now.
  # Every loopback spelling is startable, [::ffff:127.0.0.1] and
  # [0:0:0:0:0:0:0:1] included: they were the cloud before T-450, and the
  # change is listed in infrastructure.md §6.3.1 (review #1 N-1).
  if [ "$LLM_EP_CLASS" = local ]; then
    LLM_EP_TRUSTED=1
  fi
  case "$LLM_EP_HOST" in
  *.) ;;
  *)
    case "$LLM_EP_KIND" in
    loopback | localhost | docker-host)
      if [ "$LLM_EP_PROVIDER" = openai_compat ]; then
        LLM_EP_STARTABLE=1
      fi
      ;;
    esac
    ;;
  esac

  # The key is checked here, once, for every script: it is part of how the
  # endpoint is reached, and the caller already knows what to do with an error.
  if llm_has_control_char "$(llm_trim "${MV_LLM_API_KEY-}")"; then
    LLM_EP_HAS=0
    LLM_EP_STARTABLE=0
    LLM_EP_ERROR='llm: MV_LLM_API_KEY carries a control character (a byte below 0x20 — a key pasted across two lines of .env is the usual cause); an HTTP header cannot hold it, and sending it truncated would look like a wrong key. Put the value on one line'
    return 1
  fi
  return 0
}

# llm_endpoint_resolve picks the variable by provider and parses it. This is
# what scripts/llm-server.sh calls.
#
#   LLM_EP_HAS=0 with no error   — this provider has no address on this side
#   LLM_EP_HAS=0 with an error   — it should have had one; the caller goes red
llm_endpoint_resolve() {
  LLM_EP_PROVIDER=$(llm_env_or MV_LLM_PROVIDER openai_compat)
  LLM_EP_HAS=0
  LLM_EP_STARTABLE=0
  LLM_EP_TRUSTED=0
  LLM_EP_ERROR=''
  LLM_EP_URL='' LLM_EP_PROBE='' LLM_EP_RAW='' LLM_EP_HOST='' LLM_EP_PORT=''

  if ! llm_provider_ok "$LLM_EP_PROVIDER"; then
    LLM_EP_ERROR="llm: MV_LLM_PROVIDER=$LLM_EP_PROVIDER is not one of openai_compat, ollama, anthropic, recorded, fake; the set is declared in shared/env and mvctl env check refuses the same value"
    return 1
  fi

  case "$LLM_EP_PROVIDER" in
  anthropic | recorded | fake)
    LLM_EP_VAR=''
    return 0
    ;;
  ollama) LLM_EP_VAR=MV_OLLAMA_URL ;;
  *) LLM_EP_VAR=MV_LLM_URL ;;
  esac

  local value
  value=$(llm_trim "${!LLM_EP_VAR-}")
  if [ -z "$value" ]; then
    if [ "$LLM_EP_VAR" = MV_OLLAMA_URL ]; then
      LLM_EP_ERROR='llm: MV_LLM_PROVIDER=ollama but MV_OLLAMA_URL is empty — no address to probe (ADR-005 add. 2 p. 2)'
      return 1
    fi
    # No address, no green light. This is review C-1: `??` does not react to an
    # empty string, so an MV_LLM_URL that had been cleared but not removed made
    # the PowerShell twin print "nothing to check" and return 0 — a fully
    # unconfigured LLM passing the strict `make health` on the main machine of
    # the project.
    #
    # Blank and unset are the same answer here, and since review #2 they are the
    # same answer on the other side too: MV_LLM_URL has NO default in
    # shared/env/vars.go any more, it is Required(). It used to declare
    # http://127.0.0.1:1234, so a machine with the line deleted from .env had
    # the platform quietly talking to 1234 while this wrapper said there was no
    # address — two sources of truth for the one case the task exists to close
    # (review M-4, orchestrator decision). An address cannot be guessed.
    LLM_EP_ERROR="llm: $LLM_EP_VAR has no value — the platform has no LLM address (the variable is required and has no default; an empty value is not configuration either). Put the address in .env"
    return 1
  fi

  if ! llm_parse_url "$value"; then
    LLM_EP_ERROR="llm: $LLM_EP_ERROR"
    return 1
  fi

  llm_endpoint_finish
}

# llm_endpoint_resolve_var parses a NAMED variable, whatever the provider says.
# scripts/llm-bench.sh needs this: which variable holds the endpoint is written
# in the measurement matrix (url_env), one per configuration.
llm_endpoint_resolve_var() {
  LLM_EP_VAR=$1
  LLM_EP_PROVIDER=$(llm_env_or MV_LLM_PROVIDER openai_compat)
  LLM_EP_HAS=0
  LLM_EP_STARTABLE=0
  LLM_EP_TRUSTED=0
  LLM_EP_ERROR=''

  if ! llm_provider_ok "$LLM_EP_PROVIDER"; then
    LLM_EP_ERROR="llm: MV_LLM_PROVIDER=$LLM_EP_PROVIDER is not one of openai_compat, ollama, anthropic, recorded, fake; the set is declared in shared/env and mvctl env check refuses the same value"
    return 1
  fi

  local value
  value=$(llm_trim "${!LLM_EP_VAR-}")
  if [ -z "$value" ]; then
    LLM_EP_ERROR="llm: $LLM_EP_VAR is empty — no address to probe"
    return 1
  fi
  if ! llm_parse_url "$value"; then
    LLM_EP_ERROR="llm: $LLM_EP_ERROR"
    return 1
  fi
  llm_endpoint_finish
}

# --- probing -----------------------------------------------------------------

# llm_curl is the only place a key is put on the wire. Three decisions live here.
#
# 1. The key goes through a curl config on stdin, never through -H: an argument
#    list is readable by every process on the machine (SEC-22, ADR-009 p. 4).
# 2. Backslash and double quote are escaped. The config format understands both,
#    so a key containing one used to arrive at the server truncated or mangled,
#    and the operator saw a 401 and went looking in the wrong place (review Mi-3).
# 3. --noproxy '*': the probe measures the ENDPOINT, not the proxy in front of
#    it. On the owner's machine HTTP_PROXY is set, Invoke-WebRequest honoured it
#    and curl did not, and the proxy answered 503 "the model is loading" for an
#    address where nothing listens at all (review M-2). Both implementations now
#    go direct. This is deliberately NOT what the platform does — Go's
#    http.ProxyFromEnvironment honours the variables — and the direction is
#    chosen on purpose: a probe through a proxy can invent readiness, a direct
#    probe can only fail to see an endpoint that needs one, and a false red is
#    survivable where a false green is not. An endpoint that is only reachable
#    through a proxy is a cloud endpoint; it is noted in Docs/ops/runbook.md §3.
llm_curl() {
  local key escaped
  key=$(llm_trim "${MV_LLM_API_KEY-}")
  if [ -n "$key" ]; then
    escaped=${key//\\/\\\\}
    escaped=${escaped//\"/\\\"}
    printf 'header = "Authorization: Bearer %s"\n' "$escaped" |
      curl -s --noproxy '*' -K - "$@"
  else
    curl -s --noproxy '*' "$@"
  fi
}

# llm_probe_code sets LLM_PROBE_CODE to the status of one path. 000 means
# nothing answered: curl prints 000 through -w AND exits 7 on a refused
# connection, so the `|| echo` shorthand would append a second value and yield
# 0000, a code that matches no branch anywhere.
#
# It ASSIGNS instead of printing, and that is the whole point of this shape.
# While it printed, every caller wrote `$(llm_probe_code …)` — a subshell — and
# LLM_PROBE_FAULT died with it: llm_health_probe read the initial 0 every single
# time, so the diagnostic below was unreachable in bash while the PowerShell
# twin printed it, and a broken TLS handshake was told to run `make llm-up`
# (review M-2). Two variables, one call, no subshell.
#
# LLM_PROBE_FAULT is set when the failure was NOT "nothing answered at that
# address": a TLS failure, a refused proxy, a malformed request. curl exit 6
# (name), 7 (connect) and 28 (timeout) are the no-answer codes; everything else
# is a fault. The PowerShell twin classifies its exceptions the same way. Before
# this, `catch { return 0 }` folded every failure into "the process is not
# running (make llm-up)", advice that is wrong in each of these cases (Mi-6).
LLM_PROBE_FAULT=0
LLM_PROBE_CODE=000
llm_probe_code() {
  # `|| status=$?` and not a bare assignment: a failing command substitution in
  # an assignment trips `set -e` in the callers that do not test the result, and
  # curl exits non-zero for the most ordinary case there is — nothing listening.
  local code status=0
  code=$(llm_curl -o /dev/null -w '%{http_code}' --max-time "${3:-5}" "$1$2" 2>/dev/null) || status=$?
  LLM_PROBE_FAULT=0
  case "$status" in
  0 | 6 | 7 | 28) ;;
  *) LLM_PROBE_FAULT=1 ;;
  esac
  LLM_PROBE_CODE=${code:-000}
}

# llm_health_probe sets LLM_HEALTH_CODE and LLM_HEALTH_VIA for one base URL.
#
# /health belongs to llama.cpp, not to the contract. What the platform depends
# on is the OpenAI-compatible surface (ADR-005), and a server can serve that
# without /health: on the owner's stand a running server answers /v1/models with
# 200 and /health with 404, and the stack was reported down while it was up. A
# code that means "this is not an endpoint of mine" therefore falls back to the
# endpoint the contract names — a cloud endpoint answers 404, 401 or 405 there
# for the same reason. 503 keeps meaning "the model is still loading", every
# other code keeps its own meaning (T-403; carried into the bench by T-404
# review M-1, which is where its absence produced a false "not running").
LLM_HEALTH_CODE=000
LLM_HEALTH_VIA=/health
LLM_HEALTH_FAULT=0
llm_health_probe() {
  local base=$1 timeout=${2:-5}
  LLM_HEALTH_VIA=/health
  llm_probe_code "$base" /health "$timeout"
  LLM_HEALTH_CODE=$LLM_PROBE_CODE
  LLM_HEALTH_FAULT=$LLM_PROBE_FAULT
  case "$LLM_HEALTH_CODE" in
  401 | 403 | 404 | 405 | 501)
    llm_probe_code "$base" /v1/models "$timeout"
    if [ "$LLM_PROBE_CODE" = 200 ]; then
      LLM_HEALTH_VIA=/v1/models
      LLM_HEALTH_CODE=200
      LLM_HEALTH_FAULT=0
    fi
    ;;
  esac
}

# llm_models lists the model ids of /v1/models, comma separated.
#
# The greedy `.*"id"` of the previous sed collapsed the list to its LAST entry,
# because the answer arrives on one line: three models printed as one, the
# owner's twenty-two printed as "comfy" (review M-5). That matters more than it
# looks: T-404 made the model list the way to pin the version of a runtime that
# is not llama.cpp, and Docs/ops/runbook.md sends the operator here when the
# model changes.
llm_models() {
  # awk joins, not `paste -sd', '`: paste treats a multi-character delimiter as
  # a LIST of delimiters used in turn, so three models came out as
  # "alpha,beta gamma" while the PowerShell twin printed "alpha, beta, gamma".
  llm_curl --max-time "${2:-5}" "$1/v1/models" 2>/dev/null |
    grep -o '"id"[[:space:]]*:[[:space:]]*"[^"]*"' |
    sed 's/.*"\([^"]*\)"$/\1/' |
    awk '{ printf "%s%s", (NR > 1 ? ", " : ""), $0 } END { if (NR > 0) print "" }'
}
