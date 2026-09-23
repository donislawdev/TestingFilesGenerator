package parts

// listContents is what an open list holds and what it has drawn: the values,
// how they are grouped and narrowed, the rows that arrangement comes to, and
// the rows on the screen showing it.
//
// Its own type rather than more of OpenList, since the list of 2026-09-23
// took headings and a filter. OpenList stood at 24 methods, the fourth type in
// the tree past the crowding line, and TestNoSecondTypeIsCreepingUpOnTheTypeCeilings
// asks for behaviour to move out rather than for the line to move. What moved
// is one question - what is in this list, and what does each row draw - which
// the widget, the keyboard and the layout only ask. OpenList embeds it, so a
// guard still reads list.Rows() and list.DrawnRows() where it always did.
type listContents struct {
	options []string
	// chosen is the value in the box, marked with a tick. Empty when the box
	// shows a default nobody has confirmed - see the note in preset.go about a
	// filled field making "I did not say" impossible to express.
	chosen string

	// headingOf is the heading a value stands under, or nil for a list with no
	// headings. See OpenList.GroupUnder.
	headingOf func(string) string
	// typed is what is in the filter box, or nothing for a list without one.
	typed string
	// entries is what the list draws now - headings, values and the notice
	// that nothing matched - worked out by arrange. Every row number in this
	// package is a position in entries, never in options, because the two part
	// the moment a list has a heading.
	entries []listEntry

	// view is the rows on the screen. Asked rather than walked, because a
	// walk cannot get in: the rows are inside the list's renderer, so a tree
	// walk stops at the list and reports an open list with nothing in it.
	// Measured on 2026-08-18 while trying to photograph a row under the
	// pointer, when the rows were widget.List's.
	view *rowView
}

// rearrange works out the rows again after the filter or the grouping
// changed, and forgets the rows on the screen until they are filled again: a
// row filled for the old arrangement can still hold a label the list no
// longer draws, and RowShowing would report it.
func (c *listContents) rearrange() {
	c.entries = arrange(c.options, c.headingOf, c.typed)
	c.view.shown = 0
}

// Rows is what this list is showing, for a guard to read, in the order it is
// drawn - headings and the notice included, so that a position from Active
// is a position here.
//
// The toolkit's menu turned items into widgets of an unexported type, so what
// was marked could not be read back off the canvas - only that something was
// open. This is the half of that pair we own, and it says what is in the list
// and which row carries the tick.
func (c *listContents) Rows() []Choice {
	out := make([]Choice, 0, len(c.entries))
	for _, e := range c.entries {
		value := e.kind == entryValue
		out = append(out, Choice{Label: e.text, Marked: value && c.isChosen(e.text), Choosable: value})
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
func (c *listContents) isChosen(value string) bool { return value == c.chosen }

// DrawnRows is every row in sight, for a guard that has to ask what is on the
// screen rather than what the list holds.
//
// Rows above answers from the options, which is the right half for "what is in
// this list" and the wrong half for "what does a row draw". A picture that
// never reaches a row would pass the first and fail this one.
func (c *listContents) DrawnRows() []*ListRow {
	var out []*ListRow
	for at, row := range c.view.built[:c.view.shown] {
		if c.view.inSight(at) {
			out = append(out, row)
		}
	}
	return out
}

// RowShowing is the row currently drawing one value, or nil if that value is
// scrolled out of sight. For a guard that needs to press or hover a real row.
func (c *listContents) RowShowing(label string) *ListRow {
	for _, row := range c.DrawnRows() {
		if row.Label() == label {
			return row
		}
	}
	return nil
}
