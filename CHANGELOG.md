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

- Two file names that a Mac stores as one file are now refused while the run is
  being planned, even where the letters are different ones. A name written with
  the German sharp s and the same name written with a double s used to be
  accepted, because Windows and Linux really do keep them apart. macOS does not:
  on a default APFS volume they are one file, so the same recipe wrote two files
  on one machine and one on another, and the manifest described two either way.
  The long s against a plain s, and a typographic ligature against the letters
  inside it, behave the same way there.

  This is the rule that already refuses a path separator and the characters
  Windows will not store, applied to the last case where a recipe still meant
  different things on different machines. The refusal says which two targets are
  involved and what the names have in common, and nothing is written.

  One pair stops being refused, which is the same change facing the other way: a
  dotted capital I and a plain lower case i were treated as one letter and are
  two on every filesystem we measured. If you were working around that, you no
  longer have to.

- A file named exactly `nul` is refused as well, for a worse reason: on Windows
  that is the null device, so writing to it succeeds and the bytes go nowhere.
  The run was already stopped, but by a check that then told you to remove a
  file you cannot remove. `nul.txt` is an ordinary name and still works, and so
  do `con`, `con.txt`, `prn`, `aux`, `com1` and the rest of the names people
  expect to be reserved - they were each tried on Windows 11 and on Windows
  Server 2025, and every one of them is an ordinary file there now.

### Fixed

- `verify` and `cleanup` no longer contradict each other about a file stored
  under a different case. On Windows and on a Mac, `REPORT.TXT` and `report.txt`
  are one file, and `verify` used to call such a file `extra` - the word it uses
  for somebody else's file - while `cleanup`, given the same directory and the
  same manifest, deleted it and reported a clean sweep.

  `verify` now says `respelled` and names what the manifest calls the file, so
  the report says what happened instead of sending you looking for a stranger's
  file. `cleanup` refuses to remove it and ends with the partial exit code,
  because it removes the names the manifest lists and that name is not one of
  them. Rename the file back, or verify against a manifest written for the names
  you have.

  Nothing changes where the filesystem keeps the two spellings apart, as Linux
  does: there they are two files, and both commands always agreed.

- The window no longer slows down as a recipe grows. Every keystroke on the
  batches screen re-reads the whole recipe, and the cost of doing that used to
  rise with the square of its size: a hundred batches took a quarter of a
  second per key, which reads as the window stalling while you type. It now
  takes about a sixtieth of that, and the cost rises in step with the recipe
  rather than ahead of it. Files, hashes and every message are unchanged.

- A run whose manifest could not be written no longer leaves an empty
  `manifest.json` beside the files it wrote.

  The name is taken before the first file, as an empty file, so that two runs
  into one directory cannot both claim it. When the manifest then failed to be
  written - a full disk, a permission, something already sitting under the
  temporary name - that empty claim stayed. `tfg cleanup` and `tfg verify` both
  refused it with "unexpected end of JSON input", so the files it should have
  described could not be removed by the one thing allowed to remove them. Worse,
  the next run into that directory was refused with a sentence saying the file
  "is the only record of what an earlier run wrote" - true every other time it
  is printed, and here about a file that recorded nothing.

  The claim is now given back when the write fails, so the next run is refused
  about a file that really is in the way, and names it. A manifest with anything
  in it is never removed.

- A run that could not save its manifest now says what that leaves behind. The
  message about the manifest was about the manifest, and the problem is the
  files: they are on the disk, nothing records them, and cleanup works from a
  manifest. That is now said in the run rather than discovered later.

- Working out what a run would cost no longer freezes the window, and can now be
  stopped. Pressing Preview or Generate used to work the whole plan out on the
  thread that draws, on the reasoning that planning is fast. It is fast for text:
  measured across formats at two thousand files, a text run plans in about 0.4 s
  and a PNG run in 16 to 23 s, because a picture is encoded while it is planned.
  Ten thousand pictures was a minute and a half of a window that did not redraw,
  with both buttons still looking pressable. It now happens off that thread, with
  Cancel offered while it goes.

- Closing the window during a preview no longer waits for the whole preview. The
  check that a preview runs asks the filesystem about every planned file, twice
  each, and could not be interrupted - so on a large set or a directory on a
  network share, closing the window sat there until it finished.

- An archive or an Office file asked for four gigabytes or more is now refused
  while it is planned. Above that line a ZIP needs extra records to describe
  itself, and the arithmetic this tool uses to work out an archive's size before
  writing it cannot account for them - so the file came out 112 bytes longer than
  planned, which the tool then caught and reported as a fault in itself, after
  writing four gigabytes and removing them. TAR.GZ is unaffected and keeps
  working at those sizes.

- `tfg cleanup --json` now reports counts that add up. A run of four files with
  one already deleted reported three removed, none kept, and four files - because
  the kept count only counted files that were still there and blocked, while the
  list called every file it did not remove "kept". Adding the two numbers lost an
  entry with no way to tell which.

- A WAV asked for more than about four gigabytes is now refused instead of being
  written with a length field that does not match the file. A RIFF file states
  its own length in a four byte field, and nothing checked it, so a request for
  eight gigabytes produced a file of exactly that size whose header announced
  four - and every part of this tool agreed the file was fine. The size was
  right, so the run succeeded, the hash went into the manifest, and `tfg verify`
  called it a match. A file that is broken and certified as sound is worse than
  one that fails loudly, which is why this is a refusal.

  The ceiling is 4294967303 B rather than four gigabytes exactly, and the
  difference is deliberate: the length field counts everything after itself, so
  a file eight bytes over four gigabytes still describes itself correctly.

