#!/usr/bin/env python3
"""Render the WinGet and Chocolatey packages for one published release.

Two programs ship, the window and the command line, each in an archive of its
own, and each becomes a package of its own in both feeds - decided by the owner
on 2026-09-25, so that a build agent can take the command line without the
window and one package waiting in moderation does not hold the other.

The templates in packaging/ hold the shape. This fills them from the one place
each value lives: the version from the tag, the checksums from the release's
own checksum file, the addresses from go.mod and web/public/CNAME, the date
from CHANGELOG.md. Nothing is typed twice, and the two values that are - the
product's name and its licence - are held to their Go originals by a guard.

Usage:
    python .github/scripts/build_packages.py --tag v0.4.0 \\
        --sums verify-SHA256SUMS.txt --out <a directory outside the repository>

Exit codes:
    0  every package was rendered into --out
    1  refused, and the message says which input and why

Nothing here submits anything. Submitting is a person's step: a package is
published under the project's name to a feed somebody else moderates.
"""
import argparse
import os
import re
import shutil
import sys
import tempfile
from collections import namedtuple

# realpath, not abspath: the check that keeps --out outside the repository
# compares resolved paths, so a symbolic link or a junction pointing into the
# tree cannot walk the packages into it (outside review of #143).
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.realpath(__file__))))
TEMPLATES = os.path.join(ROOT, "packaging")
TEMPLATE_SUFFIX = ".in"
PLACEHOLDER = re.compile(r"\{\{([A-Z0-9_]+)\}\}")

# The manifest schema WinGet's submission pipeline accepts TODAY. The number
# documented in the winget-pkgs tree is not the same question: the sibling
# project was refused on 1.28.0, which validated and installed locally. Read
# from freshly merged manifests on 2026-09-25 - 7 of 7 carried this one.
WINGET_SCHEMA = "1.12.0"

# Held by a guard to internal/gui/run_cgo.go and internal/legal/spdx.go.
APP_NAME = "Testing Files Generator"
LICENCE = "GPL-3.0-only"

# The name both feeds show as the author, and the first part of the WinGet
# identifiers - the same publisher folder the sibling project already has in
# winget-pkgs.
PUBLISHER = "DonislawDev"
ICON = "internal/gui/icon/chickpea.png"

# WinGet's name for an architecture, keyed by the one the archive names use.
WINGET_ARCH = {"amd64": "x64", "arm64": "arm64"}

Package = namedtuple("Package", "kind winget_id choco_id title program archive arches moniker")

PACKAGES = (
    Package("window", "DonislawDev.TestingFilesGenerator", "testing-files-generator",
            APP_NAME, "tfg-gui", "tfg-gui_{version}_windows_{arch}.zip", ("amd64",), "tfg-gui"),
    Package("cli", "DonislawDev.TestingFilesGenerator.CLI", "testing-files-generator-cli",
            APP_NAME + " CLI", "tfg", "tfg_{version}_windows_{arch}.zip", ("amd64", "arm64"), "tfg"),
)

DESCRIPTION = (
    "Testing Files Generator makes real files to test against - an upload form, a parser, "
    "anything that takes a file. Pick a format and a size and you get a file of exactly "
    "that size, to the byte, that a real reader opens. Every run also writes a manifest "
    "saying what your system should do with each file. It runs entirely on your machine."
)

SHORT = {
    "window": "Real test files at any exact size, with a manifest saying how your system "
              "should react. The desktop window.",
    "cli": "Real test files at any exact size, with a manifest saying how your system "
           "should react. The command line, for scripts and pipelines.",
}

TAGS = ("qa", "testing", "test-data", "test-files", "file-generator", "fixtures")
KIND_TAG = {"window": "gui", "cli": "cli"}


def how_to_start(package, feed):
    """The paragraph that differs by feed: what the package gives and how to start it."""
    other = next(p for p in PACKAGES if p is not package)
    other_id = other.winget_id if feed == "winget" else other.choco_id
    if package.kind == "cli":
        return ("This package is the command line, for scripts and pipelines. The desktop "
                "window is the package %s. Type tfg help to see the commands." % other_id)
    if feed == "winget":
        return ("This package is the desktop window. The command line is the package %s. "
                "WinGet adds no Start menu shortcut for it. Open a new terminal and type "
                "tfg-gui. The window offers a tfg-out folder in the directory it was started "
                "from." % other_id)
    return ("This package is the desktop window. The command line is the package %s. It "
            "adds a Start menu shortcut and the tfg-gui command. Started from the shortcut, "
            "the window offers a tfg-out folder in your user profile." % other_id)


