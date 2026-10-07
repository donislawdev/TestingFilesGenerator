package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// A film read back from its bytes, not from the plan that made it - the lesson
// of animation_test.go, where a GIF the plan called animated came out still.
//
// The reader knows WebM's elements and AV1's headers as far as the questions
// below need, and no further, and filmmp4_test.go reads MP4 into the same
// shape. What it does not understand it refuses, so a change in the writer
// that this reader cannot follow fails loudly rather than being read as
// something it is not.

type filmBlock struct {
	ts   int64
	key  bool
	data []byte
}

// filmRead is a film of either container, as the questions below ask it.
type filmRead struct {
	durationMs   float64
	width        int
	height       int
	codecPrivate []byte
	blocks       []filmBlock
	// index is how many places a player can start from the container lists:
	// the cue points of a WebM, the sync samples of an MP4.
	index int
	// tail is the IDs of a WebM segment's last two elements, the padding and
	// then the Cues (docs/WIDEO-2026-10-06.md section 15).
	tail [2]uint64
	// ends says what is wrong with how the file ends - where its padding
	// sits - and is empty when nothing is.
	ends string
}

// filmFormats are the containers a film is made in. The questions of the
// container are asked of each.
var filmFormats = []string{"webm", "mp4"}

// walkFilm reads a film of either container.
func walkFilm(id string, b []byte) (filmRead, error) {
	switch id {
	case "webm":
		return walkWebM(b)
	case "mp4":
		return walkMP4(b)
	}
	return filmRead{}, fmt.Errorf("no reader for films in %s", id)
}

func filmOne(t *testing.T, target engine.Target) ([]byte, map[string]any) {
	t.Helper()
	b, facts, _ := filmWithSeed(t, target)
	return b, facts
}

// av1Frame is what one OBU of a sample says about showing pictures.
type av1Frame struct {
	showExisting bool
	slot         int
	frameType    int
	shown        bool
	showable     bool
	refresh      int
}

// readFrames reads the frame headers of one sample, under the sequence header
// this tool writes: no frame ids, no order hint, screen content off. A sample
// holding anything else - a temporal delimiter, a tile group on its own - is
// refused, because the containers carry neither.
func readFrames(sample []byte) ([]av1Frame, error) {
	var out []av1Frame
	for len(sample) > 0 {
		h := sample[0]
		if h&0x80 != 0 || h&0x04 != 0 || h&0x02 == 0 {
			return nil, fmt.Errorf("an OBU header %#02x", h)
		}
		typ := int(h>>3) & 0xf
		size, used := 0, 0
		for i := 1; i < len(sample) && i < 9; i++ {
			size |= int(sample[i]&0x7f) << (7 * (i - 1))
			if sample[i]&0x80 == 0 {
				used = i
				break
			}
		}
		if used == 0 || 1+used+size > len(sample) {
			return nil, fmt.Errorf("an OBU of type %d runs past its sample", typ)
		}
		payload := sample[1+used : 1+used+size]
		sample = sample[1+used+size:]
		switch typ {
		case 1: // sequence header
		case 3, 6:
			out = append(out, readFrameHeader(payload))
		default:
			return nil, fmt.Errorf("an OBU of type %d in a sample", typ)
		}
	}
	return out, nil
}

func readFrameHeader(p []byte) av1Frame {
	pos := 0
	bit := func() int {
		v := int(p[pos/8]>>(7-pos%8)) & 1
		pos++
		return v
	}
	bits := func(n int) int {
		v := 0
		for range n {
			v = v<<1 | bit()
		}
		return v
	}
	var f av1Frame
	if bit() == 1 {
		f.showExisting, f.slot = true, bits(3)
		return f
	}
	f.frameType = bits(2)
	f.shown = bit() == 1
	if f.shown {
		f.showable = f.frameType != 0
	} else {
		f.showable = bit() == 1
	}
	if f.frameType == 0 && f.shown {
		f.refresh = 0xFF
		return f
	}
	bit() // error_resilient_mode
	bit() // disable_cdf_update
	bit() // frame_size_override_flag
	f.refresh = bits(8)
	return f
}

