"""The set as pytest sees it: one test for each file, by its title, its markers and its skip."""

import pytest
from conftest import SMALL, SRC, tfg_binary

from testing_files_generator import file_set


def _module(cache):
    return f"""
import pytest
from testing_files_generator import file_set

uploads = file_set(**{SMALL!r}, cache_dir={str(cache)!r}, binary={tfg_binary()!r})


@pytest.mark.parametrize("file", uploads.params())
def test_upload(file):
    file.check(file.expected["outcome"])
"""


@pytest.fixture(autouse=True)
def _package_on_the_path(monkeypatch):
    # The pytest a test here starts imports the package from src, as this one does.
    monkeypatch.setenv("PYTHONPATH", SRC)


def _run(pytester, cache, *args):
    pytester.makepyfile(test_uploads=_module(cache))
    pytester.makeini("[pytest]\n")
    return pytester.runpytest_subprocess("-p", "testing_files_generator.pytest_plugin", "--strict-markers",
                                         "-W", "error::pytest.PytestUnknownMarkWarning", "-p", "no:cacheprovider",
                                         *args)


def test_every_file_is_a_test_and_a_file_with_no_right_answer_is_skipped_with_the_reason(pytester, tmp_path):
    cache = tmp_path / "cache"
    s = file_set(**SMALL, cache_dir=cache, binary=tfg_binary())
    unspecified = len(s.only(outcome="unspecified"))
    result = _run(pytester, cache, "-rs")
    result.assert_outcomes(passed=len(s) - unspecified, skipped=unspecified)
    result.stdout.fnmatch_lines(["*has no expected outcome - what your system does with it is its own policy*"])


def test_the_test_ids_are_the_titles_of_the_files(pytester, tmp_path):
    cache = tmp_path / "cache"
    s = file_set(**SMALL, cache_dir=cache, binary=tfg_binary())
    result = _run(pytester, cache, "--collect-only", "-q")
    for f in s.files:
        if f.title.isascii():
            result.stdout.fnmatch_lines([f"*test_upload[[]{f.title}[]]*"])


def test_a_run_is_narrowed_by_the_expected_outcome_and_by_the_group(pytester, tmp_path):
    cache = tmp_path / "cache"
    s = file_set(**SMALL, cache_dir=cache, binary=tfg_binary())
    result = _run(pytester, cache, "-m", "reject")
    result.assert_outcomes(passed=len(s.only(outcome="reject")))
    result = _run(pytester, cache, "-m", "bulk-upload")
    result.assert_outcomes(passed=len(s.only(group="bulk-upload")))


def test_the_report_header_names_the_tfg_the_run_uses(pytester, tmp_path, monkeypatch):
    monkeypatch.setenv("TFG_BINARY", tfg_binary())
    result = _run(pytester, tmp_path / "cache", "--collect-only", "-q", "-v")
    result.stdout.fnmatch_lines([f"testing-files-generator: tfg at {tfg_binary()}"])
