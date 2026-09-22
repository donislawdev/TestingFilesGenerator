package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What a finished run says about the record it left, and the way to open it.
//
// The command line has printed "manifest: <path>" since there was a manifest,
// and the window said "3 files written." and nothing else - so the one thing
// this tool makes that other generators do not was, from a window, a file you
// found in the folder afterwards and wondered about. D1 asks for parity
// between the two surfaces and this is the kind that goes quietly: nothing the
// engine can do was missing, only the sentence about it.

// TestAFinishedRunNamesTheManifestAndOffersToOpenIt.
//
// Neither half asks the window what it decided. The NAME is compared against
// the file that is actually on the disk after the run, and the BUTTON is
// pressed and its destination is compared against that same file - so a
// screen that names a manifest it did not write, or opens one it did not
// name, is red. A guard that read text.ManifestNamed and looked for it on the
// screen would be comparing the window with itself.
func TestAFinishedRunNamesTheManifestAndOffersToOpenIt(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	// Nothing has run, so there is nothing to open. Asked about what is SHOWN
	// rather than what is in the tree: the button is built with the bar and
	// hidden, so a guard that only looked for it would find it every time.
	if shownButton(screen, text.ButtonOpenManifest()) != nil {
		t.Fatal("the manifest button is on the screen before anything was written, " +
			"so it points at a file that need not exist")
	}

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("done")
	press(t, screen, text.ButtonGenerate())
	waitForManifest(t, host, dir)
	join(host)

	// The file the run really wrote, found by looking rather than by naming:
	// the manifest's name is a setting, so a guard holding the default would
	// stop asking anything the day a screen wrote it somewhere else.
	written := manifestIn(t, dir)

	if said := shownText(screen); !strings.Contains(said, filepath.Base(written)) {
		t.Errorf("the run wrote %s and the screen never names it. It says:\n%s\n"+
			"Somebody generating from this window has no way to learn they got a manifest at all",
			filepath.Base(written), said)
	}

	button := shownButton(screen, text.ButtonOpenManifest())
	if button == nil {
		t.Fatal("the run wrote a manifest and there is no way to open it")
	}
	button.OnTapped()
	if host.fileCount != 1 {
		t.Errorf("the manifest button was pressed and the desktop was asked to open a file %d times", host.fileCount)
	}
	if host.file != written {
		t.Errorf("the button opens %q and the manifest of this run is %q", host.file, written)
	}
	// And the folder button is untouched by any of this, because two buttons
	// that both open the folder would be one button drawn twice.
	if host.folderCount != 0 {
		t.Errorf("pressing the manifest button asked the desktop for a folder %d times", host.folderCount)
	}
}

// TestAPreviewNamesNoManifestAndOffersNone is the other end of it.
//
// A preview goes through the whole of planning with nothing written, so there
// is no record to name and no file to open - and a screen that said there was
// would be sending somebody to a file the run deliberately did not create.
// This is also the half that fails if the sentence is ever attached to the
// outcome rather than to the saving.
func TestAPreviewNamesNoManifestAndOffersNone(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("planned")
	press(t, screen, text.ButtonPreview())
	join(host)

	// The preview really did go through, or the two questions below are being
	// asked of a screen where nothing happened at all.
	if said := shownText(screen); !strings.Contains(said, "1 file") {
		t.Fatalf("the preview said nothing about what the form comes to, so this guard is "+
			"asking about a press that did not work. It says:\n%s\nRefusal: %q", said, anyRefusal(screen))
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("a preview wrote into %s: %v (err %v)", dir, whatIsIn(dir), err)
	}

	if shownButton(screen, text.ButtonOpenManifest()) != nil {
		t.Error("a preview wrote nothing and the screen offers to open a manifest")
	}
	if said := shownText(screen); strings.Contains(said, text.ManifestNamed("")) {
		t.Errorf("a preview wrote nothing and the screen names a manifest:\n%s", said)
	}
}

// manifestIn is the one record a run left in a directory, found on the disk.
//
// It refuses to guess where there is more than one, because the question this
// helper answers - which file did the window just name - has no answer then.
func manifestIn(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s holds %d files that could be the manifest: %v", dir, len(found), whatIsIn(dir))
	}
	return found[0]
}
