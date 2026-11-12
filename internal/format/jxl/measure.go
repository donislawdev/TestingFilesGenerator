// The door the ladder ceilings are measured through.
//
// The ceilings in jxl.go are the one thing here that cannot be reasoned out -
// they are readings, and a reading is only worth having if it came off the
// code that actually runs. So this asks the real picture builder and the real
// encoder, rather than a copy of them kept in a probe, which is the shape that
// goes stale quietly and then reports a ceiling for a picture nobody draws.
//
// Used by tools/probes/jxlladder to produce the table, and by the guard in
// internal/guard/jxlladder_test.go to keep asking whether the table is still
// true. It costs an encode per call, which is why nothing on the generating
// path uses it.
package jxl

// FileSizeFor reports how many bytes this generator produces for one picture:
// the coded picture, the container around it, and the empty free box every
// file carries. It is the number a rung's ceiling has to be at least as large
// as.
//
// label is passed in rather than derived, because its length moves with the
// size of the file asked for and the ceilings have to cover the longest one.
func FileSizeFor(width, height, quality int, seed uint64, label string) (int, error) {
	m := memo{width: width, height: height, quality: quality, seed: seed, label: label}
	coded, err := encode(picture(m), quality)
	if err != nil {
		return 0, err
	}
	return len(coded) + containerOverhead + boxHeader, nil
}

// Rungs reports the ladder as the dimensions of each rung with its two
// ceilings - for a file nothing damages and for a damaged one - so a probe and
// a guard walk exactly the table planning walks rather than a second copy.
func Rungs() [][4]int64 {
	out := make([][4]int64, 0, len(sizeLadder))
	for _, r := range sizeLadder {
		out = append(out, [4]int64{int64(r.width), int64(r.height), r.ceiling, r.damaged})
	}
	return out
}

// DefaultQuality is the quality the ceilings were measured at, so a probe
// cannot drift from the table by measuring a different one.
const DefaultQuality = defaultQuality

// MinimumBytes is the floor this format announces.
const MinimumBytes = minimumBytes
