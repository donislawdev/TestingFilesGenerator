package parts

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
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

// visibleRows is how many rows are shown before the list starts scrolling.
//
// Eight, decided by the owner on 2026-08-18. NN/g puts it as a rule rather than
// a number - the label and the context stay in view while the list is open -
// and eight is what leaves most of the form visible at the window sizes this
// program opens at, including 800x600.
const visibleRows = 8

// rowPadding is the room above and below a row's contents.
//
// Ours rather than the theme's, which is the entire point of this control. The
// theme's innerPadding is what a box to type in and a button are also built
// from, so a list cannot be made denser through it without making every control
// on the form denser too.
const rowPadding = 4

// rowGutter is the room in front of the mark, and rowGap the room after it.
// Together they put the text where the toolkit put it, so this change moves the
// density and nothing else - one thing at a time.
const (
	rowGutter = 6
	rowGap    = 6
)

// OpenList is the list of values a Chooser drops down.
type OpenList struct {
	widget.BaseWidget

	options []string
	// room is how much height the window has left for this list, or nought for
	// no limit. See LimitTo and MinSize.
	room float32
	// chosen is the value in the box, marked with a tick. Empty when the box
	// shows a default nobody has confirmed - see the note in preset.go about a
	// filled field making "I did not say" impossible to express.
	chosen string
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

	list *widget.List
	// rows is the row showing each position, recorded as the list fills them.
	//
	// A registry rather than a walk, because a walk cannot get in: widget.List
	// keeps the rows it built inside its renderer, so a tree walk stops at the
	// list and reports an open list with nothing in it. Measured on 2026-08-18
	// while trying to photograph a row under the pointer.
	//
	// Every entry is current whatever the list has scrolled past, because a
	// recycled row is refilled before it is shown and fill is what writes here.
	rows map[widget.ListItemID]*ListRow

	// KindOf says what picture goes in front of one value, or nil for a list
	// whose values are not things of different kinds. Set from outside, because
	// only the screen putting values in knows what they are.
	KindOf func(string) fyne.Resource
}

// NewOpenList builds the list. take is called with the value somebody settled
// on, and close when they left without settling on one.
func NewOpenList(options []string, chosen string, take func(string, bool), close func(bool)) *OpenList {
	l := &OpenList{options: options, chosen: chosen, active: -1, take: take, close: close,
		rows: map[widget.ListItemID]*ListRow{}}
	l.list = widget.NewList(
		func() int { return len(l.options) },
		func() fyne.CanvasObject { return newListRow() },
		l.fill,
	)
	l.ExtendBaseWidget(l)
	return l
}

// fill puts one option into one row. The row it is given is recycled, so every
// field is set every time - a row left holding the last value it had is the
// classic defect of a list that only builds what it can see.
func (l *OpenList) fill(id widget.ListItemID, row fyne.CanvasObject) {
	r, ok := row.(*ListRow)
	if !ok || id < 0 || id >= len(l.options) {
		return
	}
	value := l.options[id]
	r.label = value
	r.kind = nil
	if l.KindOf != nil {
		r.kind = l.KindOf(value)
	}
	r.marked = l.isChosen(value)
	r.active = l.shown && id == l.active
	r.onTap = func() { l.take(value, false) }
	l.rows[id] = r
	r.Refresh()
}

// Rows is what this list is showing, for a guard to read.
//
// The toolkit's menu turned items into widgets of an unexported type, so what
// was marked could not be read back off the canvas - only that something was
// open. This is the half of that pair we own, and it says what is in the list
// and which row carries the tick.
func (l *OpenList) Rows() []Choice {
	out := make([]Choice, 0, len(l.options))
	for _, value := range l.options {
		out = append(out, Choice{Label: value, Marked: l.isChosen(value)})
	}
	return out
}

// isChosen says whether a value is the one in the box.
//
// One function rather than the same comparison written where a row is filled
// and again where the list reports itself. Two copies is what it was for an
// hour on 2026-08-18, and the mutation runner said so at once: blanking the
// drawn mark left the guard green, because the guard was reading the other
// copy. A rule with two homes is a rule no test can pin down.
func (l *OpenList) isChosen(value string) bool { return value == l.chosen }

// DrawnRows is every row the list has actually built, for a guard that has to
// ask what is on the screen rather than what the list holds.
//
// Rows above answers from the options, which is the right half for "what is in
// this list" and the wrong half for "what does a row draw". A picture that
// never reaches a row would pass the first and fail this one.
func (l *OpenList) DrawnRows() []*ListRow {
	out := make([]*ListRow, 0, len(l.rows))
	for _, row := range l.rows {
		out = append(out, row)
	}
	return out
}

// RowShowing is the row currently drawing one value, or nil if that value is
// scrolled out of sight. For a guard that needs to press or hover a real row.
func (l *OpenList) RowShowing(label string) *ListRow {
	for _, row := range l.rows {
		if row.Label() == label {
			return row
		}
	}
	return nil
}

// Choice is one row of an open list, for a guard to read.
type Choice struct {
	Label  string
	Marked bool
}

