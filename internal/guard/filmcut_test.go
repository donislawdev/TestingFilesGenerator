package guard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// An MP4 or a WebM laid out for a cut comes to every size asked for, every
// sample of it is where its tables say, and the cut keeps the share of its
// pictures.
//
// Both put padding in front of the pictures for a cut - free boxes between the
// movie box and the media data, a Void between the Tracks and the first
// cluster - and each side is nothing or at least a piece, eight bytes for a
// free box and two for a Void: where the share wants less than a piece in
// front, a piece goes there, and where the padding left behind would be
// shorter than one, it goes in front too (format.SplitFront). The padding in
// front moves every sample, and the chunk places of an MP4 and the Cues of a
// WebM give their places in bytes, so they move with it. A WebM is coded twice
// for it, and the first pass's lengths have to be the second's. No other guard
// asks for the sizes and the cuts where those happen.
//
// So this writes consecutive sizes cut at the half, and cuts of one size a
// byte at a time where the share starts to want padding in front and where
// the padding behind runs out, holds each to its size and to the share, and
// asserts it met each layout. Instead of decoding, every sample is read where
// the file's own tables put it and has to be the bytes of the same sample in
// the film nothing cuts - the same stream gives the same pictures in every
// decoder, which is what docs/TRUNCATE-2026-11-11.md section 14.4 measured for
// the layouts - and every cue and seek of a WebM has to land on the element it
// names.
//
// An MP4 of 5 GiB cut at 90% is planned and only its head is written: its
// media data starts past where four bytes reach, so the chunk places widen to
// co64.
//
// Canary: a chunk place a byte off is refused.
func TestAnMP4OrAWebMLaidOutForACutComesToEverySizeAndPlays(t *testing.T) {
	for _, id := range []string{"mp4", "webm"} {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		least := map[string]int64{"mp4": 8, "webm": 2}[id]
		wholes := map[int64][][]byte{}
		met := map[string]bool{}
		check := func(size, cutAt int64) {
			data, regions := writeBoxes(t, desc, size, cutAt)
			layout, err := filmLayoutOf(data, regions, size, cutAt, least)
			if err != nil {
				t.Errorf("%s %d B cut at %d: %v", id, size, cutAt, err)
				return
			}
			met[layout] = true
			if _, ok := wholes[size]; !ok {
				whole, _ := writeBoxes(t, desc, size, 0)
				if wholes[size], err = filmSamples(id, whole); err != nil {
					t.Fatalf("%s %d B nothing cuts: %v", id, size, err)
				}
			}
			if err := sameSamples(id, data, wholes[size]); err != nil {
				t.Errorf("%s %d B cut at %d, %s: %v", id, size, cutAt, layout, err)
			}
		}
		smallest := desc.SmallestAccepted(format.Request{Label: true})
		for size := smallest; size < smallest+6; size++ {
			check(size, size/2)
		}
		// Cuts a byte apart where the share starts to want padding in front -
		// it wants cutAt times the part of the file that is not the pictures,
		// less the head - and near the end, where what the share leaves behind
		// runs out: j bytes from the end it leaves j times that part, less
		// what stands behind the padding (a WebM's Cues).
		const size = 16_000
		_, regions := writeBoxes(t, desc, size, 0)
		head, pics, end := filmParts(t, regions)
		rest := size - pics.Length
		start := (head - 3) * size / rest
		for cutAt := start; cutAt < start+(least+5)*size/rest; cutAt++ {
			check(size, cutAt)
		}
		for j := max(1, (end-1)*size/rest); j <= (end+least+2)*size/rest; j++ {
			check(size, size-j)
		}
		layouts := []string{"nothing in front, the share wants none", "a piece in front, the share wants less",
			"the share", "all in front, what would stay behind is shorter than a piece"}
		if end > 0 {
			layouts = append(layouts, "all in front, the share wants more than there is")
		}
		for _, layout := range layouts {
			if !met[layout] {
				t.Errorf("%s: no size met the layout %q - met %v", id, layout, met)
			}
		}
	}

	mp4TooLargeForFourBytes(t)

	// Canary: a chunk place a byte off.
	desc, _ := format.Get("mp4")
	whole, _ := writeBoxes(t, desc, 64<<10, 0)
	samples, err := filmSamples("mp4", whole)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := writeBoxes(t, desc, 64<<10, 32<<10)
	at := bytes.Index(data, []byte("stco")) + 12
	broken := bytes.Clone(data)
	binary.BigEndian.PutUint32(broken[at:], binary.BigEndian.Uint32(broken[at:])+1)
	if err := sameSamples("mp4", broken, samples); err == nil {
		t.Errorf("canary: a chunk place a byte off passed")
	}
}

