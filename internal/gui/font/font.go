// Package font carries the typeface the window is set in.
//
// Why a package of its own holding bytes and nothing else: the same reason as
// internal/gui/icon. The bytes import nothing, so a guard asking what we ship
// can read them whatever the build, and the one line that hands them to the
// toolkit as a font resource lives beside the theme (internal/gui/parts).
//
// What is here. Inter, the static Regular and Bold instances from the v4.1
// release of github.com/rsms/inter (extras/ttf in the release archive,
// published 2024-11-16), under the SIL Open Font License 1.1. That is read
// from LICENSE.txt in the release and from the name table of each file
// (records 0 and 13), not remembered: the copyright line is "Copyright 2016
// The Inter Project Authors", no Reserved Font Name is declared, and the
// files call themselves version 4.001. The entry that travels with a release
// is in internal/legal/assets.go and THIRD-PARTY-NOTICES.md.
//
// Two weights and not four. Measured on 2026-09-15: nothing the window draws
// asks for italic or monospace - the only TextStyle{} outside these two is a
// measurement in a guard - so those styles keep the toolkit's own faces and
// the third of a megabyte an italic would cost buys nothing anybody sees.
//
// Static instances rather than the variable font. docs/GUI-REDESIGN
// section 5 left open whether the toolkit's shaper handles a variable font,
// and a static file asks it nothing it does not already do for Noto Sans.
package font

import _ "embed"

// Regular is what the body of every screen is set in.
//
//go:embed Inter-Regular.ttf
var Regular []byte

// Bold is what titles and field names are set in.
//
//go:embed Inter-Bold.ttf
var Bold []byte
