package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Only a format that holds other files offers to put files in it.
//
// Every batch used to carry an "Add files inside" button, whatever it was going
// to produce, and a PNG batch is where the owner saw it on 2026-08-27. The
// reason written beside the code was that asking a format whether it holds
// files would put a rule about formats into the window. That premise is false:
// Container is a DECLARED field on a descriptor, sitting beside the properties
// this same screen already draws straight from the declaration, so asking it is
// the opposite of inventing a rule.
//
// Asked of the registry rather than of a list written here, so a format
// registered tomorrow is covered on the day it is registered - which is the
// same reason the menu itself is built from format.IDs().
func TestOnlyAFormatThatHoldsFilesOffersToPutFilesInIt(t *testing.T) {
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()

	holders, plain := 0, 0
	for _, d := range format.All() {
		chooserIn(t, screen.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected(d.ID)

		offered := buttonNamed(body, text.ButtonAddContents()) != nil
		switch {
		case d.Container && !offered:
			t.Errorf("%s holds other files and its batch offers no way to say what they are", d.ID)
		case !d.Container && offered:
			t.Errorf("%s holds no other files and its batch offers to put files in it, "+
				"which leads nowhere but a refusal", d.ID)
		}
		if d.Container {
			holders++
		} else {
			plain++
		}
	}

	// Both sides have to have happened. A build registering only containers -
	// or only plain formats - would satisfy every branch above without either
	// half of the rule being exercised.
	if holders == 0 || plain == 0 {
		t.Fatalf("this guard saw %d format(s) that hold files and %d that do not, "+
			"so one half of the rule went unchecked", holders, plain)
	}
	t.Logf("%d format(s) offer to hold files, %d do not", holders, plain)
}

// What somebody already typed into an archive stays on screen when the format
// changes under it.
//
// This is the half of the decision worth being careful about, and it is the
// owner's, taken on 2026-08-27 with both of the alternatives named. Dropping
// the rows would throw away typed work on the strength of one menu press, which
// is the silence untouchable rule 6 forbids. Hiding them while still sending
// them would put the run in the state folding the batches away was allowed to
// close on condition of avoiding: the engine refuses over a value, the screen
// has no box to mark, and the button reads as having done nothing.
//
// So the rows outlive the format and the refusal has somewhere to land. The way
// out of it stays visible too - remove them, or put the format back.
func TestContentsAlreadyTypedStayOnScreenWhenTheFormatChanges(t *testing.T) {
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()

	at := recipe.ContentAddress(1, 1, recipe.KeySize)
	chooserIn(t, screen.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("zip")
	pressNamed(t, body, text.ButtonAddContents())
	setBox(t, screen.Fields(), at, "3kb")

	// A format that holds nothing. The offer to add more goes, since another
	// one would not be legal.
	chooserIn(t, screen.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("txt")

	if buttonNamed(body, text.ButtonAddContents()) != nil {
		t.Error("a txt batch offers to add files inside it")
	}

	if findField(screen.Fields(), at) == nil {
		t.Fatalf("the row typed into an archive is gone from the screen after the format changed, "+
			"so a refusal about %s would have nothing to mark", at)
	}
	if got := boxText(t, screen.Fields(), at); got != "3kb" {
		t.Errorf("the row typed into an archive says %q after the format changed and it was typed as %q",
			got, "3kb")
	}
	if !strings.Contains(everythingSaid(body), text.ContentsHeading()) {
		t.Errorf("the rows are still registered and %q is not on the screen, "+
			"so what is left is a value nobody can see", text.ContentsHeading())
	}
}
