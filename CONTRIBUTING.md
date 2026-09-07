# Contributing

Thank you for looking. This is a small project with a narrow purpose, so the
most useful thing this file can do is tell you, before you spend an evening on
something, what it will and will not take.

Everything here runs on your own machine. There is no server, no account and no
telemetry, and that is a design decision rather than a stage the project has not
reached yet.

## The quickest ways to help

- **Report what broke.** A file that a real reader would not open, a size that
  came out wrong, a message that sent you the wrong way. The issue forms are at
  [Issues](https://github.com/donislawdev/TestingFilesGenerator/issues/new/choose).
- **Translate the window.** See [Translations](#translations) below. No Go
  needed.
- **Say what a manifest should have told you.** This tool exists to say how a
  system under test ought to react to a file. If it stayed quiet about something
  you had to work out by hand, that is worth an issue.
- **Send code.** See below for what it has to satisfy.

## Building it

Needs Go 1.27.0 or newer, and nothing else for the command line tool.

```
git clone https://github.com/donislawdev/TestingFilesGenerator
cd TestingFilesGenerator
go build -tags "$(cat .github/build-tags)" ./cmd/tfg
go test -tags "$(cat .github/build-tags)" ./...
```

**The build tag is not optional and it is not decoration.** A build without it
does not compile and says why. It turns off an assembly path in the AVIF encoder
that reads past the end of a buffer, and the files produced are the same either
way. Read the tag from the file rather than typing it, because it is the file
the workflows read too.

The desktop window is a second binary, `./cmd/tfg-gui`. It draws through OpenGL
and reaches it through C, so that one needs a C compiler and is built natively
on each system. Built without one it still compiles, and says on start that it
has no window in it and that everything is on the command line.

The test suite takes a few minutes. Most of it is one package, `internal/guard`,
which is where every test that defends a promise of the product lives.

## Translations

The window reads its words from a catalogue, and English is one file in it:
`internal/gui/text/locale/en.json`. To add a language, copy that file to
`<code>.json` next to it, using the language code, and translate the values.

```json
"ButtonCancel": {
  "description": "The words on a button.",
  "other": "Cancel"
}
```

Four things to know before you start:

- **Translate `other`, and leave `description` alone.** The description is a note
  to you about where the words appear, and it is generated from the source.
- **A placeholder such as `{{.Count}}` has to stay spelled exactly that way.**
  It is where a number or a name is put in. You may move it inside the sentence.
- **Plural forms are yours to decide.** English needs `one` and `other`. If your
  language needs `few`, `many` or `zero`, add them beside the English ones. The
  code hands over the number and both English forms, so a language with more
  forms than English is not something you have to work around.
- **Anything you leave out falls back to English**, so a half finished
  catalogue leaves English sentences rather than empty ones.

**One limit, said plainly so you do not waste an evening: the window does not
offer a language switch yet.** A catalogue you contribute is built into the
program and is loaded, and English is still what it answers in, until the
setting that chooses a language lands. If you would rather wait for that, say so
in an issue and it will be weighed as a reason to do it sooner.

The command line is **English only** and stays that way, translations included.
It goes into scripts and CI logs that other people read, and a message that
changes language with the machine it ran on is a message nobody can search for.

## What a change has to satisfy

These are not style preferences. Each one is a promise the tool makes to people
whose test suites depend on it.

- **A file is the size that was asked for, to the byte, or it is an error.**
  Never a size close to it, never a quiet rounding. A batch of ten thousand
  files is not something anybody checks by hand.
- **The same recipe and the same seed produce the same bytes**, on every
  operating system. Anything that moves the bytes of a generated file is a
  breaking change, because it turns other people's test suites red. It is
  allowed, and it needs a note in the changelog and a version bump, and the
  version is bumped by the maintainer rather than in the pull request.
- **Nothing is written over, and nothing is deleted that a manifest does not
  list.** This tool runs in directories that belong to other people.
- **Silence is banned.** A file that was not produced, a name the filesystem
  would not take, a limit the tool invented for you - each of those has to be
  visible in the output and in the manifest. A manifest that quietly skipped ten
  files looks complete and becomes a false result in somebody's test run.
- **No outgoing network connection, ever.** No telemetry, no update check, no
  fetching anything while it runs. A test checks this by asking the compiler
  what the binaries link, so a pull request that adds it will not get past CI.
- **A new dependency needs a reason and a licence check.** The project is
  GPL-3.0, which rules some licences out, and the command line binary currently
  has two dependencies in total. Say in the pull request why the standard
  library will not do.
- **Every change in behaviour comes with a test that fails without it.** A green
  suite is not evidence on its own. The question to answer in the pull request
  is "which test would go red if this change were undone".
- **Words a user reads are English, with a flat hyphen and no semicolons.** That
  covers the README, the changelog, `--help`, every message, and every comment
  in the code. A test enforces the punctuation.
- **An error message says four things**: what happened, why, what value would be
  accepted, and what to do instead.

## Sending a pull request

- Branch from `main` and open the pull request against `main`. Nothing is pushed
  to `main` directly, including by the maintainer.
- Keep it to one subject. A pull request that fixes a bug and tidies three
  unrelated files is one that cannot be reviewed or reverted cleanly.
- CI has to be green. It builds and tests on Windows, Linux and macOS, runs the
  linters, and checks the dependency and supply chain gates.
- The commit message and the pull request are public and permanent. Do not put
  anything in them you would not show a stranger.
- Pull requests are merged by the maintainer, as a squash.

## Security

Do not open an issue for a vulnerability. [SECURITY.md](SECURITY.md) says how to
report one privately and what is in scope.

## Conduct and licence

By taking part you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

Contributions are licensed under **GPL-3.0-or-later**, the same licence as the
rest of the project. Files the tool generates are yours, and carry no licence
from us at all - run `tfg license` and it says so.
