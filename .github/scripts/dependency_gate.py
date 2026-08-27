#!/usr/bin/env python3
"""Decide a pull request from GitHub's dependency review data.

    gh api "repos/$REPO/dependency-graph/compare/$BASE...$HEAD" > deps.json
    python .github/scripts/dependency_gate.py deps.json

Why this exists next to actions/dependency-review-action
--------------------------------------------------------
The action already fails a pull request that ADDS a dependency with a known
vulnerability, and it does that well. It cannot do the other half. Its own
documentation is explicit: "If we can't detect the license for a dependency we
will inform you, but the action won't fail."

This project is GPL-3.0 and ships a binary, so an unidentified licence is not an
informational note. It is the one answer nobody can act on, because whether the
thing may be distributed at all is exactly what it fails to say. The same data
is read here and an unknown licence blocks, like a denied one. The REST field is
documented as "string or null", and null is what "not determined" looks like.

Why this lives in .github/scripts and not in tools
--------------------------------------------------
tools/ is outside this repository by an owner decision from 2026-08-01, on the
grounds that it is never shipped. This script is the opposite: CI checks out the
repository and runs it, so it has to travel with the repository. That also puts
it under the guards that read public text, which is correct for a file the world
can see.

Scope
-----
Only dependencies that a pull request ADDS. Removing something never creates an
obligation, and re-checking what is already in the tree would make every pull
request answer for decisions taken years ago.
"""
import argparse
import json
import sys

# SPDX identifiers this project may distribute alongside its own code.
#
# The criterion is recorded rather than invented here: LICENSING.md states that
# a library must be compatible with GPL-3.0, that Apache-2.0, MIT, BSD, LGPL and
# MPL-2.0 qualify, and that GPL-2.0-only does not. GPL-2.0-only is therefore
# ABSENT on purpose. It is famously incompatible with GPL-3.0 and it is exactly
# the kind of entry that looks fine in a list of open source licences and is not.
#
# Every identifier below except the last was measured in this repository's own
# dependency graph on 2026-08-27, so the list describes what we actually have
# rather than what somebody expected us to have.
ALLOWED = frozenset({
    "0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "CC0-1.0", "ISC",
    "MIT", "MIT-0", "MPL-2.0", "Unlicense", "Zlib",
    "LGPL-2.1-only", "LGPL-2.1-or-later", "LGPL-3.0-only", "LGPL-3.0-or-later",
    "GPL-3.0-only", "GPL-3.0-or-later",

    # The Go project's PATENTS file, which scancode names and GitHub reports
    # beside the BSD licence of golang.org/x/sys, x/net and x/image. Read on
    # 2026-08-27 from the pinned module rather than recalled: it is an
    # "Additional IP Rights Grant" that GIVES a patent licence. It grants rights
    # and restricts no distribution, so it cannot make a dependency unusable
    # here. Without this entry three modules already in the tree would be
    # blocked the next time they moved.
    "LicenseRef-scancode-google-patent-license-golang",
})

# Packages whose licence GitHub cannot resolve and which a person has already
# looked at. A name here is a decision with a reason, not a way to make a red
# build green - and it names one package, never a whole ecosystem.
EXCEPTIONS = {
    # (empty on purpose - add "name": "why this is fine" when it happens)
}

# GitHub reports license: null for every action, measured on this repository on
# 2026-08-27: seven of seven came back null while all 36 Go modules carried a
# real licence.
#
# They are skipped, and not for convenience. An action is CI machinery that
# never reaches a user, so it creates no distribution obligation, which is the
# thing an unknown licence is dangerous for. Keeping them would block every pull
# request that touches a workflow for ever, and a gate that always fires is a
# gate people learn to bypass.
#
# What actions ARE checked for is stricter and lives next door: every one must
# name a commit rather than a tag, which TestEveryActionIsPinnedToACommitRatherThanATag
# refuses to let slide. That guard is what makes this skip honest.
SKIPPED_ECOSYSTEMS = frozenset({"actions"})


def added(review):
    return [d for d in review
            if str(d.get("change_type", "")) == "added"
            and str(d.get("ecosystem", "")).lower() not in SKIPPED_ECOSYSTEMS]


def parts_of(expression):
    """The individual identifiers in an SPDX expression.

    Both AND and OR are split the same way and every part has to be allowed.
    For AND that is simply correct. For OR it is stricter than the licence
    requires, because an OR lets the user pick the half they like - and that is
    deliberate: being wrong this way can only block a dependency somebody then
    records a decision about, while the generous reading can let one through
    unnoticed. A blocked dependency asks a question. An admitted one does not.
    """
    flat = str(expression).replace(" AND ", " OR ")
    return [p.strip("() ") for p in flat.split(" OR ") if p.strip("() ")]


def verdict(dependency):
    """("ok" | "unknown" | "denied", licence) for one added dependency."""
    licence = dependency.get("license")
    name = str(dependency.get("name", "?"))
    if name in EXCEPTIONS:
        return "ok", licence
    if licence is None or not str(licence).strip():
        return "unknown", licence
    if all(part in ALLOWED for part in parts_of(licence)):
        return "ok", licence
    return "denied", licence


def split(review):
    blocked, passed = [], []
    for dependency in added(review):
        state, licence = verdict(dependency)
        row = (state, str(dependency.get("name", "?")),
               str(dependency.get("version", "?")), licence,
               str(dependency.get("scope", "?")))
        (passed if state == "ok" else blocked).append(row)
    return blocked, passed


def report_lines(blocked, passed):
    lines = []
    if blocked:
        lines.append("blocked - a licence that is denied or could not be determined:")
        for state, name, version, licence, scope in blocked:
            lines.append("  %-8s %s %s  licence=%s  scope=%s"
                         % (state, name, version, licence, scope))
        lines.append("")
        lines.append("An unknown licence blocks on purpose. This project is GPL-3.0 and ships a")
        lines.append("binary, so 'we could not tell' is the one answer nobody can act on. Read the")
        lines.append("LICENSE file of the pinned version, then record the decision in")
        lines.append(".github/scripts/dependency_gate.py, in ALLOWED or in EXCEPTIONS.")
    if passed:
        lines.append("allowed (%d): %s" % (
            len(passed), ", ".join("%s %s (%s)" % (n, v, lic) for _s, n, v, lic, _sc in passed)))
    lines.append("dependency gate: %d blocked, %d allowed" % (len(blocked), len(passed)))
    return lines


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("review", help="JSON from the dependency review API")
    args = parser.parse_args(argv)
    try:
        with open(args.review, encoding="utf-8") as handle:
            review = json.load(handle)
    except (OSError, ValueError) as exc:
        # A gate that cannot read its input has not passed anything. The API
        # answers 403 for some repository shapes, and that has to look like a
        # failure rather than an empty list of problems.
        print("dependency gate: cannot read %s: %s" % (args.review, exc), file=sys.stderr)
        return 2
    if not isinstance(review, list):
        print("dependency gate: expected a list of dependencies, got %s"
              % type(review).__name__, file=sys.stderr)
        return 2
    blocked, passed = split(review)
    for line in report_lines(blocked, passed):
        print(line)
    return 1 if blocked else 0


if __name__ == "__main__":
    raise SystemExit(main())
