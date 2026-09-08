// Package svgfile generates SVG drawings.
//
// The package carries the "file" suffix like logfile, csvfile, jsonfile and
// xmlfile, so the whole group reads the same way. The format id is "svg".
package svgfile

import (
	"context"
	"fmt"
	"io"
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagedim"
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

	defaultWidth  = 800
	defaultHeight = 600

	// The range the two dimensions accept.
	//
	// The upper end is the same number the nine picture formats use, and that
	// is deliberate rather than lazy: somebody who has learnt "width up to
	// 20000" for PNG does not learn a second number here. What is NOT carried
	// over is their joint ceiling of 40 megapixels, because the reason for it
	// does not exist here - a picture format holds the raster in memory while
	// it encodes, and this one writes text.
	//
	// Measured 2026-09-08, a document of this shape rendered headless by
	// Inkscape 1.4.4 and read back with Pillow:
	//
	//	  10000x8000       80 Mpx     5.3 s      339 kB
	//	  20000x20000     400 Mpx    25.6 s      1.6 MB
	//	  32000x32000    1024 Mpx    43.6 s      4.1 MB
	//	  65536x65536    4295 Mpx   165.3 s       17 MB
	//
	// So the renderer does not set this ceiling - it never broke. The line
	// worth crossing belongs to a reader instead: Pillow refuses an image over
	// PIL.Image.MAX_IMAGE_PIXELS, measured at 89478485 px, as a decompression
	// bomb. 20000 per axis reaches 400 Mpx, four and a half times over that
	// line, so a set can hold files on both sides of it. That is how the CSV
	// column ceiling was chosen too - above the point where a real reader
	// starts to say no, not as high as the arithmetic allows.
	minDimension = 1
	maxDimension = 20000

	// TextBand is the strip along the bottom edge that shapes stay out of, so
	// the two lines of text below it are read against plain background.
	//
	// Without it about one shape in ten landed on the label - measured on a
	// small and a large file, 11.0% and 10.3% - and the label is the one thing
	// in the file that says what the file is. A drawing is still a drawing with
	// a margin. A label with a circle through it is not a label.
	TextBand = 56

	// Width and Height name the two settings. Exported so that a guard presses
	// the key this format actually declares rather than a string spelled twice,
	// which is the same reason jsonfile exports the name of its layout setting.
	//
	// Taken from the package the picture formats share rather than spelled
	// again here, so that a drawing and a photograph cannot end up naming the
	// same setting differently.
	Width  = imagedim.SettingWidth
	Height = imagedim.SettingHeight

	declaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	rootClose   = "</svg>\n"

	textClose = "</text>\n"

	tailLast = textClose + rootClose
)

// rootOpen is the opening tag for a drawing of these dimensions.
//
// It used to be a constant with 800 and 600 written into it, which is why the
// minimum below was a constant too. Both now depend on the dimensions asked
// for: "width=\"20000\"" is three bytes longer than "width=\"800\"", and the
// baseline of a text element near the bottom edge of a tall drawing is a
// longer number as well.
func rootOpen(w, h int) string {
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		w, h, w, h)
}

// textOpen opens the closing record: a text element, so the drawing ends with
// something that can be stretched to any length without changing what it is.
//
// It gets its own baseline. Sharing one with the identity label meant the
// closing text, written last, painted straight over it - the label was there
// in the bytes and unreadable on screen. Nothing caught that: the size was
// exact, the file parsed, and a renderer still drew a full page of shapes.
func textOpen(h int) string {
	return fmt.Sprintf(`<text x="16" y="%d" font-size="13" fill="#333333">`, h-34)
}

// labelOpen carries the identity label along the bottom edge, on the other
// baseline of the two.
func labelOpen(h int) string {
	return fmt.Sprintf(`<text x="16" y="%d" font-size="13" fill="#333333">`, h-16)
}

// labelFits says whether the band along the bottom has room to show the label.
//
// Below this the file is still produced and still named - it simply carries no
// visible label, and says so. That is the same answer the picture formats give
// for a drawing too small to write on, and reusing their sentence is the point:
// a second wording for one situation is a second thing to keep true.
func labelFits(h int) bool { return h > TextBand }

// drawHeightFor is the strip shapes are drawn in.
//
// At least one row, always. A drawing shorter than the text band has no room
// for the band, and the band is a courtesy to the label rather than a
// structural part of the document.
func drawHeightFor(h int) int {
	if d := h - TextBand; d > 0 {
		return d
	}
	return 1
}

