package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// An open list floats above the form rather than merging into it.
//
// Reported from a screenshot on 2026-08-12 as a list where everything runs
// together, and it was. Measured off that render: the menu was painted
// #2E2E30, which is the input colour to the byte, over a panel at #262628 -
// 3.8 L* apart, with no border and a shadow in whatever colour the toolkit
// defaults to. Three surfaces within four L* of each other, one of them
// floating.
//
// Until 2026-09-24 this was held by lightness: the list stood on the lightest
// surface of the palette with no edge and no shade, and had to clear the
// highest surface it opens over by half of what separates the page from a box.
// The owner's report from the running window was that it looked like a plain
// grey block, and of three looks drawn side by side he chose a card: a box's
// surface, a box's edge, a panel's corner and a shade below it. So the list is
// told from the form by its edge and its shade now, and this asks for those on
// the list as drawn - the palette colour it used to measure is one the list no
// longer paints, and a guard of it would have gone on passing about nothing.
func TestAnOpenListIsToldFromTheFormBehindIt(t *testing.T) {
	_, _, list, _ := openFormatList(t)
	floatsAsACard(t, test.WidgetRenderer(list).Objects()[0], "an open list")

	// And what is written on it stays readable. A surface that moved without
	// its text being re-measured is the defect the palette guard caught on its
	// first day.
	face := parts.PaletteColour(theme.ColorNameInputBackground, theme.VariantDark)
	if got := contrast(parts.PaletteColour(theme.ColorNameForeground, theme.VariantDark), face); got < 4.5 {
		t.Errorf("the values in an open list are %.2f:1 on it, under the 4.5 a reader needs", got)
	}
}

// The open list says which value is the one in the box.
//
// The toolkit's own menu does not. widget.Select builds its items and never
// sets Checked - select.go, showPopUp - so thirteen formats opened with nothing
// marked, and the only place the answer existed was the box now covered by the
// list. Reported from a screenshot.
//
// It opens the list the way somebody does and reads what is on the canvas,
// rather than asking the widget what it would build. A menu built correctly and
// never shown looks identical from the widget's side.
func TestTheOpenListMarksTheValueThatIsChosen(t *testing.T) {
	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(parts.Theme())

	host := newFakeHost(t)
	window.Open(host)
	content := tabNamed(t, host.content, text.TabOneTarget())

	w := test.NewWindow(host.content)
	defer w.Close()
	w.Resize(window.LargestOpening)
	host.content.Refresh()

	picker, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
	if !ok {
		t.Fatalf("the format field is %T rather than a menu", controlUnder(content, text.FieldFormat()))
	}
	const chosen = "png"
	picker.SetSelected(chosen)
	picker.Tapped(&fyne.PointEvent{})

	// Two questions rather than one. The canvas says a list actually appeared,
	// and the menu that was built says what is in it - and neither alone is
	// worth anything: a list built correctly and never shown looks right from
	// the widget's side, and a list on screen says nothing about what is
	// marked, because the toolkit turns the items into widgets of a type it
	// does not export.
	if !listIsOpenOn(w.Canvas()) {
		t.Fatal("tapping the format menu put nothing on the canvas, so nothing opened")
	}

	list := picker.Opened()
	if list == nil {
		t.Fatal("the menu built no list at all")
	}

	// The list is asked what it holds, and one function answers that question
	// and fills the rows - see OpenList.isChosen. Two copies of the rule was
	// the state this was in for an hour, and a mutation blanking the drawn mark
	// left this guard green.
	//
	// What is DRAWN is held by the stored picture and the stored tree, where
	// the tick appears as a confirmIcon on one row. Neither guard covers the
	// other: this one reaches the values below the ceiling, which no picture
	// can show, and the picture reaches the drawing, which no list can promise.
	rows := list.Rows()
	// Values only: since 2026-09-23 the list of formats carries a heading
	// over each kind, which is a row and not a value.
	values := 0
	for _, row := range rows {
		if row.Choosable {
			values++
		}
	}
	if values != len(picker.Options) {
		t.Errorf("the list holds %d values and the menu offers %d", values, len(picker.Options))
	}
	marked := 0
	for _, row := range rows {
		if row.Marked {
			marked++
			if row.Label != chosen {
				t.Errorf("the list marks %q and the box holds %q", row.Label, chosen)
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d value(s) are marked in a list of %d, and exactly one is in the box",
			marked, len(rows))
	}
}

// listIsOpenOn says whether a list is showing over a canvas.
//
// It asks for OUR list rather than the toolkit's popup menu, changed on
// 2026-08-18 when the list stopped being the toolkit's. That is not a rename:
// a guard still looking for widget.PopUpMenu would report "nothing opened"
// however well the new one worked, which is the failure that reads as a defect
// and is not one.
func listIsOpenOn(c fyne.Canvas) bool {
	for _, overlay := range c.Overlays().List() {
		found := false
		walk(overlay, func(obj fyne.CanvasObject) {
			if _, ok := obj.(*parts.OpenList); ok {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}
