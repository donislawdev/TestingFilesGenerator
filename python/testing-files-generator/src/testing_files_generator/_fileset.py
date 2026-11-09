# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

"""Sets of files: planned when they are declared, written once, kept and checked.

The same logic as lib/fileset.js in the npm package, in the same order, so
the two can be read side by side. What a file says of itself and the sentences
check raises are held to the npm package's by testdata/wrappers/vectors.json,
which the tests of both packages read.
"""

from __future__ import annotations

import errno
import hashlib
import json
import ntpath
import os
import posixpath
import re
import secrets
import subprocess
import sys
import threading
import time
import warnings
from collections.abc import Callable, Iterable, Iterator, Mapping, Sequence
from typing import Any

from ._binary import binary_path
from ._run import _FLAGS, TfgError, ended, run, run_to_file, said
from ._testfile import OUTCOMES, TestFile

# The manifest major this package reads. Another major is refused rather than
# read as if it were this one - docs/MANIFEST.md, the version of the contract.
MANIFEST_MAJOR = "1"

# The code tfg verify ends with when a directory does not match its manifest,
# from the exit codes in docs/CLI.md. Any other code is verify failing to look.
_EXIT_VERIFY = 7

# Flags this package sets itself. An option of the same name would fight it.
RESERVED = {
    "out": "this package writes every set into a directory of its own",
    "json": "this package reads the manifest from the directory it writes",
    "dry-run": "this package plans each set with a dry run and then writes it",
    "preset": "a preset is named by the preset argument beside the options",
}

# Everything this package leaves while it works starts with this, followed by
# the process id, so a later run can tell a set being written now from one a
# stopped run left behind.
UNFINISHED = ".tmp-"

# The markers pytest registers itself, as pytest --markers lists them with no
# plugin and no configuration, which a test compares. A group by one of these
# names would not mark its files but change how their tests run, measured
# 2026-11-09 on pytest 9.2.0: skip and skipif skip them, xfail hides a failure,
# and pytest.param refuses usefixtures. A plugin's markers are not known here,
# and the README says so.
PYTEST_OWN_MARKS = frozenset({"filterwarnings", "skip", "skipif", "xfail", "parametrize", "usefixtures",
                              "tryfirst", "trylast"})

# A rename the system refuses for a moment. On Windows a scanner holding a new
# file open is enough, so these are tried again before anything is said.
_BUSY_ERRNO = {errno.EACCES, errno.EPERM, errno.EBUSY}
_BUSY_WINERROR = {5, 32}  # access denied, sharing violation
_RENAME_TRIES = 10

# One plan per set and process. A test module is imported once per process,
# and two modules can ask for one set.
_sources: dict[str, _Source] = {}
_sources_lock = threading.Lock()
_noted = False

# What a write did - wrote a copy, moved it to its name, or took one another
# process moved first. Not part of the package's interface: the tests listen,
# because a race that never happened passes as well as one won.
_listeners: list[Callable[[str, str], None]] = []

# The pytest run this process is part of, given by the plugin, for the
# directory sets are kept in and the markers a set's params use.
_pytest_config: Any = None
_registered: set[str] = set()


class UnfinishedSetWarning(UserWarning):
    """A run that stopped part way left an unfinished set, which nothing uses."""


def _emit(event: str, path: str) -> None:
    for listener in list(_listeners):
        listener(event, path)


def use_pytest(config: Any) -> None:
    """Called by the pytest plugin when a run starts, and with None when it ends."""
    global _pytest_config
    _pytest_config = config
    _registered.clear()


