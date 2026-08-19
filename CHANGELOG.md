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

- **The expectation reason `extension_denied` is now `extension_rule`.** A recipe using the
  old spelling is refused, with the list of what is allowed. Rename it in place - nothing
  else about expectations changes.

  A reason says which rule a case is about, not what the system did to the file. That is
  what lets the same reason sit under any outcome: a boundary set expects the file one byte
  under the limit to be **accepted**, and the rule in play is still the size limit. Fourteen
  of the fifteen reasons were named that way and this one was not - it carried a verdict in
  its name, so pairing it with `accept` read as a contradiction while every other reason read
  fine.

### Added
- **`jpg`.** Real JPEG pictures, with the label drawn into the image. `--set quality=1`
  through `--set quality=100` trades detail for a smaller picture, and `--set width=` and
  `--set height=` fix the dimensions. Smallest file: 602 B.

  - **JPEG throws detail away by design**, so the picture that comes back out of a viewer
    is not pixel for pixel the one that went in. That is the format, not this tool. Asking
    for the same size with the same seed still gives you the same file, byte for byte, as
    every other format here does.
  - **The label is readable but not perfectly sharp**, and at a low quality it gets softer.
    Every other format in this tool draws it crisply. If you need a picture whose label is
    exact, use `png`.
  - **Every size from 602 B upward works.** Most formats here cannot produce the handful of
    sizes just above their smallest file - `png` misses eleven of them - because the piece
    they pad with costs more than the difference. This one has no such gap.

- **Three Office formats: `docx`, `xlsx` and `pptx`.** Real packages, not renamed ZIPs:
  each one opens, renders and carries the label in text you can read.

  - **DOCX** holds paragraphs. `--set paragraphs=40` for a longer document. Smallest
    file: 1227 B.
  - **XLSX** holds one sheet. `--set rows=500 --set columns=8` for a grid. Smallest
    file: 1735 B.
  - **PPTX** holds slides. `--set slides=12`. Smallest file: 4826 B - a presentation
    needs a slide master, a layout and a theme whether it shows them or not, which is
    why it costs more than the other two.

  The size you ask for beyond the document itself is carried by an extra part inside
  the package. Every size from the smallest upwards is reachable, with no gaps.

  One thing worth knowing before you compare files: for a workbook and a presentation,
  turning the label OFF with `--clean` makes the file slightly BIGGER, because the
  label takes the place of the filler text in the first cell or on the first slide
  rather than being added to it. In a Word document the label is an extra paragraph
  and `--clean` makes the file smaller, as you would expect.

  These were opened and rendered with LibreOffice 26.2.5.2. **Microsoft Office was not
  available to test against**, so if you use these against Word, Excel or PowerPoint,
  that is the one thing nobody has checked.

### Fixed
- **The window no longer opens a black console window next to it.** Starting `tfg-gui.exe`
  used to give you two windows: the program, and an empty terminal that stayed in the
  taskbar for as long as the program ran and shut the program down if you closed it. Only
  the program opens now.

  `tfg.exe` is unchanged. It still writes to the terminal you run it from, which is what a
  command line tool is for.

- **The window offers a `tfg-out` folder to write into, instead of the folder it was started
  from.** Started by double clicking it, the old default was wherever you had unpacked the
  program - so a run of ten thousand files went straight into that folder, mixed in with
  everything already there and with nothing marking which files were new. Now they go to one
  named folder you can delete in a single go. The field is still yours to change, and the
  folder is only made when a run starts.

  `tfg` on the command line is unchanged and still writes to the directory you are standing
  in, because there you chose that directory by walking to it.

- **The window says where the files will go, without scrolling for it.** The line sits at the
  foot of the window and stays there. The output directory is at the bottom of a form that is
  taller than the window, so it was the one setting deciding where your disk gets written to
  that you could not see before pressing Generate.

- **The form no longer jumps when you press Preview or Generate.** The foot of the window
  used to grow by the height of the progress bar and its message the moment you pressed
  something, which slid the whole form upward - twice per run, at the moment you were
  looking at the buttons. That room is now kept whether or not a run is going.

- **The window opens taller**, 1000 px instead of 900. The forms are still taller than that
  and still scroll. This does not fix that, it makes it shorter.

- **A setting you pick from a list now shows when you have not picked anything.** Lists with a
  default started on that default drawn exactly like a value you had chosen, so there was no
  way to tell "pdf because nobody said otherwise" from "pdf because I chose it" - and once you
  had changed it, no way back to saying nothing. They start on `not stated - pdf` now, and you
  can return to it. Leaving it there records the value in the manifest as defaulted, which is
  the same thing leaving the flag off does on the command line.

