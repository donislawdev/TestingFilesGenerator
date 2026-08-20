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

### Changed

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