def file_set(preset: str | None = None, recipe: str | os.PathLike[str] | None = None,
             options: Mapping[str, Any] | None = None, cache_dir: str | os.PathLike[str] | None = None,
             binary: str | os.PathLike[str] | None = None) -> FileSet:
    """Plans a set of files, writing nothing, so tests can be declared from it while their module loads.

    The files are written the first time one of them is asked for.
    """
    if options is not None and not isinstance(options, Mapping):
        raise TypeError("options takes a dict, for example {'limit': '2mb', 'bulk': 0}.")
    shown = _described(preset, recipe, options)
    # Told from the flags and not from the arguments: an option set to False
    # or to an empty list adds no flag, and tfg asked for nothing answers in flags.
    args = _args_of(preset, recipe, options)
    if len(args) == 1:
        raise ValueError("file_set was not told which files to write. Name a preset, a recipe or the options "
                         "of a set, for example file_set(preset='upload-validation'). tfg preset list lists "
                         "the presets.")
    tfg = binary_path() if binary is None else _named(os.fspath(binary), "binary",
                                                       "Point it at a tfg binary, or leave it out to use the one "
                                                       "this package installs.")
    content = ""
    if recipe:
        where = os.path.abspath(os.fspath(recipe))
        _named(where, "recipe", f"A relative path is taken from the directory the tests started in, here "
                                f"{os.getcwd()}, so Path(__file__).parent / 'uploads.yaml' names a recipe beside "
                                "the test file.")
        # A pipe or a device would be read here without end, and what is read
        # here is gone before tfg reads it.
        if not os.path.isfile(where):
            raise ValueError(f"The recipe option names {where}, which is a directory, a pipe or a device rather "
                             "than a file. A recipe is read twice, once here to tell whether it changed and once "
                             "by tfg, so it has to be a YAML file on the disk. Give the path of that file.")
        content = _content_of(where)
    cache = os.path.abspath(os.fspath(cache_dir) if cache_dir is not None else default_cache_dir())
    key = _digest([_identity(tfg), args, content, cache])
    with _sources_lock:
        source = _sources.get(key)
        if source is None:
            source = _Source(tfg, args, cache, preset, recipe, shown)
            _sources[key] = source
    return FileSet(source, source.files)


def _content_of(recipe: str) -> str:
    """The sha256 of a recipe's bytes, so an edited recipe is planned again.

    Read in pieces and never decoded: tfg reads the recipe itself and says
    what is wrong with one too large or not in UTF-8, in its own words.
    """
    digest = hashlib.sha256()
    with open(recipe, "rb") as f:
        for piece in iter(lambda: f.read(1 << 16), b""):
            digest.update(piece)
    return digest.hexdigest()


def _text(value: Any) -> str:
    # A whole number given as a float is passed as a whole number, 2 and not
    # 2.0, as the npm package passes it.
    if isinstance(value, float) and value.is_integer():
        return str(int(value))
    return str(value)


def _args_of(preset: str | None, recipe: Any, options: Mapping[str, Any] | None) -> list[str]:
    args = ["generate"]
    if recipe:
        args.append(os.path.abspath(os.fspath(recipe)))
    if preset:
        args.append(f"--preset={preset}")
    for raw, value in (options or {}).items():
        # far_over is far-over: a Python name cannot hold a hyphen, and no flag
        # of tfg holds an underscore.
        key = raw.replace("_", "-") if isinstance(raw, str) else raw
        if key in RESERVED:
            raise ValueError(f"The option {raw} cannot be set here - {RESERVED[key]}.")
        if not isinstance(key, str) or not re.fullmatch(r"[a-z][a-z0-9-]*", key):
            raise ValueError(f"{raw!r} is not a flag tfg has. Options are flag names without the dashes, "
                             "such as limit or far-over.")
        for one in value if isinstance(value, (list, tuple)) else [value]:
            if one is True:
                args.append(f"--{key}")
            elif one is False or one is None:
                continue
            elif isinstance(one, (str, int, float)):
                args.append(f"--{key}={_text(one)}")
            else:
                raise TypeError(f"The option {raw} takes a string, a number, True or a list of them. "
                                f"It was given a {type(one).__name__}.")
    return args


def _named(file: str, option: str, way_out: str) -> str:
    """A file an argument names, or a sentence saying there is none, before anything reads it."""
    if not os.path.exists(file):
        raise FileNotFoundError(f"The {option} option names {file}, and there is no file there. {way_out}")
    return file


