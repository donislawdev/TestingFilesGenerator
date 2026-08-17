// Package icon carries the picture the desktop shows for this program.
//
// Why a package of its own holding bytes and nothing else. Everything in
// internal/gui that touches the toolkit sits behind the cgo build tag, so a
// guard asking what we actually ship would have to be built with a C compiler
// to see it. Bytes import nothing, so they live here in the open and can be
// read whatever the build, while the one line that turns them into a toolkit
// resource stays behind the tag.
//
// The picture itself is drawn by tools/appicon.py from shapes, so there is no
// artwork anybody else made anywhere in this repository and nothing to
// attribute. See docs/LICENSING.md.
package icon

import _ "embed"

// PNG is the application icon, 256 px square with an alpha channel.
//
// One size rather than a set. The toolkit scales it down itself and asks for
// one resource, and the sizes a person meets it at were checked by looking at
// them - tools/appicon.py --sheet draws every size on a light and a dark
// background, which is the only way to judge an icon that is 16 px in a
// taskbar and 512 px in an installer.
//
//go:embed chickpea.png
var PNG []byte
