package video

// maxSuffixBits is the longest frame header splitFrame accepts from tile_info
// on: tile_info at most 3 bits for one tile, the quantizer 14, the loop filter
// 29 with the tx mode, and reduced_tx_set 1. A bound stream is built with it,
// so no picture the reader accepts can carry a longer header than planning
// counted.
const maxSuffixBits = 47

// Stream is a film's AV1 samples, one for each frame, as both containers carry
// them: no temporal delimiters, every OBU with its size.
type Stream struct {
	Timeline
	Width, Height int
	// Level is the seq_level_idx the sequence header declares.
	Level int

	config []byte
	keyTU  []byte
	copyTU []byte
	showTU []byte
}

// NewStream wraps a coded picture in the stream's three kinds of sample.
//
// The sequence header travels in every key sample as well as in the
// container's codec configuration, so a player that starts at a key frame has
// it there - which is what the AV1 bindings of both containers ask of a sample
// a player can start from.
func NewStream(t Timeline, c Coded, width, height int) Stream {
	key := keyFrame(c)
	return newStream(t, c, width, height, chooseLevel(width, height, t.FPS, len(key)))
}

// Bound is a stream no smaller than any picture of this size whose tile is at
// most tileBytes can make, for planning without coding the picture.
//
// The level is the maximum parameters one, because a level above seven costs
// the sequence header a tier bit and the bound must not be the short one.
func Bound(t Timeline, tileBytes, width, height int) Stream {
	c := Coded{suffix: make([]byte, (maxSuffixBits+7)/8), suffixBits: maxSuffixBits, tile: make([]byte, tileBytes)}
	return newStream(t, c, width, height, levelMaximumParameters)
}

func newStream(t Timeline, c Coded, width, height, level int) Stream {
	seq := obu(obuSequenceHeader, sequenceHeader(width, height, level))
	return Stream{
		Timeline: t, Width: width, Height: height, Level: level,
		config: codecConfig(level, seq),
		keyTU:  append(append([]byte{}, seq...), keyFrame(c)...),
		copyTU: append(hiddenCopy(c), showCopy()...),
		showTU: showCopy(),
	}
}

// Config is the AV1 codec configuration record with the sequence header OBU,
// what MP4 puts in av1C and Matroska in CodecPrivate.
func (s Stream) Config() []byte { return s.config }

// Sample is frame i's bytes. The slices are shared between frames and must not
// be written to.
func (s Stream) Sample(i int64) []byte {
	switch {
	case s.IsKey(i):
		return s.keyTU
	case i%s.KeyEvery == 1:
		return s.copyTU
	default:
		return s.showTU
	}
}

// SampleBytes is how long frame i's sample is.
func (s Stream) SampleBytes(i int64) int { return len(s.Sample(i)) }

// KindBytes is the length of each of the three kinds of sample - what a
// container's arithmetic needs instead of walking every frame.
func (s Stream) KindBytes() (key, copied, shown int) {
	return len(s.keyTU), len(s.copyTU), len(s.showTU)
}