// showRuleBroken walks a film's frames the way a decoder fills its slots and
// names the first frame that shows a slot holding nothing showable - which is
// what showing a shown key frame again is.
func showRuleBroken(blocks []filmBlock) (int, string) {
	var showable [8]bool
	for i, b := range blocks {
		frames, err := readFrames(b.data)
		if err != nil {
			return i, err.Error()
		}
		for _, f := range frames {
			switch {
			case f.showExisting && !showable[f.slot]:
				return i, fmt.Sprintf("shows slot %d, which holds nothing that may be shown again", f.slot)
			case f.showExisting:
			default:
				for s := range 8 {
					if f.refresh&(1<<s) != 0 {
						showable[s] = f.showable
					}
				}
			}
		}
	}
	return -1, ""
}

func filmTarget(id string, bytes int64, props map[string]string) engine.Target {
	return engine.Target{ID: "film", Format: id, Sizes: engine.Uniform(1, bytes), Label: true, Properties: props}
}

// The stream's one shape: a key frame, its hidden copy, and every other frame
// showing the copy - read back from the file, frame by frame.
//
// What it guards: a shown key frame may not be shown again. libaom refuses a
// stream that does it, and gav1d's own decoder and Chromium play it without a
// word (docs/WIDEO-2026-10-06.md section 9.2), so a writer that slipped into
// it would pass every check here that only plays the file.
func TestAFilmNeverShowsAKeyFrameASecondTime(t *testing.T) {
	cases := []map[string]string{
		{},
		{"duration": "3s", "keyframe_interval": "1s", "frame_rate": "25"},
		{"duration": "2s", "keyframe_interval": "1s", "frame_rate": "1"},
		{"duration": "40ms", "frame_rate": "25"},
	}
	for _, id := range filmFormats {
		for _, props := range cases {
			b, _ := filmOne(t, filmTarget(id, 64*1024, props))
			f, err := walkFilm(id, b)
			if err != nil {
				t.Fatalf("%s %v: reading the film back: %v", id, props, err)
			}
			if len(f.blocks) == 0 {
				t.Fatalf("%s %v: the film has no frames, so nothing here was checked", id, props)
			}
			if i, why := showRuleBroken(f.blocks); i >= 0 {
				t.Errorf("%s %v: frame %d %s - a stream libaom refuses", id, props, i, why)
			}
		}
	}
}

// The film is as long as the manifest says, frame for frame, with key frames
// where the interval puts them, and its picture as large as declared.
func TestAFilmHoldsTheFramesTheManifestDeclares(t *testing.T) {
	cases := []map[string]string{
		{},
		{"duration": "1m", "keyframe_interval": "10s", "frame_rate": "24"},
		{"duration": "59.9s", "frame_rate": "30"},
		{"duration": "1h", "frame_rate": "60"},
	}
	for _, id := range filmFormats {
		for _, props := range cases {
			b, facts := filmOne(t, filmTarget(id, 2*1024*1024, props))
			f, err := walkFilm(id, b)
			if err != nil {
				t.Fatalf("%s %v: reading the film back: %v", id, props, err)
			}
			filmAgreesWithItsManifest(t, id+" "+fmt.Sprint(props), f, facts)
		}
	}
}

