#!/usr/bin/env bash
# =============================================================================
# scripts/backup-prune-test.sh — the test of scripts/backup-prune.sh (T-463)
# =============================================================================
# Usage:
#   scripts/backup-prune-test.sh [--script <path>] [--require-links]
#                                                    the test (make backup-prune-test)
#   scripts/backup-prune-test.sh --mutants [--require-links]
#                                                    the control mutants
#                                                    (make backup-prune-mutants)
#
#   --require-links  a case of symbolic links this system cannot make is a
#                    failure, not SKIPPED. CI on Linux passes it: there a skip
#                    can only mean the test broke (T-463 review #1 Mi-5).
#
# No Docker and no real backup directory: every case builds its own directory
# under a fresh `mktemp -d`, fakes the stamps in the names and the mtimes of the
# files against a fixed --now, runs the script and looks at what is left. The
# temporary directory is removed at the end by the exact path mktemp returned.
# TZ is UTC for the whole run, so the stamps mean the same on every machine;
# only the case `gap` sets a zone with a clock change, for its one run.
#
# The directories under test live in <mktemp>/backups/: the script refuses a
# directory without the word backup in its path, and the temporary directory
# itself deliberately has none, so that the refusal can be tested too.
#
# Every `rm` the script runs goes through a shim on PATH that records its
# arguments: the test holds the script to one `rm -f -- <full path>` per file,
# inside the directory under test, and to exactly the files it expects gone.
# The shim fails, without removing, for the one path named in
# BACKUP_PRUNE_TEST_RM_FAIL: the branch "a file could not be removed".
#
# --mutants breaks a copy of the script one defect at a time and requires the
# test to fail on each: the control (a syntax error) first — if even that
# passes, the test does not run the script — and the identity (no change, must
# pass) second. A mutant whose anchor does not occur exactly once is BROKEN.
# =============================================================================
set -euo pipefail

case ${BASH_SOURCE[0]} in
*/*) here=${BASH_SOURCE[0]%/*} ;;
*) here=. ;;
esac
here=$(CDPATH='' cd -- "$here" && pwd)
self="$here/${BASH_SOURCE[0]##*/}"
subject="$here/backup-prune.sh"
mode=test
require_links=0

while [ $# -gt 0 ]; do
  case "$1" in
  --script)
    [ $# -ge 2 ] || { echo "backup-prune-test: --script needs a value" >&2; exit 2; }
    subject=$2
    shift 2
    ;;
  --mutants)
    mode=mutants
    shift
    ;;
  --require-links)
    require_links=1
    shift
    ;;
  *)
    echo "backup-prune-test: unknown argument $1" >&2
    exit 2
    ;;
  esac
done

work=$(mktemp -d "${TMPDIR:-/tmp}/prune-test.XXXXXX")
[ -n "$work" ] && [ -d "$work" ] || { echo "backup-prune-test: mktemp gave no directory" >&2; exit 2; }
junctions=() # Windows junctions this run made; each goes by rmdir before rm -rf
cleanup() {
  local j
  # A junction is taken down by itself first, so that nothing walks into its
  # target (which is inside $work anyway).
  for j in "${junctions[@]}"; do
    if [ -L "$j" ]; then MSYS2_ARG_CONV_EXCL='*' cmd /c rmdir "$(cygpath -w "$j")" >/dev/null 2>&1 || :; fi
  done
  # The one path mktemp returned, nothing else.
  rm -rf -- "$work"
}
trap cleanup EXIT

