"""Mutation testing for the guards.

A green suite is not evidence. The evidence is the sentence "this test would be
red if I broke that", and the only way to get it is to break it on purpose.

Each entry in entries.py breaks one behaviour and names the guard that has to
notice. This runs them, each in a disposable copy of the repository.

    python internal/guard/mutation/mutate.py                  every entry
    python internal/guard/mutation/mutate.py --changed BASE   the entries a change reaches
    python internal/guard/mutation/mutate.py --shard 2/16     the third of sixteen parts
    python internal/guard/mutation/mutate.py --plan           how many entries and parts, nothing run
    python internal/guard/mutation/mutate.py --json PATH      every verdict written to PATH as well

CI runs it from .github/workflows/mutation.yml, every entry once a week and
the entries a pull request reaches on that pull request.
"""
import argparse
import collections
import json
import queue
import statistics
import sys
import threading
import time
from pathlib import Path

from entries import MUTATIONS
from pool import DEFAULT_WORKERS, TreeDamaged, drop_trees, make_trees
from runner import (
    record_proof, proofs_recorded, require_go, unbuilt_files, unjudged_reason,
    ROOT, Budget, TreeCorrupted, below_floor, free_bytes, measured_ceiling,
    mutation_applied, no_room_for, run_test, time_cap_for,
)
from selection import (
    canary as selection_canary, changed_files, entries_at, guard_files, reached,
    shard, shard_count,
)


# The canary. A mutation that deliberately does not compile, so the detection
# of that case is exercised on every run rather than only on the day somebody
# writes a broken mutation by accident.
#
# Without it the BROKEN path is code nobody proves. That is the same gap this
# whole script exists to close, one level further in.
CANARY = (
    "canary: a mutation that cannot compile must report BROKEN",
    "internal/core/label.go",
    "func Label(formatID string, bytes int64, seed uint64) string {",
    "func Label(formatID string, bytes int64, seed uint64) string {\n\tvar orphan int",
    "TestOurOwnGeneratorsHaveNotDrifted",
)


# How long building the guard binary in an unmutated tree may take. Generous,
# because this is the cold build an entry must not pay for - 409.7 s on the
# owner's machine with cgo - and what it guards against is a build that hangs.
WARM_CAP_SECONDS = 1800


def warm(trees) -> str:
    """Build the guard binary in every tree before anything is timed.

    Why, measured on 2026-10-15: the first build in a fresh tree took 409.7 s
    and the second 4.8 s. The time ceiling exists to stop an entry that runs
    away, and without this the first entry on every worker carried a cold
    build of the whole module inside its own ceiling - and a CI machine starts
    every run cold. All trees build at once, and the answer is the reason the
    run cannot go on, or an empty string.

    It also answers a question nothing asked before: whether the tree builds
    at all with no mutation in it. If it does not, every entry would come back
    BROKEN and the run would say so seventeen hundred times.
    """
    faults: list[str] = []

    def one(tree) -> None:
        result = run_test("", tree.path, None, tree.slot, WARM_CAP_SECONDS)
        if result.capped:
            faults.append(f"worker {tree.slot}: building took too long ({result.capped})")
        elif not result.compiled:
            faults.append(f"worker {tree.slot}: the tree does not build with no mutation in it\n"
                          f"{result.output[-2000:]}")

    started = time.perf_counter()
    threads = [threading.Thread(target=one, args=(tree,), daemon=True) for tree in trees]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    print(f"  {len(trees)} worker tree(s) built in {time.perf_counter() - started:.1f} s, "
          f"before anything is timed")
    return "\n".join(faults)


def check_canary_in(tree) -> bool:
    """True when a non compiling mutation is correctly reported as BROKEN.

    Runs in a worker tree like every other entry, so the real repository is
    never mutated - not even by the one entry that is meant to leave the tree
    unable to compile.
    """
    label, rel, old, new, test = CANARY
    path = tree.file(rel)
    original = path.read_bytes()
    if old.encode() not in original:
        print(f"SKIP  {label}\n      pattern not found in {rel} - the canary is stale")
        return False

    with mutation_applied(path, original, original.replace(old.encode(), new.encode(), 1)):
        compiled = run_test(test, tree.path).compiled

    if compiled:
        print(f"HOLE  {label}\n      a tree that does not compile was reported as compiling, "
              f"so every BROKEN below is unreliable")
        return False
    print(f"ok    {label}\n      the broken tree was detected")
    return True


