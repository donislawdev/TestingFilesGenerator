package guard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs/fromttf"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/raster"
)

// placeholderFont is the font the placeholder outlines are read from. Bold,
// chosen on 2026-11-14 after both weights were drawn side by side: the label
// line at ten pixels to the em reads in Bold and smudges in Regular
// (docs/PLACEHOLDER-2026-11-14.md section 13).
const placeholderFont = "internal/gui/font/Inter-Bold.ttf"

// TestThePlaceholderTypefaceIsInter holds the outline file the build embeds
// to the font it says it is: read the font again, and the bytes must be the
// same. A file edited by hand, or left behind when the font moves to a new
// version, is red here before it draws a letter nobody chose.
//
// TFG_WRITE_PLACEHOLDER_OUTLINES=1 writes the file from the font instead.
func TestThePlaceholderTypefaceIsInter(t *testing.T) {
	root := repoRoot(t)
	ttf, err := os.ReadFile(filepath.Join(root, placeholderFont))
	if err != nil {
		t.Fatalf("reading %s: %v", placeholderFont, err)
	}
	want, err := fromttf.Encode(ttf, fromttf.Subset)
	if err != nil {
		t.Fatalf("reading the outlines out of %s: %v", placeholderFont, err)
	}
	target := filepath.Join(root, "internal", "format", "placeholder", "glyphs", "inter.outlines")
	if os.Getenv("TFG_WRITE_PLACEHOLDER_OUTLINES") == "1" {
		if err := os.WriteFile(target, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d B to %s", len(want), target)
		return
	}
	if !bytes.Equal(glyphs.File, want) {
		t.Fatalf("the embedded outlines (%d B) are not what %s gives (%d B). Write them again with\n"+
			"  TFG_WRITE_PLACEHOLDER_OUTLINES=1 go test ./internal/guard/ -run ^TestThePlaceholderTypefaceIsInter$\n"+
			"and know that every placeholder picture changes its bytes with them (D11).",
			len(glyphs.File), placeholderFont, len(want))
	}

	f := glyphs.Inter()
	// The letters a placeholder is most often asked to carry, each of which
	// a smaller subset once left out in the analysis.
	for _, r := range "0123456789×AaŻżĄąŁłЖжΩω€№—„”" {
		if _, ok := f.Glyph(r); !ok {
			t.Errorf("the placeholder typeface does not carry %q (%U)", r, r)
		}
	}
	// Canary: a letter the subset leaves out must be reported missing, or a
	// text in a script the face cannot draw would pass the check that is meant
	// to refuse it.
	if m := f.Missing("a中b"); len(m) != 1 || m[0] != '中' {
		t.Errorf("Missing(%q) = %q, want exactly the one Chinese letter", "a中b", string(m))
	}
}

// TestARasterCoversWhatItsOutlineEncloses fills a square that starts and ends
// halfway through a pixel and asks every pixel: a corner is a quarter
// covered, an edge half, the middle whole, and nothing outside is touched. It
// also asks for the same square twice in opposite directions of travel - the
// non-zero rule must count both as inside.
func TestARasterCoversWhatItsOutlineEncloses(t *testing.T) {
	half := int64(raster.One / 2)
	square := func(reverse bool) *raster.Path {
		p := raster.NewPath()
		x0, y0, x1, y1 := 1*raster.One+half, 1*raster.One+half, 4*raster.One+half, 4*raster.One+half
		pts := [][2]int64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
		if reverse {
			pts = [][2]int64{{x0, y0}, {x0, y1}, {x1, y1}, {x1, y0}}
		}
		p.MoveTo(pts[0][0], pts[0][1])
		for _, q := range pts[1:] {
			p.LineTo(q[0], q[1])
		}
		p.Close()
		return p
	}
	want := [6][6]uint8{}
	for y := 1; y <= 4; y++ {
		for x := 1; x <= 4; x++ {
			v := 255
			if x == 1 || x == 4 {
				v /= 2
			}
			if y == 1 || y == 4 {
				v /= 2
			}
			want[y][x] = uint8(v)
		}
	}
	got := func(p *raster.Path) [6][6]uint8 {
		var g [6][6]uint8
		raster.Fill(p, 6, 6, func(x, y int, a uint8) { g[y][x] = a })
		return g
	}
	// Within one step of 255: the corner is a quarter of 255, which has no
	// whole value, and the rasteriser rounds where this table truncates.
	close := func(a, b [6][6]uint8) bool {
		for y := range a {
			for x := range a[y] {
				d := int(a[y][x]) - int(b[y][x])
				if d < -1 || d > 1 {
					return false
				}
			}
		}
		return true
	}
	for _, reverse := range []bool{false, true} {
		if g := got(square(reverse)); !close(g, want) {
			t.Errorf("reverse=%t: coverage\n%v\nwant\n%v", reverse, g, want)
		}
	}

	// One square cannot tell the non-zero rule from even-odd - both fill it.
	// Two contours travelling the same way, one inside the other, can: the
	// middle is wound twice, which non-zero fills and even-odd leaves as a
	// hole. The letters need non-zero wherever a contour of one glyph overlaps
	// another of the same glyph.
	nested := raster.NewPath()
	for _, side := range [][2]int64{{0, 6}, {2, 4}} {
		lo, hi := side[0]*raster.One, side[1]*raster.One
		nested.MoveTo(lo, lo)
		nested.LineTo(hi, lo)
		nested.LineTo(hi, hi)
		nested.LineTo(lo, hi)
		nested.Close()
	}
	if g := got(nested); g[2][2] != 255 || g[3][3] != 255 || g[0][0] != 255 {
		t.Errorf("two contours wound the same way, one inside the other: the middle is %d and %d and the outer ring %d, "+
			"all should be 255 under the non-zero rule\n%v", g[2][2], g[3][3], g[0][0], g)
	}
	// Canary: a square shifted by a pixel must not pass the same comparison.
	shifted := raster.NewPath()
	shifted.MoveTo(2*raster.One+half, 1*raster.One+half)
	shifted.LineTo(5*raster.One+half, 1*raster.One+half)
	shifted.LineTo(5*raster.One+half, 4*raster.One+half)
	shifted.LineTo(2*raster.One+half, 4*raster.One+half)
	if close(got(shifted), want) {
		t.Fatal("canary: a square a pixel to the right passed as the square, so the comparison cannot see a misplaced fill")
	}
}

// TestAFlattenedCurveStaysWithinASixteenthOfAPixel holds the number of lines
// QuadTo draws a curve as to the promise beside it: no point of the curve is
// further than a sixteenth of a pixel from the lines. Asked of the exact
// curve and the exact lines in floating point, so this is the bound itself
// and not the rounding of the points.
//
// The curves are a flat one, the one an outside review gave - 0,0 through
// 256,240 to 512,0, which the count before 2026-11-16 drew as two lines
// 30/256 px away from it - and steeper ones up to where the cap of 256 lines
// takes over. The canary draws the review's curve with one line fewer than
// the count, which has to stray too far, or this cannot see a count that is
// short.
func TestAFlattenedCurveStaysWithinASixteenthOfAPixel(t *testing.T) {
	const tol = float64(raster.One) / 16
	stray := func(c [6]int64, n int64) float64 {
		at := func(s float64) (float64, float64) {
			u := 1 - s
			return u*u*float64(c[0]) + 2*s*u*float64(c[2]) + s*s*float64(c[4]),
				u*u*float64(c[1]) + 2*s*u*float64(c[3]) + s*s*float64(c[5])
		}
		worst := 0.0
		for i := int64(0); i < n; i++ {
			ax, ay := at(float64(i) / float64(n))
			bx, by := at(float64(i+1) / float64(n))
			for k := 1; k < 64; k++ {
				px, py := at((float64(i) + float64(k)/64) / float64(n))
				worst = max(worst, toSegment(px, py, ax, ay, bx, by))
			}
		}
		return worst
	}
	review := [6]int64{0, 0, 256, 240, 512, 0}
	curves := [][6]int64{
		{0, 0, 256, 0, 512, 0},
		review,
		{0, 0, 300, 4000, 600, 0},
		{0, 0, 5000, 90000, 10000, 0},
		{0, 0, 2000, -6000, 8000, -8000},
		{0, 0, 40000, -900000, 3000, 70000},
	}
	for _, c := range curves {
		n := raster.QuadLines(c[0], c[1], c[2], c[3], c[4], c[5])
		if n == 256 {
			continue
		}
		if d := stray(c, n); d > tol {
			t.Errorf("curve %v as %d lines strays %.2f/256 px, past the promised 16/256", c, n, d)
		}
	}
	n := raster.QuadLines(review[0], review[1], review[2], review[3], review[4], review[5])
	if n < 2 || stray(review, n-1) <= tol {
		t.Fatalf("canary: the review's curve as %d lines stays within a sixteenth of a pixel, so this cannot see a short count", n-1)
	}
}

// toSegment is how far px, py is from the segment from a to b.
func toSegment(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	s := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		s = min(max(((px-ax)*dx+(py-ay)*dy)/l, 0), 1)
	}
	return math.Hypot(px-ax-s*dx, py-ay-s*dy)
}

