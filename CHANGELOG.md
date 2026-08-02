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

### Added

- **Files of varying size, with `size-range`.** `size-range: 1kb-8kb` in a
  recipe, or `--size-range 1kb-8kb` on the command line, gives every file its
  own size drawn from that range. The draw comes from the seed, so the same
  seed gives the same sizes on any machine and after any number of years, and
  `--dry-run` reports the exact total before anything reaches the disk.
  Raising the count leaves the earlier files untouched, byte for byte. That is
  the same promise every other part of this tool makes, and it is the reason
  sizes are derived from the position of the file rather than read one after
  another.
  A container takes a range too. `size-range` beside `contains` gives archives
  of varying size holding the same files, with the difference padded.
  A range whose low end the format cannot deliver is refused before a single
  size is drawn, so it fails on every run or on none. An error that appeared
  and disappeared depending on the seed would be worse than the mistake it
  reports.
- **`--boundary <limit>` on the command line.** Three files around a limit: one
  byte under it, the limit itself, one byte over. The recipe key already did
  this and the flag did not, so an ad hoc run had to become a file first.

### Added

- **A run says how far it has got.** A large run used to print nothing between
  starting and finishing, so there was no way to tell a working tool from a
  hung one. It now shows the files done, the bytes written and an estimate of
  the time left, updated in place.
  The line goes to the error channel, so `tfg generate recipe.yaml --json | jq`
  still works, and it appears only when that channel is a terminal. Redirected
  into a file or a CI log it stays silent rather than filling the log with
  thousands of redrawn lines.
  Progress arrives while a single file is still being written, not only when
  one finishes, so asking for one very large file reports as it goes rather
  than once at the end.

### Fixed

- **A second run no longer destroys the record of the first.** The manifest was
  not covered by the refusal to write over existing files, so running into a
  directory that already held one replaced it. Everything the earlier manifest
  listed then stopped being anybody's - `tfg cleanup` reported nothing to
  remove and those files could never be cleaned up by this tool again.
  When the file names collided the run at least ended with an error. When they
  did not, it ended with code 0 and said nothing, and the files were lost from
  the record anyway.
  A run into a directory that already holds a manifest is now refused before
  anything is written, with code 5, and the message says what to do. To
  generate alongside an earlier run, give the manifest its own name with
  `output.manifest` in a recipe, or write to another directory with `--out`.

- **`--dry-run` now reaches the same verdict the real run would.** The free
  space check and the collision check both sat behind the point where a dry run
  returned, so `--dry-run` reported success for runs that refuse to start. It
  now ends with code 6 when the run does not fit on the disk and code 5 when a
  name is already taken, and it still writes nothing at all.

- **Asking for help is no longer reported as a mistake.** `tfg generate --help`
  and the same for `validate`, `verify`, `cleanup`, `formats` and `recipe fmt`
  ended with exit code 2, which is the code that means you typed something
  wrong, and printed the text on the error channel. The top level help says to
  run exactly those commands, so the tool was telling you to run something it
  then called a mistake.
  All of them now end with 0 and write to standard output, so `tfg generate
  --help | less` works and a shell script can capture the text.
  Two things deliberately did not change. A mistyped command or flag still ends
  with 2 and still writes to the error channel, because that really is a
  mistake. Running `tfg` with no arguments at all also still ends with 2 - in a
  script that is an oversight and it should stop the build.

- **A number in a recipe now means what it looks like.** YAML decides for itself
  what a bare number means, and its rules are not the ones you apply reading the
  file. Every one of these was wrong, and none of them said a word: `seed: 010`
  ran with seed 8, `count: 010` produced eight files rather than ten, and
  `width: 0100` gave a 100 by 100 image the dimensions 64 by 64. A leading zero
  is what you write numbering runs 001, 002, 003, or keeping a column straight.
  The value changed, the manifest recorded the changed value, and nothing was
  left to notice the mistake by.
  Recipes are now read from the text you wrote and the numbers are parsed in
  base ten, so a leading zero means nothing at all. This covers `version`,
  `seed`, `count`, `size`, `boundary`, `output.split_threshold`, the values
  under `properties`, and `count` and `size` inside a `contains` entry.
  A recipe written in plain decimal produces exactly the bytes it produced
  before. Nothing that was correct has moved.

