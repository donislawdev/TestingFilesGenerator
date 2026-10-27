"""Are the versions this repository's CI pins still the current ones?

Covers what Dependabot does not read. Dependabot reads go.mod, `uses:` and
requirements-semgrep.in. Everything below is a version written into a shell
command or a file of our own, so nothing else will ever say it went stale:

- `go run module@version` in the workflows - asked of the Go module proxy
- GO_VERSION in the workflows - asked of go.dev/dl
- the release of the software renderer in .github/mesa-dist-win - asked of
  that project's GitHub releases
- `npm install package@version` in the workflows - asked of the npm registry
- python-version and the WiX version - held back on purpose, see HELD

Reads the pins out of the repository rather than carrying its own copy, because
a list copied into a checker stops describing the thing it checks the moment
somebody edits the original.

Each pin is asked of the registry its own install command reads, not of the
GitHub releases page, and that is the whole correctness of this file. The first
version asked GitHub and produced two false alarms out of three: staticcheck
releases under two names at once, so v0.8.1 was reported behind "2026.2.1" when
they are the same thing, and golang.org/x/vuln answered with v1.1.4 as its
latest while the pin was already on v1.7.0. A pin is written in the words of the
thing that installs it, so that thing is what knows what the newest one is.

A version younger than a week is not yet due. That is the cooldown in
.github/dependabot.yml, which holds a release back for seven days because the
tj-actions and trivy-action compromises were poisoned releases pulled in within
hours of publication, and the owner chose the same hold for everything bumped by
hand on 2026-10-26. It is printed, so a reader sees it coming, and it does not
fail the run. The Go toolchain is the exception: its point releases are security
releases more often than not, and the cooldown does not hold back security fixes
for Dependabot either.

That matters more than tidiness here. A gate that cries wolf gets ignored, and
an ignored gate is what this whole file was written to replace. For the same
reason a decision the owner has taken is printed as a note and not as a
failure every Monday - see HELD.

Exits non-zero when a pin is behind, when a registry cannot be asked after a
few tries, and when it finds no pin at all: a checker that quietly checks
nothing is the same failure wearing a green tick.
"""

import datetime
import json
import os
import pathlib
import re
import sys
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github" / "workflows"
RENDERER_PIN = ROOT / ".github" / "mesa-dist-win"
RENDERER_PROJECT = "pal1000/mesa-dist-win"

HOLD = datetime.timedelta(days=7)

# Pins the owner holds where they are on purpose. A newer line is printed with
# the reason and never fails the run. A newer release INSIDE the held line is
# still due, and fails like any other.
HELD = {
    "python": "semgrep runs on Python 3.14 at most (owner, 2026-10-26)",
    "wix": "WiX 6 and 7 come under the OSMF EULA, 5.0.2 is MS-RL with no agreement "
           "(owner, 2026-10-26, docs/PACKAGING-2026-09-25.md section 6)",
}

# A registry that does not answer is asked again before the run fails. The run
# of 2026-10-26 went red on one read timeout from the module proxy, with every
# pin current.
RETRY_WAITS = (0, 5, 20)


def ask(url, raw=False):
    """The registry's answer, asked again after a pause when it does not come."""
    headers = {"User-Agent": "tool-versions"}
    token = os.environ.get("GH_TOKEN")
    if token and url.startswith("https://api.github.com/"):
        headers["Authorization"] = "Bearer " + token
    failure = None
    for wait in RETRY_WAITS:
        time.sleep(wait)
        try:
            request = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(request, timeout=30) as r:  # noqa: S310 - fixed https hosts
                body = r.read().decode("utf-8")
            return body if raw else json.loads(body)
        except Exception as exc:  # noqa: BLE001 - every failure is retried, then loud
            failure = exc
    raise RuntimeError(f"{url} did not answer after {len(RETRY_WAITS)} tries: {failure}")


def release_key(version):
    """A version as a comparable tuple, or None for a pre-release."""
    bare = re.sub(r"^(v|go)", "", version)
    if not re.fullmatch(r"\d+(\.\d+)*", bare):
        return None
    return tuple(int(part) for part in bare.split("."))


