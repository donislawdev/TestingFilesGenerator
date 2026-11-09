"""The wheels a release puts on PyPI, laid out from archives made here."""

import base64
import hashlib
import importlib.util
import os
import stat
import struct
import sys
import zipfile
from email.parser import Parser
from pathlib import Path

import pytest
from conftest import PACKAGE

_spec = importlib.util.spec_from_file_location("assemble", PACKAGE / "release" / "assemble.py")
assemble = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(assemble)


def macho(minos=0x000D0000):
    """A 64-bit Mach-O header with one other load command and LC_BUILD_VERSION, as Go writes them."""
    other = struct.pack("<II", 0x19, 72) + b"\0" * 64
    build = struct.pack("<IIIIII", 0x32, 24, 1, minos, 0x001A0200, 0)
    header = struct.pack("<IiiIIIII", 0xFEEDFACF, 0x0100000C, 0, 2, 2, len(other) + len(build), 0, 0)
    return header + other + build + b"\0" * 32


def test_the_archives_are_the_ones_the_release_attaches():
    lines = [f"{t['dir']} {assemble.archive_of('0.5.0', t)} {t['binary']}" for t in assemble.targets()]
    assert lines == [
        "win32-x64 tfg_0.5.0_windows_amd64.zip tfg.exe",
        "win32-arm64 tfg_0.5.0_windows_arm64.zip tfg.exe",
        "linux-x64 tfg_0.5.0_linux_amd64.tar.gz tfg",
        "linux-arm64 tfg_0.5.0_linux_arm64.tar.gz tfg",
        "darwin-arm64 tfg_0.5.0_macos_arm64.tar.gz tfg.app/Contents/MacOS/tfg",
    ]


@pytest.mark.parametrize(("tag", "pypi"), [
    ("0.6.0", "0.6.0"), ("1.12.3", "1.12.3"), ("0.6.0-rc.1", "0.6.0rc1"), ("0.6.0-rc1", "0.6.0rc1"),
    ("0.6.0-alpha.2", "0.6.0a2"), ("0.6.0-beta.3", "0.6.0b3"), ("0.6.0-b.03", "0.6.0b3"),
])
def test_a_release_version_is_written_as_pypi_writes_it(tag, pypi):
    assert assemble.pep440(tag) == pypi


@pytest.mark.parametrize("bad", ["v0.6.0", "0.6", "0.6.0+build", "0.6.0-dev.1", "0.6.0-rc", "", "0.6.0-rc.1.2"])
def test_a_version_pypi_has_no_form_for_is_refused_with_the_way_out(bad):
    with pytest.raises(SystemExit) as caught:
        assemble.pep440(bad)
    assert "release version" in str(caught.value) or "has no form PyPI takes" in str(caught.value)


def test_the_oldest_macos_is_read_from_the_binary():
    assert assemble.macos_minimum(macho(0x000D0000)) == (13, 0)
    assert assemble.macos_minimum(macho(0x000E0200)) == (14, 2)
    with pytest.raises(SystemExit, match="not a 64-bit Mach-O"):
        assemble.macos_minimum(b"MZ" + b"\0" * 64)


def release_archives(root: Path) -> Path:
    """Extracted archives as pypi.yml lays them out, with a binary of each system made up here."""
    binaries = root / "binaries"
    for t in assemble.targets():
        d = binaries / t["dir"]
        if t["system"] == "darwin":
            app = d / "tfg.app" / "Contents"
            (app / "MacOS").mkdir(parents=True)
            (app / "MacOS" / "tfg").write_bytes(macho())
            (app / "Info.plist").write_text("<plist/>", encoding="utf-8")
            (app / "_CodeSignature").mkdir()
            (app / "_CodeSignature" / "CodeResources").write_text("<plist/>", encoding="utf-8")
        else:
            d.mkdir(parents=True)
            (d / t["binary"]).write_bytes(f"tfg for {t['key']}".encode())
        (d / "LICENSE").write_text("GNU GENERAL PUBLIC LICENSE", encoding="utf-8")
        (d / "THIRD-PARTY-NOTICES.md").write_text("notices", encoding="utf-8")
        (d / "README.md").write_text("not in the wheel", encoding="utf-8")
    return binaries