- **The line under a list no longer repeats the list.** The `format` setting spelled all twenty
  formats out in a sentence directly under the menu offering the same twenty, and it grew by a
  name every time a format was added. It says what the setting is for. `tfg formats` and the
  refusals on the command line still name every value, because there is no menu to read there.

### Added
- **Three image formats: `bmp`, `gif` and `ico`.** Both surfaces offer them, `tfg formats`
  describes them, and every size they accept is exact to the byte.

  - **BMP** is uncompressed, so the picture is grown to fill the size you asked for
    rather than sitting in a corner of it. Ask for 100 kB and you get a 184 by 185
    picture, not a thumbnail followed by 99 kB of filler. Set `width` or `height` and
    the other side is worked out from what is left. Smallest file: 58 B.
  - **GIF** carries one frame, and its colour table is sized to the picture instead of
    always being the full 256 entries - a full table costs 768 B and would put the
    smallest GIF at 799 B. Smallest file: 41 B. Three sizes just above a bare picture
    cannot be produced at all, because the block the padding lives in costs 3 B empty
    and 5 B carrying anything. Asking for one of them is refused with all three
    reachable sizes named.
  - **ICO** holds one image, and you choose what sits inside it with
    `--set embed=bmp` or `--set embed=png`. A bitmap inside is read by every version of
    Windows. A PNG inside makes a much smaller file and wants Windows Vista or newer.
    Sides go up to 256, which is the format's own ceiling. Smallest file: 70 B.

  All three carry the label burned into the pixels, like PNG. All three were opened by
  hand: BMP in Paint, GIF and ICO in Windows Photos.

### Added
- **Both binaries carry their details, and both wear the icon.** Right click either
  `tfg.exe` or `tfg-gui.exe`, open Properties and the Details tab now names the product,
  the version, the author and the licence. They had none of that, so a tool found on a
  build agent six months later could not be identified from the file alone. `tfg.exe` also
  gets the same icon the window has, instead of the blank placeholder Windows draws for a
  binary it cannot identify.

### Changed
- **Preview and Generate sit in the middle of the bar**, and the Donate button moved
  from above the tabs to the left of that same bar. It is on every screen, including
  About.
- **A little more space between the panels on every screen.** With a panel per batch
  stacked one under another, two of them a hair apart read as one panel with a line
  across it.

### Fixed
- **The window menu is dark, like the rest of the window.** Right clicking the title
  bar brought up Restore, Move, Size, Minimise, Maximise and Close in white against a
  dark program. That menu is drawn by Windows rather than by this tool, and it follows
  a setting separate from the one that darkens a title bar. The program now asks for
  both. Windows only - other desktops draw those menus from their own theme.

### Added
- **A Donate button, top right of the window.** It opens the support page in your
  browser. The tool is free and stays free - this pays for the time that goes into it.

  The program fetches nothing and sends nothing. It hands the address to your desktop,
  on a press you made, and that is all it does.

- **A third screen in the window: several batches in one run.** Until now the window
  produced one batch of files at a time, and everything else a recipe can ask for was
  reachable only from the command line. The new screen holds a list of batches and runs
  them together, with the settings the single batch screen had no room for: a size range,
  a boundary set, a class shared by several batches, a declared expectation and its reason,
  and the files an archive should hold. Beside them sit the settings the whole run shares -
  the output directory, the name of the manifest, the seed, and whether a label goes inside
  each file.

  It reads and writes no recipe file. The batches live in the window and the run is made
  from them, so nothing is opened and nothing is saved.

  A refusal marks the box it came from, in the batch it came from - so two batches both
  asking for a size are told apart, and a run refused for three reasons marks three boxes
  rather than the first.

- **`validate --json` says which setting each problem is about.** Every entry under
  `problems` gains an `at` field naming the setting, as a recipe key with a
  position where a list is involved - `targets[2].size`, `targets[1].contains[1].format`,
  `seed`. A script that groups a refused recipe by field had only the sentence
  before, and the sentence names a target by its id, which a target refused for
  having no id does not have. The field is left out when a problem is about the
  document as a whole rather than one of its settings, so read it as optional.
  The three parts beside it - `what`, `why` and `fix` - are unchanged.

- **The three files of a `--boundary` set say how far from the limit they are.**
  They were `<id>_under_limit`, `<id>_at_limit` and `<id>_over_limit`, which left
  out the number that matters: each is exactly one byte from the limit. They are
  now `<id>_under_1b`, `<id>_at_limit` and `<id>_over_1b`. The `size-boundaries`
  preset has always named the distance, so a flag and a preset answering the same
  question now name their files the same way.
