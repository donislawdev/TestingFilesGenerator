package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The words in an open list start where the word in the box does, and the
// tick stands at the far end of the row.
//
// Reported by the owner from the running window on 2026-09-16: the list of
// formats looked right and the lists of outcomes and rules looked like words
// floating in a rectangle. Measured: every row kept a column for the tick in
// front of its words whether or not anything in the list was ticked, so the
// words of every list stood 36 px to the right of the word in the box - the
// pictures in front of the formats made that look meant, and a list with no
// picture and nothing chosen showed the empty column for what it was (O220).
//
// Asked of two real lists on the screen: one without pictures, where the row's
// words must start within a step of the box's word, and one with them, where
// the picture stands in front and the tick behind. Positions are read off the
// canvas, because a row laid out at one width in a probe says nothing about
// the width the form gives it.
func TestTheWordsInAnOpenListStartWhereTheWordInTheBoxDoes(t *testing.T) {
	cv, content := screenOnACanvas(t)
	drv := fyne.CurrentApp().Driver()

	for _, tc := range []struct {
		field    string
		pictured bool
	}{
		{text.FieldDamage(), false},
		{text.FieldFormat(), true},
	} {
		menu := chooserUnder(t, content, tc.field)
		menu.Tapped(&fyne.PointEvent{})
		cv.Capture()
		list := menu.Opened()
		if list == nil {
			t.Fatalf("pressing the %s menu opened no list", tc.field)
		}
		boxWord := drv.AbsolutePositionForObject(wordsInTheBox(t, menu)).X

		rows := list.DrawnRows()
		if len(rows) == 0 {
			t.Fatalf("the %s list is drawing no row at all", tc.field)
		}
		for _, row := range rows {
			words, tick, picture := piecesOfARow(t, row)
			rowWord := drv.AbsolutePositionForObject(words).X
			if !tc.pictured && (rowWord < boxWord-parts.GapInline || rowWord > boxWord+parts.GapInline) {
				t.Errorf("%s: the words of row %q start at %.1f and the word in the box at %.1f - the list reads as words floating in a rectangle",
					tc.field, row.Label(), rowWord, boxWord)
			}
			if tick.Position().X < words.Position().X+words.Size().Width {
				t.Errorf("%s: the tick of row %q stands at %.1f, in front of words ending at %.1f - the column it keeps pushes every list's words off the box's word",
					tc.field, row.Label(), tick.Position().X, words.Position().X+words.Size().Width)
			}
			if tc.pictured && picture.Position().X+picture.Size().Width > words.Position().X {
				t.Errorf("%s: the picture of row %q reaches %.1f, over words starting at %.1f",
					tc.field, row.Label(), picture.Position().X+picture.Size().Width, words.Position().X)
			}
		}
		list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	}
}

// wordsInTheBox is the text the closed menu draws, read off its renderer -
// the toolkit draws it through a RichText, and a guard that cannot find it
// says so rather than measuring nothing.
func wordsInTheBox(t *testing.T, menu *parts.Chooser) *canvas.Text {
	t.Helper()
	for _, o := range test.WidgetRenderer(menu).Objects() {
		rich, is := o.(*widget.RichText)
		if !is {
			continue
		}
		for _, drawn := range test.WidgetRenderer(rich).Objects() {
			if words, is := drawn.(*canvas.Text); is {
				return words
			}
		}
	}
	t.Fatal("the closed menu draws no text this guard can find, so it cannot say where the word in the box starts")
	return nil
}

// piecesOfARow is what one row of a list draws: its words, its tick and its
// picture, the last two told apart by what they show - the tick is the
// toolkit's confirm icon, and the picture is whatever kind the row has.
func piecesOfARow(t *testing.T, row *parts.ListRow) (words *canvas.Text, tick, picture *canvas.Image) {
	t.Helper()
	for _, o := range test.WidgetRenderer(row).Objects() {
		switch drawn := o.(type) {
		case *canvas.Text:
			words = drawn
		case *canvas.Image:
			if drawn.Resource != nil && drawn.Resource.Name() == theme.ConfirmIcon().Name() {
				tick = drawn
			} else if row.Kind() != nil {
				picture = drawn
			}
		}
	}
	if words == nil || tick == nil {
		t.Fatalf("row %q draws no words or no tick, so this guard read the wrong objects", row.Label())
	}
	if row.Kind() != nil && picture == nil {
		t.Fatalf("row %q has a kind and draws no picture for it", row.Label())
	}
	return words, tick, picture
}
