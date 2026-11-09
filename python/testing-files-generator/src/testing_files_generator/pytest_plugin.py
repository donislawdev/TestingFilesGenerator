# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""The pytest plugin, which pytest loads on its own once the package is installed.

It registers the markers a set's params carry, keeps sets in the run's
.pytest_cache, and names the tfg the run uses in the report header. Turn it
off with -p no:testing_files_generator.
"""

from __future__ import annotations

from typing import Any

from . import _fileset
from ._binary import binary_path

_MEANING = {
    "accept": "files the system under test should take",
    "reject": "files the system under test should turn away",
    "sanitize": "files the system under test should take and change",
    "unspecified": "files with no expected outcome, which are skipped",
}


def pytest_configure(config: Any) -> None:
    for outcome, meaning in _MEANING.items():
        config.addinivalue_line("markers", f"{outcome}: {meaning} (testing-files-generator)")
    _fileset.use_pytest(config)


def pytest_unconfigure(config: Any) -> None:
    _fileset.use_pytest(None)


def pytest_report_header(config: Any) -> str:
    try:
        return f"testing-files-generator: tfg at {binary_path()}"
    except RuntimeError as err:
        return f"testing-files-generator: {err}"
