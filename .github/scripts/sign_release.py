#!/usr/bin/env python3
"""Sign a release on the two machines that hold its keys, then hand it back.

    python .github/scripts/sign_release.py v0.2.0 --macos-host user@mac
    python .github/scripts/sign_release.py v0.2.0 --dry-run   # everything but sign and upload

Why this is a step a person runs
--------------------------------
The Windows key lives on a cryptographic card in a USB reader and cannot be
exported - that is the whole value of it - so no GitHub hosted runner can ever
reach it. A self hosted runner could, and this is a PUBLIC repository, where a
self hosted runner is a machine strangers can aim a pull request at. So the build
happens where builds belong and the signature happens where the card is, and this
script is the seam between them.

The Apple key is the same problem with a different shape: Apple's notary service
only talks to credentials held in a keychain, and that keychain is on the owner's
Mac. So five of the eight archives are signed here, in two places, and the
checksums are written once at the end over everything.

What it does, in order, and what it refuses
-------------------------------------------
 1. downloads the unsigned build the release workflow produced for this tag;
 2. **verifies that build's provenance** before touching it. Signing something
    you did not check is how a supply chain gets a signature on it;
 3. signs the three Windows programs with the card, each with an RFC 3161
    timestamp - **without a timestamp a signature dies when the certificate
    expires**, and this one expires in 2027;
 4. reads the certificate back OUT of each signed file and refuses to go on
    unless it hashes to the pin in internal/legal. A second code signing
    certificate on the same machine - a renewal, a test one, one from another
    project - signs just as willingly and the release page looks identical;
 5. hands the two macOS archives to the Mac, which signs the .app bundle inside
    each one, notarises it with Apple and staples the ticket on. A bare macOS
    binary cannot be stapled at all, so without this those two archives are
    refused by Gatekeeper even though they are signed;
 6. repacks those archives and writes verify-SHA256SUMS.txt over everything it
    is about to publish, signed and unsigned alike;
 7. uploads the lot to the DRAFT release and asks attest-release.yml for the
    statement about the signed bytes;
 8. waits for that statement and confirms the draft is complete. Until this
    existed in the project this came from, the script ended at "dispatched, go
    look" - and a draft missing one file looks almost exactly like a finished one.

Nothing here publishes. The release stays a draft until a person reads it and
presses the button.
"""
import argparse
import datetime
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import zipfile

REPO = "donislawdev/TestingFilesGenerator"
ATTEST_WORKFLOW = "attest-release.yml"
TIMESTAMP_URL = "http://time.certum.pl/"

# The programs that get a signature. Everything else in the build travels
# untouched: a Linux or a macOS archive is the finished article here, and the
# statement the build made about it stays true because its bytes do not move.
SIGNED_ARCHIVES = ("windows_amd64.zip", "windows_arm64.zip")

# The macOS half. These are signed on the owner's Mac rather than here, because
# Apple's notary service only talks to credentials held in a keychain there.
#
# A bare binary cannot carry a notarisation ticket, measured on 2026-08-28, so
# the release workflow wraps both macOS binaries in .app bundles and it is the
# bundle inside each archive that gets signed and stapled.
MACOS_SUFFIX = "_macos_arm64.tar.gz"
NOTARY_PROFILE = "tfg-notary"

# The four files a person uses to CHECK a download, rather than download itself.
#
# The prefix is the only thing that puts them at the end of the release page.
# Measured on 2026-08-28 against a draft release: GitHub orders the download
# list ALPHABETICALLY BY FILE NAME and nothing else moves it - not the upload
# order, not the asset id, not the label, and the sort ignores letter case.
# Without a prefix SHA256SUMS.txt sorts FIRST, above every archive, and the
# .json files land in the middle, between the window archives and the command
# line ones, because a dot sorts before an underscore.
#
# verify- rather than a number, because the word is true: these four are what
# you verify a download with. A number would sort just as well and would say
# nothing.
AUX_PREFIX = "verify-"

# Where the pin lives. Read out of the Go source rather than copied here,
# because two written copies of one digest is exactly the drift this pin exists
# to catch.
PIN_FILE = os.path.join("internal", "legal", "codesign.go")

# The OID, not the display name. Measured on this machine on 2026-08-28:
# filtering certificates by the friendly name "Code Signing" returns NOTHING on
# a Polish Windows, which calls it "Podpisywanie kodu" - and the script would
# then report a missing card while the card was plugged in and working.
CODE_SIGNING_OID = "1.3.6.1.5.5.7.3.3"