def built(tmp_path, version="0.6.0-rc.1", out="out"):
    binaries = tmp_path / "binaries" if (tmp_path / "binaries").exists() else release_archives(tmp_path)
    return assemble.build(version, binaries, tmp_path / out)


def test_every_system_gets_its_wheels_and_every_other_one_the_wheel_with_no_binary(tmp_path):
    names = [p.name for p in built(tmp_path)]
    assert names == [
        "testing_files_generator-0.6.0rc1-py3-none-win_amd64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-win_arm64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-manylinux_2_17_x86_64.manylinux2014_x86_64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-musllinux_1_2_x86_64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-manylinux_2_17_aarch64.manylinux2014_aarch64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-musllinux_1_2_aarch64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-macosx_13_0_arm64.whl",
        "testing_files_generator-0.6.0rc1-py3-none-any.whl",
        "pytest_testing_files_generator-0.6.0rc1-py3-none-any.whl",
    ]


def _read(wheel: Path):
    with zipfile.ZipFile(wheel) as z:
        return {i.filename: (i, z.read(i)) for i in z.infolist()}


def test_a_wheel_holds_the_code_the_binary_with_its_execute_bit_and_both_licences(tmp_path):
    wheels = {p.name: p for p in built(tmp_path)}
    linux = _read(wheels["testing_files_generator-0.6.0rc1-py3-none-musllinux_1_2_x86_64.whl"])
    info = "testing_files_generator-0.6.0rc1.dist-info"
    for name, (entry, _) in linux.items():
        assert entry.create_system == 3, f"{name} is not marked as made on Unix, and uv drops its execute bit"
        assert entry.date_time == assemble.STAMP
        mode = entry.external_attr >> 16
        want = 0o755 if name == "testing_files_generator/bin/tfg" else 0o644
        assert stat.S_IMODE(mode) == want, name
    assert linux["testing_files_generator/bin/tfg"][1] == b"tfg for linux x64"
    assert "testing_files_generator/__init__.py" in linux
    assert "testing_files_generator/py.typed" in linux
    assert "testing_files_generator/bin/README.md" not in linux
    assert linux[f"{info}/licenses/tfg/LICENSE"][1] == b"GNU GENERAL PUBLIC LICENSE"
    assert linux[f"{info}/licenses/LICENSE"][1].startswith(b"Mozilla Public License Version 2.0")
    meta = Parser().parsestr(linux[f"{info}/METADATA"][1].decode())
    assert meta["Version"] == "0.6.0rc1"
    assert meta["License-Expression"] == "MPL-2.0 AND GPL-3.0-only"
    assert meta.get_all("License-File") == ["LICENSE", "tfg/LICENSE", "tfg/THIRD-PARTY-NOTICES.md"]
    assert meta["Requires-Python"] == ">=3.10"
    assert meta.get_payload().startswith("# testing-files-generator")
    wheel = linux[f"{info}/WHEEL"][1].decode()
    assert "Root-Is-Purelib: false" in wheel and "Tag: py3-none-musllinux_1_2_x86_64" in wheel
    points = linux[f"{info}/entry_points.txt"][1].decode()
    assert "tfg = testing_files_generator.__main__:main" in points
    assert "[pytest11]\ntesting_files_generator = testing_files_generator.pytest_plugin" in points


def test_a_tag_set_is_written_out_one_tag_a_line(tmp_path):
    wheels = {p.name: p for p in built(tmp_path)}
    many = _read(wheels["testing_files_generator-0.6.0rc1-py3-none-manylinux_2_17_x86_64.manylinux2014_x86_64.whl"])
    wheel = many["testing_files_generator-0.6.0rc1.dist-info/WHEEL"][1].decode()
    assert "Tag: py3-none-manylinux_2_17_x86_64\nTag: py3-none-manylinux2014_x86_64\n" in wheel


