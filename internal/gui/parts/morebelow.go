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
	fade := canvas.NewLinearGradient(color.Transparent, edgeShadow(), 0)
	fade.Hide()

	mark := canvas.NewImageFromResource(theme.NewThemedResource(theme.MenuDropDownIcon()))
	mark.FillMode = canvas.ImageFillContain
	mark.Hide()

	over := container.New(&fadeAtTheFoot{scroll: scroll, fade: fade, mark: mark}, scroll, fade, mark)
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

// MarkSize is how big the arrow at the foot of a form that has more below is.
//
// The size of a glyph on a line of text rather than a picture. It is a second
// way of saying the same thing, not the thing itself.
const MarkSize = 14

// edgeShadow is what the foot of a form fades INTO.
//
// It used to fade into the page, and that was half a sign. Measured on
// 2026-08-24 off a render of the preset screen: where the last thing above the
// edge is text the fade is unmistakable, because the writing goes from 91 L* to
// the page in twelve pixels - but where the last thing is an empty stretch of
// panel the whole band moves 5.9 L*, which is under the 10 this project calls
// noticeable. So the sign was strongest exactly where a reader least needed it
// and absent where a form quietly went on past the edge.
//
// It fades into the shadow the palette already keeps for a thing that floats
// over the page - which is what the bar at the foot IS - rather than into a new
// number. Composited onto the page so it stays opaque: a translucent end would
// leave the cut-off line dark but legible, and a half-line of readable text at
// the edge is the rendering fault this whole thing exists to stop looking like.
//
// Measured after the change: the band runs from the panel down to 2.8 L*, which
// is 14.4 - so it reads with no text in it at all.
func edgeShadow() color.Color {
	page := PaletteColour(theme.ColorNameBackground, theme.VariantDark)
	return flatten(PaletteColour(theme.ColorNameShadow, theme.VariantDark), page)
}

// flatten lays one colour over another and returns what an eye would see, with
// no transparency left.
//
// The palette holds several colours the toolkit BLENDS rather than paints - see
// the note on hover in theme.go - and a gradient needs somewhere solid to end.
func flatten(over, under color.Color) color.Color {
	or, og, ob, oa := over.RGBA()
	ur, ug, ub, _ := under.RGBA()
	mix := func(o, u uint32) uint8 {
		return uint8((o*oa + u*(0xFFFF-oa)) / 0xFFFF >> 8)
	}
	return color.NRGBA{R: mix(or, ur), G: mix(og, ug), B: mix(ob, ub), A: 0xFF}
}

// fadeAtTheFoot lays the scroll out whole and pins the fade across its bottom
// edge, showing it only while the form runs past what the window can show.
type fadeAtTheFoot struct {
	scroll *container.Scroll
	fade   *canvas.LinearGradient
	// mark is the arrow drawn in the fade, and it is there because UX1 says
	// colour is never the only carrier. A band that darkens is a colour and
	// nothing else - somebody who cannot tell these two darks apart gets no
	// sign at all from it, and an arrow is a shape they do get.
	mark *canvas.Image
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
		f.mark.Show()
	} else {
		f.fade.Hide()
		f.mark.Hide()
	}
	f.fade.Resize(fyne.NewSize(size.Width, FadeHeight))
	f.fade.Move(fyne.NewPos(0, size.Height-FadeHeight))

	// Centred across the width and sitting in the darkest part of the band, so
	// it has the whole of the fade behind it to be read against.
	f.mark.Resize(fyne.NewSize(MarkSize, MarkSize))
	f.mark.Move(fyne.NewPos((size.Width-MarkSize)/2, size.Height-MarkSize))
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
