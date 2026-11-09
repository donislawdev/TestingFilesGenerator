# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Running tfg and telling how it ended."""

from __future__ import annotations

import os
import signal
import subprocess
import sys
import tempfile
from collections.abc import Sequence

# How much of what tfg says on its error stream is kept for a message. Its
# refusals are a few lines, and a run of many files can print a note for each.
KEPT_ERROR = 64 * 1024

# A console window is not opened for tfg when the tests run without one.
_FLAGS = subprocess.CREATE_NO_WINDOW if sys.platform == "win32" else 0


def ended(status: int | str) -> str:
    """How tfg stopped - with a code, or by a signal named as it is."""
    return f"was stopped by {status}" if isinstance(status, str) else f"ended with code {status}"


def said(stderr: str) -> str:
    return stderr.strip() or "tfg said nothing about why."


def not_started(binary: str, cause: OSError) -> str:
    """A binary the system would not start, with the system's reason and the way out.

    The reason stays: "Exec format error" and "Permission denied" are what
    tell a binary for another system from one without its execute bit.
    """
    return (f"tfg could not be started from {binary}: {cause}. It has to be a tfg for this system that this "
            "user can run. Reinstall the package with pip install --force-reinstall testing-files-generator, "
            "or set TFG_BINARY to the path of such a tfg.")


class TfgError(Exception):
    """tfg ending with a code other than nought, or stopped by a signal, whose name is then the status.

    The message carries what tfg said, because its refusals already name the
    value and the way out. A caller that can say what went wrong in the words
    the test was written in passes its own message, and the status and
    everything tfg said stay on the error.
    """

    def __init__(self, args: Sequence[str], status: int | str, stderr: str, message: str | None = None):
        if message is None:
            message = f"tfg {' '.join(args)} {ended(status)}.\n{said(stderr)}"
        super().__init__(message)
        self.status = status
        self.stderr = stderr


def _status(code: int) -> int | str:
    # A negative code on Linux and macOS is the signal that stopped the process.
    if code < 0:
        try:
            return signal.Signals(-code).name
        except ValueError:
            return f"signal {-code}"
    return code


def _tail(handle) -> str:
    handle.seek(0, os.SEEK_END)
    size = handle.tell()
    handle.seek(max(0, size - KEPT_ERROR))
    return handle.read().decode("utf-8", errors="replace")


def _run(binary: str, args: Sequence[str], stdout) -> None:
    # The error stream goes to a file with no name, because a pipe that fills
    # stops tfg part way, and only its end is kept.
    with tempfile.TemporaryFile() as err:
        try:
            code = subprocess.run([binary, *args], stdin=subprocess.DEVNULL, stdout=stdout, stderr=err,
                                  creationflags=_FLAGS, check=False).returncode
        except OSError as cause:
            raise RuntimeError(not_started(binary, cause)) from cause
        if code != 0:
            raise TfgError(args, _status(code), _tail(err))


def run_to_file(binary: str, args: Sequence[str], out_file: str) -> None:
    """Runs tfg to the end, writing its standard output to a file.

    A file and not memory, because the manifest of a large set is large. The
    file is made new and readable by its owner alone, so neither a file nor a
    link someone else left under the same name is written through.
    """
    fd = os.open(out_file, os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_BINARY", 0), 0o600)
    with os.fdopen(fd, "wb") as out:
        _run(binary, args, out)


def run(binary: str, args: Sequence[str]) -> None:
    """Runs tfg to the end, keeping the end of what it says on its error stream."""
    _run(binary, args, subprocess.DEVNULL)
