#!/usr/bin/env bash
# Puts the software renderer beside the window binary, for the Windows
# archive: the two files of Mesa's llvmpipe that the window loads when the
# graphics driver offers no OpenGL 2.1.
#
# Usage: fetch_software_renderer.sh <directory the window binary is in>
#
# The release of pal1000/mesa-dist-win to take them from, the SHA-256 of that
# release's archive, and the two files with their own sums, come from
# .github/mesa-dist-win and nowhere else. The same version and sums stand in
# internal/legal/companions.go, which is what the notices and the bill of
# materials are rendered from, and a guard holds the two files equal - so a
# bump here without the review there is refused before it is built.
#
# The archive's sum is checked BEFORE anything is unpacked. A download that
# does not match is an archive nobody reviewed, and unpacking it first would
# put two files nobody reviewed next to a program that loads them by path.
#
# Exactly two files, taken from one directory of the archive, into one
# directory beside the program. The names are the ones the window looks for -
# see internal/gui/software.go - and the guard above holds these to the
# registry as well, so a renamed file cannot ship under the old name.
set -euo pipefail

into="${1:?usage: fetch_software_renderer.sh <directory>}"
pin=".github/mesa-dist-win"

version="$(grep '^version=' "${pin}" | cut -d= -f2-)"
archive="$(grep '^archive=' "${pin}" | cut -d= -f2-)"
sha256="$(grep '^sha256=' "${pin}" | cut -d= -f2-)"
for value in "${version}" "${archive}" "${sha256}"; do
  if [ -z "${value}" ]; then
    echo "fetch_software_renderer: ${pin} does not name the version, the archive and the sum" >&2
    exit 1
  fi
done

# Beside the target rather than under /tmp: on one machine /tmp was a
# directory this user could write and not read back, and a download that
# cannot be summed is a download that cannot be trusted.
mkdir -p "${into}"
fetched="$(mktemp -d "${into}/.software-renderer.XXXXXX")"
curl --silent --show-error --fail --location --retry 3 \
  --output "${fetched}/${archive}" \
  "https://github.com/pal1000/mesa-dist-win/releases/download/${version}/${archive}"
(cd "${fetched}" && echo "${sha256}  ${archive}" | sha256sum -c -)

# The files, each with its own sum, from the same pin. The archive's sum
# already covers them, and this is the half a person can check against the
# notices without downloading seventy megabytes: the notices name these two
# sums, and so does the registry the notices are rendered from.
mkdir -p "${into}/opengl"
grep '^file=' "${pin}" | cut -d= -f2- | while read -r inside sum; do
  7z e -bso0 -o"${into}/opengl" "${fetched}/${archive}" "${inside}"
  (cd "${into}/opengl" && echo "${sum}  $(basename "${inside}")" | sha256sum -c -)
done
rm -rf "${fetched}"

# Two files and no more, or the archive is not what the notices describe.
count="$(find "${into}/opengl" -type f | wc -l | tr -d ' ')"
if [ "${count}" != "2" ]; then
  echo "fetch_software_renderer: expected two files under opengl and found ${count}" >&2
  exit 1
fi
ls -l "${into}/opengl"
