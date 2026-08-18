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
// The threshold is the same shape as the one for the stacked surfaces below the
// page: a menu is the thing furthest from everything, so it has to clear the
// highest surface it opens over rather than merely differ from the page.
func TestAnOpenListIsToldFromTheFormBehindIt(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		page := parts.PaletteColour(theme.ColorNameBackground, variant.v)
		panel := parts.PaletteColour(parts.ColorNamePanel, variant.v)
		input := parts.PaletteColour(theme.ColorNameInputBackground, variant.v)
		menu := parts.PaletteColour(theme.ColorNameMenuBackground, variant.v)

		// The furthest surface it can open over. On the dark palette that is an
		// input box, on the light one it is the page - which is why this is
		// asked as "the highest of them" rather than named.
		highest, from := panel, "the panel"
		if lightnessGap(input, page) > lightnessGap(panel, page) {
			highest, from = input, "an input box"
		}

		gap := lightnessGap(menu, highest)
		// Half of what separates the page from an input box. The stack below is
		// held to a third each, and a thing that floats has to do better than a
		// thing that lies flat.
		least := lightnessGap(input, page) / 2
		if gap < least {
			t.Errorf("%s: an open list is %.1f L* from %s it opens over, and %.1f is the least that reads as floating",
				variant.name, gap, from, least)
		}

		// And what is written on it stays readable. A surface that moved
		// without its text being re-measured is the defect the palette guard
		// caught on its first day.
		if got := contrast(parts.PaletteColour(theme.ColorNameForeground, variant.v), menu); got < 4.5 {
			t.Errorf("%s: the values in an open list are %.2f:1 on it, under the 4.5 a reader needs",
				variant.name, got)
		}
		t.Logf("%s: an open list is %.1f L* above %s, values on it at %.2f:1",
			variant.name, gap, from, contrast(parts.PaletteColour(theme.ColorNameForeground, variant.v), menu))
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

	host := &fakeHost{}
	window.Open(host)
	content := tabNamed(t, host.content, text.TabOneTarget)

	w := test.NewWindow(host.content)
	defer w.Close()
	w.Resize(window.OpenSize)
	host.content.Refresh()

	picker, ok := controlUnder(content, text.FieldFormat).(*parts.Chooser)
	if !ok {
		t.Fatalf("the format field is %T rather than a menu", controlUnder(content, text.FieldFormat))
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
	if len(rows) != len(picker.Options) {
		t.Errorf("the list holds %d values and the menu offers %d", len(rows), len(picker.Options))
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
