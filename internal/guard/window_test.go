package guard

import (
	"image"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The window, rendered without a screen.
//
// Measured on 2026-08-05 with Fyne v2.8.0: the toolkit's test driver builds a
// widget tree and renders it to an image with CGO_ENABLED=0, on a machine with
// no graphics environment and no C compiler. That is what lets the whole CI
// matrix look at a screen, and it is why internal/gui/window and
// internal/gui/parts never import the toolkit's app package - the one that
// does sits alone behind a build tag.
//
// What these guards deliberately do NOT do is compare pixels against a stored
// image. Fonts differ between the three systems in the matrix and every
// toolkit update moves a few pixels, so a pixel golden would be a machine for
// producing false alarms, and a guard that cries wolf is uninstalled by the
// third week. The two questions worth asking are cheaper and steadier: does it
// draw anything at all, and does it say the thing it exists to say.

// A tree that renders as one flat colour passes every structural check and
// shows nothing. That defect is not hypothetical here: SVG at exactly its
// minimum size rendered as a single colour and passed every other guard in
// this project, which is why its oracle counts colours rather than parsing.
// Same question, same method.
//
// The limit of the method, measured on 2026-08-05 rather than assumed: this
// counts whether anything was drawn, NOT how much. A screen with the licence
// notice and a screen with that notice emptied both render 188 distinct
// colours, because the number is the anti-aliasing palette of the font rather
// than a measure of content. A screen with no sections at all renders 1.
//
// So this guard answers one question and the two below it answer the other.
// Worth writing down, because a colour count reads like a content check and is
// not one.
func TestTheWindowActuallyDrawsSomething(t *testing.T) {
	w := test.NewWindow(window.Start())
	defer w.Close()
	w.Resize(window.StartSize)

	img := w.Canvas().Capture()
	bounds := img.Bounds()
	if bounds.Dx() < 100 || bounds.Dy() < 100 {
		t.Fatalf("the canvas came back %dx%d, which is too small to be the window", bounds.Dx(), bounds.Dy())
	}

	colours := distinctColours(img)
	if colours < 3 {
		t.Errorf("the window rendered %d distinct colour(s), so it drew a flat rectangle rather than a screen", colours)
	}
	t.Logf("rendered %dx%d with %d distinct colours, no screen and no C compiler",
		bounds.Dx(), bounds.Dy(), colours)
}

func distinctColours(img image.Image) int {
	seen := map[uint32]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			seen[r>>8<<16|g>>8<<8|bl>>8] = true
		}
	}
	return len(seen)
}

// The sentence the first screen exists for.
//
// "tfg license" has carried it since 2026-08-04 and somebody using only the
// window had no way to read it - the drift D1 exists to stop, in the one place
// the parity guard cannot see, because a licence is not a capability of the
// engine. docs/GUI.md section 7 names this as the thing to do with the first
// window.
//
// Asserted on the text in the tree rather than on pixels, so it survives a
// font change and fails for the only reason worth failing for: the sentence
// went away.
func TestTheFirstScreenSaysWhoOwnsTheGeneratedFiles(t *testing.T) {
	shown := textIn(window.Start())

	for _, want := range []string{
		"General Public License",
		"generate are yours",
		"THIRD-PARTY-NOTICES.md",
		version.Version,
	} {
		if !strings.Contains(shown, want) {
			t.Errorf("the first screen does not say %q. What it says:\n%s", want, shown)
		}
	}
}

// Both surfaces read one constant, so they cannot come to say different
// things. The window and the command sit on the same layer and cannot import
// each other, which is exactly the shape in which a second copy gets written
// and nobody compares the two again.
func TestTheWindowAndTheCommandQuoteTheSameLicence(t *testing.T) {
	shown := textIn(window.Start())
	if !strings.Contains(shown, strings.TrimSpace(version.LicenceNotice)) {
		t.Error("the window does not show the licence notice verbatim, so it is a second copy now")
	}
}

// textIn walks a widget tree and collects everything a person would read.
func textIn(o fyne.CanvasObject) string {
	var b strings.Builder
	var walk func(fyne.CanvasObject)
	walk = func(obj fyne.CanvasObject) {
		switch v := obj.(type) {
		case *widget.Label:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *widget.Button:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *fyne.Container:
			for _, child := range v.Objects {
				walk(child)
			}
		}
	}
	walk(o)
	return b.String()
}
