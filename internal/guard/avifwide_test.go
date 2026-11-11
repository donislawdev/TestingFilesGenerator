package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gen2brain/gav1d/avif"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// An AVIF picture larger than one AV1 tile is coded as tiles, and it is read
// back whole - by the specification's tile_info, by gav1d's own decoder and
// by the format's reference tool.
//
// AV1 holds a tile to 4096 px across and to 4096x2304 px in area
// (06.bitstream.syntax.md lines 1176-1192), so a picture past either has to be
// coded as several. gav1d up to v0.2.5 wrote such a picture's tile_info as
// several tiles over the data of one (docs/REVIEW-165-2026-10-06.md), and
// this format takes pictures to 16384 px and 40 megapixels - so every AVIF
// wider than 4096 px or past that area was a file no decoder reads. Pillow,
// libaom and ImageMagick refused all four such sizes measured on 2026-11-11,
// and read them once gav1d v0.2.6 coded them in tiles
// (docs/GAV1D-026-2026-11-11.md). Films were never hit:
// they cut a picture into tiles themselves (internal/format/video, grid.go).
//
// The two pictures are the two ways tile_info cuts one: a width a few pixels
// past one column, and an area one superblock row past the limit at the full
// width of a tile, which is cut into rows. Each is held to its tiles read
// from the file first, so this guard knows it asked about a picture of tiles
// rather than assuming it. A missing reference tool skips that part loudly.
func TestAnAVIFLargerThanOneAV1TileSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("avif")
	if err != nil {
		t.Fatal(err)
	}
	checker, known := oracle.For(d.Oracle)
	if !known {
		t.Fatalf("avif declares the oracle %q and nothing implements it", d.Oracle)
	}
	for _, c := range []struct {
		width, height int
		cols, rows    int
	}{
		{width: 4100, height: 64, cols: 2, rows: 1},
		{width: 4096, height: 2305, cols: 1, rows: 2},
	} {
		name := strconv.Itoa(c.width) + "x" + strconv.Itoa(c.height)
		plan, err := d.Generator.Plan(format.Request{Bytes: 4 << 20, Seed: goldenSeed, Label: true,
			Properties: map[string]string{"width": strconv.Itoa(c.width), "height": strconv.Itoa(c.height)}})
		if err != nil {
			t.Fatalf("%s: planning failed: %v", name, err)
		}
		var buf bytes.Buffer
		if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
			t.Fatalf("%s: writing failed: %v", name, err)
		}
		b := buf.Bytes()

		mdat, err := mp4Child(b, "mdat")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		frame, _, err := frameOBU(mdat)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// A still picture's frame header has three bits ahead of tile_info:
		// disable_cdf_update, allow_screen_content_tools and
		// render_and_frame_size_different (internal/format/video, still.go).
		tiles, err := readFilmTiles(frame, 3, c.width, c.height)
		if err != nil {
			t.Errorf("%s: the picture's tiles do not read back from the file: %v", name, err)
			continue
		}
		if len(tiles.rects) != c.cols*c.rows {
			t.Fatalf("%s: %d tiles read back where tile_info derives %d columns and %d rows - the case no longer asks about a picture of tiles",
				name, len(tiles.rects), c.cols, c.rows)
		}

		m, err := avif.Decode(bytes.NewReader(b))
		if err != nil {
			t.Errorf("%s: gav1d's decoder refused it: %v", name, err)
			continue
		}
		if got := m.Bounds(); got.Dx() != c.width || got.Dy() != c.height {
			t.Errorf("%s: gav1d's decoder read a %dx%d picture", name, got.Dx(), got.Dy())
		}

		path := filepath.Join(t.TempDir(), name+".avif")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		res := checker.Check(path)
		switch {
		case !res.Available:
			t.Logf("SKIPPED %s: %s is not installed, so only the tiles and gav1d's decoder were asked", name, checker.Name)
		case res.Err != nil:
			t.Errorf("%s: %v", name, res.Err)
		default:
			t.Logf("%s: %d tiles, %s accepted it - %s", name, len(tiles.rects), checker.Name, firstLine(res.Output))
		}
	}
}
