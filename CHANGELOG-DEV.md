# Technical changelog

Changes a user of the tool never sees: layer boundaries, guard tests, CI
configuration, architectural decisions. Anything a user would notice goes to
`CHANGELOG.md` instead.

## Unreleased

### 2026-08-01 - WAV, and oracles that actually judge

- `wav` completes the five formats of the first milestone. The one whose size
  follows from the parameters of the signal rather than from content.
- **A measurement decided where the padding goes.** RIFF pads an odd length
  chunk to an even boundary, so every chunk contributes an even number of
  bytes and a strictly padded file is always an even size. That would put
  every odd size out of reach - and a boundary set of limit-1, limit, limit+1
  needs consecutive sizes. Putting the JUNK chunk last and leaving off its
  final alignment byte fixes it. Measured on four parsers: the Python wave
  module, FFmpeg, the .NET SoundPlayer and Windows Media Foundation, at odd
  payloads from one byte to a hundred kilobytes. Verified end to end at
  1048575, 1048576 and 1048577 bytes.
- **Oracle tests exist now, and mutation showed why one layer is not enough.**
  Removing the alignment padding from a RIFF chunk changed the bytes, kept
  the size exact, kept the run repeatable, and every guard stayed green.
- Worse, three more format defects walked straight past the reference tools:
  ffprobe ignores a wrong size in the RIFF header, Pillow does not verify the
  checksum of an ancillary PNG chunk, and a PDF reader rebuilds a cross
  reference table whose offset is off by one. Real readers are tolerant on
  purpose.
- So oracles come in two layers. The reference tool answers "would a real
  reader accept this". A structural checker written to the specification, in
  Python, answers "is it well formed". A file has to pass both. With the
  second layer all four defects turn the suite red, with messages naming the
  exact chunk and the exact offset.
- A missing tool skips loudly and the run reports how many were skipped.
- Guards: 34.

### 2026-08-01 - ZIP, and an abstraction that turned out unnecessary

- `zip` is the fourth generator and the first container. Entries are real
  generated files of another format, each valid on its own.
- **The two stage padding strategy in its pure form.** ZIP is the only Tier 1
  format whose padding channel has a ceiling, so it is the only one that
  really needs both stages: the archive comment up to 65 535 bytes, then a
  stored entry above it. Measured: 7-Zip reports no warning at any comment
  size up to the limit.
- Everything is stored rather than compressed, so the archive size is a
  linear function of its parts and planning computes it exactly. Measured:
  entry overhead is a constant plus twice the name length, data and comment
  both count one for one.
- **The child generator injection this project designed was not needed.** The
  architecture note had the engine injecting a child producing function into
  the archive generator, to avoid the archive reaching upwards for the
  engine. Reaching for the engine would indeed have broken the layering - but
  an archive needs the registry and another generator, and both sit on its
  own layer. format.Get is a sideways edge that was already allowed. The
  machinery was removed before it was written and the note corrected.
- `entry_size` accepts the same size syntax as --size. Anything else would
  have meant entry_size=200kb failing while size=200kb works.
- Nesting an archive inside an archive is refused for now, because it needs a
  depth limit first and there is none.
- Guards: 33, all green with four formats registered. Five mutations of the
  ZIP generator all turn them red.

### 2026-08-01 - PDF, and an assumption of ours disproved

- `pdf` is the third generator. Exact size, byte determinism, pages and page
  size as properties, label in the footer and in the document title.
- **The padding channel this project had written down was wrong, and
  measurement caught it.** The note said "a comment block before %%EOF, no
  limit". The reasoning behind it was sound as far as it went - objects and
  the cross reference table sit earlier, so a comment at the end moves no
  offset. What it missed is that a reader finds the table by scanning
  backwards for startxref, and how far back is up to the reader. Xpdf 4.06
  reads 1004 bytes of comment there and fails at 1005. The Windows renderer
  reads any amount.
- **That is worse than a hard limit**, because the file would open for one
  tester and not for another. The channel moved to a comment after the
  trailer and before startxref, where the tail stays short whatever the
  padding size. Measured at 64 B, 4 KB, 100 KB, 1 MB, 2 MB and 10 MB against
  Xpdf, exiftool and the Windows renderer.
- This is the third time a claim written from memory has been disproved by a
  few minutes of measurement, after the 7Z tail and the size units.
- **The property test found two defects the moment PDF was registered.** The
  comment splitter could leave exactly one byte over, which cannot be a
  comment line. And the seed test asked for the declared minimum with a label
  on, while the declared minimum is the label free one.
- Fonts are not embedded. Helvetica is one of the fourteen a reader already
  has, which keeps a font file and its licence out of the picture for now.
- **Fidelity is declared full but not confirmed in Adobe.** Three engines
  read it. Adobe Acrobat DC is installed and stays a manual checklist item.
- Guards: 33, all still green with three formats registered.

### 2026-08-01 - review after the second format

Another read through everything, this time with memory measured rather than
reasoned about.

- **PNG built its padding as one buffer.** Measured: a 600 MiB PNG peaked at
  613 MB while the same size of text peaked at 42 MB. The padding now streams
  in 32 KiB rounds with the checksum fed as it goes, and the same file peaks
  at 13 MB.
- **Unknown properties were accepted in silence.** `--set widht=100` gave a
  file with default dimensions and exit 0, which is the failure the recipe
  document names explicitly. A format now declares the keys it understands
  and the engine refuses anything else.
- **A picture large enough to exhaust memory was accepted.** Measured:
  10000x10000 peaks at 1.17 GB and 20000x20000 at 4.65 GB, because the
  picture is held whole and encoded twice. Capped at 40 megapixels, which
  covers the documented range up to 8K.
- **The price of planning everything up front was measured, not guessed.**
  About 2.3 kB per planned file: 12 MB at the ten thousand file design point,
  4.5 GB at two million. That ceiling is now written down and guarded.
- Guards went from 28 to 33.

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