def refuse(message):
    raise SystemExit("build_packages: %s" % message)


def read_text(path, what="the file"):
    """A text file, or a refusal naming it - never a traceback.

    A file that is missing, unreadable or not UTF-8 is an input a person got
    wrong, and the answer has to say which file and what to do, not print a
    Python exception (outside review of #143).
    """
    try:
        with open(path, encoding="utf-8-sig") as handle:
            return handle.read().replace("\r\n", "\n")
    except UnicodeDecodeError:
        refuse("%s is not UTF-8 text, so it is not %s" % (path, what))
    except OSError as err:
        refuse("cannot read %s %s: %s" % (what, path, err.strerror or err))


# A release's checksum file is under a kilobyte - 970 bytes for v0.4.0. Anything
# near this is another file passed by mistake, an archive for instance, and is
# refused by size before a byte of it is read.
SUMS_LIMIT = 1024 * 1024


def repository():
    """owner/name, from the module path - the one place the address is written."""
    found = re.search(r"^module github\.com/([^/\s]+/[^/\s]+)\s*$",
                      read_text(os.path.join(ROOT, "go.mod")), re.M)
    if not found:
        refuse("go.mod does not name a github.com module, so there is no address to point at")
    return found.group(1)


def site():
    return "https://%s/" % read_text(os.path.join(ROOT, "web", "public", "CNAME")).strip()


def parse_tag(tag):
    """The version a tag names, or a refusal saying why it names none."""
    if re.fullmatch(r"v\d+\.\d+\.\d+-.+", tag):
        refuse("%s is a release candidate. The feeds get published releases only, "
               "because a person installing from them has no way to see the difference" % tag)
    found = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)", tag)
    if not found:
        refuse("%r is not a release tag. Pass it the way the release is tagged, for "
               "example v0.4.0" % tag)
    return "%s.%s.%s" % found.groups()


def release_date(version):
    """The date the changelog gives the version - one reader, not a second answer."""
    found = re.search(r"^## \[%s\] - (\d{4}-\d{2}-\d{2})\s*$" % re.escape(version),
                      read_text(os.path.join(ROOT, "CHANGELOG.md")), re.M)
    if not found:
        refuse("CHANGELOG.md has no dated section for %s. Package a version that has "
               "been released" % version)
    return found.group(1)


def read_sums(path):
    """{file name: lowercase sha256} from a checksum file, in any of its spellings.

    sha256sum writes '<hash>  <name>' in text mode and '<hash> *<name>' in binary
    mode, and that star is not part of the name. A file saved on Windows may
    carry a byte order mark and CRLF. None of that may reach an address.
    """
    hint = "Download verify-SHA256SUMS.txt from the release you are packaging"
    try:
        size = os.path.getsize(path)
    except OSError as err:
        refuse("cannot read the checksum file %s: %s. %s" % (path, err.strerror or err, hint))
    if size > SUMS_LIMIT:
        refuse("%s is %d bytes, so it is not a release's checksum file. %s" % (path, size, hint))
    sums = {}
    for number, line in enumerate(read_text(path, "a checksum file").split("\n"), 1):
        if not line.strip():
            continue
        found = re.fullmatch(r"([0-9A-Fa-f]{64}) [ *](\S.*)", line.strip())
        if not found:
            refuse("%s line %d is not '<sha256>  <file>': %r" % (path, number, line))
        sums[found.group(2).strip()] = found.group(1).lower()
    return sums


def archives(package, version):
    return {arch: package.archive.format(version=version, arch=arch) for arch in package.arches}


def checked_archives(sums, version, path):
    """Every archive a package needs, with its checksum, or a refusal naming what is missing."""
    found = {}
    for package in PACKAGES:
        for arch, name in archives(package, version).items():
            if name in sums:
                found[name] = sums[name]
                continue
            other = sorted(n for n in sums if re.fullmatch(
                re.escape(package.archive).replace(r"\{version\}", r"[^_]+")
                .replace(r"\{arch\}", re.escape(arch)), n))
            hint = (" It lists %s - that is the checksum file of another release." % other[0]
                    if other else "")
            refuse("%s has no line for %s.%s Download the checksum file of the release "
                   "you are packaging." % (path, name, hint))
    return found


