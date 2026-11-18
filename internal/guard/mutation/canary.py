"""The canary of a mutation run: a file that cannot compile, planted on purpose.

Every BROKEN verdict rests on the runner telling a tree that does not build
from a guard that failed. The canary proves that on every run, before the
first entry: it plants a file the compiler must refuse and demands BROKEN
with the compiler's own complaint about that file.

It is a new file rather than a change to an existing one, because existing
code moves. A canary pointing at code that moved stops every run before its
first entry, and on a pull request that reads as a part that did not report
(O366). A new file points at nothing in the code, so no refactor leaves it
behind. What it still needs from the tree is asked by fault(), which reads
text only, so the plan of a run and staleness.py ask it too.
"""
from pathlib import Path

# label, file, what the file holds, guard. The file sits in a package every
# guard builds, under a name nothing in the project uses. Any guard does as
# the guard, because the tree fails to build before a test is chosen.
CANARY = (
    "canary: a file that cannot compile must report BROKEN",
    "internal/core/zz_mutation_canary.go",
    "package core\n\nfunc init() {\n\tvar orphan int\n}\n",
    "TestOurOwnGeneratorsHaveNotDrifted",
)

GUARDS = "internal/guard"


class Unreadable(Exception):
    """A file the canary's checks need could not be read as UTF-8 text."""


def text(path: Path, root: Path) -> str:
    """The text of path, or Unreadable naming it relative to root.

    Raised rather than skipped, so a file nobody can read is the fault said
    instead of a traceback with no verdict, and is not taken for a file that
    lacks what was looked for.
    """
    try:
        return path.read_text(encoding="utf-8")
    except (OSError, UnicodeError) as err:
        raise Unreadable(f"{path.relative_to(root).as_posix()} cannot be read - {err}") from err


def module(root: Path) -> str:
    """The module path go.mod declares, or an empty string. Unreadable when go.mod cannot be read."""
    lines = text(root / "go.mod", root).splitlines()
    return next((line.split()[1] for line in lines if line.startswith("module ") and len(line.split()) == 2), "")


def fault(root: Path) -> str:
    """Why the canary cannot be planted in the tree at root, or an empty string."""
    try:
        return planting_fault(root)
    except Unreadable as err:
        return f"{err}, so where the canary belongs cannot be checked"


def planting_fault(root: Path) -> str:
    """fault(), with a file that cannot be read raised as Unreadable."""
    _label, rel, body, _test = CANARY
    path = root / rel
    home = Path(rel).parent.as_posix()
    if path.exists():
        return f"{rel} is already there, so planting the canary would overwrite it"
    if not path.parent.is_dir():
        return f"{home} is gone, so there is no package to plant the canary in"
    clause = body.split("\n", 1)[0]
    sources = [g for g in path.parent.glob("*.go") if not g.name.endswith("_test.go")]
    if not any(clause in text(g, root).splitlines() for g in sources):
        return f"no file in {home} says {clause!r}, so the canary would not belong to its package"
    if not (root / "go.mod").exists():
        return "there is no go.mod, so nothing says which guards import the canary's package"
    declared = module(root)
    if not declared:
        return "go.mod declares no module, so nothing says which guards import the canary's package"
    imported = f'"{declared}/{home}"'
    guards = (root / GUARDS).glob("*_test.go")
    if not any(imported in text(g, root) for g in guards):
        return f"no guard imports {home}, so a canary there would not stop the guards from building"
    return ""
