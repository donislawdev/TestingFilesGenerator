package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Tab is one screen of the window and the word on the strip that leads to it.
type Tab struct {
	Text    string
	Content fyne.CanvasObject
}

// Tabs is the strip of words across the top of the window: one word a screen,
// the chosen one underlined.
//
// Our own rather than the toolkit's container.AppTabs since 2026-09-15, and
// the reasons are three measurements rather than a preference. The toolkit's
// strip stands on the edge of the window while everything a person reads
// stands on the column - 10 px against 157 at the width the window opens at -
// and its renderer reserves the bar's height unconditionally (tabs.go,
// layout), so a strip drawn anywhere else would have paid for the toolkit's
// as well. Its buttons answer the pointer and not the keyboard (tabs.go:
// tabButton is Tappable and Hoverable and nothing else), so four words no key
// could reach stood across the top of a window whose rule is that whatever the
// mouse can do the keyboard can (UX9). And the one thing a strip has to say -
// which word is chosen - it said in a colour handed in through a theme
// override, because the words were drawn inside a renderer nothing here could
// reach.
//
// The strip is a widget because it has behaviour: it is the one thing that
// knows which screen is on show. What it stands ABOVE is not its business -
// see Tabbed, which puts it and the screens in a plain container, so that every
// walk that already knows a container reaches every screen, shown or not.
type Tabs struct {
	widget.BaseWidget

	items   []*Tab
	words   []*TabWord
	current int

	// OnSelected is told which tab was chosen and whether the keyboard chose
	// it. The second answer decides where the keyboard goes next: after a
	// press it is placed quietly, after a key it is handed on where it can be
	// seen - see PointerFocus for why those are two different things.
	OnSelected func(tab *Tab, byKeyboard bool)
}

// NewTabs builds a strip leading to these screens, open on the first.
func NewTabs(tabs ...*Tab) *Tabs {
	t := &Tabs{items: tabs}
	t.ExtendBaseWidget(t)
	for i, tab := range tabs {
		t.words = append(t.words, newTabWord(t, i, tab.Text))
		// Shown and hidden by the strip rather than by whatever lays the
		// screens out, so that there is one answer to which screen is on show
		// and no second copy of it to fall out of step.
		if i == 0 {
			tab.Content.Show()
		} else {
			tab.Content.Hide()
		}
	}
	return t
}

// Items is every screen the strip leads to, in the order of the words.
func (t *Tabs) Items() []*Tab { return t.items }

// Selected is the screen on show, or nil for a strip with no words.
func (t *Tabs) Selected() *Tab {
	if t.current < 0 || t.current >= len(t.items) {
		return nil
	}
	return t.items[t.current]
}

// Select moves to a screen the way a press does. A tab this strip does not
// hold, or the one already on show, changes nothing and says nothing.
func (t *Tabs) Select(tab *Tab) {
	for i, item := range t.items {
		if item == tab {
			t.choose(i, false)
			return
		}
	}
}

// Words are the things on the strip a person presses, hovers or puts the
// keyboard on, in the order of the screens. For a guard that wants to do one
// of those.
func (t *Tabs) Words() []*TabWord { return t.words }

func (t *Tabs) choose(i int, byKeyboard bool) {
	if i == t.current || i < 0 || i >= len(t.items) {
		return
	}
	t.items[t.current].Content.Hide()
	t.current = i
	t.items[i].Content.Show()
	t.Refresh()
	if t.OnSelected != nil {
		t.OnSelected(t.items[i], byKeyboard)
	}
}

// focusWord puts the keyboard on the word at i, for the arrow keys. Past either
// end an arrow does nothing rather than wrapping round - a strip of four words
// is short enough to see the whole of, so there is nothing to wrap to.
func (t *Tabs) focusWord(i int) {
	if i < 0 || i >= len(t.words) {
		return
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
		c.Focus(t.words[i])
	}
}

// CreateRenderer draws the words in a row and the mark under the chosen one.
func (t *Tabs) CreateRenderer() fyne.WidgetRenderer {
	indicator := canvas.NewRectangle(PaletteColour(theme.ColorNamePrimary, theme.VariantDark))
	r := &tabsRenderer{strip: t, indicator: indicator}
	r.Refresh()
	return r
}

type tabsRenderer struct {
	strip     *Tabs
	indicator *canvas.Rectangle
}

// Layout puts the words in a row and the mark below the row, so that the mark
// never covers the fill or the ring a word draws for itself.
func (r *tabsRenderer) Layout(size fyne.Size) {
	x := float32(0)
	row := fyne.Max(0, size.Height-TabIndicator)
	for _, word := range r.strip.words {
		min := word.MinSize()
		word.Move(fyne.NewPos(x, 0))
		word.Resize(fyne.NewSize(min.Width, row))
		x += min.Width + GapTabs
	}
	r.placeIndicator(size)
}

func (r *tabsRenderer) placeIndicator(size fyne.Size) {
	if r.strip.Selected() == nil {
		r.indicator.Hide()
		return
	}
	word := r.strip.words[r.strip.current]
	r.indicator.Show()
	r.indicator.Move(fyne.NewPos(word.Position().X, size.Height-TabIndicator))
	r.indicator.Resize(fyne.NewSize(word.Size().Width, TabIndicator))
}

