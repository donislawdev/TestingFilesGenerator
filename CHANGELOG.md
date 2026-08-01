# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Changes that a user of the tool would notice. Written for people who run
`tfg`, not for people who read its source. Internal work is recorded
separately in `CHANGELOG-DEV.md`.

A major version bump is also a statement about file hashes. Anything that
changes the bytes of a generated file is listed here as a breaking change,
because it turns other people's test suites red.

## [Unreleased]

### Added

- **`tfg verify manifest.json` checks that a directory still matches what was
  generated.** It reports files that went missing, files nobody asked for, and
  files whose content changed - which is what a backup restore, a storage
  migration or a sync gets wrong. Point it at a restored copy with `--against`,
  or leave that off and it checks the directory the manifest sits in. `--json`
  gives the same report for a script. It ends with code 7 on any difference and
  0 on a full match, so CI can tell the two apart.
  Size and content are reported separately, because a truncated file and an
  edited one point at different causes. A manifest entry for a file the run
  reported as never written is not chased - a run that was honest about failing
  does not then fail verification for it.
- **`tfg generate recipe.yaml` takes its settings from a file.** Put the run in
  a YAML file, commit it, and get the same files back on any machine. The file
  keeps your comments, which is why it is YAML and not JSON.
- **`boundary: 10mb` gives the three files a limit needs** - one byte under,
  exactly on it, and one byte over. That is the case this tool exists for, and
  it is why WAV pads the way it does: a format that could only reach even sizes
  would put two of the three out of reach. Stating a `size` or a `count` beside
  a boundary is refused rather than one of them being picked silently.
- **`tfg recipe fmt` prints a recipe in its settled shape**, comments and blank
  lines kept. `-w` writes it back, and a file already settled is left
  untouched. `--check` prints nothing and ends with code 3 when the file is not
  settled, which suits a pre commit hook.
- **`tfg validate recipe.yaml` checks a recipe and writes nothing**, so it
  suits a pre commit hook. It reports **every** problem at once rather than the
  first one, and a recipe that does not pass produces no files at all.
- **A flag you did not write never overrides the recipe.** Passing `--seed 99`
  wins over the seed in the file. Passing nothing leaves the file alone, so
  adding a flag you never touched cannot change what your recipe produces.
- **The manifest records where the settings came from** - `recipe_hash`
  identifies the recipe, and `overrides` lists every value a flag took away
  from it, with both the old and the new value.
- **A misspelled key in a recipe is an error.** `siez: 10mb` used to be a file
  of the default size and an hour spent wondering why the test passes. The
  message names the line and the column.
- **A key that is documented but not built yet says so in those words**, rather
  than being reported as a typo or accepted and ignored. That covers
  `boundary`, `size-range`, `contains`, `extends`, `policy` and `engine`.
- **A file holding two YAML documents is refused.** It used to produce the
  first one only and report success, so half the fixtures asked for went
  missing without a word.
- `tfg generate` produces **text, PNG, PDF, ZIP and WAV files** of an exact
  size. Ask for 10 485 761 bytes and you get exactly that.
- **Sizes count in 1024s.** `10mb` is 10 485 760 bytes, which is what
  Windows Explorer and `ls -lh` both call "10 MB". `mib`, `kib` and `gib` are
  spelled out versions of the same thing. This departs from the SI standard
  on purpose - a file you asked for as 10mb has to look like 10 MB when you
  check it. Write the number if you want exactly ten million bytes. A size
  that does not land on a whole byte is an error rather than a rounded
  number.
- **`--size` is required.** Every target declares its size, which is what
  lets `--dry-run` report exact numbers before anything reaches the disk.
- Text files are UTF-8 with LF line endings. Other encodings and line endings
  are not implemented yet.
- **The last word can be clipped.** Hitting the exact size wins over ending
  on a word boundary.
- **The label needs room.** A text file smaller than the label carries no
  label. The run says so and the manifest records it, so a missing label is
  never a silent difference from what you ordered.
- PNG carries the label burned into the picture, and its size is made exact
  by a private chunk that decoders ignore. Checked against Pillow, FFmpeg,
  Tcl/Tk, the Windows Imaging Component and exiftool - all five read the
  image, the pixels are unchanged and none of them reports a warning.
- **Every format has a gap in the sizes just above its minimum.** PNG cannot
  reach 74 to 82 bytes and WAV cannot reach 45 to 51, because the chunk that
  makes up any difference costs 12 and 8 bytes of its own. A boundary set
  placed there is refused with both neighbouring sizes named. This is a
  property of the formats, not a limit of the tool - reaching those sizes would
  need a chunk smaller than the format allows.
- **A few PNG sizes cannot be reached.** The smallest PNG is 73 bytes and the
  next reachable size is 83, because the chunk that makes up any difference
  costs 12 bytes on its own. Anything from 83 bytes upward works. Asking for
  a size in between gets an error naming both neighbours - never a file of a
  different size.
