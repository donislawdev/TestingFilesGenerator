package video

import "math/big"

// levelMaximumParameters is seq_level_idx 31: "no level-based constraints on
// the bitstream", which the specification reserves for streams that conform to
// no other level.
const levelMaximumParameters = 31

// level is one row of the AV1 level tables, Annex A.3, copied from the
// specification's own source (annex.a.levels.md in AOMediaCodec/av1-spec,
// read 2026-10-06), only the columns a stream from this package can reach.
// MainMbps is kept in kilobits so every comparison below is a whole number.
type level struct {
	idx            int
	maxPicSize     int64
	maxHSize       int64
	maxVSize       int64
	maxDisplayRate int64
	mainKbps       int64
	mainCR         int64
}

// levels runs from the lowest up, so the first that admits a stream is the one
// to declare. 2.2, 2.3, 3.2, 3.3, 4.2, 4.3 and 7.x are not defined, and the
// specification says so in the note under its table.
var levels = []level{
	{0, 147456, 2048, 1152, 4_423_680, 1_500, 2},
	{1, 278784, 2816, 1584, 8_363_520, 3_000, 2},
	{4, 665856, 4352, 2448, 19_975_680, 6_000, 2},
	{5, 1065024, 5504, 3096, 31_950_720, 10_000, 2},
	{8, 2359296, 6144, 3456, 70_778_880, 12_000, 4},
	{9, 2359296, 6144, 3456, 141_557_760, 20_000, 4},
	{12, 8912896, 8192, 4352, 267_386_880, 30_000, 6},
	{13, 8912896, 8192, 4352, 534_773_760, 40_000, 8},
	{14, 8912896, 8192, 4352, 1_069_547_520, 60_000, 8},
	{15, 8912896, 8192, 4352, 1_069_547_520, 60_000, 8},
	{16, 35651584, 16384, 8704, 1_069_547_520, 60_000, 8},
	{17, 35651584, 16384, 8704, 2_139_095_040, 100_000, 8},
	{18, 35651584, 16384, 8704, 4_278_190_080, 160_000, 8},
	{19, 35651584, 16384, 8704, 4_278_190_080, 160_000, 8},
}

// chooseLevel is the lowest level a stream of this package's shape conforms
// to, or the maximum parameters level when none does.
//
// The shape matters to three of the constraints. Every frame is shown at the
// frame rate, so the display rate is width times height times fps. The one
// second buffer has to hold every coded picture one second of the film
// carries - perSecond of them, from Timeline.CodedPerSecond: a key frame and
// its hidden copy, and the picture each change opens with. Until 2026-10-06
// this counted two whatever the film, which was short for a film whose key
// frames come more than once a second. And each coded frame has to meet the
// compression ratio, which is why the level is chosen from how big a picture
// may be rather than from its size alone: a picture at quality 100 can be too
// big for the level its size would suggest.
//
// A frame narrower or shorter than 16 conforms to no level - the specification
// asks FrameWidth and FrameHeight to be at least 16 - so a small picture
// always declares the maximum parameters level.
//
// Whole numbers throughout, because the answer is written into the sequence
// header and so into the bytes (D11), and a float compared at a boundary is a
// place for two machines to disagree.
func chooseLevel(width, height, fps, frameBytes, perSecond int) int {
	if width < 16 || height < 16 {
		return levelMaximumParameters
	}
	pic := int64(width) * int64(height)
	display := pic * int64(fps)
	for _, l := range levels {
		if pic > l.maxPicSize || int64(width) > l.maxHSize || int64(height) > l.maxVSize {
			continue
		}
		if display > l.maxDisplayRate {
			continue
		}
		if int64(perSecond)*8*int64(frameBytes) > l.mainKbps*1000 {
			continue
		}
		if !compressedEnough(pic, display, int64(frameBytes), l) {
			continue
		}
		return l.idx
	}
	return levelMaximumParameters
}

// LevelFor is chooseLevel, exported for the guard that holds it to the
// specification's own examples.
func LevelFor(width, height, fps, frameBytes, perSecond int) int {
	return chooseLevel(width, height, fps, frameBytes, perSecond)
}

// compressedEnough is CompressedRatio >= MinPicCompressRatio, Annex A.3:
//
//	UnCompressedSize / CompressedSize >= max(0.8, MainCR * display / MaxDisplayRate)
//
// with UnCompressedSize = pic * 15 / 8 for profile 0 and CompressedSize the
// frame's bytes less 128, cross multiplied so nothing is divided.
func compressedEnough(pic, display, frameBytes int64, l level) bool {
	compressed := frameBytes - 128
	if compressed <= 0 {
		return true
	}
	uncompressed := pic * 15 >> 3
	// uncompressed * 10 * MaxDisplayRate >= compressed * max(8 * MaxDisplayRate, 10 * MainCR * display)
	floor := new(big.Int).Mul(big.NewInt(8), big.NewInt(l.maxDisplayRate))
	scaled := new(big.Int).Mul(big.NewInt(10*l.mainCR), big.NewInt(display))
	if scaled.Cmp(floor) > 0 {
		floor = scaled
	}
	right := new(big.Int).Mul(big.NewInt(compressed), floor)
	left := new(big.Int).Mul(big.NewInt(uncompressed*10), big.NewInt(l.maxDisplayRate))
	return left.Cmp(right) >= 0
}