def default_cache_dir() -> str:
    """Where sets are kept: TFG_CACHE_DIR, the cache of the pytest run, or the user's own cache directory."""
    given = os.environ.get("TFG_CACHE_DIR")
    if given:
        return given
    cache = getattr(_pytest_config, "cache", None)
    if cache is not None:
        return str(cache.mkdir("testing-files-generator"))
    return _user_cache_dir()


def _user_cache_dir(env: Mapping[str, str] | None = None, system: str | None = None,
                    home: str | None = None) -> str:
    # A directory of the user's own, never the system's temporary directory,
    # which every user of a Linux machine shares - someone else could make the
    # directory first and change the files the tests run on (code scanning
    # alerts 23 and 24, docs/NPM-PUBLISH-2026-11-09.md).
    env = os.environ if env is None else env
    system = sys.platform if system is None else system
    home = os.path.expanduser("~") if home is None else home
    # The paths of the system asked about, which are this system's paths
    # except in the tests.
    paths = ntpath if system == "win32" else posixpath
    if system == "win32":
        base = env.get("LOCALAPPDATA") or paths.join(home, "AppData", "Local")
    elif system == "darwin":
        base = paths.join(home, "Library", "Caches")
    else:
        xdg = env.get("XDG_CACHE_HOME", "")
        base = xdg if paths.isabs(xdg) else paths.join(home, ".cache")
    if not paths.isabs(base):
        raise RuntimeError("testing-files-generator found no home directory to keep sets in. Set TFG_CACHE_DIR, "
                           "or the cache_dir argument of file_set, to a directory of your own.")
    return paths.join(base, "testing-files-generator")


def _make_cache(cache: str) -> None:
    """Makes the directory sets are kept in, or says where it comes from and how to give another."""
    try:
        os.makedirs(cache, exist_ok=True)
    except OSError as err:
        raise _cache_refused(cache, "it could not be made a directory", err) from err


def _cache_refused(cache: str, what: str, err: OSError) -> RuntimeError:
    """The directory of sets this user cannot use, with the system's reason and how to give another."""
    return RuntimeError(f"Sets are kept in {cache}, and {what} - {err.strerror or err}. Set TFG_CACHE_DIR, or the "
                        "cache_dir argument of file_set, to a directory this user can read and write.")


def _label_of(preset: str | None, recipe: Any) -> str:
    if preset:
        name = preset
    elif recipe:
        name = re.sub(r"\.[^.]*$", "", os.path.basename(os.fspath(recipe)))
    else:
        name = "flags"
    return re.sub(r"[^A-Za-z0-9_-]+", "-", name)[:40]


def _described(preset: Any, recipe: Any, options: Any) -> str:
    """The call as the test wrote it, for a message - what makes the set, without where it is kept."""
    shown = [f"{key}={value!r}" for key, value in (("preset", preset), ("recipe", recipe), ("options", options))
             if value is not None]
    return f"file_set({', '.join(shown)})"


def _identity(binary: str) -> list[Any]:
    # What tells two binaries apart without running them, so a new build or
    # another version plans again rather than reusing an old plan.
    stat = os.stat(binary)
    return [os.path.realpath(binary), stat.st_size, stat.st_mtime_ns]


def _digest(value: Any) -> str:
    return hashlib.sha256(json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode()).hexdigest()


def _scratch(cache: str) -> str:
    return os.path.join(cache, f"{UNFINISHED}{os.getpid()}-{secrets.token_hex(4)}")


def set_name(label: str, args: Sequence[str], plan: Mapping[str, Any]) -> str:
    """The name of a set's directory.

    The recipe hash leaves out the seed and --clean, measured 2026-11-04, so
    the arguments are part of the name and two seeds never share a set.
    """
    run_ = plan["run"]
    return f"{label}-{_digest([list(args), run_.get('recipe_hash'), run_['seed'], plan['tool']])[:20]}"


