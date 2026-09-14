package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// RequiredMark is the star beside the name of a field that has to be filled in.
//
// It answers a question none of the three screens could answer before, and the
// gap was counted rather than felt: on the batch screen six boxes stood empty
// with nothing in them at all, and among those six were settings a run refuses
// - the group name, which anchors a target's seed - standing beside settings
// nobody need ever touch, drawn identically. Nothing on the screen told them
// apart until somebody pressed Generate.
//
// A SHAPE as well as a colour, which is UX1 and not decoration: a reader who
// cannot tell the red from the grey around it still sees a star that the fields
// beside it do not have. The colour is the palette's error red, the same one a
// refusal about the field will use, so the mark and the message that follows it
// are the same colour rather than two reds. A grey star was tried on 2026-09-14
// and went back the same day: beside a regular-weight name and the grey button
// that opens the explanation it was one more grey glyph, and a mark that does
// not stand out from what it marks is not a mark.
//
// A type of its own for the reason DetailButton is one: the heading of a field
// is a row, guards and probes read that row to find the control under it, and
// recognising a thing by its position in a list is what breaks the third time
// somebody adds a fourth thing to the row. Drawn with no room of its own
// around the glyph, like every other word on the form - see words.
type RequiredMark struct {
	widget.BaseWidget
	glyph *canvas.Text
}

func newRequiredMark() *RequiredMark {
	m := &RequiredMark{glyph: words(text.RequiredMark, TextBody, true, theme.ColorNameError)}
	m.ExtendBaseWidget(m)
	return m
}

// CreateRenderer draws the glyph and nothing else.
func (m *RequiredMark) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.glyph)
}

// MinSize is the glyph's, so the mark takes the room of a star and not of a
// label round one.
func (m *RequiredMark) MinSize() fyne.Size { return m.glyph.MinSize() }

// headingRow is a field's name, the mark saying it must be filled in, and the
// button holding its longer explanation - in that order, on one line.
//
// Flat rather than nested, and that is load bearing. Every walk in this project
// finds a field by looking for a name with its control after it, so the name
// has to stay the FIRST thing in this row - wrapping the name and the star in a
// box of their own would hide the name one level down and every one of those
// walks would stop finding fields. Measured cost of getting that wrong: the
// probe reported "there is no field labelled width" the first time this was
// tried the other way round.
func headingRow(label string, detail Detail, required bool) fyne.CanvasObject {
	head := Heading(label)
	row := []fyne.CanvasObject{head}
	if required {
		row = append(row, newRequiredMark())
	}
	if detail.Text != "" && detail.on != nil {
		row = append(row, newDetailButton(detail))
	}
	if len(row) == 1 {
		return head
	}
	return container.New(headingLine{}, row...)
}

// headingLine lays a field's name, its star and its explanation button on one
// line, with the star closer to the name than to the button.
//
// A layout of ours rather than an HBox, and it is the horizontal twin of Column
// with the same reason: a box adds its own padding between children, so
// anything put between them can only make a gap BIGGER, and the gap this needs
// is smaller than that padding.
//
// What it is for is proximity. Measured off the render the first time this was
// built with an HBox: the name ended at x=128, the star ran 148 to 154 and the
// button began at 174 - twenty pixels to the name and twenty to the button, so
// the star sat exactly between the two things it could belong to and therefore
// belonged to neither. A mark that qualifies a name has to be nearer the name
// than the next control, which is the same rule the form already follows
// between a label, its box and the line explaining it.
//
// Everything on the line is centred on its height, so a star and a button of
// two different heights both sit level with the middle of the name.
type headingLine struct{}

// gapBefore is the room left in front of one thing on the heading line.
//
// It answers about WHAT is next rather than about where it is in the list, and
// that distinction is the whole reason this layout exists rather than an
// HBox with a smaller gap. Written by position first, it moved the explanation
// button of every field on every screen - including the ones with no star -
// because the button is second in the row when nothing else is. The stored
// trees said so on the first regeneration: a label 32 px tall became 31 and a
// button at x=69 moved to 63 on fields this change was not supposed to touch.
//
// So: the smallest step in front of the star, which qualifies the name, and
// the next one in front of everything else. Both on the scale, and the star
// nearer the name than the button is to the star.
func gapBefore(o fyne.CanvasObject) float32 {
	if _, star := o.(*RequiredMark); star {
		return GapInline
	}
	return GapLabel
}

func (h headingLine) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	first := true
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if !first {
			size.Width += gapBefore(o)
		}
		first = false
		size.Width += min.Width
		size.Height = fyne.Max(size.Height, min.Height)
	}
	return size
}

func (h headingLine) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	first := true
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if !first {
			x += gapBefore(o)
		}
		first = false
		o.Resize(min)
		o.Move(fyne.NewPos(x, (size.Height-min.Height)/2))
		x += min.Width
	}
}
