# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Where the tfg binary this package runs is."""

from __future__ import annotations

import os
import platform as _platform
import sys
from collections.abc import Mapping

# The systems a release builds a command line for, named as Node names them so
# the list reads the same here and in the npm package. A guard holds it to the
# list of targets in .github/workflows/release.yml, and the release build in
# release/assemble.py makes one wheel with the binary for each of them. Intel
# Macs are on neither list, by the owner's decision of 2026-08-20.
PLATFORMS = ("win32 x64", "win32 arm64", "linux x64", "linux arm64", "darwin arm64")

# How a machine name reads as an architecture. Windows says AMD64 and ARM64,
# Linux x86_64 and aarch64, macOS x86_64 and arm64.
_ARCH = {"amd64": "x64", "x86_64": "x64", "arm64": "arm64", "aarch64": "arm64"}

_HERE = os.path.dirname(os.path.abspath(__file__))


def binary_in(system: str) -> str:
    """Where the binary sits inside the package's bin directory, as the release archive lays it out.

    On macOS that is inside the app the release signed and notarised: its
    signature covers the app's Info.plist, and macOS kills the binary taken out
    on its own (exit 137, measured 2026-11-09 on macOS 27.2 -
    docs/NPM-PUBLISH-2026-11-09.md).
    """
    if system == "win32":
        return "tfg.exe"
    if system == "darwin":
        return "tfg.app/Contents/MacOS/tfg"
    return "tfg"


def system_name(system: str | None = None, machine: str | None = None) -> str:
    """This system as PLATFORMS names one, such as linux x64."""
    system = sys.platform if system is None else system
    if system.startswith("linux"):
        system = "linux"
    machine = (_platform.machine() if machine is None else machine).lower()
    return f"{system} {_ARCH.get(machine, machine)}"


def _why_none(bits: int) -> str:
    # Why pip took the wheel without a binary on a system that has one. The
    # wheel pip takes is decided by the Python that installs it, and a 32-bit
    # Python on 64-bit Windows is matched to wheels for 32-bit Python.
    if bits == 32:
        return "this Python is 32-bit, and the binary comes in wheels for 64-bit Python"
    return "pip found no wheel with a binary for this system"


# A Python made for Intel runs on a Mac with Apple silicon under Rosetta, and
# says it is on an Intel Mac. A Python made for Apple silicon gets the wheel
# with the binary.
_ROSETTA = (" If this Mac has Apple silicon, this Python runs under Rosetta - install the package with a "
            "Python made for Apple silicon.")


def binary_path(*, env: Mapping[str, str] | None = None, system: str | None = None,
                machine: str | None = None, bits: int | None = None, here: str | None = None) -> str:
    """The tfg binary this package runs.

    TFG_BINARY comes first and is taken as given - a tfg installed another way,
    or one built from the repository. PATH is never searched: a different tfg
    found there would answer with its own version and say nothing about it.
    Everything is passed in so the tests can ask about a system they are not on.
    """
    env = os.environ if env is None else env
    given = env.get("TFG_BINARY")
    if given:
        if not os.path.isfile(given):
            raise RuntimeError(f"TFG_BINARY is set to {given} and there is no file there. "
                               "Point it at a tfg binary, or unset it to use the one this package installs.")
        return given
    raw = sys.platform if system is None else system
    name = system_name(raw, machine)
    bits = (64 if sys.maxsize > 2**32 else 32) if bits is None else bits
    here = _HERE if here is None else here
    bin_dir = os.path.join(here, "bin")
    found = os.path.join(bin_dir, *binary_in(name.split(" ")[0]).split("/"))
    if os.path.isfile(found):
        return found
    known = ", ".join(PLATFORMS)
    if name not in PLATFORMS:
        raise RuntimeError(f"testing-files-generator has no binary for {name}. It comes for {known}. "
                           "On another system, run the tests in a Linux container, where pip installs the "
                           "package for Linux, or install tfg another way and set TFG_BINARY to its path."
                           + (_ROSETTA if name == "darwin x64" else ""))
    if not os.path.isdir(bin_dir):
        raise RuntimeError(f"This install of testing-files-generator has no tfg binary: {_why_none(bits)}. "
                           f"The binary comes for {known}, each in a wheel of its own. Install the package with "
                           "a 64-bit Python made for this system, or install tfg another way and set "
                           "TFG_BINARY to its path.")
    raise RuntimeError(f"The tfg binary of this install is not at {found}, where the package puts it. "
                       "Something removed it after the install. Install the package again with "
                       "pip install --force-reinstall testing-files-generator, or set TFG_BINARY to the path "
                       "of a tfg binary.")
