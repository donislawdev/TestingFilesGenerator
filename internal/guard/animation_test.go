package guard

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// An animated format that writes one frame answers none of the question a
// tester asks of it.
//
// The complaint this guards against was reported from use rather than found
// here: a GIF went into an upload form, came back out as a still picture, and
// nothing about that told anybody whether the site keeps animations, flattens
// them to the first frame, or re-encodes them. A file that cannot tell those
// three apart is not a test file for this format.
//
// The pinned hashes in generatorbytes_test.go already stop the bytes moving.
// They say nothing about what the bytes ARE - a generator that quietly went
// back to writing one frame would drift, get a new hash written down, and pass
// for ever after. This reads the file back and asks whether the animation is
// in it.
//
// Read rather than asked: the structure below is parsed out of the bytes the
// engine wrote, not taken from the plan. Asking the generator to confirm its
// own intention is the shape of guard this project has been caught by before.

// gifStructure is what a reader finds when it walks a GIF data stream.
type gifStructure struct {
	globalTable int
	frames      int
	localTables int
	comments    int
	controls    int
	loopBlocks  int
	// lefts is the horizontal offset of each image descriptor, in order. A
	// marker that travels puts a different number in each.
	lefts []int
}

// walkGIF parses the block structure. It is deliberately strict: anything it
// does not recognise is an error rather than a block to skip, because a guard
// that shrugs at an unknown byte cannot tell a malformed file from a valid one.
func walkGIF(b []byte) (gifStructure, error) {
	var s gifStructure
	if len(b) < 13 {
		return s, fmt.Errorf("the file is %d B and the screen descriptor alone needs 13 B", len(b))
	}
	if string(b[:3]) != "GIF" {
		return s, fmt.Errorf("the file does not start with a GIF signature")
	}
	packed := b[10]
	pos := 13
	if packed&0x80 != 0 {
		s.globalTable = 1 << ((packed & 0x07) + 1)
		pos += 3 * s.globalTable
	}

	blocks := func(p int) (int, error) {
		for {
			if p >= len(b) {
				return 0, fmt.Errorf("a sub block chain runs past the end of the file")
			}
			n := int(b[p])
			p++
			if n == 0 {
				return p, nil
			}
			p += n
		}
	}

	for {
		if pos >= len(b) {
			return s, fmt.Errorf("the file ends with no trailer")
		}
		marker := b[pos]
		pos++
		switch marker {
		case 0x3B:
			if pos != len(b) {
				return s, fmt.Errorf("the trailer is at %d and %d B follow it", pos-1, len(b)-pos)
			}
			return s, nil
		case 0x21:
			if pos >= len(b) {
				return s, fmt.Errorf("an extension has no label")
			}
			switch b[pos] {
			case 0xFE:
				s.comments++
			case 0xF9:
				s.controls++
			case 0xFF:
				s.loopBlocks++
			}
			pos++
			next, err := blocks(pos)
			if err != nil {
				return s, err
			}
			pos = next
		case 0x2C:
			if pos+9 > len(b) {
				return s, fmt.Errorf("an image descriptor runs past the end of the file")
			}
			s.frames++
			s.lefts = append(s.lefts, int(binary.LittleEndian.Uint16(b[pos:pos+2])))
			local := b[pos+8]
			pos += 9
			if local&0x80 != 0 {
				s.localTables++
				pos += 3 * (1 << ((local & 0x07) + 1))
			}
			if pos >= len(b) {
				return s, fmt.Errorf("an image has no code size")
			}
			pos++
			next, err := blocks(pos)
			if err != nil {
				return s, err
			}
			pos = next
		default:
			return s, fmt.Errorf("the byte %#02x at offset %d is not a block marker", marker, pos-1)
		}
	}
}

func TestAGeneratedGifReallyCarriesAnAnimation(t *testing.T) {
	target := engine.Target{
		ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
		Properties: map[string]string{"width": "64", "height": "64"},
	}
	b := generateOne(t, target)

	s, err := walkGIF(b)
	if err != nil {
		t.Fatalf("reading back the generated GIF: %v", err)
	}

	d, err := format.Get("gif")
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, p := range d.Properties {
		if p.Name == "frames" {
			if _, e := fmt.Sscanf(p.Default, "%d", &want); e != nil {
				t.Fatalf("the frames setting declares %q as its default and that is not a number", p.Default)
			}
		}
	}
	if want < 2 {
		t.Fatalf("the frames setting defaults to %d, so a plain run writes a still picture and "+
			"a tester learns nothing about how the system under test treats an animation", want)
	}

	if s.frames != want {
		t.Errorf("a plain GIF holds %d frame(s) and the format declares %d as its default",
			s.frames, want)
	}
	if s.controls != want {
		t.Errorf("%d frame(s) carry %d graphic control block(s) - without one a frame has no delay, "+
			"and a reader is free to show the whole animation in an instant", s.frames, s.controls)
	}
	if s.loopBlocks != 1 {
		t.Errorf("the file carries %d looping block(s), so the animation plays once and a tester "+
			"who blinks sees a still picture", s.loopBlocks)
	}

	// The marker has to be somewhere different in every frame. Equal offsets
	// would still decode, still animate on paper, and show nothing moving.
	seen := map[int]bool{}
	for i, left := range s.lefts {
		if seen[left] {
			t.Errorf("frame %d starts at x=%d and so does an earlier one, so nothing appears to move", i, left)
		}
		seen[left] = true
	}

	// One table for the whole file. Measured on 2026-08-29: without it every
	// frame carries its own copy of the palette, and a 12 px square cost 233 B
	// instead of 41 B.
	if s.globalTable == 0 {
		t.Error("the file has no global colour table, so every frame carries its own copy of the palette")
	}
	if s.localTables != 0 {
		t.Errorf("%d frame(s) carry their own colour table on top of the global one", s.localTables)
	}

	// The padding channel still has to be there, because the size is exact.
	if s.comments != 1 {
		t.Errorf("the file carries %d comment block(s) and the padding channel is the comment", s.comments)
	}
	if len(b) != 65536 {
		t.Errorf("the file is %d B where 65536 B was asked for", len(b))
	}
}

func TestAStillGifIsStillAvailableAndSaysSo(t *testing.T) {
	target := engine.Target{
		ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
		Properties: map[string]string{"width": "64", "height": "64", "frames": "1"},
	}
	b := generateOne(t, target)

	s, err := walkGIF(b)
	if err != nil {
		t.Fatalf("reading back the generated GIF: %v", err)
	}
	if s.frames != 1 {
		t.Errorf("frames: 1 produced %d frame(s)", s.frames)
	}
	// A single frame goes through the plain encoder, which writes neither a
	// control block nor a looping block. Those two absences are what make the
	// bytes the same as the ones this package wrote before it could animate,
	// and generatorbytes_test.go pins that with the hash the animated case
	// used to carry.
	if s.controls != 0 || s.loopBlocks != 0 {
		t.Errorf("a still GIF carries %d control block(s) and %d looping block(s), so it is not "+
			"the file the plain encoder writes", s.controls, s.loopBlocks)
	}
	if len(b) != 65536 {
		t.Errorf("the file is %d B where 65536 B was asked for", len(b))
	}
}
