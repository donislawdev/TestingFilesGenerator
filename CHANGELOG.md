# Changelog

Changes that a user of the tool would notice. Written for people who run
`tfg`, not for people who read its source. Internal work is recorded
separately in `CHANGELOG-DEV.md`.

A major version bump is also a statement about file hashes. Anything that
changes the bytes of a generated file is listed here as a breaking change,
because it turns other people's test suites red.

## Unreleased

### Added

- `tfg generate` produces **text, PNG, PDF and ZIP files** of an exact size.
  Ask for 10 485 761 bytes and you get exactly that.
- **A ZIP holds real generated files, not random bytes.**
  `--set entries=5 --set entry_format=pdf --set entry_size=200kb` gives an
  archive whose five entries are valid PDFs that open on their own. Checked
  with 7-Zip, Windows Explorer and by extracting and opening the contents.
- PDF takes `--set pages=` and `--set page_size=` (A4, A3, A5, Letter,
  Legal). The label goes in the page footer and in the document title.
  Checked against Xpdf, exiftool and the Windows PDF renderer.
- PNG carries the label burned into the picture, and its size is made exact
  by a private chunk that decoders ignore. Checked against Pillow, FFmpeg,
  Tcl/Tk, the Windows Imaging Component and exiftool - all five read the
  image, the pixels are unchanged and none of them reports a warning.
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

### Safety

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

### Fixed

- **A large PNG no longer needs as much memory as the file it produces.** A
  600 MiB image used to take 613 MB of memory and now takes 13 MB.
- **A misspelled property is an error rather than a shrug.** `--set widht=100`
  used to produce a file with default dimensions and say nothing. It now says
  which property does not exist and which ones do.
- **A picture too large to hold in memory is refused** with a message saying
  so, rather than ending the run with an out of memory error.

### Notes on behaviour

- **Sizes count in 1024s.** `10mb` is 10 485 760 bytes, which is what
  Windows Explorer and `ls -lh` both call "10 MB". `mib`, `kib` and `gib` are
  spelled out versions of the same thing. This departs from the SI standard
  on purpose - a file you asked for as 10mb has to look like 10 MB when you
  check it. Write the number if you want exactly ten million bytes. A size
  that does not land on a whole byte is an error rather than a rounded
  number.
- **`--size` is required.** Every target declares its size, which is what
  lets `--dry-run` report exact numbers before anything reaches the disk.
- **The label needs room.** A text file smaller than the label carries no
  label. The run says so and the manifest records it, so a missing label is
  never a silent difference from what you ordered.
- **The last word can be clipped.** Hitting the exact size wins over ending
  on a word boundary.
- **A few PNG sizes cannot be reached.** The smallest PNG is 73 bytes and the
  next reachable size is 83, because the chunk that makes up any difference
  costs 12 bytes on its own. Anything from 83 bytes upward works. Asking for
  a size in between gets an error naming both neighbours - never a file of a
  different size.
- Text files are UTF-8 with LF line endings. Other encodings and line endings
  are not implemented yet.
