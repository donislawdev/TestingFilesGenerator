# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Installs the wheels of a release on the system this runs on, the way people install them, and uses them.

.github/workflows/pypi.yml runs it on every system before anything reaches
PyPI. It makes a fresh environment with pip or with uv, installs
testing-files-generator from the wheels alone, and then checks that:

- the system is the one it was told to try, so a runner that moved to
  another architecture does not try one wheel and report on another,
- tfg answers with the version of the release, and the installed binary is
  byte for byte the one in the release's archive,
- on macOS the app the binary runs in is still signed,
- a whole set is written through file_set, every file the size it should be,
- pytest-testing-files-generator installs testing-files-generator of its own
  version, pytest loads the plugin, and a set's params pass.

    python try_wheels.py <dist> <version> <binaries.txt> <system, such as linux-x64> pip|uv
"""

from __future__ import annotations

import hashlib
import os
import re
import subprocess
import sys
import tempfile
import textwrap
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from assemble import pep440  # noqa: E402

SET = """
from testing_files_generator import file_set
s = file_set(preset="upload-validation", options={"limit": "64kb", "bulk": 0})
assert len(s) > 0, "the set has no files"
import os
for f in s.files:
    assert os.path.getsize(f.path()) == f.bytes, f"{f.title} is not the size the manifest gives"
    if not f.unspecified:
        o = f.expected["outcome"]
        f.check({"outcome": o, "reason": f.expected.get("reason")} if o == "reject" else o)
print(f"{len(s)} files written, each the size it should be")
"""

TEST_MODULE = """
import pytest
from testing_files_generator import file_set

uploads = file_set(preset="upload-validation", options={"limit": "64kb", "bulk": 0})


@pytest.mark.parametrize("file", uploads.params())
def test_upload(file):
    file.check(file.expected["outcome"])
