package parts

import (
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The list a menu drops down, drawn by us rather than by the toolkit.
//
// It is a whole control rather than another adjustment, and that is the answer
// to a question this project has now been asked twice - reported from the
// screen on 2026-08-12 and again on 2026-08-18. The first answer was to tighten
// theme.SizeNameInnerPadding from 8 to 6, and the note left in theme.go that
// day says exactly why it could not be enough:
//
//	the theme is asked for a size by NAME and not by widget, so a menu cannot
//	be tightened on its own. That makes it a change to the density of
//	everything - a box to type in is text plus twice this, and so is a button.
//
// Measured off the stored tree before this existed: thirteen formats made a
// list 476 px tall, a row of 31 px repeating every 37, so 78 px of it was the
// gap between rows and nothing else. It covered the whole form beneath it. At
// the twenty-five formats T1 is heading for it would have been about 925 px,
// which does not fit in the window at all.
//
// What a control of our own buys, none of which the toolkit's menu offers:
// a row height we choose, a ceiling with scrolling under it, the letter a
// person types taken as a jump, and the value in the box marked in a list we
// can read back. See docs/UX.md and OBSERVATIONS.md O92c and O92d.

// OpenList is the list of values a Chooser drops down.
type OpenList struct {
	widget.BaseWidget

	// listContents is what the list holds and what it has drawn - see
	// listcontents.go for why it is a type of its own.
	listContents

	// room is how much height the window has left for this list, or nought for
	// no limit. See LimitTo and MinSize.
	room float32
	// active is the row the keyboard is on, or -1 when it has not been used,
	// and shown is whether that is drawn.
	//
	// Two fields rather than one, and it is the same rule as everywhere else in
	// this window: a list opened with the mouse starts its arrow keys on the
	// value already in the box WITHOUT painting a bar across that row. The bar
	// says "the keyboard is here" and nobody has used the keyboard yet. Seen on
	// the first render of this control, where opening the list with a press
	// highlighted the current row as though it had been arrowed to.
	active int
	shown  bool

	take  func(value string, byKeyboard bool)
	close func(byKeyboard bool)

	// KindOf says what picture goes in front of one value, or nil for a list
	// whose values are not things of different kinds. Set from outside, because
	// only the screen putting values in knows what they are.
	KindOf func(string) fyne.Resource

	// filter is the box at the top that narrows the list, or nil for a list
	// without one. What is typed into it is listContents.typed. See WithFilter.
	filter *FilterBox
}

// NewOpenList builds the list. take is called with the value somebody settled
// on, and close when they left without settling on one.
func NewOpenList(options []string, chosen string, take func(string, bool), close func(bool)) *OpenList {
	l := &OpenList{active: -1, take: take, close: close,
		listContents: listContents{options: options, chosen: chosen, view: newRowView()}}
	l.entries = arrange(options, nil, "")
	l.ExtendBaseWidget(l)
	return l
}

// GroupUnder puts every value under a heading, the one headingOf gives it.
// Called before the list is shown, the way KindOf is set.
func (l *OpenList) GroupUnder(headingOf func(string) string) {
	l.headingOf = headingOf
	l.rearrange()
}

// WithFilter gives the list a box at the top that narrows it to the values
// holding what is typed there. The box takes the keyboard when the list
// opens - see Keyboard.
//
// Only a long list gets one, and which lists are long is the Chooser's call,
// not this list's: a filter over five values is a box to type in that
// answers a question nobody has.
func (l *OpenList) WithFilter() {
	l.filter = newFilterBox(l)
}

// Keyboard is what the keyboard goes to when the list opens: the filter box
// when there is one, the list itself when there is not.
func (l *OpenList) Keyboard() fyne.Focusable {
	if l.filter != nil {
		return l.filter
	}
	return l
}

// Filter is the box at the top of the list, or nil, for a guard to type into.
func (l *OpenList) Filter() *FilterBox { return l.filter }

// narrowTo is the filter box reporting what it now holds. The keyboard lands
// where landing says and the bar is drawn, because typing is using the
// keyboard - except when the box has been emptied, where the list goes back
// to how it opened: on the value in the box, with nothing drawn.
func (l *OpenList) narrowTo(typed string) {
	l.typed = typed
	l.rearrange()
	l.view.toTop()
	if strings.TrimSpace(typed) == "" {
		l.active = -1
		l.StartOn(l.chosen)
		return
	}
	if at := landing(l.entries, typed); at >= 0 {
		l.moveTo(at)
		return
	}
	l.active = -1
	l.view.show(len(l.entries), l.fill)
}

// fill puts one row of the arrangement into one row of the list. The row it is
// given is recycled - the same row shows whatever stands at its position in
// the arrangement of the moment - so every field is set every time: a row left
// holding the last value it had is the classic defect of a list that reuses
// its rows. It is drawn by whoever asked for the fill.
func (l *OpenList) fill(id int, r *ListRow) {
	entry := l.entries[id]
	r.label = entry.text
	r.heading = entry.kind != entryValue
	r.from, r.to = entry.from, entry.to
	r.kind = nil
	r.marked = false
	r.active = false
	r.onTap = nil
	r.hovered = r.hovered && !r.heading
	if entry.kind == entryValue {
		value := entry.text
		if l.KindOf != nil {
			r.kind = l.KindOf(value)
		}
		r.marked = l.isChosen(value)
		r.active = l.shown && id == l.active
		r.onTap = func() { l.take(value, false) }
	}
}

// Choice is one row of an open list, for a guard to read. Choosable is false
// on a heading and on the notice that nothing matched.
type Choice struct {
	Label     string
	Marked    bool
	Choosable bool
}

// MinSize is as wide as the widest value and as tall as all of them, cut to
// the room the list was told it has.
//
// The ceiling is the point. Without one the list is as tall as it likes, which
// at thirteen formats already covered the form and at twenty-five would not fit
// in the window. The ceiling is not worked out here, because it is a share of
// the window (ListCeiling) and the list does not know the window - the Chooser
// that opens it does, and tells it through LimitTo. Until 2026-09-15 a count
// of rows lived here instead, which made the list 224 px tall in every window
// there is (O203).
//
// All of them means the whole arrangement with nothing typed, and not what the
// filter has left. The list does not shrink while somebody types into it: a
// list that opened upward would pull its bottom edge away from the box it
// belongs to with every letter, and nothing on this screen jumps under a
// person's hands (GUI rule 3).
func (l *OpenList) MinSize() fyne.Size {
	rows := len(arrange(l.options, l.headingOf, ""))
	if rows < 1 {
		rows = 1
	}
	head := l.HeadHeight()
	height := head + float32(rows)*listRowHeight()
	// The room the window has left is the whole of the ceiling, and this has
	// to happen in MinSize rather than by resizing the popup afterwards,
	// because a popup is never laid out smaller than its content's minimum.
	// Measured on 2026-08-19: asking for 195 px around a list whose minimum
	// was 224 gave 224 (O113).
	if l.room > 0 && height > l.room {
		height = l.room
	}
	if height < head+listRowHeight() {
		height = head + listRowHeight()
	}
	return fyne.NewSize(l.view.scroll.MinSize().Width, height)
}

// HeadHeight is the room the filter box takes at the top of the list, with
// the space round it, or nought for a list without one. RoomForList is told
// it, so that the rows under it still end on a row's edge.
func (l *OpenList) HeadHeight() float32 {
	if l.filter == nil {
		return 0
	}
	return l.filter.MinSize().Height + 2*filterInset
}

// LimitTo tells the list how much room it has - the share of the window it
// may cover, cut further to the room on the side it opens on. Nought means no
// limit.
func (l *OpenList) LimitTo(room float32) { l.room = room }

// ListCeiling is how tall an open list may be in a window this tall: listShare
// of the height, in whole rows, and never less than one row.
//
// Whole rows rather than the share to the pixel, so that a list at its ceiling
// ends on a row's edge the way it did when the ceiling was a count - a row cut
// through the middle is what the room on one side of a box does to a list in
// a cramped window, and that is the emergency, not the everyday.
func ListCeiling(canvasHeight float32) float32 {
	row := listRowHeight()
	rows := float32(math.Floor(float64(canvasHeight * listShare / row)))
	if rows < 1 {
		rows = 1
	}
	return rows * row
}

func (l *OpenList) CreateRenderer() fyne.WidgetRenderer {
	l.view.drawn = true
	l.view.show(len(l.entries), l.fill)
	// The surface is drawn here rather than left to the popup, so that the
	// colour a guard measures for "an open list is told from the form behind
	// it" is the colour actually on the screen.
	rows := l.view.scroll
	if l.filter == nil {
		return widget.NewSimpleRenderer(container.NewStack(floatingSurface(), rows))
	}
	head := container.New(layout.NewCustomPaddedLayout(filterInset, filterInset, filterInset, filterInset), l.filter)
	return widget.NewSimpleRenderer(container.NewStack(floatingSurface(), container.NewBorder(head, nil, nil, nil, rows)))
}

// ListRowHeight is one row, for a guard asking how many rows fit in a height.
func ListRowHeight() float32 { return listRowHeight() }

// listRowHeight is one row, worked out rather than typed: whatever is taller
// out of the text and the mark, plus our own room above and below.
//
// The text measured rather than a label built and asked, since 2026-09-23:
// every row asks this whenever it is sized, and a label built each time was
// about 2 MB an opening that stayed for the minute the toolkit keeps a
// renderer (docs/GUI-MEMORY-2026-09-23.md section 2.5). The same number - a
// label's height less its inner padding is its text's.
func listRowHeight() float32 {
	text := fyne.MeasureText("Ag", Theme().Size(theme.SizeNameText), fyne.TextStyle{}).Height
	icon := Theme().Size(theme.SizeNameInlineIcon)
	tall := text
	if icon > tall {
		tall = icon
	}
	return tall + 2*rowPadding
}

// The keyboard, and it follows the pattern rather than our taste.
//
// The WAI-ARIA authoring practices for a combobox with a closed set say what a
// person expects: the arrows move and Enter accepts, Escape closes AND gives
// the keyboard back to the box, and a printable character moves to the values
// starting with it. NN/g says the same about typing a letter. The toolkit
// offers none of it - widget.Select.TypedRune is "intentionally left blank" and
// widget.PopUpMenu.TypedRune does nothing at all - so there was no letter jump
// in the box and none in the open list either. O92c.

func (l *OpenList) FocusGained() {}
func (l *OpenList) FocusLost()   {}

// TypedKey moves, takes and closes. Headings and the notice are stepped over:
// the keyboard only ever stands on a value somebody can take.
func (l *OpenList) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyDown:
		l.step(+1)
	case fyne.KeyUp:
		l.step(-1)
	case fyne.KeyHome:
		l.moveTo(edgeValue(l.entries, +1))
	case fyne.KeyEnd:
		l.moveTo(edgeValue(l.entries, -1))
	case fyne.KeyEscape:
		l.close(true)
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		if l.active >= 0 && l.active < len(l.entries) && l.entries[l.active].kind == entryValue {
			l.take(l.entries[l.active].text, true)
		}
	}
}