def report_time(timings: list[tuple[float, str, str, str]], wall: float) -> None:
    """Say where the minutes went, per entry and per mutated file.

    The run takes about three times what the measured cycle predicts, and that
    gap has been open since 2026-08-10. A single median cannot close it: the
    cost of one entry is the cost of rebuilding whatever sits above the file it
    touches, so a mutation in internal/core rebuilds most of the tree and one
    in internal/gui/window rebuilds almost none. Grouping by file is what makes
    that visible, and it is measured here rather than reasoned about.
    """
    if not timings:
        return
    seconds = [t for t, _, _, _ in timings]
    inside = sum(seconds)
    # Wall and sum are no longer the same quantity, and the old arithmetic
    # became a false statement the moment entries started overlapping. It
    # printed "inside go test 165.8 min (586% of wall)" and then "everything
    # else -137.5 min" on the first parallel run: a negative amount of time,
    # which is this report lying about the runner rather than measuring it.
    #
    # So what is printed is what the two numbers actually mean now. Their
    # ratio is the concurrency the run ACHIEVED, which is worth more than the
    # width that was asked for - it says how much of the width the machine
    # gave back, and it is the number to watch when choosing --workers.
    #
    # "Everything else" is gone rather than fixed. Python's share was 2.7 s of
    # a whole sequential pass, so it was never the interesting half, and in a
    # parallel run it cannot be derived by subtraction at all.
    print("\nwhere the time went")
    print(f"  wall clock            {wall / 60:6.1f} min")
    print(f"  summed over entries   {inside / 60:6.1f} min")
    print(f"  concurrency achieved  {inside / wall:6.2f}x")
    print(f"  entries               {len(seconds)}")
    print(f"  per entry: median {statistics.median(seconds):5.2f}s   "
          f"mean {statistics.fmean(seconds):5.2f}s   "
          f"min {min(seconds):5.2f}s   max {max(seconds):5.2f}s")

    print("\n  slowest 15 entries")
    for secs, label, rel, test in sorted(timings, reverse=True)[:15]:
        print(f"    {secs:6.2f}s  {rel:<42} {label[:44]}")

    by_file: dict[str, list[float]] = collections.defaultdict(list)
    for secs, _, rel, _ in timings:
        by_file[rel].append(secs)
    print("\n  by mutated file, costliest first")
    ranked = sorted(by_file.items(), key=lambda kv: -sum(kv[1]))
    for rel, secs in ranked[:20]:
        print(f"    {sum(secs) / 60:5.1f} min  {len(secs):3d} entries  "
              f"median {statistics.median(secs):5.2f}s  {rel}")


# say_wrecked lived here and is gone with the sequential loop. It told the
# reader which file to put back with git checkout, and there is no longer a
# file to put back: entries mutate a worker copy, so the failure it reported -
# a mutant left in the working tree, which happened on 2026-08-26 and was
# noticed only by git status - cannot reach the repository any more. A damaged
# worker is reported as DAMAGED, counted as proving nothing, and its tree is
# rebuilt. Exit code 3 went with it and was documented nowhere.


class Outcome:
    """What one entry produced, kept apart from how it is reported.

    A named thing rather than a tuple because six verdicts each carry
    different extras, and the summary below has to sort them by entry order -
    results arrive in the order workers finish, which is not the order they
    were submitted.
    """

    def __init__(self, index: int, verdict: str, label: str, rel: str,
                 test: str, secs: float, detail: str = "") -> None:
        self.index = index
        self.verdict = verdict
        self.label = label
        self.rel = rel
        self.test = test
        self.secs = secs
        self.detail = detail


