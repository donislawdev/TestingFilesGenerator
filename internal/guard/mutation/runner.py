"""One way to run a mutated tree, shared by every mutation tool.

Why it exists. mutate.py, subset.py and try-one.py each held their own copy of
"swap the file, run the guard, decide what happened", and the copies had already
drifted: the two smaller ones still ran a separate "go vet ./..." that the full
runner dropped, and none of them knew about a guard that skips itself. On
2026-08-11 the full runner gained a memory ceiling after the entry that switches
off core.MaxFilesPerRun took the owner's machine down - guard.test reached
19.8 GB of 34.3 GB, twice. The other two would still have done it, and the fact
that the main one was safe by then made that easy to assume away.

So the deciding lives here and the tools only choose what to feed it.

The same argument came back on 2026-08-26 for the swap rather than the run. All
three tools held their own copy of "put the mutant down, put the original
back", and none of the three checked that the second half worked. Two runs that
day left a mutated file in the tree and the only thing that noticed either was
git status. That code now lives here as well, and it proves the restore instead
of assuming it.
"""
import contextlib
import hashlib
import json
import os
import shutil
import subprocess
import threading
import time
from collections.abc import Iterator
from pathlib import Path
from typing import NamedTuple

# psutil is used to watch the memory of the process tree and nothing else. It
# is installed from .github/requirements-mutation.txt, outside the module and
# outside both binaries, so it is not a dependency of the product and go.mod
# never sees it.
import psutil

# Worker trees are pool's business, but the disk check below has to account
# for them, so the measured size comes from there rather than being written
# down twice. pool imports nothing from here, so this cannot cycle.
from pool import ROOT, TREE_BYTES

# The go command on PATH. Until 2026-10-15 this was an absolute path on the one
# Windows machine the runner had ever run on, and CI runs it on Linux. Empty
# when there is none, and require_go says so before anything is copied.
GO = shutil.which("go") or ""


def require_go() -> str:
    """Why no entry can run here, or an empty string when go was found."""
    if GO:
        return ""
    return "the go command is not on PATH, so no entry can be built or run"

# The tags the project ships with, read from the same file the workflows read.
#
# Without them this runner builds a DIFFERENT binary than CI and the release do,
# and at least one guard would then run against an encoder whose assembly reads
# past its buffers - which does not fail a test, it takes the process down, and
# a crash is scored as a mutation that was caught.
BUILD_TAGS = (ROOT / ".github" / "build-tags").read_text(encoding="utf-8").strip()

# A mutation that removes a refusal makes the code do the thing the refusal
# prevented, and one of those things is "take all the memory on the machine".
#
# Measured 2026-08-11, and not by reading: the entry that switches off
# core.MaxFilesPerRun feeds the reader a count of 9223372036854775807, so it
# grows a list towards nine quintillion entries. The test asserts on "out of
# memory" because that was the original defect, so the mutation cannot be made
# cheap without making it prove something else.
#
# Both ceilings earn their place, measured separately: the count entry is
# stopped by memory at 4.5 GB after 4.92 s, and the markdown entry never nears
# 4 GB and is stopped by time. Neither ceiling alone catches both. An ordinary
# entry takes 1.64 s and touches neither.
MEMORY_CAP_BYTES = 4 * 1024**3
TIME_CAP_SECONDS = 120

# Disk, alongside memory and time, and it earns its place the same way they
# did: by a run that took the machine somewhere it could not come back from.
#
# Measured 2026-08-26 by walking GOCACHE before and after, and it has to be
# done that way. Free space on this machine is not a measurement over minutes:
# it moved by gigabytes in both directions during one session, and a 79 entry
# run showed 7250 MB gone that turned up in neither the cache nor TEMP. Two
# earlier readings taken with df alone said 12.5 and 13.4 MB per entry, and
# they happened to land near the truth rather than being evidence of it.
#
# What the cache actually says, on two fresh subsets:
#   internal/engine    47 entries   7.97 -> 8.49 GB   11.3 MB per entry
#   internal/manifest  13 entries   no change        about nothing
# df agreed with the first to within 1%, which is what makes it trustworthy.
#
# So the cost DOES follow how much sits above the mutated package, and an
# earlier version of this comment said the opposite off the df readings alone.
# 14 MB is the conservative end of that range rather than an average - this is
# a safety check, so erring high refuses a run that would have fitted, and
# erring low lets through one that cannot finish.
#
# The build cache evicts nothing while a run is going, so this accumulates for
# the whole pass. Five hundred and ten entries is therefore about 7 GB at
# worst, and the 27 GB that filled the disk was weeks of accumulated cache
# rather than what one run adds.
CACHE_BYTES_PER_ENTRY = 14 * 1024**2

