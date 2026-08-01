# Testing Files Generator

Generate test files, and know how the system under test should react to them.

Most test file generators hand you a folder of dummy bytes. This one also
writes a manifest that states, per file, whether your application should
accept it, reject it, or sanitise it - and why. That turns a folder into a
test suite.

Runs entirely on your machine. No account, no cloud, no network calls.

> **Status: early development.** There is no working command yet. The
> repository currently holds the layout, the guard tests and the project
> documentation. Nothing here generates a file so far.

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
