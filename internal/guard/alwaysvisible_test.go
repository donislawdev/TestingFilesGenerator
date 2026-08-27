package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Adding a batch can be reached without scrolling for it.
//
// The batch screen calls itself "Run several batches together" and showed one
// batch and no visible way to add a second: the button sat at the end of the
// list, and one batch is already 1340 px against 826 px of room. So the single
// control that makes this screen different from the other two was off the
// bottom of it the moment it opened (O112).
//
// The test is that the control is OUTSIDE the scrolling part, not that it
// exists - it existed before. A control that exists somewhere below the fold is
// exactly the state being fixed.
//
// It is also checked to go out of use during a run, because pressing it then
// rebuilds the form underneath a run that is writing files.
func TestAddingABatchIsReachableWithoutScrolling(t *testing.T) {
	content, _ := screenInAWindow(t, text.TabRecipe())

	add := buttonNamed(content, text.ButtonAddBatch())
	if add == nil {
		t.Fatalf("the batch screen has no %q button. It has: %v",
			text.ButtonAddBatch(), buttonNames(content))
	}

	scroll := scrollIn(content)
	if scroll == nil {
		t.Fatal("the batch screen has no scrolling area, so this guard read the wrong tree")
	}
	if holds(scroll.Content, add) {
		t.Errorf("%q is inside the scrolling part of the batch screen, so it is only reachable "+
			"after scrolling past a batch taller than the window - and it is the one control "+
			"this screen exists for.", text.ButtonAddBatch())
	}
}

// A run takes the batch screen's own button out of use with the rest of the
// form.
//
// It is not a field, so freezing the registry does not reach it, and it is not
// one of the run buttons either - it would have been the one control on the
// screen still live during a run, and pressing it rebuilds the form under a run
// that is writing files.
func TestAddingABatchIsOutOfUseWhileARunIsGoing(t *testing.T) {
	dir := t.TempDir()
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()
	fields := screen.Fields()

	add := buttonNamed(body, text.ButtonAddBatch())
	if add == nil {
		t.Fatalf("the batch screen has no %q button. It has: %v",
			text.ButtonAddBatch(), buttonNames(body))
	}
	if add.Disabled() {
		t.Fatal("the add button starts disabled, so this guard cannot tell a run from a fresh screen")
	}

	// Enough files that the run is still going when the question is asked. The
	// same shape the cancel guard uses, and for the same reason.
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "files")
	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("txt")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "64kb")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyCount), "400")
	setBox(t, fields, recipe.KeyOutputDir, dir)

	pressNamed(t, body, "Generate")

	if !add.Disabled() {
		t.Error("a batch can still be added while a run is writing files, which rebuilds the form " +
			"the run is going through.")
	}

	cancel := buttonNamed(body, "Cancel")
	if cancel == nil {
		t.Fatalf("there is no Cancel button. The screen has: %v", buttonNames(body))
	}
	cancel.OnTapped()

	if add.Disabled() {
		t.Error("the add button is still out of use after the run stopped, so it never comes back.")
	}
}
