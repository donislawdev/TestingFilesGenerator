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

## [0.1.0] - 2026-08-20

Initial release.

[Unreleased]: https://github.com/donislawdev/TestingFilesGenerator/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/donislawdev/TestingFilesGenerator/releases/tag/v0.1.0