- **`--boundary 15mb` is accepted again and counts in 1024s**, like every other
  size this tool reads. It was refused between 2026-08-03 and 2026-08-18 on the
  grounds that a limit written "15 MB" usually means 15000000 B in the system
  under test - which is the rare case rather than the usual one. The run still
  prints the limit it built around, in bytes, above the three files and before a
  byte is written, so a decimal limit is visible in a dry run.


- **The window says where the files will go, and lets you browse to it.** The
  output directory is written out in full instead of as a dot, and there is a
  Choose button beside it. A dot is clear in a terminal, where you walked into
  the directory yourself. Started from a desktop it means somewhere, and this is
  the part of the tool that writes into your directories.
  The box is still a box, so a path somebody sent you can be pasted straight in.

- **Presets in the window.** A second screen that starts from a question rather
  than from numbers: pick what you are testing, and the set is worked out from
  the answer. It shows the question the preset closes and the mistakes it
  typically finds, and its settings are drawn from the preset itself, the same
  way a format's settings are.
  It produces exactly what `tfg generate --preset` produces - the same files,
  byte for byte, and the same record of the run. When a number was not given and
  ours stood in, the window says so out loud, because a set built around a limit
  we invented carries expectations that read exactly like a set built around
  yours.
  No field arrives filled in any more. What a setting falls back to is shown in
  the box as a hint instead, so leaving a field alone means "I did not state
  this" and the manifest can say which numbers were ours.

- **The window generates files.** Pick a format, say how big and how many, and
  press Generate. It writes the same bytes the command line writes from the same
  settings, because it is the same engine underneath rather than a second one.
  The settings a format accepts are drawn from the format itself, so choosing
  PNG offers width and height, choosing WAV offers sample rate, channels, bit
  depth and content, and each field says what it takes and what it is for.
  Preview sits before Generate and answers the question that costs nothing to
  ask: how many files, how many bytes, and how much room is left on the disk.
  It writes nothing, and it runs the same checks the real run does, so a run it
  says will work is a run that starts.
  Progress shows bytes, percentage and an estimate of the time left. Cancel
  stops the run, and so does closing the window - what was finished stays, the
  manifest describes exactly that, and neither leaves a half written file
  behind.
  A refusal appears in full, with what went wrong, why, what is allowed and
  what to do instead. The words are the engine's own, so the window and the
  command line cannot come to disagree about what they accept.
  The licence notice moved to an About screen, reachable from the generate
  screen and with a way back. The window opens on the work.

- **A window.** `tfg-gui` opens one, and it is a separate binary from `tfg` on
  purpose: the toolkit that draws it needs a C compiler and OpenGL, and the
  command line needs neither. Nothing about `tfg` changes - a server or a build
  agent runs it exactly as before, without the toolkit, without graphics and
  without a network stack.
  The first screen says what the tool is and what its licence means for the
  files you generate. That sentence has been in `tfg license` since it was
  added and there was no way to read it from a window, which is the whole
  reason this screen came first.
  A build made without C support has no window in it at all. It says so, says
  that every feature is on the command line instead, and ends with the code
  that means the fault is not yours.
- **`tfg preset list`, `tfg preset show`, `tfg preset eject` and
  `tfg generate --preset`.** A preset is a named test question. `list` says
  which ones this build answers, `show` says what one takes and what it would
  produce, `eject` prints the recipe it stands for, and `generate --preset`
  writes the files.
  Ejecting is not an export. The recipe it prints is the same bytes a run
  consumes, so ejecting one to a file and running that file produces the same
  files and the same recipe hash in the manifest as running the preset
  directly. There are no closed presets and that is now a fact rather than a
  promise.
  `show` counts its budget by planning the run, at the parameters you gave, so
  the number of files and bytes it prints is what a run would really write.
  `list` and `show` also take `--json`. `eject` does not, because a recipe is
  already something a script can read.
- **`--format` is no longer required when a preset is given.** The preset
  supplies a value for it. Asking for files without saying what to produce now
  names all three ways of saying it - a format, a recipe file, or a preset.
- **The manifest records which preset a run came from, and which of its numbers
  were ours.** Under `run.preset`: the id, the settled parameters, and
  `defaulted` - the parameters nobody gave, where a number of ours stood in.
  Some of those describe your system rather than the files. The size limit of
  an upload form is yours, and a set built around a limit we invented carries
  expectations that read exactly like a set built around the real one. The run
  says so while it runs, and that sentence scrolls away. This is the part that
  stays beside the files.