// step moves the keyboard one value up or down. From nowhere, either arrow
// goes to the first value - which is where Down went before this list had
// headings, and Up from nowhere was clamped to the same row.
func (l *OpenList) step(by int) {
	if l.active < 0 {
		l.moveTo(edgeValue(l.entries, +1))
		return
	}
	l.moveTo(nextValue(l.entries, l.active, by))
}

// TypedRune jumps to the next value starting with the letter typed.
//
// From the row after the current one and wrapping, so pressing the same letter
// again walks through the values that share it - the behaviour of every desktop
// menu. One letter rather than a typed prefix: a prefix needs a timer to know
// when the word ended, and a timer in a control is a thing that behaves
// differently on a slow machine.
//
// A list with a filter box has a better answer than one letter, so a letter
// that reaches the list itself - the keyboard moved off the box with Tab - is
// put in the box, and the box gets the keyboard back.
func (l *OpenList) TypedRune(r rune) {
	if l.filter != nil {
		l.filter.TypedRune(r)
		if surface := fyne.CurrentApp().Driver().CanvasForObject(l); surface != nil {
			surface.Focus(l.filter)
		}
		return
	}
	want := strings.ToLower(string(r))
	for step := 1; step <= len(l.entries); step++ {
		at := (l.active + step) % len(l.entries)
		if at < 0 {
			at += len(l.entries)
		}
		if e := l.entries[at]; e.kind == entryValue && strings.HasPrefix(strings.ToLower(e.text), want) {
			l.moveTo(at)
			return
		}
	}
}

