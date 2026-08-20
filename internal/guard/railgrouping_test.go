package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Donate and Add a batch do not read as a pair.
//
// They stand side by side at the left of the action bar in the same style, and
// they are the two least related controls in the window: one adds to the form
// in front of you and the other hands an address to your browser. Side by side
// in one style, proximity says they belong together - which is the only thing
// proximity ever says.
//
// Answered with a boundary rather than by moving the button. Under the last
// batch is where the button belongs by proximity, and a guard already keeps it
// reachable without scrolling - at three batches it would be off the screen, so
// somebody would have to scroll to add a fourth. That guard is the recorded
// decision and this change does not touch it.
//
// The comparison is with two buttons that ARE a pair. Preview and Generate do
// the same kind of thing to the same form, they sit together on purpose, and
// whatever gap they use is what "these belong together" looks like on this
// screen. Anything claiming to separate has to be wider than that.
func TestDonateAndAddABatchAreNotDrawnAsAPair(t *testing.T) {
	content, w := screenInAWindow(t, text.TabRecipe)
	settle(content, w)

	donate := buttonNamed(content, text.ButtonDonate)
	add := buttonNamed(content, text.ButtonAddBatch)
	preview := buttonNamed(content, text.ButtonPreview)
	generate := buttonNamed(content, text.ButtonGenerate)
	for name, b := range map[string]fyne.CanvasObject{
		text.ButtonDonate: donate, text.ButtonAddBatch: add,
		text.ButtonPreview: preview, text.ButtonGenerate: generate,
	} {
		if b == nil {
			t.Fatalf("the batch screen has no %q button, so this guard read the wrong tree", name)
		}
	}

	apart := horizontalGap(t, content, donate, add)
	together := horizontalGap(t, content, preview, generate)

	if apart <= together {
		t.Errorf("Donate and %q are %.1f px apart and two buttons that do belong together are %.1f px apart."+
			" Two unrelated controls drawn at the spacing of a pair read as a pair",
			text.ButtonAddBatch, apart, together)
	}
}

// horizontalGap is the empty space between the right edge of one thing and the
// left edge of the next.
func horizontalGap(t *testing.T, screen fyne.CanvasObject, left, right fyne.CanvasObject) float32 {
	t.Helper()
	a, ok := objectBox(screen, left)
	if !ok {
		t.Fatal("the button on the left is not laid out")
	}
	b, ok := objectBox(screen, right)
	if !ok {
		t.Fatal("the button on the right is not laid out")
	}
	return b.X - (a.X + a.Width)
}