class _Source:
    """One set: its plan, where it is written, and the one write a process makes of it however many files ask."""

    def __init__(self, binary: str, args: list[str], cache: str, preset: str | None, recipe: Any, shown: str):
        self.binary = binary
        self.args = args
        self.cache = cache
        self.preset = preset
        self.shown = shown
        _make_cache(cache)
        _note_unfinished(cache)
        self.plan = self._planned()
        self.dir = os.path.join(cache, set_name(_label_of(preset, recipe), args, self.plan))
        self.files = tuple(TestFile(self, entry) for entry in self.plan["files"])
        self._lock = threading.Lock()
        self._written: str | None = None

    def _planned(self) -> dict[str, Any]:
        # The dry run names a directory that is never made, because a dry run
        # into a directory holding a manifest is refused like a real one.
        out = f"{_scratch(self.cache)}.json"
        try:
            run_to_file(self.binary, [*self.args, f"--out={_scratch(self.cache)}", "--dry-run", "--json"], out)
            with open(out, encoding="utf-8") as f:
                plan = json.load(f)
            readable(plan)
        except TfgError as err:
            raise _refused_plan(self.shown, self.preset, err) from err
        except OSError as err:
            # The plan is the first file written in the directory of sets.
            if err.filename != out:
                raise
            raise _cache_refused(self.cache, "nothing could be written in it", err) from err
        except (ValueError, RecursionError) as err:
            raise RuntimeError(f"{self.binary} printed a plan that is not a manifest of tfg - {err}. It has to be "
                               "tfg of the same version as this package. Reinstall the package with pip install "
                               "--force-reinstall testing-files-generator, or set TFG_BINARY to the path of such "
                               "a tfg.") from err
        finally:
            try:
                os.remove(out)
            except FileNotFoundError:
                pass
        return plan

    def write(self) -> str:
        with self._lock:
            if self._written is None:
                self._written = self._write()
            return self._written

    def _write(self) -> str:
        if os.path.exists(self.dir):
            self._taken()
            return self.dir
        tmp = _scratch(self.cache)
        os.mkdir(tmp)
        try:
            run(self.binary, [*self.args, f"--out={tmp}"])
            problem = mismatch(tmp, self.plan)
            if problem:
                raise RuntimeError(f"tfg wrote a set that does not match its own plan - {problem}. "
                                   "This is a fault in tfg or in this package. Please report it.")
        except BaseException:
            _discard(self.binary, tmp)
            raise
        _emit("wrote", tmp)
        return self._settle(tmp)

    def _settle(self, tmp: str) -> str:
        # Several workers can write the same set at once, and the first to move
        # it wins - the others find the winner whole, take it, and remove their
        # own copy through its manifest.
        for attempt in range(1, _RENAME_TRIES + 1):
            try:
                os.rename(tmp, self.dir)
                _emit("moved", self.dir)
                return self.dir
            except OSError as err:
                if os.path.exists(self.dir):
                    _discard(self.binary, tmp)
                    self._taken()
                    _emit("took", self.dir)
                    return self.dir
                busy = err.errno in _BUSY_ERRNO or getattr(err, "winerror", None) in _BUSY_WINERROR
                if not busy or attempt == _RENAME_TRIES:
                    _discard(self.binary, tmp)
                    raise RuntimeError(f"The set was written, and it could not be moved to {self.dir} - "
                                       f"{err.strerror or err}. Run the tests again. If this repeats, another "
                                       "program may be holding its files open, such as a virus scanner. Set "
                                       "TFG_CACHE_DIR, or the cache_dir argument of file_set, to a directory that "
                                       "program leaves alone.") from err
                time.sleep(attempt * 0.1)
        raise AssertionError("unreachable")

    def _taken(self) -> None:
        # A set this process did not write is checked before it is used:
        # against the plan, and then every byte against the manifest through
        # tfg verify. A test that writes into a file it was given changes the
        # set for every later run, and the size alone does not show it.
        problem = mismatch(self.dir, self.plan)
        if problem:
            raise _stale(self.dir, problem)
        try:
            run(self.binary, ["verify", os.path.join(self.dir, "manifest.json")])
        except TfgError as err:
            if err.status == _EXIT_VERIFY:
                raise _stale(self.dir, "its files are not the ones it was written with", err.stderr.strip()) from err
            raise