# -----------------------------------------------------------------------------
# --mutants
# -----------------------------------------------------------------------------
if [ "$mode" = mutants ]; then
  # id|title|anchor|replacement — the replacement of an empty anchor is none.
  mutants=(
    'P00|control: a syntax error in backup-prune.sh|set -euo pipefail|set -euo pipefail; if then'
    'P01|identity: nothing changed, the test must pass||'
    'P02|the term is 0 days: everything is removed, the young copies too|readonly RETENTION_DAYS=30|readonly RETENTION_DAYS=0'
    'P03|the newest queued file is kept: "keep the last one"|for i in "${!doomed[@]}"; do|for i in $(seq 0 $((${#doomed[@]} - 2))); do'
    'P04|exactly 30 days is removed as well|if [ "$age" -gt "$limit" ]; then|if [ "$age" -ge "$limit" ]; then'
    'P05|no fallback to the mtime: a name without a parsable stamp lives forever|    if epoch=$(date -r "$path" +%s 2>/dev/null); then|    if false; then'
    'P06|any prefix with .tgz is an archive: postgres-<stamp>.tgz is removed|consider "$path" "$name" minio- .tgz \|\||consider "$path" "$name" "" .tgz \|\|'
    'P07|the lines of the removed archives stay in SHA256SUMS|[ -n "${gone[${BASH_REMATCH[2]}]:-}" ]|false'
    'P08|a symbolic link links/ is followed out of the directory|if [ -L "$links" ]; then|if false; then'
    'P09|the files are removed by a pattern of the directory, not one by one|  if rm -f -- "$path" && [ ! -e "$path" ]; then|  if rm -f -- "${path%/*}"/*"${path##*.}" && [ ! -e "$path" ]; then'
    'P10|a symbolic link with the name of an old archive is removed|  if [ -L "$path" ] \|\| [ ! -f "$path" ]; then|  if [ ! -f "$path" ]; then'
    'P11|the line of an archive leaves SHA256SUMS although the file could not be removed (review #1 R3)|[ -e "$root/$name" ] \|\| gone[$name]=1|gone[$name]=1'
    'P12|a failed rm counts as removed and the exit code stays 0|  if rm -f -- "$path" && [ ! -e "$path" ]; then|  if rm -f -- "$path" \|\| true; then'
    'P13|a stamp far ahead of now is trusted: the file lives until its stamp plus 30 days (review #1 Mi-3)|if [ "$epoch" -gt "$((now_epoch + FUTURE_SLACK))" ]; then|if false; then'
    'P14|a relative --dir is accepted (review #1 Mi-2)|if ! [[ $dir =~ ^/ \|\| $dir =~ ^[A-Za-z]:/ ]]; then|if false; then'
    'P15|a directory without the word backup in its path is accepted (review #1 Mi-2)|    *backup*) return 0 ;;|    *) return 0 ;;'
    'P16|the root of a drive is not recognised (review #1 Mi-2)|  local p=$1|  return 1; local p=$1'
    'P17|a mtime far ahead of now stays silent (review #2 N-7)|      if [ "$((epoch - now_epoch))" -gt "$FUTURE_SLACK" ]; then|      if false; then'
    'P18|a mtime exactly a day ahead is warned about as well (review #2 N-7)|      if [ "$((epoch - now_epoch))" -gt "$FUTURE_SLACK" ]; then|      if [ "$((epoch - now_epoch))" -ge "$FUTURE_SLACK" ]; then'
  )
  # The mutants only the case of the symbolic links can see: on a system that
  # cannot make that kind of link the part is SKIPPED, and so is the mutant.
  declare -A needs_links=([P08]='links-dir' [P10]='links-file')
  pass_on=()
  [ "$require_links" = 0 ] || pass_on=(--require-links)
  bad=0
  n=0
  echo "backup-prune-test: ${#mutants[@]} mutants against $subject, control first"
  for entry in "${mutants[@]}"; do
    # Fields are split on | that is not escaped as \|.
    rest=${entry//\\|/$'\x1e'}
    IFS='|' read -r id title anchor replace <<<"$rest"
    anchor=${anchor//$'\x1e'/|}
    replace=${replace//$'\x1e'/|}
    mdir="$work/mutants/$id"
    mkdir -p "$mdir"
    copy="$mdir/backup-prune.sh"
    content=$(<"$subject")
    if [ -n "$anchor" ]; then
      stripped=${content//"$anchor"/}
      count=$(((${#content} - ${#stripped}) / ${#anchor}))
      if [ "$count" -ne 1 ]; then
        printf '%-9s %s  %s\n          the anchor occurs %d times — update the mutant\n' BROKEN "$id" "$title" "$count"
        bad=$((bad + 1))
        n=$((n + 1))
        continue
      fi
      content=${content/"$anchor"/"$replace"}
    fi
    printf '%s\n' "$content" >"$copy"
    set +e
    out=$(TMPDIR="$mdir" bash "$self" --script "$copy" "${pass_on[@]}" 2>&1)
    code=$?
    set -e
    part=${needs_links[$id]:-}
    if [ -z "$anchor" ]; then
      if [ "$code" = 0 ]; then verdict=GREEN; else verdict=RED; fi
    elif [ "$code" = 0 ] && [ -n "$part" ] && [[ $out == *"SKIPPED $part:"* ]]; then
      verdict=SKIPPED
    elif [ "$code" = 0 ]; then
      verdict=SURVIVED
    elif [ "$code" = 1 ]; then
      verdict=KILLED
    else
      verdict=BROKEN
    fi
    printf '%-9s %s  %s\n' "$verdict" "$id" "$title"
    case "$verdict" in
    KILLED) printf '%s\n' "$out" | grep '^FAIL' | head -n 3 | sed 's/^/          /' ;;
    GREEN) ;;
    SKIPPED) echo "          this system cannot make the link of the part $part; the mutant is checked where it can (the CI job on Linux, with --require-links)" ;;
    *)
      bad=$((bad + 1))
      printf '%s\n' "$out" | sed 's/^/          | /'
      if [ "$n" = 0 ]; then
        echo "backup-prune-test: the control mutant was not killed — the test does not run the script; stopping"
        exit 1
      fi
      ;;
    esac
    n=$((n + 1))
  done
  if [ "$bad" -gt 0 ]; then
    echo "backup-prune-test: $bad mutant(s) not as they must be"
    exit 1
  fi
  echo "backup-prune-test: mutants ok — every defect turns the test red, the identity stays green"
  exit 0
fi

# -----------------------------------------------------------------------------
# the test
# -----------------------------------------------------------------------------
export TZ=UTC
[ -f "$subject" ] || { echo "backup-prune-test: $subject does not exist" >&2; exit 2; }
# Every run starts in $work (see run), so the script is named by a full path.
subject=$(CDPATH='' cd -- "$(dirname -- "$subject")" && pwd)/${subject##*/}

DAY=86400
NOW=20260913-120000
now_epoch=$(date -d '2026-09-13 12:00:00' +%s)
failures=0
skipped=0
base="$work/backups"
mkdir -p "$base"

fail() {
  printf 'FAIL %s\n' "$*"
  failures=$((failures + 1))
}
# skip PART WHY — SKIPPED, or a failure under --require-links.
skip() {
  if [ "$require_links" = 1 ]; then
    fail "$1: $2 (--require-links: a skip is a failure here)"
  else
    echo "SKIPPED $1: $2"
    skipped=$((skipped + 1))
  fi
}

# stamp SECONDS_AGO — the stamp `make backup` would have written then.
stamp() { date -d "@$((now_epoch - $1))" +%Y%m%d-%H%M%S; }

# put PATH MTIME_SECONDS_AGO — a file of a few bytes with that mtime.
put() {
  mkdir -p "${1%/*}"
  printf 'archive %s\n' "${1##*/}" >"$1"
  touch -d "@$((now_epoch - $2))" "$1"
}

# The rm shim. It records every call — one argument per line, the calls apart
# by an empty line — and then does what rm does, except for the one path of
# BACKUP_PRUNE_TEST_RM_FAIL: that call fails and removes nothing.
real_rm=$(command -v rm)
mkdir -p "$work/shim"
cat >"$work/shim/rm" <<EOF
#!/usr/bin/env bash
{ for a in "\$@"; do printf '%s\n' "\$a"; done; printf '\n'; } >>"\$BACKUP_PRUNE_TEST_RM_LOG"
if [ -n "\${BACKUP_PRUNE_TEST_RM_FAIL:-}" ] && [ "\${!#}" = "\$BACKUP_PRUNE_TEST_RM_FAIL" ]; then
  echo "rm: cannot remove '\${!#}': Device or resource busy (the shim of the test)" >&2
  exit 1
fi
exec "$real_rm" "\$@"
EOF
chmod +x "$work/shim/rm"

# run CASE ARGS... — runs the script with the shim; sets $out, $code, $rm_log.
# RUN_TZ and RUN_RM_FAIL, when set, reach the one run. The run starts in $work:
# a relative --dir of a refusal — or of a mutant that accepts one — must never
# resolve against the directory make runs in (the repository has a backups/).
run() {
  local case=$1
  shift
  rm_log="$work/$case.rm.log"
  : >"$rm_log"
  set +e
  out=$(cd "$work" && TZ=${RUN_TZ:-$TZ} PATH="$work/shim:$PATH" BACKUP_PRUNE_TEST_RM_LOG="$rm_log" \
    BACKUP_PRUNE_TEST_RM_FAIL="${RUN_RM_FAIL:-}" bash "$subject" "$@" 2>&1)
  code=$?
  set -e
}

# physical DIR — the path as the script prints it (cd -P; pwd -P).
physical() { (CDPATH='' cd -P -- "$1" && pwd -P); }

gone() {
  local case=$1 path
  shift
  for path in "$@"; do
    if [ -e "$path" ] || [ -L "$path" ]; then fail "$case: $path is still there, it is older than 30 days"; fi
  done
}
here() {
  local case=$1 path
  shift
  for path in "$@"; do
    if [ ! -e "$path" ] && [ ! -L "$path" ]; then fail "$case: $path was removed, it must stay"; fi
  done
}

# rm_calls CASE ROOT EXPECTED_PATH... — every call is exactly `-f -- <path>`,
# the path is inside ROOT and carries no pattern, and the paths are exactly the
# expected ones (a temporary SHA256SUMS.prune-<pid> of the script aside).
rm_calls() {
  local case=$1 root=$2 line call=() p want
  shift 2
  local -A expected=() got=()
  for want in "$@"; do expected[$want]=1; done
  while IFS= read -r line || [ -n "$line" ]; do
    if [ -n "$line" ]; then
      call+=("$line")
      continue
    fi
    if [ "${#call[@]}" -ne 3 ] || [ "${call[0]}" != -f ] || [ "${call[1]}" != -- ]; then
      fail "$case: rm was called as 'rm ${call[*]}', not as 'rm -f -- <one path>'"
    else
      p=${call[2]}
      case "$p" in
      "$root"/*) ;;
      *) fail "$case: rm of $p, outside $root" ;;
      esac
      case "$p" in
      *[*?[]*) fail "$case: rm of a pattern $p" ;;
      esac
      case "${p##*/}" in
      SHA256SUMS.prune-*) ;;
      *) got[$p]=1 ;;
      esac
    fi
    call=()
  done <"$rm_log"
  for p in "${!got[@]}"; do
    [ -n "${expected[$p]:-}" ] || fail "$case: rm of $p, which is not one of the files older than 30 days"
  done
  for want in "${!expected[@]}"; do
    [ -n "${got[$want]:-}" ] || fail "$case: no 'rm -f -- $want'"
  done
}

says() {
  local case=$1 text=$2
  [[ $out == *"$text"* ]] || fail "$case: the output lacks '$text'; it was:"$'\n'"$out"
}
lacks() {
  local case=$1 text=$2
  [[ $out != *"$text"* ]] || fail "$case: the output must not say '$text'; it was:"$'\n'"$out"
}

# ---- case 1: a mixed directory ----------------------------------------------
c=mixed
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
s31=$(stamp $((31 * DAY)))
s30p=$(stamp $((30 * DAY + 1)))
s30=$(stamp $((30 * DAY)))
s29=$(stamp $((29 * DAY)))
s40=$(stamp $((40 * DAY)))
sfut=$(stamp $((-1 * DAY)))
sfar=$(stamp $((-2 * DAY)))
sfar10=$(stamp $((-10 * DAY)))

# Aged by the stamp: the mtime says the opposite on purpose.
put "$d/minio-$s31.tgz" 0
put "$d/redpanda-$s31.tgz" 0
put "$d/minio-$s30p.tgz" 0
put "$d/minio-$s30.tgz" $((400 * DAY))
put "$d/redpanda-$s29.tgz" $((400 * DAY))
# Exactly a day ahead is a clock a little off: the stamp is still trusted.
put "$d/redpanda-$sfut.tgz" $((400 * DAY))
put "$d/links/links-$s31.db.age" 0
put "$d/links/gateway-$s31.db" 0
put "$d/links/links-$s29.db.age" $((400 * DAY))
put "$d/links/gateway-$s30.db" $((400 * DAY))
# More than a day ahead: the stamp is not trusted, the mtime decides (Mi-3).
put "$d/minio-$sfar.tgz" $((31 * DAY))
put "$d/redpanda-$sfar10.tgz" 0
# A covered prefix without a parsable stamp: aged by the mtime.
put "$d/minio-copy.tgz" $((31 * DAY))
put "$d/redpanda-copy.tgz" $((29 * DAY))
put "$d/minio-20261301-000000.tgz" $((29 * DAY))
put "$d/redpanda-20260230-000000.tgz" $((31 * DAY))
put "$d/links/links-old.db.age" $((31 * DAY))
put "$d/links/gateway-old.db" $((29 * DAY))
# No stamp and a mtime ahead: nothing better than the mtime, so it stays — with
# a warning when more than a day ahead, silently at exactly a day (N-7).
put "$d/minio-ahead.tgz" $((-400 * DAY))
put "$d/redpanda-ahead.tgz" $((-1 * DAY))
# Foreign: other names, other places, other shapes — all very old.
old=$((400 * DAY))
put "$d/notes.txt" $old
put "$d/postgres-$s31.tgz" $old
put "$d/minio-$s31.tgz.bak" $old
put "$d/minio-$s31.tar.gz" $old
put "$d/minio-.tgz" $old
put "$d/links-$s31.db.age" $old
put "$d/gateway-$s31.db" $old
put "$d/backup-prune.log" $old
put "$d/links/links-$s31.db" $old
put "$d/links/gateway-$s31.db.age" $old
put "$d/links/notes.txt" $old
put "$d/links/sub/links-$s31.db.age" $old
put "$d/other/minio-$s31.tgz" $old
put "$d/legacy-20200101/minio-as-is.tgz" $old
put "$d/legacy-20200101/SHA256SUMS" $old
put "$d/minio-$s40.tgz/inside.txt" $old
touch -d "@$((now_epoch - old))" "$d/minio-$s40.tgz"
h=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
{
  printf '%s  %s\n' "$h" "minio-$s31.tgz" "$h" "redpanda-$s31.tgz" "$h" "postgres-$s31.tgz"
  printf '%s  %s\n' "$h" "minio-$s30.tgz" "$h" "redpanda-$s29.tgz" "$h" "minio-$s40-missing.tgz"
  printf '%s *%s\n' "$h" "minio-$s30p.tgz"
  printf '%s  ./%s\n' "$h" "minio-copy.tgz"
  printf '%s  %s\n' "$h" "redpanda-copy.tgz" "$h" "redpanda-20260230-000000.tgz"
  printf '%s  %s\n' "$h" "minio-$sfar.tgz" "$h" "redpanda-$sfar10.tgz"
} >"$d/SHA256SUMS"
touch -d "@$((now_epoch - old))" "$d/SHA256SUMS"

run $c --dir "$d" --now "$NOW"
[ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
expect_gone=("$d/minio-$s31.tgz" "$d/redpanda-$s31.tgz" "$d/minio-$s30p.tgz" "$d/links/links-$s31.db.age"
  "$d/links/gateway-$s31.db" "$d/minio-copy.tgz" "$d/redpanda-20260230-000000.tgz" "$d/links/links-old.db.age"
  "$d/minio-$sfar.tgz")
gone $c "${expect_gone[@]}"
here $c "$d/minio-$s30.tgz" "$d/redpanda-$s29.tgz" "$d/redpanda-$sfut.tgz" "$d/links/links-$s29.db.age" \
  "$d/links/gateway-$s30.db" "$d/redpanda-copy.tgz" "$d/minio-20261301-000000.tgz" "$d/links/gateway-old.db" \
  "$d/redpanda-$sfar10.tgz" "$d/minio-ahead.tgz" "$d/redpanda-ahead.tgz"
here $c "$d/notes.txt" "$d/postgres-$s31.tgz" "$d/minio-$s31.tgz.bak" "$d/minio-$s31.tar.gz" "$d/minio-.tgz" \
  "$d/links-$s31.db.age" "$d/gateway-$s31.db" "$d/backup-prune.log" "$d/links/links-$s31.db" \
  "$d/links/gateway-$s31.db.age" "$d/links/notes.txt" "$d/links/sub/links-$s31.db.age" "$d/other/minio-$s31.tgz" \
  "$d/legacy-20200101/minio-as-is.tgz" "$d/legacy-20200101/SHA256SUMS" "$d/minio-$s40.tgz/inside.txt" "$d/SHA256SUMS"
rm_calls $c "$d" "${expect_gone[@]}"
for p in "${expect_gone[@]}"; do says $c "removed $p ("; done
says $c "removed $d/minio-$s31.tgz (31 days old by its stamp)"
says $c "removed $d/minio-copy.tgz (31 days old by its mtime)"
says $c "removed $d/minio-$sfar.tgz (31 days old by its mtime)"
says $c "WARNING $d/minio-$sfar.tgz: its stamp is more than a day ahead of now"
says $c "WARNING $d/redpanda-$sfar10.tgz: its stamp is more than a day ahead of now"
lacks $c "WARNING $d/redpanda-$sfut.tgz"
says $c "WARNING $d/minio-ahead.tgz: its mtime is more than a day ahead of now"
lacks $c "WARNING $d/redpanda-ahead.tgz"
lacks $c "WARNING $d/redpanda-$sfar10.tgz: its mtime"
lacks $c "WARNING $d/minio-$sfar.tgz: its mtime"
says $c "done: 9 removed, 11 kept"
want_sums=$(
  printf '%s  %s\n' "$h" "postgres-$s31.tgz" "$h" "minio-$s30.tgz" "$h" "redpanda-$s29.tgz" "$h" "minio-$s40-missing.tgz"
  printf '%s  %s\n' "$h" "redpanda-copy.tgz" "$h" "redpanda-$sfar10.tgz"
)
got_sums=$(<"$d/SHA256SUMS")
[ "$got_sums" = "$want_sums" ] || fail "$c: SHA256SUMS is"$'\n'"$got_sums"$'\n'"want"$'\n'"$want_sums"

# ---- case 2: only old copies — the directory is emptied of them ------------
c=only-old
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
only_old=("$d/minio-$s31.tgz" "$d/redpanda-$s31.tgz" "$d/minio-$(stamp $((60 * DAY))).tgz"
  "$d/redpanda-$(stamp $((60 * DAY))).tgz" "$d/links/links-$s31.db.age" "$d/links/gateway-$(stamp $((45 * DAY))).db")
for p in "${only_old[@]}"; do put "$p" 0; done
for p in "${only_old[@]:0:4}"; do printf '%s  %s\n' "$h" "${p##*/}"; done >"$d/SHA256SUMS"
run $c --dir "$d" --now "$NOW"
[ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
gone $c "${only_old[@]}"
rm_calls $c "$d" "${only_old[@]}"
says $c "done: 6 removed, 0 kept"
[ ! -s "$d/SHA256SUMS" ] || fail "$c: SHA256SUMS still holds:"$'\n'"$(<"$d/SHA256SUMS")"

# ---- case 3: --dry-run removes nothing -------------------------------------
c=dry-run
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
put "$d/minio-$s31.tgz" 0
put "$d/links/gateway-$s31.db" 0
printf '%s  %s\n' "$h" "minio-$s31.tgz" >"$d/SHA256SUMS"
run $c --dir "$d" --now "$NOW" --dry-run
[ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
here $c "$d/minio-$s31.tgz" "$d/links/gateway-$s31.db"
rm_calls $c "$d"
says $c "would remove $d/minio-$s31.tgz (31 days old by its stamp)"
says $c "done: 2 would be removed, 0 kept"
[ "$(<"$d/SHA256SUMS")" = "$h  minio-$s31.tgz" ] || fail "$c: SHA256SUMS changed in a dry run"

# ---- case 4: a file that cannot be removed keeps its line (Mi-5) -----------
# The shim refuses one path, as a file held open by an antivirus would be. The
# run goes on with the others, exits 1 — the "last result 1" of the runbook —
# and the line of the archive that is still there stays in SHA256SUMS.
c=stuck
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
put "$d/minio-$s31.tgz" 0
put "$d/redpanda-$s31.tgz" 0
printf '%s  %s\n' "$h" "minio-$s31.tgz" "$h" "redpanda-$s31.tgz" >"$d/SHA256SUMS"
RUN_RM_FAIL="$d/minio-$s31.tgz" run $c --dir "$d" --now "$NOW"
[ "$code" = 1 ] || fail "$c: exit $code, want 1; output:"$'\n'"$out"
here $c "$d/minio-$s31.tgz"
gone $c "$d/redpanda-$s31.tgz"
rm_calls $c "$d" "$d/minio-$s31.tgz" "$d/redpanda-$s31.tgz"
says $c "could not remove $d/minio-$s31.tgz"
lacks $c "removed $d/minio-$s31.tgz ("
says $c "done: 1 removed, 0 kept"
[ "$(<"$d/SHA256SUMS")" = "$h  minio-$s31.tgz" ] ||
  fail "$c: SHA256SUMS is"$'\n'"$(<"$d/SHA256SUMS")"$'\n'"want only the line of minio-$s31.tgz, which is still there"

# ---- case 5: a stamp inside the gap of a clock change (N-1) ----------------
# A POSIX zone and not Europe/Berlin: Git for Windows ships no zoneinfo, and a
# name it does not know would silently be UTC. 2026-03-29 02:30 does not exist
# there. Such a stamp is not one `make backup` wrote, so the mtime decides: 29
# days, the file stays — by the stamp it would be half a year old and gone. The
# stamp an hour earlier exists and is aged by itself.
c=gap
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
put "$d/redpanda-20260329-023000.tgz" $((29 * DAY))
put "$d/minio-20260329-013000.tgz" $((29 * DAY))
RUN_TZ='CET-1CEST,M3.5.0,M10.5.0/3' run $c --dir "$d" --now "$NOW"
[ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
here $c "$d/redpanda-20260329-023000.tgz"
gone $c "$d/minio-20260329-013000.tgz"
rm_calls $c "$d" "$d/minio-20260329-013000.tgz"
says $c "run at 2026-09-13 12:00:00 +0200"
says $c "removed $d/minio-20260329-013000.tgz (168 days old by its stamp)"

# ---- case 6: symbolic links are not followed -------------------------------
# Two parts, each skipped on its own: a link to a file with the name of an old
# archive (links-file), and links/ itself a link to a directory outside
# (links-dir). Native links or nothing: Git Bash copies the target when it
# cannot make a link, and a copy would prove nothing here. Windows without
# Developer Mode cannot make a symbolic link, but it can make a junction to a
# directory, and bash sees one as a link: links-dir falls back to it (N-3).
c=links
d="$base/$c"
outside="$work/$c-outside"
mkdir -p "$d" "$outside/links-dir"
d=$(physical "$d")
outside=$(physical "$outside")
put "$outside/minio-$s31.tgz" $old
put "$outside/links-dir/links-$s31.db.age" $old
file_link=0
dir_link=''
if MSYS=winsymlinks:nativestrict ln -s "$outside/minio-$s31.tgz" "$d/minio-$s40.tgz" 2>/dev/null &&
  [ -L "$d/minio-$s40.tgz" ]; then
  file_link=1
fi
if MSYS=winsymlinks:nativestrict ln -s "$outside/links-dir" "$d/links" 2>/dev/null && [ -L "$d/links" ]; then
  dir_link=symlink
elif command -v cygpath >/dev/null 2>&1 && command -v cmd >/dev/null 2>&1; then
  # MSYS2_ARG_CONV_EXCL: Git Bash would otherwise turn /J into a path.
  if MSYS2_ARG_CONV_EXCL='*' cmd /c mklink /J "$(cygpath -w "$d/links")" "$(cygpath -w "$outside/links-dir")" >/dev/null 2>&1; then
    junctions+=("$d/links")
    if [ -L "$d/links" ]; then dir_link=junction; fi
  fi
fi
if [ "$file_link" = 1 ] || [ -n "$dir_link" ]; then
  run $c --dir "$d" --now "$NOW"
  [ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
  here $c "$outside/minio-$s31.tgz" "$outside/links-dir/links-$s31.db.age"
  rm_calls $c "$d"
fi
if [ "$file_link" = 1 ]; then
  here $c "$d/minio-$s40.tgz"
  says $c "skipped $d/minio-$s40.tgz: not a regular file"
else
  skip links-file "this system cannot make a symbolic link to a file here (Windows without Developer Mode); the CI job on Linux runs it"
fi
if [ -n "$dir_link" ]; then
  here $c "$d/links"
  says $c "skipped $d/links: a symbolic link, not followed"
  [ "$dir_link" = symlink ] || echo "note $c: links/ is a junction here, not a symbolic link"
else
  skip links-dir "this system can make neither a symbolic link nor a junction to a directory here"
fi

# ---- case 7: refusals --------------------------------------------------------
c=refusals
mkdir -p "$base/empty" "$work/plain"
run $c --now "$NOW"
[ "$code" = 2 ] || fail "$c: no --dir: exit $code, want 2"
says $c "--dir is required"
run $c --dir '' --now "$NOW"
[ "$code" = 2 ] || fail "$c: an empty --dir: exit $code, want 2"
says $c "--dir is required"
run $c --dir "$base/does-not-exist" --now "$NOW"
[ "$code" = 2 ] || fail "$c: a missing directory: exit $code, want 2"
says $c "is not a directory; nothing was removed"
run $c --dir "$base/mixed" --now 2026-09-13
[ "$code" = 2 ] || fail "$c: a malformed --now: exit $code, want 2"
run $c --dir "$base/mixed" --bogus
[ "$code" = 2 ] || fail "$c: an unknown argument: exit $code, want 2"
# Mi-2: the path. Every refusal removes nothing: rm is never called.
for bad_dir in / '//' 'C:\' 'C:/' /c '/c/'; do
  run $c --dir "$bad_dir" --now "$NOW"
  [ "$code" = 2 ] || fail "$c: --dir $bad_dir: exit $code, want 2; output:"$'\n'"$out"
  says $c "the root of a drive"
  rm_calls $c "$base"
done
# Relative paths, run from $work: backups/empty WOULD name a backup directory
# from there, and holds an old archive — refused all the same, because a task
# of the Task Scheduler resolves a relative path against somewhere else.
put "$base/empty/minio-$s31.tgz" 0
for bad_dir in . ./ c: backups 'backups\empty' backups/empty; do
  run $c --dir "$bad_dir" --now "$NOW"
  [ "$code" = 2 ] || fail "$c: --dir $bad_dir: exit $code, want 2; output:"$'\n'"$out"
  says $c "is not an absolute path"
  rm_calls $c "$base"
done
here $c "$base/empty/minio-$s31.tgz"
# A path without the word backup — spelled out, or reached by .. from one that
# has it. Only where the temporary directory itself does not hold the word.
plain=$(physical "$work/plain")
case "${plain,,}" in
*backup*)
  echo "note $c: the temporary directory $plain holds the word backup; the refusal of a path without it is not checked here"
  ;;
*)
  put "$work/plain/minio-$s31.tgz" 0
  run $c --dir "$work/plain" --now "$NOW"
  [ "$code" = 2 ] || fail "$c: a directory without the word backup: exit $code, want 2; output:"$'\n'"$out"
  says $c "no component of it holds the word backup"
  here $c "$work/plain/minio-$s31.tgz"
  rm_calls $c "$work/plain"
  put "$work/minio-$s31.tgz" 0
  run $c --dir "$base/.." --now "$NOW"
  [ "$code" = 2 ] || fail "$c: --dir <backups>/..: exit $code, want 2; output:"$'\n'"$out"
  says $c "no component of it holds the word backup"
  here $c "$work/minio-$s31.tgz"
  rm_calls $c "$work"
  ;;
esac
# Backups in any letter case, and a Windows path with backslashes, are accepted.
mkdir -p "$work/Nightly-BACKUP"
run $c --dir "$(physical "$work/Nightly-BACKUP")" --now "$NOW"
[ "$code" = 0 ] || fail "$c: a directory named Nightly-BACKUP: exit $code, want 0; output:"$'\n'"$out"
if command -v cygpath >/dev/null 2>&1; then
  run $c --dir "$(cygpath -w "$base/empty")" --now "$NOW" --dry-run
  [ "$code" = 0 ] || fail "$c: --dir in the Windows form: exit $code, want 0; output:"$'\n'"$out"
  says $c "would remove"
fi
# N-2: a leading zero of --now @<epoch> is not octal.
run $c --dir "$base/empty" --now "@0$now_epoch" --dry-run
[ "$code" = 0 ] || fail "$c: --now @0<epoch>: exit $code, want 0; output:"$'\n'"$out"
says $c "run at 2026-09-13 12:00:00 +0000"

# ---- case 8: a normal run takes the clock of the system, and --log ---------
c=clock
d="$base/$c"
mkdir -p "$d"
d=$(physical "$d")
real_now=$(date +%s)
touch_old() { mkdir -p "${1%/*}"; : >"$1"; touch -d "@$((real_now - $2))" "$1"; }
touch_old "$d/minio-a.tgz" $((31 * DAY))
touch_old "$d/minio-b.tgz" $((29 * DAY))
run $c --dir "$d" --log "$d/backup-prune.log"
[ "$code" = 0 ] || fail "$c: exit $code, want 0; output:"$'\n'"$out"
gone $c "$d/minio-a.tgz"
here $c "$d/minio-b.tgz" "$d/backup-prune.log"
grep -qF "removed $d/minio-a.tgz" "$d/backup-prune.log" || fail "$c: the log lacks the removed file"

echo "backup-prune-test: $failures failure(s), $skipped part(s) skipped"
[ "$failures" = 0 ] || exit 1
echo "backup-prune-test: ok"
