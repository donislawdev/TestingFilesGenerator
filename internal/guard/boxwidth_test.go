package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// A box is a promise about what goes in it.
//
// Measured off a render on 2026-08-20: the width and height boxes of a BMP
// were 806 px wide for a whole number from 1 to 20000, on a screen where "how
// many" - also a whole number - was 140. Two boxes on one screen, the same
// kind of value, six times the difference. A box running most of the window
// while holding four digits promises something the field cannot take, and it
// costs a full row of height each.
//
// Asked of every format the registry holds rather than of BMP, because the
// width comes from the declared kind and nothing here names a format.
func TestABoxForANumberIsNotAsWideAsTheForm(t *testing.T) {
	generate, choose := formatsLaidOut(t)

	checked := 0
	for _, d := range format.All() {
		choose.to(d.ID)
		for _, p := range d.Properties {
			if p.Kind != format.PropertyInt && p.Kind != format.PropertySize {
				continue
			}
			control := controlUnder(generate, p.Name)
			if control == nil {
				t.Errorf("%s declares %q and the window draws no field for it", d.ID, p.Name)
				continue
			}
			width := typedInWidth(control)
			if width > float32(parts.ColumnWidth)/2 {
				t.Errorf("%s.%s holds a %s and its box is %.0f px of a %.0f px column."+
					" A box that wide promises something the value cannot be",
					d.ID, p.Name, p.Kind, width, float32(parts.ColumnWidth))
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a number or a size, so this guard checked nothing")
	}
	t.Logf("%d boxes for a number or a size, none of them the width of the form", checked)
}

// And it is wide enough for what it is already showing.
//
// The other half, and it is here because shrinking the boxes broke it on the
// first try: at the width every other number box uses, "worked out from the
// size" was cut off mid-word. A box too narrow to show its own placeholder is
// the same defect as one six times too wide - it is not the width of what goes
// in it.
func TestABoxIsWideEnoughForItsOwnPlaceholder(t *testing.T) {
	generate, choose := formatsLaidOut(t)

	checked := 0
	for _, d := range format.All() {
		choose.to(d.ID)
		for _, p := range d.Properties {
			if p.Kind != format.PropertyInt && p.Kind != format.PropertySize {
				continue
			}
			control := controlUnder(generate, p.Name)
			if control == nil {
				continue
			}
			hint := placeholderShownFor(p)
			words := fyne.MeasureText(hint, theme.TextSize(), fyne.TextStyle{})
			if got := typedInWidth(control); got < words.Width {
				t.Errorf("%s.%s shows %q, which needs %.0f px, in a box %.0f px wide - so it is cut off",
					d.ID, p.Name, hint, words.Width, got)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a number or a size, so this guard checked nothing")
	}
}

// placeholderShownFor is what an untouched box of this setting says, worked out
// the same way the window works it out.
//
// A copy of one line rather than a call, because the window's own is
// unexported - and it is the string the guard has to know to ask its question
// at all. TestTheWindowDrawsAFieldForEveryDeclaredProperty already pins that
// the sentence under the field comes from the declaration, so the two cannot
// drift apart in what they mean.
func placeholderShownFor(p format.Property) string {
	if p.Default == "" {
		return text.PlaceholderWorkedOut
	}
	return text.PlaceholderLeftEmpty(p.Default)
}

// formatsLaidOut opens the window, puts it on a canvas and hands back a way to
// change format that leaves the screen laid out afterwards.
//
// Laid out is the point. The first version of this guard read MinSize, and a
// bare entry asks for very little however wide it is drawn - so a mutation
// taking the width control away entirely left it green. What a person sees is
// the size the layout gave the box, which only exists once something has been
// laid out, and choosing a format rebuilds that part of the tree.
func formatsLaidOut(t *testing.T) (fyne.CanvasObject, *formatChooser) {
	t.Helper()
	ourTheme(t)
	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(window.OpenSize.Width, 1600))

	generate := tabNamed(t, host.content, text.TabOneTarget)
	picker, ok := controlUnder(generate, text.FieldFormat).(*parts.Chooser)
	if !ok {
		t.Fatal("the format field is not a list to choose from, so this guard read the wrong tree")
	}
	return generate, &formatChooser{t: t, w: w, picker: picker}
}

type formatChooser struct {
	t      *testing.T
	w      fyne.Window
	picker *parts.Chooser
}

// to chooses a format and lays the screen out again. Resizing to the size it
// already is does nothing, so it goes past and comes back.
func (c *formatChooser) to(id string) {
	c.t.Helper()
	c.picker.SetSelected(id)
	c.w.Resize(fyne.NewSize(window.OpenSize.Width, 1599))
	c.w.Resize(fyne.NewSize(window.OpenSize.Width, 1600))
}

// typedInWidth is how wide the box a person types into ended up.
//
// The registered control is a wrapper - Sized decides the width and is itself
// stretched to the column by the layout above it - so asking the registered
// object its size answers about the wrapper and reports the full width for
// every field. The same trap parts.inside exists for, and it cost this guard
// a run: it reported nine boxes at 808 px of an 820 px column while the render
// beside it showed them at 178.
func typedInWidth(control fyne.CanvasObject) float32 {
	widest := float32(0)
	var step func(o fyne.CanvasObject)
	step = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Entry:
			if v.Size().Width > widest {
				widest = v.Size().Width
			}
		case *fyne.Container:
			for _, child := range v.Objects {
				step(child)
			}
		}
	}
	step(control)
	if widest == 0 {
		return control.Size().Width
	}
	return widest
}