"""


def fail(message: str) -> None:
    print(f"::error::{message}", flush=True)
    raise SystemExit(1)


def run(*args: str, **kwargs) -> str:
    print("+", " ".join(args), flush=True)
    done = subprocess.run(args, capture_output=True, text=True, encoding="utf-8", check=False, **kwargs)
    if done.returncode != 0:
        print(done.stdout, done.stderr, sep="\n")
        fail(f"{' '.join(args[:3])} ended with code {done.returncode}")
    return done.stdout


def main(dist: str, version: str, binaries: str, system: str, installer: str) -> None:
    dist_dir = Path(dist).resolve()
    links = ["--find-links", str(dist_dir / "main"), "--find-links", str(dist_dir / "plugin")]
    wanted = pep440(version)
    work = Path(tempfile.mkdtemp(prefix="try-wheels-"))
    venv = work / "venv"
    if installer == "uv":
        # uv picks an interpreter of its own unless told, and this pass tries
        # the Python the pip pass tries, the oldest the package declares on
        # one runner.
        run("uv", "venv", "--quiet", "--python", sys.executable, str(venv))
    else:
        run(sys.executable, "-m", "venv", str(venv))
    bin_dir = venv / ("Scripts" if os.name == "nt" else "bin")
    python = str(bin_dir / ("python.exe" if os.name == "nt" else "python"))

    def install(*what: str, index: bool = False) -> None:
        if installer == "uv":
            run("uv", "pip", "install", "--quiet", "--python", python, *([] if index else ["--no-index"]), *links, *what)
        else:
            run(python, "-m", "pip", "install", "--quiet", "--disable-pip-version-check",
                *([] if index else ["--no-index"]), *links, *what)

    install(f"testing-files-generator=={wanted}")

    here = run(python, "-c", "from testing_files_generator._binary import system_name; "
                             "print(system_name().replace(' ', '-'))").strip()
    if here != system:
        fail(f"this runner is {here}, and it was to try {system}. Give {system} a runner of its own system.")

    tfg = str(bin_dir / ("tfg.exe" if os.name == "nt" else "tfg"))
    said = run(tfg, "version").strip()
    if said != version:
        fail(f"tfg version says {said}, and the release is {version}, so the wheel carries another release's "
             "binary. Nothing is published. Look at the archives of the release before running this again.")

    installed = run(python, "-c", "import testing_files_generator as t; print(t.binary_path())").strip()
    sums = dict((line.split()[0], line.split()[2]) for line in Path(binaries).read_text().splitlines() if line)
    got = hashlib.sha256(Path(installed).read_bytes()).hexdigest()
    if got != sums.get(system):
        fail(f"{installed} hashes to {got}, and the binary in the release archive to {sums.get(system)}")
    print(f"{installed} is the binary of the release")

    if sys.platform == "darwin":
        # The binary runs only inside the app it was signed in, so the app has
        # to come out of the wheel whole.
        run("codesign", "--verify", "--deep", "--strict", str(Path(installed).parents[2]))

    print(run(python, "-c", SET, cwd=work).strip())

    install("pytest", index=True)
    install(f"pytest-testing-files-generator=={wanted}")
    (work / "test_uploads.py").write_text(textwrap.dedent(TEST_MODULE), encoding="utf-8")
    out = run(python, "-m", "pytest", "-q", "-p", "no:cacheprovider", "--strict-markers", str(work / "test_uploads.py"),
              cwd=work, env={**os.environ, "TFG_CACHE_DIR": str(work / "sets")})
    summary = out.strip().splitlines()[-1]
    # pytest ends with code nought when every test is skipped, so a params()
    # that skips all of them would pass here without a file checked.
    passed = re.search(r"(\d+) passed", summary)
    if not passed or int(passed.group(1)) == 0:
        fail(f"pytest passed no test of the set. It said: {summary}")
    print(summary)
    header = run(python, "-m", "pytest", "--collect-only", "-q", "-p", "no:cacheprovider", "-v",
                 str(work / "test_uploads.py"), cwd=work)
    if f"testing-files-generator: tfg at {installed}" not in header:
        fail("pytest did not load the plugin of testing-files-generator, so its report header names no tfg")
    print(f"{system} with {installer}: installed, tfg {said}, a set written, the plugin loaded")


def without_binary(dist: str, version: str) -> None:
    """The wheel pip takes on a system with no binary installs, and its first call says why there is no tfg."""
    work = Path(tempfile.mkdtemp(prefix="try-any-"))
    run(sys.executable, "-m", "pip", "install", "--quiet", "--disable-pip-version-check", "--no-index",
        "--find-links", str(Path(dist).resolve() / "main"), "--only-binary=:all:", "--platform", "freebsd_14_0_amd64",
        "--target", str(work), f"testing-files-generator=={pep440(version)}")
    if (work / "testing_files_generator" / "bin").exists():
        fail("pip installed a wheel with a binary for a system no binary is made for")
    done = subprocess.run([sys.executable, "-c", "from testing_files_generator import binary_path; binary_path()"],
                          capture_output=True, text=True, encoding="utf-8", check=False,
                          env={**os.environ, "PYTHONPATH": str(work), "TFG_BINARY": ""})
    if done.returncode == 0 or "TFG_BINARY to its path" not in done.stderr:
        print(done.stderr)
        fail("the wheel with no binary does not say what to do when it is asked for tfg")
    print("the wheel with no binary installs where no other does, and says what to do")


if __name__ == "__main__":
    if sys.argv[1:2] == ["--without-binary"] and len(sys.argv) == 4:
        without_binary(sys.argv[2], sys.argv[3])
    elif len(sys.argv) == 6 and sys.argv[5] in ("pip", "uv"):
        main(*sys.argv[1:])
    else:
        raise SystemExit("usage: try_wheels.py <dist> <version> <binaries.txt> <system> pip|uv\n"
                         "       try_wheels.py --without-binary <dist> <version>")
