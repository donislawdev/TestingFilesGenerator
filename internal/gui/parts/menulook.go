package parts

import (
	"fyne.io/fyne/v2"
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
func (c *Chooser) CreateRenderer() fyne.WidgetRenderer {
	look := &menuLook{WidgetRenderer: c.Select.CreateRenderer(), menu: c}
	look.accent()
	return look
}

type menuLook struct {
	fyne.WidgetRenderer
	menu *Chooser
}

func (m *menuLook) Refresh() {
	m.WidgetRenderer.Refresh()
	m.accent()
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
