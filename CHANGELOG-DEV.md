# Technical changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Changes a user of the tool never sees: layer boundaries, guard tests, CI
configuration, architectural decisions. Anything a user would notice goes to
`CHANGELOG.md` instead.

Entries are grouped by the kind of change rather than by the day they were
made. When each change happened is a question git already answers, and a fact
belongs in one place.

## [Unreleased]

### Added

- **`cleanup` joined `audit`, reading the manifest through the same `Claimed`
  as `verify`.** The two cannot drift into disagreeing about which entries
  describe a file that should be on the disk, which for the deleting one is not
  a tidiness question.
  Two decisions worth their reasons. The manifest is not in its own list, so
  removing it by default would be the tool guessing - untouchable rule 7 stays
  literally intact and `--with-manifest` is the explicit ask. And a file that
  was already gone is not counted as left behind, because counting it made the
  second run fail, which is the run a script makes. Both of those were found by
  the guard, not by reasoning.
  The mutation for rule 7 needed rewriting too. The first guard used a manifest
  entry that no other check could have passed anyway, so it proved nothing and
  the mutation stayed green. It now uses an entry the manifest describes with a
  correct hash and deliberately did not write, sitting on top of somebody's
  real file - where which entries are claimed is the only thing standing
  between that file and deletion.
- **The bytes of our own generators are pinned, which nothing was watching.**
  Untouchable rule 3 promises a generated file does not change inside a major
  version, and two guards looked like they covered it without doing so. The
  standard library golden file pins flate, gzip, zip and png - that catches a
  Go upgrade moving the ground under us, and says nothing about our own code.
  The determinism tests compare two runs of one binary, so they stay green when
  every byte of a format changes, because both runs change together.
  Eight values now sit in `testdata/generator-golden.json`: the five formats,
  the label in both positions because it is embedded in the file, an archive
  holding real files of another format, and an archive padded past the comment
  limit into a stored entry. They go through the engine rather than calling a
  generator directly, because the label, the padding and the naming are all
  part of what somebody's hash covers.
  It went in before `contains` rather than after. That work rewrites how an
  archive builds its children, and without this a refactor could have moved the
  bytes of every archive anybody has generated with nothing to notice.
- **`audit` reads a finished run back off the disk, on layer 3 beside the
  engine.** The engine writes files and records what it wrote. This is the
  other half. It went in its own package rather than into `manifest`, which
  owns the schema and has no business walking directories, and rather than into
  `cli`, which would have put the rule about what a manifest claims where the
  window cannot reach it.
  `Claimed` is that rule, in one place. Both commands read the manifest through
  it, so they cannot drift into disagreeing about which entries describe a file
  that should be on the disk.
- **`manifest.Load` checks the major of `manifest_version` before believing
  anything.** A manifest from a future major describes fields this build does
  not know, and acting on the recognised half is how `verify` reports a
  directory sound on the strength of the part it could read. It classifies as
  `IO` rather than `RUNTIME`, because the file came from a person, not from a
  bug in the tool.
- **Every command takes its file before or after the flags.** The flag package
  stops at the first non flag argument, which turned the form written in
  docs/CLI.md - `tfg verify manifest.json --against dir` - into a usage error.
  `generate` already had a private fix for this. It is now shared, and it also
  repaired `tfg recipe fmt recipe.yaml -w`, which had the same hole.
- `130` gained a name, `ExitInterrupted`. It sat in the same frozen table as
  the rest while being written as a bare number at the point of use.
- **`recipe` parses, validates and canonicalises the recipe.** It is the first
  package that reads configuration rather than producing bytes, and the first
  one standing on a dependency.
- **The first external dependency, chosen by measurement rather than by
  reputation: `github.com/goccy/go-yaml`, MIT, zero transitive modules.** The
  stack document never mentioned YAML at all - the dependency map was written
  per file format, while the recipe stands on something that is in neither the
  standard library nor `golang.org/x`. Measured: `go list std` has no YAML
  package.
- **What decided it was blank lines, not comments.** Both candidates carry all
  ten comments of a real recipe through a parse and a write. `gopkg.in/yaml.v3`
  collapses every blank line, which turns a 78 line recipe into a wall of text
  on the first save from the window - the failure the canonical form exists to
  prevent. Both settle after one pass. The probe is in
  `tools/probes/yaml-roundtrip`.
