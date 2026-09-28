#!/usr/bin/env python3
"""Build the Windows installer of one release from its two signed archives.

    python .github/scripts/build_msi.py --tag v0.5.0 --archives <dir> --out-dir <dir>
    python .github/scripts/build_msi.py --tag v0.5.0 --check
    python .github/scripts/build_msi.py --tag v0.5.0 --source-only
    python .github/scripts/build_msi.py --tag v0.5.0 --product-name

One installer carries both programs, the window and the command line, for every
account on the machine - decided by the owner on 2026-09-28, beside the zips and
the feed packages, which stay as they are. It is built from the SIGNED amd64
archives, because the signature is on the programs inside them: an installer made
before the card signed them would carry unsigned copies. So a release builds it in
sign_release.py, on the machine with the card, and never in release.yml. The same
script builds an unsigned one on a Windows runner in ci.yml, from the latest
published release, so that what the installer DOES is asked on a clean machine.

Every value comes from the tree this script sits in: the template in packaging/msi/,
the names and addresses build_packages.py already reads, the icon. sign_release.py
runs it from the tree of the TAG, exported with git archive, so the installer is
built from what was tagged, whatever the checkout happens to stand on.

--check asks only whether this machine can build it (the tag, the template, WiX),
so sign_release.py can refuse before the card signs anything. --source-only prints
the rendered installer source and builds nothing, which is what the guards read.
--product-name prints the name the installer carries and nothing else, which is
what sign_release.py signs it with.

Exit codes:
    0  the installer was written to --out-dir, or --check found nothing missing
    1  refused, and the message says which input and why

Nothing here signs or publishes anything.
"""
import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import build_packages as packages  # noqa: E402 - the sibling script, found beside this one

ROOT = packages.ROOT
TEMPLATE = os.path.join(ROOT, "packaging", "msi", "tfg-setup.wxs.in")
ICON = os.path.join(ROOT, "internal", "gui", "icon", "chickpea.ico")

# The file a person downloads, public for good (owner's decision of 2026-09-28).
# Not tfg_*, because the release notes say tfg_* is the command line, and this
# carries both programs. It sorts between the window's archives and the command
# line's on the release page, never among the verify- files.
NAME = "tfg-setup_{version}_windows_amd64.msi"

# The product, not a build, and it never changes - see the comment on it in the
# template. Generated once on 2026-09-28 and pinned by a guard.
UPGRADE_CODE = "7DB637B7-3BEB-4FBD-BE84-60822854AA2F"

# The WiX this project builds with. Another version builds another package from
# the same source, so a different one is refused rather than used.
WIX_VERSION = "5.0.2"

# The window has no arm64 build, so neither has the installer.
ARCH = "amd64"


def refuse(message):
    raise SystemExit("build_msi: %s" % message)


def parse_version(tag):
    """The version a tag names, or a refusal saying why it gets no installer.

    A tag with a hyphen is a release candidate, the same rule release.yml uses
    to mark one as a pre-release. Windows Installer reads only the three
    numbers, so a candidate's installer would take the release's own version
    and the release could not replace it.
    """
    if "-" in tag:
        refuse("%s is a release candidate, and candidates get no installer. Windows "
               "Installer reads only the three numbers, so it would take %s for the "
               "release itself and the release would not replace it"
               % (tag, tag.split("-")[0]))
    found = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", tag)
    if not found:
        refuse("%r is not a release tag. Pass it the way the release is tagged, for "
               "example v0.5.0" % tag)
    major, minor, patch = (int(n) for n in found.groups())
    if major > 255 or minor > 255 or patch > 65535:
        refuse("%s does not fit an installer version, which holds at most 255 for the "
               "first two numbers and 65535 for the third" % tag)
    return "%s.%s.%s" % found.groups()