// MinSize is as wide as the widest value and as tall as visibleRows of them.
//
// The ceiling is the point. Without one the list is as tall as it likes, which
// at thirteen formats already covered the form and at twenty-five would not fit
// in the window.
func (l *OpenList) MinSize() fyne.Size {
	rows := len(l.options)
	if rows > visibleRows {
		rows = visibleRows
	}
	if rows < 1 {
		rows = 1
	}
	height := float32(rows) * listRowHeight()
	// The room the window has left beats the row ceiling, where there is less
	// of it. The ceiling is about not covering the form and says nothing about
	// a window with fewer than eight rows to spare - and this has to happen in
	// MinSize rather than by resizing the popup afterwards, because a popup is
	// never laid out smaller than its content's minimum. Measured on
	// 2026-08-19: asking for 195 px around a list whose minimum was 224 gave
	// 224 (O113).
	if l.room > 0 && height > l.room {
		height = l.room
	}
	if height < listRowHeight() {
		height = listRowHeight()
	}
	return fyne.NewSize(l.list.MinSize().Width, height)
}

// LimitTo tells the list how much room it has, so it can be shorter than its
// row ceiling when the window is shorter than that. Nought means no limit.
func (l *OpenList) LimitTo(room float32) { l.room = room }

func (l *OpenList) CreateRenderer() fyne.WidgetRenderer {
	// The surface is drawn here rather than left to the popup, so that the
	// colour a guard measures for "an open list is told from the form behind
	// it" is the colour actually on the screen.
	back := canvas.NewRectangle(Theme().Color(theme.ColorNameMenuBackground, theme.VariantDark))
	back.CornerRadius = Theme().Size(theme.SizeNameInputRadius)
	return widget.NewSimpleRenderer(container.NewStack(back, container.NewThemeOverride(l.list, rowTheme{})))
}

// rowTheme is our theme with the room between rows taken out.
//
// It exists because a sentence written in theme.go on 2026-08-12 was wrong,
// and the way it was wrong is the expensive kind. That note said the theme is
// asked for a size by NAME and not by widget, so "it is the only knob there
// is" - and the first half is true while the conclusion is not. A theme can be
// replaced for a SUBTREE with container.NewThemeOverride, which nobody had
// looked for, so a list was tightened by moving the padding of the entire form.
//
// Measured: widget.List spaces its rows by theme.SizeNamePadding, called
// separatorThickness in list.go - the same 6 px the form is built from. With
// the override the rows sit against each other and the row height is the whole
// of the pitch.
type rowTheme struct{}

func (rowTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return Theme().Color(n, v)
}
func (rowTheme) Font(s fyne.TextStyle) fyne.Resource     { return Theme().Font(s) }
func (rowTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return Theme().Icon(n) }
func (rowTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		// Rows sit against each other. What separates them is the surface a
		// row draws when the pointer or the keyboard is on it, which is a
		// thing somebody can see, rather than a gap, which is not.
		return 0
	case theme.SizeNameSeparatorThickness:
		// And no rule between them either. widget.List draws a hairline
		// between rows, which the room between them used to hide - with the
		// room gone it came out as a line every 28 px and the list read as a
		// ruled table rather than as a menu. Seen on the render, which is the
		// only place it could have been seen: the tree says a separator is
		// present either way.
		return 0
	}
	return Theme().Size(n)
}

// ListRowHeight is one row, for a guard asking how many rows fit in a height.
func ListRowHeight() float32 { return listRowHeight() }

// listRowHeight is one row, worked out rather than typed: whatever is taller
// out of the text and the mark, plus our own room above and below.
func listRowHeight() float32 {
	text := widget.NewLabel("Ag").MinSize().Height - 2*Theme().Size(theme.SizeNameInnerPadding)
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

// TypedKey moves, takes and closes.
func (l *OpenList) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyDown:
		l.moveTo(l.active + 1)
	case fyne.KeyUp:
		l.moveTo(l.active - 1)
	case fyne.KeyHome:
		l.moveTo(0)
	case fyne.KeyEnd:
		l.moveTo(len(l.options) - 1)
	case fyne.KeyEscape:
		l.close(true)
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		if l.active >= 0 && l.active < len(l.options) {
			l.take(l.options[l.active], true)
		}
	}
}

// TypedRune jumps to the next value starting with the letter typed.
//
// From the row after the current one and wrapping, so pressing the same letter
// again walks through the values that share it - the behaviour of every desktop
// menu. One letter rather than a typed prefix: a prefix needs a timer to know
// when the word ended, and a timer in a control is a thing that behaves
// differently on a slow machine.
func (l *OpenList) TypedRune(r rune) {
	want := strings.ToLower(string(r))
	for step := 1; step <= len(l.options); step++ {
		at := (l.active + step) % len(l.options)
		if at < 0 {
			at += len(l.options)
		}
		if strings.HasPrefix(strings.ToLower(l.options[at]), want) {
			l.moveTo(at)
			return
		}
	}
}

// moveTo puts the keyboard on one row and scrolls it into view. Clamped rather
// than wrapped, because a list that jumps from the last value to the first
// under a held arrow key is a list somebody overshoots in both directions.
func (l *OpenList) moveTo(at int) {
	if len(l.options) == 0 {
		return
	}
	if at < 0 {
		at = 0
	}
	if at >= len(l.options) {
		at = len(l.options) - 1
	}
	l.active = at
	l.shown = true
	l.list.ScrollTo(at)
	l.list.Refresh()
}

// Active is the row the keyboard is on, or -1, for a guard.
func (l *OpenList) Active() int { return l.active }

// StartOn puts the arrow keys on the value already in the box, without drawing
// it, so that opening a list of thirteen and pressing Down once does not go to
// the first value while the box shows the ninth.
func (l *OpenList) StartOn(value string) {
	for i, option := range l.options {
		if option == value {
			l.moveTo(i)
			l.shown = false
			l.list.Refresh()
			return
		}
	}
}

// Showing says whether the keyboard position is drawn, for a guard.
func (l *OpenList) Showing() bool { return l.shown }