- **`group` in a recipe target, and in the manifest beside every file it
  produced.** It names the class of case a file belongs to, so a test can
  assert about a whole class at once - "every file in
  extension-content-mismatch was rejected" - instead of naming files one by
  one. The manifest has described the field since it was written and nothing
  filled it. A target that names no group has no `group` in its entries at
  all, rather than an empty one, so "no class" and "a class called nothing"
  stay different.

- **The program has an icon of its own.** A chickpea, so it is findable in a
  taskbar, a window switcher and a folder of files instead of wearing the
  toolkit's logo or the blank placeholder Windows gives a program it cannot
  identify. On Windows those are two different pictures in two different
  places, and both are ours now: the window frame gets one from the toolkit,
  and the taskbar and File Explorer read one compiled into the exe.
  It is drawn from shapes by a script in this project rather than taken from
  anywhere, so nothing here is anybody else's artwork.

### Security

- **Built with Go 1.26.6 instead of 1.26.5.** Five standard library
  vulnerabilities were reachable from the window binary under the older
  toolchain, among them ones in URL parsing, TLS and XML decoding. All are
  fixed in 1.26.6. The command line binary was not affected.
  The bytes of generated files are unchanged: the byte stability guards were
  run under the new toolchain first, and none of them moved. So file hashes
  from an earlier build still match and no test suite of yours turns red.

### Changed

- **Counts are written the way English writes them: "1 file", "7 files".** Every
  line that counts something - files, targets, differences - used to say
  "1 file(s)". The window stopped writing the brackets a day before the command
  line did, so for one day the same run was described two ways depending on
  which of them you read it from.
  This changes sentences on standard output and standard error, so a script
  matching the old wording needs the new one. The machine readable output is
  not affected: `--json` and the manifest carry the same fields with the same
  values, and only the wording of sentences written for a person changed.
  Two messages from `cleanup` were reworded rather than patched. Putting the
  right number on the noun alone would have left "1 file were removed", which
  reads worse than the brackets it replaces.

### Fixed

- **A mistyped `--spread` is your mistake again, not a crash report against
  this tool.** `--spread notasize` ended with the code that means the program
  itself broke, which told a build server to file a bug when somebody had
  simply typed the wrong thing. It now ends with the code that means the value
  is not one this preset takes, and says which value and why.
  Three more shapes of the same flag were wrong and are refused now. The same
  distance twice built a set holding one step twice, and complained about names
  nobody typed. The same distance spelled two ways - `1024` and `1kb` - was
  accepted in silence and built that duplicate step anyway. And a distance
  holding a character a filename cannot carry reached the recipe and broke it,
  because the text of a distance becomes the name of a file. All four were
  found by fuzzing.
- **A refusal about one setting now appears under that setting.** Asking for 0
  files put the sentence explaining it at the foot of the form, below every
  other field - and that distance grew with each field rather than with the
  size of the window. The message now sits under the box that caused it, on
  both screens: the settings of a target and the settings of a preset. Where a
  refusal is about the run rather than about one box, it appears where it
  always did.

- **The form no longer stretches to the width of the window.** Maximised on a
  wide screen every box ran the whole way across - the seed field holding `0`
  was nearly four thousand pixels wide, and a row that long cannot be followed
  from its label to its value. The form now stops at a readable width and stays
  at the left. Nothing wraps that did not wrap before.

- **The self describing label switch says what it is.** It was a bare square
  with its name above it and its explanation below, so there were no words on
  the part you click. The words are on the switch now, which also makes them
  part of what you can click.

- **Messages no longer answer in command line spelling.** A preset that could
  not build its set said "Raise `--limit` above 1051991 B, narrow `--spread`",
  and the window has no such thing as `--limit` on it - the fields there are
  called `limit` and `spread`. Fifteen messages were written that way, and all
  of them can appear in the window, because they are written once and shown by
  both. They now name the settings the way both surfaces do.
  Two of them used to hand you a command to paste. They give the values and the
  setting instead, so an ambiguous `--boundary 15mb` still names both readings
  in bytes and a kept file still says it can be forced.

- **Cancel appears when there is a run to cancel, and not before.** It used to
  sit permanently beside Preview and Generate, greyed out, so a row of two
  choices read as three.

- **Hovering a button no longer washes its colour out.** The hover shade was
  being painted over the button instead of blended into it, so pointing at
  Generate turned it grey.

