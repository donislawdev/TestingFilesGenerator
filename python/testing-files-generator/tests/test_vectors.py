"""What a file says of itself and what check makes of what a system did.

Read from the file the npm package's tests read as well
(testdata/wrappers/vectors.json), so a sentence changed in one package and not
in the other fails here or there.
"""

import os

import pytest
from conftest import VECTORS

from testing_files_generator import ExpectationError, TestFile


class _Written:
    def write(self):
        return os.path.join(os.sep, "set")


def file(name):
    assert name in VECTORS["entries"], f"vectors.json has no entry {name}"
    return TestFile(_Written(), VECTORS["entries"][name])


def test_every_file_says_what_the_vectors_say_it_says():
    assert len(VECTORS["files"]) >= 4
    for want in VECTORS["files"]:
        f = file(want["entry"])
        assert f.title == want["title"], want["entry"]
        assert f.marks == want["marks"], want["entry"]
        assert f.why_unspecified == want["why_unspecified"], want["entry"]


def test_check_makes_of_every_observation_what_the_vectors_say_it_makes():
    kinds = set()
    for c in VECTORS["checks"]:
        label = f"{c['entry']} {c['observed']!r}"
        kinds.add(c["result"])
        f = file(c["entry"])
        if c["result"] == "pass":
            f.check(c["observed"])
        elif c["result"] == "mismatch":
            with pytest.raises(ExpectationError) as caught:
                f.check(c["observed"])
            assert str(caught.value) == c["message"], label
        elif c["result"] == "unspecified":
            with pytest.raises(RuntimeError, match="has no expected outcome, so there is nothing to check it against"):
                f.check(c["observed"])
        elif c["result"] == "bad-input":
            with pytest.raises(TypeError):
                f.check(c["observed"])
        else:
            pytest.fail(f"vectors.json has a result {c['result']} this test does not know")
    # Every kind is there, so a vectors file that lost one is not green by
    # having nothing left to compare.
    assert sorted(kinds) == ["bad-input", "mismatch", "pass", "unspecified"]


def test_a_difference_carries_what_was_expected_and_what_was_seen():
    with pytest.raises(ExpectationError) as caught:
        file("big").check({"outcome": "reject", "reason": "mime_mismatch"})
    assert caught.value.expected == {"outcome": "reject", "reason": "size_limit"}
    assert caught.value.observed == {"outcome": "reject", "reason": "mime_mismatch"}
    assert isinstance(caught.value, AssertionError), "pytest is to show a difference as a failed test"


def test_the_path_is_the_set_directory_joined_with_the_file():
    assert file("ok").path() == os.path.join(os.sep, "set", "ok.png")


def test_a_file_is_not_taken_for_a_test_class():
    assert TestFile.__test__ is False