def values(package, version, tag, sums):
    """Every placeholder a template may use, for one package."""
    repo = repository()
    repo_url = "https://github.com/%s" % repo
    names = archives(package, version)
    installers = []
    for arch, name in names.items():
        installers += ["- Architecture: %s" % WINGET_ARCH[arch],
                       "  InstallerUrl: %s/releases/download/%s/%s" % (repo_url, tag, name),
                       "  InstallerSha256: %s" % sums[name].upper()]
    return {
        "VERSION": version,
        "WINGET_SCHEMA": WINGET_SCHEMA,
        "WINGET_ID": package.winget_id,
        "CHOCO_ID": package.choco_id,
        "TITLE": package.title,
        "PUBLISHER": PUBLISHER,
        "PROGRAM": package.program,
        "EXE": package.program + ".exe",
        "MONIKER": package.moniker,
        "LICENSE": LICENCE,
        "REPO_URL": repo_url,
        "PROJECT_URL": site(),
        "DOCS_URL": site() + "docs/",
        "LICENSE_URL": "%s/blob/%s/LICENSE" % (repo_url, tag),
        "RELEASE_NOTES_URL": "%s/releases/tag/%s" % (repo_url, tag),
        "PACKAGE_SOURCE_URL": "%s/tree/main/packaging/chocolatey" % repo_url,
        # A CDN pinned to the tag, both halves load-bearing: Chocolatey's moderation
        # refuses raw.githubusercontent.com and github.com/.../raw alike, and an icon
        # on a branch keeps changing under a package that is already approved.
        "ICON_URL": "https://cdn.jsdelivr.net/gh/%s@%s/%s" % (repo, tag, ICON),
        "RELEASE_DATE": release_date(version),
        "URL_AMD64": "%s/releases/download/%s/%s" % (repo_url, tag, names["amd64"]),
        "SHA256_AMD64": sums[names["amd64"]],
        "SHORT_DESCRIPTION": SHORT[package.kind],
        "DESCRIPTION": DESCRIPTION,
        "WINGET_HOW_TO_START": how_to_start(package, "winget"),
        "CHOCO_HOW_TO_START": how_to_start(package, "chocolatey"),
        "WINGET_TAGS": "\n".join("- " + t for t in TAGS + (KIND_TAG[package.kind],)),
        "CHOCO_TAGS": " ".join(TAGS + (KIND_TAG[package.kind],)),
        "WINGET_INSTALLERS": "\n".join(installers),
    }


def render(text, table, name):
    """Fill every placeholder, or refuse. An unknown one is an error, never an empty string.

    A placeholder alone on its line may hold several lines, and each takes the
    indentation the placeholder had - that is how a YAML block stays a block. A
    placeholder inside a line takes one line only.
    """
    out = []
    for line in text.split("\n"):
        for key in PLACEHOLDER.findall(line):
            if key not in table:
                refuse("%s uses {{%s}}, which the renderer does not know" % (name, key))
        alone = re.fullmatch(r"( *)\{\{([A-Z0-9_]+)\}\}", line)
        if alone:
            out += [alone.group(1) + part if part else "" for part in table[alone.group(2)].split("\n")]
            continue
        for key in PLACEHOLDER.findall(line):
            if "\n" in table[key]:
                refuse("%s puts the several lines of {{%s}} inside a line" % (name, key))
            breaks = breaking(name, table[key])
            if breaks:
                refuse("{{%s}} holds %r, which breaks %s: %s" % (key, table[key], name, breaks))
        out.append(PLACEHOLDER.sub(lambda m: table[m.group(1)], line))
    return "\n".join(out)


# What a value substituted inside a line must not hold, by the file it lands in.
# Each would still render, and each would hand the feed a file that means
# something else: a quote ends a PowerShell string early, a bracket or an
# ampersand is markup in the nuspec, and a colon followed by a space or a space
# followed by a hash turns the rest of a plain YAML value into a key or a
# comment.
BREAKS = (
    (".ps1", "'", "a single quote ends the PowerShell string it sits in"),
    (".nuspec", "<", "the nuspec reads it as markup"),
    (".nuspec", ">", "the nuspec reads it as markup"),
    (".nuspec", "&", "the nuspec reads it as markup"),
    (".yaml", ": ", "YAML reads the rest as a key"),
    (".yaml", " #", "YAML reads the rest as a comment"),
)


def breaking(name, value):
    for suffix, text, why in BREAKS:
        if name.endswith(suffix) and text in value:
            return why
    return ""


def check_script(text, name):
    """A package script is read by Windows PowerShell 5.1, which takes a file without a
    byte order mark as the machine's ANSI code page - so a script that is not ASCII
    reaches it changed."""
    for number, line in enumerate(text.split("\n"), 1):
        if any(ord(c) > 127 for c in line):
            refuse("%s line %d is not ASCII: %r" % (name, number, line))