# The verdict vocabulary is unchanged, and deliberately so. These tokens are
# what a person greps for after a long run - the session brief for 2026-09-01
# records a whole pass reported as "nothing worrying" because the reader
# grepped for NOT CAUGHT while the runner writes HOLE. Renaming any of them
# here would repeat that on the next reader.
def judge(index: int, entry, tree, budget, deadline: float,
          unbuilt: frozenset[str] = frozenset()) -> Outcome:
    """Run one entry in one worker tree and say what happened.

    Runs on the worker's own thread. Everything it touches belongs to that
    worker: its tree, its slot in the budget, its own copy of the file. The
    main thread does the printing and the proof recording, so neither needs a
    lock.

    Two answers come before anything is mutated, and both are UNJUDGED rather
    than a verdict. A file that is not in this tree - docs/ is not in the
    repository, so a CI checkout has none of it - cannot be mutated here. A Go
    file this platform does not build cannot change what any guard sees, so
    its guard stays green and the entry would read as a hole.
    """
    label, rel, old, new, test = entry

    if not tree.file(rel).exists():
        return Outcome(index, "UNJUDGED", label, rel, test, 0.0,
                       f"{rel} is not in this tree")
    if rel in unbuilt:
        return Outcome(index, "UNJUDGED", label, rel, test, 0.0,
                       f"{rel} is not built on this platform")

    try:
        original = tree.read(rel)
    except TreeDamaged as damage:
        tree.rebuild()
        return Outcome(index, "DAMAGED", label, rel, test, 0.0, str(damage))

    if old.encode() not in original:
        return Outcome(index, "SKIP", label, rel, test, 0.0,
                       f"pattern not found in {rel} - the mutation is stale")

    path = tree.file(rel)
    try:
        with mutation_applied(path, original,
                              original.replace(old.encode(), new.encode(), 1)):
            result = run_test(test, tree.path, budget, tree.slot, deadline)
    except TreeCorrupted as damage:
        # In the sequential runner this ended the run, because every later
        # entry would read the damaged file and take it for its own original.
        # The damage cannot leave a worker copy, so the answer is a fresh copy
        # and a verdict that says the entry proved nothing.
        tree.rebuild()
        return Outcome(index, "DAMAGED", label, rel, test, 0.0, str(damage))

    secs = result.seconds
    if result.capped:
        return Outcome(index, "CAPPED", label, rel, test, secs, result.capped)
    if not result.compiled:
        return Outcome(index, "BROKEN", label, rel, test, secs)
    if result.passed:
        reason = unjudged_reason(test, tree.path)
        if reason:
            return Outcome(index, "UNJUDGED", label, rel, test, secs, reason)
    if result.passed:
        return Outcome(index, "HOLE", label, rel, test, secs)
    return Outcome(index, "ok", label, rel, test, secs)


def say(out: Outcome) -> None:
    """One entry's verdict, in the wording the sequential runner used."""
    if out.verdict == "SKIP":
        print(f"SKIP  {out.label}\n      {out.detail}", flush=True)
    elif out.verdict == "DAMAGED":
        print(f"DAMAGED {out.label}\n      {out.detail}\n"
              f"      the worker tree was rebuilt - this entry proved nothing, run it again",
              flush=True)
    elif out.verdict == "CAPPED":
        print(f"CAPPED [{out.secs:5.2f}s] {out.label}\n"
              f"      stopped on {out.detail} - the guard was not allowed to finish", flush=True)
    elif out.verdict == "BROKEN":
        print(f"BROKEN [{out.secs:5.2f}s] {out.label}\n"
              f"      the mutated tree does not compile - this proves nothing, rewrite the mutation",
              flush=True)
    elif out.verdict == "UNJUDGED":
        print(f"UNJUDGED [{out.secs:5.2f}s] {out.label}\n"
              f"      {out.detail}, so this platform cannot say", flush=True)
    elif out.verdict == "HOLE":
        print(f"HOLE  [{out.secs:5.2f}s] {out.label}\n"
              f"      {out.test} stayed GREEN - the guard does not guard this", flush=True)
    else:
        print(f"ok    [{out.secs:5.2f}s] {out.label}\n      {out.test} went red", flush=True)


