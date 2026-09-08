// Package typeface carries the letters this program is written in.
//
// A package of bytes and nothing else, for the same reason internal/gui/icon is
// one: everything in internal/gui that touches the toolkit sits behind the cgo
// build tag, so a guard asking what we actually ship would need a C compiler to
// see it. Bytes import nothing, so they live here in the open and can be read
// whatever the build, while the line that turns them into a toolkit resource
// stays behind the tag.
//
// Only the window links this. The command line prints to a terminal and the
// letters there are the terminal's, so shipping a face inside that binary would
// be 812 kB nobody can see - and docs/QUALITY.md already has a guard saying the
// bill of materials must not mix the two binaries.
//
// # Which faces, and why only two
//
// Inter 4.1, the static Regular and SemiBold, unmodified. fyne.TextStyle in
// v2.8.1 offers Bold, Italic, Monospace and Symbol and nothing between them -
// measured, not assumed - so a type ramp cannot be carried by weight. Two faces
// is what the toolkit can address, and the rest of the difference between a
// screen title, a section title and a field name is carried by size and colour.
//
// SemiBold rather than Bold for the heavy one. At 14 px on a dark ground Bold
// closes up, and every heavy word in this window is short - a section title or
// a button.
//
// Italic is deliberately absent. docs/UX.md section 8.5 bans it, citing that it
// costs readers with dyslexia, so the theme answers the italic style with the
// upright face and there is nothing here to answer it with.
//
// # Licence
//
// SIL Open Font License 1.1, Copyright (c) 2016 The Inter Project Authors,
// read from LICENCE-Inter.txt at tag v4.1 rather than from memory, which is
// what docs/SECURITY.md section 4 asks for. There is no Reserved Font Name, so
// these files may be shipped as they are. They are shipped as they are: unmodified, un-subset, so
// nothing here needs renaming and the bytes can be compared against the
// upstream release.
//
// The licence text travels with the release in THIRD-PARTY-NOTICES.md, which is
// what the OFL asks for, and the file below is the copy this repository keeps.
package typeface

import _ "embed"

// Regular is the face nearly every word on screen is set in.
//
//go:embed Inter-Regular.ttf
var Regular []byte

// SemiBold is the face for a section title, a screen title and a button.
//
//go:embed Inter-SemiBold.ttf
var SemiBold []byte

// Licence is the OFL text that has to travel with the faces.
//
// Embedded rather than left as a file beside the binary, because a file beside
// the binary is a file somebody unzips without, and then the obligation is
// unmet by accident. The notices command reads this.
//
//go:embed LICENCE-Inter.txt
var Licence string
