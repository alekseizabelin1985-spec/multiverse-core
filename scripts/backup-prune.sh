#!/usr/bin/env bash
# =============================================================================
# scripts/backup-prune.sh — the 30 day retention of the backup directory
# (T-463; infrastructure.md §5.6; runbook section 4)
# =============================================================================
# Usage:
#   scripts/backup-prune.sh --dir <backup directory> [--dry-run] [--log <file>]
#                           [--now <YYYYMMDD-HHMMSS | @epoch seconds>]
#
#   --dir      the directory `make backup` writes to ($(BACKUP_DIR) of the
#              Makefile). Required: a tool that deletes has no default place.
#              It must be an absolute path (/..., C:\... or C:/...), not the
#              root of a drive, and one of its components must hold the word
#              "backup" in any letter case (multiverse-backups, Backups): a
#              task of the Task Scheduler resolves a relative path against
#              System32, and a mistyped /TR must not clean some other place
#              (T-463 review #1 Mi-2).
#   --dry-run  print what would be removed, remove nothing.
#   --log      append everything this run prints to that file as well; the
#              daily task of the Windows Task Scheduler passes it, so that the
#              last run can be read afterwards.
#   --now      the moment the ages are measured from. For the test only
#              (scripts/backup-prune-test.sh); a normal run takes the clock of
#              the system.
#
# Exit code: 0 on success (nothing to remove included), 1 when a file could not
# be removed or SHA256SUMS could not be rewritten (the line of a file that is
# still there stays), 2 on a usage error, a missing directory or a --dir that is
# refused (relative, the root of a drive, no component with "backup").
#
# What it removes, and only that. The copies of player data do not live longer
# than 30 days (decision of the orchestrator, journal.md 2026-09-13; security
# review of T-318, M-1 and R2-Mi-1; the same term as the prompts-* buckets of
# SEC-22). The rule is AGE, never count: there is no "keep the last N" and no
# "keep the newest archive" — a directory holding only old copies is emptied.
#
#   <dir>/minio-<stamp>.tgz          the volume archives of `make backup`, and
#   <dir>/redpanda-<stamp>.tgz       their lines in <dir>/SHA256SUMS
#   <dir>/links/links-<stamp>.db.age the host copies of links.db and gateway.db
#   <dir>/links/gateway-<stamp>.db   (§5.6; the backup itself is EPIC-004)
#
# <stamp> is `date +%Y%m%d-%H%M%S` in local time, the format of `make backup`.
# The age is read from the stamp, because a copy of the file changes its mtime
# and not its name. A name with the prefix and the suffix above whose stamp does
# not parse (minio-copy.tgz, a month 13) is aged by its mtime instead: otherwise
# it would live forever. So is a stamp more than a day ahead of now (a clock
# that was wrong during `make backup`, a renamed file): it is not trusted, the
# mtime decides, and the run prints a warning (T-463 review #1 Mi-3). A stamp at
# most a day ahead is a clock that differs a little and stays trusted. A mtime
# more than a day ahead has nothing behind it: it decides, with a warning of
# its own (T-463 review #2 N-7). A file
# is removed when it is STRICTLY older than 30 days; exactly 30 days, and
# anything younger, stays.
#
# What it never touches: any other name (notes.txt, postgres-<stamp>.tgz,
# SHA256SUMS itself, the log), any subdirectory other than links/ (legacy-*,
# anything else), a directory or a symbolic link carrying a covered name, and
# links/ itself when it is a symbolic link. Nothing is followed out of the
# directory; nothing is removed by a pattern or recursively: the directory is
# listed, and each file goes by its own full path with `rm -f -- <path>`.
# =============================================================================
set -euo pipefail

# The term of the decision. A constant and not a variable of the environment:
# a new MV_* would need the manifest (shared/env), and a term an operator can
# widen by one line of .env is not the decision any more.
readonly RETENTION_DAYS=30

me=backup-prune
dir=''
now=''
dry_run=0
log=''

say() {
  printf '%s\n' "$me: $*"
  if [ -n "$log" ]; then printf '%s\n' "$me: $*" >>"$log"; fi
}
complain() {
  printf '%s\n' "$me: $*" >&2
  if [ -n "$log" ]; then printf '%s\n' "$me: $*" >>"$log"; fi
}
usage() {
  complain "$1"
  complain "usage: scripts/backup-prune.sh --dir <backup directory> [--dry-run] [--log <file>] [--now <YYYYMMDD-HHMMSS | @epoch>]"
  exit 2
}

while [ $# -gt 0 ]; do
  case "$1" in
  --dir)
    [ $# -ge 2 ] || usage "--dir needs a value"
    dir=$2
    shift 2
    ;;
  --now)
    [ $# -ge 2 ] || usage "--now needs a value"
    now=$2
    shift 2
    ;;
  --log)
    [ $# -ge 2 ] || usage "--log needs a value"
    log=${2//\\//}
    shift 2
    ;;
  --dry-run)
    dry_run=1
    shift
    ;;
  *) usage "unknown argument $1" ;;
  esac
