import os

import pytest

from testing_files_generator._binary import PLATFORMS, binary_in, binary_path, system_name


def test_tfg_binary_is_taken_as_given_and_named_when_it_is_not_there(tmp_path):
    given = tmp_path / "my-tfg"
    given.write_bytes(b"")
    assert binary_path(env={"TFG_BINARY": str(given)}) == str(given)
    missing = str(tmp_path / "gone")
    with pytest.raises(RuntimeError, match="TFG_BINARY is set to .* and there is no file there") as caught:
        binary_path(env={"TFG_BINARY": missing})
    assert "unset it" in str(caught.value)


def test_each_system_has_its_binary_where_its_release_archive_puts_it():
    assert binary_in("win32") == "tfg.exe"
    assert binary_in("darwin") == "tfg.app/Contents/MacOS/tfg"
    assert binary_in("linux") == "tfg"


@pytest.mark.parametrize(("system", "machine", "name"), [
    ("win32", "AMD64", "win32 x64"),
    ("win32", "ARM64", "win32 arm64"),
    ("linux", "x86_64", "linux x64"),
    ("linux", "aarch64", "linux arm64"),
    ("darwin", "arm64", "darwin arm64"),
    ("darwin", "x86_64", "darwin x64"),
    ("linux", "i686", "linux i686"),
])
def test_a_system_is_named_as_the_list_names_it(system, machine, name):
    assert system_name(system, machine) == name


def _installed(tmp_path, system):
    here = tmp_path / "testing_files_generator"
    binary = here / "bin" / binary_in(system)
    binary.parent.mkdir(parents=True)
    binary.write_bytes(b"")
    return here, binary


@pytest.mark.parametrize("platform", PLATFORMS)
def test_the_binary_of_an_installed_wheel_is_found_on_every_system_of_the_list(tmp_path, platform):
    system, arch = platform.split(" ")
    machine = {"x64": "x86_64", "arm64": "arm64"}[arch]
    here, binary = _installed(tmp_path, system)
    assert binary_path(env={}, system=system, machine=machine, bits=64, here=str(here)) == str(binary)


def test_a_system_with_no_binary_is_told_where_the_package_runs(tmp_path):
    with pytest.raises(RuntimeError) as caught:
        binary_path(env={}, system="linux", machine="i686", bits=32, here=str(tmp_path))
    assert "has no binary for linux i686" in str(caught.value)
    assert "Linux container" in str(caught.value) and "TFG_BINARY" in str(caught.value)
    with pytest.raises(RuntimeError, match="Rosetta"):
        binary_path(env={}, system="darwin", machine="x86_64", bits=64, here=str(tmp_path))


def test_the_wheel_with_no_binary_says_why_pip_took_it(tmp_path):
    with pytest.raises(RuntimeError) as caught:
        binary_path(env={}, system="win32", machine="AMD64", bits=32, here=str(tmp_path))
    assert "has no tfg binary: this Python is 32-bit" in str(caught.value)
    assert "TFG_BINARY" in str(caught.value)


def test_a_binary_removed_after_the_install_is_told_as_that(tmp_path):
    here, binary = _installed(tmp_path, "linux")
    os.remove(binary)
    with pytest.raises(RuntimeError, match="Something removed it after the install") as caught:
        binary_path(env={}, system="linux", machine="x86_64", bits=64, here=str(here))
    assert "--force-reinstall" in str(caught.value)
