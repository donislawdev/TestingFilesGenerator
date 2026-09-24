package guard

import (
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The list of formats names every format beside its identifier, since
// 2026-09-24 - "jxl" says nothing to somebody who has not met it, "JPEG XL"
// does. Decided by the owner, with the list wider than the box it drops from
// and the box itself as narrow as before. Recorded in
// docs/FORMAT-NAMES-2026-09-24.md.

// drawnWords is every visible piece of text a row draws, in its order.
func drawnWords(row *parts.ListRow) []*canvas.Text {
	var out []*canvas.Text
	for _, o := range test.WidgetRenderer(row).Objects() {
		if words, ok := o.(*canvas.Text); ok && words.Visible() && words.Text != "" {
			out = append(out, words)
		}
	}
	return out
}

// TestTheFormatListNamesEveryFormatBesideItsIdentifier reads the rows as
// drawn: every value row carries the registry's name for its value, the names
// start on one line down the list, clear of the widest identifier, and they
// are drawn a step quieter than the identifier - the identifier is what lands
// in the box.
func TestTheFormatListNamesEveryFormatBesideItsIdentifier(t *testing.T) {
	_, _, list, _ := openFormatList(t)
	quiet := parts.PaletteColour(parts.ColorNameLabel, theme.VariantDark)
	loud := parts.Theme().Color(theme.ColorNameForeground, theme.VariantDark)

	var column float32 = -1
	named := 0
	for _, row := range list.DrawnRows() {
		if row.Heading() {
			continue
		}
		d, err := format.Get(row.Label())
		if err != nil {
			t.Fatalf("the format list draws %q, which is not a registered format", row.Label())
		}
		words := drawnWords(row)
		if len(words) != 2 || words[0].Text != d.ID || words[1].Text != d.Name {
			texts := make([]string, 0, len(words))
			for _, w := range words {
				texts = append(texts, w.Text)
			}
			t.Errorf("the row for %s draws %q, where it should draw the identifier and then %q", d.ID, texts, d.Name)
			continue
		}
		named++
		value, name := words[0], words[1]
		if column < 0 {
			column = name.Position().X
		}
		if name.Position().X != column {
			t.Errorf("the name of %s starts at x=%.1f and the first name at x=%.1f - the names are not one column",
				d.ID, name.Position().X, column)
		}
		if ends := value.Position().X + fyne.MeasureText(value.Text, value.TextSize, fyne.TextStyle{Bold: true}).Width; name.Position().X <= ends {
			t.Errorf("the name of %s starts at x=%.1f, inside the room its identifier takes in bold, which ends at x=%.1f",
				d.ID, name.Position().X, ends)
		}
		if name.Color != quiet || value.Color != loud {
			t.Errorf("the row for %s draws its identifier in %v and its name in %v - the name should be the quieter ink of a field's name (%v), the identifier the ink of a value (%v)",
				d.ID, value.Color, name.Color, quiet, loud)
		}
	}
	if named == 0 {
		t.Fatal("no row of the open format list drew a name, so nothing about the names was checked")
	}
}

// TestTheFormatListIsWiderThanItsBoxAndCutsNoName opens the list in the
// narrowest window each screen with a list of formats allows: the list is
// wider than the box it drops from, every row fits in it, and all of it is
// inside the window.
//
// The narrowest window because that is where a list wider than its box has
// the least room beside it. Screens rather than one: the format box stands in
// a different place on each.
func TestTheFormatListIsWiderThanItsBoxAndCutsNoName(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			content, w := screenInAWindow(t, tab)
			width := w.Content().MinSize().Width
			if w.Padded() {
				width += 2 * theme.Padding()
			}
			w.Resize(fyne.NewSize(width, referenceHeight))
			content.Refresh()
			w.Resize(fyne.NewSize(width, referenceHeight))

			menu := chooserUnder(t, content, text.FieldFormat())
			menu.Tapped(&fyne.PointEvent{})
			list := menu.Opened()
			pop := popUpIn(w.Canvas().Overlays().Top())
			if list == nil || pop == nil {
				t.Fatal("pressing the format menu put no list on the canvas")
			}
			box := menu.Size().Width
			if pop.Size().Width <= box {
				t.Errorf("the list is %.1f px wide under a box %.1f px wide - the names beside the formats need it wider",
					pop.Size().Width, box)
			}
			left, right := pop.Position().X, pop.Position().X+pop.Size().Width
			if left < 0 || right > w.Canvas().Size().Width {
				t.Errorf("the list runs from x=%.1f to x=%.1f in a window %.1f px wide", left, right, w.Canvas().Size().Width)
			}
			rows := 0
			for _, row := range list.DrawnRows() {
				rows++
				if need := row.MinSize().Width; row.Size().Width < need-0.5 {
					t.Errorf("the row for %q is %.1f px wide and needs %.1f, so its words are cut off",
						row.Label(), row.Size().Width, need)
				}
			}
			if rows == 0 {
				t.Fatal("the open list drew no rows, so none could be measured")
			}
		})
	}
}

