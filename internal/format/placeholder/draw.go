package placeholder

import (
	"image"
	"image/color"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/raster"
)

// Look is what one placeholder picture shows. Text is already expanded - the
// tokens of the setting replaced for this file.
type Look struct {
	Text       string
	Label      string
	Background color.RGBA
	Foreground color.RGBA
	Guides     bool
}

// Drawn is what a picture ended up carrying. A text or a label that does not
// fit is left out rather than squeezed below what can be read, and the caller
// says so in the manifest (rule 6).
type Drawn struct {
	Text  bool
	Label bool
}

// MinType is the smallest size of type, in pixels to the em, a text or a label
// is set in. Below it the capitals are under seven pixels tall and a digit is
// a smudge, so the words are left out and the manifest says why.
const MinType = 10

// The share of the picture the text may take: four fifths of the width, and a
// line a quarter of the height. A label is a quarter of the size of the text
// and may take nine tenths of the width.
const (
	textWidthNum, textWidthDen   = 8, 10
	lineHeightDen                = 4
	labelShareDen                = 4
	labelWidthNum, labelWidthDen = 9, 10
)

type line struct {
	text     string
	size     int64 // pixels to the em, 0 when the line is not drawn
	x        int64 // left end of the pen, in coordinates
	baseline int64 // in coordinates
	// span is how wide the line is taken to be, in units of the font: its
	// advance, or for the label the bound labelAdvance gives.
	span int64
}

type layout struct {
	main, label line
	plate       image.Rectangle
}

// Measure is what Draw would carry on a picture of w by h, without drawing -
// what a plan asks before any pixel exists.
func Measure(w, h int, l Look) Drawn {
	lay := arrange(glyphs.Inter(), int64(w), int64(h), l)
	return Drawn{Text: lay.main.size > 0, Label: lay.label.size > 0}
}

func arrange(f *glyphs.Face, w, h int64, l Look) layout {
	upm := int64(f.UnitsPerEm)
	var lay layout
	if adv := f.Advance(l.Text); adv > 0 {
		lay.main = line{text: l.Text, span: adv, size: readable(min(textWidthNum*w*upm/(textWidthDen*adv),
			h*upm/(lineHeightDen*int64(f.Ascent+f.Descent))))}
	}
	if adv := labelAdvance(f, l.Label); adv > 0 {
		lay.label = line{text: l.Label, span: adv, size: readable(min(max(lay.main.size/labelShareDen, MinType),
			labelWidthNum*w*upm/(labelWidthDen*adv)))}
	}
	top, bottom := block(f, lay, h)
	if bottom > h*raster.One && lay.label.size > 0 {
		// The label goes first. It says what the file is, the text is what
		// the picture is for.
		lay.label = line{}
		top, bottom = block(f, lay, h)
	}
	lay.main.baseline = top + capOf(f, lay.main.size)
	lay.label.baseline = bottom - descOf(f, lay.label.size)
	lay.main.x = centred(f, lay.main, w)
	lay.label.x = centred(f, lay.label, w)
	lay.plate = plateOf(lay, w, h, top, bottom)
	return lay
}

// readable is size, or nought when it is below the smallest type set.
func readable(size int64) int64 {
	if size < MinType {
		return 0
	}
	return size
}

func capOf(f *glyphs.Face, size int64) int64 {
	return int64(f.CapHeight) * size * raster.One / int64(f.UnitsPerEm)
}

func descOf(f *glyphs.Face, size int64) int64 {
	return int64(f.Descent) * size * raster.One / int64(f.UnitsPerEm)
}

// block is where the lines stand together, centred on the height: from the
// top of the capitals of the first to the foot of the descenders of the last,
// with half a capital between them.
func block(f *glyphs.Face, lay layout, h int64) (top, bottom int64) {
	height, last := int64(0), lay.main.size
	if lay.main.size > 0 {
		height += capOf(f, lay.main.size)
	}
	if lay.label.size > 0 {
		height += capOf(f, lay.main.size)/2 + capOf(f, lay.label.size)
		last = lay.label.size
	}
	top = (h*raster.One - height) / 2
	return top, top + height + descOf(f, last)
}

// numerals are the characters a label's numbers are written in: the size in
// decimal and the seed in hexadecimal.
const numerals = "0123456789abcdef"

// labelAdvance is how wide the label is taken to be wherever its width decides
// something - its size and the plate behind it: every numeral counted as the
// widest of them.
//
// The label states a seed in eight numerals, and which ones came up changes
// how wide they are - measured on 2026-11-14, a drawing's smallest size moved
// by 2.5 kB between ten seeds while its picture stayed the same. A layout
// taken from the real width would make what a file of these settings needs
// depend on its seed, and the smallest size the tool announces has to hold
// for every seed. The cost is a label a hair smaller where the width is what
// limits it. The letters of words that happen to be numerals, such as the a of
// damage, are counted the same way, which only widens the bound.
func labelAdvance(f *glyphs.Face, label string) int64 {
	widest := int64(0)
	for _, r := range numerals {
		if g, ok := f.Glyph(r); ok {
			widest = max(widest, int64(g.Advance))
		}
	}
	var adv int64
	for _, r := range label {
		if strings.ContainsRune(numerals, r) {
			adv += widest
		} else if g, ok := f.Glyph(r); ok {
			adv += int64(g.Advance)
		}
	}
	return adv
}

