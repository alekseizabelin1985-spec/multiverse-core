#!/usr/bin/env bash
# =============================================================================
# scripts/coverage-gate.sh — the coverage floor of the core packages
# (ADR-010 p. 5, infrastructure.md v0.3 §3.1, job `unit`)
# =============================================================================
# Usage:
#   scripts/coverage-gate.sh <min percent> <package> [package...]
# Environment:
#   COVERAGE_PROFILE   the profile to read (default: coverage.out)
#
# The floor is per package, not over the module: an average hides a context
# without a single test behind a well covered neighbour, and the packages this
# gate names — internal/{state,mechanics,swarm,llm,replay} — are the ones whose
# defects a player sees.
#
# A package that does not exist yet is a warning, not a failure: the gate is
# written in wave 0 and the contexts arrive in waves 1 and 2 (ADR-010 p. 5,
# "packages that are not there are skipped with a warning"). The same holds for
# a package that exists but has no statements — there is nothing to measure.
#
# The percentage is computed from the profile rather than from
# `go tool cover -func`, which reports one line per function: statements per
# function differ, so an unweighted average of those lines is not the coverage
# of the package. A profile line is
#
#   <import path>/<file>.go:<from>,<to> <statements> <times executed>
#
# so the coverage of a package is the sum of the statements of its files that
# ran, over the sum of all of them.
#
# Runs in Git Bash on Windows as well as on ubuntu-latest: awk only, no jq.
# =============================================================================
set -euo pipefail

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo_root"

if [ $# -lt 2 ]; then
  printf 'usage: %s <min percent> <package> [package...]\n' "$0" >&2
  exit 2
fi

min=$1
shift
if ! printf '%s' "$min" | grep -Eq '^[0-9]+([.][0-9]+)?$'; then
  printf 'coverage-gate: %s is not a percentage\n' "$min" >&2
  exit 2
fi

profile=${COVERAGE_PROFILE:-coverage.out}
if [ ! -f "$profile" ]; then
  printf 'coverage-gate: no profile at %s — run go test -coverprofile=%s ./... first\n' \
    "$profile" "$profile" >&2
  exit 2
fi

module=$(go list -m)
failed=0

for pkg in "$@"; do
  # ./<pkg>/... rather than ./<pkg>: the floor covers the subpackages of a
  # context too, and `go list` is what decides whether the package is there.
  if ! go list "./${pkg}/..." >/dev/null 2>&1; then
    printf 'coverage-gate: %s does not exist yet — skipped\n' "$pkg"
    continue
  fi

  read -r covered total < <(
    awk -v prefix="${module}/${pkg}/" '
      NR > 1 {
        colon = index($1, ":")
        file = substr($1, 1, colon - 1)
        if (index(file, prefix) == 1) {
          total += $2
          if ($3 > 0) covered += $2
        }
      }
      END { printf "%d %d\n", covered + 0, total + 0 }
    ' "$profile"
  )

  if [ "$total" -eq 0 ]; then
    printf 'coverage-gate: %s has no statements in the profile — skipped\n' "$pkg"
    continue
  fi

  percent=$(awk -v c="$covered" -v t="$total" 'BEGIN { printf "%.1f", 100 * c / t }')
  if awk -v p="$percent" -v m="$min" 'BEGIN { exit !(p + 0 < m + 0) }'; then
    printf 'coverage-gate: %s %s%% < %s%% (%s of %s statements)\n' \
      "$pkg" "$percent" "$min" "$covered" "$total" >&2
    failed=1
  else
    printf 'coverage-gate: %s %s%% (%s of %s statements)\n' \
      "$pkg" "$percent" "$covered" "$total"
  fi
done

exit "$failed"