def stamp(text):
    """A registry's timestamp as an aware datetime."""
    return datetime.datetime.fromisoformat(text.replace("Z", "+00:00"))


def workflow_text():
    return {p.name: p.read_text(encoding="utf-8") for p in sorted(WORKFLOWS.glob("*.yml"))}


def single(found, what):
    """The one value a pattern found across the workflows, or a failure."""
    values = sorted(set(found))
    if not values:
        raise RuntimeError(f"no {what} was found in the workflows, so this checked nothing")
    if len(values) > 1:
        raise RuntimeError(f"the workflows disagree about {what}: {', '.join(values)}")
    return values[0]


def due(pinned, released, now):
    """Compare a pin with every release and its date: the row to print and whether it fails.

    released maps a version to when it was published. Only the newest release
    outside the hold is due. A newer one inside it is mentioned.
    """
    mine = release_key(pinned)
    later = sorted(
        ((release_key(v), v, when) for v, when in released.items()
         if release_key(v) is not None and release_key(v) > mine),
        reverse=True,
    )
    owed = [v for _, v, when in later if now - when >= HOLD]
    waiting = [(v, when) for _, v, when in later if now - when < HOLD]
    note = ""
    if waiting:
        v, when = waiting[0]
        note = f", {v} is {(now - when).days} day(s) old and waits out the week's hold"
    if owed:
        return f"BEHIND, {owed[0]} is due" + note, owed[0]
    return "current" + note, None


def go_tools(texts, now):
    """`go run module/cmd/x@version` - asked of the module proxy."""
    found = {}
    for text in texts.values():
        for m in re.finditer(r"go run ([\w./-]+?)/cmd/[\w-]+@(v[\w.+-]+)", text):
            found[m.group(1)] = m.group(2)
    if not found:
        raise RuntimeError("no `go run module@version` was found in the workflows, so this checked nothing")
    rows = []
    for module, pinned in sorted(found.items()):
        # Module paths are case encoded for the proxy: an upper case letter
        # becomes "!" and the lower case letter. None of ours have one today,
        # and doing it anyway costs a line and stops a silent 404 the day one does.
        encoded = re.sub(r"[A-Z]", lambda c: "!" + c.group(0).lower(), module)
        base = f"https://proxy.golang.org/{encoded}/@v/"
        newer = [v for v in ask(base + "list", raw=True).split()
                 if release_key(v) is not None and release_key(v) > release_key(pinned)]
        released = {v: stamp(ask(base + v + ".info")["Time"]) for v in newer}
        rows.append((module, pinned) + due(pinned, released, now))
    return rows


def go_toolchain(texts):
    """GO_VERSION - asked of go.dev/dl, without the hold."""
    pinned = single(re.findall(r'GO_VERSION:\s*"([\d.]+)"', "\n".join(texts.values())), "GO_VERSION")
    line = pinned.rsplit(".", 1)[0]
    stable = [x["version"][2:] for x in ask("https://go.dev/dl/?mode=json&include=all") if x.get("stable")]
    same_line = sorted((v for v in stable if v.rsplit(".", 1)[0] == line), key=release_key)
    newer_lines = sorted((v for v in stable if release_key(v)[:2] > release_key(pinned)[:2]), key=release_key)
    newest = same_line[-1] if same_line else pinned
    note = ""
    if newer_lines:
        note = (f", {newer_lines[-1]} starts a newer line - moving to it is a byte stability "
                "question (D11) and the owner's call")
    if release_key(newest) > release_key(pinned):
        return [("go (GO_VERSION)", pinned, f"BEHIND, {newest} is due" + note, newest)]
    return [("go (GO_VERSION)", pinned, "current" + note, None)]


def renderer(now):
    """The release of the software renderer the Windows archive carries."""
    text = RENDERER_PIN.read_text(encoding="utf-8")
    m = re.search(r"^version=(\S+)$", text, re.M)
    if not m:
        raise RuntimeError(f"{RENDERER_PIN.name} names no version, so this checked nothing")
    pinned = m.group(1)
    released = {
        r["tag_name"]: stamp(r["published_at"])
        for r in ask(f"https://api.github.com/repos/{RENDERER_PROJECT}/releases?per_page=30")
        if not r.get("draft") and not r.get("prerelease") and r.get("published_at")
    }
    return [(RENDERER_PROJECT, pinned) + due(pinned, released, now)]