// filmAgreesWithItsManifest asks a film read back whether it holds what its
// manifest declares: the frames, their length, the key frames where the
// interval puts them and each in the container's index, the picture's size,
// and the padding where the container keeps it.
func filmAgreesWithItsManifest(t *testing.T, name string, f filmRead, facts map[string]any) {
	t.Helper()
	frames, _ := facts["frame_count"].(int64)
	keys, _ := facts["keyframe_count"].(int64)
	durationMs, _ := facts["duration_ms"].(int64)
	fps, _ := facts["frame_rate"].(int)
	if frames == 0 || fps == 0 || len(f.blocks) == 0 {
		t.Fatalf("%s: the manifest declares %d frames at %d a second and the file holds %d, so nothing here was checked", name, frames, fps, len(f.blocks))
	}
	if int64(len(f.blocks)) != frames {
		t.Errorf("%s: the file holds %d frames and the manifest declares %d", name, len(f.blocks), frames)
	}
	if f.durationMs != float64(durationMs) {
		t.Errorf("%s: the file says it lasts %v ms and the manifest %d ms", name, f.durationMs, durationMs)
	}
	var seenKeys int64
	interval, _ := facts["keyframe_interval_ms"].(int64)
	every := interval * int64(fps) / 1000
	for i, blk := range f.blocks {
		if blk.key {
			seenKeys++
			if every > 0 && int64(i)%every != 0 {
				t.Errorf("%s: frame %d is a key frame and the interval puts them every %d frames", name, i, every)
			}
		}
	}
	if seenKeys != keys || int64(f.index) != keys {
		t.Errorf("%s: %d key frames and %d in the index, and the manifest declares %d key frames", name, seenKeys, f.index, keys)
	}
	if w, _ := facts["width"].(int); w != f.width {
		t.Errorf("%s: the track is %d px wide and the manifest says %d", name, f.width, w)
	}
	if h, _ := facts["height"].(int); h != f.height {
		t.Errorf("%s: the track is %d px tall and the manifest says %d", name, f.height, h)
	}
	if f.ends != "" {
		t.Errorf("%s: %s", name, f.ends)
	}
	if last := f.blocks[len(f.blocks)-1].ts; last >= durationMs {
		t.Errorf("%s: the last frame starts at %d ms, at or after the film's end at %d ms", name, last, durationMs)
	}
}

// The level a film declares is the lowest the AV1 specification allows it.
//
// Held to the specification's own examples - the Example column of Annex A.3,
// one resolution and rate per level - rather than to a table copied from it,
// because a copy of a table agrees with the table it was copied from. A level
// too low makes a hardware decoder refuse the film, one too high says the
// film needs more than it does.
func TestAFilmDeclaresTheLowestLevelTheSpecificationAllows(t *testing.T) {
	small := 1000 // a picture's frame, far inside every level's buffer and ratio
	cases := []struct {
		w, h, fps, want int
		example         string
	}{
		{426, 240, 30, 0, "2.0"}, {640, 360, 30, 1, "2.1"}, {854, 480, 30, 4, "3.0"},
		{1280, 720, 30, 5, "3.1"}, {1920, 1080, 30, 8, "4.0"}, {1920, 1080, 60, 9, "4.1"},
		{3840, 2160, 30, 12, "5.0"}, {3840, 2160, 60, 13, "5.1"},
		{7680, 4320, 30, 16, "6.0"}, {7680, 4320, 60, 17, "6.1"},
	}
	for _, c := range cases {
		if got := video.LevelFor(c.w, c.h, c.fps, small, 2); got != c.want {
			t.Errorf("%dx%d at %d fps is the example of level %s (seq_level_idx %d) and declares %d", c.w, c.h, c.fps, c.example, c.want, got)
		}
	}
	// No level admits a frame under 16 on a side, so those declare the
	// maximum parameters level, 31.
	for _, s := range [][2]int{{1, 1}, {15, 240}, {320, 15}} {
		if got := video.LevelFor(s[0], s[1], 30, 100, 2); got != 31 {
			t.Errorf("%dx%d declares level %d, and no defined level admits a side under 16", s[0], s[1], got)
		}
	}
	// A frame too big for a level's buffer, or too little compressed for it,
	// moves the film up - 640x360 at quality 100 is not a level 2.1 film.
	if got := video.LevelFor(640, 360, 30, 300_000, 2); got <= 1 {
		t.Errorf("a 300 kB frame of 640x360 declares level %d, whose one second buffer it does not fit twice", got)
	}
	// The buffer holds every picture a second carries, not two. A 20 kB
	// picture of 640x360 is a level 2.1 film shown twice a second, and not
	// one coded thirty times a second - a new picture every frame, or a key
	// frame every frame - because thirty of them are 4.8 Mbit and level 2.1
	// holds 3.
	if got := video.LevelFor(640, 360, 30, 20_000, 2); got != 1 {
		t.Errorf("a 20 kB picture of 640x360 coded twice a second declares level %d, and level 2.1 (1) holds it", got)
	}
	if got := video.LevelFor(640, 360, 30, 20_000, 30); got <= 1 {
		t.Errorf("a 20 kB picture of 640x360 coded thirty times a second declares level %d, whose one second buffer holds 3 Mbit", got)
	}
}