- PDF takes `--set pages=` and `--set page_size=` (A4, A3, A5, Letter,
  Legal). The label goes in the page footer and in the document title.
  Checked against Xpdf, exiftool and the Windows PDF renderer.
- **A ZIP holds real generated files, not random bytes.**
  `--set entries=5 --set entry_format=pdf --set entry_size=200kb` gives an
  archive whose five entries are valid PDFs that open on their own. Checked
  with 7-Zip, Windows Explorer and by extracting and opening the contents.
- WAV takes `--set sample_rate=`, `--set bit_depth=`, `--set channels=` and
  `--set content=` (tone, silence, noise, sweep). Odd file sizes work, so a
  boundary set of limit-1, limit and limit+1 gives three real files.
- `--set` sets a format property and can be repeated:
  `--set width=1920 --set height=1080`. Leave it out and the picture size is
  chosen to suit the file size you asked for.
- `tfg formats` lists what this build supports, with the fidelity level, the
  determinism level, the minimum size and the padding channel of each format.
- Every run writes a `manifest.json` next to the files - path, size, SHA-256,
  format, seed, tool version and the declared expectation for each file.
- `--dry-run` counts and shows without writing a single byte.
- `--seed` makes a run repeatable. The same seed gives the same bytes.
- `--clean` turns off the self describing label.
- `--json` writes the manifest to standard output, so it can be piped.
- `--expected` records what the system under test should do with the files:
  accept, reject, sanitize or unspecified.
- **Nothing is written over.** A run that would land on a file already there
  stops and says which one, before writing anything. This tool runs in
  directories that belong to you.
- **Two files cannot head for one name.** A name template with no index and a
  count above one is refused while planning, rather than leaving one file on
  disk and a manifest describing three.
- **A run larger than the free space is refused before the first byte.**
  Finding out at file five thousand of ten thousand leaves a half written set
  and a full disk.
- **Ctrl+C is handled.** An interrupted run finishes or removes the file it
  was writing, keeps everything already completed, and still writes the
  manifest - so nothing incomplete is ever left in the output directory and
  cleanup has something to work with.

### Security

- **A file name can no longer carry a path.** A recipe asking for
  `name: "../../notes.txt"` used to write two directories above the one you
  pointed the run at, and report success. Names now stay inside the output
  directory, and the same applies to the manifest file name. Recipes are meant
  to be shared, so one you were sent could otherwise write where you did not
  expect. Both `/` and `\` are refused on every system, so a recipe that works
  on one machine works on all of them.

### Changed

- **The archive comment stays small.** Filling it to the format maximum of
  65 535 bytes makes p7zip 17.06 on macOS crash while every other archiver
  tested reads the file fine. The padding goes into a stored entry instead,
  which reaches the same sizes exactly, so nothing is lost.

### Fixed

- **A recipe saved by an editor that adds a byte order mark is read.** Notepad
  on Windows adds one by default. The mark used to reach the reader as part of
  the first key, so the tool reported `version` as an unknown field - and the
  mark does not show on screen, which left a message pointing at a typo that
  was not there. `tfg recipe fmt -w` takes the mark off. A mark anywhere else
  in the file is text and is kept.
- **A recipe starting with a comment above `---` is accepted.** A leading `---`
  is ordinary YAML house style, and putting a comment above it used to be read
  as two recipes in one file and turned down. So did a `---` left at the end of
  a file. Both are one recipe and both now run. A file that really does hold
  two recipes is still refused.
- **`tfg recipe fmt` refuses the same files `tfg generate` refuses.** A file
  holding two recipes used to format cleanly and end with code 0, and `-w`
  settled it so `--check` passed as well - after which `tfg generate` turned
  down the file a pre commit hook had just called clean. All three forms now
  end with code 3 and say the file holds more than one recipe, `-w` leaves the
  file exactly as it was, and the message names the file it stopped on.
- **A name template with an unknown placeholder is an error.**
  `name: "file_{index}.txt"` used to produce a file called exactly that, braces
  and all, instead of the numbering asked for. The only placeholder is
  `{index:04}` and the message says so.
- **The reason behind an expectation reaches the manifest.** A recipe declaring
  `outcome: reject` with `reason: size_limit` used to lose the reason on the
  way, so the manifest said what should happen and never why. Reasons are
  checked against the list in the manifest schema, and anything else inside
  `expected` is refused rather than dropped.
- **An empty output directory is refused with a sentence** rather than an
  operating system error, and no longer leaves a manifest behind in the
  directory you happened to be standing in.
- **A large PNG no longer needs as much memory as the file it produces.** A
  600 MiB image used to take 613 MB of memory and now takes 13 MB.
- **A misspelled property is an error rather than a shrug.** `--set widht=100`
  used to produce a file with default dimensions and say nothing. It now says
  which property does not exist and which ones do.
- **A picture too large to hold in memory is refused** with a message saying
  so, rather than ending the run with an out of memory error.

[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/commits/main