### Changed

- **A spelling only YAML calls a number is refused instead of guessed at.**
  `seed: 0x10`, `seed: 1_000` and `version: 1.0` end with exit code 3 and a
  message saying what to write instead. Reading them would mean deciding on your
  behalf what your digits meant, which is the behaviour above. A number too
  large for the value it sets is refused for the same reason, rather than
  wrapping into a different run than the one you asked for.

### Added

- **Two more formats: HTML and SVG.** `--format html` produces a complete HTML5
  document - doctype, character set, title and language - with five kinds of
  block in the body: headings, lists, tables, quotes and paragraphs. One quote
  carries an escaped ampersand, so a parser under test meets an entity rather
  than only plain text. `--format svg` produces a drawing with a viewBox and
  four kinds of shape, where lines are stroked and closed shapes are filled -
  a shape painted the wrong way round is invisible, and the size never notices.
  Both hit the size you ask for to the byte, and the filling goes through whole
  blocks and whole shapes. Nothing is ever cut in the middle, so a list, a table
  or a shape is always complete.
  The smallest HTML is 118 B and the smallest SVG is 193 B, which is the
  skeleton plus one whole block. Asking for less says so and names the number.
  **This finishes the text and structured formats.** Twelve of the twenty five
  now work end to end.
- **Three more formats: CSV, JSON and XML.** `--format csv` produces a table
  with a header and six columns, where the description column is quoted and
  carries commas - the case a CSV reader has to get right. `--format json`
  produces an array of records, one per line, and every record carries a value
  of each JSON type, so a parser under test meets numbers, text, booleans,
  null, arrays and nested objects rather than only strings. `--format xml`
  produces a document with an encoding declaration, an attribute on every
  record and names like `Baker &amp; Sons`, so escaping is exercised instead of
  assumed.
  All three hit the size you ask for to the byte, and the filling goes through
  whole records. The last record is built to fit rather than cut short, so
  **every row, object and element is a whole one a parser will accept**. A
  truncated last record looks exactly like a file caught mid write, which is a
  failure that reads as realism unless something checks for it.
  Each of the three has a smallest size, reported when you ask for less: 117 B
  for CSV, 219 B for JSON and 264 B for XML. Those are a header and one row, an
  array and one record, and a declaration with a root and one record. An empty
  table or an empty document is a legal file and a reasonable thing to want,
  and it is coming as a setting rather than as a byte count, because asking for
  `[]` by naming a size is guesswork on both sides.
  **CSV and JSON never carry the label in the file.** A comment row breaks half
  the CSV readers and an extra field changes the very structure under test, so
  the file name and the manifest identify those two instead. `--clean` makes no
  difference to them for the same reason. XML carries it in a comment, out of
  the way of the content, and the manifest says which files carry one.
- **`tfg cleanup --json` and `tfg validate --json`.** Every command that
  produces a report now has a machine readable form, so a pipeline reads a
  result instead of parsing sentences that change when the wording improves.
  The cleanup report says whether it `applied` anything, so a preview and a
  real run cannot be mistaken for each other, and it carries what was found at
  each path. The validate report carries **every problem separately**, each
  with what is wrong, why the rule exists and what to do instead - a recipe
  with five faults arrives as five entries.
- **A stop by signal and Ctrl+C are told apart.** A run cancelled by a person
  ends with code 130 and one stopped by a signal - a CI timeout - ends with
  143, as the exit code table always said. Every stop used to report 130, so a
  job that ran out of time looked like somebody had cancelled it.
- **Two more formats: Markdown and access logs.** `--format md` produces a real
  document - headings, bullet lists, tables, fenced code blocks, block quotes -
  rather than prose with a `.md` on the end, because the structure is what a
  renderer or a converter under test has to cope with. `--format log` produces
  entries in the Apache combined format, with addresses, paths, status codes
  and user agents that look like traffic.
  Both hit the size you ask for to the byte. In a log that means the last entry
  is built to fit rather than cut short, so **every line is a whole entry a
  parser will accept** - a truncated last line looks exactly like a real log
  caught mid rotation, which is the worst kind of broken fixture. In Markdown
  it means structure is only written while a whole block fits and the rest is a
  paragraph, so a file never ends on an unclosed code fence.
  A log has a minimum of 155 bytes, which is one whole entry. Asking for less
  says so and names the number.