// A length that does not end on a frame is refused with the two nearest that
// do - the owner's decision of 2026-10-06 (docs/WIDEO-2026-10-06.md section
// 12) - and not rounded to one of them. The same for the key frame interval,
// and for an interval that would give a film more key frames than it may have.
func TestALengthThatDoesNotEndOnAFrameIsRefusedWithTheTwoNearest(t *testing.T) {
	cases := []struct {
		props   map[string]string
		key     string
		offered string
	}{
		{map[string]string{"duration": "1.05s"}, "duration", "Ask for 1s or 1.1s"},
		{map[string]string{"duration": "1.1s", "frame_rate": "24"}, "duration", "Ask for 1s or 1.125s"},
		{map[string]string{"duration": "50ms"}, "duration", "Ask for 100ms, the shortest film at this rate"},
		{map[string]string{"keyframe_interval": "1.05s"}, "keyframe_interval", "Ask for 1s or 1.1s"},
		{map[string]string{"duration": "24h", "keyframe_interval": "500ms"}, "keyframe_interval", "Ask for 900ms or longer, or a shorter film"}, // 864ms is not on a frame at 30 fps
	}
	for _, id := range filmFormats {
		for _, c := range cases {
			_, err := engine.Plan([]engine.Target{filmTarget(id, 1024*1024, c.props)}, engine.Options{OutDir: t.TempDir(), Seed: goldenSeed, Command: "test"})
			var refused *format.PropertyValueError
			if !errors.As(err, &refused) {
				t.Errorf("%s %v was not refused as a setting it cannot have: %v", id, c.props, err)
				continue
			}
			// The way out is in the sentence the command line prints, which
			// is the reason - a remedy beside it would reach the window alone.
			if refused.Key != c.key || !strings.HasSuffix(refused.Error(), c.offered) {
				t.Errorf("%s %v was refused about %q as %q - expected %q ending in %q", id, c.props, refused.Key, refused.Error(), c.key, c.offered)
			}
		}
		// And a length that does end on a frame is not refused at all.
		for _, props := range []map[string]string{{"duration": "1.1s"}, {"duration": "1.125s", "frame_rate": "24"}, {"duration": "40ms", "frame_rate": "25"}} {
			if _, err := engine.Plan([]engine.Target{filmTarget(id, 1024*1024, props)}, engine.Options{OutDir: t.TempDir(), Seed: goldenSeed, Command: "test"}); err != nil {
				t.Errorf("%s %v ends on a frame and was refused: %v", id, props, err)
			}
		}
	}
}

