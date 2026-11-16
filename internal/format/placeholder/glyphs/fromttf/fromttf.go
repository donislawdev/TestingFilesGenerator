// Package fromttf turns a TrueType font into the outline file package glyphs
// reads. It runs when the file is made and when a guard checks the file still
// is the font - it is never part of a binary, which is why it may parse a font
// with golang.org/x/image while the binaries do not.
package fromttf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs"
)

// Range is a run of letters, both ends included.
type Range struct{ First, Last rune }

// Subset is what the placeholder typeface carries: printable ASCII, the
// Latin-1 and Latin Extended-A letters (Polish, Czech, Turkish and the rest
// of Europe), Greek, the Cyrillic of Russian, Ukrainian, Belarusian, Serbian
// and Bulgarian, and the punctuation a product name or a caption uses. A
// letter of a range the font does not carry is left out rather than drawn as
// an empty box, and a text asking for it is refused.
var Subset = []Range{
	{0x0020, 0x007E},
	{0x00A0, 0x00FF},
	{0x0100, 0x017F},
	{0x0370, 0x03FF},
	{0x0400, 0x045F},
	{0x2010, 0x2027},
	{0x2030, 0x2030},
	{0x2039, 0x203A},
	{0x20AC, 0x20AC},
	{0x2116, 0x2116},
	{0x2122, 0x2122},
}

// Encode is the outline file of the letters of ranges, read from ttf.
//
// Every number is read at a size of one pixel per unit of the font, where
// the library's arithmetic is exact, and refused if it is not a whole number
// of units - the file says what the font says, not a rounding of it.
func Encode(ttf []byte, ranges []Range) ([]byte, error) {
	f, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, err
	}
	e := &encoder{f: f, ppem: fixed.I(int(f.UnitsPerEm()))}
	m, err := f.Metrics(&e.buf, e.ppem, font.HintingNone)
	if err != nil {
		return nil, err
	}
	e.out.WriteString(glyphs.Magic)
	e.out.WriteByte(glyphs.Version)
	e.put(uint16(f.UnitsPerEm()))
	e.put(e.units(m.Ascent, "the ascent"))
	e.put(e.units(m.Descent, "the descent"))
	e.put(e.units(m.CapHeight, "the cap height"))

	letters, err := e.letters(ranges)
	if err != nil {
		return nil, err
	}
	e.put(uint16(len(letters)))
	for _, l := range letters {
		e.letter(l)
	}
	if e.err != nil {
		return nil, e.err
	}
	return e.out.Bytes(), nil
}

// encoder is one Encode in progress. The first refusal sticks in err and
// everything after it is skipped, so each step reads as what it writes.
type encoder struct {
	f    *sfnt.Font
	buf  sfnt.Buffer
	ppem fixed.Int26_6
	out  bytes.Buffer
	err  error
}

type found struct {
	r rune
	x sfnt.GlyphIndex
}

func (e *encoder) put(v any) {
	if e.err == nil {
		_ = binary.Write(&e.out, binary.LittleEndian, v)
	}
}

// units is v in units of the font, which it has to be exactly.
func (e *encoder) units(v fixed.Int26_6, what string) int16 {
	u := int64(v) / 64
	switch {
	case e.err != nil:
	case v%64 != 0:
		e.err = core.Defect(fmt.Errorf("%s is %d/64 of a unit, not a whole number", what, v))
	case u < -32768 || u > 32767:
		e.err = core.Defect(fmt.Errorf("%s is %d units, past what the file holds", what, u))
	}
	return int16(u)
}

// letters is every letter of ranges the font carries, in order.
func (e *encoder) letters(ranges []Range) ([]found, error) {
	var out []found
	for _, r := range runes(ranges) {
		x, err := e.f.GlyphIndex(&e.buf, r)
		if err != nil {
			return nil, err
		}
		if x != 0 {
			out = append(out, found{r, x})
		}
	}
	if len(out) > 0xFFFF {
		return nil, core.Defect(errors.New("more letters than the file counts"))
	}
	return out, nil
}

// runes is every letter of ranges, in order.
func runes(ranges []Range) []rune {
	var out []rune
	for _, rg := range ranges {
		for r := rg.First; r <= rg.Last; r++ {
			out = append(out, r)
		}
	}
	return out
}

// letter writes the record of one letter: its advance and its outline.
func (e *encoder) letter(l found) {
	adv, err := e.f.GlyphAdvance(&e.buf, l.x, e.ppem, font.HintingNone)
	if err == nil {
		var segs sfnt.Segments
		segs, err = e.f.LoadGlyph(&e.buf, l.x, e.ppem, nil)
		e.put(uint32(l.r))
		e.put(uint16(e.units(adv, fmt.Sprintf("the advance of %U", l.r))))
		e.put(uint16(len(segs)))
		for _, s := range segs {
			e.segment(l.r, s)
		}
	}
	if err != nil && e.err == nil {
		e.err = err
	}
}

// ops is what each kind of step of the font is in the file, and how many
// points it carries. A cubic curve is missing because TrueType has none.
var ops = map[sfnt.SegmentOp]struct {
	op     glyphs.Op
	points int
}{
	sfnt.SegmentOpMoveTo: {glyphs.MoveTo, 1},
	sfnt.SegmentOpLineTo: {glyphs.LineTo, 1},
	sfnt.SegmentOpQuadTo: {glyphs.QuadTo, 2},
}

func (e *encoder) segment(r rune, s sfnt.Segment) {
	o, ok := ops[s.Op]
	if !ok {
		if e.err == nil {
			e.err = core.Defect(fmt.Errorf("%U has a cubic curve, and a TrueType font has none", r))
		}
		return
	}
	e.put(byte(o.op))
	for p := 0; p < o.points; p++ {
		e.put(e.units(s.Args[p].X, fmt.Sprintf("a point of %U", r)))
		e.put(e.units(s.Args[p].Y, fmt.Sprintf("a point of %U", r)))
	}
}
