package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The pieces a report panel is made of.
//
// Why they exist at all. The column beside the form held one sentence and
// whatever height was left over, so at 1120x760 it drew a 414 px panel around
// 28 px of words - measured 2026-09-08, and the owner's report was that the
// window looked emptier than before it was rebuilt. ReportColumn stopped taking
// the leftover in the same change, and this is the other half: a report that
// has something to say does not need the room hidden.
//
// What is NOT here is arithmetic. G1 says the window asks the engine rather
// than working things out, and every number these draw arrives already worked
// out - a count off engine.Target, a size off core.ParseSize, a total off
// engine.TotalBytes. The parts spell numbers and never combine them.

// Fact is one line of a report: a quiet name, and what it came to.
//
// Monospace on the value half for the reason ByteCount is monospace. Fyne
// exposes no OpenType features in v2.8.1, so tabular figures cannot be turned
// on in the face the rest of the window uses, and a column of numbers that does
// not line up is a column that has to be read one row at a time. The face that
// has them by construction is the one to use.
type Fact struct {
	object *fyne.Container
	value  *widget.Label
}

// NewFact makes a line, with nothing on it. It stays off the screen until it
// has something to say - see Say.
func NewFact(name string) *Fact {
	value := widget.NewLabel("")
	value.TextStyle = fyne.TextStyle{Monospace: true}
	value.Alignment = fyne.TextAlignTrailing

	f := &Fact{value: value}
	f.object = container.NewBorder(nil, nil, quietLabel(name), value)
	f.object.Hide()
	return f
}

// Object is the line, to put on a screen.
func (f *Fact) Object() fyne.CanvasObject { return f.object }

// Say puts a value on the line, and an empty one takes the line off the screen.
//
// Hidden rather than blank, because a name with nothing beside it is a question
// the panel asked and did not answer - and the column layout skips what is
// hidden, so a line that says nothing costs no height either.
func (f *Fact) Say(value string) {
	if value == "" {
		f.object.Hide()
		return
	}
	f.value.SetText(value)
	f.object.Show()
}

// SizeFact is a fact whose value is a number of bytes, said twice.
//
// Both halves, always. The readable one is what somebody is looking for and the
// exact one is the whole point of this tool - a generator that says "about
// 10 MB" has not done its job - so neither is allowed to stand alone. The
// readable one leads because it answers the glance, and the exact one sits
// under it in the rank of a note.
type SizeFact struct {
	object *fyne.Container
	human  *widget.Label
	exact  *widget.Label
}

// NewSizeFact makes the block, with nothing in it.
func NewSizeFact(name string) *SizeFact {
	human := widget.NewLabel("")
	human.TextStyle = fyne.TextStyle{Monospace: true}

	exact := widget.NewLabel("")
	exact.TextStyle = fyne.TextStyle{Monospace: true}
	exact.Importance = widget.LowImportance
	exact.SizeName = theme.SizeNameCaptionText

	s := &SizeFact{human: human, exact: exact}
	// No gap between the three. They are one fact drawn on three lines, and the
	// vertical scale says a gap of nought is what "inside one thing" looks like
	// when the thing is a stack of text.
	s.object = Column(0, quietLabel(name), human, exact)
	s.object.Hide()
	return s
}

// Object is the block, to put on a screen.
func (s *SizeFact) Object() fyne.CanvasObject { return s.object }

// Say puts a number of bytes on the screen, readable and exact.
//
// It is handed bytes rather than a string, so that the spelling of a byte count
// happens in one place for the whole program. core.HumanBytes and
// text.ExactBytes are that place, and they are the same two the command line
// prints with.
func (s *SizeFact) Say(bytes int64) {
	s.human.SetText(core.HumanBytes(bytes))
	s.exact.SetText(text.ExactBytes(bytes))
	s.object.Show()
}

// Nothing takes the block off the screen, for a value that is not known yet.
func (s *SizeFact) Nothing() { s.object.Hide() }

// quietLabel is the name half of a fact, at the rank of a field's own name.
//
// The same rank on purpose. A report panel and a form panel stand side by side,
// so a name in one that outweighed a name in the other would say the two are
// different kinds of thing when they are not - both are the quiet half of a
// pair whose other half is the content.
func quietLabel(name string) *widget.Label {
	label := widget.NewLabel(name)
	label.Importance = widget.LowImportance
	label.SizeName = theme.SizeNameCaptionText
	return label
}

// Rule is the hairline between two groups of facts.
//
// theme.ColorNameSeparator rather than a colour of its own, and that is worth a
// line because the palette here is measured and adding to it is not free - see
// ColorNamePanelRaised for what one new value costs. This one was already
// declared and already the right weight.
func Rule() fyne.CanvasObject {
	line := canvas.NewRectangle(PaletteColour(theme.ColorNameSeparator, theme.VariantDark))
	line.SetMinSize(fyne.NewSize(0, 1))
	return line
}

// Path is a directory drawn where somebody has to be able to read all of it.
//
// Wrapped rather than cut off. What it holds is where this program is about to
// write on somebody's disk, and the end of a path is the part that says which
// directory - so a path shortened at the right is a path that answers the wrong
// half of the question. Monospace because it is a literal value, not prose.
func Path(dir string) *widget.Label {
	label := widget.NewLabel(dir)
	label.TextStyle = fyne.TextStyle{Monospace: true}
	label.SizeName = theme.SizeNameCaptionText
	label.Wrapping = fyne.TextWrapBreak
	return label
}