// moveTo puts the keyboard on one row and scrolls it into view. Clamped rather
// than wrapped, because a list that jumps from the last value to the first
// under a held arrow key is a list somebody overshoots in both directions.
//
// A value standing first under its heading brings the heading into view with
// it, so arrowing up to the top of a kind shows what kind it was.
func (l *OpenList) moveTo(at int) {
	if at < 0 || len(l.entries) == 0 {
		return
	}
	if at >= len(l.entries) {
		at = len(l.entries) - 1
	}
	l.active = at
	l.shown = true
	if at > 0 && l.entries[at-1].kind == entryHeading {
		l.view.bringIntoView(at - 1)
	}
	l.view.bringIntoView(at)
	l.view.show(len(l.entries), l.fill)
}

// Active is the row the keyboard is on, or -1, for a guard.
func (l *OpenList) Active() int { return l.active }

// StartOn puts the arrow keys on the value already in the box, without drawing
// it, so that opening a list of thirteen and pressing Down once does not go to
// the first value while the box shows the ninth.
func (l *OpenList) StartOn(value string) {
	for i, e := range l.entries {
		if e.kind == entryValue && e.text == value {
			l.moveTo(i)
			l.shown = false
			break
		}
	}
	// Drawn whether or not the value was found. A box holding no value finds
	// nothing, and an emptied filter lands here with the rows of what it had
	// narrowed to still drawn.
	l.view.show(len(l.entries), l.fill)
}

// Showing says whether the keyboard position is drawn, for a guard.
func (l *OpenList) Showing() bool { return l.shown }