# What has to stay free. One entry needs about 63 MB at its peak: 49.8 MB for
# the linked guard.test, measured the same day, plus its cache entry. Two
# gigabytes is thirty of those, and it is deliberately far above one entry -
# the check runs between entries, so a floor a single entry could cross would
# report the damage rather than prevent it.
#
# Overridable, and that is not a convenience. A ceiling nobody can reach on
# purpose is a ceiling nobody has watched work, and this project has thrown
# away six pieces of defensive code for that exact reason. Setting the variable
# to more than the disk holds makes the refusal happen on the next run, which
# is how both paths below were seen to fire at all.
DISK_FLOOR_BYTES = int(os.environ.get("TFG_MUTATE_DISK_FLOOR", 2 * 1024**3))


class Result(NamedTuple):
    """What one mutated run produced.

    capped is empty unless the run was stopped for holding too much memory or
    running too long. A capped run is not a result: the guard was not allowed
    to finish, so it neither caught the mutation nor failed to. Reporting it as
    caught would be true by accident, because a killed run does end non-zero.
    """
    compiled: bool
    passed: bool
    seconds: float
    capped: str
    output: str


def time_cap_for(workers: int) -> float:
    """The wall clock ceiling for one entry, given how many run at once.

    A flat ceiling was right while entries ran alone and is wrong now. The
    ceiling exists to stop a mutation that removed a refusal from doing the
    thing the refusal prevented for ever - it is about the ENTRY running away,
    not about the machine being busy. But it is measured in wall clock, and
    wall clock stretches with contention.

    Measured 2026-09-01 over 48 entries in list order, same block each time:
    the entry itself has a median of 4.97-6.76 s at width 1, 7.22 s at width 3,
    12.43 s at width 6 and 21.78 s at width 9. So six workers roughly double an
    entry's wall clock while the work inside it is unchanged.

    Left flat, an entry that took 50 s alone would cross a 120 s ceiling at
    width 6 and come back CAPPED - which is not a verdict about the mutation at
    all, it is a verdict about how many workers the machine happened to be
    running. That would make the proof depend on the width, and a run at
    width 6 would quietly prove less than the same run at width 1.

    So the ceiling stretches the way the measurement says the entries do:
    unchanged at width 1, doubled at width 6.
    """
    return TIME_CAP_SECONDS * (1 + (max(1, workers) - 1) / 5)


def run_test(name: str, tree: Path = ROOT, budget: "Budget | None" = None,
             slot: int = 0, deadline: float = TIME_CAP_SECONDS) -> Result:
    """Run the named guard against whatever is currently on disk.

    tree says WHERE. It defaults to the real repository, which is what the
    three smaller tools still pass, and the parallel runner hands over a
    worker copy instead - two entries sharing a tree would judge each other's
    mutants.

    budget is the ceiling across ALL workers, and it is the piece that made
    width safe rather than reckless. The per-entry ceilings below are
    unchanged and still bind on their own, but six of them side by side would
    permit 24 GB on a machine that had 8.0 GB available when this was written.
    Measured the same day: an ordinary entry peaks at about 650 MiB, so six of
    those are 3.9 GB and fit easily - the danger is the one entry that
    legitimately climbs to 4 GB while five ordinary ones are already holding
    3.2 GB. That sum, not the ordinary case, is what this stands in front of.

    The two failure shapes have to be told apart. A test run reports failure
    for a mutation that broke the behaviour AND for one that broke the syntax,
    and the exit code alone cannot separate them - so a mutation that does not
    compile would read as caught while proving nothing.

    There is deliberately no separate "go vet ./..." here. It compiles the
    whole tree a second time for every entry, and a mutation invalidates the
    build cache, so neither pass is free. Go already says [build failed] in its
    own output and reading that costs nothing.
    """
    started = time.perf_counter()
    proc = psutil.Popen(
        [GO, "test", "-tags", BUILD_TAGS, "./internal/guard/", "-run", "^" + name + "$", "-count=1"],
        cwd=tree, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
    )
    capped = ""
    while True:
        try:
            # Retrying communicate after a timeout does not lose output, which
            # is what makes it safe to poll in this loop.
            out, err = proc.communicate(timeout=0.25)
            break
        except subprocess.TimeoutExpired:
            used = tree_memory(proc)
            # Told to the shared budget before the private ceiling is
            # consulted, so the sum is current even for entries that never
            # come near 4 GB themselves. A worker that is not the largest
            # gets an empty answer and carries on.
            shared = budget.note(slot, used) if budget is not None else ""
            if used > MEMORY_CAP_BYTES:
                capped = f"memory, {used / 1024**3:.1f} GB"
            elif shared:
                capped = shared
            elif time.perf_counter() - started > deadline:
                capped = f"time, over {deadline:.0f} s"
            if capped:
                kill_tree(proc)
                out, err = proc.communicate()
                break

    elapsed = time.perf_counter() - started
    # Every break above lands here, so one release covers them all. A slot
    # left registered would count a finished entry's memory against the ones
    # still running and cap them for it.
    if budget is not None:
        budget.release(slot)
    output = out + err
    if capped:
        return Result(True, False, elapsed, capped, output)
    if "[build failed]" in output or "setup failed" in output:
        return Result(False, False, elapsed, "", output)
    return Result(True, proc.returncode == 0, elapsed, "", output)


