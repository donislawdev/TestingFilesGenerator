#!/usr/bin/env bash
# Which guards one part of a job runs, and whether it ran every one of them.
#
#   guard_part.sh pick PART PARTS LISTED [flag...]   writes mine.txt
#   guard_part.sh ran LOG                            fails on a test never started
#
# The guards are split over the parts of three jobs of ci.yml - the tests on
# each system, coverage and the race detector - because one process of them
# reached the eighteen minutes it is given on Windows and macOS (O361). This is
# the one place the three agree on how a part is picked and how it is checked.
#
# pick asks the test binary for its tests, with the flags it will be built with
# (the race job passes -race, because a file in the package builds only without
# it), and keeps the ones race_parts.py lays into this part by the weights in
# .github/race-weights. Listed rather than written down, so a new guard cannot
# be missed by a list kept by hand. It fails when the index the job was given is
# not the one the matrix lists, or below the total, and when the part is given
# no test - go test with nothing to run passes.
#
# ran fails when the log of a part (go test -v) never started a test the part
# was given, by name rather than by count, so a stray line that looks like a
# start cannot stand in for a test that did not run.
set -euo pipefail

# python3 where it runs, python otherwise: the Windows runner may carry only
# the second, and a python3 there can be the stub that sends you to the store.
python_here() {
  local py
  for py in python3 python; do
    if "$py" -c 'import sys' >/dev/null 2>&1; then
      echo "$py"
      return
    fi
  done
  echo "neither python3 nor python runs here, and the split of the guards is a Python script." >&2
  exit 1
}

pick() {
  local part=$1 parts=$2 listed=$3
  shift 3
  if [ "$part" != "$listed" ] || [ "$part" -ge "$parts" ]; then
    echo "job $part of $parts is part $listed in the matrix, and the split needs the two equal and below the total."
    echo "List strategy.matrix.part as 0, 1, 2 and on, in order, and check the context names in this step's env."
    exit 1
  fi
  go test -tags "$(cat .github/build-tags)" ./internal/guard/ "$@" -list '.*' > listed.txt || { cat listed.txt; exit 1; }
  grep -E '^(Test|Fuzz|Example)' listed.txt > names.txt || { echo "the test binary listed no Test, Fuzz or Example function. What it printed:"; cat listed.txt; exit 1; }
  # Without carriage returns: Python on Windows ends its lines with them, and a
  # name with one never equals the name go test prints when it starts it.
  "$(python_here)" .github/scripts/race_parts.py split names.txt .github/race-weights "$part" "$parts" | tr -d '\r' > mine.txt
  local planned
  planned=$(wc -l < mine.txt)
  echo "part $part of $parts runs $planned of $(wc -l < names.txt) tests"
  if [ "$planned" -eq 0 ]; then
    echo "part $part of $parts was given no test - the checksum of no listed name lands here."
    echo "With this few tests, use fewer parts in strategy.matrix.part."
    exit 1
  fi
}

ran() {
  local log=$1 missing
  missing=$(comm -23 <(sort mine.txt) <(sed -n 's/^=== RUN   \([^/]*\)$/\1/p' "$log" | tr -d '\r' | sort -u))
  if [ -n "$missing" ]; then
    echo "this part was given $(wc -l < mine.txt) tests and never ran these:"
    echo "$missing"
    echo "Each of these is a name the part did not reach, or a test the binary skipped before it started."
    exit 1
  fi
}

case "${1:-}" in
  pick) shift; pick "$@" ;;
  ran) shift; ran "$@" ;;
  *) echo "usage: guard_part.sh pick PART PARTS LISTED [flag...] | ran LOG" >&2; exit 2 ;;
esac
