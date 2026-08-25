package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Folding is a section whose body can be put away, leaving its name, one line
// saying what is in it, and whatever has to stay pressable while it is away.
//
// It exists because of a measurement rather than a preference. The batch screen
// draws every batch in full, one under another, and on 2026-08-25 somebody
// finally counted what that costs: one batch is a form 913 px tall in 849 px of
// room, and each further batch adds 659 px. Ten batches is 6884 px, which is
// eight screens of scrolling and 1534 objects in the tree - and ten batches is
// an ordinary thing to want from a tool whose whole point is producing sets of
// files.
//
// The alternative shape - a list with one batch open at a time - was rejected
// on 2026-08-18 and the reason still stands: a refusal names the batch it is
// about, and a batch with no fields on the screen has no box to mark, so
// refusals would fall back to the foot of the form. That is the defect the
// whole addressing effort removed.
//
// This is not that shape. Every batch is here and every box is registered, so a
// refusal still finds its box - and a screen that is refused OPENS the fold the
// box is in, which is what keeps the objection answered rather than dodged. See
// the batch screen's use of it.
type Folding struct {
	open   bool
	body   fyne.CanvasObject
	line   *widget.Label
	toggle *widget.Button

	// object is the whole thing, rebuilt when the fold moves so the layout
	// above it is told to take the room back.
	object *fyne.Container

	// OnChange is told whenever this opens or folds shut.
	//
	// It exists because a screen that rebuilds its panels has to remember the
	// fold somewhere that survives the rebuild. This one is built again every
	// time a batch is added or removed, and a fold that lived only in here
	// would spring open each time.
	OnChange func(open bool)
}

// NewFolding builds one, open, with the title it keeps and the controls that
// stay reachable while it is folded away.
//
// The controls in the head are there for a reason worth stating: a folded batch
// that could not be removed or copied would be a batch somebody has to open to
// do the two things they are most likely to want from a list of them.
func NewFolding(title string, head []fyne.CanvasObject, content ...fyne.CanvasObject) *Folding {
	f := &Folding{open: true}

	f.line = widget.NewLabel("")
	f.line.Importance = widget.LowImportance
	f.line.Hide()

	f.toggle = widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() { f.Set(!f.open) })
	f.toggle.Importance = widget.LowImportance

	f.body = Column(GapField, content...)

	row := []fyne.CanvasObject{f.toggle, sectionTitle(title), f.line, layout.NewSpacer()}
	row = append(row, head...)

	f.object = container.NewStack(panelSurface(), container.NewPadded(
		Column(GapField, container.NewHBox(row...), f.body)))
	return f
}

// Object is the panel to put on a screen.
func (f *Folding) Object() fyne.CanvasObject { return f.object }

// IsOpen says whether the body is on the screen.
func (f *Folding) IsOpen() bool { return f.open }

// Set opens or folds it.
//
// Opening is idempotent on purpose: a refusal opens every fold holding a box it
// is about, and most of them are open already.
func (f *Folding) Set(open bool) {
	if f.open == open {
		return
	}
	f.open = open
	if open {
		f.body.Show()
		f.line.Hide()
		f.toggle.SetIcon(theme.MenuDropDownIcon())
	} else {
		f.body.Hide()
		if f.line.Text != "" {
			f.line.Show()
		}
		f.toggle.SetIcon(theme.MenuExpandIcon())
	}
	f.object.Refresh()
	if f.OnChange != nil {
		f.OnChange(open)
	}
}

// Say is the one line shown while this is folded away.
//
// A fold with nothing to say is a row of titles somebody has to open one by one
// to find the one they meant, which is a worse screen than a long one. Empty
// hides the line rather than leaving a gap where a sentence would be.
func (f *Folding) Say(summary string) {
	f.line.SetText(summary)
	if summary == "" || f.open {
		f.line.Hide()
		return
	}
	f.line.Show()
}
