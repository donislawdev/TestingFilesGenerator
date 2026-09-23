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

// The words in an open list without pictures start where the word in the box
// does, with the tick at the far end of the row - and a list WITH pictures
// keeps its tick in front, the picture next and the words after it.
//
// Reported by the owner from the running window on 2026-09-16: the list of
// formats looked right and the lists of outcomes and rules looked like words
// floating in a rectangle. Measured: every row kept a column for the tick in
// front of its words whether or not anything in the list was ticked, so the
// words of every list stood 36 px to the right of the word in the box - the
// pictures in front of the formats made that look meant, and a list with no
// picture and nothing chosen showed the empty column for what it was (O220).
//
// The second half is the owner's too, from 2026-09-21: the tick moved to the
// end of EVERY row on 2026-09-16, and on the list of formats that pulled the
// picture and the word a column to the left of where they had stood - "what
// was nicely in the middle is at the left edge again". So a pictured row keeps
// the shape it had before O220 - tick, picture, words - and only a row with
// nothing to draw in front of its words starts them at the gutter.
//
// Asked of two real lists on the screen: one without pictures, where the row's
// words start at the gutter with nothing in front of them, and one with them,
// where the tick's column stands at the gutter, the picture a column later
// and the words after it. Positions are read off rows the list is actually
// drawing, because a row laid out at one width in a probe says nothing about
// the width the form gives it.
//
// The rule is held against OUR geometry - the gutter, the tick, the picture -
// and the distance to the box's own word is only logged. The first version
// asserted that distance within a step of the scale, and CI turned it red:
// the toolkit draws the box's word 2 px from where the row's word starts on
// the owner's machine and 6 px on the runners (2026-09-16, all three systems),
// because the inset of a Select's RichText is the toolkit's and not a token
// of ours. What the owner saw was 36 px, and that is what the tick column in
// front of the words was.
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
			// A heading is its words at the gutter and nothing else - no tick,
			// no picture. Its shape is TestTheFormatListStandsUnderAHeadingForEachKind's.
			if row.Heading() {
				continue
			}
			words, tick, picture := piecesOfARow(t, row)
			if tc.pictured {
				// Tick, picture, words: each starts where the one before it
				// ends, a gap later, and the first of them at the gutter.
				if tick.Position().X != parts.RowGutter() {
					t.Errorf("%s: the tick of row %q stands at %.1f rather than at the gutter (%.1f) - the column that kept the picture and the word off the edge is gone",
						tc.field, row.Label(), tick.Position().X, parts.RowGutter())
				}
				if picture.Position().X <= tick.Position().X+tick.Size().Width {
					t.Errorf("%s: the picture of row %q stands at %.1f, not after the tick's column ending at %.1f",
						tc.field, row.Label(), picture.Position().X, tick.Position().X+tick.Size().Width)
				}
				if words.Position().X <= picture.Position().X+picture.Size().Width {
					t.Errorf("%s: the words of row %q start at %.1f, not after the picture ending at %.1f",
						tc.field, row.Label(), words.Position().X, picture.Position().X+picture.Size().Width)
				}
				continue
			}
			if words.Position().X != parts.RowGutter() {
				t.Errorf("%s: row %q starts its words at %.1f rather than at the gutter (%.1f) - a column stands in front of the words and the list reads as words floating in a rectangle",
					tc.field, row.Label(), words.Position().X, parts.RowGutter())
			}
			t.Logf("%s: row %q words at %.1f, the box's word at %.1f (the toolkit's inset, logged and not held)",
				tc.field, row.Label(), drv.AbsolutePositionForObject(words).X, boxWord)
			if tick.Position().X < words.Position().X+words.Size().Width {
				t.Errorf("%s: the tick of row %q stands at %.1f, in front of words ending at %.1f - the column it keeps pushes every list's words off the box's word",
					tc.field, row.Label(), tick.Position().X, words.Position().X+words.Size().Width)
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
			// The first text, which holds the words whole while nothing is
			// typed into the list's filter. Two more follow it for the bold
			// part and the rest, empty and hidden at rest.
			if words == nil {
				words = drawn
			}
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
