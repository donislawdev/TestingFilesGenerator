#!/usr/bin/env python3
"""Hold the licence registry to what a scanner finds in the built binaries.

The registry is the reviewed list of what this project ships, and the bill of
materials is generated from it. The scanner's job is not to write that document
- measured on 2026-08-27, syft reading the window binary names all thirty
modules with exact versions and attaches a licence to one of them - its job is
to ASK THE REGISTRY A QUESTION: does the thing we are about to publish contain
something the list does not know about?

That direction has already paid. Seven fonts and ninety-seven drawings shipped
inside the window binary for as long as it existed, named nowhere, because every
check this project had asked about modules and a font is a file inside one.

Usage:
    python .github/scripts/sbom_gate.py <syft-scan.json> <ours.spdx.json>

Exit codes:
    0  every name the scan found is accounted for
    1  the scan found something the registry does not know
    2  the scan itself is unusable, which is not the same as a clean tree
"""
import json
import re
import sys

# What a scan reports that is not a module of somebody else's, mapped to why it
# is fine. Anything not matched here has to appear in our own document by name.
#
# Written as patterns rather than as names because syft spells a binary
# classifier with the product name it read out of the file properties, and that
# name carries the version of the day.
NOT_A_DEPENDENCY = (
    # The Go runtime, which syft calls stdlib and our document calls what a
    # person calls it. Not a module, so it appears in no dependency list.
    (r"^stdlib$", "the Go runtime, named in our document as the runtime"),
    # The binaries themselves, recognised by their file properties.
    (r"^testing files generator$", "the program itself"),
    (r"^tfg(-gui)?$", "the program itself"),
    # Our own module, which is the program rather than something it carries.
    (r"^github\.com/donislawdev/testingfilesgenerator$", "our own module"),
)


def unusable(message):
    print("sbom gate: %s" % message)
    return 2


def load(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def main(argv):
    if len(argv) != 3:
        return unusable("usage: sbom_gate.py <syft-scan.json> <ours.spdx.json>")

    try:
        scan = load(argv[1])
        ours = load(argv[2])
    except (OSError, ValueError) as err:
        # A scanner that fell over writes something that is not a report, and a
        # gate reading its exit code would call that a clean tree. Measured in
        # this repository on 2026-08-27, with a different scanner and a 23 byte
        # file that was not JSON.
        return unusable("cannot read a report: %s" % err)

    artifacts = scan.get("artifacts")
    if not artifacts:
        return unusable("the scan names no artifact at all, so it scanned nothing")
    files = len(scan.get("files", []) or [])
    print("scan: %d artifact(s), %d file(s) read" % (len(artifacts), files))

    known = {package["name"].strip().lower() for package in ours.get("packages", [])}
    if len(known) < 10:
        return unusable("our own document holds %d packages, which cannot be right" % len(known))

    unaccounted = []
    for artifact in artifacts:
        name = str(artifact.get("name", "")).strip()
        low = name.lower()
        if low in known:
            continue
        if any(re.search(pattern, low) for pattern, _why in NOT_A_DEPENDENCY):
            continue
        unaccounted.append("%s %s" % (name, artifact.get("version", "")))

    if unaccounted:
        print("\nThe built binaries contain components the licence registry does not know:\n")
        for line in sorted(set(unaccounted)):
            print("  %s" % line)
        print(
            "\nRead the licence that ships with each of them, add an entry to internal/legal,\n"
            "and put it in THIRD-PARTY-NOTICES.md before releasing. An SBOM generated from\n"
            "that registry would otherwise claim to be complete while missing them."
        )
        return 1

    print("every component the scan found is accounted for by the registry")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
