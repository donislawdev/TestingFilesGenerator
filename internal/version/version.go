// Package version holds the tool version in exactly one place.
//
// Only the owner raises it. A major bump is also a statement about file
// hashes in other people's CI, so it is never routine tidying.
package version

// Version is written into every manifest. A manifest without it cannot
// explain a hash mismatch after an upgrade.
//
// Set to 0.1.0 by the owner on 2026-08-02. It is an internal number: nothing
// has been released, the repository is private, and both changelogs keep
// everything under [Unreleased] because there is no release to point at. What
// it buys today is that a manifest found later says which line of the tool
// produced it, which "0.0.0-dev" could not.
const Version = "0.1.0"