def values(version, tag):
    """Every placeholder the template may use."""
    repo_url = "https://github.com/%s" % packages.repository()
    window = next(p for p in packages.PACKAGES if p.kind == "window")
    return {
        "APP_NAME": packages.APP_NAME,
        "PUBLISHER": packages.PUBLISHER,
        "VERSION": version,
        "UPGRADE_CODE": UPGRADE_CODE,
        "PROJECT_URL": packages.site(),
        "ISSUES_URL": repo_url + "/issues",
        "RELEASE_NOTES_URL": "%s/releases/tag/%s" % (repo_url, tag),
        "SHORTCUT_DESCRIPTION": packages.SHORT["window"],
        "WINDOW_EXE": window.program + ".exe",
    }


def source(version, tag):
    """The installer source for one release, every placeholder filled."""
    if not os.path.isfile(TEMPLATE):
        refuse("there is no template at %s. Run this from a tree that has one - a tag "
               "from before the installer existed has none" % TEMPLATE)
    text = packages.read_text(TEMPLATE)
    table = values(version, tag)
    unused = set(table) - set(packages.PLACEHOLDER.findall(text))
    if unused:
        refuse("the template uses no %s any more - take it out of build_msi.py"
               % ", ".join(sorted(unused)))
    return packages.render(text, table, "tfg-setup.wxs")


def find_wix():
    """The wix command, at the version this project builds with, or a refusal."""
    found = shutil.which("wix") or shutil.which(
        "wix", path=os.path.join(os.path.expanduser("~"), ".dotnet", "tools"))
    install = ("    dotnet tool install --global wix --version %s\n"
               "It runs on .NET - install the .NET SDK first if there is no dotnet command."
               % WIX_VERSION)
    if not found:
        refuse("WiX is not installed, and the installer is built with it. Install the "
               "version this project builds with:\n" + install)
    said = subprocess.run([found, "--version"], capture_output=True, text=True)
    version = said.stdout.strip().split("+")[0]
    if said.returncode != 0 or version != WIX_VERSION:
        refuse("%s says it is version %r, and the installer is built with %s - another "
               "version builds another package from the same source. Replace it:\n"
               "    dotnet tool uninstall --global wix\n%s"
               % (found, version, WIX_VERSION, install))
    return found


def safe_name(name):
    """Whether a name inside an archive stays inside the folder it is unpacked into."""
    parts = name.split("/")
    return (bool(name) and not name.startswith("/") and "\\" not in name and ":" not in name
            and all(part not in ("", ".", "..") for part in parts))


def unpack(archives, version, into):
    """The window's and the command line's amd64 archives, in one folder.

    A file both archives hold goes in once, and only when both hold the same
    bytes - the three documents, today. Two different copies of one file are a
    question about how the release was built, and the installer does not pick one.

    One file means one name to Windows, where the installer puts it: LICENSE
    and License are the same file there, so the names are compared folded to
    one case. Compared as written, the second would have overwritten the first
    without a word (outside review of #147).
    """
    came_from = {}
    for package in packages.PACKAGES:
        name = package.archive.format(version=version, arch=ARCH)
        path = os.path.join(archives, name)
        if not os.path.isfile(path):
            refuse("%s holds no %s. Pass the folder that holds the signed archives of "
                   "this release" % (archives, name))
        try:
            with zipfile.ZipFile(path) as archive:
                for entry in archive.infolist():
                    if entry.is_dir():
                        continue
                    if not safe_name(entry.filename):
                        refuse("%s holds %r, a name that leads out of the folder it is "
                               "unpacked into. That is not an archive the release built"
                               % (name, entry.filename))
                    target = os.path.join(into, *entry.filename.split("/"))
                    key = entry.filename.casefold()
                    if key in came_from:
                        first, held_at, held_as = came_from[key]
                        with open(held_at, "rb") as held:
                            if held.read() != archive.read(entry):
                                refuse("%s holds %s and %s holds %s, one file on Windows, and "
                                       "not the same bytes. One installer carries one copy - "
                                       "look at how the release built the two archives"
                                       % (first, held_as, name, entry.filename))
                        continue
                    came_from[key] = (name, target, entry.filename)
                    os.makedirs(os.path.dirname(target), exist_ok=True)
                    with archive.open(entry) as src, open(target, "wb") as dst:
                        shutil.copyfileobj(src, dst)
        except zipfile.BadZipFile as err:
            refuse("%s is not a zip archive it can read (%s). Download it again" % (path, err))
    for package in packages.PACKAGES:
        program = package.program + ".exe"
        if program.casefold() not in came_from:
            refuse("the archives hold no %s at the top, and the installer puts it on PATH. "
                   "That is not the shape the release builds" % program)
    return came_from


