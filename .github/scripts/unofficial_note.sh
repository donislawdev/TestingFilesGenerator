#!/usr/bin/env bash
# Write the note that travels inside every build-on-demand archive.
#
# Why a script rather than a heredoc in the workflow. The same sentences go into
# the command line archives and the window archives, built by two different jobs
# on four different runners, and a note that says one thing in one archive and
# something else in the other is worse than no note. One file, called twice.
#
# It is also the only thing in the archive that can be honest about which code
# this is. internal/version is a Go const, so it cannot be stamped at link time
# and a build from a fix branch reports whatever version that branch inherited.
# The commit below is the fact - the version string inside the binary is not.
#
# Usage: unofficial_note.sh <output path> <short commit>
set -euo pipefail

out="${1:?first argument is the file to write}"
commit="${2:?second argument is the short commit}"

repo="${GITHUB_REPOSITORY:-donislawdev/TestingFilesGenerator}"
ref="${GITHUB_REF_NAME:-unknown branch}"
run="${GITHUB_RUN_ID:-}"
built="$(date -u '+%Y-%m-%d %H:%M UTC')"

{
  echo "UNOFFICIAL BUILD - this is not a release"
  echo "========================================"
  echo
  echo "Built on demand from commit ${commit} of ${ref}, on ${built}."
  if [ -n "${run}" ]; then
    echo "Run: https://github.com/${repo}/actions/runs/${run}"
  fi
  echo
  echo "What this is. Somebody asked for a build of work that has not been"
  echo "released yet - usually a fix for something they reported. It is the code"
  echo "at the commit above and nothing more."
  echo
  echo "What it is NOT."
  echo
  echo "  - It is NOT signed. There is no Windows code signing signature and no"
  echo "    Apple notarisation. Windows SmartScreen and macOS Gatekeeper will"
  echo "    both object to it, and they are right to."
  echo "  - It carries NO provenance attestation and NO bill of materials."
  echo "    A real release carries both and you can verify them."
  echo "  - The version it reports is NOT a claim to be that release. The"
  echo "    version is compiled in as a constant, so a build from a branch"
  echo "    reports the version that branch started from. The commit above is"
  echo "    the only thing that identifies this build."
  echo
  echo "Do not pass this on as a release, and do not keep it once the fix ships."
  echo "Releases live at https://github.com/${repo}/releases - they are signed,"
  echo "they carry checksums you can check, and they say which version they are."
} > "${out}"