// filmParts reads a film's map as what stands in front of all of its padding,
// its pictures, and what stands behind all of its padding - a WebM's Cues.
func filmParts(t *testing.T, regions []format.Region) (head int64, pics format.Region, end int64) {
	t.Helper()
	seen := false
	for _, r := range regions {
		switch r.Role {
		case format.RoleStructure:
			head += r.Length
		case format.RoleEnd:
			end += r.Length
		case format.RoleContent:
			pics, seen = r, true
		}
	}
	if !seen || pics.Length < 0 {
		t.Fatalf("the map marks the pictures as %+v", pics)
	}
	return head, pics, end
}

// filmLayoutOf holds a film laid out for a cut to its size and the share the
// cut keeps of its pictures, and names the layout it took. What stands behind
// the padding behind the pictures - the Cues of a WebM - is the end region.
func filmLayoutOf(data []byte, regions []format.Region, size, cutAt, least int64) (string, error) {
	if int64(len(data)) != size {
		return "", fmt.Errorf("came to %d B", len(data))
	}
	return filmLayoutOfMap(regions, size, cutAt, least)
}

// filmLayoutOfMap is filmLayoutOf from the map alone, for a film whose bytes
// are not all written.
func filmLayoutOfMap(regions []format.Region, size, cutAt, least int64) (string, error) {
	content := -1
	for i, r := range regions {
		if r.Role == format.RoleContent {
			content = i
		}
	}
	if content < 0 {
		return "", errors.New("no content marked")
	}
	pics := regions[content]
	if pics.Length < 0 {
		return "", errors.New("the pictures are marked with no length")
	}
	front, back, end := int64(0), int64(0), int64(0)
	if content > 0 && regions[content-1].Role == format.RolePadding {
		front = regions[content-1].Length
	}
	for _, r := range regions[content+1:] {
		switch {
		case r.Role == format.RolePadding && end == 0:
			back += r.Length
		case r.Role == format.RoleEnd:
			end += r.Length
		default:
			return "", fmt.Errorf("a %s region at %d, behind the pictures", r.Role, r.At)
		}
	}
	if pics.At+pics.Length+back+end != size {
		return "", fmt.Errorf("the pictures end at %d, and %d B of padding and %d B of end follow them in a %d B file",
			pics.At+pics.Length, back, end, size)
	}
	if (front > 0 && front < least) || (back > 0 && back < least) {
		return "", fmt.Errorf("a side shorter than a piece: %d in front, %d behind", front, back)
	}
	head := pics.At - front
	want := format.PaddingInFront(head, pics.Length, cutAt, size)
	kept, share := cutAt-head-front, pics.Length*cutAt/size
	switch {
	case front == 0 && want <= 0:
		return "nothing in front, the share wants none", nil
	case front == 0:
		return "", fmt.Errorf("nothing in front and %d B behind, where the share wants %d B in front", back, want)
	case front == want && kept == share:
		return "the share", nil
	case front == least && want > 0 && want < least:
		return "a piece in front, the share wants less", nil
	case back == 0 && front > want && front-want < least:
		return "all in front, what would stay behind is shorter than a piece", nil
	case back == 0 && front < want && end > 0:
		// A cut in the last pictures, close to the Cues, wants more in front
		// than there is padding. The cut keeps more than the share then, the
		// one way format.SplitFront allows, and the manifest says how much.
		return "all in front, the share wants more than there is", nil
	}
	return "", fmt.Errorf("%d B in front where the share wants %d, keeping %d B of the pictures where the share is %d", front, want, kept, share)
}

// sameSamples requires every sample of a film, read where its own tables put
// it, to be the bytes of the same sample of the film nothing cuts.
func sameSamples(id string, data []byte, whole [][]byte) error {
	got, err := filmSamples(id, data)
	if err != nil {
		return err
	}
	if len(got) != len(whole) {
		return fmt.Errorf("%d samples, where the film nothing cuts has %d", len(got), len(whole))
	}
	for i := range got {
		if !bytes.Equal(got[i], whole[i]) {
			return fmt.Errorf("sample %d is not the bytes of the film nothing cuts", i+1)
		}
	}
	return nil
}

// filmSamples is every sample of a film, read where its own tables put it -
// stsz, stsc and stco or co64 for an MP4, the blocks of each cluster for a
// WebM, whose cues and seeks have to land on the elements they name.
func filmSamples(id string, data []byte) ([][]byte, error) {
	if id == "webm" {
		return webmSamples(data)
	}
	top, err := mp4Boxes(data)
	if err != nil {
		return nil, err
	}
	var moov []byte
	for _, b := range top {
		if b.name == "moov" {
			moov = b.body
		}
	}
	stbl, err := mp4Path(moov, "trak", "mdia", "minf", "stbl")
	if err != nil {
		return nil, err
	}
	starts, lens, err := mp4Samples(stbl)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(starts))
	for i := range starts {
		if starts[i] < 0 || starts[i]+lens[i] > int64(len(data)) {
			return nil, fmt.Errorf("sample %d lies outside the file", i+1)
		}
		out[i] = data[starts[i] : starts[i]+lens[i]]
	}
	return out, nil
}

