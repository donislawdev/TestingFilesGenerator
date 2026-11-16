package svgfile

import (
	"context"
	"fmt"
	"io"
	// D11 promises the same bytes from the same seed. Nothing here makes a
	// secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/raster"
)

// A placeholder drawing is the shapes the picture formats fill with pixels,
// written as shapes (placeholder.Shapes): the same outlines to the 256th of a
// pixel, so a page laid out with the SVG and with the PNG looks the same, and
// the letters are Inter on every machine rather than whatever font the viewer
// has. docs/PLACEHOLDER-2026-11-14.md.
//
// The words go into a title as well, which is what a screen reader and a
// search read. The padding goes into the description, which no renderer
// draws: unlike the test drawing, more bytes must not mean more shapes here,
// or a placeholder of 2 MB would be covered in circles.
//
// The label is the one thing written as words, a text element in Inter where
// the viewer has it, as the label of the test drawing is. Drawn as outlines it
// was as long in bytes as the shapes of the numerals the seed came up with,
// and measured on 2026-11-14 the smallest drawing moved by 2.5 kB between ten
// seeds - a size the tool announced would be refused for the next file. As
// words it is as long as its characters, and its place is worked out from a
// width that does not depend on them either (placeholder.labelAdvance).

const (
	// unitScale turns the coordinates of an outline, 256ths of a pixel, into
	// the pixels of the drawing. Exact in decimal, as every power of two is,
	// so every coordinate is a whole number in the file.
	unitScale = `transform="scale(0.00390625)"`

	descOpen = "<desc>"
	descTail = "</desc>\n" + rootClose
)

// planPlaceholder is Plan for content: placeholder.
func planPlaceholder(r format.Request, w, h int, look placeholder.Settings) (format.Plan, error) {
	label := ""
	if r.Label {
		label = r.PictureLabel("svg")
	}
	text := look.Expand(r.Number, w, h)
	body, drawn := drawing(w, h, look.Look(text, label))
	least := int64(len(declaration)+len(rootOpen(w, h))+len(body)+len(descOpen)) + int64(len(descTail))
	if r.Bytes < least {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "SVG",
			Requested: r.Bytes,
			Minimum:   least,
			Reason:    core.Says("format.APlaceholderPictureHolds", "a placeholder picture holds its shapes, its words and its label, and a %dx%d one needs that much", core.A("Width", w), core.A("Height", h)),
			Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", least)),
		}
	}
	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			Width:         w,
			Height:        h,
			"view_box":    fmt.Sprintf("0 0 %d %d", w, h),
		},
	}
	p.Properties[format.PropertyLabelEmbedded], p.Notes = placeholder.Claim(drawn, label, w, h, p.Notes)
	look.Describe(p.Properties, text, drawn)
	// The length of the drawing rather than the drawing, which is a few
	// kilobytes of outline: a plan holds its memo until the file is written,
	// and a run plans every file before it writes one.
	p.Memo = memo{seed: r.Seed, width: w, height: h, look: look, text: text, label: label, body: int64(len(body))}
	return p, nil
}

// writePlaceholder is Write for content: placeholder.
func writePlaceholder(ctx context.Context, out io.Writer, p format.Plan, m memo) error {
	body, _ := drawing(m.width, m.height, m.look.Look(m.text, m.label))
	if int64(len(body)) != m.body {
		return core.Defect(fmt.Errorf("svg: the placeholder came to %d B where planning said %d B", len(body), m.body))
	}
	head := make([]byte, 0, len(declaration)+len(body)+256)
	head = append(head, declaration...)
	head = append(head, rootOpen(m.width, m.height)...)
	head = append(head, body...)
	head = append(head, descOpen...)
	if err := core.WriteAll(out, head); err != nil {
		return err
	}
	return core.FillRecords(ctx, out, core.NewRand(m.seed), p.Bytes-int64(len(head)), description{})
}