def _refused_plan(shown: str, preset: str | None, err: TfgError) -> TfgError:
    """A plan tfg refused, told in the words the test was written in.

    An option tfg does not know brings the whole help of tfg generate, every
    flag behind a dash, where one sentence names the mistake. Everything tfg
    said stays on the error, in stderr.
    """
    unknown = re.search(r"^flag provided but not defined: -+(\S+)", err.stderr, re.M)
    known = [name for name in re.findall(r"^\s+-([a-z][a-z0-9-]*)\b", err.stderr, re.M) if name not in RESERVED]
    if unknown and known:
        help_ = (f"tfg preset show {preset} says what the preset's own options do, and tfg generate --help "
                 "the rest." if preset else "tfg generate --help says what each one does.")
        return TfgError([], err.status, err.stderr,
                        f"{shown} gives the option {unknown.group(1)}, and tfg generate has no flag of that name. "
                        f"Its flags are {', '.join(known)}. Not every one goes with every set, and tfg says why "
                        f"when one does not. {help_}")
    return TfgError([], err.status, err.stderr, f"{shown} could not be planned - tfg {ended(err.status)}.\n"
                                                f"{said(err.stderr)}")


def readable(plan: Any) -> None:
    """Refuses a manifest this package cannot read for what it says.

    A manifest of another version is a RuntimeError, and one of this version
    that has no run or no list of files described by objects a ValueError.
    """
    version = str((plan or {}).get("manifest_version") or "") if isinstance(plan, dict) else ""
    if version.split(".")[0] != MANIFEST_MAJOR:
        raise RuntimeError(f"tfg wrote a manifest of version {version or 'none'}, and this package reads version "
                           f"{MANIFEST_MAJOR}. Install the version of testing-files-generator that comes with that "
                           "tfg, or point TFG_BINARY at a tfg of the same version as this package.")
    files = plan.get("files")
    if not isinstance(plan.get("run"), dict) or not isinstance(files, list) \
            or not all(isinstance(entry, dict) for entry in files):
        raise ValueError("it has no run, or no list of files each described by an object")
    for entry in files:
        outcome = (entry.get("expected") or {}).get("outcome")
        if outcome not in OUTCOMES:
            raise RuntimeError(f"{entry.get('path')} expects {json.dumps(outcome)}, which this package does not "
                               f"know. It knows {', '.join(OUTCOMES)}. Use the version of this package that "
                               "comes with the tfg it runs.")


def mismatch(directory: str, plan: Mapping[str, Any]) -> str:
    """How a written set differs from its plan, or an empty string when it does not.

    It compares names and sizes, which is enough for a set tfg has just
    written. The bytes of a set written earlier are checked through tfg verify.
    Every file has to be on the disk: a run that ends with code nought writes
    every file it plans, until a run can describe a file without writing it on
    purpose - the archive and manifest-only output of D10.
    """
    try:
        with open(os.path.join(directory, "manifest.json"), encoding="utf-8") as f:
            written = json.load(f)
        if not isinstance(written, dict):
            raise ValueError("it is not an object")
    except (OSError, ValueError, RecursionError) as err:
        # RecursionError is JSON nested deeper than the parser goes, which is
        # not a ValueError.
        return f"its manifest cannot be read ({err})"
    run_ = written.get("run") if isinstance(written.get("run"), dict) else {}
    if run_.get("recipe_hash") != plan["run"].get("recipe_hash") or run_.get("seed") != plan["run"]["seed"]:
        return "it was written from another recipe or seed"
    if run_.get("complete") is not True:
        return "the run that wrote it did not finish"
    files = written.get("files")
    if not isinstance(files, list) or len(files) != len(plan["files"]):
        count = len(files) if isinstance(files, list) else "no"
        return f"it holds {count} files where the plan has {len(plan['files'])}"
    for i, (want, got) in enumerate(zip(plan["files"], files), start=1):
        # The manifest of a kept set is on the disk, where a person or a test
        # can change it, so an entry is not taken to be what tfg writes.
        if not isinstance(got, dict):
            return f"its file {i} is not described by an object where the plan has {want['path']}"
        if got.get("path") != want["path"] or got.get("bytes") != want["bytes"]:
            return f"its file {i} is {got.get('path')} of {got.get('bytes')} B where the plan has " \
                   f"{want['path']} of {want['bytes']} B"
        if not got.get("materialized"):
            return f"{got['path']} was not written"
        try:
            size = os.stat(os.path.join(directory, *got["path"].split("/"))).st_size
        except OSError:
            return f"{got['path']} is missing"
        if size != got["bytes"]:
            return f"{got['path']} is {size} B and its manifest says {got['bytes']} B"
    return ""


