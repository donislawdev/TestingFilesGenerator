#!/usr/bin/env bash
# Sign, notarise and staple the macOS archives of a release. Runs ON THE MAC.
#
#   sign_macos.sh <directory> <certificate-sha256> <notarytool-profile>
#
# Why this is a separate script on a separate machine
# ---------------------------------------------------
# The Developer ID key lives in a keychain on the owner's Mac and Apple's notary
# service will only talk to credentials stored there. The Windows card and this
# key are on two different machines, so the release is signed in two places and
# the checksums are written once, at the end, over everything.
#
# What it refuses, and why each refusal is there
# ----------------------------------------------
# Every one of these was measured on 2026-08-28, on real hardware, and several
# of them are refusals precisely because the failure is SILENT otherwise:
#
#  * a locked keychain. codesign then exits 1 with errSecInternalComponent and
#    leaves the file UNSIGNED, while the command looks like it did something.
#    An ssh session is its own security session - launchctl managername says
#    Background, not Aqua - so unlocking the keychain in the Mac's own Terminal
#    does NOT carry over here;
#  * a missing notarytool profile. Note that a LOCKED keychain reports the same
#    error as a missing profile, so this script separates the two rather than
#    blaming the owner for work they already did;
#  * a certificate that is not the pinned one. A second Developer ID on the same
#    machine signs just as willingly and the release looks identical;
#  * notarisation that came back anything but Accepted;
#  * a staple that did not take, or a bundle Gatekeeper still rejects. Without
#    this last one the script would happily hand back an archive that fails on
#    the first machine that downloads it.
set -uo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: sign_macos.sh <directory> <certificate-sha256> <profile>" >&2
  exit 2
fi

DIR="$1"
PIN="$(echo "$2" | tr '[:lower:]' '[:upper:]')"
PROFILE="$3"

say() { echo "  $*"; }
die() { echo "sign_macos: $*" >&2; exit 1; }

# --------------------------------------------------------------------------- #
# the keychain
# --------------------------------------------------------------------------- #
# Asked for on the terminal rather than taken as an argument or an environment
# variable. A password in argv is visible in ps to every process on the machine.
# This asks every time and does NOT claim to know whether it needed to, because
# measured on 2026-08-28 it cannot know: over ssh, show-keychain-info answers
# "User interaction is not allowed" (exit 36) even when the keychain is unlocked
# and signing is about to work. An earlier version of this used that as a test
# and so printed "the keychain is locked" at a moment when it was not - a
# sentence that reads like a diagnosis and is really just a blind spot.
#
# Asking unconditionally is cheap: unlocking an already unlocked keychain
# succeeds and changes nothing. The real proof that the key is reachable is
# codesign itself, further down, which refuses loudly rather than silently.
unlock_keychain() {
  echo
  echo "  An ssh session is its own security session and cannot see whether the"
  echo "  keychain is unlocked, so it asks. If it is already unlocked, press"
  echo "  Enter. Otherwise this is your Mac password."
  echo
  if ! security unlock-keychain "$HOME/Library/Keychains/login.keychain-db"; then
    die "the keychain would not unlock, so nothing has been signed.
Run this by hand on the Mac and try again:
  security unlock-keychain ~/Library/Keychains/login.keychain-db"
  fi
  say "keychain reachable"
}

# --------------------------------------------------------------------------- #
# the credentials, told apart from a locked keychain
# --------------------------------------------------------------------------- #
check_profile() {
  local out rc
  out="$(xcrun notarytool history --keychain-profile "$PROFILE" 2>&1)"
  rc=$?
  if [ $rc -eq 0 ]; then
    say "notarytool profile '$PROFILE' answers"
    return 0
  fi
  case "$out" in
    *keychainLocked*)
      die "the keychain locked itself again between two steps. Nothing signed." ;;
    *"No Keychain password item"*)
      die "there is no notarytool profile called '$PROFILE' on this Mac.
Create it once, in the Mac's own Terminal rather than over ssh:

  xcrun notarytool store-credentials \"$PROFILE\" --apple-id <your-apple-id> --team-id <your-team-id>

It asks for an APP-SPECIFIC password from appleid.apple.com, not the password
to the Apple account itself." ;;
  esac
  die "notarytool would not answer: $out"
}

# --------------------------------------------------------------------------- #
# the certificate, chosen by its own bytes
# --------------------------------------------------------------------------- #
# codesign selects by SHA-1 and SHA-1 is not worth pinning anything to, so the
# pin is a SHA-256 and the selector is derived from it here. That also settles
# the ambiguity of the name: the same certificate is installed in both the login
# and the system keychain, so "Developer ID Application" names two entries.
find_identity() {
  local sha1
  sha1="$(security find-certificate -a -c "Developer ID Application" -Z 2>/dev/null |
    awk -v pin="$PIN" '
      /^SHA-256 hash:/ { s256 = $3 }
      /^SHA-1 hash:/   { if (s256 == pin) { print $3; exit } }')"
  [ -n "$sha1" ] ||
    die "no Developer ID certificate on this Mac hashes to the pin $PIN.
A renewal is a DIFFERENT certificate and internal/legal has to move with it."
  security find-identity -v -p codesigning 2>/dev/null | grep -qi "$sha1" ||
    die "the pinned certificate is installed but has no private key here, so it cannot sign"
  echo "$sha1"
}

