# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""tfg, and python -m testing_files_generator, run the binary this package installed.

The same arguments, the same streams and the same exit code, so a script
cannot tell it from tfg run directly.
"""

from __future__ import annotations

import os
import signal
import subprocess
import sys

from ._binary import binary_path

# RUNTIME in docs/CLI.md section 3 - the tool could not do its work, and the
# arguments were never looked at.
_CANNOT_RUN = 1


def main() -> int:
    try:
        binary = binary_path()
    except RuntimeError as err:
        print(f"tfg: {err}", file=sys.stderr)
        return _CANNOT_RUN
    args = [binary, *sys.argv[1:]]
    try:
        if sys.platform != "win32":
            # tfg takes this process's place, so signals and the exit code are
            # its own with nothing in between.
            os.execv(binary, args)
        # Windows has no such thing. A Ctrl+C in a console reaches tfg as well
        # as this process, so this one waits for tfg to stop the way it stops
        # on its own, and its exit code is the one a script sees.
        signal.signal(signal.SIGINT, signal.SIG_IGN)
        return subprocess.call(args)
    except OSError as err:
        print(f"tfg: tfg could not be started from {binary}: {err}", file=sys.stderr)
        return _CANNOT_RUN


if __name__ == "__main__":
    sys.exit(main())