def tree_memory(proc: psutil.Popen) -> int:
    """Bytes held by the process and everything under it.

    The tree matters rather than the process: go test is a launcher and the
    memory is in guard.test, its child. Watching only what was spawned here
    would report a few megabytes while the machine went under.
    """
    total = 0
    try:
        total = proc.memory_info().rss
        for child in proc.children(recursive=True):
            try:
                total += child.memory_info().rss
            except psutil.Error:
                pass
    except psutil.Error:
        pass
    return total


def kill_tree(proc: psutil.Popen) -> None:
    """Kill the children first, then the launcher.

    The other order leaves guard.test holding its memory with nothing waiting
    on it, which is the state this ceiling exists to avoid.
    """
    try:
        for child in proc.children(recursive=True):
            try:
                child.kill()
            except psutil.Error:
                pass
        proc.kill()
    except psutil.Error:
        pass


def unjudged_reason(name: str, tree: Path = ROOT) -> str:
    """Why the named guard said nothing here, or an empty string when it ran.

    Asked only after a guard stayed green, so the normal path pays nothing for
    it. A skipped test exits 0 exactly like a passing one, and without this a
    runner reports "the guard does not guard this" about a guard that never
    ran - which is the same lie as counting a tree that does not compile as
    caught. It needs -v, because go test says nothing about a skip without it.

    The second answer came with CI on 2026-10-15. A guard in a file only one
    platform builds does not exist on the others, and go test then runs
    nothing at all and exits 0 - which read as a hole in a healthy guard.
    """
    r = subprocess.run(
        [GO, "test", "-tags", BUILD_TAGS, "./internal/guard/", "-run", "^" + name + "$", "-count=1", "-v"],
        cwd=tree, capture_output=True, text=True,
    )
    output = r.stdout + r.stderr
    if "--- SKIP:" in output:
        return f"{name} skipped itself here"
    if "=== RUN   " + name + "\n" not in output:
        return f"{name} does not exist on this platform"
    return ""


def was_skipped(name: str, tree: Path = ROOT) -> bool:
    """True when the named guard did not run here. Kept for the smaller tools."""
    return unjudged_reason(name, tree) != ""


def unbuilt_files(tree: Path = ROOT) -> set[str]:
    """The Go files this platform leaves out of the build, relative to the root.

    A mutation in a file the compiler never reads changes nothing, so its
    guard stays green and the entry would read as a hole. Measured on
    2026-10-15: six entries mutate files only Windows builds, and CI on Linux
    builds without cgo, which leaves out eight more. The answer comes from go
    itself, asked once per run, with the same tags and environment the entries
    are run under, so it follows the platform rather than a list kept by hand.
    """
    r = subprocess.run(
        [GO, "list", "-tags", BUILD_TAGS, "-e", "-json=Dir,IgnoredGoFiles", "./..."],
        cwd=tree, capture_output=True, text=True,
    )
    if r.returncode != 0:
        raise SystemExit(f"go list could not say which files are built here:\n{r.stderr}")
    found: set[str] = set()
    decoder = json.JSONDecoder()
    text, at = r.stdout, 0
    while True:
        while at < len(text) and text[at].isspace():
            at += 1
        if at >= len(text):
            break
        pkg, at = decoder.raw_decode(text, at)
        base = Path(pkg["Dir"]).relative_to(tree.resolve())
        for name in pkg.get("IgnoredGoFiles") or []:
            found.add((base / name).as_posix())
    return found