def run(argv, **kw):
    """Run something and let it print. A failure raises."""
    print("    $ %s" % " ".join(str(a) for a in argv))
    return subprocess.run(argv, check=True, **kw)


def powershell(script):
    out = subprocess.run(
        ["powershell", "-NoProfile", "-NonInteractive", "-Command", script],
        capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit("sign_release: powershell failed:\n%s" % out.stderr.strip())
    return out.stdout


def pinned_digest(name="CodeSigningSHA256"):
    """A certificate digest this project signs with, read from the Go source."""
    with open(PIN_FILE, encoding="utf-8") as handle:
        body = handle.read()
    found = re.search(r'%s = "([0-9a-f]{64})"' % name, body)
    if not found:
        raise SystemExit("sign_release: no %s in %s" % (name, PIN_FILE))
    return found.group(1)


def find_signtool():
    """The newest x64 signtool.exe from the Windows SDK, or a clear refusal."""
    kits = r"C:\Program Files (x86)\Windows Kits\10\bin"
    if os.path.isdir(kits):
        for version in sorted(os.listdir(kits), reverse=True):
            candidate = os.path.join(kits, version, "x64", "signtool.exe")
            if os.path.isfile(candidate):
                return candidate
    raise SystemExit(
        "sign_release: no signtool.exe under %s - install the Windows SDK "
        "(the Signing Tools component is enough)." % kits)


def expiry_notice(not_after, now):
    """What to say about a certificate that will not last forever."""
    if not not_after:
        return ["  the certificate does not say when it expires"]
    try:
        when = datetime.datetime.fromisoformat(str(not_after).replace("Z", "+00:00"))
    except ValueError:
        return ["  cannot read the expiry date: %s" % not_after]
    if when.tzinfo:
        when = when.replace(tzinfo=None)
    days = (when - now).days
    if days < 0:
        raise SystemExit(
            "sign_release: the certificate expired on %s. Nothing can be signed with it.\n"
            "A renewal is a DIFFERENT certificate, so internal/legal/codesign.go has to "
            "move with it." % when.date())
    if days < 90:
        return ["  %d days left on this certificate." % days,
                "     A renewal is a DIFFERENT certificate, so the pin in "
                "internal/legal/codesign.go goes with it."]
    return ["  %d days left on this certificate" % days]


def signing_thumbprint(pin):
    """The SHA-1 thumbprint of the certificate whose DER bytes hash to the pin.

    Two digests, one source of truth. signtool selects by SHA-1 because that is
    the only selector it takes, and the repository pins SHA-256 because that is
    the digest worth pinning. Resolving one to the other here means the two can
    never drift apart in a configuration file.
    """
    script = (
        "$out = @(); "
        "Get-ChildItem Cert:\\CurrentUser\\My, Cert:\\LocalMachine\\My "
        "-ErrorAction SilentlyContinue | Where-Object { "
        "  $_.Extensions.EnhancedKeyUsages.Value -contains '%s' } | ForEach-Object { "
        "  $h = [System.Security.Cryptography.SHA256]::Create().ComputeHash($_.RawData); "
        "  $out += [pscustomobject]@{ "
        "    sha256 = (($h | ForEach-Object { $_.ToString('x2') }) -join ''); "
        "    thumb = $_.Thumbprint; subject = $_.Subject; "
        "    notAfter = $_.NotAfter.ToString('s') } "
        "}; $out | ConvertTo-Json -Compress" % CODE_SIGNING_OID
    )
    entries = json.loads(powershell(script).strip() or "[]")
    if isinstance(entries, dict):
        entries = [entries]
    for entry in entries:
        if entry.get("sha256") == pin:
            # The common name only. The rest of the subject carries the owner's
            # town and region, and printing it into a terminal somebody pastes
            # into a chat helps nobody.
            print("  certificate: %s" % entry.get("subject", "").split(",")[0])
            for line in expiry_notice(entry.get("notAfter"), datetime.datetime.now()):
                print(line)
            return entry["thumb"]
    raise SystemExit(
        "sign_release: the pinned certificate (%s...) is not in the Windows store.\n"
        "Plug in the card reader and check the card manager can see it. If the "
        "certificate was renewed, internal/legal/codesign.go has to move with it."
        % pin[:16])


def certificate_of(path):
    """The sha256 of the certificate that actually signed a file."""
    script = (
        "$s = Get-AuthenticodeSignature -LiteralPath '%s'; "
        "if ($s.Status -ne 'Valid') { Write-Error ('signature status: ' + $s.Status); exit 1 }; "
        "$h = [System.Security.Cryptography.SHA256]::Create()"
        ".ComputeHash($s.SignerCertificate.RawData); "
        "(($h | ForEach-Object { $_.ToString('x2') }) -join '')" % path
    )
    return powershell(script).strip()


def sha256_of(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def gh_json(*args):
    out = subprocess.run(["gh", *args], capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit("sign_release: gh %s failed:\n%s" % (args[0], out.stderr.strip()))
    return json.loads(out.stdout or "null")


# --------------------------------------------------------------------------- #
# the phases
# --------------------------------------------------------------------------- #

def fetch_build(tag, into):
    """The build the release workflow made for this tag.

    A missing artefact is the ordinary case rather than a crash: the tag has
    not been pushed, or the build is still running, or it failed. Saying so
    beats a traceback, because the person reading it is holding a card reader
    and wondering which half of this is broken.
    """
    if os.path.isdir(into):
        shutil.rmtree(into)
    os.makedirs(into)
    try:
        run(["gh", "run", "download", "--repo", REPO, "--name",
             "unsigned-build-%s" % tag, "--dir", into])
    except subprocess.CalledProcessError:
        raise SystemExit(
            "sign_release: there is no build called unsigned-build-%s.\n"
            "Either the tag has not been pushed, or release.yml has not finished, "
            "or it failed. Look at the run before signing anything." % tag)
    names = sorted(os.listdir(into))
    print("  %d file(s): %s" % (len(names), ", ".join(names)))
    return names


def verify_before_touching(directory):
    """Refuse to sign a build whose own statement does not check out.

    This is the step that makes the rest worth anything. Signing something
    you did not check is how a supply chain gets a signature on it - the whole
    attack is to hand the person with the key something that is not what they
    think it is.
    """
    bundle = os.path.join(directory, "build.provenance.sigstore.json")
    if not os.path.isfile(bundle):
        raise SystemExit(
            "sign_release: the build carries no provenance bundle, so there is "
            "nothing to check it against. Do not sign it.")
    archives = [n for n in sorted(os.listdir(directory))
                if n.endswith(".zip") or n.endswith(".tar.gz")]
    if not archives:
        raise SystemExit("sign_release: the build carries no archives at all")
    for name in archives:
        run(["gh", "attestation", "verify", os.path.join(directory, name),
             "-R", REPO, "--bundle", bundle])
    print("  %d archive(s) verified against the build's own statement" % len(archives))


def windows_archives(directory):
    """The archives that get a signature, refusing a surprise in the count."""
    found = [n for n in sorted(os.listdir(directory))
             if any(n.endswith(suffix) for suffix in SIGNED_ARCHIVES)]
    if len(found) != 3:
        raise SystemExit(
            "sign_release: expected three Windows archives to sign and found %d: %s\n"
            "Two command line builds and one window build carry a Windows program. "
            "Signing two of three would publish an unsigned binary next to signed ones."
            % (len(found), ", ".join(found) or "none"))
    return found


def sign_archive(path, thumbprint, pin, signtool, dry_run):
    """Sign the program inside one archive and put the archive back together."""
    work = path + ".unpacked"
    if os.path.isdir(work):
        shutil.rmtree(work)
    os.makedirs(work)
    with zipfile.ZipFile(path) as archive:
        archive.extractall(work)
    programs = [n for n in sorted(os.listdir(work)) if n.endswith(".exe")]
    if len(programs) != 1:
        raise SystemExit("sign_release: %s holds %d programs, expected one"
                         % (os.path.basename(path), len(programs)))
    program = os.path.join(work, programs[0])

    command = [signtool, "sign", "/sha1", thumbprint, "/fd", "sha256",
               "/tr", TIMESTAMP_URL, "/td", "sha256", "/v", program]
    if dry_run:
        print("    DRY RUN, would run: %s" % " ".join(command))
    else:
        run(command)
        run([signtool, "verify", "/pa", "/v", program])
        actual = certificate_of(program)
        if actual != pin:
            raise SystemExit(
                "sign_release: %s was signed by a DIFFERENT certificate\n"
                "  expected %s\n  got      %s\nNothing has been uploaded."
                % (programs[0], pin, actual))

    os.remove(path)
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        for name in sorted(os.listdir(work)):
            archive.write(os.path.join(work, name), name)
    shutil.rmtree(work)
    print("    %s: %s signed and repacked" % (os.path.basename(path), programs[0]))


def macos_archives(directory):
    """The macOS archives, refusing a surprise in the count."""
    found = [n for n in sorted(os.listdir(directory)) if n.endswith(MACOS_SUFFIX)]
    if len(found) != 2:
        raise SystemExit(
            "sign_release: expected two macOS archives and found %d: %s\n"
            "One command line build and one window build carry a macOS program. "
            "Signing one of two would publish an unsigned binary next to a signed one."
            % (len(found), ", ".join(found) or "none"))
    return found


def sign_macos(tag, directory, host, dry_run):
    """Hand the macOS archives to the Mac, and take back signed ones.

    Apple's notary service only talks to credentials in a keychain, and the
    Developer ID key is in one on the owner's Mac. So this half of the signing
    happens over ssh, the same way the Windows half happens at the card.

    -t because the script over there asks for the keychain password on the
    terminal. A password given that way is not in argv, so it is not in ps, and
    it is nowhere in this repository.

    The address of the Mac is an argument rather than a constant. A machine
    address is not something a public repository should carry.
    """
    archives = macos_archives(directory)
    if dry_run:
        print("  DRY RUN, would sign %d macOS archive(s) on %s"
              % (len(archives), host))
        return
    remote = "tfg-signing-%s" % tag
    run(["ssh", host, "rm -rf %s && mkdir -p %s/dist" % (remote, remote)])
    run(["scp", os.path.join(".github", "scripts", "sign_macos.sh"),
         "%s:%s/" % (host, remote)])
    for name in archives:
        run(["scp", os.path.join(directory, name), "%s:%s/dist/" % (host, remote)])
    run(["ssh", "-t", host, "bash %s/sign_macos.sh %s/dist %s %s"
         % (remote, remote, pinned_digest("AppleDeveloperIDSHA256"), NOTARY_PROFILE)])
    for name in archives:
        run(["scp", "%s:%s/dist/%s" % (host, remote, name), directory])
    run(["ssh", host, "rm -rf %s" % remote])
    print("  %d macOS archive(s) signed, notarised and stapled" % len(archives))


def name_for_publication(directory, tag):
    """Give the build's own files the names a person will see, or drop them.

    build.sha256 was the subject list for the statement the runner made. It
    describes archives that no longer exist in that form, so publishing it beside
    the real checksum file would put two lists of checksums on one page and one of
    them would be wrong about half the files.

    The provenance bundle does stay, under a name a person can use: it still
    describes the five archives nothing signed, byte for byte.
    """
    build_sums = os.path.join(directory, "build.sha256")
    # Not renamed and not published: this one names bytes that no longer exist
    # in that form, and it is deleted just below.
    if os.path.isfile(build_sums):
        os.remove(build_sums)
    made = os.path.join(directory, "build.provenance.sigstore.json")
    if os.path.isfile(made):
        os.rename(made, os.path.join(
            directory, "verify-tfg_%s.provenance.sigstore.json" % tag.lstrip("v")))


def write_checksums(directory):
    """The checksum file over what is about to be published, and its own digest."""
    published = sorted(os.listdir(directory))
    lines = []
    for name in published:
        lines.append("%s  %s\n" % (sha256_of(os.path.join(directory, name)), name))
    sums = os.path.join(directory, AUX_PREFIX + "SHA256SUMS.txt")
    with open(sums, "w", encoding="utf-8", newline="\n") as handle:
        handle.writelines(lines)
    print("  %d file(s) listed in %sSHA256SUMS.txt" % (len(published), AUX_PREFIX))
    return sha256_of(sums)


def upload_to_draft(tag, directory, dry_run):
    """Everything a person downloads, onto the draft."""
    files = [os.path.join(directory, n) for n in sorted(os.listdir(directory))
             if not n.endswith(".unpacked")]
    if dry_run:
        print("  DRY RUN, would upload %d file(s)" % len(files))
        return
    run(["gh", "release", "upload", tag, "--repo", REPO, "--clobber", *files])


def ask_for_the_statement(tag, digest, dry_run):
    """Hand the signed bytes back to a workflow, which can sign a statement."""
    if dry_run:
        print("  DRY RUN, would dispatch %s for %s" % (ATTEST_WORKFLOW, digest[:16]))
        return
    run(["gh", "workflow", "run", ATTEST_WORKFLOW, "--repo", REPO,
         "-f", "tag=%s" % tag, "-f", "digest=%s" % digest])


def confirm_draft(tag, wait_seconds, dry_run):
    """Wait for the statement and check the draft is actually complete.

    Without this the script ends at "dispatched, go look" - and a draft
    missing one file looks almost exactly like a finished one.
    """
    if dry_run:
        print("  DRY RUN, not waiting")
        return
    deadline = time.time() + wait_seconds
    while True:
        release = gh_json("release", "view", tag, "--repo", REPO,
                          "--json", "assets,isDraft")
        names = sorted(asset["name"] for asset in release.get("assets", []))
        has_sbom_statement = any(n.endswith(".sbom.sigstore.json") for n in names)
        if has_sbom_statement:
            break
        if time.time() > deadline:
            raise SystemExit(
                "sign_release: the statement about the signed bytes has not arrived after "
                "%d seconds.\nThe draft holds: %s\nLook at the run of %s before publishing "
                "anything." % (wait_seconds, ", ".join(names), ATTEST_WORKFLOW))
        print("    waiting for %s ..." % ATTEST_WORKFLOW)
        time.sleep(10)

    release = gh_json("release", "view", tag, "--repo", REPO, "--json", "assets,isDraft")
    names = sorted(asset["name"] for asset in release.get("assets", []))
    missing = []
    if sum(1 for n in names if n.endswith((".zip", ".tar.gz"))) != 8:
        missing.append("eight archives")
    # Each of the four is asked for WITH its prefix, not by suffix alone. A
    # suffix would be happy with a file the prefix fell off, and the prefix is
    # the only thing keeping these four at the end of the download list.
    for needed in (AUX_PREFIX + "SHA256SUMS.txt", ".spdx.json",
                   ".sbom.sigstore.json", ".provenance.sigstore.json"):
        if not any((n.startswith(AUX_PREFIX) and n.endswith(needed)) or n == needed
                   for n in names):
            missing.append(needed)
    if missing:
        raise SystemExit(
            "sign_release: the draft is not complete - missing %s.\nIt holds: %s"
            % (", ".join(missing), ", ".join(names)))
    if not release.get("isDraft"):
        raise SystemExit(
            "sign_release: %s is no longer a draft. Nothing here publishes, so "
            "somebody or something else did." % tag)
    print("  the draft holds %d asset(s) and is still a draft" % len(names))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("tag", help="the release tag, for example v0.2.0")
    parser.add_argument("--dry-run", action="store_true",
                        help="everything except signing, uploading and dispatching")
    parser.add_argument("--wait", type=int, default=300,
                        help="seconds to wait for the statement (default 300)")
    parser.add_argument("--macos-host", default=os.environ.get("TFG_MACOS_HOST"),
                        help="user@host of the Mac that signs the macOS archives, "
                             "or set TFG_MACOS_HOST")
    args = parser.parse_args(argv)

    if not os.path.isfile(PIN_FILE):
        raise SystemExit("sign_release: run this from the root of the repository")
    # Refused here rather than after the Windows half, because stopping halfway
    # would leave a build with three signed archives and two unsigned ones, and
    # a release page cannot say that.
    if not args.macos_host:
        raise SystemExit(
            "sign_release: no Mac to sign the macOS archives on.\n"
            "Pass --macos-host user@host, or set TFG_MACOS_HOST.\n"
            "A bare macOS binary cannot carry a notarisation ticket, so those two\n"
            "archives are rejected by Gatekeeper unless this step runs.")
    pin = pinned_digest()
    work = os.path.join("dist", "signing", args.tag)

    print("\n[1/8] fetching the build for %s" % args.tag)
    fetch_build(args.tag, work)

    print("\n[2/8] checking it before touching it")
    verify_before_touching(work)

    print("\n[3/8] the card")
    thumbprint = signing_thumbprint(pin)
    signtool = find_signtool()

    print("\n[4/8] signing the Windows programs")
    for name in windows_archives(work):
        sign_archive(os.path.join(work, name), thumbprint, pin, signtool, args.dry_run)

    print("\n[5/8] signing the macOS bundles on %s" % args.macos_host)
    sign_macos(args.tag, work, args.macos_host, args.dry_run)

    print("\n[6/8] checksums over what will be published")
    name_for_publication(work, args.tag)
    digest = write_checksums(work)
    print("  %sSHA256SUMS.txt: %s" % (AUX_PREFIX, digest))

    print("\n[7/8] uploading to the draft")
    upload_to_draft(args.tag, work, args.dry_run)
    ask_for_the_statement(args.tag, digest, args.dry_run)

    print("\n[8/8] waiting for the statement and checking the draft")
    confirm_draft(args.tag, args.wait, args.dry_run)

    print("\nDone. %s is a complete DRAFT." % args.tag)
    print("Read it, then publish it from the Releases page.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
