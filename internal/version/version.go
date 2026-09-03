// Package version holds the tool version in exactly one place.
//
// Only the owner raises it. A major bump is also a statement about file
// hashes in other people's CI, so it is never routine tidying.
package version

// Version is written into every manifest. A manifest without it cannot
// explain a hash mismatch after an upgrade.
//
// Set to 0.1.0 by the owner on 2026-08-02, raised to 0.2.0 on 2026-08-28 and
// to 0.3.0-rc1 on 2026-09-03, every one of them by the owner. It stopped being
// an internal number the day 0.1.0 was published: this is what somebody
// compares their build against when a hash they stored months ago does not
// match.
//
// 0.2.0 was a minor bump that moved no bytes. This one moves them, and before
// 1.0 the minor is where that goes - D11 and immutable rule 3 ask for a bump,
// not for a particular digit. Six formats have different bytes under Go 1.27,
// a log advances through time, a GIF moves, and a CSV quotes only the fields
// that need it. The changelog lists each one and says whether there is a way
// back to the old bytes.
//
// The suffix is not decoration and it is not spelled freely. The release
// workflow marks a release as a prerelease when the TAG NAME carries a hyphen,
// so 0.3.0rc1 would have gone out as the latest stable release. The changelog
// section has to be spelled the same way, because the workflow greps for a
// heading naming exactly this string before it will build anything.
const Version = "0.3.0-rc1"

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
// terminal and wrong for a window, so this points at where it is instead.
//
// It points at the canonical address as well as at the file, since 2026-08-19.
// Both sentences named a file and nothing else, and somebody who downloaded
// only the window binary has neither file next to it - so the one screen whose
// entire purpose is pointing somewhere was pointing at nothing. Naming a place
// that is always there costs a line and removes the dead end (O108).
const LicenceNotice = `Testing Files Generator
Copyright (C) 2026 DonislawDev

Released under the GNU General Public License, version 3. The full text is at
https://www.gnu.org/licenses/gpl-3.0.html, and in the LICENSE file if you have
the source or the full release.

There is no warranty, to the extent the law allows.

The files you generate are yours. Generated files, recipes and manifests are
output of this program and not derived works of it, so this licence does not
reach them. You can generate fixtures, commit them and ship them inside a
closed source product with no obligation of any kind.

Code from other projects is compiled into this program. Their licences and
copyright notices are in THIRD-PARTY-NOTICES.md, which comes with the source
and with the full release.
`