def dispatch(entries, trees, budget, deadline: float | None = None,
             unbuilt: frozenset[str] = frozenset()) -> tuple[list[Outcome], str]:
    """Run every entry across the worker trees, printing verdicts as they land.

    entries are pairs of a position in the whole list and the entry, so a
    verdict names the same entry whichever part of the list a run was given.

    One thread per tree, each pulling from a shared queue, rather than a pool
    that hands trees out per task: a worker owns its tree for the whole run,
    so nothing has to decide who may write to which copy.

    The disk floor is still asked BETWEEN entries rather than only at the
    start. The estimate is this machine's number and everything else on the
    disk is free to move while a run this long goes - and a wide run fills the
    cache faster than the sequential one it replaces, not slower.
    """
    if deadline is None:
        deadline = time_cap_for(len(trees))

    work: queue.Queue = queue.Queue()
    for index, entry in entries:
        work.put((index, entry))

    done: queue.Queue = queue.Queue()
    stop = threading.Event()
    stopped_because = []

    def worker(tree) -> None:
        while not stop.is_set():
            try:
                index, entry = work.get_nowait()
            except queue.Empty:
                return
            low = below_floor()
            if low:
                stopped_because.append(low)
                stop.set()
                return
            try:
                done.put(judge(index, entry, tree, budget, deadline, unbuilt))
            finally:
                # Belt and braces over the release inside run_test, and the
                # reason is what a leak would DO rather than how likely it is.
                # A slot left registered keeps a finished entry's memory in the
                # shared total, so the workers still running are measured
                # against a number that includes a process which has exited -
                # and the ceiling then fires on them. That reads as CAPPED,
                # which is this runner saying "proved nothing", about entries
                # that were fine. Releasing twice is free; releasing never is
                # a run that quietly proves less.
                budget.release(tree.slot)

    threads = [threading.Thread(target=worker, args=(tree,), daemon=True)
               for tree in trees]
    for t in threads:
        t.start()

    outcomes: list[Outcome] = []
    alive = len(threads)
    while alive:
        try:
            out = done.get(timeout=0.2)
        except queue.Empty:
            alive = sum(1 for t in threads if t.is_alive())
            continue
        outcomes.append(out)
        say(out)
        if out.verdict == "ok":
            # Recorded here, on the main thread, and against the REAL tree
            # rather than the worker copy. The proof is about the content the
            # entry was run against, and proven.json is one file that six
            # threads must not write at once. O127.
            record_proof(out.label, ROOT / out.rel)

    for t in threads:
        t.join()
    while not done.empty():
        out = done.get()
        outcomes.append(out)
        say(out)
        if out.verdict == "ok":
            record_proof(out.label, ROOT / out.rel)

    return outcomes, (stopped_because[0] if stopped_because else "")


def part(text: str) -> tuple[int, int]:
    """I/K as two numbers, refused unless 0 <= I < K."""
    try:
        index, count = (int(n) for n in text.split("/"))
    except ValueError:
        raise argparse.ArgumentTypeError(f"{text!r} is not I/K, for example 0/16") from None
    if count < 1 or not 0 <= index < count:
        raise argparse.ArgumentTypeError(f"{text!r} names no part - I runs from 0 to K-1")
    return index, count


def arguments() -> argparse.Namespace:
    ask = argparse.ArgumentParser(description="Break each behaviour on purpose and check its guard notices.")
    ask.add_argument("--workers", type=int, default=DEFAULT_WORKERS,
                     help=f"entries run at once, each in its own copy (default {DEFAULT_WORKERS})")
    ask.add_argument("--changed", metavar="BASE",
                     help="only the entries the change since BASE reaches")
    ask.add_argument("--shard", type=part, metavar="I/K",
                     help="only part I of K, counted from 0")
    ask.add_argument("--plan", action="store_true",
                     help="print entries=N, parts=K and shards=[...] for the workflow and run nothing")
    ask.add_argument("--json", type=Path, metavar="PATH",
                     help="write every verdict to PATH as well")
    return ask.parse_args()


