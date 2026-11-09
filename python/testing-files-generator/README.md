# testing-files-generator

Real files of exact sizes and formats for your tests, and what the system under test should do with each one.

A PDF of exactly 1 MiB and one byte is a real PDF, so a server that looks inside it can only turn it away for its size. A file made with `b"\0" * size` is zeros, so the same server turns it away for what it holds, and a test meant to prove a size limit passes on a server that has none.

## Contents

- [What it is](#what-it-is)
- [Install](#install)
- [Your first test](#your-first-test)
- [The words this package uses](#the-words-this-package-uses)
- [Telling the set what your system did](#telling-the-set-what-your-system-did)
- [Files with no right answer](#files-with-no-right-answer)
- [Choosing the files](#choosing-the-files)
- [When the files are written, and where they are kept](#when-the-files-are-written-and-where-they-are-kept)
- [In CI](#in-ci)
- [Without pytest](#without-pytest)
- [Reference](#reference)
- [When something goes wrong](#when-something-goes-wrong)
- [The tfg program](#the-tfg-program)
- [Licences](#licences)

## What it is

tfg, the Testing Files Generator, is a free and open source program that writes real files for tests: a JPEG that opens in a viewer, a PDF a reader can read, a CSV a spreadsheet imports, each at exactly the size you ask for. Beside the files it writes a manifest, a list that says for every file what it is for and what a system should do with it.

This package runs tfg from your tests:

1. You choose a set of files. Usually that is a preset, a set tfg knows by name that answers one question, such as "does my upload form take what it should and turn the rest away?".
2. The package gives you one test for each file, with an id that says what the file should meet.
3. Each test hands its file to your system the way your system takes files, and ends with one line that tells the set what your system did.
4. When that is not what the file expects, the test fails with a sentence that says what the difference means.

tfg comes with this package, so there is nothing else to install. It works offline and never connects to the network.

## Install

```bash
pip install testing-files-generator
```

- **Python 3.10 or later.** pytest finds the package's plugin on its own. `pip install pytest-testing-files-generator` installs the same thing, under the name pytest's list of plugins knows.
- **The tfg program is inside the wheel pip picks for your system**: Windows on x64 and Arm, Linux on x64 and Arm (glibc and Alpine), macOS on Apple silicon. Nothing runs when the package is installed, and nothing is downloaded when the tests run. The program is the one from the tfg release of the same version: signed on Windows, and on macOS signed and notarised inside the app it was signed in.
- **pip and uv both install it.**
- On any other system pip installs the package without the program, and the first call says so and what to do. See [The tfg program](#the-tfg-program).

## Your first test

Say your app has an upload form at `/upload` that takes one file at a time, has a size limit of `2mb`, and says `Uploaded` when it takes a file. With [pytest-playwright](https://pypi.org/project/pytest-playwright/), this module gives it one test for each file of the `upload-validation` preset:

```python
# tests/test_upload.py
import re

import pytest
from playwright.sync_api import expect
from testing_files_generator import file_set

# 1. The set, planned around the limit your form declares. Nothing is written yet.
#    bulk 0 leaves out the mass upload, which a form taking one file at a time has no use for.
uploads = file_set(preset="upload-validation", options={"limit": "2mb", "bulk": 0})


# 2. One test for each file of the set. A file whose right answer is your own policy
#    is skipped, and the report says why.
@pytest.mark.parametrize("file", uploads.params())
def test_upload(file, page):
    # 3. Upload the file the way a person does. The first call to file.path() writes the set.
    page.goto("/upload")
    page.get_by_label("File").set_input_files(file.path())
    page.get_by_role("button", name="Upload").click()

    # 4. Tell the set what your app did. check fails the test when the file expected something else.
    status = page.get_by_role("status")
    expect(status).to_have_text(re.compile("Uploaded|Refused"))
    file.check("accept" if "Uploaded" in status.text_content() else "reject")
```

Steps 3 and 4 are the only part about your app. Change them to however your app takes a file and shows what it did with it. Keep step 4 waiting for an answer your app gives on purpose, `Uploaded` or `Refused` here, so a page that shows an error fails the test instead of passing as a refusal.

**See the tests before you run them.** Listing them writes none of the files:

```bash
pytest --collect-only -q
```

Each test is named by its file and what the file expects:

```
tests/test_upload.py::test_upload[2mb_under_1b.jpg - expected accept]
tests/test_upload.py::test_upload[2mb_at_limit.jpg - expected accept]
tests/test_upload.py::test_upload[2mb_over_1b.jpg - expected reject (size_limit)]
tests/test_upload.py::test_upload[empty.jpg - expected unspecified (size_zero)]
tests/test_upload.py::test_upload[denied.exe - expected reject (extension_rule)]
tests/test_upload.py::test_upload[pdf_as_jpg.jpg - expected reject (mime_mismatch)]
...
```

**Run them** with `pytest`. The first test that asks for a file writes the whole set. A test fails when your app does something other than what its file expects, and the failure says what that means:

```
ExpectationError: 2mb_at_limit.jpg was turned away, and it should have been taken (size_limit).
A JPEG file of exactly 2mb, the limit itself. Your system should take it. A refusal here usually
means the comparison is off by one, or your system counts a kilobyte as 1000 bytes where this
set counts 1024.
```

A file with no right answer is skipped, never passed, and `pytest -rs` gives the reason, such as `empty.jpg has no expected outcome - what your system does with it is its own policy`. [Files with no right answer](#files-with-no-right-answer) says what to do once you have decided.

**Run part of the set.** Every test carries its expected outcome and its file's group as markers:

```bash
pytest -m reject
pytest -m size-limit
pytest -k "pdf"
```

## The words this package uses

| word | what it means |
|---|---|
| set | the files one `file_set` call describes, each with what it should meet |
| preset | a set tfg knows by name, built around one question. `upload-validation` is one |
| recipe | a YAML file that describes a set file by file. A preset can be turned into one and edited |
| plan | the files of a set and what each expects, known before any of them is written |
| manifest | `manifest.json`, which tfg writes beside a set: every file with its size, its hash and what it expects |
| outcome | what your system should do with a file: `accept`, `reject`, `sanitize` or `unspecified` |
| reason | the rule a file is about, such as `size_limit` or `mime_mismatch`, from a closed list |
| group | the part of a set a file belongs to, such as `size-limit` or `hostile-names` |
| target | one entry of a recipe. A target can make several files |
| purpose | one sentence on what a file is for and what it means when a system gets it wrong. A failure shows it |

The four outcomes:

- `accept` - your system should take the file.
- `reject` - your system should turn it away. A file to be turned away usually names the rule as its reason.
- `sanitize` - your system should take the file and change it, for example store it under a safe name. No preset expects this today, and a recipe can.
- `unspecified` - there is no right answer until you decide one, as for an empty file. These files are skipped.

## Telling the set what your system did

`file.check` compares what your system did with what the file expects, and fails the test when the two differ. It takes one of three words:

```python
file.check("accept")    # your system took the file
file.check("reject")    # it turned the file away
file.check("sanitize")  # it took the file and changed it
```

When your system says why it turned a file away, pass the reason too, and a file turned away for the wrong rule fails as well. Here the same files go straight to the server with [requests](https://pypi.org/project/requests/), and the server answers with a JSON body holding a `reason`:

```python
import pytest
import requests


@pytest.mark.parametrize("file", uploads.params())
def test_upload_api(file):
    with open(file.path(), "rb") as body:
        response = requests.post("http://localhost:3000/upload", files={"file": (file.name, body)})
    assert response.status_code < 500, "a server error is not a refusal"
    file.check("accept" if response.ok else {"outcome": "reject", "reason": response.json().get("reason")})
```

- **A server error is not a refusal.** An answer of 500 or more fails the test before `check`, so a file that breaks your server is found rather than counted as turned away.
- **A reason is compared only for a file that should be turned away, and only when you give one.** A reason is a name from the list in [Reference](#reference), such as `size_limit`. If your system has codes of its own, turn them into those names, or pass the outcome alone.
- **`check` never passes a file with no right answer.** On an `unspecified` file it raises, so such a file cannot pass by accident.
- **What your system did can come from anywhere a test can read**: the text on the page, an HTTP status, the body of a response, a row in a database.
- **The same files can go through the page and straight to the server.** The checks of a page run in the browser and anything other than that page skips them, so a limit only the page enforces shows up only in the tests that go to the server.

## Files with no right answer

Some files have no answer the set could know: whether an empty file should go through, whether `PHOTO.JPG` is the same file as `photo.jpg`, whether a name in Japanese is stored as it is. Their outcome is `unspecified`, and `params()` skips them with the reason in the report. In `filename-handling` nearly every file is like this, because what a system does with a strange name is its own rule.

Once you have decided what your system should do with such a file, write that down, and the file is checked like any other:

1. **Turn the preset into a recipe**, a file of your own to edit and commit:

   ```bash
   tfg preset eject upload-validation --limit 2mb --bulk 0 -o tests/uploads.yaml
   ```

2. **Find the file in the recipe** and change `unspecified` to what your system does. Change its `purpose` too, because that is the sentence a failure shows:

   ```yaml
     - id: empty
       format: txt
       count: 1
       size: 0
       name: empty.jpg
       group: degenerate
       purpose: "An empty file. Our form turns it away, because an empty upload is always a mistake."
       expected:
         outcome: reject
         reason: size_zero
   ```

3. **Use the recipe instead of the preset:**

   ```python
   from pathlib import Path

   uploads = file_set(recipe=Path(__file__).parent / "uploads.yaml")
   ```

`empty.jpg` is then a test like any other, named `empty.jpg - expected reject (size_zero)`, and it fails when your form takes an empty file.

## Choosing the files

### Presets

`file_set(preset=...)` takes one of these:

| preset | the question it answers |
|---|---|
| `upload-validation` | Does my upload form take what it should and turn the rest away? |
| `size-boundaries` | Is a size limit enforced exactly where it is declared? |
| `filename-handling` | Will my system store, show and give back a file name it did not expect? |
| `tabular-import` | Does my table import survive what real tools export? |
| `text-encoding` | Does my reader know which encoding a file is in, or is it guessing? |
| `empty-and-minimal` | Does a file that is valid and as small as the format allows get through? |

`tfg preset show <preset>` lists what a preset takes, what it would write and what it typically catches:

```
$ tfg preset show upload-validation
...
parameters:
  --limit        a size such as 2mb, or a plain byte count, default 10mb
  --allow        format ids separated by commas, default jpg,png,pdf
  --deny         extensions separated by commas, default svg,html,exe,sh
  --far-over     one of: 10x, 2x, off, default 2x
  --bulk         whole number of files from 0 to 10000, default 50
...
```

### Options

Every parameter there is an option of `file_set`, written without the dashes:

```python
file_set(preset="upload-validation", options={"limit": "5mb", "allow": "jpg,png", "far_over": "off", "bulk": 0})
```

- **`far_over` and `far-over` are the same option.** A name with underscores is the flag with hyphens, so `options=dict(far_over="off")` works too.
- **A list is one string with commas**, as `allow` above.
- **Sizes count in 1024s**, so `2mb` is 2 097 152 bytes. If your system means 2 000 000 bytes by 2 MB, give the number of bytes, `"limit": 2000000`, and the files are named after it, such as `2000000_over_1b.jpg`.
- **The same options give the same files**, byte for byte, on every run and every machine, for as long as the major version of tfg stays the same. `"seed": 7` gives other bytes, and those are again the same on every run.
- **Options also take the flags of `tfg generate` itself**, such as `seed` or `clean`. `True` stands for a flag with no value, `"clean": True`, and a list gives a flag that repeats, `"set": ["width=640", "height=480"]`.

### Part of a set

`only` and `except_` take part of a set by `group`, `target` or `outcome`, one name or a list of them. `except_` has its underscore because `except` is a word of Python:

```python
refusals = uploads.only(outcome="reject")
single_files = uploads.except_(group=["bulk-upload", "hostile-names"])
```

Every part shares the set's one write. `paths()` gives the files of a part together, for a form that takes several at once:

```python
uploads = file_set(preset="upload-validation", options={"limit": "2mb"})


def test_the_mass_upload_is_taken_whole(page):
    page.goto("/upload")
    page.get_by_label("Files").set_input_files(uploads.only(group="bulk-upload").paths())
    ...
```

### Your own files

A recipe describes a set of your own, file by file, and the [recipe section of the tfg README](https://github.com/donislawdev/TestingFilesGenerator#-recipes) says how to write one. A relative recipe path is taken from the directory the tests were started in, so `Path(__file__).parent / "uploads.yaml"` names one beside the test module wherever the tests are started from.

A few files of one kind need no recipe:

```python
file_set(options={"format": "png", "size": "5mb", "count": 3, "expected": "accept"})
```

Files made this way expect nothing unless `expected` says what, so without it every one of them is skipped. `tfg formats` lists the formats.

## When the files are written, and where they are kept

```
pytest collects the tests   file_set plans the set: tfg lists the files and writes none of them
the first file.path()       tfg writes the whole set, and the first copy finished is the one kept
every later run             the set is taken from where it was kept, once every byte of it is checked
```

1. **While pytest collects the tests, none of the set is written.** `file_set` asks tfg for the plan, and the tests are made from it. That is why `--collect-only` writes none of the files. The plan comes back through a short-lived file in the directory sets are kept in, so that directory has to be one the tests can write to, even for `--collect-only`.
2. **The first test that asks for a path writes the whole set.** Workers of [pytest-xdist](https://pypi.org/project/pytest-xdist/) that ask at the same moment each write a copy of their own. The first one finished is kept and the others are removed, so for a moment the disk holds a copy for each of those workers. A worker that asks later takes the kept set. The `upload-validation` set at its defaults, 71 files and 115 MB, was written in under a second on the machine it was measured on.
3. **Every later run takes the same files.** Under pytest a set is kept in the run's `.pytest_cache`, which pytest keeps out of git, and `pytest --cache-clear` empties. `TFG_CACHE_DIR`, or the `cache_dir` argument, names another directory. Before a kept set is used, every byte of it is checked against its manifest, which took under a tenth of a second for the same set. A set a test wrote into, or a file someone changed, is refused with the names of the files that differ.

**Do not write into a file the set gave you.** Copy it first. A file changed in place is changed for every later run, and the next one refuses the set.

**To look at the files yourself**, `uploads.write()` gives the directory of the set. It holds the files, `manifest.json` and `manifest.instructions.md`, which says in plain words what each file is for.

**A large set is better written before the tests start**, so the time it takes does not count against the first test:

```python
@pytest.fixture(scope="session", autouse=True)
def written_sets():
    uploads.write()
```

**Nothing is ever removed on its own.** Delete the directory whenever you like, and the next run writes the sets again. A run stopped part way leaves an unfinished set, which nothing uses, and the next run names it in a warning so it does not sit there unseen.

## In CI

- **Nothing to set up.** `pip install` puts tfg in with the package, and the tests write their sets when they run.
- **A fresh checkout has no `.pytest_cache`**, so every CI run writes its sets again. For the `upload-validation` set at its defaults that is under a second on the machine it was measured on. To keep sets from one run to the next, point `TFG_CACHE_DIR` at a directory your CI keeps.
- **Runners on Windows, Linux and macOS on Apple silicon all get a wheel with tfg.** An Intel macOS runner gets none, see [The tfg program](#the-tfg-program).

## Without pytest

`file_set` gives plain data and plain paths, and imports nothing from pytest until `params()` is called. Any runner that can make tests in a loop works the same way. With unittest:

```python
import unittest

import requests
from testing_files_generator import file_set

uploads = file_set(preset="upload-validation", options={"limit": "2mb", "bulk": 0})


class Uploads(unittest.TestCase):
    def test_every_file(self):
        for file in uploads.files:
            with self.subTest(file.title):
                if file.unspecified:
                    self.skipTest(file.why_unspecified)
                with open(file.path(), "rb") as body:
                    response = requests.post("http://localhost:3000/upload", files={"file": (file.name, body)})
                self.assertLess(response.status_code, 500, "a server error is not a refusal")
                file.check("accept" if response.ok else "reject")
```

Outside pytest, sets are kept in a directory of your own: `%LOCALAPPDATA%\testing-files-generator` on Windows, `~/Library/Caches/testing-files-generator` on macOS, and `$XDG_CACHE_HOME/testing-files-generator` or `~/.cache/testing-files-generator` on Linux.

## Reference

### `file_set(preset=None, recipe=None, options=None, cache_dir=None, binary=None)`

Plans a set and gives it back. Nothing is written until a test asks for a file.

| argument | what it is |
|---|---|
| `preset` | a preset by name, as `tfg preset list` lists them |
| `recipe` | the path of a recipe file. A relative path is taken from the directory the tests were started in |
| `options` | a dict of the parameters of the preset and the flags of `tfg generate`, without the dashes |
| `cache_dir` | where sets are kept. By default `TFG_CACHE_DIR`, then the `.pytest_cache` of the run, then a directory of your own as [Without pytest](#without-pytest) says |
| `binary` | the tfg to run. By default `TFG_BINARY`, then the one this package installed |

`file_set` raises at once, while the tests are collected, when the binary or the recipe is not there or tfg refuses the plan. Two calls with the same arguments in one process share one plan and one write.

### A set

| member | what it is |
|---|---|
| `files` | every file of the set, in order. A set also iterates over its files and has a `len` |
| `plan` | the manifest tfg gave for the plan: every file, nothing written yet |
| `params()` | one `pytest.param` for each file, for `@pytest.mark.parametrize`: the title as the id, the expected outcome and the group as markers, and a skip for a file with no right answer |
| `only(...)`, `except_(...)` | part of the set, by `group`, `target` or `outcome` |
| `write()` | writes the whole set, once for the process, and gives its directory |
| `paths()` | writes the set if it is not written yet, and gives the paths of this part's files |

### A file

| member | what it is |
|---|---|
| `title` | the file's path and what it expects, such as `2mb_over_1b.jpg - expected reject (size_limit)`. Unique within a set |
| `marks` | the names of its markers: the expected outcome, and the group with anything other than a letter, digit, underscore or hyphen made a hyphen |
| `unspecified` | `True` for a file with no right answer |
| `why_unspecified` | the sentence a skip shows |
| `path()` | writes the set the first time it is called, and gives this file's absolute path |
| `check(observed)` | `"accept"`, `"reject"`, `"sanitize"` or `{"outcome": ..., "reason": ...}`. Raises `ExpectationError` when it is not what the file expects |
| `name`, `relative_path` | the file's name, and its path within the set |
| `format`, `bytes` | the format, such as `jpg`, and the exact size in bytes |
| `group`, `target`, `purpose` | the file's group, the recipe target that made it, and its purpose |
| `expected` | `{"outcome", "reason", "detail", "confidence"}` as the manifest has it |
| `entry` | the file's whole entry in the manifest |

The reasons a file can name are `content_malformed`, `count_limit`, `dimensions_limit`, `duplicate`, `encoding_invalid`, `extension_rule`, `filename_invalid`, `filename_too_long`, `filename_traversal`, `malware_signature`, `mime_mismatch`, `nesting_depth`, `none`, `size_limit` and `size_zero`.

### Errors

- **`ExpectationError`**, a kind of `AssertionError`, is what `check` raises when your system did something other than what the file expects. `expected` and `observed` hold the two.
- **`TfgError`** is what `file_set`, `write`, `path` and `paths` raise when tfg itself refuses or fails. `status` is the code tfg ended with, or the name of the signal that stopped it, and `stderr` is the end of everything tfg said. The codes are listed in [the tfg README](https://github.com/donislawdev/TestingFilesGenerator), under Using it in CI.
- **`UnfinishedSetWarning`** is the warning about a set a stopped run left behind.

### `binary_path()`

The path of the tfg binary this package runs.

## When something goes wrong

Every message names what it is about and what to do. The ones you are most likely to meet:

| the message starts with | what it means and what to do |
|---|---|
| `... was taken, and it should have been turned away` | Your system did something other than what the file expects. That is the test doing its job, and the next line says what the difference means |
| `file_set(...) could not be planned` | tfg refused what the call asks for. The line under it is tfg's own sentence, which names the value and the way out |
| `file_set(...) gives the option ...` | An option tfg does not have, often a typo. The message lists the flags tfg generate has |
| `file_set was not told which files to write` | The call has no `preset`, `recipe` or `options` |
| `The recipe option names ..., and there is no file there` | A relative recipe path is taken from the directory the tests were started in. Use `Path(__file__).parent / ...` |
| `... is where this set belongs, and it does not hold it` | A kept set was changed, usually by a test writing into one of its files. Delete the directory the message names and run again |
| `... has no expected outcome, so there is nothing to check it against` | `check` was called on a file with no right answer. Skip it, or decide what your system should do and write it in a recipe |
| `This install of testing-files-generator has no tfg binary` | pip installed the wheel without the program, and the message says why, such as a 32-bit Python. Install with a 64-bit Python made for your system |
| `testing-files-generator has no binary for ...` | This system has no wheel with tfg. See [The tfg program](#the-tfg-program) |
| `tfg wrote a manifest of version ...` | `TFG_BINARY` points at a tfg whose manifest this package cannot read. Point it at a tfg of the same version as this package, or leave it unset |
| `tfg could not be started from ...` | The system would not run that file, for the reason after the colon - a tfg for another system, or one without its execute bit. Reinstall the package, or point `TFG_BINARY` at a tfg this user can run |
| `Sets are kept in ..., and it could not be made a directory` | The directory for sets is a file, or this user cannot write where it would go. Set `TFG_CACHE_DIR` or `cache_dir` to a directory you can write |
| `testing-files-generator: 1 unfinished set was left in ...` | A warning. A run stopped part way, and what it left can be deleted |

## The tfg program

`tfg`, and `python -m testing_files_generator`, run the tfg this package installed, with the same arguments, output and exit codes as tfg run directly:

```bash
tfg preset list
tfg formats
tfg generate --help
```

To use a tfg installed another way, set `TFG_BINARY` to its path, or pass `binary` to `file_set`. What decides is the manifest it writes. A tfg whose manifest is of another major version than this package reads is refused, because that manifest can mean something else, and a tfg of the same version as this package never is.

On a system with no wheel of its own, such as an Intel Mac, run the tests in a Linux container, where pip installs the wheel for Linux. A Python made for Intel running under Rosetta on a Mac with Apple silicon is told to use a Python made for Apple silicon.

tfg also writes sets from the command line on its own, has a desktop window, and installs in other ways. All of that is in [the tfg README](https://github.com/donislawdev/TestingFilesGenerator).

## Licences

The code of this package is under the [Mozilla Public License 2.0](LICENSE). The `tfg` program inside its wheels is under the GNU General Public License 3.0 and runs as a separate program. Its licence and the notices of the code built into it are in the wheel's licence files. The files it generates are yours: they are output of the program and not derived works of it, so you can commit them and ship them inside a closed source product with no obligation of any kind.