func (r *tabsRenderer) MinSize() fyne.Size {
	size := fyne.NewSize(0, TabIndicator)
	for i, word := range r.strip.words {
		min := word.MinSize()
		if i > 0 {
			size.Width += GapTabs
		}
		size.Width += min.Width
		size.Height = fyne.Max(size.Height, min.Height+TabIndicator)
	}
	return size
}

func (r *tabsRenderer) Refresh() {
	for i, word := range r.strip.words {
		word.chosen = i == r.strip.current
		word.Refresh()
	}
	r.placeIndicator(r.strip.Size())
	redraw(r.indicator)
}

func (r *tabsRenderer) Objects() []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, 0, len(r.strip.words)+1)
	for _, word := range r.strip.words {
		objects = append(objects, word)
	}
	return append(objects, r.indicator)
}

func (r *tabsRenderer) Destroy() {}

// TabWord is one word on the strip.
//
// Its own widget for the reasons a ListRow is: it answers the pointer itself,
// it draws its own surface for the states it has, and it can hold the keyboard
// - which the toolkit's tab button cannot. Four states, drawn in this order of
// precedence: chosen, holding the keyboard, under the pointer, at rest. A tab
// has no disabled state and no refused one in this window, so neither is drawn
// - a state nothing can reach is a state that lies the day something does.
type TabWord struct {
	widget.BaseWidget

	strip *Tabs
	index int
	text  string

	chosen  bool
	hovered bool
	marked  bool
	from    PointerFocus
}

func newTabWord(strip *Tabs, index int, text string) *TabWord {
	w := &TabWord{strip: strip, index: index, text: text}
	w.ExtendBaseWidget(w)
	return w
}

// Text is the word, for a guard.
func (w *TabWord) Text() string { return w.text }

// Chosen says whether this word's screen is the one on show.
func (w *TabWord) Chosen() bool { return w.chosen }

// Hovered says whether the pointer is over this word.
func (w *TabWord) Hovered() bool { return w.hovered }

// Marked says whether the keyboard mark is drawn - see PointerFocus.
func (w *TabWord) Marked() bool { return w.marked }

// Tapped chooses this word's screen. The keyboard comes to the word quietly
// first, so that the mark meaning "the keyboard is here" is not drawn for
// somebody using the mouse - the same decision every control in this package
// takes, see PointerFocus.
func (w *TabWord) Tapped(*fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(w); c != nil {
		w.from.Quietly(func() { c.Focus(w) })
	}
	w.strip.choose(w.index, false)
}

func (w *TabWord) MouseIn(*desktop.MouseEvent) {
	w.hovered = true
	w.Refresh()
}

func (w *TabWord) MouseMoved(*desktop.MouseEvent) {}

func (w *TabWord) MouseOut() {
	w.hovered = false
	w.Refresh()
}

// FocusGained draws the mark only for the keyboard. See PointerFocus.
func (w *TabWord) FocusGained() {
	if w.from.Quiet() {
		return
	}
	w.mark()
}

func (w *TabWord) mark() {
	w.marked = true
	w.Refresh()
}

func (w *TabWord) FocusLost() {
	w.marked = false
	w.Refresh()
}

// Quietly runs a focus change without drawing the mark. See FocusQuietly.
func (w *TabWord) Quietly(focus func()) { w.from.Quietly(focus) }

func (w *TabWord) TypedRune(rune) {}

// TypedKey is the keyboard's half of the strip. Enter and Space choose the
// word's screen. The arrows and Home and End move the keyboard along the strip
// WITHOUT choosing - choosing hands the keyboard on to the screen's first field,
// so an arrow that chose would leave the strip on every press, and somebody
// looking for the third word would be on the second screen's form instead.
// That is the manual activation pattern the web uses for a row of tabs, and it
// is the one that fits a strip whose choice moves the keyboard.
func (w *TabWord) TypedKey(event *fyne.KeyEvent) {
	if !w.marked {
		w.mark()
	}
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		w.strip.choose(w.index, true)
	case fyne.KeyLeft:
		w.strip.focusWord(w.index - 1)
	case fyne.KeyRight:
		w.strip.focusWord(w.index + 1)
	case fyne.KeyHome:
		w.strip.focusWord(0)
	case fyne.KeyEnd:
		w.strip.focusWord(len(w.strip.words) - 1)
	}
}

func (w *TabWord) CreateRenderer() fyne.WidgetRenderer {
	back := canvas.NewRectangle(color.Transparent)
	back.CornerRadius = RadiusField
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = RadiusField
	ring.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	r := &tabWordRenderer{word: w, back: back, ring: ring, text: words(w.text, TextBody, true, theme.ColorNamePlaceHolder)}
	r.Refresh()
	return r
}

type tabWordRenderer struct {
	word *TabWord
	back *canvas.Rectangle
	ring *canvas.Rectangle
	text *canvas.Text
}

