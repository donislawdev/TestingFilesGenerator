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

// LicenceNotice is what "tfg license" prints and what the window's about
// screen shows.
//
// It lives here, on the bottom layer, because both surfaces need it and they
// sit on the same layer as each other - so neither can import the other, and a
// second copy is how the two would come to say different things. Which is D1
// in the one place it would be least noticed: nobody compares an about screen
// against a command.
//
// The second paragraph is the reason this text exists at all. Somebody
// deciding whether to put a generator into a closed source product has to know
// whether its licence reaches the files it produces, and the answer is no -
// but that is not obvious from the name of the licence, and guessing wrong in
// either direction costs them either a tool or a lawyer.
//
// The full licence text is not here. It is 674 lines, which is wrong for a
// terminal and wrong for a window, so this points at the file instead.
const LicenceNotice = `Testing Files Generator
Copyright (C) 2026 DonislawDev

Released under the GNU General Public License, version 3. The full text is in
the LICENSE file beside the source.

There is no warranty, to the extent the law allows.

The files you generate are yours. Generated files, recipes and manifests are
output of this program and not derived works of it, so this licence does not
reach them. You can generate fixtures, commit them and ship them inside a
closed source product with no obligation of any kind.

Code from other projects is compiled into this program. Their licences and
copyright notices are in THIRD-PARTY-NOTICES.md.
`
