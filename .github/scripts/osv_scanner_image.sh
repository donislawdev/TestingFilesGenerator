#!/usr/bin/env bash
#
# Is the scanner image on this runner the one this repository recorded.
#
# The OSV actions are pinned by commit, and each of them names its Docker image
# by a tag, ghcr.io/google/osv-scanner-action:vX.Y.Z, which whoever owns it can
# push again. Pinning the action does not stop that. The runner pulls the image
# while it sets the job up, before the first step, so by the time this runs the
# image is here and has not run yet. If its digest is not the one recorded in
# the file, this fails, and the scan steps after it do not start. Measured on
# the runner 2026-10-15, docs/OSV-2-2026-10-15.md.
#
# Fails rather than passes when there is no image of the scanner at all. That
# would mean the runner stopped pulling it during set-up, and a check that found
# nothing to check has not checked anything.
#
# A Dependabot update of google/osv-scanner-action brings a new image and stops
# here until its digest is written to the file. That is on purpose: it is the
# moment somebody reads what the new release changed.
#
# Usage: osv_scanner_image.sh [file with the digest]
set -euo pipefail

file="${1:-.github/osv-scanner-digest}"
repository="ghcr.io/google/osv-scanner-action"

expected=$(tr -d '[:space:]' 2>/dev/null < "$file" || true)
if ! [[ "$expected" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "::error::$file does not hold one sha256 digest, so there is nothing to compare the scanner image with."
  exit 1
fi

ids=$(docker image ls --no-trunc --quiet "$repository")
if [ -z "$ids" ]; then
  echo "::error::No image of $repository is on this runner. The runner used to pull it while setting the job up, before any step, and this check relies on that. Read the Set up job section of this run."
  exit 1
fi

status=0
for id in $(printf '%s\n' "$ids" | sort -u); do
  digests=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$id")
  if printf '%s\n' "$digests" | grep -qxF "$repository@$expected"; then
    echo "osv_scanner_image: $id is $repository@$expected"
    continue
  fi
  echo "::error::The scanner image $id is not the one recorded in $file."
  echo "It was pulled as:"
  printf '%s\n' "$digests" | sed 's/^/  /'
  echo "If google/osv-scanner-action was just updated, read what its new release changed and write the new digest to $file."
  echo "If it was not, the image behind the tag has changed, and nothing should run it."
  status=1
done
exit "$status"
