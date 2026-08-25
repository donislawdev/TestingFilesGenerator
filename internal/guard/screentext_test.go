package guard

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The licence notice points somewhere that is always there.
//
// It named two files and nothing else - the licence beside the source, and the
// third party notices. Somebody who downloaded only the window binary has
// neither of them next to it, so the one screen whose entire purpose is
// pointing somewhere was pointing at nothing (O108). Naming the canonical
// address as well costs a line and cannot go missing.
//
// It asks for an address rather than for particular words, because the wording
// is the sort of thing that gets improved and the property is that a reader who
// has only the binary can still get to the licence.
func TestTheLicenceNoticePointsSomewhereThatAlwaysExists(t *testing.T) {
	if !strings.Contains(version.LicenceNotice, "https://") {
		t.Errorf("the licence notice names no address:\n%s\n"+
			"Everything it points at is a file, and somebody who downloaded only the binary has "+
			"none of them - so the notice is a dead end for exactly the reader it is for.",
			version.LicenceNotice)
	}
}

// The batch screen says what an empty box will do.
//
// The single batch screen arrives with "files" and "1" typed in and this one
// arrives empty, which reads as one of the two being wrong. Neither is: a value
// typed in is a value STATED, and on this screen an unstated setting has to
// stay unstated, because that is what makes the recipe leave the key out. The
// difference was deliberate and written down only in the code, so from the
// screen it looked like an inconsistency (O109).
//
// The box has to stay EMPTY and say the default some other way. Filling it in
// would be the fix that breaks the thing the emptiness is for.
func TestTheBatchScreenSaysWhatAnEmptyBoxWillDo(t *testing.T) {
	content, _ := screenInAWindow(t, text.TabRecipe())

	box := entryUnder(t, content, text.FieldCount())
	if box == nil {
		t.Fatal("the batch screen has no count box, so this guard read the wrong tree")
	}
	if box.Text != "" {
		t.Errorf("the count box arrives holding %q. A box with a value in it cannot say the setting "+
			"was left unstated, which is what keeps the key out of the recipe.", box.Text)
	}
	if want := strconv.Itoa(recipe.DefaultCount); box.PlaceHolder != want {
		t.Errorf("the count box shows %q as what happens if it is left alone and the recipe uses %q",
			box.PlaceHolder, want)
	}
}

// Three clicks in a box select what is in it.
//
// The interface audit reported this as broken and marked it low confidence,
// which was the right call: it was measured with synthetic clicks, and this is
// the sixth thing that method got wrong about this window. The toolkit does
// implement it - a third press within the double tap delay selects the row -
// and our boxes are its own Entry with nothing of ours overriding the pointer
// events, so it works and the report was an artefact (O110).
//
// It is guarded rather than crossed off, because the behaviour depends on these
// boxes staying the toolkit's own Entry.
//
// What it does NOT prove, said here rather than left to be assumed: it drives
// the box directly, so it would catch the box being replaced by something that
// is not a text box, and it would not catch a wrapper put in front of one and
// swallowing the presses before they arrive. Listed as unproven by mutation for
// that reason.
func TestThreeClicksInABoxSelectWhatIsInIt(t *testing.T) {
	content, _ := screenInAWindow(t, text.TabOneTarget())

	box := entryUnder(t, content, text.FieldSize())
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	box.SetText("10mb")

	// The same three steps the toolkit's own test for this uses: a double tap,
	// then a press soon enough after it to count as the third.
	test.DoubleTap(box)
	time.Sleep(50 * time.Millisecond)
	box.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: fyne.NewPos(1, 1)},
	})

	if got := box.SelectedText(); got != "10mb" {
		t.Errorf("three clicks in a box holding %q selected %q, so typing over it appends to the "+
			"value instead of replacing it.\n"+
			"What to do: these boxes are the toolkit's own Entry and it handles this itself. "+
			"Something of ours is now intercepting the pointer events on them.", "10mb", got)
	}
}
