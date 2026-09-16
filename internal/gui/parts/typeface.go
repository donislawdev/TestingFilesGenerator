package parts

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/font"
)

// The window is set in Inter (internal/gui/font).
//
// Until 2026-09-15 it was set in Noto Sans, which is what the toolkit bundles
// as its text face (theme/bundled-fonts.go in fyne v2.8.1 - measured, because
// an earlier note in this project said the window already drew in Inter, and
// what the toolkit embeds of Inter is the symbols file alone). Nothing was
// wrong with Noto Sans as a face. What was wrong is the sentence
// docs/GUI.md section 10.1 records: a window of stock widgets in the stock
// face looks like the library it was built with, and after steps 1 to 5 of
// the rework this was the one thing left on screen that still did.
//
// One resource each rather than one per call: the painter caches the faces
// it shapes with under the resource's name, so the two names below are the
// keys of that cache and must stay distinct from each other and from the
// toolkit's own.
var (
	interRegular = fyne.NewStaticResource("Inter-Regular.ttf", font.Regular)
	interBold    = fyne.NewStaticResource("Inter-Bold.ttf", font.Bold)
)

// Font is Inter for the two styles the window draws in, and the toolkit's own
// face for everything else.
//
// The toolkit resolves a style in the order monospace, bold, italic, symbol
// (theme.go, DefaultTheme). The same order is kept here so that a style this
// window never asks for - bold italic, say - falls to exactly the face the
// toolkit would have chosen, rather than to a bold Inter beside an italic Noto.
func (o ours) Font(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Monospace || style.Italic || style.Symbol:
		return o.Theme.Font(style)
	case style.Bold:
		return interBold
	}
	return interRegular
}
