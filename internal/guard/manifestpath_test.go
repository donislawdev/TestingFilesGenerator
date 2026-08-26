package guard

import (
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// Where the manifest lands is worked out once, and the savers ask.
//
// The engine claims the manifest name before the first file and refuses the
// run if that name is taken, so a saver joining the path its own way is a
// second answer to a question already answered. There were three answers on
// 2026-08-26: the engine's, the command line's, and the window's - and the
// window's was the one that did not handle an empty name. Review item N1.
//
// With the name blank, filepath.Join(OutDir, "") is the output DIRECTORY, so
// saving would have renamed a file onto a directory. Nothing reached it because
// all three screens fill the field in, and "no caller reaches it today" is a
// description of a fault waiting for a fourth caller.
func TestWhereTheManifestLandsIsOneAnswer(t *testing.T) {
	dir := filepath.Join("some", "out")

	// The case the window would have got wrong.
	blank := engine.ManifestPath(engine.Options{OutDir: dir})
	if blank == dir || blank == filepath.Clean(dir) {
		t.Fatalf("with no manifest name given, the path is the output directory itself (%s). "+
			"Saving there renames a file onto a directory", blank)
	}
	if want := filepath.Join(dir, engine.DefaultManifestName); blank != want {
		t.Errorf("with no manifest name given the path is %s, want %s", blank, want)
	}

	// And a name that was given is honoured rather than replaced.
	named := engine.ManifestPath(engine.Options{OutDir: dir, ManifestName: "run.json"})
	if want := filepath.Join(dir, "run.json"); named != want {
		t.Errorf("with a manifest name given the path is %s, want %s", named, want)
	}
}