def templates(kind):
    """(template path, output path relative to the package) for one kind of package.

    A template named name.<kind>.ext.in belongs to that kind only, and renders to
    name.ext. One named name.ext.in belongs to every package.
    """
    kinds = {p.kind for p in PACKAGES}
    for feed in ("winget", "chocolatey"):
        for base, _, files in os.walk(os.path.join(TEMPLATES, feed)):
            for file in sorted(files):
                if not file.endswith(TEMPLATE_SUFFIX):
                    continue
                parts = file[:-len(TEMPLATE_SUFFIX)].split(".")
                owner = parts[1] if len(parts) > 2 and parts[1] in kinds else None
                if owner not in (None, kind):
                    continue
                target = ".".join(p for i, p in enumerate(parts) if not (i == 1 and owner))
                relative = os.path.relpath(os.path.join(base, target), TEMPLATES)
                yield os.path.join(base, file), relative.replace(os.sep, "/")


def destination(package, relative, version):
    """Where a rendered file goes: WinGet's in the folder layout winget-pkgs uses, so
    the three files can be copied across as they are."""
    feed, _, rest = relative.partition("/")
    if feed == "winget":
        kind = rest[:-len(".yaml")]
        name = package.winget_id + ("" if kind == "version" else "." + kind) + ".yaml"
        parts = package.winget_id.split(".")
        return "/".join(["winget", "manifests", parts[0][0].lower()] + parts + [version, name])
    if rest == "package.nuspec":
        rest = package.choco_id + ".nuspec"
    return "/".join(["chocolatey", package.choco_id, rest])


def check_out(out):
    """--out is outside the repository and empty or absent, or a refusal."""
    out = os.path.realpath(out)
    root = os.path.normcase(ROOT)
    if os.path.normcase(out) == root or os.path.normcase(out).startswith(root + os.sep):
        refuse("--out %s is inside the repository. Rendered packages are not source - put "
               "them somewhere outside it" % out)
    if os.path.exists(out) and (not os.path.isdir(out) or os.listdir(out)):
        refuse("--out %s already holds something. Nothing is overwritten - pass an empty "
               "or new directory" % out)
    return out


def build(tag, sums_path, out):
    """Render every package into out, all or nothing."""
    version = parse_tag(tag)
    out = check_out(out)
    sums = checked_archives(read_sums(sums_path), version, sums_path)
    if not os.path.isfile(os.path.join(ROOT, ICON)):
        refuse("the icon %s is not in the repository" % ICON)

    parent = os.path.dirname(out)
    try:
        os.makedirs(parent, exist_ok=True)
        work = tempfile.mkdtemp(prefix=".packages-", dir=parent)
    except OSError as err:
        refuse("cannot create a working folder in %s: %s" % (parent, err.strerror or err))
    try:
        used = set()
        known = set()
        for package in PACKAGES:
            table = values(package, version, tag, sums)
            known |= set(table)
            for source, relative in templates(package.kind):
                text = read_text(source)
                used |= set(PLACEHOLDER.findall(text))
                rendered = render(text, table, relative)
                if relative.endswith(".ps1"):
                    check_script(rendered, relative)
                target = os.path.join(work, *destination(package, relative, version).split("/"))
                os.makedirs(os.path.dirname(target), exist_ok=True)
                with open(target, "w", encoding="utf-8", newline="\n") as handle:
                    handle.write(rendered)
        if known - used:
            refuse("no template uses %s any more - take it out of the renderer"
                   % ", ".join(sorted(known - used)))
        if os.path.isdir(out):
            os.rmdir(out)
        os.rename(work, out)
    except OSError as err:
        refuse("cannot write the packages to %s: %s. Nothing was left behind" % (out, err.strerror or err))
    finally:
        if os.path.isdir(work):
            shutil.rmtree(work)
    return out


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("--tag", required=True, help="the published release, for example v0.4.0")
    parser.add_argument("--sums", required=True, help="that release's verify-SHA256SUMS.txt")
    parser.add_argument("--out", required=True, help="an empty or new directory outside the repository")
    args = parser.parse_args(argv)
    out = build(args.tag, args.sums, args.out)
    for base, _, files in sorted(os.walk(out)):
        for file in sorted(files):
            print(os.path.join(base, file))
    return 0


if __name__ == "__main__":
    sys.exit(main())
