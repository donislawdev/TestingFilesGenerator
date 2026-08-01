package core

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"strconv"
)

// Seeds are derived, never consumed from a shared stream.
//
//	seed(target) = H(global seed, target id)
//	seed(file)   = H(seed(target), index)
//
// The naive alternative is one random stream that every target draws from in
// order. Adding a single file at the top then shifts the bytes of everything
// below it, every hash in someone's CI goes red at once, and after the second
// time that happens the team stops trusting the tool and goes back to keeping
// binaries in the repository - which is the problem this is here to solve.
//
// Deriving instead buys these for free: reordering targets changes nothing,
// adding or removing a target changes nothing for the others, and raising a
// count leaves the earlier files byte for byte identical.

// TargetSeed derives the seed of one target from the run seed and the target
// id. The id is the identity of the target, so changing it changes every file
// of that target on purpose.
func TargetSeed(runSeed int64, targetID string) uint64 {
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(runSeed))
	h.Write(buf[:])
	h.Write([]byte{0})
	h.Write([]byte(targetID))
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

// FileSeed derives the seed of one file from its target seed and its index.
func FileSeed(targetSeed uint64, index int) uint64 {
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], targetSeed)
	h.Write(buf[:])
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(index)))
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}

// NewRand returns the random source for one file.
//
// It is deterministic given the seed and it is not shared between files. A
// generator that reaches for a global source would make its output depend on
// the order files happen to be produced in, which breaks the promise that the
// same recipe gives the same bytes.
func NewRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
}

// SeedLabel renders a seed the way it appears in the self describing label
// and in file names - short enough to read off a screenshot.
func SeedLabel(seed uint64) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = hexDigits[seed&0xf]
		seed >>= 4
	}
	return string(out)
}
