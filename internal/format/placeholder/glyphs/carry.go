package glyphs

import "github.com/donislawdev/TestingFilesGenerator/internal/legal"

// The outlines announce themselves to the licence registry the moment this
// package is linked, which is how "tfg license" and the bill of materials know
// that a binary carries a part of Inter. internal/gui/font/carry.go says why
// it is done this way rather than read from the build.
func init() {
	legal.Carrying(packagePath)
}

// packagePath is this package as go list spells it, which is the spelling the
// registry entry carries.
const packagePath = "github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs"
