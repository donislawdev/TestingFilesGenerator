package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// CreateRenderer draws the toolkit's menu with its arrow in the primary
// colour.
//
// Why, measured on the owner's screenshot of the running window on
// 2026-09-23: a menu and a box to type in were the same to the pixel -
// (57,57,63) inside, (81,81,87) at the edge - and a button was one step
// lighter at (72,72,78), so a menu could be told from neither. Three ways of
// drawing it were shown side by side in three real windows - the arrow cut
// off by a line, an outline with no fill, the arrow in the accent - and the
// owner chose the accent. It is the one mark on the form that says "this
// opens a list" and nothing else says it: a box has no arrow, a button has no
// arrow, and nothing else is drawn in the primary colour at rest but Generate.
//
// The toolkit's renderer is kept whole: its objects are a background
// rectangle, a tap animation, the value and the arrow (fyne v2.8.1
// widget/select.go, CreateRenderer), and only the arrow's colour changes. It
// is put back after every refresh, because the toolkit's refresh sets the
// resource it chose again. A disabled menu keeps the toolkit's disabled arrow,
// so a menu frozen for a run does not look live.
//
// A menu whose values are kinds of file also draws the picture of its value
// in front of the word, the same picture the value's row carries in the open
// list. Until 2026-09-23 the closed box said "avif" and nothing else, so the
// kind of the file being made was on the screen only while the list was open.
func (c *Chooser) CreateRenderer() fyne.WidgetRenderer {
	look := &menuLook{WidgetRenderer: c.Select.CreateRenderer(), menu: c}
	look.objects = look.WidgetRenderer.Objects()
	if c.KindOf != nil {
		look.kind = canvas.NewImageFromResource(nil)
		look.kind.FillMode = canvas.ImageFillContain
		look.objects = append(append([]fyne.CanvasObject{}, look.objects...), look.kind)
	}
	look.accent()
	look.showKind()
	return look
}

type menuLook struct {
	fyne.WidgetRenderer
	menu *Chooser
	// kind is the picture of the value in the box, nil on a menu without
	// pictures, and objects is the toolkit's objects with it added.
	kind    *canvas.Image
	objects []fyne.CanvasObject
}

func (m *menuLook) Objects() []fyne.CanvasObject { return m.objects }

// Layout lets the toolkit place its objects and then makes room for the
// picture. After, because the toolkit's Layout puts the value back where it
// keeps it every time it runs.
func (m *menuLook) Layout(size fyne.Size) {
	m.WidgetRenderer.Layout(size)
	m.placeKind(size)
}

func (m *menuLook) Refresh() {
	// The toolkit's Refresh lays itself out again, which moves the value back
	// under the picture - so the picture's room is made again after it.
	m.WidgetRenderer.Refresh()
	m.accent()
	m.showKind()
	m.placeKind(m.menu.Size())
}

// showKind puts the picture of the value now in the box into the box, or
// hides it when the value has none.
func (m *menuLook) showKind() {
	if m.kind == nil {
		return
	}
	m.kind.Resource = m.menu.KindOf(m.menu.Selected)
	if m.kind.Resource == nil {
		m.kind.Hide()
	} else {
		m.kind.Show()
	}
	m.kind.Refresh()
}

// placeKind stands the picture where the toolkit starts the value's words and
// moves the words along by the picture and a gap. The words are found by type,
// the way accent finds the arrow: the toolkit's renderer holds exactly one
// RichText (fyne v2.8.1 widget/select.go, CreateRenderer).
//
// Placed by the toolkit's own arithmetic rather than by moving whatever is
// there, so running it twice gives the same picture: the words start at the
// padding and end at the arrow, which stands the inner padding in from the
// right edge (selectRenderer.Layout, same file), and the words draw their
// own inset of the padding inside that.
func (m *menuLook) placeKind(size fyne.Size) {
	if m.kind == nil || !m.kind.Visible() {
		return
	}
	icon := Theme().Size(theme.SizeNameInlineIcon)
	pad := Theme().Size(theme.SizeNamePadding)
	arrow := size.Width - icon - Theme().Size(theme.SizeNameInnerPadding)
	for _, o := range m.WidgetRenderer.Objects() {
		words, ok := o.(*widget.RichText)
		if !ok {
			continue
		}
		m.kind.Resize(fyne.NewSquareSize(icon))
		m.kind.Move(fyne.NewPos(2*pad, (size.Height-icon)/2))
		shift := icon + rowGap
		words.Move(fyne.NewPos(pad+shift, words.Position().Y))
		words.Resize(fyne.NewSize(arrow-pad-shift, words.Size().Height))
		return
	}
}

// accent colours the arrow, unless the menu is disabled.
func (m *menuLook) accent() {
	if m.menu.Disabled() {
		return
	}
	for _, o := range m.WidgetRenderer.Objects() {
		if icon, ok := o.(*widget.Icon); ok {
			icon.Resource = theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNamePrimary)
			icon.Refresh()
			return
		}
	}
}
