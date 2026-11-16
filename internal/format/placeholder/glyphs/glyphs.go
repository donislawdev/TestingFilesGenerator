// Package glyphs is the typeface a placeholder picture is written in: the
// outlines of Inter as whole numbers in the units of the font, read from a
// file this package embeds.
//
// Why outlines as data and not a font read at run time. A picture is bytes
// somebody's suite holds a hash of (D11), so the pixels of a letter cannot be
// allowed to move when a library that reads fonts is updated - and that
// library would come in through the window toolkit, which updates it on its
// own schedule. The shapes are read out of internal/gui/font once, by
// glyphs/fromttf, and drawn by internal/format/placeholder/raster, which is
// whole number arithmetic only, so the same letter is the same pixels on every
// system and every version (docs/PLACEHOLDER-2026-11-14.md, sections 5 and 12).
//
// The file holds a subset: Latin with the letters of the European languages,
// Greek, Cyrillic and the punctuation a label or a product name uses. A text
// outside it is refused before anything is written, never drawn with a box in
// place of a letter.
package glyphs

import (
	_ "embed"
	"encoding/binary"
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// File is what the outline file holds, embedded so the command line and the
// window draw the same letters.
//
//go:embed inter.outlines
var File []byte

// Op is what one step of an outline does.
type Op uint8

const (
	// MoveTo starts a contour at one point.
	MoveTo Op = iota
	// LineTo draws a straight line to one point.
	LineTo
	// QuadTo draws a quadratic curve through a control point to an end point.
	QuadTo
)

// Point is a place in the units of the font, with the baseline at y zero and
// y growing downwards, the way pixel rows count.
type Point struct{ X, Y int32 }

// Glyph is one letter: how far the pen moves after it, and its outline.
type Glyph struct {
	Advance int32
	Ops     []Op
	// Points holds one point for a MoveTo and a LineTo and two for a QuadTo,
	// control point first, in the order of Ops.
	Points []Point
}

// Face is the typeface as a whole: its measures and its letters.
type Face struct {
	// UnitsPerEm is how many units of the font make the size of the type.
	UnitsPerEm int32
	// Ascent and Descent are how far the tallest and the deepest letters
	// reach above and below the baseline, both positive.
	Ascent, Descent int32
	// CapHeight is how tall a capital letter is.
	CapHeight int32

	glyphs map[rune]Glyph
}

// Glyph is the letter r, and false when the face does not carry it.
func (f *Face) Glyph(r rune) (Glyph, bool) {
	g, ok := f.glyphs[r]
	return g, ok
}

// Missing is every letter of s the face does not carry, each once, in the
// order they first appear.
func (f *Face) Missing(s string) []rune {
	var out []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if _, ok := f.glyphs[r]; !ok && !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// Count is how many letters the face carries.
func (f *Face) Count() int { return len(f.glyphs) }

// Advance is how wide s is set in the units of the font.
//
// Without kerning. Inter adjusts pairs such as A and V in its GPOS table, and
// golang.org/x/image v0.46.0, which reads the font when the outline file is
// made, finds none of them there - measured on 2026-11-14, "not found" for
// AV, To, Ty, LT and Wa (tools/probes/placeholder/kern). The pictures drawn
// without it were looked at the same day and read well, so the file carries
// no pairs rather than a section nothing could fill.
func (f *Face) Advance(s string) int64 {
	var w int64
	for _, r := range s {
		w += int64(f.glyphs[r].Advance)
	}
	return w
}

// inter is the face the embedded file holds, read when the program starts.
//
// At start rather than at first use, so there is nothing to share between the
// writers that draw at once - reading 97 KB of outlines costs well under a
// millisecond, and a lazily read face would be the one piece of state the
// pictures of a run share. A file that does not read is a defect of this build
// rather than something a person brought about -
// TestThePlaceholderTypefaceIsInter holds it to the font it came from - so a
// build that cannot read its own letters stops at start, as one that cannot
// state the minimum of a format does (format.Register).
var inter = mustDecode(File)

func mustDecode(b []byte) *Face {
	f, err := Decode(b)
	if err != nil {
		panic(fmt.Sprintf("glyphs: the embedded outlines do not read: %v", err))
	}
	return f
}

// Inter is the face the embedded file holds.
func Inter() *Face { return inter }

// Magic opens every outline file, and Version is the layout this package
// reads.
const (
	Magic   = "tfgglyph"
	Version = 1
)

// Decode reads an outline file. The layout, little endian:
//
//	"tfgglyph" u8 version
//	u16 units per em, i16 ascent, i16 descent, i16 cap height
//	u16 glyph count, then per glyph, by rune:
//	  u32 rune, u16 advance, u16 step count, then per step
//	  u8 op and one i16 x, y pair, two for a curve
//
// Every refusal is a defect: the only file this ever reads is the one the
// build embeds.
func Decode(b []byte) (*Face, error) {
	r := reader{b: b}
	if string(r.take(len(Magic))) != Magic || r.u8() != Version {
		return nil, core.Defect(fmt.Errorf("the file is not version %d of %s", Version, Magic))
	}
	f := &Face{
		UnitsPerEm: int32(r.u16()),
		Ascent:     int32(r.i16()),
		Descent:    int32(r.i16()),
		CapHeight:  int32(r.i16()),
		glyphs:     map[rune]Glyph{},
	}
	n := int(r.u16())
	for i := 0; i < n && r.err == nil; i++ {
		ch, g, err := r.glyph()
		if err != nil {
			return nil, err
		}
		if _, dup := f.glyphs[ch]; dup {
			return nil, core.Defect(fmt.Errorf("letter %U is in the file twice", ch))
		}
		f.glyphs[ch] = g
	}
	switch {
	case r.err != nil:
		return nil, r.err
	case len(r.b) != r.at:
		return nil, core.Defect(fmt.Errorf("%d bytes follow the last record", len(r.b)-r.at))
	case f.UnitsPerEm <= 0:
		return nil, core.Defect(fmt.Errorf("the file says a size of type is %d units", f.UnitsPerEm))
	}
	return f, nil
}

// glyph reads one letter's record.
func (r *reader) glyph() (rune, Glyph, error) {
	ch := rune(r.u32())
	g := Glyph{Advance: int32(r.u16())}
	steps := int(r.u16())
	for s := 0; s < steps && r.err == nil; s++ {
		op := Op(r.u8())
		points, ok := pointsOf[op]
		if !ok {
			return 0, Glyph{}, core.Defect(fmt.Errorf("letter %U has a step %d this build does not know", ch, op))
		}
		g.Ops = append(g.Ops, op)
		for k := 0; k < points; k++ {
			g.Points = append(g.Points, Point{X: int32(r.i16()), Y: int32(r.i16())})
		}
	}
	return ch, g, nil
}

// pointsOf is how many points each step carries.
var pointsOf = map[Op]int{MoveTo: 1, LineTo: 1, QuadTo: 2}

type reader struct {
	b   []byte
	at  int
	err error
}

func (r *reader) take(n int) []byte {
	if r.err != nil || r.at+n > len(r.b) {
		r.err = core.Defect(fmt.Errorf("the file ends in the middle of a record, at byte %d", r.at))
		return make([]byte, n)
	}
	out := r.b[r.at : r.at+n]
	r.at += n
	return out
}

func (r *reader) u8() uint8   { return r.take(1)[0] }
func (r *reader) u16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *reader) i16() int16  { return int16(r.u16()) }
func (r *reader) u32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
