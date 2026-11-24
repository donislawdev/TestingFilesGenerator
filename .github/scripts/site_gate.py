"""Whether the site may go out for the CI run that triggered it.

    site_gate.py JOBS HEAD TIP

JOBS is the jobs of the CI run, a name, a conclusion and the runner labels to
a line with a tab between each, as the ci job of pages.yml asks the API for
them. filter=latest answers with the newest try of every job, so a job that
was run again is judged by the try that counts. HEAD is the commit CI ran on
and TIP the commit main is at now.

Exit 0: the site goes out. Exit 3: it does not, and the lines above say why.
Anything else: the script could not judge, and the job goes red rather than
holding the site back without anyone seeing it.

Jobs on macOS are left out. GitHub has had no macOS arm64 runner free long
enough, again and again, that it cancelled those jobs before their first step
(O371), and nothing the site shows depends on macOS: the pages are compared
with the program by the guards on every system, and the sample files are made
on Linux. A job is on macOS when its runner labels name macOS, or when its name
does - the verdict "test on macos-latest" runs on Linux and judges the macOS
parts. Every other job has to have succeeded or been skipped, and at least one
verdict "test on ..." has to have succeeded, so a list that lost the guards
does not pass for a green one.

A commit that is no longer the tip of main does not go out. Every try of CI
ends in a run of this workflow, so an older commit whose macOS jobs were run
again later would otherwise put its site back over a newer one.
"""

import sys

PUBLISH, HOLD, CANNOT = 0, 3, 2

# What a job that is neither red nor still running concludes.
PASSED = ("success", "skipped")

# The name the verdicts of ci.yml carry, one for each system.
VERDICT = "test on "


def cannot(message):
    print("site_gate: " + message, file=sys.stderr)
    sys.exit(CANNOT)


def on_macos(name, labels):
    """A job about macOS: its runner, or its name, says so."""
    if "macos" in name.lower():
        return True
    return any(label.strip().lower().startswith("macos") for label in labels.split(","))


def read_jobs(path):
    try:
        with open(path, encoding="utf-8") as f:
            lines = [line.rstrip("\r\n") for line in f]
    except OSError as e:
        cannot("could not read the list of jobs %s: %s" % (path, e))
    jobs = []
    for number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        fields = line.split("\t")
        if len(fields) < 2 or not fields[0].strip() or not fields[1].strip():
            cannot("line %d of %s is not a name and a conclusion with a tab between: %r" % (number, path, line))
        jobs.append((fields[0].strip(), fields[1].strip(), fields[2] if len(fields) > 2 else ""))
    if not jobs:
        cannot("%s lists no job, so nothing says CI ran at all" % path)
    return jobs


def judge(jobs, head, tip):
    """The reasons the site does not go out, none when it does."""
    reasons, verdicts = [], 0
    for name, conclusion, labels in jobs:
        if on_macos(name, labels):
            print("left out, on macOS: %s (%s)" % (name, conclusion))
            continue
        if conclusion not in PASSED:
            reasons.append("%s concluded %s" % (name, conclusion))
        if name.startswith(VERDICT) and conclusion == "success":
            verdicts += 1
    if verdicts == 0:
        reasons.append("no verdict \"%s...\" outside macOS succeeded, so nothing says the guards passed" % VERDICT)
    if head != tip:
        reasons.append("CI ran on %s and main is at %s now, so this site would replace a newer one" % (head, tip))
    return reasons


def main():
    if len(sys.argv) != 4:
        cannot("usage: site_gate.py JOBS HEAD TIP")
    _, path, head, tip = sys.argv
    if not head.strip() or not tip.strip():
        cannot("the commit CI ran on or the tip of main is empty")
    reasons = judge(read_jobs(path), head.strip(), tip.strip())
    for reason in reasons:
        print("the site does not go out: " + reason)
    if reasons:
        sys.exit(HOLD)
    print("the site goes out: every job outside macOS passed, on the tip of main")
    sys.exit(PUBLISH)


if __name__ == "__main__":
    main()
