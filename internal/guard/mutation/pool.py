"""Worker trees and a global memory budget, so entries can run side by side.

Why this exists, and it is a measurement rather than a preference. A full run
on 2026-09-01 took 74.0 min for 691 entries, and CLAUDE.md explained it with
"every mutation invalidates the cache, so guard.test links again". Measured
2026-09-01 with a -run pattern that matches no test - which builds, links and
starts the binary while running nothing, so the guard's own time is removed:

    go test, nothing changed at all      3.41 s   (3.25-3.55, 4 rounds)
    the prebuilt binary, run directly    0.13 s   (0.12-0.17)
    go test after touching a source file 3.80 s   (3.28-4.93)

The relink costs 0.39 s. The go command spends 3.28 s deciding it has nothing
to do, and it spends it on every entry including the 69 that mutate no Go file
at all - 691 x 3.28 s is 37.8 min of the 74. So the minutes are not in the
compiler, and making the compiler's job smaller was never going to pay.

What does pay is running entries at the same time. Measured the same day, the
same command K at a time, widths 1/2/4/6/8 interleaved over 3 rounds:

    per entry   3.33  1.83  1.36  1.15  1.15 s

That is 2.90x, saturating at width 6 - width 8 buys nothing, on a machine with
16 cores, so the limit is filesystem metadata rather than the processor. Six is
therefore the default, and asking for more is allowed but will not help.

Entries cannot share a tree: each one puts a mutant on disk and takes it off
again, and two doing that at once would judge each other's damage. So every
worker gets its own copy. Measured: 3.55 s and 82.8 MiB per copy, and six of
them cost 21 s of setup and 0.48 GiB, against a run that saves half an hour.

The copy carries .git, and that is not tidiness. Six entries name guards that
read tracked files, commit messages and branch names. Those guards call
t.Skipf when git cannot answer, so a tree without .git does not produce a
false verdict - it produces UNJUDGED, and the run would quietly prove six
entries less than the sequential one it replaces. Verified rather than
assumed: with .git copied, TestNoCommitMessageCarriesPrivateContent PASSES in
a worker tree.

What the copy leaves out is tfg-out (251 MB of generated files) and dist
(77 MB of release artefacts). Neither is an input to any build.

One thing this buys that has nothing to do with speed: the real tree is never
mutated. On 2026-08-26 a run that filled the disk left a mutant in the working
copy and the only thing that noticed was git status. A worker tree is
disposable, so the same failure now costs a re-copy instead of a hand repair.
"""
import os
import shutil
import stat
import tempfile
import time
from pathlib import Path

# The root of the repository, found from where this file sits rather than
# written down. This file is internal/guard/mutation/pool.py, three levels
# below it. Until 2026-10-15 it was an absolute path on one Windows machine.
ROOT = Path(__file__).resolve().parents[3]

# Not inputs to any build. tfg-out alone is 251 MB and would be copied per
# worker for nothing.
NOT_COPIED = {"tfg-out", "dist", "__pycache__", ".pytest_cache"}

# Lock files are excluded because a copy taken while git is mid-operation
# otherwise carries an index.lock and the copy looks permanently busy.
NOT_COPIED_SUFFIX = (".exe", ".lock")

# What worker trees were called before 2026-10-15, a fresh name per run. Still
# swept, so a machine that ran the old runner gets its space back.
PREFIX = "tfg-mutate-tree-"

# Where worker trees live: one fixed directory, the same on every run, and that
# is the speed of a run rather than tidiness. Go keys its build cache on the
# directory a package sits in, so a tree under a new name builds every package
# of this module from nothing. Measured 2026-10-15 on the guard binary: 409.7 s
# in a fresh copy, 4.8 s for the same copy built again, 4.0 s in the checkout
# itself. Under a fresh name per run, the first entry of every worker paid that
# cold build inside its own time ceiling. A fixed path lets the next run find
# what the last one built.
TREES = "tfg-mutate-trees"

# Measured 2026-09-01: 82.8 MiB per copy. Rounded up, because this feeds the
# disk check and erring high refuses a run that would have fitted while erring
# low lets through one that cannot finish.
TREE_BYTES = 90 * 1024**2

# Where the speedup stops. Measured, not chosen: width 8 costs the same per
# entry as width 6 on a 16 core machine.
DEFAULT_WORKERS = 6


