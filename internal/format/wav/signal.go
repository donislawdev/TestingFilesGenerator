package wav

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math"
	"math/rand/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What the sound IS, kept apart from how it reaches the disk.
//
// The split is by job rather than by size. wav.go works out how many frames fit
// in the requested number of bytes and writes them out. This file answers the
// one question that sits underneath: what is the value of frame n. Those change
// for different reasons - a new kind of content touches only this file, and a
// change to the padding chunk touches only the other one.

// sampler yields the value of one frame, in the range -1 to 1.
type sampler struct {
	content string
	base    float64
	rate    int
	rng     *rand.Rand

	// tone holds one full cycle of the steady tone, empty for everything else.
	tone []float64
	at   int
}

// newSampler prepares whatever the chosen content needs before the first frame.
func newSampler(m memo) *sampler {
	s := &sampler{
		content: m.content,
		// A fixed pitch, nudged by the seed so two seeds sound different.
		base: 220.0 + float64(m.seed%880),
		rate: m.rate,
		rng:  core.NewRand(m.seed),
	}
	// The condition mirrors the switch in next rather than naming the tone, so
	// a fifth kind of content added one day lands on the same branch in both
	// places instead of reading past the end of an empty table.
	switch m.content {
	case "silence", "noise", "sweep":
	default:
		s.tone = tonePeriod(m.rate, s.base, m.frames)
	}
	return s
}

// next returns the value of one frame and moves the sampler on.
func (s *sampler) next(frame int64) float64 {
	switch s.content {
	case "silence":
		return 0
	case "noise":
		return s.rng.Float64()*2 - 1
	case "sweep":
		// A sweep changes pitch as it goes, so it never repeats and there is
		// nothing to read back.
		t := float64(frame) / float64(s.rate)
		return math.Sin(2 * math.Pi * (s.base + s.base*t) * t)
	default:
		v := s.tone[s.at]
		if s.at++; s.at == len(s.tone) {
			s.at = 0
		}
		return v
	}
}

// tonePeriod works out one full cycle of the steady tone, so the rest of the
// file can read it back instead of asking for the same sine again.
//
// Measured 2026-09-06 on a 256 MB file: the sine was 542 ms of a 1495 ms run,
// and every call after the first cycle was recomputing a number the file
// already held. Reading it back instead costs 2.0 times less processor time
// and 1.7 times less wall clock, ranges disjoint on both.
//
// The tone at frame f is sin(2*pi*base*f/rate). Adding P to f moves that angle
// by 2*pi*base*P/rate, which is a whole number of turns exactly when
// P = rate/gcd(base, rate) - and base is a whole number of hertz, so that
// divides cleanly. The seed picks a pitch between 220 and 1099 Hz, which at the
// default rate makes the cycle anything from 42 frames to 44100.
//
// The table is never longer than the file needs, so a one kilobyte WAV does not
// reserve a table for sound it will never make. That clamp is about memory
// only. It is NOT what keeps short files identical - most of them wrap anyway,
// since 374 of the 880 pitches have a cycle shorter than an eight thousand
// frame file.
//
// D11: reading the table back is not bit for bit what recomputing gives, and
// the reason is rounding rather than mathematics. float64(frame)/float64(rate)
// and float64(frame % period)/float64(rate) are different arguments, and
// math.Sin reduces a large one differently. So the tone is now periodic by
// definition rather than periodic to within the last bit of a mantissa.
//
// Whether that reaches the file depends on the bit depth, because the gap is
// far smaller than the step between two neighbouring sample values. Measured
// 2026-09-06 rather than argued: at sixteen bits NOTHING moved across forty
// combinations of seed, size, rate and channel count, and at 24 and 32 bits 25
// of 30 combinations did. That is a measurement and not a proof - a sample
// sitting exactly on a rounding boundary would flip at any depth.
func tonePeriod(rate int, base float64, frames int64) []float64 {
	period := int64(rate) / gcd(int64(base), int64(rate))
	if frames < period {
		period = frames
	}
	table := make([]float64, period)
	for i := range table {
		table[i] = math.Sin(2 * math.Pi * base * (float64(i) / float64(rate)))
	}
	return table
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}
