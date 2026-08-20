package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// FadeHeight is how tall the sign that there is more below is drawn.
//
// Enough to read as a soft edge rather than as a line somebody drew, and small
// enough that it never covers a whole control - a field is about 60 px.
const FadeHeight = 20

// WithMoreBelow puts a soft edge at the foot of a form that has more under it.
//
// Two of the three screens cannot fit in the window and one of them never will:
// the batch screen is a list of batches, so at two batches it scrolls whatever
// anybody does to the spacing. Measured on 2026-08-20, after this run of
// changes: 832 px of form in 837 px of room on the single batch screen, 1000 in
// 837 on presets, 1259 in 837 on the batch list.
//
// So the honest answer for those two is not to keep chasing the height. It is
// to stop the form ending in a way that looks like the end. A form cut off at
// the window edge mid-control reads as a screen that is broken, and a form that
// fades reads as a screen with more on it - which is what it is.
//
// It is drawn over the scroll rather than under it, and it takes no room: the
// form is exactly as tall as it was, and this covers the last 20 px of it while
// there is something below to reach.
//
// It disappears at the bottom, which is the half that makes it a report rather
// than a decoration. A permanent shadow at the foot of every screen says
// "there is more" on a screen where there is not, and that is the silence rule
// inverted - a mark that is always on carries no information.
func WithMoreBelow(scroll *container.Scroll) fyne.CanvasObject {
	page := PaletteColour(theme.ColorNameBackground, theme.VariantDark)
	fade := canvas.NewLinearGradient(color.Transparent, page, 0)
	fade.Hide()

	over := container.New(&fadeAtTheFoot{scroll: scroll, fade: fade}, scroll, fade)
	// Told when the form moves, and once at the start so a screen that opens
	// already too tall says so before anybody touches it.
	was := scroll.OnScrolled
	scroll.OnScrolled = func(p fyne.Position) {
		if was != nil {
			was(p)
		}
		over.Refresh()
	}
	return over
}

// fadeAtTheFoot lays the scroll out whole and pins the fade across its bottom
// edge, showing it only while the form runs past what the window can show.
type fadeAtTheFoot struct {
	scroll *container.Scroll
	fade   *canvas.LinearGradient
}

func (f *fadeAtTheFoot) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if f.scroll == nil {
		return fyne.NewSize(0, 0)
	}
	return f.scroll.MinSize()
}

func (f *fadeAtTheFoot) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	f.scroll.Resize(size)
	f.scroll.Move(fyne.NewPos(0, 0))

	if f.more(size) {
		f.fade.Show()
	} else {
		f.fade.Hide()
	}
	f.fade.Resize(fyne.NewSize(size.Width, FadeHeight))
	f.fade.Move(fyne.NewPos(0, size.Height-FadeHeight))
}

// more says whether anything is still below the bottom edge.
//
// Asked of the scroll rather than remembered, because the answer changes three
// ways: the window is resized, the form grows when a format declares settings,
// and somebody scrolls to the end.
func (f *fadeAtTheFoot) more(size fyne.Size) bool {
	if f.scroll == nil || f.scroll.Content == nil {
		return false
	}
	height := size.Height
	if height == 0 {
		height = f.scroll.Size().Height
	}
	// A pixel of slack, so a form that fits exactly does not flicker a fade on
	// and off with rounding.
	return f.scroll.Content.MinSize().Height-(f.scroll.Offset.Y+height) > 1
}
