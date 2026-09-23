package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// QuietText is words that recede: a screen's subtitle, a caption, the line a
// folded section keeps, the count of bytes beside a size. Drawn in the hint's
// colour, with no room of its own around the ink.
//
// The hint's colour and not the disabled one is O213, the owner's decision of
// 2026-09-15: the toolkit draws a low importance label in ColorNameDisabled,
// #C2C8CD at 80 L*, a step BRIGHTER than a hint - so a caption meant to recede
// sat louder than the placeholder in an empty box beside it. The disabled
// colour stays for a value that is switched off.
//
// Its own widget rather than a toolkit label under a theme override, since
// 2026-09-23. A label's colour follows its importance and nothing else, so the
// only way to give it the hint's colour was an override - and Fyne gives an
// override a new scope at construction, at CreateRenderer and at every
// Refresh, and parses a fresh ~7.4 MB set of fonts for every scope a new
// string is drawn in. Measured in the real window: ten rebuilds of the batch
// screen, each followed by a new size, parsed ten sets for the count of bytes
// alone, and nothing frees them (docs/GUI-MEMORY-2026-09-23.md section 4c).
// A rich text names its colour per segment, so none of that is needed.
//
// The segment is the one widget.Label builds for itself (widget/label.go,
// syncSegments), with the colour changed and nothing else, and the room is
// taken off by the layout inkTight uses - so what reaches the screen is what
// the label under the override drew, pixel for pixel on every stored screen.
//
// Like a label it does not wrap unless asked to.
type QuietText struct {
	widget.BaseWidget

	// Text is what it says, SizeName the rank of the scale it says it at
	// (the body text when empty), and Wrapping whether a long line breaks.
	Text     string
	SizeName fyne.ThemeSizeName
	Wrapping fyne.TextWrap

	rich *widget.RichText
}

// NewQuietText makes quiet words.
func NewQuietText(text string) *QuietText {
	q := &QuietText{}
	q.start(text)
	q.ExtendBaseWidget(q)
	return q
}

// start is what every QuietText needs before it is drawn, a ByteCount
// included - which is a QuietText inside and extends itself rather than this.
func (q *QuietText) start(text string) {
	q.Text = text
	q.rich = widget.NewRichText(&widget.TextSegment{})
}

// SetText changes what it says.
func (q *QuietText) SetText(text string) {
	q.Text = text
	q.Refresh()
}

// Refresh carries the fields into the segment before anything is redrawn.
// The rich text is refreshed once, by the renderer through its container -
// widget.Label refreshes its own twice, which a count redrawn on every key
// has no need of.
func (q *QuietText) Refresh() {
	q.sync()
	q.BaseWidget.Refresh()
}

// CreateRenderer draws the rich text ink tight.
func (q *QuietText) CreateRenderer() fyne.WidgetRenderer {
	q.sync()
	return widget.NewSimpleRenderer(inkTight(q.rich))
}

// sync is widget.Label's syncSegments with the hint's colour.
func (q *QuietText) sync() {
	size := q.SizeName
	if size == "" {
		size = theme.SizeNameText
	}
	q.rich.Wrapping = q.Wrapping
	segment := q.rich.Segments[0].(*widget.TextSegment)
	segment.Text = q.Text
	segment.Style = widget.RichTextStyle{
		ColorName: theme.ColorNamePlaceHolder,
		Inline:    true,
		SizeName:  size,
	}
}