def test_the_macos_wheel_holds_the_whole_app_and_the_windows_one_the_exe(tmp_path):
    wheels = {p.name: p for p in built(tmp_path)}
    mac = _read(wheels["testing_files_generator-0.6.0rc1-py3-none-macosx_13_0_arm64.whl"])
    for rel in ("Contents/MacOS/tfg", "Contents/Info.plist", "Contents/_CodeSignature/CodeResources"):
        assert f"testing_files_generator/bin/tfg.app/{rel}" in mac, rel
    assert stat.S_IMODE(mac["testing_files_generator/bin/tfg.app/Contents/MacOS/tfg"][0].external_attr >> 16) == 0o755
    win = _read(wheels["testing_files_generator-0.6.0rc1-py3-none-win_arm64.whl"])
    assert win["testing_files_generator/bin/tfg.exe"][1] == b"tfg for win32 arm64"


def test_the_wheel_with_no_binary_is_the_code_alone_under_the_licence_of_the_code(tmp_path):
    wheels = {p.name: p for p in built(tmp_path)}
    pure = _read(wheels["testing_files_generator-0.6.0rc1-py3-none-any.whl"])
    assert not [n for n in pure if "/bin/" in n]
    meta = Parser().parsestr(pure["testing_files_generator-0.6.0rc1.dist-info/METADATA"][1].decode())
    assert meta["License-Expression"] == "MPL-2.0"
    assert "Root-Is-Purelib: true" in pure["testing_files_generator-0.6.0rc1.dist-info/WHEEL"][1].decode()


def test_the_plugin_package_installs_testing_files_generator_of_exactly_its_version(tmp_path):
    wheels = {p.name: p for p in built(tmp_path)}
    plugin = _read(wheels["pytest_testing_files_generator-0.6.0rc1-py3-none-any.whl"])
    assert all(n.startswith("pytest_testing_files_generator-0.6.0rc1.dist-info/") for n in plugin)
    meta = Parser().parsestr(plugin["pytest_testing_files_generator-0.6.0rc1.dist-info/METADATA"][1].decode())
    assert meta["Name"] == "pytest-testing-files-generator"
    assert meta.get_all("Requires-Dist") == ["testing-files-generator==0.6.0rc1", "pytest>=8"]
    assert "Framework :: Pytest" in meta.get_all("Classifier")


def test_every_file_of_every_wheel_is_in_its_record_with_its_hash(tmp_path):
    for wheel in built(tmp_path):
        files = _read(wheel)
        record = next(data for name, (_, data) in files.items() if name.endswith(".dist-info/RECORD"))
        rows = [line.split(",") for line in record.decode().splitlines()]
        assert sorted(r[0] for r in rows) == sorted(files), wheel.name
        for name, digest, size in rows:
            if name.endswith("/RECORD"):
                assert (digest, size) == ("", "")
                continue
            data = files[name][1]
            want = "sha256=" + base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
            assert (digest, int(size)) == (want, len(data)), name


def test_the_same_release_gives_the_same_bytes(tmp_path):
    first = built(tmp_path, out="one")
    second = built(tmp_path, out="two")
    for a, b in zip(first, second):
        assert a.read_bytes() == b.read_bytes(), a.name


def test_an_archive_that_lost_its_binary_or_its_licence_is_refused(tmp_path):
    binaries = release_archives(tmp_path)
    os.remove(binaries / "linux-arm64" / "THIRD-PARTY-NOTICES.md")
    with pytest.raises(SystemExit, match="has no THIRD-PARTY-NOTICES.md, which has to go with the binary"):
        assemble.build("0.6.0", binaries, tmp_path / "out")
    os.remove(binaries / "win32-x64" / "tfg.exe")
    with pytest.raises(SystemExit, match="holds no tfg.exe"):
        assemble.build("0.6.0", binaries, tmp_path / "again")


def test_a_link_in_the_macos_app_is_refused(tmp_path):
    binaries = release_archives(tmp_path)
    target = binaries / "darwin-arm64" / "tfg.app" / "Contents" / "Info.plist"
    link = binaries / "darwin-arm64" / "tfg.app" / "Contents" / "Resources"
    try:
        os.symlink(target, link)
    except OSError as err:
        if sys.platform == "win32":
            pytest.skip(f"this Windows user cannot make a link ({err}). The release builds the wheels on Linux.")
        raise
    with pytest.raises(SystemExit, match="is a link, and a wheel cannot hold one"):
        assemble.build("0.6.0", binaries, tmp_path / "out")


def test_an_out_directory_already_there_is_not_written_into(tmp_path):
    built(tmp_path)
    with pytest.raises(FileExistsError):
        built(tmp_path)