def _stale(directory: str, problem: str, details: str = "") -> RuntimeError:
    return RuntimeError(f"{directory} is where this set belongs, and it does not hold it - {problem}. Nothing here "
                        "removes that directory, because a person may have changed it on purpose. Delete it and "
                        "run again." + (f"\n{details}" if details else ""))


def _discard(binary: str, directory: str) -> None:
    # Removes a set this process wrote and will not use. The files go through
    # tfg cleanup, which removes only what the manifest lists, and then the
    # manifest and the directories it leaves empty. Whatever is left stays,
    # and a later run says so.
    manifest = os.path.join(directory, "manifest.json")
    if os.path.exists(manifest):
        try:
            code = subprocess.run([binary, "cleanup", manifest, "--yes"], stdin=subprocess.DEVNULL,
                                  stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, creationflags=_FLAGS,
                                  check=False).returncode
        except OSError:
            return
        if code != 0:
            return
        for name in ("manifest.json", "manifest.instructions.md"):
            try:
                os.remove(os.path.join(directory, name))
            except FileNotFoundError:
                pass
    _remove_empty(directory)


def _remove_empty(directory: str) -> None:
    try:
        entries = list(os.scandir(directory))
    except OSError:
        return
    for entry in entries:
        if entry.is_dir(follow_symlinks=False):
            _remove_empty(entry.path)
    try:
        os.rmdir(directory)
    except OSError:
        pass  # not empty: left where it is


def _note_unfinished(cache: str) -> None:
    """Says, once per process and only outside a pytest-xdist worker, which sets a stopped run left behind.

    Nothing reads them and nothing removes them on its own, so without this
    they would fill the disk in silence.
    """
    global _noted
    if _noted or os.environ.get("PYTEST_XDIST_WORKER"):
        return
    _noted = True
    try:
        names = sorted(os.listdir(cache))
    except OSError as err:
        raise _cache_refused(cache, "it could not be read", err) from err
    left = []
    for name in names:
        if not name.startswith(UNFINISHED):
            continue
        pid = name[len(UNFINISHED):].split("-")[0]
        if pid.isdigit() and int(pid) > 0 and not _alive(int(pid)):
            left.append(name)
    if left:
        what = "set was" if len(left) == 1 else "sets were"
        warnings.warn(UnfinishedSetWarning(
            f"testing-files-generator: {len(left)} unfinished {what} left in {cache} by a run that stopped part "
            f"way - {', '.join(left)}. Nothing uses them and they can be deleted."), stacklevel=4)


