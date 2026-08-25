# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Changes that a user of the tool would notice. Written for people who run
`tfg`, not for people who read its source.

A major version bump is also a statement about file hashes. Anything that
changes the bytes of a generated file is listed here as a breaking change,
because it turns other people's test suites red.

## [Unreleased]

### Breaking

- A file name holding `<`, `>`, `"`, `|`, `?`, `*` or a control character is now
  refused while the run is being planned, on every system. Windows will not
  store such a name, so on Windows the file was never written anyway - it was
  reported as one file that could not be produced, at the end, in the operating
  system's words. On Linux and macOS the same recipe wrote the file, which is
  the part that changes: a recipe is meant to mean one thing everywhere, and a
  path separator has been refused on every system for that reason since the
  start. If you want such a name on purpose, that is a test case rather than an
  accident and it belongs inside an archive, where the name survives whatever
  the host filesystem thinks of it.

  Two things get better on Windows as well. `--dry-run` used to report success
  for a run that would then fail, and the refusal now says which character is
  the problem and what to do instead, instead of quoting a system error that
  named an internal temporary file.

- A file named exactly `nul` is refused as well, for a worse reason: on Windows
  that is the null device, so writing to it succeeds and the bytes go nowhere.
  The run was already stopped, but by a check that then told you to remove a
  file you cannot remove. `nul.txt` is an ordinary name and still works, and so
  do `con`, `con.txt`, `prn`, `aux`, `com1` and the rest of the names people
  expect to be reserved - they were each tried on Windows 11 and on Windows
  Server 2025, and every one of them is an ordinary file there now.

### Fixed

- On the batch screen, a refusal about one batch now marks that batch's box. A
  batch asking for a size its format cannot deliver, or a name your system will
  not store, used to stop the run and mark nothing at all - with twenty batches
  on screen there was nothing to say which one to change. Those are the two
  refusals you meet most.
- A bad name for the manifest marks the manifest box rather than nothing. It
  was reported as though it were the name of a file.
- `tfg validate` now checks the name of the manifest, so it stops calling a
  recipe valid that `tfg generate` refuses a second later. If you run validate
  in a pre-commit hook, that is one fewer way for a broken recipe to get past
  it.
- A run whose files would take the name of its own manifest is now refused
  before anything is written. It used to write every file, put one of them where
  the manifest was going, and then stop with "file already exists" - leaving the
  files on disk with no manifest, which means `tfg cleanup` could never remove
  them again. It happened whenever a target produced a file named exactly what
  the manifest is called, including the `manifest.json` a run uses when the
  recipe does not name one. `tfg validate` and `--dry-run` both called such a
  recipe fine, so there was no way to find out before the files were on disk.
  All three now give the same refusal, and it says which target to change.
- `tfg verify` no longer calls a file extra because the manifest spells its path
  the long way round. A manifest listing `./report.txt` for a file called
  `report.txt` used to report `extra report.txt` - one difference rather than
  the pair a real mismatch shows, so it read as a directory somebody had put a
  file into rather than as two spellings of one name. Both spellings were
  already accepted everywhere else in the tool. Nothing about which files are
  looked at changed, only which spellings count as the same name.
- The manifest and `tfg recipe fmt -w` now flush their work to the disk before
  putting it in place. Both are written beside the target and renamed over it,
  and a rename can reach the disk before the bytes do - so a power cut at the
  wrong moment could leave an empty file under the name of your manifest, or of
  the recipe you had just formatted. Generated files are still not flushed, on
  purpose: that is ten thousand of them against one of these.
- A machine readable report that could not be written whole no longer ends with
  a zero exit code. `tfg verify --json | head` on a large report used to hand
  you half a document and say the run was fine, so a script parsing it failed on
  the syntax and blamed itself. A run that had already failed keeps the code it
  failed with - a broken pipe is not why your recipe was wrong.
- A recipe is now refused for being too large however that size is discovered.
  The check used to ask the directory entry before reading, which is a look
  rather than a limit: a file can grow between the look and the read, and
  `tfg recipe fmt` would then have formatted the first megabyte of a longer file
  and reported success. The message and the exit code are unchanged, including
  the size it reports. The same applies to reading a manifest.

### Added

- Keyboard shortcuts: `Ctrl+Enter` generates, `Ctrl+P` previews, `Esc` stops a
  run that is going. They work while you are typing in a box, which is when you
  are most likely to want them, and they do nothing when the button they press
  is out of use - so `Ctrl+Enter` cannot start a second run during the first.
- The keyboard starts on the first field of whichever screen you are on, and
  moves with you between screens. No reaching for the mouse to begin.
- A finished run offers **Open folder**, which opens the directory the files
  actually went into. It appears when there is something to open and goes away
  when the next run starts.
- The window folds away what a format decides for itself and the notes that
  describe the case, so the batch screen fits without scrolling for the first
  time. Both open with a click, and a run refused because of a setting inside
  one of them opens it and marks the box - you never have to go looking.
- A folded section says what is in it, so a value you typed is never hidden
  without a word.

### Changed

- A refusal about a format setting in a recipe now says which target it is
  about, and says what to do instead: `target "photos": width cannot be
  "99999"` rather than `bmp: width cannot be "99999"`. With twenty batches of
  the same format, the old wording did not tell you which one to fix. The
  machine readable report carries the address too, as
  `targets[2].properties.width`.

