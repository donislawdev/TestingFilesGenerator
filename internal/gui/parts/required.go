package parts

import (
	"fyne.io/fyne/v2"
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
// are the same colour rather than two reds.
//
// A type of its own for the reason DetailButton is one: the heading of a field
// is a row, guards and probes read that row to find the control under it, and
// recognising a thing by its position in a list is what breaks the third time
// somebody adds a fourth thing to the row.
type RequiredMark struct {
	widget.Label
}

func newRequiredMark() *RequiredMark {
	m := &RequiredMark{}
	m.ExtendBaseWidget(m)
	m.Text = text.RequiredMark
	// Bold, because it stands beside a bold name and a light star next to heavy
	// words reads as a smudge rather than as a mark.
	m.TextStyle = fyne.TextStyle{Bold: true}
	// The palette's error colour, asked for by role rather than by value. The
	// same name ErrorArea uses, so a field's mark and a field's refusal cannot
	// come apart.
	m.Importance = widget.DangerImportance
	return m
}

// headingRow is a field's name, the mark saying it must be filled in, and the
// button holding its longer explanation - in that order, on one line.
//
// Flat rather than nested, and that is load bearing. Every walk in this project
// finds a field by looking for a label with its control after it, so the name
// has to stay the FIRST thing in this row - wrapping the name and the star in a
// box of their own would hide the label one level down and every one of those
// walks would stop finding fields. Measured cost of getting that wrong: the
// probe reported "there is no field labelled width" the first time this was
// tried the other way round.
func headingRow(label string, detail Detail, required bool, trailing fyne.CanvasObject) fyne.CanvasObject {
	head := Heading(label)
	row := []fyne.CanvasObject{head}
	if required {
		row = append(row, newRequiredMark())
	}
	if detail.Text != "" && detail.on != nil {
		row = append(row, newDetailButton(detail))
	}
	// Last, and laid out against the far edge rather than after the button -
	// see headingLine. It belongs to the box below rather than to the name, so
	// crowding it against the name would make it read as part of the name.
	if trailing != nil {
		row = append(row, trailing)
	}
	// A row of one is the label itself. Wrapping it would put every field's
	// name a level deeper for no reason and change the shape of every stored
	// screen that has no explanation and no star.
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
// So: nothing in front of the star, and the toolkit's own padding in front of
// everything else. A field with no star lays out exactly as it did before, and
// what a reader sees between the name and the star is the padding a label
// carries inside itself.
func gapBefore(o fyne.CanvasObject) float32 {
	if _, star := o.(*RequiredMark); star {
		return 0
	}
	return theme.Padding()
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
		width := o.MinSize().Width
		// The full height of the line rather than each thing's own, which is
		// what a horizontal box does - and matching it is what keeps a field
		// with no star laid out to the pixel as it was.
		o.Resize(fyne.NewSize(width, size.Height))

		// A count of bytes is pushed to the far edge instead of following the
		// name. It describes the box underneath rather than the words beside
		// it, and the whole point of putting it on this line was the empty half
		// of the column - laid out next to the name it would sit in the middle
		// of nothing, and read as part of the label.
		if _, trailing := o.(*ByteCount); trailing {
			o.Resize(fyne.NewSize(size.Width-x, size.Height))
			o.Move(fyne.NewPos(x, 0))
			continue
		}

		if !first {
			x += gapBefore(o)
		}
		first = false
		o.Move(fyne.NewPos(x, 0))
		x += width
	}
}