// TestAListWiderThanTheRoomBesideItsBoxMovesLeft asks the arithmetic directly.
// Every box with a list wider than itself stands in a form's first column
// today, so no screen reaches the move and a screen level guard would be
// green without having been there.
func TestAListWiderThanTheRoomBesideItsBoxMovesLeft(t *testing.T) {
	const canvasWidth, box, wide = 800, 180, 340

	if left, width := parts.ColumnForList(canvasWidth, 20, box, wide); left != 20 || width != wide {
		t.Errorf("a list with room beside its box went to x=%.1f at %.1f px, where it belongs at x=20 at %d", left, width, wide)
	}

	left, width := parts.ColumnForList(canvasWidth, 600, box, wide)
	switch {
	case width != wide:
		t.Errorf("a list with room in the window was cut from %d to %.1f px", wide, width)
	case left >= 600:
		t.Errorf("a list %d px wide under a box at x=600 stayed at x=%.1f and ends at %.1f in a window %d px wide",
			wide, left, left+width, canvasWidth)
	case left+width >= canvasWidth:
		t.Errorf("a list moved left ends at x=%.1f, on the edge of a window %d px wide rather than inside it", left+width, canvasWidth)
	}

	// No wider than its box: it stands where it always has, even with the box
	// closer to the edge than the gap a wider list keeps.
	for _, at := range []float32{600, canvasWidth - box} {
		if left, width := parts.ColumnForList(canvasWidth, at, box, box); left != at || width != box {
			t.Errorf("a list as wide as its box at x=%.1f went to x=%.1f at %.1f px", at, left, width)
		}
	}

	// Wider than the whole window: cut to it, with the gap kept on both sides.
	const narrow = 300
	if left, width := parts.ColumnForList(narrow, 10, box, wide); left <= 0 || left+width >= narrow || width < box {
		t.Errorf("a list %d px wide in a window %d px wide went from x=%.1f to x=%.1f", wide, narrow, left, left+width)
	}
}

// TestTheFilterFindsAFormatByAWordOfItsName types words of names. A word of
// the name counts from its START, the rule the headings follow: "a" keeps avif
// for AV1, and does not keep gif for the a inside Graphics - anywhere in the
// name, one letter would keep most of the list. Words part at anything that is
// not a letter or a digit, so "office" finds "(Office" and "separated" finds
// "Comma-Separated".
func TestTheFilterFindsAFormatByAWordOfItsName(t *testing.T) {
	_, _, list, filter := openFormatList(t)
	for typed, want := range map[string][]string{
		"excel":     {"xlsx"},
		"office":    {"docx", "pptx", "xlsx"},
		"separated": {"csv"},
		"vector":    {"svg"},
	} {
		typeInto(filter, typed)
		got := valuesOf(list)
		slices.Sort(got)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%q typed keeps %v, where the names say %v", typed, got, want)
		}
	}
	typeInto(filter, "a")
	got := valuesOf(list)
	if !slices.Contains(got, "avif") {
		t.Errorf("a typed does not keep avif, whose name starts with AV1: %v", got)
	}
	if slices.Contains(got, "gif") {
		t.Errorf("a typed keeps gif, for the a inside Graphics Interchange Format - a name counts from the start of its words: %v", got)
	}
}

// TestTheListSaysWhatToDoWhenNothingMatches types what no format holds. The
// one row left is the sentence from the text package, which says what to do as
// well as what happened (review of #127), drawn as a sentence - the size of
// one and not bold - rather than as the heading of a group with nothing under
// it, and not cut off by the list it stands in.
func TestTheListSaysWhatToDoWhenNothingMatches(t *testing.T) {
	_, _, list, filter := openFormatList(t)
	typeInto(filter, "zz")
	rows := list.DrawnRows()
	if len(rows) != 1 || !rows[0].Heading() || rows[0].Label() != text.ListNothingMatches() {
		labels := make([]string, 0, len(rows))
		for _, r := range rows {
			labels = append(labels, r.Label())
		}
		t.Fatalf("zz typed leaves %q, where it should leave the one sentence %q", labels, text.ListNothingMatches())
	}
	words := drawnWords(rows[0])
	if len(words) != 1 {
		t.Fatalf("the sentence is drawn as %d pieces of text", len(words))
	}
	sentence := words[0]
	if sentence.TextStyle.Bold || sentence.TextSize != parts.Theme().Size(theme.SizeNameText) {
		t.Errorf("the sentence is drawn at %.0f px, bold %v - the look of a heading rather than of a sentence",
			sentence.TextSize, sentence.TextStyle.Bold)
	}
	if need := fyne.MeasureText(sentence.Text, sentence.TextSize, sentence.TextStyle).Width; sentence.Position().X+need > rows[0].Size().Width {
		t.Errorf("the sentence needs %.1f px from x=%.1f in a row %.1f px wide, so it is cut off",
			need, sentence.Position().X, rows[0].Size().Width)
	}
}
