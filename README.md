# Testing Files Generator

Generate test files, and know how the system under test should react to them.

Most test file generators hand you a folder of dummy bytes. This one also
writes a manifest that states, per file, whether your application should
accept it, reject it, or sanitise it - and why. That turns a folder into a
test suite.

Runs entirely on your machine. No account, no cloud, no network calls.

> **Status: early development.** Thirteen formats work end to end - text,
> Markdown, access logs, CSV, JSON, XML, HTML, SVG, PNG, PDF, ZIP, TAR.GZ and
> WAV. Each one is produced at an exact size and checked against independent
> tools. Runs can come from a recipe file, from flags, or from a named test
> question with `--preset`, and an archive can say what it holds - and what it
> holds is real files of the other formats, not random bytes. `verify` and
> `cleanup` work.
>
> The desktop window opens and shows what this tool is and what its licence
> means for the files you generate. **It cannot generate anything yet** - every
> feature listed above is on the command line only.

## Try it

```
tfg generate --format png --size 2mb --out ./fixtures
```

Or put the run in a file and commit it:

```yaml
# fixtures.yaml - the upload endpoint claims a 1 MB limit
version: 1
seed: 7741

targets:
  - id: edges
    format: wav
    boundary: 1mb          # gives 1mb-1B, 1mb and 1mb+1B
    expected:
      outcome: reject
      reason: size_limit

  - id: invoices
    format: pdf
    count: 25
    size: 300kb
    expected: accept

output:
  dir: ./fixtures
```

```
tfg validate fixtures.yaml     # check it, write nothing
tfg generate fixtures.yaml     # 28 files and a manifest
```

The same recipe and seed give byte identical files on any machine, so the
recipe replaces the fixtures you would otherwise commit.

## What it is meant to do

- **Exact size, to the byte.** Ask for 10 485 761 bytes and get exactly that,
  or get an error explaining why the format cannot go that small. Never a
  silently rounded file.
- **25 file formats** in the highest fidelity we can reach, each one opening
  correctly in its native application.
- **Reproducible.** The same recipe and seed produce byte identical files, so
  a recipe in your repository replaces binary fixtures.
- **A manifest that is a test oracle.** Path, size, hash, format, fidelity,
  seed, tool version - and the declared expectation.
- **Two surfaces, one engine.** Command line for CI, desktop window for
  manual testing. Neither gets a feature the other lacks. The window is at the
  start of that road - see the status note above for where it actually is.

## Building

Requires Go 1.26.5 or newer.

```
go build ./cmd/tfg
```

The command line binary builds without CGO on Windows, macOS and Linux, and it
carries no graphics toolkit and no network stack.

The desktop window is a separate binary:

```
go build ./cmd/tfg-gui
```

That one draws through OpenGL and reaches it through C, so building a binary
with a window in it needs a C compiler and is done natively on each system.
Built without one it still compiles, and says on start that it has no window
in it and that everything is available from the command line.

## Everything inside a generated file is made up

The contents are synthesised from a seed. Names, addresses, e-mail addresses,
IP addresses, timestamps, invoice numbers, log lines and every other value are
produced by an algorithm and describe nobody. No real person, company, account
or system is represented, and any resemblance to a real one is coincidence.

Nothing is copied in from anywhere either. The tool reads no dataset, contacts
no service and embeds no third party content - the vocabulary it draws from is
a short list of ordinary English words written for this project. That is also
why the output is safe to commit: a fixture generated here carries no personal
data, so it does not turn your repository into something a privacy regulation
has an opinion about.

Two things this does not promise. A generated value can collide with a real one
by chance, the same way any random string can, so treat a generated e-mail
address as unusable rather than unused - do not send anything to it. And the
files are built to exercise software, not to look convincing to a reader, so
they are not a substitute for anonymised production data when what you need is
realistic distributions.

## Licence

Copyright (C) 2026 DonislawDev.

Released under the GNU General Public License, version 3. The full text is in
[LICENSE](LICENSE). Run `tfg license` for the short version, including what it
means for the files you generate.

Code from other projects is compiled into the binary. Their licences and
copyright notices are in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).

### The files you generate are yours

Using this tool does not place your project under the GPL. Generated files,
recipes and manifests are **output of the program, not derived works of it**.
The licence covers the code of the tool, not what the tool produces. You can
generate fixtures with it, commit them, ship them and use them in a closed
source product without any obligation.