- **The window is grouped, not a single long form.** Every screen was one column
  of fields with nothing separating one part from another, and the explanations
  were the same size as the values they explain, so the words outweighed the
  controls. Settings now sit in named sections, fields that are read together
  sit side by side, and an explanation is smaller and quieter than the field it
  belongs to.
  Preview, Generate and Cancel stay in place while the form scrolls, so
  producing files no longer means scrolling to the end to find the button. The
  three of them look like what they are: Generate does the work, Preview is
  beside it, Cancel waits until there is something to cancel.

- **Moving between the two screens is tabs across the top.** The way to the
  other screen was a button in the row of actions under the last field, so
  changing screen meant scrolling past the whole form to find it. It also read
  as something you do to the form, sitting between Preview and Generate, which
  is not what it is. The licence moved up there too, and no longer needs a Back
  button - a tab is its own way out.

- **Starting the window no longer prints a warning about itself.** Every start
  wrote three lines saying this application has not been migrated to the
  toolkit's threading model. It had been - everything that touches the window
  from a background task goes through the toolkit's own handoff, and there is a
  check that keeps it that way. The application simply never said so.

- **The window has its own colours.** Until now it took whatever the toolkit
  shipped, and one thing that cost was the focus ring - the outline saying
  which box the keyboard is in was very nearly the colour of the page behind
  it. The colours are worked out against readability thresholds rather than
  picked.
  It is one look, dark, whatever the desktop is set to.

- **Hints in the window are no longer slanted, and read as quieter than the
  labels above them.** Every line of explanation under every field was italic,
  on both screens. Slanted text is harder to read, and hardest for the people
  who already find reading hardest. The words are unchanged.

- **Every file of a `size-boundaries` set now says which limit it was built
  around.** The files were named after their distance from the limit and never
  after the limit itself, so `at_limit.pdf` from a `--limit 10mb` run and
  `at_limit.pdf` from a `--limit 5mb` run were one name for two different
  claims. A directory holding both runs was a directory of guesses, and reading
  the sizes off the disk was the only way to tell which file was which. They
  are now called `10mb_at_limit.pdf`, `10mb_under_1kb.pdf` and so on.
  The files themselves are unchanged, byte for byte. Only the names are
  different, so a script that opens one by name needs the new name.
  A limit whose text a filename cannot carry is now refused with a sentence
  saying so, rather than reaching the recipe.
- **`tfg validate` no longer refuses a recipe that `tfg generate` accepts.** A
  boundary set names its three files `under_limit`, `at_limit` and
  `over_limit`, and `validate` was checking for collisions against `0001`,
  `0002` and `0003` instead. A recipe holding a boundary set beside a target
  named `<id>_0001.<ext>` was rejected while the run it described was fine.
  This is the command meant to sit in a pre-commit hook, where a false alarm
  blocks work that was never wrong.

### Changed

- **BREAKING: the files inside a ZIP or a TAR.GZ now default to 8 kB, not 4 kB.**
  That is the size `tfg formats zip` has always printed. The declaration said
  8kb and the generator used 4096, so reading the tool and running it gave two
  different answers, and the one people read is the one they plan around. Only
  archives generated without `--set entry_size=` change, they hold the same
  number of files and those files are bigger, and the archive itself is still
  exactly the size that was asked for. Say `--set entry_size=4kb` to keep what
  you had.

### Fixed

- **Two targets whose names differ only in how an accent is spelled are now
  refused.** An accented letter can be written as one character or as the plain
  letter followed by its accent. The two print identically and are different
  bytes, and macOS stores both under one name. A recipe asking for both used to
  end with 0, leave one file on the disk, and write a manifest describing two -
  and `tfg verify` then failed on that same output. A pair like this is refused
  on every system, the way names differing only in case already were, because a
  recipe is meant to travel between machines and one that loses a file on
  somebody else's is worse than one refused on both. An accented name on its
  own is ordinary input and keeps working.

### Added

- **TAR.GZ, the thirteenth format.** `tfg generate --format targz --size 3mb`
  gives an archive of exactly that many bytes. It holds real generated files of
  other formats rather than random bytes, the same way ZIP does, through
  `contains` in a recipe or through `--set entries=`, `--set entry_format=` and
  `--set entry_size=`. Entries are written in the ustar format with a fixed
  timestamp, so the same recipe and seed give the same archive everywhere.
  The smallest archive is 1052 B.

  Nothing inside is compressed. That is what lets the size be worked out before
  a byte is written, so `--dry-run` stays cheap and the number it prints is the
  number you get. The cost is honest and worth knowing up front: a `.tar.gz`
  from this build does not shrink anything, and `tfg formats targz` does not
  offer a compression level because there is none to choose.

  One size cannot be produced without the self describing label: exactly one
  byte above the smallest archive. The gzip comment that would make up the
  difference cannot be one byte long. With the label on, which is the default,
  every size from the smallest upward works.