// span keeps an argument to IntN positive.
//
// The shape code below assumes a margin of up to eighty units, which was safe
// for as long as the canvas was a constant 800 by 600. It is not safe now:
// measured 2026-09-08 by running it, IntN(0) and IntN(-30) both panic with
// "invalid argument to IntN", so "--set width=50" would have been a panic on a
// value that looks entirely legal. A generator panic costs one file rather
// than the process - there is a guard for that - but a panic is not an answer
// to a legal setting.
//
// At 800 by 600 every argument is already positive, so this changes no byte of
// any file this tool has produced.
func span(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// fit keeps a shape's lower edge out of the text band on any canvas.
//
// Arithmetic, not caution: at the default dimensions this can never bind. The
// widest rect starts at 719 and reaches 798 of 800, the tallest at 463 and
// reaches 542 of 544, and the largest circle and ellipse both reach 542 as
// well. Two units of slack on every axis, so the clamp is inert at 800 by 600
// and the stored hashes prove it.
//
// room is always at least one, so this never returns a shape of no size. Each
// caller subtracts a coordinate drawn from span(drawHeight-margin) from
// drawHeight: above the margin that leaves the margin itself, and at or below
// it span returns one, the coordinate is nought and the room is the whole
// drawing. There is no third case.
//
// That claim was a guarded branch here until it was measured. A panic put in
// its place did not fire once across the whole canvas sweep, so the branch was
// a defence nothing could turn red - the eighth of its kind removed from this
// codebase. It is written down instead, which is what a claim nothing can
// contradict is worth.
func fit(extent, room int) int { return min(extent, room) }

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
		//
		// It is the minimum at the DEFAULT dimensions, and only that. Larger
		// numbers in the root element and in a text baseline make a longer
		// document, so the floor moves with the settings - the same shape as
		// the JSON layouts, where the registry states the default layout's
		// minimum and each of the others answers for its own. Nobody has to
		// keep a second number in step: Plan refuses with the figure for the
		// dimensions actually asked for, and Descriptor.SmallestAccepted asks
		// Plan rather than reading anything declared here.
		MinBytes: minimumBytes(defaultWidth, defaultHeight),

		Padding: format.PaddingChannel{
			Name:     "the text of the closing label",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// A drawing shows what it is, so the label is a visible text element
		// rather than a comment.
		Label:  format.LabelVisible,
		Oracle: "inkscape",
		// Shape counts, gradients, fonts, embedded rasters and SMIL come
		// later. Declaring none of them makes a recipe asking for one fail
		// loudly rather than quietly.
		Properties: []format.Property{
			imagedim.Width(imagedim.Side{Largest: maxDimension,
				Default: strconv.Itoa(defaultWidth),
				Detail: "How wide the drawing says it is. Nothing is drawn into pixels here, " +
					"so a large number costs a few bytes in the file and a great deal of memory in whatever opens it."}),
			imagedim.Height(imagedim.Side{Largest: maxDimension,
				Default: strconv.Itoa(defaultHeight),
				Detail: "How tall the drawing says it is. The label sits along the bottom edge, " +
					"so a drawing shorter than that strip carries no visible label."}),
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	labelLine string // includes the trailing newline, empty when absent
	seed      uint64
	width     int
	height    int
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	w, err := imagedim.Value("svg", Width, r.Properties, maxDimension, defaultWidth)
	if err != nil {
		return format.Plan{}, err
	}
	h, err := imagedim.Value("svg", Height, r.Properties, maxDimension, defaultHeight)
	if err != nil {
		return format.Plan{}, err
	}

	min := minimumBytes(w, h)
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
			Width:         w,
			Height:        h,
			"view_box":    fmt.Sprintf("0 0 %d %d", w, h),
		},
	}

	m := memo{seed: r.Seed, width: w, height: h}
	if r.Label {
		switch line := labelOpen(h) + core.Label("svg", r.Bytes, r.Seed) + textClose; {
		case !labelFits(h):
			// Room on the page rather than room in the byte count, so it is a
			// separate sentence from the one below. Both leave the file named
			// by its own name and by the manifest.
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The drawing is %d px tall and the label needs the %d px strip along the bottom, so this file carries no visible label. Its name and the manifest still identify it.",
					h, TextBand),
			})
		case int64(len(line))+min <= r.Bytes:
			m.labelLine = line
		default:
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label needs %d B and this file has no room for it beside a whole shape. Its name and the manifest still identify it.",
					len(line)),
			})
		}
	}

	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("svg: the plan was not produced by this generator")
	}

	head := declaration + rootOpen(m.width, m.height) + m.labelLine
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), shapesFor(m.width, m.height))
}

// shapes builds the drawing. A natural record is one shape, and the closing
// record is a text label stretched to land the byte count.
//
// It carries the canvas rather than reading package constants, because the
// canvas is a setting now. Nothing else changed about what it draws.
type shapes struct {
	width      int
	drawHeight int
	// open is the closing record's opening tag, whose baseline depends on how
	// tall the drawing is.
	open string
}

func shapesFor(w, h int) shapes {
	return shapes{width: w, drawHeight: drawHeightFor(h), open: textOpen(h)}
}

// Shortest is the smallest closing record: the label element with no text at
// all, plus the bytes that close the root.
func (s shapes) Shortest() int64 { return int64(len(s.open) + len(tailLast)) }

