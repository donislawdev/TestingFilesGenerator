"""Are the tool versions pinned in ci.yml still the current ones?

Reads the pins out of the workflows rather than carrying its own copy, because
a list copied into a checker stops describing the thing it checks the moment
somebody edits the original.

Asks the Go MODULE PROXY, not the GitHub releases API, and that is the whole
correctness of this file. The first version asked GitHub and produced two false
alarms out of three: staticcheck releases under two names at once, so v0.8.1 was
reported behind "2026.2.1" when they are the same thing, and golang.org/x/vuln
answered with v1.1.4 as its latest while the pin was already on v1.7.0. A pin is
a MODULE VERSION - `go run module@version` - so the module system is the thing
that knows what the newest one is, and it answers in the same words the pin is
written in.

That matters more than tidiness here. A gate that cries wolf gets ignored, and
an ignored gate is what this whole file was written to replace.

Exits non-zero when a pin is behind, and also when it finds no pin at all: a
checker that quietly checks nothing is the same failure wearing a green tick.
"""

import json
import pathlib
import re
import sys
import urllib.request

WORKFLOWS = pathlib.Path(__file__).resolve().parents[1] / "workflows"
PROXY = "https://proxy.golang.org/{}/@latest"


def pins():
    """Every `go run module/path/cmd/x@version` this repository's workflows run."""
    found = {}
    for path in sorted(WORKFLOWS.glob("*.yml")):
        text = path.read_text(encoding="utf-8")
        for m in re.finditer(r"go run ([\w./-]+?)/cmd/[\w-]+@(v[\w.+-]+)", text):
            found[m.group(1)] = m.group(2)
    return found


def latest(module):
    """What the module proxy calls the newest version of this module."""
    # Module paths are case encoded for the proxy: an upper case letter becomes
    # "!" and the lower case letter. None of ours have one today, and doing it
    # anyway costs a line and stops a silent 404 the day one does.
    encoded = re.sub(r"[A-Z]", lambda c: "!" + c.group(0).lower(), module)
    with urllib.request.urlopen(PROXY.format(encoded), timeout=30) as r:  # noqa: S310 - fixed https host
        return json.load(r)["Version"]


def main():
    found = pins()
    if not found:
        print("FAIL: no pinned tool was found in the workflows, so this checked nothing")
        return 1

    behind = []
    for module, pinned in sorted(found.items()):
        try:
            now = latest(module)
        except Exception as exc:  # noqa: BLE001 - any failure here has to be loud
            print(f"FAIL: could not ask the module proxy about {module}: {exc}")
            return 1
        state = "current" if now == pinned else f"BEHIND, latest is {now}"
        print(f"  {module:<42} {pinned:<10} {state}")
        if now != pinned:
            behind.append((module, pinned, now))

    print()
    if not behind:
        print(f"all {len(found)} pinned tool(s) are current")
        return 0

    print(f"{len(behind)} pinned tool(s) are behind:")
    for module, pinned, now in behind:
        print(f"  {module} {pinned} -> {now}")
    print()
    print("Nothing else watches these. Dependabot reads go.mod and `uses:`, and these are shell")
    print("commands, so this job is the only thing that will ever say so. Raise them in .github/")
    print("workflows/ci.yml, and check the new version against this tree before merging - the last")
    print("two of these to go stale had stopped working with the compiler entirely.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