// centred is where the pen starts for ln to stand in the middle of the width.
func centred(f *glyphs.Face, ln line, w int64) int64 {
	return (w*raster.One - f.Advance(ln.text)*ln.size*raster.One/int64(f.UnitsPerEm)) / 2
}

// plateOf is the rectangle of background the lines stand on, an eighth of the
// larger size clear around them, inside the picture.
func plateOf(lay layout, w, h, top, bottom int64) image.Rectangle {
	if lay.main.size == 0 && lay.label.size == 0 {
		return image.Rectangle{}
	}
	pad := max(lay.main.size, lay.label.size) / 8 * raster.One
	x0, x1 := w*raster.One, int64(0)
	for _, ln := range []line{lay.main, lay.label} {
		if ln.size > 0 {
			left := (w*raster.One - ln.span*ln.size*raster.One/int64(glyphs.Inter().UnitsPerEm)) / 2
			x0, x1 = min(x0, left), max(x1, w*raster.One-left)
		}
	}
	return image.Rect(int((x0-pad)>>raster.Frac), int((top-pad)>>raster.Frac),
		int((x1+pad+raster.One-1)>>raster.Frac), int((bottom+pad+raster.One-1)>>raster.Frac)).
		Intersect(image.Rect(0, 0, int(w), int(h)))
}

// Scene is a placeholder of w by h as shapes rather than pixels. Draw fills
// these and a format that draws in vectors writes them, so a drawing and a
// picture of one look are the same shapes to the 256th of a pixel.
//
// Laid out in order: the background over the whole picture, the guides in the
// foreground colour, the plate in the background colour, then the text and the
// label in the foreground colour.
type Scene struct {
	// Guides are the frame, the two diagonals and the circle, each filled on
	// its own - see guides. Nil without guides.
	Guides []*raster.Path
	// Plate is the background the lines stand on, so a guide never runs
	// through a letter. Empty without guides, where nothing runs there.
	Plate image.Rectangle
	// Text is the outline of the words, nil when they did not fit.
	Text *raster.Path
	// Label is the outline of the label, nil when it did not fit or there is
	// none. LabelSize and LabelBaseline are where it stands - pixels to the em
	// and 256ths of a pixel down - for a format that writes it as words rather
	// than as shapes. It stands in the middle of the width.
	Label         *raster.Path
	LabelSize     int64
	LabelBaseline int64
	Drawn         Drawn
}

// Shapes is the scene of l on a picture of w by h.
func Shapes(w, h int, l Look) Scene {
	var s Scene
	if l.Guides {
		s.Guides = guides(int64(w), int64(h))
	}
	f := glyphs.Inter()
	lay := arrange(f, int64(w), int64(h), l)
	if l.Guides {
		s.Plate = lay.plate
	}
	if lay.main.size > 0 {
		s.Text = textPath(f, lay.main)
	}
	if lay.label.size > 0 {
		s.Label = textPath(f, lay.label)
		s.LabelSize, s.LabelBaseline = lay.label.size, lay.label.baseline
	}
	s.Drawn = Drawn{Text: s.Text != nil, Label: s.Label != nil}
	return s
}

// Draw paints l over the whole of img: the scene of its size, filled.
func Draw(img *image.RGBA, l Look) Drawn {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	s := Shapes(w, h, l)
	fillRect(img, image.Rect(0, 0, w, h), l.Background)
	plot := func(x, y int, a uint8) { blend(img, x, y, l.Foreground, a) }
	for _, g := range s.Guides {
		raster.Fill(g, w, h, plot)
	}
	fillRect(img, s.Plate, l.Background)
	for _, ln := range []*raster.Path{s.Text, s.Label} {
		if ln != nil {
			raster.Fill(ln, w, h, plot)
		}
	}
	return s.Drawn
}

// fillRect sets every pixel of r to c, opaque.
func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[y*img.Stride+r.Min.X*4 : y*img.Stride+r.Max.X*4]
		for x := 0; x < len(row); x += 4 {
			row[x], row[x+1], row[x+2], row[x+3] = c.R, c.G, c.B, 255
		}
	}
}

// blend lays colour c over the pixel at x, y with coverage a out of 255.
func blend(img *image.RGBA, x, y int, c color.RGBA, a uint8) {
	i := y*img.Stride + x*4
	p := img.Pix[i : i+4 : i+4]
	k, rest := int(a), 255-int(a)
	p[0] = uint8((int(c.R)*k + int(p[0])*rest + 127) / 255)
	p[1] = uint8((int(c.G)*k + int(p[1])*rest + 127) / 255)
	p[2] = uint8((int(c.B)*k + int(p[2])*rest + 127) / 255)
	p[3] = 255
}

