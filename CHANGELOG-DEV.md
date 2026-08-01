# Technical changelog

Changes a user of the tool never sees: layer boundaries, guard tests, CI
configuration, architectural decisions. Anything a user would notice goes to
`CHANGELOG.md` instead.

## Unreleased

### 2026-08-01 - the first compressed format

- `png` is the first generator where the output size does not follow from the
  content. Everything before the closing chunk streams straight out while
  being counted, then a padding chunk makes up the difference. Nothing is
  buffered, whatever the picture size.
- **The padding channel was measured before it was built.** Five independent
  decoders - Pillow, FFmpeg, Tcl/Tk, the Windows Imaging Component and
  exiftool - all read an image with an extra chunk between IDAT and IEND and
  return identical pixels. This had been recorded as an open question that
  had to be measured rather than assumed, and it was.
- **A private chunk rather than tEXt.** Both work everywhere, but exiftool
  reports "Text/EXIF chunk(s) found after PNG IDAT" for tEXt. A warning in
  the checking tool is what makes a tester think the generator is broken,
  which is the same cost already recorded for 7Z.
- **PNG has unreachable sizes and that is a property of the format.** The
  smallest is 73 bytes, the next reachable is 83, and 74 to 82 cannot be hit
  because the smallest possible chunk costs 12 bytes. Measured by scanning
  every size from 0 to 400.
- The exact size guard was restated as the rule it actually protects: exact
  or an error, never a different size in silence. It now also walks sizes
  below the declared minimum, so the refusal path is exercised rather than
  assumed.
- **Mutation found two holes in the guards themselves.** The seed test only
  ran with the label on, and the label carries the seed in its text - so a
  generator could ignore the seed everywhere else and still pass. And no test
  ever asked for a size below the minimum, so the code that refuses could
  have been deleted unnoticed. Both closed.
- `imagelabel` draws a 3 by 5 bitmap font written here rather than taken from
  a font file. No dependency, no licence to check, and identical pixels on
  every machine. Known limit: at three pixels wide M, N and W stay similar.
- `--set key=value` is repeatable and maps one to one onto the properties
  block of a recipe.
- Measured while building this: a gradient compresses to 0.1% of its raw
  size, noise to 75%. A 3840x2160 gradient encodes in about 176 ms, and
  writing it to disk instead of counting it costs nothing measurable - so the
  temp file option would have bought time it does not buy and cost disk space
  it does not have.

### 2026-08-01 - review after the first format

A read through everything written so far, before starting the second format.
Seven gaps, two of them dangerous to a user's own files.

- **The tool wrote over existing files without a word.** Measured on a file
  holding real content - it was replaced. Now refused with the IO code.
- **Three files heading for one name gave one file and a manifest describing
  three.** The manifest looked complete and would have reached a test suite
  as a false truth. Now refused while planning.
- **No free space guard at all.** The exit code for it was defined and never
  returned. Now checked before the first byte.
- **No signal handling anywhere.** The context was plumbed through the whole
  engine and nothing ever cancelled it, so Ctrl+C killed the process, left a
  partial file and wrote no manifest.
- Three exit codes disagreed with the frozen table: a count of zero, an
  unrecognised expectation, and a name clash.
- Guards went from 17 to 24, coverage from 43.7% to 77.3%, threshold raised
  from 40 to 70.
- **The free space probe is injected.** The first version of its test asked
  for a petabyte, and when the guard was mutated away that test wrote until
  the disk filled - 50 seconds, on a machine with 11 GB free. A guard test
  that damages the machine when the guard breaks is the wrong shape.

### 2026-08-01 - first format through the whole path

- `core` holds the size arithmetic, the seed derivation and the label
  composition. It knows about no format, which the layer test enforces.
- `format` defines the generator interface. Planning is separate from
  writing, so `--dry-run` is the first half of the real path rather than a
  second path that can drift away from it, and a size a format cannot deliver
  is refused before any file exists.
- The padding channel declaration carries **where** in the stream it sits,
  not only how much it holds. Four Tier 1 formats pad at the front, so an
  interface assuming the end would have to be rewritten at the twelfth
  format.
- `txt` is the first generator. Exact size from 0 bytes upward, byte
  determinism, label on the first line.
- `engine` plans, writes under a temporary name, renames on completion and
  keeps the manifest in step. A run that is cut short still leaves a manifest,
  otherwise cleanup has nothing to work with.
- **Guards grew from 4 to 17.** New ones cover the exact size as a property
  over 118 cases per format, determinism, refusal below the minimum, edit
  locality in all four of its forms, `--dry-run`, and a dropped label staying
  visible. Every one verified by mutation.
- **Mutation testing found a real defect, not just proved the tests.** An off
  by one in the padding arithmetic made the writer loop for ever instead of
  failing. A run that hangs cannot be told apart from a very large file, so
  the writer now refuses to continue when a round emits nothing.

### 2026-08-01 - repository skeleton

- Directory layout and layer boundaries. Packages are empty and carry only
  their package comment. No feature is implemented.
- **Four guards armed before the first line of feature code**, each one
  verified by mutation - broken deliberately and watched turn red:
  - **layering** - nothing imports upwards, and the command line binary can
    never import the window, which would drag in CGO and end cross
    compilation
  - **network isolation** - no `net` or `os/exec` imports in the lower layers
  - **English only** - no non ASCII characters in the command line packages
  - **byte stability** - six standard library paths that produce our bytes,
    pinned against measured values, plus a second test that the same path
    called twice in one process returns the same bytes
- Two guards are partial and say so in place. The ASCII scan does not catch
  another language written without accents - measured, not assumed. The
  import check does not prove that no traffic leaves the machine, and it
  excludes the window because the graphics toolkit brings a dependency tree
  we do not control.
- `LICENSE` holds the canonical GPL-3.0 text, SHA-256 `8ceb4b9e...`, copied
  from a local distribution rather than reproduced from memory. Source and
  copy were compared by hash.
- `.gitignore` was checked by measurement against 21 paths. It caught a bug
  in itself - the `testdata` negation was anchored to the repository root,
  which would have dropped the per package `testdata` directories that Go
  conventionally uses. `internal/engine/testdata/manifest.json` would have
  left the repository silently.
- `.gitattributes` normalises line endings to LF on every system and excludes
  `testdata` and `LICENSE` from any conversion. Without it, git on Windows
  would rewrite files that are compared byte for byte, and the failure would
  look like the compressor drifting rather than like git touching a file.
- Byte stability values were **measured fresh** on go1.26.5 instead of being
  reused from an earlier probe, because the input of that probe was never
  recorded and cannot be reconstructed. The test now defines its own input in
  its own source, so it depends on nothing outside the repository.
- Coverage gate starts at a threshold of zero, held in exactly one file. It
  rises with coverage and is never lowered to turn a red run green.
- CI runs on Windows, macOS and Linux. First run green on all three, which
  also settles something worth recording: **the six standard library paths
  produce identical bytes on all three systems.** Measured by the matrix,
  not assumed. That is evidence for the byte stability promise, although it
  covers the library rather than our own generators, which do not exist yet.