def build(tag, archives, out_dir):
    """Write the installer into out_dir, all or nothing, and return its path."""
    version = parse_version(tag)
    text = source(version, tag)
    if not os.path.isdir(out_dir):
        refuse("--out-dir %s is not a folder. Pass one that exists" % out_dir)
    target = os.path.join(out_dir, NAME.format(version=version))
    if os.path.exists(target):
        refuse("%s is already there. Nothing is overwritten - remove it or pass another "
               "--out-dir" % target)
    if not os.path.isfile(ICON):
        refuse("the icon %s is not in the tree" % ICON)

    # Beside nothing of the caller's: sign_release.py hands its own folder of
    # files to publish as --out-dir, and a folder left inside it would be
    # published, or would break the checksums written over it.
    staging = tempfile.mkdtemp(prefix="tfg-msi-")
    try:
        payload = os.path.join(staging, "payload")
        os.makedirs(payload)
        unpack(archives, version, payload)
        wix = find_wix()
        wxs = os.path.join(staging, "tfg-setup.wxs")
        with open(wxs, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(text)
        built = os.path.join(staging, os.path.basename(target))
        command = [wix, "build", wxs, "-arch", "x64", "-pdbtype", "none",
                   "-d", "PayloadDir=" + payload, "-d", "IconFile=" + ICON, "-o", built]
        print("    $ %s" % " ".join(command))
        result = subprocess.run(command)
        if result.returncode != 0 or not os.path.isfile(built):
            refuse("wix build exited %d, and nothing was written to %s. What it said is above"
                   % (result.returncode, out_dir))
        # Moved in only once it is whole, so a run that fails or is stopped
        # leaves no half written installer where the caller would find it.
        shutil.move(built, target)
    except OSError as err:
        refuse("cannot build the installer: %s. Nothing was written to %s" % (err, out_dir))
    finally:
        shutil.rmtree(staging, ignore_errors=True)
    return target


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--tag", required=True, help="the release, for example v0.5.0")
    parser.add_argument("--archives", help="the folder holding its signed amd64 zip archives")
    parser.add_argument("--out-dir", help="the folder the installer is written into")
    parser.add_argument("--check", action="store_true",
                        help="only say whether this machine can build it")
    parser.add_argument("--source-only", action="store_true",
                        help="print the installer source and build nothing")
    parser.add_argument("--product-name", action="store_true",
                        help="print the name the installer carries and build nothing")
    args = parser.parse_args(argv)

    if args.source_only:
        sys.stdout.write(source(parse_version(args.tag), args.tag))
        return 0
    if args.product_name:
        print(values(parse_version(args.tag), args.tag)["APP_NAME"])
        return 0
    if args.check:
        version = parse_version(args.tag)
        source(version, args.tag)
        if not os.path.isfile(ICON):
            refuse("the icon %s is not in the tree" % ICON)
        print("ready to build %s with %s" % (NAME.format(version=version), find_wix()))
        return 0
    if not args.archives or not args.out_dir:
        parser.error("--archives and --out-dir are needed to build")
    print(build(args.tag, args.archives, args.out_dir))
    return 0


if __name__ == "__main__":
    sys.exit(main())
