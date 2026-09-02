#!/usr/bin/env bash
#
# Does this release note say what a release note has to say.
#
# Used twice and that is the point of it being a file: the workflow that
# GENERATES the note checks its own output with this, and the workflow that
# reads the PUBLISHED page checks that with the same list. Before this, the
# list lived inline in the second one and the first one simply wrote the
# sentences out - two copies of one fact with nothing comparing them.
#
# Reports every missing line rather than the first. A person reads a release
# page once, and a check that names one problem per run turns a list into a
# queue.
#
# Usage: note_says.sh <file with the note> [file with the list]
set -euo pipefail

note="${1:?usage: note_says.sh <note file> [list file]}"
list="${2:-.github/release-note-must-say.txt}"

test -f "$note" || {
  echo "note_says: there is no note at $note, so nothing was checked"
  exit 1
}

# An empty or missing list would let this pass on any page at all, which is
# the shape of a gate that never reads its own answer.
test -s "$list" || {
  echo "note_says: $list is missing or empty, so this would pass on any note"
  exit 1
}

missing=0
asked=0
while IFS= read -r promised || [ -n "$promised" ]; do
  case "$promised" in
    '' | '#'*) continue ;;
  esac
  asked=$((asked + 1))
  if ! grep -qF -- "$promised" "$note"; then
    echo "  the note never mentions: $promised"
    missing=$((missing + 1))
  fi
done < "$list"

if [ "$asked" = "0" ]; then
  echo "note_says: $list names nothing, so this checked nothing"
  exit 1
fi

if [ "$missing" != "0" ]; then
  echo "note_says: $missing of $asked things a release note has to say are not in $note."
  echo "A person reading that page is not told how to check what they downloaded."
  exit 1
fi

echo "note_says: the note says all $asked things it has to"
