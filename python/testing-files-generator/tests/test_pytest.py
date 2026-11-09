"""The set as pytest sees it: one test for each file, by its title, its markers and its skip."""

import os
import re
import subprocess
import sys

import pytest
from conftest import SMALL, SRC, tfg_binary

from testing_files_generator import _fileset, file_set


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


def _run(pytester, cache, *args, module=None):
    pytester.makepyfile(test_uploads=module or _module(cache))
    pytester.makeini("[pytest]\n")
    # The plugin is named here because the package runs from src. Where it is
    # installed too, pytest would load it a second time through its entry
    # point and refuse to start, so the entry point is blocked.
    return pytester.runpytest_subprocess("-p", "no:testing_files_generator",
                                         "-p", "testing_files_generator.pytest_plugin", "--strict-markers",
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


def test_the_markers_a_group_cannot_become_are_the_ones_pytest_registers_itself(tmp_path):
    # Read from pytest and not written out a second time, so a marker a later
    # pytest adds is refused too.
    ini = tmp_path / "pytest.ini"
    ini.write_text("[pytest]\n", encoding="utf-8")
    listed = subprocess.run([sys.executable, "-m", "pytest", "--markers", "-p", "no:cacheprovider", "-c", str(ini)],
                            cwd=tmp_path, env={**os.environ, "PYTEST_DISABLE_PLUGIN_AUTOLOAD": "1"},
                            capture_output=True, text=True, encoding="utf-8", check=False)
    assert listed.returncode == 0, listed.stderr
    own = set(re.findall(r"^@pytest\.mark\.([A-Za-z0-9_-]+)", listed.stdout, re.M))
    assert "skip" in own, f"pytest --markers lists no skip, so this reads nothing:\n{listed.stdout}"
    assert own == _fileset.PYTEST_OWN_MARKS


def test_a_group_that_would_become_one_of_pytests_own_markers_is_refused_before_a_test_runs(pytester, tmp_path):
    cache = tmp_path / "cache"
    recipe = tmp_path / "set.yaml"
    recipe.write_text("version: 1\ntargets:\n  - id: one\n    format: txt\n    size: 1kb\n    group: skip\n",
                      encoding="utf-8")
    module = f"""
import pytest
from testing_files_generator import file_set

files = file_set(recipe={str(recipe)!r}, cache_dir={str(cache)!r}, binary={tfg_binary()!r})


@pytest.mark.parametrize("file", files.params())
def test_file(file):
    assert False, "a file of a group named skip ran"
"""
    result = _run(pytester, cache, module=module)
    result.assert_outcomes(errors=1)
    result.stdout.fnmatch_lines(["*The group skip of one_0001.txt becomes the pytest marker skip, which is one of "
                                 "pytest's own*Give the group another name in the recipe.*"])