# The certificate that ACTUALLY signed a bundle, read back out of it.
# 🔴 Nothing here changes directory, and that is the whole of what this function
# learned on 2026-08-28, on the first real release.
#
# It used to cd into the temporary directory and pass the bundle path after
# that, because --extract-certificates looked like a file NAME. The path it was
# given is relative - sign_release.py passes a directory under the home
# directory - so after the cd it named nothing. codesign answered "No such file
# or directory", no certificate came out, and the script stopped the release
# saying the bundle "was signed by a DIFFERENT certificate, got none" about a
# bundle that was correctly signed, by the pinned certificate, with a timestamp.
# A refusal that is right about there being a problem and wrong about what it is
# costs more than a silent one, because it sends somebody to look at the card.
#
# The prefix takes a path. Measured rather than assumed: with the prefix written
# as "$tmp/cert" and the bundle path left exactly as it arrives, cert0 appears
# and hashes to the pinned digest.
certificate_of() {
  local bundle="$1" tmp
  tmp="$(mktemp -d)"
  codesign -d --extract-certificates="$tmp/cert" "$bundle" >/dev/null 2>&1
  if [ ! -f "$tmp/cert0" ]; then
    rm -rf "$tmp"
    echo "none"
    return
  fi
  shasum -a 256 "$tmp/cert0" | awk '{print toupper($1)}'
  rm -rf "$tmp"
}

# --------------------------------------------------------------------------- #
# one archive, all the way through
# --------------------------------------------------------------------------- #
sign_archive() {
  local archive="$1" identity="$2"
  local name work app
  name="$(basename "$archive")"
  work="${archive}.unpacked"
  rm -rf "$work"
  mkdir -p "$work"
  tar -xzf "$archive" -C "$work" || die "$name did not unpack"

  app="$(find "$work" -maxdepth 1 -name '*.app' -type d | head -1)"
  [ -n "$app" ] ||
    die "$name holds no .app bundle. The release workflow is supposed to build
one, because a bare binary cannot carry a notarisation ticket at all."

  say "$name"
  say "  signing $(basename "$app")"
  # --options runtime is the hardened runtime, and notarisation refuses without
  # it. --timestamp is what keeps the signature alive past the certificate.
  codesign --force --options runtime --timestamp --sign "$identity" "$app" >/dev/null 2>&1 ||
    die "codesign failed on $(basename "$app") in $name.
The usual cause is a locked keychain, which fails as errSecInternalComponent and
leaves the file unsigned. Unlock it on the Mac and run this again:
  security unlock-keychain ~/Library/Keychains/login.keychain-db"

  local actual
  actual="$(certificate_of "$app")"
  [ "$actual" = "$PIN" ] ||
    die "$(basename "$app") was signed by a DIFFERENT certificate
  expected $PIN
  got      $actual
Nothing has been handed back."

  say "  notarising, this takes a few minutes"
  # Apple takes a zip, a pkg or a dmg, and will not take the tar.gz we publish.
  # So the zip exists only to carry the bundle there. What gets stapled and
  # republished is the bundle itself.
  # The same trap as certificate_of, one step further on, and it was measured
  # the same day rather than met later: this cd'd into the unpacked directory
  # and then wrote to a path that was relative to where the script STARTED, so
  # the zip was never created and the next line would have stopped the release
  # with "could not zip". Without the cd, ditto is given both paths as they
  # arrive and --keepParent still puts tfg-gui.app at the top of the archive,
  # which is the shape notarisation wants - read back out of the zip.
  local zip="${work}/notarise.zip"
  ditto -c -k --keepParent "$app" "$zip" ||
    die "could not zip $(basename "$app") for notarisation"

  local out
  out="$(xcrun notarytool submit "$zip" --keychain-profile "$PROFILE" --wait 2>&1)"
  echo "$out" | grep -q "status: Accepted" ||
    die "notarisation did not come back Accepted for $name:
$out"
  rm -f "$zip"

  say "  stapling"
  xcrun stapler staple "$app" >/dev/null 2>&1 ||
    die "the ticket would not staple onto $(basename "$app") in $name"

  # The two questions worth asking after the fact, because a staple that did not
  # take and a bundle Gatekeeper rejects both look like success up to here.
  xcrun stapler validate "$app" >/dev/null 2>&1 ||
    die "$(basename "$app") does not validate against its own ticket"
  spctl -a -t exec "$app" >/dev/null 2>&1 ||
    die "Gatekeeper still rejects $(basename "$app") after signing and stapling"

  say "  repacking"
  rm -f "$archive"
  # -C and . so the archive keeps the shape it had, symlink included. The
  # symlink is how a person still types ./tfg instead of reaching into a bundle.
  tar -czf "$archive" -C "$work" . || die "could not repack $name"
  rm -rf "$work"
  say "  done, signed and stapled"
}

# --------------------------------------------------------------------------- #
main() {
  [ -d "$DIR" ] || die "$DIR is not a directory"

  echo "[1/4] the keychain"
  unlock_keychain

  echo "[2/4] the credentials"
  check_profile

  echo "[3/4] the certificate"
  local identity
  identity="$(find_identity)" || exit 1
  say "signing with the pinned certificate"

  echo "[4/4] the archives"
  local archives=()
  while IFS= read -r line; do
    archives+=("$line")
  done < <(find "$DIR" -maxdepth 1 -name '*_macos_*.tar.gz' | sort)

  if [ "${#archives[@]}" -ne 2 ]; then
    die "expected two macOS archives and found ${#archives[@]}.
One command line build and one window build carry a macOS program. Signing one
of two would publish an unsigned binary next to a signed one."
  fi

  local archive
  for archive in "${archives[@]}"; do
    sign_archive "$archive" "$identity"
  done

  echo
  echo "Both macOS archives are signed, notarised and stapled."
}

main "$@"