def npm_packages(texts, now):
    """`npm install package@version` - asked of the npm registry."""
    found = {}
    for text in texts.values():
        for m in re.finditer(r"npm install [^\n]*?\s([@\w./-]+)@(\d[\w.+-]*)\s*$", text, re.M):
            found[m.group(1)] = m.group(2)
    rows = []
    for package, pinned in sorted(found.items()):
        times = ask(f"https://registry.npmjs.org/{package}")["time"]
        released = {v: stamp(t) for v, t in times.items() if release_key(v) is not None}
        rows.append((package + " (npm)", pinned) + due(pinned, released, now))
    return rows


def held(texts):
    """Pins the owner holds: a newer line is a note, a newer release inside the line is due."""
    rows = []
    python = single(re.findall(r'python-version:\s*"([\d.]+)"', "\n".join(texts.values())), "python-version")
    finals = [re.sub(r"^Python ", "", r["name"])
              for r in ask("https://www.python.org/api/v2/downloads/release/?is_published=true&pre_release=false")]
    finals = sorted((v for v in finals if release_key(v) is not None), key=release_key)
    state = "current"
    if finals and release_key(finals[-1])[:2] > release_key(python)[:2]:
        state = f"held, {finals[-1]} exists - {HELD['python']}"
    # setup-python takes the newest patch of the line itself, so the line is the pin.
    rows.append(("python (python-version)", python, state, None))

    wix = single(re.findall(r"dotnet tool install --global wix --version ([\d.]+)", "\n".join(texts.values())), "the WiX version")
    stable = sorted((v for v in ask("https://api.nuget.org/v3-flatcontainer/wix/index.json")["versions"]
                     if release_key(v) is not None), key=release_key)
    same_major = [v for v in stable if release_key(v)[0] == release_key(wix)[0]]
    if same_major and release_key(same_major[-1]) > release_key(wix):
        rows.append(("wix (dotnet tool)", wix, f"BEHIND, {same_major[-1]} is due", same_major[-1]))
    elif stable and release_key(stable[-1])[0] > release_key(wix)[0]:
        rows.append(("wix (dotnet tool)", wix, f"held, {stable[-1]} exists - {HELD['wix']}", None))
    else:
        rows.append(("wix (dotnet tool)", wix, "current", None))
    return rows


def main():
    now = datetime.datetime.now(datetime.timezone.utc)
    texts = workflow_text()
    rows = []
    for check in (lambda: go_tools(texts, now), lambda: go_toolchain(texts), lambda: renderer(now),
                  lambda: npm_packages(texts, now), lambda: held(texts)):
        try:
            rows.extend(check())
        except Exception as exc:  # noqa: BLE001 - any failure here has to be loud
            print(f"FAIL: {exc}")
            return 1

    for name, pinned, state, _ in rows:
        print(f"  {name:<42} {pinned:<10} {state}")
    behind = [(name, pinned, to) for name, pinned, _, to in rows if to]

    print()
    if not behind:
        print(f"all {len(rows)} pin(s) are current or held on purpose")
        return 0

    print(f"{len(behind)} pin(s) are behind:")
    for name, pinned, to in behind:
        print(f"  {name} {pinned} -> {to}")
    print()
    print("Nothing else watches these. Dependabot reads go.mod, `uses:` and requirements-semgrep.in,")
    print("and these are shell commands and files of our own, so this job is the only thing that will")
    print("ever say so. Raise each where it is written - .github/workflows, or .github/mesa-dist-win")
    print("together with internal/legal/companions.go and THIRD-PARTY-NOTICES.md - and check the new")
    print("version against this tree before merging. The last two of the Go tools to go stale had")
    print("stopped working with the compiler entirely.")
    return 1


if __name__ == "__main__":
    sys.exit(main())