// textPath is the outline of one line of text, the pen moving in units of the
// font so rounding never builds up along a long line. No kerning - see
// glyphs.Face.Advance.
func textPath(f *glyphs.Face, ln line) *raster.Path {
	p := raster.NewPath()
	upm := int64(f.UnitsPerEm)
	var pen int64
	for _, r := range ln.text {
		g, _ := f.Glyph(r)
		at := func(pt glyphs.Point) (int64, int64) {
			return ln.x + (pen+int64(pt.X))*ln.size*raster.One/upm,
				ln.baseline + int64(pt.Y)*ln.size*raster.One/upm
		}
		outline(p, g, at)
		pen += int64(g.Advance)
	}
	return p
}

// outline adds the contours of one letter, its points placed by at.
func outline(p *raster.Path, g glyphs.Glyph, at func(glyphs.Point) (int64, int64)) {
	k := 0
	for _, op := range g.Ops {
		switch op {
		case glyphs.MoveTo:
			p.MoveTo(at(g.Points[k]))
		case glyphs.LineTo:
			p.LineTo(at(g.Points[k]))
		case glyphs.QuadTo:
			cx, cy := at(g.Points[k])
			x, y := at(g.Points[k+1])
			p.QuadTo(cx, cy, x, y)
			k++
		}
		k++
	}
	p.Close()
}

// guides is the frame along the edges, the two diagonals and the circle that
// touches the frame: after an upload a missing edge is a crop, diagonals that
// no longer meet at the centre are a crop off centre, and an oval is a
// stretch.
//
// Four outlines filled one by one rather than one outline. Where two shapes
// wound in opposite directions overlap, the non-zero rule counts the overlap
// as outside, and the first drawing had a pale dot wherever a diagonal
// crossed the circle. Drawn one over the other they cannot cancel, and the
// colour laid twice is the same colour.
func guides(w, h int64) []*raster.Path {
	t := max(1, min(w, h)/200) * raster.One
	W, H := w*raster.One, h*raster.One

	frame := raster.NewPath()
	rect := func(x0, y0, x1, y1 int64) {
		frame.MoveTo(x0, y0)
		frame.LineTo(x1, y0)
		frame.LineTo(x1, y1)
		frame.LineTo(x0, y1)
		frame.Close()
	}
	rect(0, 0, W, t)
	rect(0, H-t, W, H)
	rect(0, t, t, H-t)
	rect(W-t, t, W, H-t)
	out := []*raster.Path{frame}

	length := raster.Isqrt(w*w + h*h)
	if length > 0 {
		ox, oy := h*t/(2*length), w*t/(2*length)
		bar := func(x0, y0, x1, y1, sx, sy int64) *raster.Path {
			p := raster.NewPath()
			p.MoveTo(x0+sx, y0+sy)
			p.LineTo(x1+sx, y1+sy)
			p.LineTo(x1-sx, y1-sy)
			p.LineTo(x0-sx, y0-sy)
			p.Close()
			return p
		}
		out = append(out, bar(0, 0, W, H, ox, -oy), bar(W, 0, 0, H, -ox, -oy))
	}

	ring := raster.NewPath()
	cx, cy := W/2, H/2
	r := min(W, H) / 2
	circle(ring, cx, cy, r, false)
	if r > t {
		circle(ring, cx, cy, r-t, true)
	}
	return append(out, ring)
}

// circle adds a circle of radius r as eight quadratic arcs, clockwise on
// screen, or the other way round to cut a hole. The arcs stray from the true
// circle by under a thousandth of the radius.
func circle(p *raster.Path, cx, cy, r int64, reverse bool) {
	// cos 45 and tan 22.5 degrees as fractions of 65536.
	d := r * 46341 / 65536
	k := r * 27146 / 65536
	on := [8][2]int64{{r, 0}, {d, d}, {0, r}, {-d, d}, {-r, 0}, {-d, -d}, {0, -r}, {d, -d}}
	ctl := [8][2]int64{{r, k}, {k, r}, {-k, r}, {-r, k}, {-r, -k}, {-k, -r}, {k, -r}, {r, -k}}
	if !reverse {
		p.MoveTo(cx+on[0][0], cy+on[0][1])
		for i := 0; i < 8; i++ {
			n := on[(i+1)%8]
			p.QuadTo(cx+ctl[i][0], cy+ctl[i][1], cx+n[0], cy+n[1])
		}
	} else {
		p.MoveTo(cx+on[0][0], cy+on[0][1])
		for i := 7; i >= 0; i-- {
			n := on[i]
			p.QuadTo(cx+ctl[i][0], cy+ctl[i][1], cx+n[0], cy+n[1])
		}
	}
	p.Close()
}