// The picture a viewer sees changes exactly where the manifest says, and
// nowhere else - asked of libaom's decoded frames, not of the file's bytes.
//
// Every other guard here passed the film whose picture never moved, which is
// what the owner saw on 2026-10-06 and nothing in this repository could have
// (docs/WIDEO-2026-10-06.md section 14). The cases are the ones that can go
// wrong apart: a change a second, changes faster than key frames, key frames
// inside a change - where a new key frame must not move the picture - and a
// change interval longer than the film, which is one picture throughout.
func TestAFilmPictureChangesWhereTheManifestSays(t *testing.T) {
	cases := []map[string]string{
		{},
		{"duration": "3s", "change_interval": "100ms"},
		{"duration": "5s", "change_interval": "2s", "keyframe_interval": "1s"},
		{"duration": "2s", "change_interval": "1h"},
	}
	moving, tiled := 0, 0
	for _, id := range filmFormats {
		for _, props := range cases {
			b, facts := filmOne(t, filmTarget(id, 2*1024*1024, props))
			if filmTileCount(t, id, b) > 1 {
				tiled++
			}
			path := filepath.Join(t.TempDir(), "film."+id)
			if err := os.WriteFile(path, b, 0o600); err != nil {
				t.Fatal(err)
			}
			got, res := oracle.PictureChanges(path)
			if !res.Available {
				t.Skipf("%s is not installed, so no picture was looked at - a skip, not a pass", res.Tool)
			}
			if res.Err != nil {
				t.Errorf("%s %v: %v", id, props, res.Err)
				continue
			}
			count, _ := facts["change_count"].(int64)
			interval, _ := facts["change_interval_ms"].(int64)
			fps, _ := facts["frame_rate"].(int)
			if count == 0 || interval == 0 || fps == 0 {
				t.Fatalf("%s %v: the manifest declares %d changes every %d ms at %d frames a second, so nothing here was checked", id, props, count, interval, fps)
			}
			want := make([]int64, count)
			for c := range want {
				want[c] = int64(c) * interval * int64(fps) / 1000
			}
			if !slices.Equal(got, want) {
				t.Errorf("%s %v: the decoded picture changes at frames %v and the manifest puts the changes at %v", id, props, got, want)
			}
			if count > 1 {
				moving++
			}
		}
	}
	if moving < 3*len(filmFormats) {
		t.Fatalf("only %d of the films had a picture that moves, so the guard did not ask its question", moving)
	}
	// Asserted, not assumed: a picture cut into tiles is coded a tile at a
	// time, and a tile kept from an earlier picture is where a picture that
	// should move could stand still - in MP4 also from the first of its two
	// passes.
	if tiled < 2*len(filmFormats) {
		t.Fatalf("only %d of the films were cut into tiles, so the guard did not ask about tiles", tiled)
	}
}

// libaom decodes every frame the manifest declares - not only "decodes".
//
// The count is the half the registry-wide reference tool guard cannot ask: a
// film cut off after its first frames decodes without a complaint about the
// frames that are there (measured with ffmpeg 9.0.1 on a WebM cut in half,
// docs/WIDEO-2026-10-06.md section 9.5), so "the tool said nothing" is not
// "the film is whole".
func TestEveryFrameOfAFilmSurvivesItsReferenceTool(t *testing.T) {
	checker, known := oracle.For("libaom")
	if !known {
		t.Fatal("the libaom oracle is not implemented")
	}
	cases := []map[string]string{
		{},
		{"duration": "10m", "keyframe_interval": "30s", "frame_rate": "30"},
		{"duration": "40ms", "frame_rate": "25"},
		// Wider than one tile can be (grid.go, cutToFit).
		{"width": "4240", "height": "1000", "duration": "2s"},
	}
	tiled := map[string]int{}
	for _, id := range filmFormats {
		for _, props := range cases {
			b, facts := filmOne(t, filmTarget(id, 4*1024*1024, props))
			if filmTileCount(t, id, b) > 1 {
				tiled[id]++
			}
			path := filepath.Join(t.TempDir(), "film."+id)
			if err := os.WriteFile(path, b, 0o600); err != nil {
				t.Fatal(err)
			}
			res := checker.Check(path)
			if !res.Available {
				t.Skipf("%s is not installed, so no film was decoded - a skip, not a pass", checker.Name)
			}
			if res.Err != nil {
				t.Errorf("%s %v: %v", id, props, res.Err)
				continue
			}
			frames, _ := facts["frame_count"].(int64)
			got, ok := oracle.DecodedFrames(res.Output)
			if !ok || int64(got) != frames {
				t.Errorf("%s %v: libaom decoded %d frames and the manifest declares %d", id, props, got, frames)
			}
		}
		// A frame of tiles is the one whose tile_info and tile sizes this tool
		// writes itself, so at least one film of each container has to be one.
		if tiled[id] == 0 {
			t.Fatalf("no %s film was cut into tiles, so libaom was never asked about a frame of tiles", id)
		}
	}
}