- A refusal about a size that is too large now says so. BMP, ICO and PNG already
  refused sizes they cannot describe, but all three said "cannot be smaller than
  N B" about a request that was larger - a sentence that contradicts itself and
  offers, as the way out, the very ceiling the request had just passed.

- An archive asked through `contains` to hold more files than the format allows
  is now refused, with the same sentence and the same exit code as asking for
  the same number through the `entries` setting. The two ways of saying it
  disagreed: `entries: 50000` was refused as being outside 0 to 10000, while a
  `contains` list asking for fifty thousand files validated cleanly, passed
  `--dry-run`, and then built a plan for every one of them.

- A run whose plan would not fit in memory is now refused while the plan is
  being built, rather than ending as an out of memory kill with no message.
  There was a ceiling on the number of files, but how much a file costs to plan
  depends on its format - a PDF of a thousand pages costs about six thousand
  times what a text file costs - so `--format pdf --set pages=1000 --count
  10000` passed every check and then asked for around fifty gigabytes before
  writing a byte. The ceiling is now on the memory, which is the thing that runs
  out.

- The `size-boundaries` preset now answers a limit with no room above it the way
  `--boundary` already did. A limit at the largest number there is used to be
  refused with a sentence about a file that "cannot be smaller than nothing",
  and advice to raise the limit - an answer about the bottom of the range to a
  question about the top.

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
- Files whose format writes them in many small pieces are written much faster.
  The worst shape the settings allow - a BMP one pixel wide and twenty thousand
  tall - went from 3.660 s to 0.138 s for sixty files. An ordinary 1 MB BMP is
  about a third faster, plain text and PNG a little. The bytes are identical, so
  nothing that checks a hash sees a change.
- `Ctrl+C` during `tfg verify` now stops while it is still listing the
  directory. On a tree with hundreds of thousands of files it used to finish the
  listing first, which on a slow disk is a long time to keep pressing it.
- The refusal for a boundary limit below 1 B now reads the same from the command
  line and from a recipe. Both took it from their own sentence, and the two had
  already drifted apart by a comma. The wording is the four part shape the rest
  of the tool uses: what is wrong, why, and what to do instead.

### Added

- Every manifest entry now says which target the file came from, under
  `target_id`, and the summary counts the files each target produced under
  `by_target`. Nothing else answered that question. The `id` on an entry is
  numbered across the whole run, so two targets in one recipe produce `f_0001`
  to `f_0005` with nothing marking where one ends. `format` is shared the moment
  two targets ask for one format, `group` is optional, and the file name only
  carries the target while nobody supplies a name template of their own - which
  is exactly when somebody would want to ask. Counting files per target meant
  parsing file names, which is the work a manifest exists to remove.

  `manifest_version` stays at `1.0`. These are added fields, and a reader
  written against `1.0` is unaffected by keys it never looks at.

- The manifest records which Go toolchain built the binary that wrote it, as
  `tool.go`. Alongside `tool.version` and `tool.generators`, this is what makes
  a hash mismatch diagnosable: without it, a hash that moved because somebody
  rebuilt with a newer Go looks exactly like a hash that moved because the
  recipe changed.

  `manifest_version` stays at `1.0`. The schema grows by adding fields, and a
  reader is expected to ignore fields it does not recognise.

- A run large enough that this build could not read its own manifest back now
  says so before it writes anything, and on `--dry-run` too. A manifest is read
  into memory to be compared against a directory, so there is a ceiling on how
  big one may be - and above roughly twenty thousand files a run wrote a
  manifest past it. From that point neither `tfg verify` nor `tfg cleanup` would
  read it, and since the manifest is the only thing that says which files may be
  removed, those files had nothing able to remove them. The run still happens:
  what was missing was being told.

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

- The window no longer offers to put files inside a format that holds none.
  "Add files inside" now appears under ZIP and TAR.GZ, and nowhere else. It used
  to appear under every batch, so a PNG batch carried a button whose only
  destination was a refusal. Rows you already filled in stay on screen if you
  change the format afterwards, so nothing you typed disappears and the refusal
  still has a field to point at.

- Every list of file formats in the window now draws the same small picture
  beside each value. One of the three lists had them and two did not, so the
  same twenty formats looked like different kinds of list depending on which tab
  you were on.

- A setting chosen from a list now opens on its own default instead of on an
  extra entry reading "not stated - pdf". The extra entry existed so that a
  default nobody picked could be told from a value somebody chose. Measured on
  both surfaces, nothing downstream ever read that difference for a setting
  drawn as a list: a run leaving an ICO's embed alone and a run asking for
  embed=bmp produce the same bytes and the same manifest, and the preset block's
  defaulted list is built from a preset's own parameters, which the format is
  not one of. Boxes you type into are unchanged - leaving a preset's limit empty
  still records it as ours rather than yours.

  One visible consequence: a run started from the Presets tab now records the
  format in the manifest's preset parameters, because the screen states it. The
  files are the same.

- Four fields explain themselves better. The format list dropped its second
  sentence, which described the list you were already looking at. "File names"
  and "Batch name" say what they are for and what they change. The seed says
  what 0 means - it is the seed a run uses when nobody asks for another one, and
  it is not a request for random files, which this tool never produces.

- Two refusals are worded differently. Nothing about what is accepted has
  changed - only the sentences.

  The refusal for an empty output directory used to end "or leave it out to use
  the current one". That is true when you write a recipe file, and it is advice
  you cannot take in the window, where leaving the box empty is exactly what was
  just refused. It now says "Name a directory, for example ./fixtures" and stops
  there.

  A size setting given something that is not a size used to be answered with
  different words from the ones shown under the empty field - "a size written
  the way any size is, such as 2mb or a plain byte count" against "a size such
  as 2mb, or a plain byte count". Both now say the second. Every other kind of
  setting already said the same thing in both places.

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
