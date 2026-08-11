package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The form stops at a readable width however wide the window is.
//
// O72, measured twice: maximised to 3862 px, every box on the screen was 3848
// to 3854 px of it, so the seed field holding "0" was nearly four thousand
// pixels wide. UX6 asks it as a question - run your eye along a row to the
// right edge, and if you got lost the row is too long.
//
// This guard exists because the comment beside parts.ColumnWidth calls it a
// ratchet that can be tightened and never widened, and a ratchet nothing turns
// is a sentence. A VBox stretches its children to whatever it is handed, so
// the old behaviour is one wrapper away at any time and nothing else would
// notice: every other guard here asks what a control holds, not how wide it is.
func TestTheFormDoesNotRunToTheEdgeOfTheWindow(t *testing.T) {
	for _, screenName := range []string{"generate", "preset"} {
		host := &fakeHost{}
		window.Open(host)
		if screenName == "preset" {
			press(t, host.content, "Presets")
		}
		content := host.content

		w := test.NewWindow(content)
		// Far wider than any window a person opens, which is the point: the
		// defect only shows when there is space to stretch into.
		w.Resize(fyne.NewSize(3862, 1200))
		content.Refresh()

		widest := float32(0)
		var worst fyne.CanvasObject
		walk(content, func(obj fyne.CanvasObject) {
			switch obj.(type) {
			case *widget.Entry, *widget.Select, *widget.Check:
			default:
				return
			}
			if size := obj.Size().Width; size > widest {
				widest, worst = size, obj
			}
		})
		w.Close()

		if widest == 0 {
			t.Fatalf("%s: no control was measured, so this guard read the wrong tree", screenName)
		}
		if widest > parts.ColumnWidth {
			t.Errorf("%s: a %T is %.0f px wide in a 3862 px window, over the %d px this form allows.\n"+
				"A row that long cannot be followed from its label to its value.",
				screenName, worst, widest, parts.ColumnWidth)
		}
		t.Logf("%s: widest control %.0f px in a 3862 px window, ceiling %d", screenName, widest, parts.ColumnWidth)
	}
}

// A switch says what it is on the part you click.
//
// The other half of O72. Given a heading above it like every other field, a
// switch arrives as a bare square: the name is above it, the sentence below,
// and there is nothing to read on the thing itself - nor anything but the
// square to aim at.
func TestASwitchCarriesItsOwnName(t *testing.T) {
	_, content := screen(t)

	found := 0
	walk(content, func(obj fyne.CanvasObject) {
		check, ok := obj.(*widget.Check)
		if !ok {
			return
		}
		found++
		if check.Text == "" {
			t.Errorf("a switch on the generate screen carries no words, so there is nothing to read on it and only the square to click")
		}
	})
	if found == 0 {
		t.Fatal("no switch was found, so this guard read the wrong tree")
	}
}