def _force_writable(func, path, _exc):
    """Retry a delete after clearing the read-only bit.

    Git object files are read-only on Windows, so rmtree over a copied .git
    fails half way and leaves most of the tree behind. Without this the trees
    accumulate in TEMP exactly like the 28 orphaned go-build directories that
    were found there on 2026-08-26.
    """
    try:
        os.chmod(path, stat.S_IWRITE)
        func(path)
    except OSError:
        pass


def _ignore(_dirpath: str, names: list[str]) -> set[str]:
    out = {n for n in names if n in NOT_COPIED}
    out |= {n for n in names if n.endswith(NOT_COPIED_SUFFIX)}
    return out


def sweep_old_trees() -> int:
    """Delete worker trees an earlier run left behind, and say how many.

    Runs before anything is created. A crashed run cannot clean up after
    itself, and 83 MiB a time is how TEMP quietly fills - this project has
    already measured 1.4 GB of orphaned go-build directories sitting there for
    a week with nothing to remove them.
    """
    parent = Path(tempfile.gettempdir())
    gone = 0
    try:
        for path in [*parent.glob(PREFIX + "*"), parent / TREES]:
            if path.is_dir():
                shutil.rmtree(path, onerror=_force_writable)
                gone += 1
    except OSError:
        pass
    return gone


class WorkerTree:
    """One disposable copy of the repository, and the bytes it started with.

    Baselines are kept per tree rather than globally: the failure they catch is
    an entry inheriting the damage of the one before it, and after this change
    "the one before it" means the previous entry ON THIS WORKER.
    """

    def __init__(self, slot: int, path: Path) -> None:
        self.slot = slot
        self.path = path
        self._first: dict[Path, bytes] = {}

    def file(self, rel: str) -> Path:
        return self.path / rel

    def read(self, rel: str) -> bytes:
        """The file's bytes now, refusing if they moved since this tree was made."""
        path = self.file(rel)
        current = path.read_bytes()
        known = self._first.setdefault(path, current)
        if current != known:
            raise TreeDamaged(
                f"{rel}: changed between entries on worker {self.slot}, so the "
                f"entry before this one did not put it back")
        return current

    def moved(self) -> list[str]:
        """Every touched file that no longer holds what it started with."""
        out = []
        for path, want in self._first.items():
            try:
                if path.read_bytes() != want:
                    out.append(str(path.relative_to(self.path)))
            except OSError:
                out.append(str(path))
        return out

    def rebuild(self) -> None:
        """Throw this copy away and make it again.

        The whole reason a damaged worker is survivable. In the sequential
        runner a bad restore ended the run, because every later entry would
        read the damaged file and take it for its own original. Here the
        damage cannot leave the copy, so the answer is a fresh copy.
        """
        shutil.rmtree(self.path, onerror=_force_writable)
        shutil.copytree(ROOT, self.path, ignore=_ignore, symlinks=True)
        self._first.clear()


class TreeDamaged(Exception):
    """A worker tree holds something other than what it started with.

    Separate from runner.TreeCorrupted, and the difference is what it costs.
    TreeCorrupted is about the REAL tree and ends the run, because somebody has
    to put a file back by hand. This one is about a copy nobody will ever
    commit, so it is recoverable by rebuilding that copy.
    """


def make_trees(count: int, say=print) -> list[WorkerTree]:
    """Copy the tree count times, and say what that cost.

    Said out loud because it is dead time before the first verdict, and a
    setup step that takes twenty seconds without explaining itself reads as a
    hang.
    """
    swept = sweep_old_trees()
    if swept:
        say(f"  swept {swept} worker tree(s) left by an earlier run")

    started = time.perf_counter()
    # Copied fresh every run, so nothing an earlier run left in a tree can
    # reach this one - only the path is the same, which is all the cache needs.
    parent = Path(tempfile.gettempdir()) / TREES
    parent.mkdir()
    trees = []
    for slot in range(count):
        path = parent / f"w{slot}"
        shutil.copytree(ROOT, path, ignore=_ignore, symlinks=True)
        trees.append(WorkerTree(slot, path))
    say(f"  {count} worker trees in {time.perf_counter() - started:.1f} s under {parent}")
    return trees


def drop_trees(trees: list[WorkerTree]) -> None:
    """Remove the trees and the directory holding them.

    Never raises. This runs in a finally, and a failure to tidy up must not
    replace whatever the run was actually reporting - a traceback here would
    bury the verdicts.
    """
    if not trees:
        return
    parent = trees[0].path.parent
    for tree in trees:
        try:
            shutil.rmtree(tree.path, onerror=_force_writable)
        except OSError:
            pass
    try:
        shutil.rmtree(parent, onerror=_force_writable)
    except OSError:
        pass
