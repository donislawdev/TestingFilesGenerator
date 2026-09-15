package font

import "github.com/donislawdev/TestingFilesGenerator/internal/legal"

// The font announces itself to the licence registry the moment this package
// is linked, which is how "tfg license" and the bill of materials know that
// the window carries Inter and the command line does not: both binaries are
// one module, so the build's own record cannot tell them apart, and a package
// that is linked is a package whose init ran. A guard asks that every entry of
// this kind in internal/legal is announced this way.
func init() {
	legal.Carrying(packagePath)
}

// packagePath is this package as go list spells it, which is the spelling the
// registry entry carries. Written out rather than derived, because the only
// thing that could derive it at run time is reflection on a type this package
// does not otherwise need.
const packagePath = "github.com/donislawdev/TestingFilesGenerator/internal/gui/font"