// drawing is the placeholder between the root element and the description:
// its words as a title, the background, the guides, the plate, the words and
// the label, in the order placeholder.Scene lays them.
func drawing(w, h int, l placeholder.Look) ([]byte, placeholder.Drawn) {
	s := placeholder.Shapes(w, h, l)
	bg, fg := "#"+placeholder.Hex(l.Background), "#"+placeholder.Hex(l.Foreground)
	b := make([]byte, 0, 4096)
	b = append(b, "<title>"...)
	b = append(b, escapeText(l.Text)...)
	b = append(b, "</title>\n"...)
	b = fmt.Appendf(b, `<rect width="%d" height="%d" fill="%s"/>`+"\n", w, h, bg)
	b = appendShapes(b, fg, s.Guides)
	if !s.Plate.Empty() {
		b = fmt.Appendf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`+"\n",
			s.Plate.Min.X, s.Plate.Min.Y, s.Plate.Dx(), s.Plate.Dy(), bg)
	}
	if s.Text != nil {
		b = appendShapes(b, fg, []*raster.Path{s.Text})
	}
	if s.Label != nil {
		b = appendLabel(b, w, s, l.Label, fg)
	}
	return b, s.Drawn
}

// appendLabel writes the label as a text element, centred on the width and on
// the baseline the scene gives it, rounded to a whole pixel. None of its
// numbers depends on the label's characters, so its length is the label's
// length and the fixed text around it.
func appendLabel(b []byte, w int, s placeholder.Scene, label, fill string) []byte {
	x := strconv.Itoa(w / 2)
	if w%2 == 1 {
		x += ".5"
	}
	y := (s.LabelBaseline + raster.One/2) >> raster.Frac
	b = fmt.Appendf(b, `<text x="%s" y="%d" font-family="Inter, Arial, sans-serif" font-weight="700" font-size="%d" fill="%s" text-anchor="middle">`,
		x, y, s.LabelSize, fill)
	b = append(b, escapeText(label)...)
	return append(b, "</text>\n"...)
}

// appendShapes writes outlines as one group of paths filled in one colour.
// Each is a path of its own for the reason guides are filled one by one: two
// shapes wound in opposite directions cancel where they overlap.
func appendShapes(b []byte, fill string, paths []*raster.Path) []byte {
	if len(paths) == 0 {
		return b
	}
	b = fmt.Appendf(b, `<g fill="%s" %s>`+"\n", fill, unitScale)
	for _, p := range paths {
		b = append(b, `<path d="`...)
		b = appendPathData(b, p.Segments())
		b = append(b, `"/>`+"\n"...)
	}
	return append(b, "</g>\n"...)
}

// appendPathData writes an outline as SVG path data, in whole 256ths of a
// pixel.
func appendPathData(b []byte, segs []raster.Segment) []byte {
	for i, s := range segs {
		if i > 0 {
			b = append(b, ' ')
		}
		switch s.Op {
		case raster.OpMove:
			b = appendPoint(append(b, 'M'), s.X, s.Y)
		case raster.OpLine:
			b = appendPoint(append(b, 'L'), s.X, s.Y)
		case raster.OpQuad:
			b = appendPoint(append(appendPoint(append(b, 'Q'), s.CX, s.CY), ' '), s.X, s.Y)
		case raster.OpClose:
			b = append(b, 'Z')
		}
	}
	return b
}

func appendPoint(b []byte, x, y int64) []byte {
	return strconv.AppendInt(append(strconv.AppendInt(b, x, 10), ' '), y, 10)
}

// escapeText is text as the content of an element. The words of a placeholder
// are typed by somebody, and a file served from a site opens in a browser:
// </title><script> written in as it stands would be a script (rule 23). The
// text holds no character XML forbids - the typeface has none of them, so
// placeholder.Read refuses them first.
func escapeText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// description pads a placeholder inside its description, a line of words at a
// time, so more bytes never draw anything.
type description struct{}

// descriptionWords is how many words a whole line of padding has.
const descriptionWords = 8

func (description) Append(dst []byte, rng *rand.Rand) []byte {
	for i := 0; i < descriptionWords; i++ {
		if i == 0 {
			dst = append(dst, '\n')
		} else {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[rng.IntN(len(words))]...)
	}
	return dst
}

// AppendExact ends the description at exactly n bytes: what is left of the
// padding on a line of its own, and the tags that close the document.
func (description) AppendExact(dst []byte, _ *rand.Rand, n int64) []byte {
	if room := n - int64(len(descTail)); room > 0 {
		dst = appendFiller(append(dst, '\n'), room-1)
	}
	return append(dst, descTail...)
}

func (description) Shortest() int64 { return int64(len(descTail)) }

// Discard has nothing to put back - a line of words keeps no count.
func (description) Discard() {}