- **`tfg license`.** Prints the copyright line, the licence, the absence of a
  warranty, and the one thing somebody actually needs to know before putting a
  generator into a closed source product: the licence covers this tool and not
  the files it produces. `tfg licence` and the flag spellings do the same thing. It is a
  question, so it answers on standard output and ends with 0.
- **README says what is inside a generated file.** Everything is synthesised
  from a seed and describes nobody, nothing is copied in from any dataset, and
  a generated fixture carries no personal data. It also says the two things
  that are not promised.
- **`THIRD-PARTY-NOTICES.md`.** The tool links code from the Go standard
  library, `github.com/goccy/go-yaml` and `golang.org/x/text`. Their licences
  ask that their copyright notices travel with any copy, a built binary
  included, so the notices now live in the repository and belong with any
  release. Every one of them is compatible with the GPL. Building from source
  is unaffected.

### Changed

- **A second dependency, `golang.org/x/text`, is now part of the build.** It
  supplies the Unicode normalisation behind the name check above. It is BSD
  licensed, and it adds about 131 KB to the binary. Nothing about the bytes of
  a generated file changes.

### Security

- **`cleanup` and `verify` no longer follow a link out of the directory.** A
  path such as `"jn/file.txt"` contains no climb, so it passed every reading of
  the text - and a junction or symbolic link called `jn` inside the output
  directory took it somewhere else entirely. `tfg cleanup --yes --force`
  removed the file it landed on and ended with 0. Where a path leads is now
  settled after the links have been followed rather than by reading it. A link
  is not refused for being one: a directory that is itself a link keeps
  working, which is an ordinary way to keep fixtures on another disk.
- **`cleanup` and `verify` no longer act on a manifest whose entries point
  outside the directory.** An entry such as `"path": "../notes.txt"` used to be
  resolved against the output directory and followed. `tfg cleanup --yes
  --force` removed the file it landed on, reported it as removed from the
  output directory, and ended with 0. A manifest travels with a fixture set and
  can arrive from anywhere, so it is now checked when it is read: a manifest
  with an entry that leaves the directory is refused whole, with the entry
  named, and nothing is read or removed. Paths naming a subdirectory are
  unaffected.

### Fixed

- **`tfg recipe fmt -w` no longer replaces a recipe with something it cannot
  read.** The formatter prints the file back through a library whose printer is
  not faithful for everything its parser accepts, and it never checked the
  result. Following the tool's own instructions destroyed the file: `--check`
  said "not in its settled shape, run -w", `-w` rewrote it and reported
  success, and every command afterwards refused the file as unreadable. The
  settled shape is now read back and settled again before it is handed over, so
  a file that cannot survive the round trip is left exactly as it was.
- **`verify` says what an unfinished file is instead of calling it unexpected.**
  A run killed outright leaves the file it was writing behind, because the name
  it is renamed from is removed by code and no code runs after a kill. Measured
  over three killed runs, one was left every time. `verify` reported it as an
  ordinary extra file, which says nothing about a file this tool wrote itself
  and which `cleanup` will never remove - it removes only what the manifest
  lists. It is now named as an unfinished file from an interrupted run, with
  what to do about it. Files somebody else put there are still extra.
- **A count larger than this build can plan is refused instead of crashing.**
  `--count 9223372036854775807` used to end in a Go stack trace under exit code
  2, which means a mistyped flag, and the same count in a recipe tried to
  allocate 13 GB before the system stopped it. A run now plans at most 1000000
  files and says so with exit code 3. That is a hundred times the largest
  preset.
- **A total size that does not fit is refused instead of wrapping.** Sizes that
  added up past what a byte count can hold came out negative, were reported as
  a negative total, and satisfied the free space check - so a run that could
  never fit started writing. The same wrap could reach `--boundary` through the
  file one byte above the limit.
- **A manifest is never written over, even by a run that started at the same
  time.** Two runs into one directory both ended with 0 and one manifest
  quietly replaced the other, leaving the files it described with nothing able
  to remove them. The name is now claimed rather than checked in advance. The
  manifest is also written through a temporary file, so a run that is
  interrupted mid write no longer leaves an unreadable one.
- **A manifest too large to read is refused before it is read**, the same way a
  recipe already was.
- **`--dry-run` on an archive no longer generates everything the archive
  holds.** Measuring an archive meant building it, which meant running every
  generator inside it two or three times. A dry run of a 256 MB archive took
  960 ms against 56 ms for a plain file of the same size, and one with large
  declared contents did not finish at all. It is now 57 ms, and the bytes of
  every archive are unchanged.

