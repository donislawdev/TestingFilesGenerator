"""Check that every mutation still points at code that exists.

An entry whose pattern is gone reports SKIP during a run and goes past in the
middle of twelve minutes of output, while the coverage guard sees the entry and
is satisfied. That is how 28 of 161 mutations went stale without anything
saying so, mostly when cli.go was split into seven files.

This is the check that costs nothing: it is pure text. Count the pattern in its
file and demand exactly one.

Two occurrences is a finding as well, not only zero. All three runners replace
the first occurrence and leave the rest, so a pattern that matches twice
mutates whichever one happens to come first in the file and the entry stops
saying which behaviour it breaks. Until 2026-08-26 they disagreed about even
that - the full runner replaced one and the other two replaced all of them -
which nothing noticed, because this check keeps the count at one.

Run after any refactor that moves code, rather than waiting for a full pass.
It reads the list and the files and nothing else, so it needs no package
installed.

    python internal/guard/mutation/staleness.py
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from entries import MUTATIONS  # noqa: E402
from pool import ROOT  # noqa: E402


def main():
    missing, doubled = [], []
    for label, rel, old, _new, _test in MUTATIONS:
        path = ROOT / rel
        if not path.exists():
            missing.append((label, rel, "the file is gone"))
            continue
        found = path.read_text(encoding="utf-8").count(old)
        if found == 0:
            missing.append((label, rel, "pattern not found"))
        elif found > 1:
            doubled.append((label, rel, f"pattern found {found} times"))

    print(f"{len(MUTATIONS)} mutations checked")
    for group, name in ((missing, "STALE"), (doubled, "AMBIGUOUS")):
        for label, rel, why in group:
            print(f"{name:<10} {label}\n           {rel}: {why}")

    if missing or doubled:
        print(f"\n{len(missing)} stale, {len(doubled)} ambiguous - "
              "each one is a guard nobody is proving")
        return 1
    print("every pattern occurs exactly once")
    return 0


if __name__ == "__main__":
    sys.exit(main())
