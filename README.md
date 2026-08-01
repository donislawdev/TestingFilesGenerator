# Testing Files Generator

Generate test files, and know how the system under test should react to them.

Most test file generators hand you a folder of dummy bytes. This one also
writes a manifest that states, per file, whether your application should
accept it, reject it, or sanitise it - and why. That turns a folder into a
test suite.

Runs entirely on your machine. No account, no cloud, no network calls.

> **Status: early development.** Five formats work end to end - text, PNG,
> PDF, ZIP and WAV. Each one is produced at an exact size and checked against
> independent tools and its native application. Runs can come from a recipe
> file or from flags. `verify`, `cleanup` and the desktop window are not built.

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
  manual testing. Neither gets a feature the other lacks.

## Building

Requires Go 1.26.5 or newer.

```
go build ./cmd/tfg
```

The command line binary builds without CGO on Windows, macOS and Linux. The
desktop window is a separate binary that needs a C compiler, so it is built
natively on each system.

## Licence

Released under the GNU General Public License, version 3. The full text is in
[LICENSE](LICENSE).

### The files you generate are yours

Using this tool does not place your project under the GPL. Generated files,
recipes and manifests are **output of the program, not derived works of it**.
The licence covers the code of the tool, not what the tool produces. You can
generate fixtures with it, commit them, ship them and use them in a closed
source product without any obligation.
