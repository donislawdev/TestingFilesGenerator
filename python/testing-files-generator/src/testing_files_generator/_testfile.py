# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""One file of a set, with what the system under test should do with it."""

from __future__ import annotations

import os
import re
from collections.abc import Mapping
from typing import Any

# The closed list of outcomes a manifest declares, from docs/MANIFEST.md. A
# guard holds this copy to recipe.Outcomes in the program, because two copies
# of a closed list is how they drift.
OUTCOMES = ("accept", "reject", "sanitize", "unspecified")

# How each outcome reads in a sentence about what a system did with a file.
_DONE = {"accept": "taken", "reject": "turned away", "sanitize": "taken and cleaned"}


class ExpectationError(AssertionError):
    """What check raises when a system did something other than what the file expects."""

    def __init__(self, message: str, expected: Mapping[str, Any], observed: Mapping[str, Any]):
        super().__init__(message)
        self.expected = expected
        self.observed = observed


def mark_name(word: str) -> str:
    """A word as a pytest marker - anything other than a letter, digit, underscore or hyphen becomes a hyphen."""
    return re.sub(r"[^A-Za-z0-9_-]+", "-", str(word))


def expected_text(expected: Mapping[str, Any]) -> str:
    reason = expected.get("reason")
    return f"{expected['outcome']} ({reason})" if reason else expected["outcome"]


class TestFile:
    """One file of a set: a title, its marks, its path once the set is written, and a check against the manifest."""

    # Not a test class, whatever its name, when a test module imports it.
    __test__ = False

    def __init__(self, source, entry: Mapping[str, Any]):
        self._source = source
        self.entry = entry

    @property
    def relative_path(self) -> str:
        return self.entry["path"]

    @property
    def name(self) -> str:
        return self.entry["name"]

    @property
    def send_as(self) -> str:
        """The name to upload the file under.

        A set can hold files whose test is a name no disk stores - one that
        climbs out of a directory, say. Such a file has an ordinary name on
        disk and the other one in the manifest's send_as. Every other file
        goes under its own name, so one upload loop written with this serves
        the whole set.
        """
        return self.entry.get("send_as") or self.entry["name"]

    @property
    def format(self) -> str:
        return self.entry["format"]

    @property
    def bytes(self) -> int:
        return self.entry["bytes"]

    @property
    def group(self) -> str:
        return self.entry.get("group") or ""

    @property
    def target(self) -> str:
        return self.entry["target_id"]

    @property
    def purpose(self) -> str:
        return self.entry.get("purpose") or ""

    @property
    def expected(self) -> Mapping[str, Any]:
        return self.entry["expected"]

    @property
    def unspecified(self) -> bool:
        return self.entry["expected"]["outcome"] == "unspecified"

    @property
    def title(self) -> str:
        """The path and what the file expects. The path is unique in a set, which a test id has to be."""
        return f"{self.entry['path']} - expected {expected_text(self.entry['expected'])}"

    @property
    def marks(self) -> list[str]:
        """The pytest markers of the file: its expected outcome and its group."""
        names = [mark_name(self.entry["expected"]["outcome"])]
        if self.group:
            names.append(mark_name(self.group))
        return names

    @property
    def why_unspecified(self) -> str:
        why = f"{self.entry['path']} has no expected outcome - what your system does with it is its own policy."
        return f"{why} {self.purpose}" if self.purpose else why

    def path(self) -> str:
        """Writes the set the first time any of its files is asked for, and gives this file's absolute path."""
        return os.path.join(self._source.write(), *self.entry["path"].split("/"))

    def check(self, observed: str | Mapping[str, Any]) -> None:
        """Compares what the system did with what the manifest expects, and raises when they differ.

        The error carries the file's purpose, the sentence that says what the
        difference means. A reason is compared only for a refusal: a reason
        names the rule a file is about, so a file a byte under a limit is taken
        with size_limit and there is nothing to compare it with.
        """
        __tracebackhide__ = True  # pytest shows the line of the test, not this one
        want = self.entry["expected"]
        if want["outcome"] == "unspecified":
            # Not an ExpectationError: the system did nothing wrong, the test
            # asked about a file with no right answer.
            raise RuntimeError(
                f"{self.entry['path']} has no expected outcome, so there is nothing to check it against. "
                "Skip it - the params of a set skip it on their own - or decide what your system should do "
                "with it and check that.")
        got = {"outcome": observed} if isinstance(observed, str) else observed
        if not isinstance(got, Mapping) or got.get("outcome") not in _DONE:
            raise TypeError("check takes accept, reject or sanitize, or a dict with an outcome and a reason. "
                            f"It was given {observed!r}.")
        if got["outcome"] != want["outcome"]:
            reason = want.get("reason")
            self._differs(f"{self.entry['path']} was {_DONE[got['outcome']]}, and it should have been "
                          f"{_DONE[want['outcome']]}" + (f" ({reason})." if reason else "."), got)
        if want["outcome"] == "reject" and want.get("reason") and got.get("reason") \
                and got["reason"] != want["reason"]:
            self._differs(f"{self.entry['path']} was turned away for {got['reason']}, "
                          f"and the rule it is about is {want['reason']}.", got)

    def _differs(self, sentence: str, observed: Mapping[str, Any]) -> None:
        __tracebackhide__ = True
        message = f"{sentence}\n{self.purpose}" if self.purpose else sentence
        raise ExpectationError(message, self.entry["expected"], dict(observed))

    def __repr__(self) -> str:
        return f"<TestFile {self.title}>"