done

[ -n "$dir" ] || usage "--dir is required"
if [ -n "$log" ] && ! { : >>"$log"; } 2>/dev/null; then
  target=$log
  log=''
  usage "--log $target cannot be written"
fi

# stamp_epoch STAMP prints the epoch seconds of a YYYYMMDD-HHMMSS stamp in local
# time, or fails. The stamp must survive the round trip through date. The trip
# is a guard, not the first line: GNU date (8.32 and later) refuses a month 13,
# 30 February and a time inside the gap of a clock change by itself (the case
# `gap` of the test holds the outcome, whichever of the two refuses), but a date
# that normalised such a time would hand back another time, and that is not a
# stamp `make backup` wrote (T-463 review #1 N-1).
stamp_epoch() {
  local s=$1 epoch
  [[ $s =~ ^([0-9]{4})([0-9]{2})([0-9]{2})-([0-9]{2})([0-9]{2})([0-9]{2})$ ]] || return 1
  epoch=$(date -d "${BASH_REMATCH[1]}-${BASH_REMATCH[2]}-${BASH_REMATCH[3]} ${BASH_REMATCH[4]}:${BASH_REMATCH[5]}:${BASH_REMATCH[6]}" +%s 2>/dev/null) || return 1
  [ "$(date -d "@$epoch" +%Y%m%d-%H%M%S)" = "$s" ] || return 1
  printf '%s' "$epoch"
}

if [ -z "$now" ]; then
  now_epoch=$(date +%s)
