# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Real files of exact sizes and formats for your tests, and what the system under test should do with each one.

    from testing_files_generator import file_set

    uploads = file_set(preset="upload-validation", options={"limit": "2mb", "bulk": 0})

    @pytest.mark.parametrize("file", uploads.params())
    def test_upload(file):
        ...  # hand file.path() to your system
        file.check("accept" if taken else "reject")
"""

from ._binary import binary_path
from ._fileset import FileSet, UnfinishedSetWarning, file_set
from ._run import TfgError
from ._testfile import ExpectationError, TestFile

try:
    from importlib.metadata import PackageNotFoundError, version

    __version__ = version("testing-files-generator")
except PackageNotFoundError:  # run from the repository, not installed
    __version__ = "0.0.0.dev0"

__all__ = [
    "ExpectationError",
    "FileSet",
    "TestFile",
    "TfgError",
    "UnfinishedSetWarning",
    "binary_path",
    "file_set",
]
