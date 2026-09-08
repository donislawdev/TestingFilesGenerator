package guard

// The two dimensions of an SVG drawing, and the two things that go wrong when a
// constant becomes a setting.
//
// Both were found by analysis before the code was written, not by a report:
//
//   - the shape code assumes a margin of up to eighty units, which was safe
//     while the canvas was a constant 800 by 600. Measured 2026-09-08 by
//     running it, rand.IntN(0) and rand.IntN(-30) panic with "invalid argument
//     to IntN" - so "--set width=50" would have been a panic on a value that
//     looks entirely legal;
//   - a shape whose lower edge runs past the drawing is a shape painted over
//     the label. That already happened once, before the strip along the bottom
//     existed: about one shape in ten landed on the label, measured at 11.0%
//     and 10.3%. The strip fixed it for one canvas size. It has to hold for
//     every canvas size now.
//
// Neither shows up at 800 by 600, which is the point of this file: at the
// default dimensions the clamps are inert, and the stored byte hashes prove
// that. These cases press the sizes where they are not.

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/svgfile"
)

// drawSVG produces one drawing and insists on the ordered size before anything
// else looks at it. A drawing of the wrong length is not evidence about shapes.
func drawSVG(t *testing.T, size int64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get("svg")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B at %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B at %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: ordered %d B and produced %d - the size is exact or it is an error",
			props, size, buf.Len())
	}
	return buf.Bytes()
}

func svgCanvas(w, h int) map[string]string {
	return map[string]string{svgfile.Width: strconv.Itoa(w), svgfile.Height: strconv.Itoa(h)}
}

// svgCanvases are the sizes worth pressing, and each one is here for a reason
// rather than to make a long list.
//
// 1 is the smallest the registry accepts. 80 and 81 sit either side of the
// margin the rect branch assumes, 56 and 57 either side of the strip the label
// needs, and 136 and 137 either side of the height at which the rect branch
// stopped panicking before the clamps went in. 800 by 600 is the default and is
// the control: everything here has to hold there too.
var svgCanvases = [][2]int{
	{1, 1}, {1, 600}, {800, 1},
	{2, 2}, {40, 40}, {80, 80}, {81, 81},
	{400, 56}, {400, 57}, {400, 100},
	{50, 136}, {50, 137}, {100, 200},
	{800, 600}, {20000, 20000}, {20000, 1}, {1, 20000},
}

// TestASvgCanvasSmallerThanItsOwnMargins is the claim: every canvas the
// registry accepts produces a drawing, at the exact size ordered, and one that
// an XML reader will take.
//
// A panic here is a red test rather than a crash for the run - the engine wraps
// a generator - but a panic is not an answer to a legal setting.
func TestASvgCanvasSmallerThanItsOwnMarginsStillDraws(t *testing.T) {
	for _, c := range svgCanvases {
		w, h := c[0], c[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			doc := drawSVG(t, 8192, svgCanvas(w, h))

			dec := xml.NewDecoder(bytes.NewReader(doc))
			for {
				_, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("%dx%d: the drawing is not well formed XML: %v", w, h, err)
				}
			}

			want := fmt.Sprintf(`width="%d" height="%d" viewBox="0 0 %d %d"`, w, h, w, h)
			if !bytes.Contains(doc, []byte(want)) {
				t.Errorf("%dx%d: the root element does not say %s", w, h, want)
			}
		})
	}
}

var (
	rectRe    = regexp.MustCompile(`<rect x="\d+" y="(\d+)" width="\d+" height="(\d+)"`)
	circleRe  = regexp.MustCompile(`<circle cx="\d+" cy="(\d+)" r="(\d+)"`)
	ellipseRe = regexp.MustCompile(`<ellipse cx="\d+" cy="(\d+)" rx="\d+" ry="(\d+)"`)
	lineRe    = regexp.MustCompile(`<line x1="\d+" y1="(\d+)" x2="\d+" y2="(\d+)"`)
)

