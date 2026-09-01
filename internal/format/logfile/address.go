// Addresses, and the rule that keeps the way back exact.
//
// Every draw here is made in the same order and from the same ranges the
// generator used before entry formats existed. That is not tidiness: with
// timestamps=fixed and the settings left alone, the file has to come out byte
// for byte as it did, and a single extra call to the generator would shift
// every entry after it.
//
// Which is why pick does not draw when there is only one thing to choose. A
// list of one is not a choice, and asking for one costs a number out of the
// stream that the old code never spent.
package logfile

import (
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"
)

// pick chooses one of a list, without spending a draw on a list of one.
func pick[T any](rng *rand.Rand, xs []T) T {
	if len(xs) == 1 {
		return xs[0]
	}
	return xs[rng.IntN(len(xs))]
}

const (
	// v4Longest is 255.255.255.255 and v6Longest is eight groups of four hex
	// digits with seven colons. Both are the worst case, which is what the
	// minimum has to be built from.
	v4Longest = 15
	v6Longest = 39
)

// address is one client address, drawn but not yet written.
//
// A value rather than a string, because a log of any size is millions of
// entries and one string per entry is a multiple of the file in garbage. The
// resource guard measures exactly that.
type address struct {
	v6    bool
	parts [8]uint32
}

func drawAddress(rng *rand.Rand, o options) address {
	v6 := o.ipv6
	if o.ipMixed {
		// One draw, so the stream stays predictable, and it is only ever
		// reached when the settings asked for a mixture.
		v6 = rng.IntN(2) == 1
	}
	var a address
	a.v6 = v6
	if !v6 {
		// The same ranges, in the same order, as before entry formats
		// existed. Nothing here may change or the way back stops being exact.
		a.parts[0] = uint32(10 + rng.IntN(240))
		a.parts[1] = uint32(rng.IntN(256))
		a.parts[2] = uint32(rng.IntN(256))
		a.parts[3] = uint32(1 + rng.IntN(254))
		return a
	}
	// Documentation range, so a generated log never names somebody's real
	// network. Groups are written the way a reader sees them, with leading
	// zeros suppressed, which is why the length varies.
	a.parts[0], a.parts[1] = 0x2001, 0x0db8
	for i := 2; i < 8; i++ {
		a.parts[i] = uint32(rng.IntN(0x10000))
	}
	return a
}

// length is how many bytes this address takes when written.
func (a address) length() int {
	if !a.v6 {
		return decDigits(a.parts[0]) + decDigits(a.parts[1]) +
			decDigits(a.parts[2]) + decDigits(a.parts[3]) + 3
	}
	n := 7 // the colons
	for _, p := range a.parts {
		n += hexDigits(p)
	}
	return n
}

func (a address) append(dst []byte) []byte {
	if !a.v6 {
		// No leading zeros. Padding octets to three digits made the line
		// length trivial to predict and produced addresses no real log
		// contains - and a leading zero is read as octal by some address
		// parsers, where 069 is not even valid octal.
		for i := 0; i < 4; i++ {
			if i > 0 {
				dst = append(dst, '.')
			}
			dst = strconv.AppendInt(dst, int64(a.parts[i]), 10)
		}
		return dst
	}
	for i, p := range a.parts {
		if i > 0 {
			dst = append(dst, ':')
		}
		dst = strconv.AppendInt(dst, int64(p), 16)
	}
	return dst
}

// decDigits is how many characters a byte sized number takes.
func decDigits(n uint32) int {
	switch {
	case n < 10:
		return 1
	case n < 100:
		return 2
	default:
		return 3
	}
}

// hexDigits is how many characters a sixteen bit group takes in hex, written
// without leading zeros the way every reader shows it.
func hexDigits(n uint32) int {
	switch {
	case n < 0x10:
		return 1
	case n < 0x100:
		return 2
	case n < 0x1000:
		return 3
	default:
		return 4
	}
}

// longestAddress is the worst case for the settings in force, which is what
// the minimum entry has to leave room for.
func longestAddress(o options) int {
	if o.ipv6 || o.ipMixed {
		return v6Longest
	}
	return v4Longest
}

func longestMethod(o options) int { return longest(o.methods) }
func longestAgent() int           { return longest(agents) }
func longestTag() int             { return longest(tags) }
func longestLevel() int           { return longest(levels) }

func longest(xs []string) int {
	n := 0
	for _, x := range xs {
		if len(x) > n {
			n = len(x)
		}
	}
	return n
}