def _alive(pid: int) -> bool:
    if sys.platform == "win32":
        # Never os.kill here: on Windows it ends the process whatever the signal.
        import ctypes
        from ctypes import wintypes

        kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
        kernel32.OpenProcess.restype = wintypes.HANDLE
        kernel32.OpenProcess.argtypes = (wintypes.DWORD, wintypes.BOOL, wintypes.DWORD)
        kernel32.GetExitCodeProcess.argtypes = (wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD))
        kernel32.CloseHandle.argtypes = (wintypes.HANDLE,)
        handle = kernel32.OpenProcess(0x1000, False, pid)  # PROCESS_QUERY_LIMITED_INFORMATION
        if not handle:
            return ctypes.get_last_error() == 5  # there, and not this user's to ask about
        try:
            code = wintypes.DWORD()
            if not kernel32.GetExitCodeProcess(handle, ctypes.byref(code)):
                return True
            return code.value == 259  # STILL_ACTIVE
        finally:
            kernel32.CloseHandle(handle)
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    except OSError:
        return False
    return True


def _register(names: Iterable[str]) -> None:
    # A marker pytest does not know is a warning, and an error under
    # --strict-markers. A group is known only once a set is planned, so each
    # one is registered as the params that carry it are made.
    if _pytest_config is None:
        return
    for name in names:
        if name not in _registered:
            _pytest_config.addinivalue_line("markers", f"{name}: files of a testing-files-generator set")
            _registered.add(name)


class FileSet:
    """A set of files, or a part of one. Every part shares the set's one write."""

    def __init__(self, source: _Source, files: Iterable[TestFile]):
        self._source = source
        self.files: tuple[TestFile, ...] = tuple(files)

    @property
    def plan(self) -> Mapping[str, Any]:
        """The manifest tfg gave for the plan of the whole set. Nothing in it is written yet."""
        return self._source.plan

    def only(self, *, group: str | Iterable[str] | None = None, target: str | Iterable[str] | None = None,
             outcome: str | Iterable[str] | None = None) -> FileSet:
        """The files that match every argument given. Each takes one name or a list of them."""
        return FileSet(self._source, [f for f in self.files if _matches(f, group, target, outcome)])

    def except_(self, *, group: str | Iterable[str] | None = None, target: str | Iterable[str] | None = None,
                outcome: str | Iterable[str] | None = None) -> FileSet:
        """The files that do not match. The underscore is there because except is a word of Python."""
        return FileSet(self._source, [f for f in self.files if not _matches(f, group, target, outcome)])

    def write(self) -> str:
        """Writes the whole set once per process, or takes a set kept from an earlier run, and gives its directory."""
        return self._source.write()

    def paths(self) -> list[str]:
        """Writes the set if it is not written yet, and gives the absolute paths of this part's files."""
        directory = self._source.write()
        return [os.path.join(directory, *f.relative_path.split("/")) for f in self.files]

    def params(self) -> list[Any]:
        """One pytest.param for each file, for @pytest.mark.parametrize.

        The id is the file's title, the markers are its expected outcome and its
        group, and a file with no right answer is skipped with the reason.
        """
        import pytest

        params = []
        for f in self.files:
            names = f.marks
            own = PYTEST_OWN_MARKS.intersection(names)
            if own:
                raise ValueError(f"The group {f.group} of {f.relative_path} becomes the pytest marker "
                                 f"{sorted(own)[0]}, which is one of pytest's own and changes how the test runs - "
                                 "skip and skipif skip it, xfail hides its failure. Give the group another name "
                                 "in the recipe.")
            _register(names)
            marks = [getattr(pytest.mark, name) for name in names]
            if f.unspecified:
                marks.append(pytest.mark.skip(reason=f.why_unspecified))
            params.append(pytest.param(f, id=f.title, marks=marks))
        return params

    def __iter__(self) -> Iterator[TestFile]:
        return iter(self.files)

    def __len__(self) -> int:
        return len(self.files)


def _matches(file: TestFile, group: Any, target: Any, outcome: Any) -> bool:
    def has(wanted: Any, value: str) -> bool:
        if wanted is None:
            return True
        return value == wanted if isinstance(wanted, str) else value in wanted

    return has(group, file.group) and has(target, file.target) and has(outcome, file.expected["outcome"])