- **The Norway problem does not apply here.** `locale: no`, `label: yes`, `on`,
  `off` and `NO` all arrive as text in both libraries, which implement the YAML
  1.2 core schema. Three real traps did show up instead: `version: 1.0` becomes
  a float, `size: 1_000` becomes 1000, and **`seed: 010` becomes 8** because it
  is read as octal.
- **The YAML library joined the contract surface, which no document had
  foreseen.** The manifest carries `recipe_hash` and that hash is taken from the
  canonical form, so a library upgrade moving one space changes the hash in
  every user's manifest. Same class as the `compress/flate` drift, same remedy:
  a pinned value in `internal/guard`, verified by mutation.
- **A key documented but not implemented is refused in those words.** Reporting
  `policy` as an unknown key would send the reader hunting for a typo that is
  not there, and accepting it would run something other than what was asked.
- CI gained a gate comparing the module list against the expected one, so a
  dependency arriving as somebody's transitive import turns the build red
  instead of slipping in.
- **A target carries a list of sizes rather than a count and one size.**
  `boundary` needed three different sizes under one id, and a range will need a
  different size per file. Carrying a count beside a single size would have
  needed a second way of saying the same thing the moment either arrived, so
  the general form went in instead. `Uniform` builds the common case.
- **Mutation testing is a script now**, `tools/mutate/mutate.py`. It breaks one
  behaviour at a time and checks the named guard turns red. A green suite was
  never the evidence - this is.
- **A stale mutation reports itself.** The runner matches an exact string, so
  gofmt moving a column silently disarmed one entry. It now prints SKIP and
  fails the run rather than counting a mutation nobody applied, and the entries
  aim at lines with no column alignment in them.
- Guards: 56, coverage 77.0%. Twenty of them cover the recipe, and the
  mutation register holds 18 entries.
- `wav` completes the five formats of the first milestone. The one whose size
  follows from the parameters of the signal rather than from content.
- **A measurement decided where the WAV padding goes.** RIFF pads an odd
  length chunk to an even boundary, so every chunk contributes an even number
  of bytes and a strictly padded file is always an even size. That would put
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
- **The registry demands the full declaration from every format.** The
  quality notes had asked for this since before the first line of code and it
  was never written. At twenty five formats the likeliest mistake is one added
  half way - the generator works and it never declared its minimum, its
  padding channel or where its label goes. One test walking the registry costs
  less than a test per declaration and catches exactly that. Five mutations
  turn it red.
- The same test enforces the rule that the registry is the source of a
  confirmed minimum while the format document keeps an approximate table
  beside it: where the document states a minimum for a format that exists, it
  has to be the measured one. The document lives outside the repository, so on
  a fresh checkout that half skips loudly rather than passing quietly. One of
  the five mutations was a document that had drifted from the registry by
  twenty six bytes.
- `zip` is the fourth generator and the first container. Entries are real
  generated files of another format, each valid on its own. Five mutations of
  the ZIP generator all turn the guards red.
- **The two stage padding strategy in its pure form.** ZIP is the only Tier 1
  format whose padding channel has a ceiling, so it is the only one that
  really needs both stages: the archive comment, then a stored entry above it.
- Everything is stored rather than compressed, so the archive size is a
  linear function of its parts and planning computes it exactly. Measured:
  entry overhead is a constant plus twice the name length, data and comment
  both count one for one.
- `entry_size` accepts the same size syntax as --size. Anything else would
  have meant entry_size=200kb failing while size=200kb works.
- Nesting an archive inside an archive is refused for now, because it needs a
  depth limit first and there is none.
- `pdf` is the third generator. Exact size, byte determinism, pages and page
  size as properties, label in the footer and in the document title.
- Fonts are not embedded. Helvetica is one of the fourteen a reader already
  has, which keeps a font file and its licence out of the picture for now.
- **PDF fidelity is declared full but not confirmed in Adobe.** Three engines
  read it. Adobe Acrobat DC is installed and stays a manual checklist item.
- **The price of planning everything up front was measured, not guessed.**
  About 2.3 kB per planned file: 12 MB at the ten thousand file design point,
  4.5 GB at two million. That ceiling is now written down and guarded.
- `png` is the first generator where the output size does not follow from the
  content. Everything before the closing chunk streams straight out while
  being counted, then a padding chunk makes up the difference. Nothing is
  buffered, whatever the picture size.
- **The PNG padding channel was measured before it was built.** Five
  independent decoders - Pillow, FFmpeg, Tcl/Tk, the Windows Imaging Component
  and exiftool - all read an image with an extra chunk between IDAT and IEND
  and return identical pixels. This had been recorded as an open question that
  had to be measured rather than assumed, and it was.