// placeholderGolden is the hash of one placeholder drawn with every part it
// has: Polish and Cyrillic letters, the label line in the shape a picture
// draws it, and the guides. Taken on Windows amd64 on 2026-11-16, with curves
// cut into enough lines to keep the promise of a sixteenth of a pixel. CI draws
// it on Linux and on macOS arm64, and the same hash there is what says the
// arithmetic is the same everywhere (D11, docs/PLACEHOLDER-2026-11-14.md
// section 5).
const placeholderGolden = "9c6fd0062fec85d3832de4c9f4da60f0e7f4e2741ee6b92027b6b35c6fc52ad8"

// TestAPlaceholderIsTheSamePixelsOnEverySystem draws one placeholder and
// holds its pixels to a hash.
func TestAPlaceholderIsTheSamePixelsOnEverySystem(t *testing.T) {
	draw := func(text string) string {
		img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
		d := placeholder.Draw(img, placeholder.Look{
			Text:       text,
			Label:      "tfg - png - seed 8dc2d18c",
			Background: color.RGBA{0xcc, 0xcc, 0xcc, 255},
			Foreground: color.RGBA{0x55, 0x55, 0x55, 255},
			Guides:     true,
		})
		if !d.Text || !d.Label {
			t.Fatalf("the reference picture lost a part: %+v", d)
		}
		sum := sha256.Sum256(img.Pix)
		return hex.EncodeToString(sum[:])
	}
	text := "Zdjęcie Жж 17"
	got := draw(text)
	if got != placeholderGolden {
		t.Errorf("the reference placeholder hashes to %s, pinned %s.\n"+
			"On one system only: the arithmetic differs between systems - find the operation, do not re-pin.\n"+
			"Everywhere: the drawing changed, and every placeholder anybody holds a hash of changed with it (D11).",
			got, placeholderGolden)
	}
	// Canary: one letter different must not hash the same, or the hash is of
	// something other than the pixels.
	if draw(strings.Replace(text, "17", "18", 1)) == got {
		t.Fatal("canary: a different text drew the same pixels")
	}
}
