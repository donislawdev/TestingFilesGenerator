package guard

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Working out what a run would cost can be stopped.
//
// It could not be, and the window said so in a comment: "preflight takes no
// context, so there is nothing to cancel - and that is why the button is not
// offered either". The cost of that was not the missing button. Closing the
// window asked every screen to stop, stopping a preview meant waiting for it,
// and the wait happened on the interface thread - so shutting the window during
// a preview of a large set on a slow disk froze the whole thing until preflight
// had asked the filesystem about every planned file, twice each.
//
// Two things had to give way for the button to mean anything, and both are
// asked about here: planning walks a lot of files, and so does preflight.
func TestWorkingOutTheCostCanBeStopped(t *testing.T) {
	targets := []engine.Target{{
		ID:     "files",
		Format: "txt",
		Sizes:  engine.Uniform(200000, 4096),
	}}
	opt := engine.Options{OutDir: t.TempDir(), ManifestName: "manifest.json"}

	// Already cancelled, so the answer does not depend on how fast this machine
	// is - which is what a guard about interrupting must not depend on.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := engine.PlanContext(ctx, targets, opt); !errors.Is(err, context.Canceled) {
		t.Errorf("planning two hundred thousand files ignored a cancelled context and answered %v", err)
	}

	// And the same question of the preflight, through the run it guards. A dry
	// run reaches preflight and writes nothing, which is the shape a preview
	// takes.
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning without a cancelled context: %v", err)
	}
	opt.DryRun = true
	if _, err := engine.Run(ctx, planned, opt); !errors.Is(err, context.Canceled) {
		t.Errorf("a dry run ignored a cancelled context and answered %v", err)
	}
}

// The window offers the way out of a preview, rather than hiding it.
//
// setBusy takes "stoppable" separately from "busy" because a preview used to be
// the one occupied state with nothing to cancel. A permanently dead control is
// a question the screen keeps asking and answering itself, so it was hidden -
// correctly, then. Now there is something to cancel, and an offer that is not
// made is work that can be stopped by nobody.
func TestAPreviewOffersTheWayOut(t *testing.T) {
	host, content := screen(t)
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldCount(), "20000")
	press(t, content, text.ButtonPreview())

	cancel := buttonNamed(content, text.ButtonCancel())
	if cancel == nil {
		t.Fatalf("there is no Cancel button while a preview is going. The screen has: %v", buttonNames(content))
	}
	if cancel.Hidden {
		t.Error("Cancel is hidden during a preview, so work that can be stopped looks like work that cannot")
	}
	join(host)
}

// Both counts of a cleanup report add up to the entries it lists.
//
// They did not. Measured on 2026-08-26 on a run of four files with one deleted
// by hand before cleanup: removed 3, kept 0, and four entries in the list. The
// kept count was the number BLOCKED, which is a smaller set - a file already
// gone is kept and is not blocked - so a script adding the two got three and
// had no way to know which entry it had lost.
//
// The blocked count still exists and still decides the exit code and whether
// the manifest may go, because "a file is still there" and "a file was not
// removed" are different facts. What changed is which of them the report calls
// kept, and the answer is now the one its own entries give.
func TestACleanupReportAddsUp(t *testing.T) {
	dir := t.TempDir()
	runCLI(t, "generate", "--format", "txt", "--size", "100b", "--count", "4", "--out", dir)

	// One file taken away behind the tool's back, which is the case that split
	// the two counts apart.
	if err := os.Remove(filepath.Join(dir, "files_0002.txt")); err != nil {
		t.Fatalf("taking one file away: %v", err)
	}

	said := runCLI(t, "cleanup", filepath.Join(dir, "manifest.json"), "--yes", "--json")

	var report struct {
		Removed int `json:"removed"`
		Kept    int `json:"kept"`
		Files   []struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(said)), &report); err != nil {
		t.Fatalf("the report is not readable as JSON: %v\n%s", err, said)
	}

	if report.Removed+report.Kept != len(report.Files) {
		t.Errorf("the report says %d removed and %d kept, which is %d, and it lists %d files.\n"+
			"A script adding the two has no way to know which entry it lost.",
			report.Removed, report.Kept, report.Removed+report.Kept, len(report.Files))
	}
	// And the entry for the missing file still says what happened to it, so
	// this cannot be satisfied by a report that stopped listing it.
	if !strings.Contains(said, "already gone") {
		t.Errorf("no entry says the missing file was already gone:\n%s", said)
	}
}
