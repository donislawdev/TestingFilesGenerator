package guard

import (
	"image"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

// fakeHost is a window that records rather than opens.
//
// The screen takes an interface of three methods instead of fyne.Window for
// exactly this: a real window needs a C compiler and a graphics environment,
// and the two behaviours worth proving here - closing during a run, and moving
// between screens - are behaviours of the screen rather than of the toolkit.
type fakeHost struct {
	content   fyne.CanvasObject
	intercept func()
	closed    int
	picked    string
	asked     int
}

func (h *fakeHost) SetContent(o fyne.CanvasObject) { h.content = o }
func (h *fakeHost) SetCloseIntercept(fn func())    { h.intercept = fn }
func (h *fakeHost) Close()                         { h.closed++ }

// picked is what the stand in answers when a screen asks where the files
// should go, and asked counts how often it was asked. A real picker needs a
// real window, and the behaviour worth proving is that the button reaches one
// and that the answer lands in the field.
func (h *fakeHost) ChooseDirectory(chosen func(string)) {
	h.asked++
	// Always, including the empty answer that means somebody changed their
	// mind. Staying silent here would make the caller's handling of that answer
	// unreachable from any test.
	chosen(h.picked)
}

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
// So this guard answers one question and the ones below answer the other.
// Worth writing down, because a colour count reads like a content check and is
// not one.
func TestTheWindowActuallyDrawsSomething(t *testing.T) {
	w := test.NewWindow(window.FirstScreen(&fakeHost{}))
	defer w.Close()
	w.Resize(window.OpenSize)

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

// The sentence the licence screen exists for.
//
// "tfg license" has carried it since 2026-08-04 and somebody using only the
// window had no way to read it - the drift D1 exists to stop, in the one place
// the parity guard cannot see, because a licence is not a capability of the
// engine. docs/GUI.md section 7 names it.
//
// Asserted on the text in the tree rather than on pixels, so it survives a
// font change and fails for the only reason worth failing for: the sentence
// went away.
func TestTheAboutScreenSaysWhoOwnsTheGeneratedFiles(t *testing.T) {
	shown := textIn(window.About(func() {}))

	for _, want := range []string{
		"General Public License",
		"generate are yours",
		"THIRD-PARTY-NOTICES.md",
		version.Version,
	} {
		if !strings.Contains(shown, want) {
			t.Errorf("the licence screen does not say %q. What it says:\n%s", want, shown)
		}
	}
}

// Both surfaces read one constant, so they cannot come to say different
// things. The window and the command sit on the same layer and cannot import
// each other, which is exactly the shape in which a second copy gets written
// and nobody compares the two again.
func TestTheWindowAndTheCommandQuoteTheSameLicence(t *testing.T) {
	shown := textIn(window.About(func() {}))
	if !strings.Contains(shown, strings.TrimSpace(version.LicenceNotice)) {
		t.Error("the window does not show the licence notice verbatim, so it is a second copy now")
	}
}

// The screen moved out of the way on 2026-08-05 and staying reachable is the
// whole of what docs/GUI.md section 7 asks. A notice nobody is shown and nobody
// can get to is a notice that is not there, and the difference between those
// two states is one button that anybody could delete without noticing.
func TestTheLicenceIsStillReachableFromTheOpeningScreen(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)

	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	about := buttonNamed(host.content, "About")
	if about == nil {
		t.Fatalf("the opening screen has no way to the licence. Its buttons: %v",
			buttonNames(host.content))
	}

	about.OnTapped()
	shown := textIn(host.content)
	if !strings.Contains(shown, "generate are yours") {
		t.Errorf("pressing About did not lead to the licence. What is on screen now:\n%s", shown)
	}

	// And back, or the licence is a room with no door out.
	if back := buttonNamed(host.content, "Back"); back == nil {
		t.Error("the licence screen has no way back, so reading it costs the window")
	} else {
		back.OnTapped()
		if buttonNamed(host.content, "Generate") == nil {
			t.Error("Back did not return to the generate screen")
		}
	}
}

// The window opens on the work. Decided by the owner on 2026-08-05, against
// keeping the notice as a splash - a screen shown at every start is one nobody
// reads twice, and the reachability above is what the rule actually asks for.
func TestTheWindowOpensOnTheGenerateScreen(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)

	if buttonNamed(host.content, "Generate") == nil {
		t.Errorf("the window does not open on the generate screen. Its buttons: %v",
			buttonNames(host.content))
	}
}

// walk visits every object of a tree, through both kinds of grouping this
// window uses. A walker that only knew about containers would stop at the
// scroll the generate screen sits in and report an empty screen.
func walk(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	visit(o)
	switch v := o.(type) {
	case *fyne.Container:
		for _, child := range v.Objects {
			walk(child, visit)
		}
	case *container.Scroll:
		walk(v.Content, visit)
	}
}

// textIn walks a widget tree and collects everything a person would read.
func textIn(o fyne.CanvasObject) string {
	var b strings.Builder
	walk(o, func(obj fyne.CanvasObject) {
		switch v := obj.(type) {
		case *widget.Label:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *widget.Button:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *widget.Entry:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *widget.Select:
			b.WriteString(v.Selected)
			b.WriteString("\n")
		}
	})
	return b.String()
}

func buttonNamed(o fyne.CanvasObject, name string) *widget.Button {
	var found *widget.Button
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok && b.Text == name {
			found = b
		}
	})
	return found
}

func buttonNames(o fyne.CanvasObject) []string {
	var out []string
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok {
			out = append(out, b.Text)
		}
	})
	return out
}

// controlUnder finds the control of a labelled field.
//
// A field is a heading, a control and a sentence, so the control is the object
// after the heading. Found by the label somebody reads rather than by position
// in the screen, so adding a field above does not silently point a guard at a
// different box.
func controlUnder(o fyne.CanvasObject, label string) fyne.CanvasObject {
	var found fyne.CanvasObject
	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || len(box.Objects) < 2 {
			return
		}
		if head, ok := box.Objects[0].(*widget.Label); ok && head.Text == label {
			found = box.Objects[1]
		}
	})
	return found
}

// entryUnder is the box somebody types into for a labelled field.
//
// It looks inside a grouping as well as at the control itself, because a field
// can be a box with something beside it - the output directory is a box and a
// button to browse with - and a guard should not have to know which fields are
// which shape.
func entryUnder(t *testing.T, o fyne.CanvasObject, label string) *widget.Entry {
	t.Helper()
	control := controlUnder(o, label)
	if entry, ok := control.(*widget.Entry); ok {
		return entry
	}
	var found *widget.Entry
	walk(control, func(obj fyne.CanvasObject) {
		if entry, ok := obj.(*widget.Entry); ok && found == nil {
			found = entry
		}
	})
	if found == nil {
		t.Fatalf("the field %q is %T and holds no box to type in", label, control)
	}
	return found
}