// webmSamples walks a WebM's segment by position: the blocks of every
// cluster, and every cue and seek, which have to give the place of a cluster
// starting at the cue's time and of the element the seek names, counted from
// the start of the segment's content.
func webmSamples(data []byte) ([][]byte, error) {
	var seg []byte
	if err := ebmlElements(data, func(id uint64, body []byte) error {
		if id == 0x18538067 {
			seg = body
		}
		return nil
	}); err != nil {
		return nil, err
	}
	var f filmRead
	ids, times := map[uint64]uint64{}, map[uint64]int64{}
	var cues, seeks [][2]uint64
	for pos := 0; pos < len(seg); {
		id, n, err := ebmlVint(seg[pos:], true)
		if err != nil {
			return nil, err
		}
		size, m, err := ebmlVint(seg[pos+n:], false)
		if err != nil {
			return nil, err
		}
		start := pos + n + m
		if uint64(len(seg)-start) < size {
			return nil, fmt.Errorf("element %#x at %d says %d B and %d remain", id, pos, size, len(seg)-start)
		}
		el := seg[start : start+int(size)]
		ids[uint64(pos)] = id
		switch id {
		case 0x1F43B675:
			first := len(f.blocks)
			if err := walkCluster(&f, el); err != nil {
				return nil, err
			}
			if len(f.blocks) > first {
				times[uint64(pos)] = f.blocks[first].ts
			}
		case 0x1C53BB6B:
			cues, err = webmPairs(el, 0xBB, 0xB3, 0xB7, 0xF1)
		case 0x114D9B74:
			seeks, err = webmPairs(el, 0x4DBB, 0x53AB, 0, 0x53AC)
		}
		if err != nil {
			return nil, err
		}
		pos = start + int(size)
	}
	for _, c := range cues {
		if ids[c[1]] != 0x1F43B675 || times[c[1]] != int64(c[0]) {
			return nil, fmt.Errorf("the cue at %d ms gives %d, where no cluster starts at that time", c[0], c[1])
		}
	}
	for _, s := range seeks {
		if ids[s[1]] != s[0] {
			return nil, fmt.Errorf("the seek for %#x gives %d, where %#x starts", s[0], s[1], ids[s[1]])
		}
	}
	if len(cues) == 0 || len(seeks) != 3 {
		return nil, fmt.Errorf("%d cues and %d seeks", len(cues), len(seeks))
	}
	out := make([][]byte, len(f.blocks))
	for i, b := range f.blocks {
		out[i] = b.data
	}
	return out, nil
}

// webmPairs reads each element point in el as a pair: the value of key and
// the value of place, place found inside the element in, or among point's
// own when in is 0.
func webmPairs(el []byte, point, key, in, place uint64) ([][2]uint64, error) {
	var out [][2]uint64
	err := ebmlElements(el, func(id uint64, body []byte) error {
		if id != point {
			return nil
		}
		var pair [2]uint64
		found := 0
		err := ebmlElements(body, func(id uint64, v []byte) error {
			switch {
			case id == key:
				pair[0] = beUint(v)
				found++
			case id == place && in == 0:
				pair[1] = beUint(v)
				found++
			case id == in:
				return ebmlElements(v, func(id uint64, w []byte) error {
					if id == place {
						pair[1] = beUint(w)
						found++
					}
					return nil
				})
			}
			return nil
		})
		if err == nil && found != 2 {
			err = fmt.Errorf("an element %#x without its %#x or %#x", point, key, place)
		}
		out = append(out, pair)
		return err
	})
	return out, err
}

// mp4TooLargeForFourBytes plans an MP4 of 5 GiB cut at 90% and holds the head
// it writes to chunk places of eight bytes, the first of which gives where the
// map says the samples start, and the map to the share, counted with the
// movie box the widening grew. It writes nothing past the head.
func mp4TooLargeForFourBytes(t *testing.T) {
	t.Helper()
	const size, cutAt = 5 << 30, 5 << 30 / 10 * 9
	desc, _ := format.Get("mp4")
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: cutAt})
	if err != nil {
		t.Fatalf("planning %d B: %v", int64(size), err)
	}
	head := &headOnly{keep: 64 << 10}
	rec := &format.MapRecorder{W: head}
	if err := desc.Generator.Write(t.Context(), rec, p); err == nil {
		t.Fatalf("writing %d B went through a writer that stops after its head", int64(size))
	}
	regions := rec.Map(size, true)
	if layout, err := filmLayoutOfMap(regions, size, cutAt, 8); layout != "the share" {
		t.Errorf("%d B cut at 90%%: %q, %v", int64(size), layout, err)
	}
	pics := pictureRegion(regions)
	at := bytes.Index(head.got, []byte("co64"))
	if at < 0 || pics.At <= 1<<32 {
		t.Fatalf("%d B cut at 90%%: no co64 in the head, and the media data starts at %d", int64(size), pics.At)
	}
	if first := int64(binary.BigEndian.Uint64(head.got[at+12:])); first != pics.At+8 {
		t.Errorf("%d B cut at 90%%: the first chunk place is %d, where the samples start at %d", int64(size), first, pics.At+8)
	}
}
