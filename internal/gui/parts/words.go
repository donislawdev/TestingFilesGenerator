package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// words is one line of text drawn with no room of its own around the ink.
//
// A toolkit label pads itself by the theme's inner padding on every side, and
// that padding is what a form is made of once there are labels in it: every
// distance between a name and the thing under it was the sum of a gap somebody
// chose and six pixels nobody did. Measured on 2026-09-11 as 149 different
// gaps across the stored screens. A canvas text has the size of its letters
// and nothing else, so a gap given to a layout is the gap that reaches the
// screen.
//
// What it gives up is wrapping - a canvas text is one line, always - which is
// right for a name, a title and a mark, and wrong for a sentence. Sentences
// stay toolkit labels, made ink tight by inkTight below.
//
// The colour is read from the palette directly rather than through the
// installed theme, for the reason panelSurface gives: this window answers dark
// whatever the desktop says.
func words(text string, size float32, bold bool, colour fyne.ThemeColorName) *canvas.Text {
	t := canvas.NewText(text, PaletteColour(colour, theme.VariantDark))
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

// inkTight takes the toolkit's own padding off a label, so a sentence that
// wraps stands on the scale like everything else.
//
// A theme override rather than a widget of ours, because the label keeps
// everything a sentence needs - wrapping, the caption size, the colour an
// importance gives it - and the only thing wrong with it is the room it keeps
// around itself. The override is the toolkit's one public door into a
// subtree's theme, and it is used bare rather than wrapped in a type of ours,
// because the driver casts to it.
func inkTight(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(o, noInnerPadding{Theme()})
}

// Flush puts a label a screen keeps hold of on the edge every other word
// stands on. The same override as inkTight, exported for the one label the
// runner writes to for the whole life of a screen - the line a run speaks on
// - which has to be built by the runner and placed by the bar.
func Flush(label *widget.Label) fyne.CanvasObject { return inkTight(label) }

// noInnerPadding is the window's theme with the room inside a label taken out.
// Only that one size, so a box to type in under the same override would still
// be a box.
type noInnerPadding struct{ fyne.Theme }

func (n noInnerPadding) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 0
	}
	return n.Theme.Size(name)
}

// Padded keeps one distance from the scale between its edge and its content.
//
// The toolkit's padded container reads the theme's padding, which is the
// smallest step - the right distance between two things in a row and the
// wrong one between a panel's edge and what it holds.
func Padded(inset float32, content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(padded{inset: inset}, content)
}

// padded is the layout behind Padded.
type padded struct{ inset float32 }

func (p padded) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, o := range objects {
		min := o.MinSize()
		size.Width = fyne.Max(size.Width, min.Width)
		size.Height = fyne.Max(size.Height, min.Height)
	}
	return size.Add(fyne.NewSquareSize(p.inset * 2))
}

func (p padded) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	inner := size.Subtract(fyne.NewSquareSize(p.inset * 2))
	for _, o := range objects {
		o.Resize(fyne.NewSize(fyne.Max(0, inner.Width), fyne.Max(0, inner.Height)))
		o.Move(fyne.NewPos(p.inset, p.inset))
	}
}