- The window marks the fields you have to fill in with a red star beside their
  name. Until now the only way to find out that a box could not be left empty
  was to press Generate and read the refusal - and on the batch screen a
  setting the run will not do without looked exactly like one nobody need ever
  touch.
- A box holding a size says what that size comes to, beside the field's name
  and updated as you type: `10mb` shows `10485760 B`. This tool counts in
  1024s, and the count is the only place on the screen that says so without
  being asked.

- A run refused for asking too many files, or for a total too large to measure,
  now says which batch took it over the line. Both limits are about the whole
  run, so the message always was - but the box you can change belongs to one
  batch, and on a form with twenty of them "this run asks for 1000001 files"
  with nothing marked left you to work out which. The window marks it and
  `validate --json` carries `"at": "targets[2].count"`.
- `tfg validate --json` now splits every refusal into `what`, `why` and `fix`,
  the way it already did for the ones the recipe reader produced. Before this, a
  refusal from a format, a preset or the engine arrived as one sentence and a
  script grouping by reason had to take prose apart to do it.
- Twelve refusals the engine produces read slightly differently as a result. The
  punctuation moved and nothing else: a full stop or a colon between what is
  wrong and why becomes a dash, the dash before what to do becomes a full stop,
  and that last part now starts with a capital letter. So `... holds the
  character "<". Windows refuses ... everywhere - take the character out` reads
  `... holds the character "<" - Windows refuses ... everywhere. Take the
  character out`. Nothing was added or removed, and if you match on these
  messages in a script, match on `at` and the three fields instead.
- **Exit code:** a recipe asking for a format setting the format will not take
  now ends with `3` (the recipe is wrong) rather than `4` (the format cannot do
  it). The check moved into the recipe reader so the refusal could name its
  target, and it now arrives with every other problem that recipe has, in one
  report. `--set` on the command line is unaffected and still ends with `4`.
  If your CI compares the exact number, this is the line to read.
- Every box you may leave alone now shows what happens if you do. Two on the
  batch screen said nothing at all - the class of a batch, and the seed - so
  they were indistinguishable from boxes that have to be answered.
- The foot of a form that has more content below it fades into a shadow rather
  than into the page, and carries a small arrow. The old fade was obvious where
  the last thing on screen was text and nearly invisible where it was empty
  space, which is the case where a reader has nothing else to go on.

- The window went through a design review and came out of it looking like one
  program rather than four screens. Nothing it does has changed and no
  generated file is different. What changed is what it looks like:

  - Space now says what belongs together. A field's own name, box and
    explanation sit close, two fields sit further apart, and two sections
    further still. Before this, all three distances were the same and the form
    read as a wall of text.
  - Everything a person reads on a screen starts on one left edge.
  - A finished run is drawn in green and a run that skipped files in amber.
    Until now, "3 files written." was in exactly the same grey as every other
    line on the screen, while a refusal was in red - so the window shouted
    about mistakes and whispered about success.
  - A progress bar at nothing no longer looks like one at everything. The empty
    part is a groove rather than a paler version of the fill.
  - A box switched off during a run no longer looks like an empty box with a
    hint in it. They were the same colour.
  - A box is as wide as what goes in it. A whole number from 1 to 20000 used to
    get a box running the width of the window, and two of those took a row each.
  - Settings a format declares are labelled the way every other field is -
    "Bit depth" rather than `bit_depth`. The key to write in a recipe is on the
    small letter i beside the label, and a refusal about that box now names it
    the way the screen does.
  - A refusal gets the full width of the form to say its four parts in, instead
    of the column its field sits in.
  - The format menu shows what kind of file each format is, so twenty
    three-letter names are not the only thing to go on.
  - The tab you are on is the one that stands out.
  - A form with more below the fold says so, instead of stopping at the window
    edge as though that were the end of it. The single batch screen now fits
    the window it opens at.

- `verify` and `cleanup` are much faster over large runs on Windows. Checking
  that an entry stays inside the output directory used to work the directory
  out again for every single file, and working it out is expensive on Windows -
  more so the deeper the directory sits. It is now worked out once per command.
  Measured on 3000 files of 1 kB: `verify` went from about 17 seconds to about
  0.9 seconds from a deep path, and from about 2.4 seconds to about 0.5 seconds
  from a short one. Linux was already fast and is unchanged.

  Nothing about what the two commands accept or refuse has changed. A file that
  leaves the directory through a link or a junction is still refused, and a
  directory reached through a link still works.

- The answer in the README about slow runs on Windows said the cost was the
  antivirus opening each file. That was wrong, and it is corrected.

- In the window, the form no longer shifts under the pointer when a run has
  more than one line to say. The bar at the foot keeps one height now and a
  longer message scrolls inside it, instead of growing and pushing every field
  upward at the moment you are reading the buttons.

- What a run says now comes above the notes about settings you did not fill in,
  rather than below them. With the bar keeping one height, the first line is
  the one you are sure to see, and "7 files written." is more use there than a
  note explaining a default.

## [0.1.0] - 2026-08-20

Initial release.

[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/donislawdev/TestingFilesGenerator/releases/tag/v0.1.0
