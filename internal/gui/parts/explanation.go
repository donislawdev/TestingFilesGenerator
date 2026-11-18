package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Explanation is a paragraph of this program's own words that may name values
// to type (core.Pieces): the words as Prose draws them, and each value in the
// type of code in ColorNameLiteral, without its marks. A paragraph naming
// none is Prose itself, drawn exactly as before.
//
// For our own words only. Prose stays plain for whatever comes from outside -
// a tool's result names files out of somebody's archive, and a backtick in one
// of those names is a backtick, not the start of a value.
func Explanation(s string) fyne.CanvasObject {
	pieces := core.Pieces(s)
	if len(core.Literals(s)) == 0 {
		return Prose(s)
	}
	segments := make([]widget.RichTextSegment, 0, len(pieces))
	for _, p := range pieces {
		style := widget.RichTextStyleInline
		if p.Literal {
			style.TextStyle = fyne.TextStyle{Monospace: true}
			style.ColorName = ColorNameLiteral
		}
		segments = append(segments, &widget.TextSegment{Text: p.Text, Style: style})
	}
	rich := widget.NewRichText(segments...)
	rich.Wrapping = fyne.TextWrapWord
	return inkTight(rich)
}
