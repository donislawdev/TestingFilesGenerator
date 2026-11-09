# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Lays out the wheels a release puts on PyPI.

testing-files-generator gets one wheel for each system the release builds tfg
for, with that system's binary inside, and one wheel with no binary that pip
takes on every other system. pytest-testing-files-generator gets one wheel
with no code, which installs testing-files-generator of the same version.
.github/workflows/pypi.yml runs this on the archives it has downloaded and
checked, and nothing here fetches or publishes anything.

Written with zipfile rather than a build backend: the binary has to keep its
execute bit and the macOS app has to stay whole, which a backend for Python
code does not do, and nothing else is needed. The systems come from
src/testing_files_generator/_binary.py, which a guard holds to the release's
targets. docs/PYTEST-2026-11-09.md.

    python assemble.py archives <version>                 the archive of each system: dir archive binary
    python assemble.py version <version>                  the version as PyPI writes it
    python assemble.py build <version> <binaries> <out>   the wheels, into <out>/main and <out>/plugin
"""

from __future__ import annotations

import base64
import hashlib
import re
import stat
import struct
import sys
import tomllib
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
PACKAGE = HERE.parent
PLUGIN = PACKAGE.parent / "pytest-testing-files-generator"
sys.path.insert(0, str(PACKAGE / "src"))
from testing_files_generator._binary import PLATFORMS, binary_in  # noqa: E402

# How the release names a system and an architecture in its archives.
RELEASE_SYSTEM = {"win32": "windows", "darwin": "macos", "linux": "linux"}
RELEASE_ARCH = {"x64": "amd64", "arm64": "arm64"}

# The platform tags of the wheels each system's binary goes into - pip takes a
# wheel by its tag alone. Linux gets one for glibc and one for musl (Alpine):
# the binary is static and runs on both (measured 2026-11-09, Alpine 3.22).
# macOS gets its tag from the binary, below.
WHEEL_TAGS = {
    "win32 x64": ["win_amd64"],
    "win32 arm64": ["win_arm64"],
    "linux x64": ["manylinux_2_17_x86_64.manylinux2014_x86_64", "musllinux_1_2_x86_64"],
    "linux arm64": ["manylinux_2_17_aarch64.manylinux2014_aarch64", "musllinux_1_2_aarch64"],
    "darwin arm64": None,
}

VERSION_SHAPE = re.compile(r"^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$")
PRERELEASE = {"alpha": "a", "a": "a", "beta": "b", "b": "b", "rc": "rc"}

# Every entry of every wheel carries this date, so the same release gives the
# same bytes. The zip format starts in 1980.
STAMP = (1980, 1, 1, 0, 0, 0)


def fail(message: str) -> None:
    raise SystemExit(f"assemble: {message}")


def checked_version(version: str) -> str:
    if not VERSION_SHAPE.match(version or ""):
        fail(f"{version!r} is not a release version. Give the version of a release tag without its v, "
             "for example 0.6.0 or 0.6.0-rc.1.")
    return version


def pep440(version: str) -> str:
    """The version as PyPI takes it. 0.6.0-rc.1 is 0.6.0rc1, and pip installs it only when asked with --pre."""
    core, _, pre = checked_version(version).partition("-")
    if not pre:
        return core
    found = re.fullmatch(r"([a-z]+)\.?(\d+)", pre)
    if not found or found.group(1) not in PRERELEASE:
        fail(f"the prerelease {pre} of {version} has no form PyPI takes. Tag it -alpha.N, -beta.N or -rc.N, "
             "or leave PyPI out of that release.")
    return f"{core}{PRERELEASE[found.group(1)]}{int(found.group(2))}"


def targets() -> list[dict]:
    out = []
    for key in PLATFORMS:
        system, arch = key.split(" ")
        if system not in RELEASE_SYSTEM or arch not in RELEASE_ARCH or key not in WHEEL_TAGS:
            fail(f"_binary.py names {key}, and assemble does not know what the release calls it or which "
                 "wheel it goes in. Add it to RELEASE_SYSTEM, RELEASE_ARCH and WHEEL_TAGS.")
        binary = binary_in(system)
        out.append({"key": key, "system": system, "arch": arch, "dir": f"{system}-{arch}", "binary": binary,
                    "entry": binary.split("/")[0]})
    return out


def archive_of(version: str, target: dict) -> str:
    ext = "zip" if target["system"] == "win32" else "tar.gz"
    return f"tfg_{checked_version(version)}_{RELEASE_SYSTEM[target['system']]}_{RELEASE_ARCH[target['arch']]}.{ext}"


def macos_minimum(binary: bytes) -> tuple[int, int]:
    """The oldest macOS a 64-bit Mach-O binary runs on, from its LC_BUILD_VERSION load command."""
    if binary[:4] != b"\xcf\xfa\xed\xfe":
        fail("the macOS binary is not a 64-bit Mach-O file, so the oldest macOS it runs on cannot be read")
    ncmds = struct.unpack_from("<I", binary, 16)[0]
    at = 32
    for _ in range(ncmds):
        cmd, size = struct.unpack_from("<II", binary, at)
        if cmd == 0x32:  # LC_BUILD_VERSION: platform, minos as xxxx.yy.zz
            minos = struct.unpack_from("<I", binary, at + 12)[0]
            return minos >> 16, (minos >> 8) & 0xFF
        at += size
    fail("the macOS binary says nothing of the oldest macOS it runs on (no LC_BUILD_VERSION)")
    raise AssertionError


def _project(directory: Path) -> dict:
    with open(directory / "pyproject.toml", "rb") as f:
        return tomllib.load(f)["project"]


def _metadata(project: dict, version: str, license_expression: str, license_files: list[str],
              requires: list[str], readme: str) -> bytes:
    lines = [
        "Metadata-Version: 2.4",
        f"Name: {project['name']}",
        f"Version: {version}",
        f"Summary: {project['description']}",
        f"Author: {', '.join(a['name'] for a in project.get('authors', []))}",
        f"License-Expression: {license_expression}",
        *(f"License-File: {name}" for name in license_files),
        f"Keywords: {','.join(project.get('keywords', []))}",
        *(f"Classifier: {c}" for c in project.get("classifiers", [])),
        *(f"Project-URL: {label}, {url}" for label, url in project.get("urls", {}).items()),
        f"Requires-Python: {project['requires-python']}",
        *(f"Requires-Dist: {r}" for r in requires),
        "Description-Content-Type: text/markdown",
    ]
    for line in lines:
        if "\n" in line or "\r" in line:
            fail(f"a metadata field holds a line break: {line!r}")
    return ("\n".join(lines) + "\n\n" + readme).encode("utf-8")


def _entry_points(project: dict) -> bytes:
    groups = {}
    if project.get("scripts"):
        groups["console_scripts"] = project["scripts"]
    groups.update(project.get("entry-points", {}))
    text = "".join(f"[{group}]\n" + "".join(f"{k} = {v}\n" for k, v in entries.items()) + "\n"
                   for group, entries in groups.items())
    return text.encode("utf-8")


def _record_hash(data: bytes) -> str:
    return "sha256=" + base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()


def write_wheel(out: Path, dist: str, version: str, tags: str, files: list[tuple[str, bytes, bool]],
                info: list[tuple[str, bytes]]) -> Path:
    """Writes one wheel. files are the installed files, info the files of its .dist-info besides WHEEL and RECORD."""
    stem = f"{dist.replace('-', '_')}-{version}"
    dist_info = f"{stem}.dist-info"
    pure = tags == "any"
    platforms = tags.split(".")
    wheel_text = ("Wheel-Version: 1.0\nGenerator: testing-files-generator release/assemble.py\n"
                  f"Root-Is-Purelib: {'true' if pure else 'false'}\n"
                  + "".join(f"Tag: py3-none-{p}\n" for p in platforms))
    entries = sorted(files) + [(f"{dist_info}/{name}", data, False) for name, data in info]
    entries.append((f"{dist_info}/WHEEL", wheel_text.encode(), False))
    names = [name for name, _, _ in entries]
    if len(set(names)) != len(names):
        fail(f"the wheel of {dist} for {tags} would hold one name twice")
    record = "".join(f"{name},{_record_hash(data)},{len(data)}\n" for name, data, _ in entries)
    record += f"{dist_info}/RECORD,,\n"
    entries.append((f"{dist_info}/RECORD", record.encode(), False))
    path = out / f"{stem}-py3-none-{tags}.whl"
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        for name, data, executable in entries:
            entry = zipfile.ZipInfo(name, date_time=STAMP)
            entry.compress_type = zipfile.ZIP_DEFLATED
            # Made on Unix whatever system writes it: an entry marked as made
            # on MS-DOS is installed by uv without its execute bit, where pip
            # keeps it (measured 2026-11-09, uv 0.9.30).
            entry.create_system = 3
            entry.external_attr = ((0o755 if executable else 0o644) | stat.S_IFREG) << 16
            z.writestr(entry, data)
    return path


def _code() -> list[tuple[str, bytes, bool]]:
    src = PACKAGE / "src" / "testing_files_generator"
    files = []
    for path in sorted(src.iterdir()):
        if path.is_file() and (path.suffix == ".py" or path.name == "py.typed"):
            files.append((f"testing_files_generator/{path.name}", path.read_bytes(), False))
    if not any(name.endswith("/__init__.py") for name, _, _ in files):
        fail(f"{src} holds no __init__.py")
    return files


def _binary_files(binaries: Path, target: dict) -> list[tuple[str, bytes, bool]]:
    root = binaries / target["dir"]
    entry = root / target["entry"]
    if not entry.exists():
        fail(f"{root} holds no {target['entry']}. Extract the archive of {target['key']} there.")
    files = []
    paths = [entry] if entry.is_file() else sorted(entry.rglob("*"))
    for path in paths:
        if path.is_symlink():
            fail(f"{path} is a link, and a wheel cannot hold one. The release archive changed its layout.")
        if path.is_file():
            rel = path.relative_to(root).as_posix()
            executable = rel == target["binary"] or bool(path.stat().st_mode & 0o111)
            files.append((f"testing_files_generator/bin/{rel}", path.read_bytes(), executable))
    if not any(rel == f"testing_files_generator/bin/{target['binary']}" for rel, _, _ in files):
        fail(f"{root} holds no {target['binary']}")
    return files


def _release_licences(binaries: Path, target: dict) -> list[tuple[str, bytes]]:
    out = []
    for name in ("LICENSE", "THIRD-PARTY-NOTICES.md"):
        path = binaries / target["dir"] / name
        if not path.is_file():
            fail(f"the archive of {target['key']} has no {name}, which has to go with the binary")
        out.append((f"licenses/tfg/{name}", path.read_bytes()))
    return out


def build(version: str, binaries: Path, out: Path) -> list[Path]:
    """Writes every wheel of a release and gives their paths: testing-files-generator first, the plugin last."""
    wheel_version = pep440(version)
    main_out, plugin_out = out / "main", out / "plugin"
    for d in (main_out, plugin_out):
        d.mkdir(parents=True, exist_ok=False)
    project = _project(PACKAGE)
    readme = (PACKAGE / "README.md").read_text(encoding="utf-8")
    wrapper_licence = ("licenses/LICENSE", (PACKAGE / "LICENSE").read_bytes())
    entry_points = ("entry_points.txt", _entry_points(project))
    code = _code()
    written = []
    for target in targets():
        files = code + _binary_files(binaries, target)
        licences = [wrapper_licence, *_release_licences(binaries, target)]
        tag_sets = WHEEL_TAGS[target["key"]]
        if tag_sets is None:
            major, minor = macos_minimum(dict((n, d) for n, d, _ in files)[f"testing_files_generator/bin/{target['binary']}"])
            tag_sets = [f"macosx_{major}_{minor}_arm64"]
        # The package code is MPL-2.0 and the binary beside it GPL-3.0-only,
        # each with its own licence file - the same split as the npm packages.
        meta = _metadata(project, wheel_version, "MPL-2.0 AND GPL-3.0-only",
                         ["LICENSE", "tfg/LICENSE", "tfg/THIRD-PARTY-NOTICES.md"], [], readme)
        for tags in tag_sets:
            written.append(write_wheel(main_out, project["name"], wheel_version, tags, files,
                                       [("METADATA", meta), entry_points, *licences]))
    meta = _metadata(project, wheel_version, "MPL-2.0", ["LICENSE"], [], readme)
    written.append(write_wheel(main_out, project["name"], wheel_version, "any", code,
                               [("METADATA", meta), entry_points, wrapper_licence]))
    plugin = _project(PLUGIN)
    requires = [f"{r}=={wheel_version}" if r == project["name"] else r for r in plugin.get("dependencies", [])]
    if f"{project['name']}=={wheel_version}" not in requires:
        fail(f"{PLUGIN / 'pyproject.toml'} does not depend on {project['name']}")
    meta = _metadata(plugin, wheel_version, plugin["license"], ["LICENSE"], requires,
                     (PLUGIN / "README.md").read_text(encoding="utf-8"))
    written.append(write_wheel(plugin_out, plugin["name"], wheel_version, "any", [],
                               [("METADATA", meta), ("licenses/LICENSE", (PLUGIN / "LICENSE").read_bytes())]))
    return written


def main(argv: list[str]) -> None:
    if argv[:1] == ["archives"] and len(argv) == 2:
        for target in targets():
            print(target["dir"], archive_of(argv[1], target), target["binary"])
    elif argv[:1] == ["version"] and len(argv) == 2:
        print(pep440(argv[1]))
    elif argv[:1] == ["build"] and len(argv) == 4:
        for path in build(argv[1], Path(argv[2]).resolve(), Path(argv[3]).resolve()):
            print(path)
    else:
        fail("usage: assemble.py archives <version> | version <version> | build <version> <binaries> <out>")


if __name__ == "__main__":
    main(sys.argv[1:])