// lowest reports the furthest down any shape reaches, and how many shapes were
// read.
//
// The count is returned because a drawing with no shapes in it satisfies "no
// shape reaches too far" without saying anything - two empty lists are equal.
func lowestSVGEdge(doc []byte) (edge, shapes int) {
	note := func(v int) {
		if v > edge {
			edge = v
		}
		shapes++
	}
	num := func(b []byte) int {
		n, _ := strconv.Atoi(string(b))
		return n
	}
	for _, m := range rectRe.FindAllSubmatch(doc, -1) {
		note(num(m[1]) + num(m[2]))
	}
	for _, m := range circleRe.FindAllSubmatch(doc, -1) {
		note(num(m[1]) + num(m[2]))
	}
	for _, m := range ellipseRe.FindAllSubmatch(doc, -1) {
		note(num(m[1]) + num(m[2]))
	}
	for _, m := range lineRe.FindAllSubmatch(doc, -1) {
		y1, y2 := num(m[1]), num(m[2])
		if y2 > y1 {
			y1 = y2
		}
		note(y1)
	}
	return edge, shapes
}

// TestNoSvgShapeReachesIntoTheStripTheLabelSitsIn presses the clamp.
//
// The strip is asked of the format rather than written down here, so the two
// cannot drift: a wider strip would move this bound with it.
func TestNoSvgShapeReachesIntoTheStripTheLabelSitsIn(t *testing.T) {
	for _, c := range svgCanvases {
		w, h := c[0], c[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			doc := drawSVG(t, 8192, svgCanvas(w, h))
			edge, shapes := lowestSVGEdge(doc)
			if shapes == 0 {
				t.Fatalf("%dx%d: read no shapes at all - this case proves nothing, "+
					"and two empty lists are equal", w, h)
			}
			room := h - svgfile.TextBand
			if room < 1 {
				room = 1
			}
			if edge > room {
				t.Errorf("%dx%d: a shape reaches %d and the drawing is %d deep before the "+
					"%d px strip the label sits in - a label with a circle through it is not a label",
					w, h, edge, room, svgfile.TextBand)
			}
		})
	}
}

// TestSayingTheSvgCanvasOutLoudChangesNoByte is the promise a new setting makes
// to every recipe written before it existed.
//
// Silence and stating the default are two different journeys through the
// parser, the registry and the generator, and they are allowed to disagree by
// accident. This is the one place that says they must not.
func TestSayingTheSvgCanvasOutLoudChangesNoByte(t *testing.T) {
	d, err := format.Get("svg")
	if err != nil {
		t.Fatal(err)
	}

	// The default is read from the declaration rather than written here. A
	// number copied into a test is a number that stops being the default
	// without anything saying so.
	spoken := map[string]string{}
	for _, p := range d.Properties {
		if p.Name != svgfile.Width && p.Name != svgfile.Height {
			continue
		}
		if p.Default == "" {
			t.Fatalf("%s declares no default, so silence has nothing to be equal to", p.Name)
		}
		spoken[p.Name] = p.Default
	}
	if len(spoken) != 2 {
		t.Fatalf("expected width and height to be declared, found %d of them", len(spoken))
	}

	for _, size := range []int64{194, 1024, 20480} {
		silent := drawSVG(t, size, nil)
		aloud := drawSVG(t, size, spoken)
		if !bytes.Equal(silent, aloud) {
			t.Errorf("%d B: saying %v out loud produced different bytes from leaving it out",
				size, spoken)
		}
	}
}

// TestASvgTooShortForTheLabelSaysSo is the untouchable rule about silence,
// applied to the one thing a drawing can lose by being small.
func TestASvgTooShortForTheLabelSaysSo(t *testing.T) {
	d, err := format.Get("svg")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		height int
		labels bool
	}{{1, false}, {40, false}, {svgfile.TextBand, false}, {svgfile.TextBand + 1, true}, {600, true}} {
		p, err := d.Generator.Plan(format.Request{
			Bytes: 8192, Seed: 7741, Label: true, Properties: svgCanvas(400, c.height),
		})
		if err != nil {
			t.Fatalf("planning at height %d: %v", c.height, err)
		}
		got, _ := p.Properties[format.PropertyLabelEmbedded].(bool)
		if got != c.labels {
			t.Errorf("height %d: label embedded is %v, expected %v", c.height, got, c.labels)
		}
		said := false
		for _, n := range p.Notes {
			if n.Code == "label_omitted" {
				said = true
			}
		}
		if !c.labels && !said {
			t.Errorf("height %d: no visible label and the run says nothing about it", c.height)
		}
		if c.labels && said {
			t.Errorf("height %d: the label is there and the run apologises for it anyway", c.height)
		}
	}
}