func (s shapes) Append(dst []byte, rng *rand.Rand) []byte {
	// A line is drawn with a stroke and a closed shape is filled. Each branch
	// says which it wants, because a shape painted the wrong way is invisible
	// and the size never notices.
	paint := "fill"

	switch rng.IntN(4) {
	case 0:
		x := rng.IntN(span(s.width - 80))
		dst = append(dst, `<rect x="`...)
		dst = strconv.AppendInt(dst, int64(x), 10)
		y := rng.IntN(span(s.drawHeight - 80))
		dst = append(dst, `" y="`...)
		dst = strconv.AppendInt(dst, int64(y), 10)
		dst = append(dst, `" width="`...)
		dst = strconv.AppendInt(dst, int64(20+rng.IntN(60)), 10)
		dst = append(dst, `" height="`...)
		dst = strconv.AppendInt(dst, int64(fit(20+rng.IntN(60), s.drawHeight-y)), 10)
	case 1:
		dst = append(dst, `<circle cx="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.width))), 10)
		cy := rng.IntN(span(s.drawHeight - 45))
		dst = append(dst, `" cy="`...)
		dst = strconv.AppendInt(dst, int64(cy), 10)
		dst = append(dst, `" r="`...)
		dst = strconv.AppendInt(dst, int64(fit(5+rng.IntN(40), s.drawHeight-cy)), 10)
	case 2:
		dst = append(dst, `<ellipse cx="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.width))), 10)
		dst = append(dst, `" cy="`...)
		cy := rng.IntN(span(s.drawHeight - 60))
		dst = strconv.AppendInt(dst, int64(cy), 10)
		dst = append(dst, `" rx="`...)
		dst = strconv.AppendInt(dst, int64(10+rng.IntN(50)), 10)
		dst = append(dst, `" ry="`...)
		dst = strconv.AppendInt(dst, int64(fit(10+rng.IntN(50), s.drawHeight-cy)), 10)
	default:
		dst = append(dst, `<line x1="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.width))), 10)
		dst = append(dst, `" y1="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.drawHeight))), 10)
		dst = append(dst, `" x2="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.width))), 10)
		dst = append(dst, `" y2="`...)
		dst = strconv.AppendInt(dst, int64(rng.IntN(span(s.drawHeight))), 10)
		dst = append(dst, `" stroke-width="2`...)
		paint = "stroke"
	}

	dst = append(dst, `" `...)
	dst = append(dst, paint...)
	dst = append(dst, `="`...)
	dst = append(dst, colours[rng.IntN(len(colours))]...)
	return append(dst, `"/>`+"\n"...)
}

// Discard has nothing to put back. A shape carries no state from one to the
// next, so throwing one away leaves no trace to undo.
func (shapes) Discard() {}

func (s shapes) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	start := len(dst)
	dst = append(dst, s.open...)
	used := int64(len(dst)-start) + int64(len(tailLast))
	dst = appendFiller(dst, n-used)
	return append(dst, tailLast...)
}

// appendFiller writes exactly n bytes of label text out of readable words.
//
// It never emits an ampersand or an angle bracket, the characters that would
// have to be escaped - an escape would make the text longer than the count
// asked for.
// appendFiller stretches the closing label's text to the byte.
func appendFiller(dst []byte, n int64) []byte {
	return core.AppendFiller(dst, words, n, nil)
}

// minimumBytes is the declaration, the root element and one whole record,
// computed rather than written down so it cannot drift away from the template.
// A drawing has to draw something, and one byte is what that costs.
//
// The arithmetic below gives the smallest well formed document: the
// declaration, the root element and the shortest closing label. At exactly
// that size the label has nothing in it, so the file is one empty text element
// and no shapes - valid SVG, exactly the size ordered, repeatable, and a blank
// canvas.
//
// Measured on 2026-08-03 with tools/probes/fidelity-sweep.py, rendered by
// Inkscape and counted with Pillow:
//
//	193 B   0 shapes, 1 colour     nothing painted
//	194 B   0 shapes, 11 colours   the label draws
//	260 B   1 shape,  63 colours
//
// The same with the label off, because the padding channel is the label text
// either way.
//
// One byte more is therefore the smallest file this format can honestly
// produce, and refusing the byte below is the answer rather than filling it -
// putting something into the document at that size would move every byte of
// every SVG this tool has ever made, and D11 does not allow that for a size
// nobody can usefully order. Every file at 194 B and above is untouched.
//
// This was invisible because the reference tool only ever saw one size per
// format, MinBytes plus 300 KB. The renderer that catches exactly this failure
// existed and was never pointed at the bottom of the range.
func minimumBytes(w, h int) int64 {
	s := shapesFor(w, h)
	return int64(len(declaration)+len(rootOpen(w, h))) + s.Shortest() + 1
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