- **A private chunk rather than tEXt.** Both work everywhere, but exiftool
  reports "Text/EXIF chunk(s) found after PNG IDAT" for tEXt. A warning in
  the checking tool is what makes a tester think the generator is broken,
  which is the same cost already recorded for 7Z.
- **PNG has unreachable sizes and that is a property of the format.** The
  smallest is 73 bytes, the next reachable is 83, and 74 to 82 cannot be hit
  because the smallest possible chunk costs 12 bytes. Measured by scanning
  every size from 0 to 400.
- `imagelabel` draws a 3 by 5 bitmap font written here rather than taken from
  a font file. No dependency, no licence to check, and identical pixels on
  every machine. Known limit: at three pixels wide M, N and W stay similar.
- `--set key=value` is repeatable and maps one to one onto the properties
  block of a recipe.
- Measured while building PNG: a gradient compresses to 0.1% of its raw
  size, noise to 75%. A 3840x2160 gradient encodes in about 176 ms, and
  writing it to disk instead of counting it costs nothing measurable - so the
  temp file option would have bought time it does not buy and cost disk space
  it does not have.
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
- The first wave of feature guards covers the exact size as a property over
  118 cases per format, determinism, refusal below the minimum, edit locality
  in all four of its forms, `--dry-run`, and a dropped label staying visible.
  Every one verified by mutation.
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
- At the close of the first milestone: 36 guards, coverage 77.1%, threshold
  70. Every guard is verified by mutation.

### Changed

- **The ZIP archive comment is capped at 4 KB rather than filled to the
  format maximum.** The oracle tests earned their keep on their first run.
  p7zip 17.06 on macOS segfaults reading an archive whose comment is filled to
  the format maximum of 65 535 bytes. 7-Zip 26.02 on Windows and p7zip 23.01
  on Linux read the same file without a word, and the structural checker
  agrees it is well formed - so the crash is a defect in that build rather
  than in the archive. It is still our problem. On a lot of machines 7z means
  p7zip, and a fixture that crashes the archiver a tester reaches for is a
  fixture nobody will trust. Padding through a stored entry reaches every size
  just as exactly, so the enormous comment bought nothing.
- **The oracle tells a crash apart from a rejection.** "Your file is wrong"
  and "the tool fell over" call for different answers, and reporting the
  second as the first would have sent the next reader hunting for a defect in
  our own output.
- **The child generator injection this project designed was not needed.** The
  architecture note had the engine injecting a child producing function into
  the archive generator, to avoid the archive reaching upwards for the
  engine. Reaching for the engine would indeed have broken the layering - but
  an archive needs the registry and another generator, and both sit on its
  own layer. format.Get is a sideways edge that was already allowed. The
  machinery was removed before it was written and the note corrected.
- **The PDF padding channel moved.** It now sits in a comment after the
  trailer and before startxref, where the tail stays short whatever the
  padding size. Measured at 64 B, 4 KB, 100 KB, 1 MB, 2 MB and 10 MB against
  Xpdf, exiftool and the Windows renderer.
- The exact size guard was restated as the rule it actually protects: exact
  or an error, never a different size in silence. It now also walks sizes
  below the declared minimum, so the refusal path is exercised rather than
  assumed.
- **The free space probe is injected.** The first version of its test asked
  for a petabyte, and when the guard was mutated away that test wrote until
  the disk filled - 50 seconds, on a machine with 11 GB free. A guard test
  that damages the machine when the guard breaks is the wrong shape.
- Coverage threshold raised from 40 to 70 after the first review.

### Fixed

- **A byte order mark defeated the strict decoder by the one route strictness
  cannot help with.** It arrived as part of the first key, so `version` was
  refused as an unknown field - the message the decoder gives when it is doing
  its job, pointing at a typo that did not exist. It is dropped before either
  reader sees the bytes, so the parser and the decoder cannot disagree about
  where the file starts. Only a leading mark, and only one: further along the
  file it is somebody's text.
  The first guard for it passed for the wrong reason, and mutation is what
  said so. It asserted an exit code, and stripping every mark in the file
  leaves a recipe that is still perfectly valid - so the exit code noticed
  nothing while the text was being eaten. It now asserts through the formatter,
  where destroyed text is visible.