elif [[ $now =~ ^@[0-9]+$ ]]; then
  # 10#: a leading zero is not octal (@0123 is 123, @09 is not an error).
  now_epoch=$((10#${now#@}))
elif ! now_epoch=$(stamp_epoch "$now"); then
  usage "--now $now is neither YYYYMMDD-HHMMSS nor @<epoch seconds>"
fi

# A Windows path from the Task Scheduler (C:\Users\...\multiverse-backups) is a
# glob with escapes to bash; forward slashes are the same path to Git Bash.
dir=${dir//\\//}

# is_root PATH: / (and //...), a drive (C:, C:/) or its Git Bash form (/c).
is_root() {
  local p=$1
  while [ "${#p}" -gt 1 ] && [ "${p%/}" != "$p" ]; do p=${p%/}; done
  [[ $p =~ ^/*$ || $p =~ ^[A-Za-z]:/*$ || $p =~ ^/[A-Za-z]$ ]]
}
# has_backup_component PATH: one component holds "backup", in any case.
has_backup_component() {
  local part parts=()
  # read -a and not an unquoted $1: a component like [a]* must not glob.
  IFS=/ read -r -a parts <<<"$1"
  for part in "${parts[@]}"; do
    case "${part,,}" in
    *backup*) return 0 ;;
    esac
  done
  return 1
}

# The checks of the path come before any look at the disk, and again after the
# path is resolved: /c/backups/.. is spelled right and is still the drive.
if ! [[ $dir =~ ^/ || $dir =~ ^[A-Za-z]:/ ]]; then
  usage "--dir $dir is not an absolute path; a task of the Task Scheduler resolves a relative one against System32 — write the full path of the backup directory"
fi
if is_root "$dir"; then
  usage "--dir $dir is the root of a drive; the backup directory is a directory of its own ($HOME/multiverse-backups by default)"
fi
if [ ! -d "$dir" ]; then
  complain "$dir is not a directory; nothing was removed (make backup creates it — check the --dir of the task)"
  exit 2
fi
root=$(CDPATH='' cd -P -- "$dir" && pwd -P)
if is_root "$root"; then
  usage "--dir $dir resolves to $root, the root of a drive; nothing was removed"
fi
if ! has_backup_component "$root"; then
  usage "--dir $dir resolves to $root, and no component of it holds the word backup; this is not a backup directory, nothing was removed"
fi

limit=$((RETENTION_DAYS * 86400))
# How far ahead of now a stamp is still believed: a clock a little off. Beyond
# it the stamp is not trusted and the mtime decides (Mi-3).
readonly FUTURE_SLACK=86400
say "run at $(date -d "@$now_epoch" '+%Y-%m-%d %H:%M:%S %z'), $root, files older than $RETENTION_DAYS days$([ "$dry_run" = 1 ] && printf ' (dry run)')"

doomed=()   # full paths, in the order of the listing
doomed_why=()
sums_names=() # names of removed archives, for SHA256SUMS
kept=0

# consider PATH NAME PREFIX SUFFIX: queue PATH when NAME is PREFIX<x>SUFFIX with
# a non-empty <x> and the file is older than the term.
consider() {
  local path=$1 name=$2 prefix=$3 suffix=$4 middle epoch source age
  case "$name" in
  "$prefix"?*"$suffix") ;;
  *) return 1 ;;
  esac
  # A directory, a device or a symbolic link with a covered name is not a copy
  # this script made, and a link may lead out of the directory.
  if [ -L "$path" ] || [ ! -f "$path" ]; then
    say "skipped $path: not a regular file"
    return 0
  fi
  middle=${name#"$prefix"}
  middle=${middle%"$suffix"}
  source=''
  if epoch=$(stamp_epoch "$middle"); then
    source=stamp
    if [ "$epoch" -gt "$((now_epoch + FUTURE_SLACK))" ]; then
      # A file stamped ahead would otherwise live until its stamp plus the
      # term — past the 30 days promised to the players (FR-009).
      say "WARNING $path: its stamp is more than a day ahead of now; the stamp is not trusted, the file is aged by its mtime"
      source=''
    fi
  fi
  if [ -z "$source" ]; then
    if epoch=$(date -r "$path" +%s 2>/dev/null); then
      source=mtime
      # Nothing better than the mtime is left, so a mtime far ahead still
      # decides and the file stays until it plus the term; the run says so,
      # the clock of the stand may be wrong (T-463 review #2 N-7).
      if [ "$((epoch - now_epoch))" -gt "$FUTURE_SLACK" ]; then
        say "WARNING $path: its mtime is more than a day ahead of now; the file is aged by it and stays until its mtime plus $RETENTION_DAYS days — check the clock"
      fi
    else
      say "skipped $path: its mtime cannot be read"
      return 0
    fi
  fi
  age=$((now_epoch - epoch))
  if [ "$age" -gt "$limit" ]; then
    doomed+=("$path")
    doomed_why+=("$((age / 86400)) days old by its $source")
    if [ "$prefix" = minio- ] || [ "$prefix" = redpanda- ]; then
      sums_names+=("$name")
    fi
  else
    kept=$((kept + 1))
  fi
  return 0
}

shopt -s nullglob
for path in "$root"/*; do
  name=${path##*/}
  consider "$path" "$name" minio- .tgz ||
    consider "$path" "$name" redpanda- .tgz ||
    :
done
links="$root/links"
if [ -L "$links" ]; then
  say "skipped $links: a symbolic link, not followed"
elif [ -d "$links" ]; then
  for path in "$links"/*; do
    name=${path##*/}
    consider "$path" "$name" links- .db.age ||
      consider "$path" "$name" gateway- .db ||
      :
  done
fi
shopt -u nullglob

rc=0
removed=0
for i in "${!doomed[@]}"; do
  path=${doomed[$i]}
  if [ "$dry_run" = 1 ]; then
    say "would remove $path (${doomed_why[$i]})"
    continue
  fi
  if rm -f -- "$path" && [ ! -e "$path" ]; then
    say "removed $path (${doomed_why[$i]})"
    removed=$((removed + 1))
  else
    complain "could not remove $path"
    rc=1
  fi
done

# SHA256SUMS: the lines of the removed archives go, every other line stays as
# it was, in its order. A line is `<hash>  <name>` or `<hash> *<name>`; the
# name may carry ./ in front. Only the archives actually removed count.
sums="$root/SHA256SUMS"
if [ "$dry_run" = 0 ] && [ "${#sums_names[@]}" -gt 0 ] && [ -f "$sums" ] && [ ! -L "$sums" ]; then
  declare -A gone=()
  for name in "${sums_names[@]}"; do
    [ -e "$root/$name" ] || gone[$name]=1
  done
  tmp="$root/SHA256SUMS.prune-$$"
  dropped=0
  written=1
  {
    while IFS= read -r line || [ -n "$line" ]; do
      entry=${line%$'\r'}
      if [[ $entry =~ ^[0-9a-fA-F]+\ [\ *](\./)?(.+)$ ]] && [ -n "${gone[${BASH_REMATCH[2]}]:-}" ]; then
        dropped=$((dropped + 1))
        continue
      fi
      printf '%s\n' "$line"
    done <"$sums"
  } >"$tmp" || written=0
  if [ "$written" = 0 ]; then
    complain "could not write $tmp; the lines of the removed archives stay in $sums"
    rm -f -- "$tmp"
    rc=1
  elif [ "$dropped" -gt 0 ]; then
    if mv -f -- "$tmp" "$sums"; then
      say "SHA256SUMS: $dropped line(s) of removed archives dropped"
    else
      complain "could not rewrite $sums; its old lines stay"
      rm -f -- "$tmp"
      rc=1
    fi
  else
    rm -f -- "$tmp"
  fi
fi

if [ "$dry_run" = 1 ]; then
  say "done: ${#doomed[@]} would be removed, $kept kept"
else
  say "done: $removed removed, $kept kept"
fi
exit "$rc"