// A picture larger than any AV1 level describes is refused as a setting,
// before coding, and the pictures that used to be refused for not fitting one
// tile are not.
//
// Until 2026-10-07 the bound was one AV1 tile, 4096x2304, because gav1d codes
// one tile correctly and no more (docs/REVIEW-165-2026-10-06.md). A picture
// is cut into tiles of at most that now, and the bound is AV1's own: 8192 by
// 4352, the 6.x levels' MaxPicSize, with either side up to 16384 (the owner's
// decision, docs/WEBM-LIMIT-2026-10-07.md section 10). The refused pairs are
// one pixel over the area each way and a side over 16384. The accepted ones
// are the four the old bound refused, cheap to plan, and the two longest
// sides - not 8192x4352 itself, whose plan codes a sample of 8K pictures.
func TestAFilmPictureLargerThanAnyAV1LevelIsRefusedNotBroken(t *testing.T) {
	for _, id := range filmFormats {
		plan := func(w, h string) error {
			props := map[string]string{"width": w, "height": h, "duration": "1s"}
			_, err := engine.Plan([]engine.Target{filmTarget(id, 32*1024*1024, props)}, engine.Options{OutDir: t.TempDir(), Seed: goldenSeed, Command: "test"})
			return err
		}
		for _, size := range [][2]string{{"8192", "4353"}, {"8193", "4352"}, {"16384", "2177"}, {"16385", "64"}, {"64", "16385"}} {
			var refused *format.PropertyValueError
			if err := plan(size[0], size[1]); !errors.As(err, &refused) {
				t.Errorf("%s %sx%s was not refused as a setting: %v", id, size[0], size[1], err)
			}
		}
		for _, size := range [][2]string{{"4096", "2305"}, {"4000", "2359"}, {"4097", "64"}, {"4352", "512"}, {"16384", "64"}, {"64", "16384"}} {
			if err := plan(size[0], size[1]); err != nil {
				t.Errorf("%s %sx%s is inside every bound the format declares and was refused: %v", id, size[0], size[1], err)
			}
		}
	}
}

// A film whose picture size is named by hand is written whole when it holds
// many pictures - every one of them inside the reserve planning settled from a
// sample of ten (docs/WIDEO-2026-10-06.md section 15).
//
// The sizes are the ones where the pictures of one film spread most, measured
// by tools/probes/videomotion/spread: a reserve a tenth above the first picture
// refused the 64x48 film of the golden set, and on 48x32 the largest of a film
// came out 42 percent above its first. Each case holds more than ten pictures,
// so it is the sample and its margin that is asked, not a film planning coded
// whole.
func TestAFilmOfANamedSizeKeepsEveryPictureInsideItsReserve(t *testing.T) {
	cases := []struct {
		bytes int64
		props map[string]string
	}{
		{1 << 20, map[string]string{"width": "48", "height": "32", "duration": "10m"}},
		{4 << 20, map[string]string{"width": "52", "height": "20", "duration": "1h"}},
		{2 << 20, map[string]string{"width": "64", "height": "48", "duration": "2m", "change_interval": "100ms"}},
		// Cut into tiles: a picture is its tiles together, and the sample
		// counts them that way.
		{16 << 20, map[string]string{"width": "320", "height": "180", "duration": "2m", "change_interval": "100ms"}},
	}
	tiled := 0
	for _, id := range filmFormats {
		for _, c := range cases {
			b, facts := filmOne(t, filmTarget(id, c.bytes, c.props))
			if filmTileCount(t, id, b) > 1 {
				tiled++
			}
			if changes, _ := facts["change_count"].(int64); changes <= 10 {
				t.Fatalf("%s %v: %d pictures, which planning codes whole, so the sample was not asked", id, c.props, changes)
			}
			if int64(len(b)) != c.bytes {
				t.Errorf("%s %v: the film is %d B and %d B were asked for", id, c.props, len(b), c.bytes)
			}
		}
	}
	if tiled < len(filmFormats) {
		t.Fatalf("%d films were cut into tiles, so no reserve of tiles was asked of each container", tiled)
	}
}
