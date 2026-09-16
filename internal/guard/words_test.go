package guard

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// wordsOf is the text an object shows, whichever of the two kinds of object
// that show words it is.
//
// Until 2026-09-14 every word on the window was a toolkit label, and every
// guard reading the screen asked for that one type. A field's name, a screen's
// title and a section's title are canvas texts now - drawn with no room of
// their own around the ink, so the gaps a layout is given are the gaps that
// reach the screen (parts/tokens.go) - and a guard that still asked for a
// label found nothing where the name is. That failure was loud, which is the
// good kind: nine guards named a field as nil in the first run. This is the
// one place the two kinds are told apart, so the next kind is one more case
// here rather than a hunt through thirty type assertions.
//
// A hidden object still answers. Whether something is on the screen is a
// question about its ancestors as well, and sizechoice_test.go asks it
// properly. Nothing here filters.
func wordsOf(o fyne.CanvasObject) (string, bool) {
	switch v := o.(type) {
	case *widget.Label:
		return v.Text, true
	case *canvas.Text:
		return v.Text, true
	}
	return "", false
}

// boldWordsAt is wordsOf for the guards that read RANK: whether the words are
// bold and at what size they are drawn. A label carries its size as a theme
// name and a canvas text as a number, so the size comes back in pixels for
// both, read from the theme the guards install.
func boldWordsAt(o fyne.CanvasObject) (text string, bold bool, size float32, ok bool) {
	switch v := o.(type) {
	case *widget.Label:
		name := v.SizeName
		if name == "" {
			name = "text"
		}
		return v.Text, v.TextStyle.Bold, fyne.CurrentApp().Settings().Theme().Size(name), true
	case *canvas.Text:
		return v.Text, v.TextStyle.Bold, v.TextSize, true
	}
	return "", false, 0, false
}
