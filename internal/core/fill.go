package core

import (
	"encoding/binary"
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
)

// Bulk random bytes are drawn eight at a time, and the two byte orders are two
// functions rather than one function with a flag.
//
// Eight at a time rather than one is the whole point. Measured 2026-09-06 over
// 64 MiB through a 32 KiB buffer, repetitions interleaved with the order
// reversed: one draw per byte runs at 182 MB/s, eight bytes per draw at
// 2499 MB/s. The write path of an archive is slower than the disk underneath it
// when it draws a byte at a time (P2 in PERFORMANCE-REVIEW-2026-09-05.md).
//
// The orders stay separate because picking the wrong one is not a style
// mistake - it silently rewrites every file a format has ever produced, which
// is exactly what D11 forbids. A boolean argument would put that mistake one
// typo away and would read the same in review either way. Two names cannot be
// confused by accident, and the compiler cannot help with a flag.
//
// Neither function draws anything for an empty slice, and both spend exactly
// one draw on a trailing group shorter than eight bytes. That is what the ten
// call sites did before they were folded into here, so folding them in moved no
// bytes except where the owner asked for them to move.

// FillRandomBE fills b with random bytes, most significant byte of each draw
// first. This is the order bmp, gif, ico, opc, png, tiff and - since 0.3.0 -
// targz, wav and zip write.
func FillRandomBE(b []byte, rng *rand.Rand) {
	i := 0
	for ; i+8 <= len(b); i += 8 {
		binary.BigEndian.PutUint64(b[i:], rng.Uint64())
	}
	if i < len(b) {
		var eight [8]byte
		binary.BigEndian.PutUint64(eight[:], rng.Uint64())
		copy(b[i:], eight[:])
	}
}

// FillRandomLE fills b with random bytes, least significant byte of each draw
// first. This is the order avif, jpg, jxl and webp write.
func FillRandomLE(b []byte, rng *rand.Rand) {
	i := 0
	for ; i+8 <= len(b); i += 8 {
		binary.LittleEndian.PutUint64(b[i:], rng.Uint64())
	}
	if i < len(b) {
		var eight [8]byte
		binary.LittleEndian.PutUint64(eight[:], rng.Uint64())
		copy(b[i:], eight[:])
	}
}
