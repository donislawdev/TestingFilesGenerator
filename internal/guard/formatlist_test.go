package guard

import (
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The list of formats, grouped by kind and filtered by what is typed, since
// 2026-09-23. Twenty six formats of which eighteen fitted in the open list,
// and the owner's decisions on the running window: a heading over each kind,
// a box at the top that narrows the list, the letters that matched in bold,
// a count beside each heading, and a letter typed at the shut menu opening
// the list with that letter in the box. docs/FORMAT-MENU-2026-09-23.md has
// the analysis, the measurements and what was turned down.

// openFormatList presses the format menu of the single batch screen and
// answers the menu, the list it opened and the box at the top of the list -
// failing, rather than answering nil, when any of the three is missing.
func openFormatList(t *testing.T) (fyne.Canvas, *parts.Chooser, *parts.OpenList, *parts.FilterBox) {
	t.Helper()
	c, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	if !menu.Filtered {
		t.Fatal("the format menu has no filter, so every guard in this file would be asking about another list")
	}
	menu.Tapped(&fyne.PointEvent{})
	list := menu.Opened()
	if list == nil {
		t.Fatal("pressing the format menu opened no list")
	}
	filter := list.Filter()
	if filter == nil {
		t.Fatal("the list of formats opened without the box that narrows it")
	}
	return c, menu, list, filter
}

// typeInto replaces what the filter box holds, the way a person clearing it
// and typing would end up.
func typeInto(filter *parts.FilterBox, typed string) {
	filter.SetText("")
	for _, r := range typed {
		filter.TypedRune(r)
	}
}

// valuesOf is the values a list is showing, headings and the notice left out.
func valuesOf(list *parts.OpenList) []string {
	var out []string
	for _, row := range list.Rows() {
		if row.Choosable {
			out = append(out, row.Label)
		}
	}
	return out
}

// activeLabel is the value the keyboard is on, or "" when it is on nothing.
func activeLabel(list *parts.OpenList) string {
	rows := list.Rows()
	if at := list.Active(); at >= 0 && at < len(rows) {
		return rows[at].Label
	}
	return ""
}

// TestTheFormatListStandsUnderAHeadingForEachKind asks the whole list, with
// nothing typed: every registered format exactly once, each under the heading
// of its own kind, the headings in the order of their words, and each heading
// counting the formats under it.
func TestTheFormatListStandsUnderAHeadingForEachKind(t *testing.T) {
	_, _, list, _ := openFormatList(t)

	// The rows cut into groups at each heading.
	type group struct {
		heading string
		values  []string
	}
	var groups []group
	for _, row := range list.Rows() {
		if !row.Choosable {
			groups = append(groups, group{heading: row.Label})
			continue
		}
		if len(groups) == 0 {
			t.Fatalf("%s stands above the first heading", row.Label)
		}
		groups[len(groups)-1].values = append(groups[len(groups)-1].values, row.Label)
	}
	if len(groups) < 2 {
		t.Fatalf("the list has %d heading(s), so there is no grouping to ask about", len(groups))
	}

	seen := map[string]int{}
	var titles []string
	for _, g := range groups {
		if len(g.values) == 0 {
			t.Errorf("the heading %q stands over nothing", g.heading)
			continue
		}
		title := parts.KindHeading(g.values[0])
		titles = append(titles, title)
		for _, v := range g.values {
			seen[v]++
			if parts.KindHeading(v) != title {
				t.Errorf("%s stands under %q with the %s, and it is one of the %s", v, g.heading, title, parts.KindHeading(v))
			}
		}
		if want := text.ListHeadingCount(title, len(g.values)); g.heading != want {
			t.Errorf("the heading over %d format(s) of one kind reads %q rather than %q", len(g.values), g.heading, want)
		}
	}
	for _, id := range format.IDs() {
		if seen[id] != 1 {
			t.Errorf("%s is in the list %d time(s), and every format belongs there once", id, seen[id])
		}
	}
	if len(seen) != len(format.IDs()) {
		t.Errorf("the list holds %d formats and the registry %d", len(seen), len(format.IDs()))
	}
	if !sort.StringsAreSorted(titles) {
		t.Errorf("the headings stand in the order %v, and a closed set is in the order of its words", titles)
	}
	// Counted, because a loop over the drawn rows passes just as well when no
	// heading was drawn at all - an outside review of #127 named it, and it is
	// trap 1 of CLAUDE.md: the guard has to be in the state it asks about.
	drawnHeadings := 0
	for _, row := range list.DrawnRows() {
		if !row.Heading() {
			continue
		}
		drawnHeadings++
		if row.Kind() != nil || row.Marked() {
			t.Errorf("the heading %q draws a picture or a tick, which says it is a value somebody can take", row.Label())
		}
	}
	if drawnHeadings == 0 {
		t.Fatal("no heading row is drawn, so nothing was asked about how a heading looks")
	}
}

// TestTypingIntoTheFormatListNarrowsItAndLandsOnWhatStartsWithIt types into
// the box and reads what is left.
//
// Anywhere in the name ("gz" keeps targz), and a word of the heading from its
// start ("pict" keeps every picture) but not from its middle - "t" is inside
// "Documents", and a filter that matched that would keep every document for
// one letter. The keyboard lands on the first value STARTING with what was
// typed, nothing is taken until Enter, and a filter that leaves nothing says
// so and takes nothing.
func TestTypingIntoTheFormatListNarrowsItAndLandsOnWhatStartsWithIt(t *testing.T) {
	_, menu, list, filter := openFormatList(t)
	before := menu.Selected

	typeInto(filter, "gz")
	if got := valuesOf(list); strings.Join(got, ",") != "targz" {
		t.Errorf("gz was typed and the list keeps %v, where targz is the one format holding it", got)
	}

	typeInto(filter, "p")
	if got := activeLabel(list); !strings.HasPrefix(got, "p") {
		t.Errorf("p was typed and the keyboard is on %q, not on a format starting with p", got)
	}

	typeInto(filter, "pict")
	pictures := 0
	for _, id := range format.IDs() {
		if parts.KindHeading(id) == text.ListKindPictures() {
			pictures++
		}
	}
	if got := valuesOf(list); len(got) != pictures {
		t.Errorf("pict was typed and the list keeps %d formats, where %d are pictures: %v", len(got), pictures, got)
	}

	typeInto(filter, "t")
	for _, v := range valuesOf(list) {
		if v == "xlsx" {
			t.Errorf("t was typed and xlsx is kept - neither its name nor a word of its heading starts with t, " +
				"so a heading is being matched in the middle of a word")
		}
	}

	typeInto(filter, "zz")
	if got := valuesOf(list); len(got) != 0 {
		t.Errorf("zz was typed and the list keeps %v", got)
	}
	rows := list.Rows()
	if len(rows) != 1 || rows[0].Choosable || rows[0].Label != text.ListNothingMatches() {
		t.Errorf("a filter that keeps nothing draws %v rather than saying nothing matches", rows)
	}
	filter.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if menu.Selected != before || menu.Opened() == nil {
		t.Errorf("Enter on a list with nothing in it took %q (the box held %q) or closed the list", menu.Selected, before)
	}
}

// TestTheArrowsInTheFormatListStepOverTheHeadings walks a narrowed list.
//
// "j" leaves jpg and jxl under one heading and json under another, so Down
// from jxl has a heading in its way - the cost the deferral of 2026-08-25
// wrote down, before there were any headings to step over.
func TestTheArrowsInTheFormatListStepOverTheHeadings(t *testing.T) {
	_, _, list, filter := openFormatList(t)
	typeInto(filter, "j")
	walk := []struct {
		key  fyne.KeyName
		want string
	}{
		{fyne.KeyDown, "jxl"},
		{fyne.KeyDown, "json"},
		{fyne.KeyDown, "json"},
		{fyne.KeyUp, "jxl"},
		{fyne.KeyUp, "jpg"},
		{fyne.KeyUp, "jpg"},
	}
	if got := activeLabel(list); got != "jpg" {
		t.Fatalf("j was typed and the keyboard is on %q, not on jpg", got)
	}
	for i, step := range walk {
		filter.TypedKey(&fyne.KeyEvent{Name: step.key})
		if got := activeLabel(list); got != step.want {
			t.Fatalf("step %d, %s: the keyboard is on %q, where %s is the next value", i+1, step.key, got, step.want)
		}
	}
}

// TestTypingAtTheShutFormatMenuOpensItsFilter types a whole name at the menu
// with its list shut. One letter at a time used to walk the values starting
// with each letter in turn, so "jxl" ended on log. Now the first letter opens
// the list with that letter in the box, the keyboard goes to the box, and the
// rest of the name follows it there.
func TestTypingAtTheShutFormatMenuOpensItsFilter(t *testing.T) {
	c, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	menu.TypedRune('j')
	list := menu.Opened()
	if list == nil || list.Filter() == nil {
		t.Fatal("a letter typed at the shut format menu opened no list with a box to narrow it")
	}
	filter := list.Filter()
	if filter.Text != "j" {
		t.Errorf("the box holds %q after j was typed at the shut menu", filter.Text)
	}
	if c.Focused() != fyne.Focusable(filter) {
		t.Errorf("the keyboard went to %T, so the next letter would not reach the box", c.Focused())
	}
	filter.TypedRune('x')
	filter.TypedRune('l')
	filter.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if menu.Selected != "jxl" {
		t.Errorf("jxl was typed at the shut menu and Enter pressed, and the menu holds %q", menu.Selected)
	}
}

// TestTheSpaceThatOpensTheFormatListIsNotTypedIntoItsFilter presses Space at
// the shut menu the way the driver delivers it: the key to whatever has the
// keyboard, then the character to whatever has it NOW (fyne v2.8.1
// internal/driver/glfw/window.go, processKeyPressed and processCharInput both
// ask canvas.Focused()). The key opens the list and hands the keyboard to the
// box, so the character landed in the box.
//
// Reported by an outside review of #127 and seen in the real window through
// tools/pilot.py before this was written: after Space the box lost its
// placeholder and the caret stood one space in, with nothing visible typed.
func TestTheSpaceThatOpensTheFormatListIsNotTypedIntoItsFilter(t *testing.T) {
	c, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	c.Focus(menu)
	c.Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	list := menu.Opened()
	if list == nil || list.Filter() == nil {
		t.Fatal("Space at the shut format menu opened no list with a box to narrow it, so this guard is not in the state it asks about")
	}
	if c.Focused() != fyne.Focusable(list.Filter()) {
		t.Fatalf("after Space the keyboard is on %T rather than in the box, so the character would not reach it", c.Focused())
	}
	c.Focused().TypedRune(' ')
	if got := list.Filter().Text; got != "" {
		t.Errorf("the Space that opened the list left %q in its box, which hides the placeholder and moves the caret", got)
	}
	// A space between words is still a space: only an empty box drops one.
	c.Focused().TypedRune('t')
	c.Focused().TypedRune(' ')
	if got := list.Filter().Text; got != "t " {
		t.Errorf("t then Space typed into the box left %q, and a space after a letter is somebody typing", got)
	}
}

// TestThePartOfAValueThatMatchedIsDrawnInBold reads a row's drawn words.
func TestThePartOfAValueThatMatchedIsDrawnInBold(t *testing.T) {
	_, _, list, filter := openFormatList(t)
	typeInto(filter, "gz")
	row := list.RowShowing("targz")
	if row == nil {
		t.Fatal("gz was typed and no row is drawing targz")
	}
	var bold, plain []string
	for _, o := range test.WidgetRenderer(row).Objects() {
		words, ok := o.(*canvas.Text)
		if !ok || !words.Visible() || words.Text == "" {
			continue
		}
		if words.TextStyle.Bold {
			bold = append(bold, words.Text)
		} else {
			plain = append(plain, words.Text)
		}
	}
	if strings.Join(bold, "") != "gz" || strings.Join(plain, "") != "tar" {
		t.Errorf("targz with gz typed draws %v in bold and %v plain, where gz is what matched", bold, plain)
	}
}

// TestTheShutFormatMenuDrawsThePictureOfItsValue reads the closed box: the
// picture of the value's kind in front of the words, and a new picture when
// the value changes. Until 2026-09-23 the kind was on the screen only while
// the list was open.
func TestTheShutFormatMenuDrawsThePictureOfItsValue(t *testing.T) {
	_, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	for _, value := range []string{"zip", "png"} {
		menu.SetSelected(value)
		var picture *canvas.Image
		var words *widget.RichText
		for _, o := range test.WidgetRenderer(menu).Objects() {
			switch drawn := o.(type) {
			case *canvas.Image:
				if drawn.Visible() && drawn.Resource != nil {
					picture = drawn
				}
			case *widget.RichText:
				words = drawn
			}
		}
		if picture == nil || picture.Resource.Name() != parts.KindOfFile(value).Name() {
			t.Errorf("the shut menu holding %s draws %v, not the picture of its kind", value, picture)
			continue
		}
		if words == nil || words.Position().X < picture.Position().X+picture.Size().Width {
			t.Errorf("the shut menu holding %s starts its words over the picture", value)
		}
	}
}
