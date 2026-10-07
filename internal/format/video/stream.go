package video

// maxRestBits is the longest run of frame header bits after tile_info that
// splitFrame accepts: the quantizer 14, the loop filter 29 with the tx mode,
// and reduced_tx_set 1. Planning counts every picture with it, so no picture
// the reader accepts can carry a longer header than planning counted.
const maxRestBits = 44

// The frame headers this package writes before tile_info, in bits: a shown
// key frame says eight things, a hidden intra only copy eighteen.
const (
	keyHeaderBits    = 8
	hiddenHeaderBits = 18
)

// Stream is what a film's AV1 samples have in common: the sequence header and
// its level, how its picture is cut into tiles, the frame that shows a picture
// again, and the most bytes any one picture may take. The samples that carry
// a picture are made per picture, by Pictures - no temporal delimiters, every
// OBU with its size, as both containers carry them.
type Stream struct {
	Timeline
	Width, Height int
	// Level is the seq_level_idx the sequence header declares.
	Level int
	// Reserve is the most tile bytes any picture of the film may take - the
	// bytes of all its tiles together.
	Reserve int

	geo    geometry
	shape  frameShape
	config []byte
	seq    []byte
	show   []byte
}

// NewStream is the stream of a film whose pictures each take at most reserve
// bytes of tiles, labelled with label - the label moves the clock down, and
// the clock decides the tiles (grid.go).
//
// The level is chosen from the reserve rather than from the pictures, because
// the sequence header is written before any picture after the first is coded.
// The reserve is an upper bound on every one of them, so a level that admits
// it admits the film.
//
// The sequence header travels in every key sample as well as in the
// container's codec configuration, so a player that starts at a key frame has
// it there - which is what the AV1 bindings of both containers ask of a sample
// a player can start from.
func NewStream(t Timeline, reserve, width, height int, label string) Stream {
	geo := geometryOf(width, height, label, t)
	shape := frameShape{grid: gridFor(geo, t.FPS), tileSizeBytes: tileSizeBytes(reserve)}
	key, _ := shape.frameLens(reserve)
	level := chooseLevel(width, height, t.FPS, key, int(t.CodedPerSecond()))
	seq := obu(obuSequenceHeader, sequenceHeader(width, height, level))
	return Stream{
		Timeline: t, Width: width, Height: height, Level: level, Reserve: reserve,
		geo: geo, shape: shape, config: codecConfig(level, seq), seq: seq, show: showCopy(),
	}
}

// Tiles is how many AV1 tiles each picture of the film is cut into.
func (s Stream) Tiles() int { return s.shape.grid.tiles() }

// frameLens is how long the key frame OBU and the hidden copy OBU around a
// picture are when its tiles come to tiles bytes - counted, not built, so
// planning a film never allocates its pictures. The header is counted at the
// longest the reader accepts.
func (s frameShape) frameLens(tiles int) (key, hidden int) {
	var w bitWriter
	s.grid.writeTileInfo(&w, s.tileSizeBytes)
	info := int(w.n)
	group := 0
	if n := s.grid.tiles(); n > 1 {
		group = 1 + (n-1)*s.tileSizeBytes
	}
	obuLen := func(headerBits int) int {
		payload := (headerBits+7)/8 + group + tiles
		return 1 + len(leb128(payload)) + payload
	}
	return obuLen(keyHeaderBits + info + maxRestBits), obuLen(hiddenHeaderBits + info + maxRestBits)
}

// Config is the AV1 codec configuration record with the sequence header OBU,
// what MP4 puts in av1C and Matroska in CodecPrivate.
func (s Stream) Config() []byte { return s.config }

// BoundBytes is the longest each of the three kinds of sample can be - a key
// frame, a hidden copy with the frame that shows it, and a frame that shows a
// picture again - for a container's arithmetic to plan with before any picture
// is coded.
func (s Stream) BoundBytes() [SampleKinds]int {
	k, h := s.shape.frameLens(s.Reserve)
	return [SampleKinds]int{KeySample: len(s.seq) + k, CopySample: h + len(s.show), ShowSample: len(s.show)}
}

// workPerPixel is how many bytes of writing coding one pixel of a picture is
// worth, for the bar a run draws (format.Plan.Work). Measured on 2026-10-06
// with the owner's film, 1810 pictures of 1920x1080 and 2 GB of padding
// (docs/WEBM-WYDAJNOSC-2026-10-06.md): coding 16.9 ns a pixel on sixteen
// threads and 162 on one, writing 2.2 ns a byte. So the true figure is
// between 8 and 74 depending on the machine, and this is one of four to eight
// threads. A wrong figure misjudges only the padding against the pictures,
// and the padding takes seconds, so the estimate is off by seconds rather
// than by the film. Since a film is coded a tile at a time, the pixels are
// those of the tiles coded, not of every picture.
const workPerPixel = 16

// Work is what coding the film is worth (format.Plan.Work): every tile the
// first time its key comes up, at workPerPixel a pixel. Worked out by walking
// the changes' looks, with no painting and no coding - the walk Pictures makes
// while it writes, so what writing reports change by change adds up to this.
func (s Stream) Work() int64 {
	ks := newKeyer(s.geo, s.shape.grid)
	seen := make(map[tileKey]struct{}, ks.grid.tiles())
	var px int64
	var last look
	for c := range s.Changes() {
		l := lookAt(s.geo, s.Timeline, c)
		if c > 0 && l == last {
			continue
		}
		last = l
		px += ks.fresh(l, seen)
	}
	return px * workPerPixel
}

// fresh is how many pixels the tiles of a picture with this look cover whose
// keys are not in seen yet, which it adds them to.
func (ks keyer) fresh(l look, seen map[tileKey]struct{}) int64 {
	var px int64
	for i, t := range ks.tiles {
		k := ks.key(i, l)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		px += int64(t.rect.Dx()) * int64(t.rect.Dy())
	}
	return px
}
