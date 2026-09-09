package damage

import (
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Setting names. Public names under untouchable rule 10, so they are spelled
// once.
const (
	ZeroHead = "zero-head"

	SettingBytes = "bytes"
)

const (
	// smallestHead is the fewest bytes worth zeroing, and it is a MEASUREMENT
	// rather than a round number.
	//
	// Measured 2026-09-09 across all twenty four formats, on a 20 kB file:
	// zeroing 1 byte has 20 witnesses, 2 bytes has 20, and 4, 8, 16 and 64
	// bytes all have 24. So below four the tool would offer a damage that four
	// formats produce and nothing can refuse - a file with no witness, which
	// is the one thing this axis must not write.
	//
	// This is the same rule the column ceiling followed: the bound belongs to
	// somebody else's reader, not to what our code can do. Our code can zero
	// one byte perfectly well.
	smallestHead = 4

	// largestHead is where the parameter stops.
	//
	// Not a measured refusal - 64 bytes had 24 witnesses and nothing suggests
	// a ceiling above it behaves differently. It is here because a range with
	// no upper end draws a field a person can put anything in, and because a
	// head larger than the file is already refused by the floor. 4096 is the
	// smallest round number comfortably above every signature and header this
	// tool writes.
	largestHead = 4096

	defaultHead = 8
)

func init() {
	Register(Descriptor{
		ID: ZeroHead,
		// What it does to the bytes, not what it is for. An intent-shaped
		// sentence would be wrong about a third of the formats already: txt,
		// md and log have no signature at all, and their witness is the
		// structural check refusing zero bytes inside text.
		Detail: "Overwrites the first bytes of the file with zeros, leaving its length alone. " +
			"Most readers look there first, so this is the damage almost anything notices.",
		Parameters: []format.Property{
			{
				Name: SettingBytes, Kind: format.PropertyInt,
				Min: smallestHead, Max: largestHead, Unit: "bytes",
				Default: strconv.Itoa(defaultHead),
				Detail: "How many bytes at the start are zeroed. " +
					"Below four, some formats come out with damage no reader complains about.",
			},
		},
		Floor: func(v Values) int64 { return int64(headBytes(v)) },
		Open: func(v Values, out io.Writer) (Stream, error) {
			return &zeroHead{out: out, left: headBytes(v)}, nil
		},
	})
}

// headBytes is how many bytes this run zeroes.
//
// The value has been past the declaration by the time it arrives, so a bad one
// cannot reach here through either surface. It falls back rather than failing
// for the same reason textenc.Parse keeps its branches: this is callable
// directly, a guard is such a caller, and code that trusts its input is one
// registry change away from writing a file nobody ordered.
func headBytes(v Values) int {
	raw, ok := v[SettingBytes]
	if !ok || raw == "" {
		return defaultHead
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < smallestHead || n > largestHead {
		return defaultHead
	}
	return n
}

// zeroHead zeroes the opening bytes of a stream as it goes.
//
// Streaming rather than buffering, which keeps the regression line "a
// generator does not hold the whole file in memory" untouched: it holds one
// scratch buffer the size of the head, never the file.
type zeroHead struct {
	out io.Writer
	// left is how many bytes at the front are still to be zeroed.
	left int
	// touched records whether any byte we zeroed was not already zero. See
	// Stream.Touched for why the damage answers this rather than the engine.
	touched bool
	// buf is where the altered bytes go. The slice a caller hands to Write
	// belongs to the caller, so writing zeros into it in place would be
	// changing somebody else's memory - the same rule the archive encrypter
	// had to learn.
	buf []byte
}

func (z *zeroHead) Write(p []byte) (int, error) {
	if z.left == 0 {
		return z.out.Write(p)
	}

	n := z.left
	if n > len(p) {
		n = len(p)
	}
	for _, b := range p[:n] {
		if b != 0 {
			z.touched = true
			break
		}
	}

	z.buf = append(z.buf[:0], p...)
	for i := 0; i < n; i++ {
		z.buf[i] = 0
	}
	z.left -= n

	written, err := z.out.Write(z.buf)
	if err != nil {
		return written, err
	}
	// The caller is told about ITS bytes, not about ours. They are the same
	// count here because this damage preserves length, and saying len(p)
	// rather than written is what keeps that true for a caller that checks.
	return len(p), nil
}

func (z *zeroHead) Touched() bool { return z.touched }
