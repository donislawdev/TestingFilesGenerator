package guard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// An MP4 film read back from its bytes into what the film guards ask of a
// WebM: its frames, which of them are key frames, its length and its picture -
// so every question in film_test.go is asked of both containers
// (docs/MP4-2026-10-07.md section 7).
//
// The reader knows the boxes this tool writes and the sample tables as far as
// finding each sample, and refuses what it does not understand, as the WebM
// reader does.

// mp4Box is one box: its name and its content.
type mp4Box struct {
	name string
	body []byte
}

// mp4Boxes is a run of boxes, each exactly covering the next.
func mp4Boxes(b []byte) ([]mp4Box, error) {
	var out []mp4Box
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, fmt.Errorf("%d B left where a box should start", len(b))
		}
		size, head := uint64(binary.BigEndian.Uint32(b)), uint64(8)
		if size == 1 {
			if len(b) < 16 {
				return nil, fmt.Errorf("a box with a 64 bit length and no room for it")
			}
			size, head = binary.BigEndian.Uint64(b[8:]), 16
		}
		if size < head || size > uint64(len(b)) {
			return nil, fmt.Errorf("the box %q says %d B and %d remain", b[4:8], size, len(b))
		}
		out = append(out, mp4Box{name: string(b[4:8]), body: b[head:size]})
		b = b[size:]
	}
	return out, nil
}

// mp4Child is the one box called name in b, or an error.
func mp4Child(b []byte, name string) ([]byte, error) {
	boxes, err := mp4Boxes(b)
	if err != nil {
		return nil, err
	}
	var found [][]byte
	for _, x := range boxes {
		if x.name == name {
			found = append(found, x.body)
		}
	}
	if len(found) != 1 {
		return nil, fmt.Errorf("%d %q boxes where a film has one", len(found), name)
	}
	return found[0], nil
}

// mp4Path follows the one box of each name down from b.
func mp4Path(b []byte, names ...string) ([]byte, error) {
	var err error
	for _, n := range names {
		if b, err = mp4Child(b, n); err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
	}
	return b, nil
}

func walkMP4(b []byte) (filmRead, error) {
	var f filmRead
	top, err := mp4Boxes(b)
	if err != nil {
		return f, err
	}
	if len(top) < 4 || top[0].name != "ftyp" || top[1].name != "moov" || top[2].name != "mdat" {
		return f, fmt.Errorf("the file does not open with ftyp, moov and mdat")
	}
	for _, x := range top[3:] {
		if x.name != "free" {
			f.ends = fmt.Sprintf("a %q box after the media data, where only free boxes go", x.name)
		}
	}
	moov := top[1].body
	tkhd, err := mp4Path(moov, "trak", "tkhd")
	if err != nil {
		return f, err
	}
	f.width, f.height = int(binary.BigEndian.Uint32(tkhd[76:])>>16), int(binary.BigEndian.Uint32(tkhd[80:])>>16)
	mdhd, err := mp4Path(moov, "trak", "mdia", "mdhd")
	if err != nil {
		return f, err
	}
	scale, ticks := int64(binary.BigEndian.Uint32(mdhd[12:])), int64(binary.BigEndian.Uint32(mdhd[16:]))
	f.durationMs = float64(ticks * 1000 / scale)
	stbl, err := mp4Path(moov, "trak", "mdia", "minf", "stbl")
	if err != nil {
		return f, err
	}
	stsd, err := mp4Child(stbl, "stsd")
	if err != nil {
		return f, err
	}
	// The sample entry: a version and a count, then av01, whose boxes follow
	// its 78 bytes of fields.
	entry, err := mp4Child(stsd[8:], "av01")
	if err != nil || len(entry) < 78 {
		return f, fmt.Errorf("the sample description holds no AV1 entry: %v", err)
	}
	if f.codecPrivate, err = mp4Child(entry[78:], "av1C"); err != nil {
		return f, err
	}
	starts, lens, err := mp4Samples(stbl)
	if err != nil {
		return f, err
	}
	syncs, err := mp4Syncs(stbl)
	if err != nil {
		return f, err
	}
	f.index = len(syncs)
	for i := range starts {
		if starts[i] < 0 || starts[i]+lens[i] > int64(len(b)) {
			return f, fmt.Errorf("sample %d lies outside the file", i+1)
		}
		f.blocks = append(f.blocks, filmBlock{
			ts:   (int64(i)*1000 + scale/2) / scale,
			key:  syncs[int64(i)+1],
			data: b[starts[i] : starts[i]+lens[i]],
		})
	}
	return f, nil
}

// mp4Samples is where each sample starts in the file and how long it is,
// from stsz, stsc and stco or co64.
func mp4Samples(stbl []byte) (starts, lens []int64, err error) {
	stsz, err := mp4Child(stbl, "stsz")
	if err != nil {
		return nil, nil, err
	}
	for i := range int(binary.BigEndian.Uint32(stsz[8:])) {
		lens = append(lens, int64(binary.BigEndian.Uint32(stsz[12+4*i:])))
	}
	stsc, err := mp4Child(stbl, "stsc")
	if err != nil {
		return nil, nil, err
	}
	places, err := mp4ChunkPlaces(stbl)
	if err != nil {
		return nil, nil, err
	}
	runs := int(binary.BigEndian.Uint32(stsc[4:]))
	for c, place := range places {
		per := int64(0)
		for r := range runs {
			if int(binary.BigEndian.Uint32(stsc[8+12*r:])) <= c+1 {
				per = int64(binary.BigEndian.Uint32(stsc[12+12*r:]))
			}
		}
		for range per {
			if len(starts) == len(lens) {
				return nil, nil, fmt.Errorf("the chunks hold more samples than the %d stsz lists", len(lens))
			}
			starts = append(starts, place)
			place += lens[len(starts)-1]
		}
	}
	if len(starts) != len(lens) {
		return nil, nil, fmt.Errorf("the chunks hold %d samples and stsz lists %d", len(starts), len(lens))
	}
	return starts, lens, nil
}

