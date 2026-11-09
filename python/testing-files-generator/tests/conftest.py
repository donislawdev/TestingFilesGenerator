import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

PACKAGE = Path(__file__).resolve().parents[1]
SRC = str(PACKAGE / "src")
ROOT = PACKAGE.parents[1]
BUILT = ROOT / ("tfg.exe" if sys.platform == "win32" else "tfg")
VECTORS = json.loads((ROOT / "testdata" / "wrappers" / "vectors.json").read_text(encoding="utf-8"))

# A small set that still holds every kind of case: one either side of the
# limit, unspecified files and a group of several files.
SMALL = {"preset": "upload-validation", "options": {"limit": "1mb", "bulk": 2, "far-over": "off"}}


def tfg_binary() -> str:
    """TFG_BINARY, or the one the repository's build command leaves at its root.

    Neither is a failure and not a skip - a suite that skips without a binary
    is green while testing nothing.
    """
    binary = os.environ.get("TFG_BINARY") or str(BUILT)
    if not os.path.isfile(binary):
        raise RuntimeError(f"No tfg binary at {binary}. Build one from the repository root with "
                           f'go build -tags "$(cat .github/build-tags)" -o {BUILT.name} ./cmd/tfg, or set TFG_BINARY.')
    return binary


def python(script: str, *args: str, env: dict | None = None) -> subprocess.CompletedProcess:
    """Runs a script in a fresh Python with this package and a binary. A variable given as None is taken out."""
    child = {**os.environ, "TFG_BINARY": tfg_binary(), "PYTHONPATH": SRC}
    for key, value in (env or {}).items():
        if value is None:
            child.pop(key, None)
        else:
            child[key] = value
    return subprocess.run([sys.executable, "-c", script, *args], env=child, capture_output=True, text=True,
                          encoding="utf-8", check=False)


# The script a fresh process runs to write a set, printing its directory.
WRITER = """
import json, sys
from testing_files_generator import file_set
try:
    print(file_set(**json.loads(sys.argv[1])).write())
except Exception as err:
    print(err, file=sys.stderr)
    sys.exit(3)
"""


def writer(cache, spec=None) -> subprocess.CompletedProcess:
    return python(WRITER, json.dumps({**(spec or SMALL), "cache_dir": str(cache)}))


@pytest.fixture
def cache(tmp_path):
    return tmp_path / "cache"