def chosen_entries(args: argparse.Namespace, say) -> list[int] | None:
    """The positions this run takes, or None when the selection cannot be trusted."""
    guards = guard_files(ROOT)
    faults = selection_canary(MUTATIONS, guards)
    if faults:
        say("the selection of entries cannot be trusted, so nothing was run:")
        for fault in faults:
            say(f"  {fault}")
        return None
    if not args.changed:
        return list(range(len(MUTATIONS)))
    changed = changed_files(ROOT, args.changed)
    positions = reached(MUTATIONS, changed, entries_at(ROOT, args.changed), guards)
    say(f"{len(positions)} of {len(MUTATIONS)} entries are reached by the "
        f"{len(changed)} file(s) changed since {args.changed}")
    return positions


def write_report(path: Path | None, report: dict) -> None:
    """Every verdict as JSON, for the job that puts the parts of a run together."""
    if path is None:
        return
    path.write_text(json.dumps(report, indent=1), encoding="utf-8", newline="\n")


def main() -> int:
    args = arguments()

    # The plan is read by the workflow from standard output, line by line as
    # key=value, so everything else it says goes to standard error.
    say = (lambda text: print(text, file=sys.stderr)) if args.plan else print
    positions = chosen_entries(args, say)
    if positions is None:
        return 2
    if args.plan:
        parts = shard_count(len(positions))
        print(f"entries={len(positions)}")
        print(f"parts={parts}")
        print(f"shards={json.dumps(list(range(parts)))}")
        return 0
    if args.shard:
        positions = shard(positions, *args.shard)
    chosen = [(p, MUTATIONS[p]) for p in positions]
    total = len(chosen)

    report = {
        "shard": list(args.shard) if args.shard else [0, 1],
        "changed": args.changed or "",
        "entries": total,
        "platform": sys.platform,
        "canary": False,
        "stopped": "",
        "wall": 0.0,
        "workers": 0,
        "verdicts": [],
    }

    missing = require_go()
    if missing:
        print(missing)
        report["stopped"] = missing
        write_report(args.json, report)
        return 2
    if not proofs_recorded():
        print("proofs are not written down here - tools/mutate is a local directory\n")
    if not chosen:
        # Not a success by default and not a failure. A part of an empty
        # selection is a real state, and the report has to be able to tell it
        # from a part that never answered.
        print("no entry to run in this part")
        report["canary"] = True
        write_report(args.json, report)
        return 0

    workers = min(max(1, args.workers), total)
    report["workers"] = workers
    started = time.perf_counter()

    # Asked once, before anything is copied. The answer does not change while
    # the run goes, and refusing here costs a syscall where finding out the
    # hard way costs the whole run and a log ending in BROKEN lines. The
    # canary mutates a file too, hence the extra entry.
    cramped = no_room_for(total + 1, workers)
    if cramped:
        print(f"not enough disk for this run\n  {cramped}")
        report["stopped"] = "not enough disk for this run"
        write_report(args.json, report)
        return 2

    # Asked once, of go, with the tags and the environment every entry runs
    # under. The answer is this platform's and nobody else's.
    unbuilt = frozenset(unbuilt_files(ROOT))

    ceiling = measured_ceiling(workers)
    deadline = time_cap_for(workers)
    print(f"{free_bytes() / 1024**3:.1f} GB free, {total} entries, "
          f"{workers} workers, {ceiling / 1024**3:.1f} GB shared memory ceiling, "
          f"{deadline:.0f} s per entry, {len(unbuilt)} Go file(s) not built here")

    trees = make_trees(workers)
    budget = Budget(ceiling)
    try:
        cold = warm(trees)
        if cold:
            print(f"\nno entry was run:\n{cold}")
            report["stopped"] = "the worker trees could not be built"
            write_report(args.json, report)
            return 2
        print()
        # The canary first and alone. It proves the BROKEN path works, and
        # every BROKEN verdict below is unreliable if it does not - so it is
        # worth the one entry it costs to run it before the others start.
        canary_ok = check_canary_in(trees[0])

        outcomes, stopped = dispatch(chosen, trees, budget, deadline, unbuilt)

        left = [(tree.slot, moved) for tree in trees for moved in [tree.moved()] if moved]
    finally:
        drop_trees(trees)

    wall = time.perf_counter() - started
    print()

    report.update(canary=canary_ok, stopped=stopped, wall=round(wall, 1), verdicts=[
        {"index": o.index, "verdict": o.verdict, "label": o.label, "file": o.rel,
         "guard": o.test, "seconds": round(o.secs, 2), "detail": o.detail}
        for o in sorted(outcomes, key=lambda o: o.index)])
    write_report(args.json, report)

    outcomes.sort(key=lambda o: o.index)
    timings = [(o.secs, o.label, o.rel, o.test) for o in outcomes if o.secs > 0]
    by = lambda v: [o for o in outcomes if o.verdict == v]

    # A worker tree that ended holding something else is worth saying even
    # though it is disposable and already gone. It means a restore failed, and
    # the entry that failed it proved nothing - the same fact the DAMAGED
    # lines above carry, said once more where a reader looks for it.
    if left:
        print(f"{len(left)} worker tree(s) ended holding something other than they started with:")
        for slot, moved in left:
            print(f"  worker {slot}: {', '.join(moved)}")
        print("  the real repository was never mutated, so nothing needs putting back by hand.\n")

    failures = [o.label for o in by("SKIP") + by("BROKEN") + by("HOLE") + by("DAMAGED")]
    if not canary_ok:
        failures.append(CANARY[0])
    capped_runs = [f"{o.label}  ({o.detail})" for o in by("CAPPED")]
    unjudged = [f"{o.label}  ({o.detail})" for o in by("UNJUDGED")]

    if budget.trips:
        # Loud. A shared ceiling that fires quietly turns "six workers were
        # too many for this machine today" into "these entries are flaky".
        print(f"the shared memory ceiling fired {budget.trips} time(s) - "
              f"try --workers {max(1, workers - 2)} if that keeps happening\n")

    if capped_runs:
        # Loud, because a ceiling that quietly swallows entries turns a run
        # that proves less into a run that looks the same.
        print(f"{len(capped_runs)} of {total} hit the resource ceiling and were not judged")
        for label in capped_runs:
            print(f"  capped: {label}")
        print(f"  These are proven caught by the full runs of 2026-08-11, before the ceiling existed.\n"
              f"  Raising it means giving the machine back to the mutation - see docs/QUALITY.md.\n")
    if unjudged:
        # Said out loud rather than left out. A mutation nobody can judge here
        # is not a mutation that passed, and the difference disappears if the
        # only record of it is a line somewhere above.
        print(f"{len(unjudged)} of {total} could not be judged on this platform")
        for label in unjudged:
            print(f"  unjudged: {label}")
        print()
    if failures:
        print(f"{len(failures)} of {total} mutations were not caught")
        # Named here, not left to be found by eye. A run that says only "1 of
        # 240" sends the next reader scanning 240 lines for it.
        for label in failures:
            print(f"  not caught: {label}")
        report_time(timings, wall)
        return 1
    if stopped:
        # Neither "all caught" nor a failure of any guard: the entries after
        # this one did not run at all. Rolling it into either sentence would
        # let a partial run read exactly like a whole one, which is rule 6.
        print(f"STOPPED after {len(outcomes)} of {total} entries - {stopped}")
        print("  every verdict above stands and the repository is intact, but the")
        print("  entries after this one were not tried.")
        print("  go clean -cache gives back what earlier runs left behind.")
        report_time(timings, wall)
        return 1
    unproven = len(capped_runs) + len(unjudged)
    # "all caught" has to mean all of them. An entry stopped by the ceiling or
    # skipped by its own guard is neither caught nor a failure, and rolling it
    # into the happy sentence is the silence rule 6 forbids.
    if unproven:
        print(f"{total - unproven} of {total} mutations caught, "
              f"{unproven} not judged - listed above")
    else:
        print(f"all {total} mutations caught")
    report_time(timings, wall)
    return 0


if __name__ == "__main__":
    sys.exit(main())
