// Package svgfile generates SVG drawings.
//
// The package carries the "file" suffix like logfile, csvfile, jsonfile and
// xmlfile, so the whole group reads the same way. The format id is "svg".
package svgfile

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Measured on 2026-08-01, a comment holds arbitrary bytes to 1 MiB both after
// the closing tag and inside the root. That says where the format tolerates
// arbitrary bytes. It does not say where the filling should go.
//
// A comment is the wrong answer, for the fourth time in this codebase. A five
// megabyte drawing built as a handful of shapes plus a five megabyte comment is
// the right size and draws almost nothing, and a renderer discards comments
// before it starts. So the filling goes through whole shapes, and the remainder
// to the exact byte lands in the text of a closing label, where it stays
// smaller than a single shape.
//
// Same shape as LOG, CSV, JSON and XML. At this point it is the rule rather
// than the exception: padding goes where the format has room for a long value,
// never into a truncated record.

const (
	generatorVersion = "1"

	width  = 800
	height = 600

	declaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	rootOpen    = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="600" viewBox="0 0 800 600">` + "\n"
	rootClose   = "</svg>\n"

	// The closing record is a text element, so the drawing ends with something
	// that can be stretched to any length without changing what it is.
	textOpen  = `<text x="16" y="` + textY + `" font-size="13" fill="#333333">`
	textY     = "584"
	textClose = "</text>\n"

	tailLast = textClose + rootClose

	// fixedWidth is every literal byte of the closing record.
	fixedWidth = len(textOpen) + len(tailLast)
)

func init() {
	format.Register(format.Descriptor{
		ID:          "svg",
		Extension:   ".svg",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A root element with nothing in it is legal SVG and draws a blank
		// rectangle. That is a shape request rather than a byte count, and it
		// arrives with the shape count property. The minimum here is the
		// declaration, the root and one whole record.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the text of the closing label",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// A drawing shows what it is, so the label is a visible text element
		// rather than a comment.
		Label:  format.LabelVisible,
		Oracle: "inkscape",
		// Dimensions, shape counts, gradients, fonts, embedded rasters and SMIL
		// come later. Declaring none now makes a recipe asking for them fail
		// loudly.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	labelLine string // includes the trailing newline, empty when absent
	seed      uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "SVG",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a drawing holds a declaration, a root element and whole shapes, and one of each needs that much",
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			"width":       width,
			"height":      height,
			"view_box":    "0 0 800 600",
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		line := textOpen + core.Label("svg", r.Bytes, r.Seed) + textClose
		if int64(len(line))+minimumBytes() <= r.Bytes {
			m.labelLine = line
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label needs %d B and this file has no room for it beside a whole shape. Its name and the manifest still identify it.",
					len(line)),
			})
		}
	}

	p.Properties["label_embedded"] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("svg: the plan was not produced by this generator")
	}

	head := declaration + rootOpen + m.labelLine
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), shapes{})
}

// shapes builds the drawing. A natural record is one shape, and the closing
// record is a text label stretched to land the byte count.
type shapes struct{}

// Shortest is the smallest closing record: the label element with no text at
// all, plus the bytes that close the root.
func (shapes) Shortest() int64 { return int64(fixedWidth) }

func (shapes) Append(dst []byte, rng *rand.Rand) []byte {
	// A line is drawn with a stroke and a closed shape is filled. Each branch
	// says which it wants, because a shape painted the wrong way is invisible
	// and the size never notices.
	paint := "fill"

	switch rng.IntN(4) {
	case 0:
		dst = append(dst, `<rect x="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(width-80)), 10)
		dst = append(dst, `" y="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(height-80)), 10)
		dst = append(dst, `" width="`...)
		dst = strconv.AppendInt(dst, int64(20+rng.IntN(60)), 10)
		dst = append(dst, `" height="`...)
		dst = strconv.AppendInt(dst, int64(20+rng.IntN(60)), 10)
	case 1:
		dst = append(dst, `<circle cx="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(width)), 10)
		dst = append(dst, `" cy="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(height)), 10)
		dst = append(dst, `" r="`...)
		dst = strconv.AppendInt(dst, int64(5+rng.IntN(40)), 10)
	case 2:
		dst = append(dst, `<ellipse cx="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(width)), 10)
		dst = append(dst, `" cy="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(height)), 10)
		dst = append(dst, `" rx="`...)
		dst = strconv.AppendInt(dst, int64(10+rng.IntN(50)), 10)
		dst = append(dst, `" ry="`...)
		dst = strconv.AppendInt(dst, int64(10+rng.IntN(50)), 10)
	default:
		dst = append(dst, `<line x1="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(width)), 10)
		dst = append(dst, `" y1="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(height)), 10)
		dst = append(dst, `" x2="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(width)), 10)
		dst = append(dst, `" y2="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(height)), 10)
		dst = append(dst, `" stroke-width="2`...)
		paint = "stroke"
	}

	dst = append(dst, `" `...)
	dst = append(dst, paint...)
	dst = append(dst, `="`...)
	dst = append(dst, colours[rng.IntN(len(colours))]...)
	return append(dst, `"/>`+"\n"...)
}

func (shapes) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	start := len(dst)
	dst = append(dst, textOpen...)
	used := int64(len(dst)-start) + int64(len(tailLast))
	dst = appendFiller(dst, n-used)
	return append(dst, tailLast...)
}

// appendFiller writes exactly n bytes of label text out of readable words.
//
// It never emits an ampersand or an angle bracket, the characters that would
// have to be escaped - an escape would make the text longer than the count
// asked for.
func appendFiller(dst []byte, n int64) []byte {
	if n <= 0 {
		return dst
	}
	start := len(dst)
	for i := 0; int64(len(dst)-start) < n; i++ {
		if len(dst) > start {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[i%len(words)]...)
	}
	return dst[:start+int(n)]
}

// minimumBytes is the declaration, the root element and one whole record,
// computed rather than written down so it cannot drift away from the template.
func minimumBytes() int64 {
	var s shapes
	return int64(len(declaration)+len(rootOpen)) + s.Shortest()
}

var colours = []string{
	"#4a90d9", "#7ed321", "#f5a623", "#d0021b", "#9013fe",
	"#50e3c2", "#b8e986", "#417505", "#bd10e0", "#8b572a",
}

// words is the vocabulary for the closing label. English by default, like the
// rest of the text group.
var words = []string{
	"anchor", "border", "canvas", "circle", "colour", "corner", "cursor",
	"dashed", "figure", "filter", "gradient", "layer", "legend", "marker",
	"matrix", "opacity", "outline", "overlay", "palette", "pattern", "raster",
	"render", "scale", "shadow", "shape", "sketch", "spline", "stroke",
	"surface", "texture", "transform", "vector", "viewport",
}