func (r *tabWordRenderer) Layout(size fyne.Size) {
	r.back.Resize(size)
	r.ring.Resize(size)
	ink := r.text.MinSize()
	r.text.Move(fyne.NewPos(TabInset, (size.Height-ink.Height)/2))
	r.text.Resize(ink)
}

func (r *tabWordRenderer) MinSize() fyne.Size {
	return r.text.MinSize().Add(fyne.NewSquareSize(TabInset * 2))
}

// Refresh draws the state. The word is at full strength when it is chosen and
// when the pointer is over it, and quiet otherwise, so the strip has one word
// that answers "where am I" and one that answers "where would this take me".
// The keyboard mark is a ring and not a fill, for the reason Ring gives.
func (r *tabWordRenderer) Refresh() {
	r.text.Text = r.word.text
	if r.word.chosen || r.word.hovered {
		r.text.Color = PaletteColour(theme.ColorNameForeground, theme.VariantDark)
	} else {
		r.text.Color = PaletteColour(theme.ColorNamePlaceHolder, theme.VariantDark)
	}
	if r.word.hovered {
		r.back.FillColor = PaletteColour(theme.ColorNameHover, theme.VariantDark)
	} else {
		r.back.FillColor = color.Transparent
	}
	if r.word.marked {
		r.ring.StrokeWidth = ringWidth
	} else {
		r.ring.StrokeWidth = 0
	}
	redraw(r.back, r.ring, r.text)
}

func (r *tabWordRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.back, r.ring, r.text}
}

func (r *tabWordRenderer) Destroy() {}

// Tabbed is the strip with its screens under it: the strip on the column the
// screens keep their words on, a hairline across the whole window under it,
// and every screen in the room below, one on top of another, the chosen one
// showing.
//
// A plain container rather than a widget, for the reason Section gives: a walk
// over the tree that meets a widget it was never told about stops there and
// reports nothing below it, and that has made a guard pass while proving
// nothing more than once in this project. A container is known to every walk
// already, so every screen is reached whether or not it is on show - which is
// what the guards asked of the toolkit's tabs by walking their items.
//
// The strip goes through readableWidth and Indented, which is exactly what a
// screen's title goes through, so the first word of the strip and the title
// stand on one edge by construction rather than by two numbers agreeing.
func Tabbed(strip *Tabs) *fyne.Container {
	rule := canvas.NewRectangle(PaletteColour(theme.ColorNameSeparator, theme.VariantDark))
	objects := []fyne.CanvasObject{rule, container.New(readableWidth{}, Indented(strip))}
	for _, tab := range strip.items {
		objects = append(objects, tab.Content)
	}
	return container.New(tabbed{}, objects...)
}

// tabbed is the layout behind Tabbed. Objects[0] is the hairline, Objects[1]
// the strip in its column, and everything after that a screen.
//
// The rule is laid out under the strip's last pixel rather than below it, and
// the strip is painted after the rule, so the mark under the chosen word sits
// on the line and covers it - one line that the chosen word interrupts, rather
// than a line and a mark stacked apart.
//
// The screens are all given the whole room, hidden or not. A hidden screen
// costs a layout pass and no pixels, and a screen that is laid out before it is
// shown is one that appears at its size rather than a frame later.
type tabbed struct{}

func (tabbed) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.Size{}
	}
	strip := objects[1].MinSize()
	size := fyne.NewSize(strip.Width, aboveTheScreens(objects[1]))
	// Every screen rather than the one on show, so that moving between them
	// never asks the window to grow - the same answer the toolkit's tabs gave.
	screens := fyne.NewSize(0, 0)
	for _, screen := range objects[2:] {
		screens = screens.Max(screen.MinSize())
	}
	size.Width = fyne.Max(size.Width, screens.Width)
	size.Height += screens.Height
	return size
}

// aboveTheScreens is the height Tabbed spends above its screens: the bar's
// inset, the strip and the gap under it. One function for the two places the
// layout needs it and the one place the window does - AboveTheScreens - so the
// three cannot come to three answers.
func aboveTheScreens(strip fyne.CanvasObject) float32 {
	return InsetBar + strip.MinSize().Height + GapUnderTabs
}

// AboveTheScreens is how much of a window's height Tabbed keeps above the
// screen on show. The window asks it when working out how tall to open so
// that a screen shows whole - the rest of that height is the screen's own.
func AboveTheScreens(tabbed *fyne.Container) float32 {
	return aboveTheScreens(tabbed.Objects[1])
}

func (tabbed) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	rule, strip := objects[0], objects[1]
	height := strip.MinSize().Height
	strip.Move(fyne.NewPos(0, InsetBar))
	strip.Resize(fyne.NewSize(size.Width, height))
	foot := InsetBar + height
	rule.Move(fyne.NewPos(0, foot-Hairline))
	rule.Resize(fyne.NewSize(size.Width, Hairline))

	top := aboveTheScreens(strip)
	for _, screen := range objects[2:] {
		screen.Move(fyne.NewPos(0, top))
		screen.Resize(fyne.NewSize(size.Width, fyne.Max(0, size.Height-top)))
	}
}
