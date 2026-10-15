"""Which entries a run takes: all of them, or the ones a change reaches.

A pull request runs the entries its change reaches, and the weekly run takes
every one. An entry is reached when the change touches

  - the file the entry mutates,
  - the file its guard lives in, or
  - the entry itself, which is new or different from the base.

A change to the runner reaches every entry, because every verdict passes
through it. A bumped dependency reaches none: an entry proves that a guard
notices a break in this repository's code, and a new version of somebody
else's module does not change which breaks are noticed. What a change reaches
in a third place - a helper both sides call - is what the weekly run is for.

Nothing here imports psutil or calls go, so the plan can be made by a job that
installs neither.
"""
import ast
import math
import re
import subprocess
from pathlib import Path

ENTRIES = "internal/guard/mutation/entries.py"

# Every verdict passes through these, so a change to any of them reaches every
# entry. build-tags is here because it decides what is compiled at all.
RUNNER_FILES = frozenset({
    "internal/guard/mutation/mutate.py",
    "internal/guard/mutation/runner.py",
    "internal/guard/mutation/pool.py",
    "internal/guard/mutation/selection.py",
    "internal/guard/mutation/report.py",
    ".github/requirements-mutation.txt",
    ".github/workflows/mutation.yml",
    ".github/build-tags",
})

# How a run is cut into jobs. A job takes about this many entries, and there
# are never more than this many jobs. Sixteen of the twenty jobs a free plan
# runs at once leaves room for the other workflows, and the weekly run of every
# entry fills sixteen. The first measured run is what these numbers wait for.
ENTRIES_PER_SHARD = 110
MOST_SHARDS = 16

GUARD_DECLARATION = re.compile(r"^func (Test\w+)\(", re.MULTILINE)


def read_entries(text: str) -> list[tuple]:
    """The MUTATIONS list in the text of entries.py, read without running it."""
    tree = ast.parse(text)
    for node in tree.body:
        if isinstance(node, ast.Assign) and getattr(node.targets[0], "id", "") == "MUTATIONS":
            return [tuple(entry) for entry in ast.literal_eval(node.value)]
    raise SystemExit(f"no MUTATIONS list in {ENTRIES}")


def git(root: Path, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(["git", *args], cwd=root, capture_output=True, text=True)


def changed_files(root: Path, base: str) -> set[str]:
    """Every path the change since base touches, refused loudly when git cannot say.

    A failure here must not read as an empty change. That would select nothing,
    and a run of nothing is green.
    """
    r = git(root, "diff", "--name-only", f"{base}...HEAD")
    if r.returncode != 0:
        raise SystemExit(f"git could not list what changed since {base}:\n{r.stderr}")
    return {line.strip() for line in r.stdout.splitlines() if line.strip()}


def entries_at(root: Path, base: str) -> list[tuple] | None:
    """The list as it stood at base, or None when the file did not exist there."""
    r = git(root, "show", f"{base}:{ENTRIES}")
    if r.returncode != 0:
        if "does not exist" in r.stderr or "exists on disk, but not in" in r.stderr:
            return None
        raise SystemExit(f"git could not read {ENTRIES} at {base}:\n{r.stderr}")
    return read_entries(r.stdout)


def guard_files(root: Path) -> dict[str, str]:
    """Each guard's name and the file it is declared in, relative to the root."""
    found: dict[str, str] = {}
    for path in sorted((root / "internal" / "guard").glob("*_test.go")):
        rel = path.relative_to(root).as_posix()
        for name in GUARD_DECLARATION.findall(path.read_text(encoding="utf-8")):
            found[name] = rel
    return found


def reached(entries: list[tuple], changed: set[str], before: list[tuple] | None,
            guards: dict[str, str]) -> list[int]:
    """The positions in entries that the change reaches, in list order."""
    if changed & RUNNER_FILES or before is None:
        return list(range(len(entries)))
    known = set(before)
    return [i for i, (_, rel, _, _, test) in enumerate(entries)
            if rel in changed or guards.get(test) in changed or entries[i] not in known]


def shard(positions: list[int], index: int, count: int) -> list[int]:
    """The part of positions that job index of count runs.

    Every count-th entry rather than a block, so the costly areas of the list
    are spread over all jobs instead of landing on one. The same inputs give
    the same part in every job, so no job needs another's answer.
    """
    return positions[index::count]


def shard_count(selected: int) -> int:
    """How many jobs a run of this many entries is cut into. Zero for none."""
    return min(MOST_SHARDS, math.ceil(selected / ENTRIES_PER_SHARD))


def canary(entries: list[tuple], guards: dict[str, str]) -> list[str]:
    """What is wrong with the selection, asked of a case whose answer is known.

    Run before every plan. A selection that quietly picks nothing turns every
    pull request into a green run of zero entries, and nothing else would ever
    say so - the guard would not be reached, and it would be green honestly.
    """
    faults = []
    if not entries:
        return ["the list holds no entry, so the selection was not tried"]
    first = entries[0]
    if 0 not in reached(entries, {first[1]}, entries, guards):
        faults.append(f"a change to {first[1]} did not reach the entry that mutates it")
    if first[4] not in guards:
        faults.append(f"the guard {first[4]} was not found in any file of internal/guard")
    elif 0 not in reached(entries, {guards[first[4]]}, entries, guards):
        faults.append(f"a change to {guards[first[4]]} did not reach an entry naming its guard")
    if 0 not in reached(entries, set(), entries[1:], guards):
        faults.append("a new entry was not reached")
    if reached(entries, {"no/such/file"}, entries, guards):
        faults.append("a change to a file no entry names reached entries anyway")
    everything = list(range(len(entries)))
    for count in (1, 3, MOST_SHARDS):
        parts = [shard(everything, i, count) for i in range(count)]
        if sorted(p for part in parts for p in part) != everything:
            faults.append(f"cut into {count}, the parts are not the whole list exactly once")
    return faults
