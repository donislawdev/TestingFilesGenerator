package guard

import (
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// A boundary that does not know where it is refuses, rather than waving things
// through.
//
// core.Boundary answers one question: does this path land outside the directory
// once the links are followed. It has two ways of not being able to answer -
// the directory could not be resolved, and the path could not be made absolute
// - and until 2026-08-27 both answered "it stays inside". Review item N5.
//
// That direction is the unsafe one, and which way it is unsafe is worth being
// exact about. Two layers stop a path from somebody else's manifest: the text
// is judged when the manifest is read, and where the path lands is judged when
// it is used. The first catches a written climb such as "../x". A name that
// leaves through a link holds no climb to read, so for that one there is only
// this layer - and this layer saying "inside" when it cannot tell is the layer
// not being there.
//
// The zero value is what makes this askable from outside the package, and it is
// not a contrivance: a Boundary nobody resolved is exactly the state the first
// branch describes.
func TestABoundaryThatCannotTellRefuses(t *testing.T) {
	var unresolved core.Boundary

	for _, path := range []string{
		filepath.Join("some", "where", "file.txt"),
		"file.txt",
		"",
	} {
		if !unresolved.Escapes(path) {
			t.Errorf("a boundary that resolved nothing says %q stays inside it. "+
				"It cannot know that, and answering the safe-sounding way lets a link out "+
				"past the only check that looks at links", path)
		}
	}
}

// And a boundary that DOES know still answers the ordinary cases the way it
// always did, so the change above is a refusal added rather than a rule
// replaced.
//
// Without this the guard above would pass against a build whose Escapes
// returned true for everything, which would refuse every run this tool has.
func TestABoundaryThatKnowsStillLetsItsOwnFilesThrough(t *testing.T) {
	dir := t.TempDir()
	b := core.NewBoundary(dir)

	inside := filepath.Join(dir, "files_0001.txt")
	if b.Escapes(inside) {
		t.Errorf("%s is inside %s and the boundary says it escapes", inside, dir)
	}

	outside := filepath.Join(filepath.Dir(dir), "VICTIM.txt")
	if !b.Escapes(outside) {
		t.Errorf("%s is outside %s and the boundary says it stays in", outside, dir)
	}
}
