package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Progress is how far along a run is, drawn as a groove that fills.
//
// A control of our own rather than the toolkit's, and it is the track that
// forces it. The toolkit draws the unfilled part in the accent colour at half
// alpha - progressbar.go, progressBlendColor - so the groove is the same hue
// as the fill and only lighter. Measured off two renders of one run on
// 2026-08-20:
//
//	at 1.2 s   the whole width was #4A6E8B, which is the empty track
//	at 6.0 s   #6FB7F0 to 24 per cent of the width, #4A6E8B after it
//
// So a bar at nothing is a solid blue bar across the foot of the window. The
// fill stands against the track at 2.49 and the track stands against the panel
// behind it at 2.80 - the empty part is more visible against its surroundings
// than the full part is against the empty. Told apart it needs a second look
// and a comparison of two shades of one colour, which is not what progress is
// for: it is read out of the corner of an eye.
//
// The colour cannot be moved from the theme, because both parts come from one
// name. That is the whole reason this is a widget rather than a palette entry.
//
// A groove now: the surface a box is drawn on, so the empty track sits at
// roughly the panel's own lightness and the fill is the only thing on it.
type Progress struct {
	widget.BaseWidget

	// Max is what Value is measured against, kept as a field so this can stand
	// in for the toolkit's bar where it already was.
	Max float64

	value float64
}

// NewProgress builds an empty bar. It counts to a hundred unless told
// otherwise, because a percentage is what every caller here has.
func NewProgress() *Progress {
	p := &Progress{Max: 100}
	p.ExtendBaseWidget(p)
	return p
}

// SetValue moves the fill. Out of range values are pinned rather than refused:
// a progress bar is a picture of something else's arithmetic, and the honest
// answer to a number past the end is a full bar.
func (p *Progress) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if p.Max > 0 && v > p.Max {
		v = p.Max
	}
	p.value = v
	p.Refresh()
}

// Value is how far along the bar says the run is.
func (p *Progress) Value() float64 { return p.value }

// MinSize is the height a track needs and nothing more.
//
// SlimHeight rather than a line of text, for the reason that constant records:
// the toolkit's bar is as tall as the words "100%" because it writes them
// inside itself, and the line under this one already ends with that number.
func (p *Progress) MinSize() fyne.Size {
	p.ExtendBaseWidget(p)
	return fyne.NewSize(theme.Size(theme.SizeNameInlineIcon), SlimHeight)
}

func (p *Progress) CreateRenderer() fyne.WidgetRenderer {
	p.ExtendBaseWidget(p)
	r := &progressRenderer{
		bar:   p,
		track: canvas.NewRectangle(color.Transparent),
		fill:  canvas.NewRectangle(color.Transparent),
	}
	r.applyColours()
	return r
}

type progressRenderer struct {
	bar         *Progress
	track, fill *canvas.Rectangle
}

// applyColours takes both from the palette directly, the way every other
// surface in this package does - one look, whatever the desktop is set to.
func (r *progressRenderer) applyColours() {
	r.track.FillColor = PaletteColour(theme.ColorNameInputBackground, theme.VariantDark)
	r.fill.FillColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	radius := float32(SlimHeight) / 2
	r.track.CornerRadius = radius
	r.fill.CornerRadius = radius
}

func (r *progressRenderer) Layout(size fyne.Size) {
	r.track.Resize(size)
	r.track.Move(fyne.NewPos(0, 0))

	share := float32(0)
	if r.bar.Max > 0 {
		share = float32(r.bar.value / r.bar.Max)
	}
	if share > 1 {
		share = 1
	}
	width := size.Width * share
	// A fill narrower than its own corner radius draws as a dot rather than as
	// a sliver, so nothing at all is drawn until there is something to see.
	if width < SlimHeight {
		width = 0
	}
	r.fill.Resize(fyne.NewSize(width, size.Height))
	r.fill.Move(fyne.NewPos(0, 0))
}

func (r *progressRenderer) MinSize() fyne.Size { return r.bar.MinSize() }

func (r *progressRenderer) Refresh() {
	r.applyColours()
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar)
}

func (r *progressRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill}
}

func (r *progressRenderer) Destroy() {}