### Added

- **`--expected-reason` on the command line.** A recipe could say why an
  outcome was expected and the flags could not, so a run driven by flags could
  never fill the category the closed list exists to make countable. The list is
  the same one the recipe uses, and a value that is not on it is refused with
  the list to pick from.

### Security

- **A recipe that is not UTF-8 is refused rather than read as best it can be.**
  One saved as cp1250 with accented letters in a file name was accepted, and
  the file arrived named with replacement characters instead. The manifest
  recorded the same mangled name, so nothing downstream noticed that the name
  was not the one that had been asked for.

### Breaking

- **A file name may not contain a colon, and two names may not differ only in
  case.** Both were accepted before and both lost a file. Two targets asking
  for `report.txt` and `REPORT.TXT` ended with 0, left one file on the disk and
  described two in the manifest, because NTFS, APFS and exFAT treat those as
  one name. A name such as `AB:c.txt` on Windows names a data stream rather
  than a file: the run reported the file as not produced and still left an
  empty `AB` behind, in nobody's manifest and beyond the reach of cleanup. Both
  are refused on every system, including the ones where they would have worked,
  because a recipe that quietly loses a file on somebody else's machine is
  worse than one refused on both. A recipe using either will now be refused.

- **PDF bytes have changed.** The text on a page is laid out to a fixed line
  width now. It used to be eight to thirteen drawn words of four to nine
  characters, which meant the smallest document one seed could produce was not
  the smallest another could - so `--size 3300` was accepted for six seeds out
  of ten, and the floor moved between 3090 and 3499 bytes depending on the
  seed. The words still come from the seed, so two seeds still read
  differently. Only the length is settled. Anything holding a hash of a PDF
  this tool made will need to take it again.

### Changed

- **A directory you cannot write in says so.** It reported that the manifest
  already existed and was the only record of an earlier run, about an empty
  directory, and followed that with a second message for the same fault.
- **A rule binding two settings is declared rather than described.** PNG allows
  each side of a picture up to 20000 pixels and the two multiplied up to 40
  megapixels, and that second rule lived in the generator and in a sentence -
  so `tfg formats png` offered a pair it then refused, and nothing but a person
  could read the limit. It is in the registry now, printed on its own line and
  carried in `--json`, so a script or a window sees it.
- **A second run into the same directory refuses before it writes anything.**
  It used to write its whole set of files and only then find it had nowhere to
  record them: two runs started together left sixteen files on the disk with
  eight of them in nobody's manifest. The manifest name is taken before the
  first file now, and given back if the run ends without writing one.
- **The minimum `tfg formats` prints is a size it will take.** For pdf, wav
  and zip it was not: the number came from the registry, which holds the
  structural floor of a format with no label, and the label is on unless you
  turn it off. Asking for exactly what was printed was refused. The column now
  shows what an ordinary run accepts, and `--json` carries both - `min_bytes`
  unchanged, `smallest_accepted` beside it.
- **Whether a PDF size is accepted no longer depends on the seed.** The text on
  a page is drawn, so the smallest document one seed could produce was not the
  smallest another could: `--size 3300` was accepted for six seeds out of ten,
  and across two hundred seeds the floor moved between 3090 and 3499 bytes. An
  error that comes and goes when you change the seed is the one thing a tool
  built on repeatability cannot have. The floor is now worked out from the
  longest text a page can ever hold, so it is the same for everybody. Every
  document at an accepted size is unchanged, byte for byte - the boundary
  moved, not the content. The cost is that the floor is the worst case, so
  sizes some seeds could have produced are now refused.
- **The smallest SVG this tool makes now draws something.** At exactly its old
  minimum of 193 bytes the document held one empty text element and no shapes:
  valid SVG, exactly the size ordered, repeatable, and a blank canvas when
  rendered. The minimum is 194 bytes now, which is the first size that paints.
  Every SVG of 194 bytes or more is unchanged, byte for byte - the floor moved
  rather than the content.
- **A size is written the way a person writes it.** `--size 1e5` quietly meant
  100000 bytes, while a recipe refuses `1_000` and `0x10` outright on the
  grounds that a spelling is never guessed at. The flag now applies the same
  rule. `1.5gib` and every other decimal are untouched.
- **`tfg recipe fmt -w` replaces a file in one step**, through a temporary name,
  so a process ending mid write no longer leaves the recipe at half its length.
  Generated files and the manifest already worked this way.