- **An archive says what it holds, in the recipe.** `contains` takes a list of
  groups - "3 PDFs of 8 kB and 2 PNGs of 4 kB" is two lines, not five entries -
  and the archive holds real files of those formats, each one valid on its own
  and openable after unpacking. No `size` is needed: the size of the archive
  follows from what is in it, and `--dry-run` reports the exact number before
  anything reaches the disk. State a `size` as well and that wins, with the
  difference padded.
  Contents larger than a stated size is an error naming both numbers, never a
  silent truncation. Asking a format that holds nothing to hold something is an
  error naming the formats that can. Saying what an archive holds twice, once
  with `contains` and once with the `entries` properties, is an error rather
  than one of them being picked.
- **`tfg cleanup manifest.json` removes the files a run produced.** It removes
  what the manifest lists and nothing else. Without `--yes` it deletes nothing
  and prints what it would remove, so you read the list first. A file whose
  content changed since it was written is left alone and reported, because it
  may not be yours - `--force` removes it anyway. Running it twice is not an
  error. The manifest itself is kept unless you pass `--with-manifest`, and
  even then it stays if any file it lists is still on the disk, because it is
  the only record of them. A file left behind reaches the exit code, so a
  script finds out.
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

- **A size one past the largest that fits is refused instead of coming back
  negative.** `--size 9223372036854775808` was accepted and turned into a
  negative byte count on its way to the planner. The check that was meant to
  stop it compared two floating point numbers that had both been rounded to the
  same value, so it never fired. Any size a file can actually have was and is
  unaffected.
- **A recipe with a stray tag marker is reported instead of crashing the tool.**
  A line such as `targets: !` with nothing after the marker made `tfg validate`
  stop with a Go stack trace and exit code 2, which is the code for a mistyped
  command. It now says the file could not be read as YAML, points at the kind of
  marker that does it, and exits 3 like every other unusable recipe.
- **Record numbers no longer skip. This changes the bytes of every CSV, JSON and
  XML file, so it is a breaking change.** The number just before the last record
  was missing - a file of a thousand records ran 1 to 999 and then 1001. Nothing
  else about the file was wrong, which is why it went unnoticed for so long: the
  size was exact, the bytes repeated, and every reader parsed it. A test
  asserting that ids run 1 to N failed against data this tool produced, and the
  fault was ours. Files you keep for comparison need regenerating, and their
  hashes will differ.
- **The label in an SVG can be read. This changes the bytes of every SVG file,
  so it is a breaking change.** The line naming the format, the size and the
  seed shared a baseline with the text that pads the drawing out to the
  requested size, and the padding is written second, so it covered the label.
  Shapes reached the bottom edge and covered it as well. The two lines now sit
  on separate baselines and shapes stay out of the strip they occupy, which
  costs the drawing the lowest 56 of its 600 units. The label was always in the
  file - it just could not be read on screen.
- **Pointing a command at a directory says so.** `tfg verify out/` used to answer
  `read out: Incorrect function.`, which is what Windows says about reading a
  directory and which reads like a fault in this tool. All four commands that
  take a file - `verify`, `cleanup`, `validate` and `recipe fmt` - now say the
  path is a directory and print the command to run instead. The neighbouring
  commands take directories, so this is the mistake worth answering properly.
- **A failure caused by the system is reported in our words, with the number.**
  The sentence an operating system hands back is opaque whatever language it is
  in, and on an install without the English message resource it is not English
  either. A missing recipe now reads `there is nothing at that path (system
  error 2)` rather than whatever the machine chose to say. The number is kept
  because it means the same thing everywhere.
- **A manifest describing no files carries an empty list.** A run stopped before
  its first file finished wrote `"files": null`, while every other empty
  collection in the same manifest was written as `{}`. Anything reading the
  manifest and looping over the entries met a value that was not a list. It is
  `[]` now, at every size including nought.

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