class TreeCorrupted(Exception):
    """The working tree holds something other than what the run started with.

    Raised rather than returned, because there is no useful next entry. The
    following one reads the damaged file, takes it for its own original and
    restores to it, so one bad restore becomes a mutation that outlives the
    run. That is not a worry, it is what happened on 2026-08-26: the entry
    after the damaged one counted its pattern, found it zero times and reported
    SKIP, which reads as a stale pattern rather than as a wrecked tree.
    """


def write_checked(path: Path, want: bytes, what: str) -> None:
    """Put bytes on disk, then read them back to prove they are there.

    Bytes rather than text, because writing text on Windows turns every LF into
    CRLF and the repository normalises to LF.

    The read back is the point. write_bytes truncates before it writes, so a
    failure part way through leaves the file shorter than it was rather than
    unchanged - which is worse than not having tried. On 2026-08-26 that
    failure was ENOSPC inside the restore, and the run answered with a
    traceback and a mutated file left in the tree.
    """
    try:
        path.write_bytes(want)
    except OSError as err:
        raise TreeCorrupted(f"{path}: {what} could not be written - {err}") from err
    got = path.read_bytes()
    if got != want:
        raise TreeCorrupted(
            f"{path}: {what} was written but the file holds {len(got)} bytes "
            f"where {len(want)} were asked for")


@contextlib.contextmanager
def mutation_applied(path: Path, original: bytes, mutant: bytes) -> Iterator[None]:
    """Hold the mutant on disk for the length of the block, then put the original back.

    The restore sits in a finally, so it runs when the block raised and also
    when writing the mutant itself failed half way. Both directions go through
    write_checked: a restore that is assumed rather than proven is the hole
    this exists to close, and the mutant is written the same way because a
    half written mutant would be judged as if somebody had meant it.
    """
    try:
        write_checked(path, mutant, "the mutation")
        yield
    finally:
        write_checked(path, original, "the original")


class Baselines:
    """What every mutated file held when the run started.

    Kept per file rather than per entry, because the failure it catches is the
    second entry to touch a file and not the first. On 2026-08-26 subset.py
    copied a file a previous entry had left mutated, took that copy for the
    original and restored to it. Nothing in the run said so.

    This also answers a case nobody has explained. That day a run left
    internal/format/registry.go without its sort call at 24 GB free, so the
    disk was not the reason, and the same subset run again was clean. A record
    of the starting bytes does not need to know why a file moved.
    """

    def __init__(self) -> None:
        self._first: dict[Path, bytes] = {}

    def read(self, path: Path) -> bytes:
        """The file's bytes now, refusing if they moved since the run started."""
        current = path.read_bytes()
        known = self._first.setdefault(path, current)
        if current != known:
            raise TreeCorrupted(
                f"{path}: the file changed between entries, so the entry "
                f"before this one did not put it back")
        return current

    def moved(self) -> list[Path]:
        """Every touched file that no longer holds what it started with.

        Asked once at the end. Without it the last entry of a run is the one
        entry whose restore nothing checks afterwards.
        """
        return [p for p, want in self._first.items() if p.read_bytes() != want]


class Budget:
    """One memory ceiling shared by every worker.

    Why a shared one is not the same as six private ones. The private ceiling
    is 4 GB because ONE entry needs that much - the one that switches off the
    file count limit and grows a list towards nine quintillion. Multiplying it
    by the worker count says six workers may hold 24 GB, and this machine has
    31.9 GB total with 8.0 GB available while ordinary work is open. It went
    under twice in 2026-08-11 at 19.8 GB in a single process.

    So the number here is taken from what the machine HAS when the run starts,
    not from the per-entry ceiling and not from total RAM. It is never allowed
    below the per-entry ceiling plus room for the others: a global ceiling
    under 4 GB would stop the pathological entry before it reached its own,
    turning a documented memory cap into an accident of how busy the machine
    happened to be.

    The largest holder is the one told to stop. Capping the smallest would
    free almost nothing and would punish an entry for somebody else's
    appetite, and capping everybody would throw away five verdicts to protect
    against one.
    """

    def __init__(self, ceiling: int) -> None:
        self.ceiling = ceiling
        self._held: dict[int, int] = {}
        self._lock = threading.Lock()
        self.trips = 0

    def note(self, slot: int, used: int) -> str:
        """Record this worker's usage, and say whether IT is the one to stop."""
        with self._lock:
            self._held[slot] = used
            total = sum(self._held.values())
            if total <= self.ceiling:
                return ""
            worst = max(self._held, key=lambda s: self._held[s])
            if worst != slot:
                return ""
            self.trips += 1
            return (f"shared memory, {total / 1024**3:.1f} GB across "
                    f"{len(self._held)} workers over the "
                    f"{self.ceiling / 1024**3:.1f} GB ceiling")

    def release(self, slot: int) -> None:
        with self._lock:
            self._held.pop(slot, None)