- **The one document rule counted the wrong thing, and a probe is what found
  it.** It counted raw YAML documents, and this parser makes a document out of
  a comment sitting before a leading `---`, and another out of a trailing
  `---`. Both files hold one recipe and both were refused, with a message about
  separators the reader had not got wrong. It now counts documents that carry
  content. Measured on 14 layouts in `tools/probes/yaml-roundtrip` probe4, and
  the other half of the question - whether the strict decoder then reads the
  recipe or the empty first document - in probe5, seven layouts out of seven.
  Rejected: keeping the raw count, which is what reasoning about the API would
  have left in place. The rule rests on how the library attaches comments,
  which it does not promise, so both probes get re-run on a parser upgrade
  alongside the pinned `recipe_hash`.
- **The formatter and the reader disagreed about what a recipe is, and only
  one of them was checking.** The one document rule lived inside `Parse`, so
  `Canonical` - and with it `tfg recipe fmt` - never saw it. Both now go
  through `oneDocument`, the single place that decides how many recipes a file
  holds, which is what stops the two paths drifting again. The fix was held
  back once because `recipe_hash` is taken from `Canonical`. It was verified
  the way that risk deserves: the pinned `canonicalHash` did not move.
- **A file holding two YAML documents was read as one and the rest dropped.**
  Found by mutation testing rather than by reasoning. A recipe with a `---`
  separator produced the first document only and exited zero, which is the
  silence rule broken in the worst way - somebody gets half the fixtures they
  asked for and nothing says so.
- **A read through the whole code found four ways the tool broke its own
  rules, none of which any test could see.** A name was never checked, so it
  could carry a path and write outside the run - measured, `../../x` landed two
  directories up with exit code 0. The manifest name had the same hole. An
  unknown placeholder stayed in the file name. And the reason behind an
  expectation was read from the recipe and dropped before the manifest. All
  four are silence, which is the one rule this tool cannot break, and all four
  now have a guard and a mutation.
- **Two guards that passed for the wrong reason, both found by mutation.** The
  one for "an invalid recipe writes nothing" used a recipe whose only fault was
  a missing size - which the engine refuses on its own, so the validator could
  have been switched off without the test noticing. And the engine held a loop
  checking every size against the declared minimum, which no test could see,
  because every generator already refuses and a guard walks sizes below the
  minimum for every registered format. The first got a recipe that is runnable
  apart from its expectation. The second was removed.
- **The mutation runner rewrote line endings.** Restoring a file through
  Python's text mode turned every LF into CRLF on Windows, against a repository
  that normalises to LF. gofmt caught it after the first run and it now copies
  bytes rather than text.
- **A defensive branch that no input could reach.** The canonical form added a
  trailing newline when the printer left one out, with a comment claiming the
  printer does not always add one. Mutation showed the branch was never
  executed, and measuring ten inputs - empty, comment only, no closing newline,
  CRLF, two documents - showed the printer always ends with a newline. The
  branch and the false comment are gone.
- **The layer guard walked into `tools/`.** That directory holds internal
  probes and is exempt from every rule that governs shipped code, so judging a
  probe by the layer map of the tool it measures was wrong. It is skipped now,
  next to `docs` and `testdata`.
- **The PDF padding channel this project had written down was wrong, and
  measurement caught it.** The note said "a comment block before %%EOF, no
  limit". The reasoning behind it was sound as far as it went - objects and
  the cross reference table sit earlier, so a comment at the end moves no
  offset. What it missed is that a reader finds the table by scanning
  backwards for startxref, and how far back is up to the reader. Xpdf 4.06
  reads 1004 bytes of comment there and fails at 1005. The Windows renderer
  reads any amount. **That is worse than a hard limit**, because the file
  would open for one tester and not for another.
- This was the third time a claim written from memory has been disproved by a
  few minutes of measurement, after the 7Z tail and the size units.
- **The property test found two defects the moment PDF was registered.** The
  comment splitter could leave exactly one byte over, which cannot be a
  comment line. And the seed test asked for the declared minimum with a label
  on, while the declared minimum is the label free one.
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
- **Mutation found two holes in the guards themselves.** The seed test only
  ran with the label on, and the label carries the seed in its text - so a
  generator could ignore the seed everywhere else and still pass. And no test
  ever asked for a size below the minimum, so the code that refuses could
  have been deleted unnoticed. Both closed.
- **Mutation testing found a real defect, not just proved the tests.** An off
  by one in the padding arithmetic made the writer loop for ever instead of
  failing. A run that hangs cannot be told apart from a very large file, so
  the writer now refuses to continue when a round emits nothing.
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

[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/commits/main
