#!/usr/bin/env bash
# Wrap a macOS binary in a .app bundle, because that is the only shape that can
# carry a notarisation ticket.
#
# Measured on 2026-08-28, on a real Developer ID certificate and two real
# notarisations - see docs, section 15 of the provenance plan:
#
#   * a bare Mach-O binary cannot be stapled at all. stapler exits 66 and says
#     it "is incapable of working with Document files", and Gatekeeper then
#     rejects the binary (spctl exit 3) even when it is signed AND notarised;
#   * a .dmg staples fine and does not help: the binary INSIDE a stapled dmg is
#     still rejected, because the ticket describes the container;
#   * a .app staples (exit 0) and is accepted (spctl exit 0), and the ticket
#     survives being packed into tar.gz and unpacked again.
#
# So the bundle is not decoration. It is the thing the ticket attaches to.
#
# A symlink goes next to the bundle so that a person still types ./tfg. The
# binary must NOT be lifted out of the bundle - a copy of it on its own is
# rejected with "invalid resource directory", because the signature covers the
# bundle, not the file.
set -euo pipefail

if [ "$#" -ne 4 ]; then
  echo "usage: make_app_bundle.sh <workdir> <binary> <bundle-id> <version>" >&2
  exit 2
fi

work="$1"
binary="$2"
bundle_id="$3"
version="$4"

if [ ! -f "${work}/${binary}" ]; then
  echo "make_app_bundle: ${work}/${binary} is not there, nothing to wrap" >&2
  exit 1
fi

app="${work}/${binary}.app"
mkdir -p "${app}/Contents/MacOS" "${app}/Contents/Resources"
mv "${work}/${binary}" "${app}/Contents/MacOS/${binary}"
chmod +x "${app}/Contents/MacOS/${binary}"

# The icon, and it is refused rather than skipped when it is missing.
#
# A bundle without one is not a bundle that looks slightly worse - macOS draws a
# blank sheet of paper for it in the Finder and the Dock, which is what a program
# it knows nothing about looks like. O154, reported by the owner on 2026-08-28.
#
# The file is in the repository rather than made here with sips and iconutil,
# because this script runs on ubuntu: the command line archives are cross
# compiled, so Apple's tools are not there. tools/appicon.py writes it, at every
# size macOS asks for, and Apple's own iconutil was asked whether it accepts
# what came out.
here="$(cd "$(dirname "$0")" && pwd)"
icon="${here}/../../internal/gui/icon/chickpea.icns"
if [ ! -f "${icon}" ]; then
  echo "make_app_bundle: no icon at ${icon}, so the bundle would show a blank page" >&2
  exit 1
fi
cp "${icon}" "${app}/Contents/Resources/icon.icns"

# LSMinimumSystemVersion is 11.0 because this project builds darwin/arm64 only
# and Apple silicon starts there. NSHighResolutionCapable keeps the window from
# being drawn blurry on a Retina display, and costs the command line nothing.
cat > "${app}/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>${binary}</string>
	<key>CFBundleIconFile</key>
	<string>icon</string>
	<key>CFBundleIdentifier</key>
	<string>${bundle_id}</string>
	<key>CFBundleName</key>
	<string>${binary}</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>${version}</string>
	<key>CFBundleVersion</key>
	<string>${version}</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
</dict>
</plist>
PLIST

# Relative, so it still points at the binary after the archive is unpacked
# anywhere. Measured: the symlink survives tar.gz and running through it works.
ln -s "${binary}.app/Contents/MacOS/${binary}" "${work}/${binary}"

# On a filesystem that cannot make symlinks, ln quietly makes a COPY instead,
# and a copy is far worse than nothing here: a binary sitting outside the bundle
# is rejected by Gatekeeper with "invalid resource directory", because the
# signature covers the bundle rather than the file. The archive would then carry
# something that looks like the program and refuses to run. Measured 2026-08-28,
# and measured by accident - a build on Windows produced exactly this.
if [ ! -L "${work}/${binary}" ]; then
  echo "make_app_bundle: ${binary} beside the bundle is a copy, not a symlink." >&2
  echo "A copy of the binary outside the bundle is rejected by Gatekeeper." >&2
  exit 1
fi

echo "wrapped ${binary} in ${binary}.app (${bundle_id}, ${version})"
