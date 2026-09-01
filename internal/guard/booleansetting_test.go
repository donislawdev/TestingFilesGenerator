package guard

import (
	"archive/zip"
	"path/filepath"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// A switch on the window has to reach the file, not just the screen.
//
// This is the first true or false setting any format declares. Every other
// declared setting is a box somebody types in or a menu, and both of those
// answer with text - so the path from a SWITCH to the engine had never carried
// anything, and a guard that only compared the screen against the registry
// would have been satisfied by a control that was drawn and then ignored.
//
// It presses Generate rather than Preview, and it reads the archive off the
// disk rather than the plan. A preview writes nothing, and the plan is the
// thing under test saying what it intended - neither could tell a switch that
// works from one that is drawn and dropped.
//
// The two runs differ ONLY in the switch. Same seed, same size, same depth, so
// anything that comes back different is the switch and nothing else.
func TestASwitchOnTheWindowReachesTheFileItDescribes(t *testing.T) {
	entriesFor := func(t *testing.T, ticked bool) []string {
		t.Helper()
		dir := t.TempDir()

		host := newFakeHost(t)
		screen := window.NewGenerate(host)
		content := screen.Object()
		t.Cleanup(func() { join(host) })

		fields := screen.Fields()
		chooserIn(t, fields, "format").SetSelected("zip")
		setBox(t, fields, "size", "64kb")
		setBox(t, fields, "depth", "2")
		setBox(t, fields, "entries", "2")
		toggleIn(t, fields, "directory_entries").SetChecked(ticked)

		entryUnder(t, content, text.FieldOutputDir()).SetText(dir)
		press(t, content, text.ButtonGenerate())
		join(host)

		made, err := filepath.Glob(filepath.Join(dir, "*.zip"))
		if err != nil || len(made) != 1 {
			t.Fatalf("the window wrote %v (err %v) with the switch %v, and this guard needs exactly one archive",
				made, err, ticked)
		}
		r, err := zip.OpenReader(made[0])
		if err != nil {
			t.Fatalf("the archive the window wrote cannot be opened: %v", err)
		}
		defer r.Close()

		var names []string
		for _, f := range r.File {
			names = append(names, f.Name)
		}
		return names
	}

	off := entriesFor(t, false)
	on := entriesFor(t, true)

	countDirs := func(names []string) int {
		n := 0
		for _, name := range names {
			if len(name) > 0 && name[len(name)-1] == '/' {
				n++
			}
		}
		return n
	}

	if countDirs(off) != 0 {
		t.Errorf("the switch was off and the archive still names %d directory entr(ies): %v",
			countDirs(off), off)
	}
	if got := countDirs(on); got != 2 {
		t.Errorf("the switch was ON and the archive names %d directory entries, want 2.\n"+
			"  with the switch off: %v\n"+
			"  with the switch on:  %v\n"+
			"Reason: a switch is the one control kind no format used before this setting, so the\n"+
			"path from it to the engine is the one nothing had ever carried a value along.",
			got, off, on)
	}
}