def measured_ceiling(workers: int) -> int:
    """What all workers together may hold, from what the machine has now.

    Reserve is deliberate: the owner works on this machine while runs go, and
    a ceiling that consumes everything available makes the run correct and the
    desktop unusable. The floor keeps the pathological entry able to reach its
    own 4 GB ceiling with ordinary workers alongside it.
    """
    available = psutil.virtual_memory().available
    reserve = 2 * 1024**3
    floor = MEMORY_CAP_BYTES + max(0, workers - 1) * 700 * 1024**2
    return max(floor, available - reserve)


def free_bytes() -> int:
    """Bytes free on the volume the tree lives on."""
    return shutil.disk_usage(ROOT).free


def no_room_for(entries: int, workers: int = 1) -> str:
    """Why this run will not fit on the disk, or an empty string when it will.

    Asked before the first entry. A run that dies at 459 of 510 has spent forty
    minutes to produce a log whose last forty nine lines all read BROKEN, and
    the arithmetic that would have refused it up front costs one syscall.
    """
    # Worker trees are counted too. Measured 2026-09-01: 82.8 MiB per copy
    # with .git, rounded up to 90 in pool.TREE_BYTES. Six of them is half a
    # gigabyte that the sequential runner never had to find.
    trees = workers * TREE_BYTES
    need = entries * CACHE_BYTES_PER_ENTRY + DISK_FLOOR_BYTES + trees
    free = free_bytes()
    if free >= need:
        return ""
    return (f"{entries} entries need about {need / 1024**3:.1f} GB - "
            f"{CACHE_BYTES_PER_ENTRY / 1024**2:.0f} MB each as measured, plus the "
            f"{DISK_FLOOR_BYTES / 1024**3:.0f} GB floor and {trees / 1024**2:.0f} MB "
            f"of worker trees - and {free / 1024**3:.1f} GB is free.\n"
            f"  go clean -cache gives back what earlier runs left behind.")


# Where proofs are written down, and only on a machine that keeps them. tools/
# is a local directory outside the repository, so a CI run records nothing and
# says so once rather than failing every write in silence.
PROOFS = ROOT / "tools" / "mutate" / "proven.json"


def proofs_recorded() -> bool:
    """True when this checkout has somewhere to write proofs down."""
    return PROOFS.parent.is_dir()


def content_id(path: Path) -> str:
    """Sixteen hex characters of the file's bytes, enough to tell versions apart."""
    return hashlib.sha256(path.read_bytes()).hexdigest()[:16]


def record_proof(label: str, path: Path) -> None:
    """Writes down that this entry was proven AGAINST THIS CONTENT of its file.

    O127, and it is the one variant of the mutation trap that nothing cheap
    catches. An entry can find its pattern, compile, and still prove nothing,
    because the text it breaks no longer reaches the assertion: measured on
    2026-08-25, an entry appended a rule to a hint that had been moved behind an
    information button the day before, and the guard counts VISIBLE text. The
    first variant - no entry at all - is caught by the coverage guard. The
    second - a pattern that no longer occurs - is caught by staleness.py in no
    time at all. This third one is invisible to both, and only running the
    mutation shows it.

    So what is written down is the thing that decides: the content of the file
    the entry mutates. When that content changes, the proof is about a version
    of the code that is gone, and aging.py says so. It is a REPORT rather than a
    gate, because a changed file does not mean the proof is void - it means
    nobody has checked since, which is a different sentence and the honest one.

    Failing to write this must never fail a run. The proof already happened.
    """
    if not proofs_recorded():
        return
    try:
        proofs = json.loads(PROOFS.read_text(encoding="utf-8")) if PROOFS.exists() else {}
        proofs[label] = content_id(path)
        PROOFS.write_text(json.dumps(proofs, indent=1, sort_keys=True, ensure_ascii=False),
                          encoding="utf-8", newline="\n")
    except (OSError, ValueError):
        pass


def below_floor() -> str:
    """Why the run has to stop here, or an empty string when it may go on.

    Checked between entries rather than only at the start, because the estimate
    above is this machine's number and anything else on the disk is free to
    move while a run is going.
    """
    free = free_bytes()
    if free >= DISK_FLOOR_BYTES:
        return ""
    return (f"{free / 1024**3:.2f} GB free, under the "
            f"{DISK_FLOOR_BYTES / 1024**3:.0f} GB floor")