- **The command recorded in the manifest can be run again.** It was built by
  joining the arguments with spaces, so `--name "my file.txt"` came back as two
  arguments and described a different run.
- **A run refuses to write through a name something else already occupies.**
  Only the final name was checked, and the temporary one is created in a way
  that truncates - so a file already sitting under it lost its contents without
  a word. A run killed outright leaves exactly that shape behind.
- **`tfg generate --json` reports a manifest that could not be delivered.**
  Piping into a command that closes early left the run reporting success with
  nothing on standard output.
- **`tfg recipe fmt` says what it does not check.** It settles the layout of a
  file and never claimed otherwise in `--help`, but the text implied more than
  it did. A recipe with a key nobody recognises still has a settled shape and
  is still formatted, so a check before a commit wants `tfg validate` beside
  it. Both the help and the flag description now say so.
- **A PNG asking for more pixels than it can hold is the caller's request, not
  a fault in the tool.** It ended with 1, which tells CI this build is broken,
  for a pair of numbers somebody chose. It now ends with 4, the same as every
  other value a format cannot deliver. `tfg formats png` also states the limit
  on the two dimensions multiplied - it offered each side up to 20000 without
  saying that both cannot be at their largest at once.
- **`--count 0` or below is reported with the number that was written.** It
  used to come back as "asks for 0 files" whatever was typed, because anything
  below one produced an empty list and the message described the list.
- **`--out` pointing at a file says so, once.** It produced two messages for
  one mistake, the first of them saying there was nothing at a path that had
  something at it.
- **A file name ending in a dot or a space is refused.** Windows stores such a
  name without the last character, so the file on disk was not the file the
  manifest described - `tfg generate --name "report."` ended with 0 and `tfg
  verify` then reported the same directory as wrong. Both spellings are refused
  on every system, the same way a path separator already is, so that a recipe
  means one thing everywhere. Names that merely contain a dot or a space are
  unaffected.

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

- **`tfg formats <id>` now answers about one format.** It used to print the
  whole list and ignore the argument, ending with 0, so there was no way to ask
  what a format accepts and the silence looked like an answer.
  It now lists every setting the format takes with its type, its range or the
  values it allows, its default and a sentence saying what it does - in prose
  and under `--json`. Asking about a format that does not exist ends with 4
  rather than pretending.

  ```
  properties, set with --set name=value:
    pages          whole number from 1 to 5000, default 1
                   How many pages the document has.
    page_size      one of: a4, a3, a5, letter, legal, default a4
                   The paper size every page uses.
  ```

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

- **A value a format cannot take is now reported as your mistake, not ours.**
  `--set width=abc` ended with exit code 1, which in this tool means the
  program itself failed - so a script could not tell a typo from a bug worth
  reporting. It now ends with 4, the same code a size below the minimum gives,
  and the message quotes the value and says what the setting accepts.
  One case changed verdict rather than only its code. `--set bit_depth=20` on a
  WAV used to get past the first check and fail later in different words,
  because bit depth was treated as any number from 8 to 32. It is one of 8, 16,
  24 or 32, and it is refused as such straight away.

- **`--boundary 15mb` is now refused rather than guessed at.** A boundary set
  is the one place where the number belongs to somebody else - it is the limit
  of the system you are testing, and "15 MB" on an upload form means 15000000
  bytes far more often than 15728640. Sizes here count in 1024s, so a set built
  from `15mb` sat entirely above a 15 MB limit and every file was rejected. The
  files looked right, which is what made it expensive.
  An ambiguous unit on a boundary now ends with an error naming both readings
  and giving both commands, so nothing has to be worked out. `--boundary
  15000000` and `--boundary 15mib` both say exactly which limit they mean and
  are unchanged, as is `tfg validate` on a recipe using the `boundary` key.
  `--size 15mb` is untouched. There the number describes a file this tool
  makes, and 1024s are what Explorer and `ls` show it as.

- **A boundary set now says which file is which.** The three files arrived as
  `files_0001`, `files_0002` and `files_0003`, and the only way to tell the one
  on the limit from the one a byte either side was to read their sizes off the
  disk. They are now called `<id>_under_limit`, `<id>_at_limit` and
  `<id>_over_limit`. A `--name` template still wins, and a group that is not a
  boundary set is still numbered.
  The run also prints the three exact byte counts. That matters more than it
  sounds: sizes here count in 1024s, so `--boundary 15mb` builds a set around
  15728640, while a service whose documentation says "15 MB" often means
  15000000 - and then all three files are over the limit and the set tests
  nothing. When the limit is a round number of these units, the output now says
  what the decimal equivalent would be and gives the command for it.

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