func mp4ChunkPlaces(stbl []byte) ([]int64, error) {
	boxes, err := mp4Boxes(stbl)
	if err != nil {
		return nil, err
	}
	step, table := 4, "stco"
	for _, x := range boxes {
		if x.name == "co64" {
			step, table = 8, "co64"
		}
	}
	b, err := mp4Child(stbl, table)
	if err != nil {
		return nil, err
	}
	places := make([]int64, binary.BigEndian.Uint32(b[4:]))
	for i := range places {
		if step == 8 {
			places[i] = int64(binary.BigEndian.Uint64(b[8+8*i:]))
		} else {
			places[i] = int64(binary.BigEndian.Uint32(b[8+4*i:]))
		}
	}
	return places, nil
}

// mp4Syncs is the set of sync samples, numbered from one.
func mp4Syncs(stbl []byte) (map[int64]bool, error) {
	stss, err := mp4Child(stbl, "stss")
	if err != nil {
		return nil, err
	}
	out := map[int64]bool{}
	for i := range int(binary.BigEndian.Uint32(stss[4:])) {
		out[int64(binary.BigEndian.Uint32(stss[8+4*i:]))] = true
	}
	return out, nil
}

// An MP4 carries the very frames of the WebM made from the same request.
//
// The pictures and the AV1 stream are one package's for both containers, and
// every guard of the tiles, the levels and the picture's pixels asks a WebM.
// This is what lets those answers stand for MP4 too: asserted here, frame for
// frame and byte for byte, rather than assumed from sharing the code. The
// films are the shapes the stream can take - tiles, a picture changing faster
// than key frames, key frames inside a change, one frame - each with its size
// named so both containers choose the same picture, and without a label,
// which names the format and so would differ.
func TestAnMP4CarriesTheFramesOfTheWebMOfTheSameRequest(t *testing.T) {
	cases := []map[string]string{
		{"width": "640", "height": "360"},
		{"width": "64", "height": "48", "duration": "3s", "change_interval": "100ms", "keyframe_interval": "1s"},
		{"width": "160", "height": "90", "duration": "7s", "change_interval": "300ms", "keyframe_interval": "700ms"},
		{"width": "16", "height": "16", "duration": "40ms", "frame_rate": "25"},
	}
	tiled := 0
	for _, props := range cases {
		read := map[string]filmRead{}
		for _, id := range filmFormats {
			target := filmTarget(id, 2<<20, props)
			target.Label = false
			b, _ := filmOne(t, target)
			f, err := walkFilm(id, b)
			if err != nil {
				t.Fatalf("%s %v: reading the film back: %v", id, props, err)
			}
			read[id] = f
		}
		webm, mp4 := read["webm"], read["mp4"]
		if len(webm.blocks) == 0 || len(webm.blocks) != len(mp4.blocks) {
			t.Fatalf("%v: the WebM has %d frames and the MP4 %d", props, len(webm.blocks), len(mp4.blocks))
		}
		if !bytes.Equal(webm.codecPrivate, mp4.codecPrivate) {
			t.Errorf("%v: the codec configuration differs between the containers", props)
		}
		for i := range webm.blocks {
			w, m := webm.blocks[i], mp4.blocks[i]
			if w.key != m.key || !bytes.Equal(w.data, m.data) {
				t.Errorf("%v: frame %d differs - key %v and %v, %d B and %d B", props, i, w.key, m.key, len(w.data), len(m.data))
				break
			}
		}
		if tilesOf(t, mp4) > 1 {
			tiled++
		}
	}
	if tiled == 0 {
		t.Fatalf("no film was cut into tiles, so the samples of a frame of tiles were never compared")
	}
}

// An MP4's second pass codes no tile its first pass kept.
//
// An MP4 codes its film twice (docs/MP4-2026-10-07.md section 2), and what
// keeps the second pass from costing what the first did is that it takes every
// tile the first one kept (video.Pictures.Again). Taking none makes the same
// bytes in twice the time, which no other guard would see. So a film whose
// clock's tiles all fit in what a film keeps codes exactly the tiles the WebM
// of the same film codes. The films are chosen to fit, so planning codes
// nothing and every tile counted is one writing coded - asserted, along with
// a film of tiles among them.
func TestAnMP4CodesNoTileItsFirstPassKept(t *testing.T) {
	cases := []map[string]string{
		{},
		{"duration": "1m", "change_interval": "500ms"},
	}
	tiled := 0
	for _, props := range cases {
		coded := map[string]int64{}
		for _, id := range filmFormats {
			target := filmTarget(id, 2<<20, props)
			target.Label = false
			before := video.Coded()
			b, _ := filmOne(t, target)
			coded[id] = video.Coded() - before
			if id == "mp4" && filmTileCount(t, id, b) > 1 {
				tiled++
			}
		}
		if coded["webm"] == 0 {
			t.Fatalf("%v: the WebM coded no tile, so there was nothing a second pass could code again", props)
		}
		if coded["mp4"] != coded["webm"] {
			t.Errorf("%v: the MP4 coded %d tiles and the WebM of the same film %d, so its second pass coded again what its first kept", props, coded["mp4"], coded["webm"])
		}
	}
	if tiled == 0 {
		t.Fatalf("no film was cut into tiles, so the tiles a picture is made of were never counted")
	}
}
