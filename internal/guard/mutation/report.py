"""Put the parts of a mutation run together and say what they found.

Each job of a run writes its verdicts with mutate.py --json. This reads all of
them and writes one summary, and it is the only place that decides whether
the run as a whole went red.

A part that never answered is a failure, not a smaller run. The job can die on
its time limit, on the machine running out of memory, or on a network fetch
before the first entry, and every one of those leaves no file at all. Counting
only the files that arrived would turn a run that checked half the entries
into one that reads exactly like a whole one.

    python internal/guard/mutation/report.py --dir results --entries 1779 --shards 16 [--gate]

--gate makes uncaught mutations fail the run. Without it they are reported as
warnings and the run stays green, which is how pull requests are reported
while the runner earns trust on CI.
"""
import argparse
import collections
import json
import os
import statistics
import sys
from pathlib import Path

# What makes a run red. CAPPED and UNJUDGED are said out loud and are not
# failures: neither is a verdict about the guard.
FAILING = ("HOLE", "BROKEN", "SKIP", "DAMAGED")


def command_text(text: str) -> str:
    """Text made safe for a workflow command, which ends at the first newline."""
    return text.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")


def load(directory: Path) -> list[dict]:
    reports = []
    for path in sorted(directory.rglob("result-*.json")):
        reports.append(json.loads(path.read_text(encoding="utf-8")))
    return reports


def main() -> int:
    ask = argparse.ArgumentParser(description="Put the parts of a mutation run together.")
    ask.add_argument("--dir", type=Path, required=True, help="where the parts' JSON files are")
    ask.add_argument("--entries", type=int, required=True, help="how many entries the plan chose")
    ask.add_argument("--shards", type=int, required=True, help="how many parts the plan made")
    ask.add_argument("--gate", action="store_true", help="fail the run on any uncaught mutation")
    ask.add_argument("--summary", type=Path, default=os.environ.get("GITHUB_STEP_SUMMARY"),
                     help="where to write the summary (default: the step summary)")
    args = ask.parse_args()

    reports = load(args.dir) if args.dir.is_dir() else []
    by_part = {tuple(r["shard"])[0]: r for r in reports}
    problems: list[str] = []

    missing = [i for i in range(args.shards) if i not in by_part]
    for i in missing:
        problems.append(f"part {i} of {args.shards} did not report - its entries were not run")
    strays = [i for i in by_part if not 0 <= i < args.shards]
    for i in strays:
        problems.append(f"a part numbered {i} reported, and the plan made {args.shards}")
    for i, r in sorted(by_part.items()):
        if r["shard"][1] != args.shards and args.shards > 0:
            problems.append(f"part {i} was cut for {r['shard'][1]} parts, and the plan made {args.shards}")
        if r["stopped"]:
            problems.append(f"part {i} stopped early - {r['stopped']}")
        if not r["canary"]:
            problems.append(f"part {i}: the canary failed, so its BROKEN verdicts cannot be trusted")
        if len(r["verdicts"]) != r["entries"] and not r["stopped"]:
            problems.append(f"part {i} chose {r['entries']} entries and judged {len(r['verdicts'])}")

    run = sum(r["entries"] for r in by_part.values())
    if not missing and run != args.entries:
        problems.append(f"the parts chose {run} entries between them, and the plan chose {args.entries}")

    verdicts = [v for r in by_part.values() for v in r["verdicts"]]
    counts = collections.Counter(v["verdict"] for v in verdicts)
    failing = sorted((v for v in verdicts if v["verdict"] in FAILING), key=lambda v: v["index"])
    unjudged = sorted((v for v in verdicts if v["verdict"] in ("UNJUDGED", "CAPPED")),
                      key=lambda v: v["index"])

    level = "error" if args.gate else "warning"
    for v in failing:
        print(f"::{level} title=mutation {v['verdict']}::"
              + command_text(f"{v['label']} - {v['guard']} in {v['file']}"))
    for problem in problems:
        print(f"::{level} title=mutation run::" + command_text(problem))

    lines = ["## Mutation run", ""]
    lines.append(f"{args.entries} entries planned in {args.shards} part(s), "
                 f"{len(verdicts)} judged.")
    lines.append("")
    lines.append("| verdict | entries |")
    lines.append("|---|---|")
    for name in ("ok", *FAILING, "CAPPED", "UNJUDGED"):
        if counts.get(name):
            lines.append(f"| {name} | {counts[name]} |")
    lines.append("")
    if problems:
        lines.append("### The run itself")
        lines.extend(f"- {p}" for p in problems)
        lines.append("")
    if failing:
        lines.append("### Not caught")
        lines.extend(f"- **{v['verdict']}** {v['label']} - `{v['guard']}` in `{v['file']}`"
                     + (f" ({v['detail']})" if v["detail"] else "") for v in failing)
        lines.append("")
    if unjudged:
        lines.append("<details><summary>Not judged here "
                     f"({len(unjudged)})</summary>")
        lines.append("")
        lines.extend(f"- {v['verdict']} {v['label']} - {v['detail']}" for v in unjudged)
        lines.append("")
        lines.append("</details>")
        lines.append("")
    seconds = [v["seconds"] for v in verdicts if v["seconds"] > 0]
    walls = [r["wall"] for r in by_part.values() if r["wall"] > 0]
    if seconds and walls:
        lines.append(f"Longest part {max(walls) / 60:.1f} min. Per entry median "
                     f"{statistics.median(seconds):.2f} s, longest {max(seconds):.2f} s, "
                     f"{max(r['workers'] for r in by_part.values())} worker(s) per part.")
    if not failing and not problems:
        lines.append("")
        lines.append("Every judged mutation was caught.")
    text = "\n".join(lines) + "\n"

    if args.summary:
        with open(args.summary, "a", encoding="utf-8") as out:
            out.write(text)
    else:
        print(text)

    red = bool(failing or problems)
    return 1 if red and args.gate else 0


if __name__ == "__main__":
    sys.exit(main())
