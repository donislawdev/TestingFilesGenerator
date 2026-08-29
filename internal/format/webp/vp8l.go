// The VP8L bitstream, which is the lossless half of WebP.
//
// x/image/webp decodes and does not encode, so a WebP means writing this
// ourselves. The arrangement here is deliberately the dullest legal one the
// format allows, and the reason is the promise this tool makes rather than any
// property of WebP: a file has to come out at an exact size, so the size has to
// be arithmetic that can be inverted.
//
//   - no transforms, no colour cache, no meta Huffman, no backward references
//   - green, red and blue each get a complete code in which all 256 literals
//     are eight bits long, so the canonical code for a symbol IS that symbol
//     and a pixel always costs 24 bits
//   - alpha gets a one symbol code, which costs no bits per pixel at all
//
// A real encoder would compress. This one measures out three bytes a pixel on
// purpose, which is what lets a picture GROW to fill the bytes that were asked
// for instead of being a thumbnail followed by filler - the same shape as BMP
// and TIFF.
package webp

import "io"

// The order the format reads the nineteen code length symbols in.
var codeLengthOrder = [19]int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

const (
	// literals is how many symbols a colour channel can take.
	literals = 256
	// greenAlphabet carries the literals plus the length prefixes that a
	// backward reference would use. We never emit one, but the code still has
	// to declare a length for every symbol.
	greenAlphabet = literals + 24

	// codeLengthsUsed reaches index 13 of the order above, which is where the
	// symbol meaning "eight bits" sits. Anything shorter cannot say it.
	codeLengthsUsed = 14

	// headerBits is every bit before the first pixel. Written out as a sum so
	// that changing any part of the stream above forces this to be changed
	// with it, rather than leaving a number nobody can trace.
	headerBits = 8 + 14 + 14 + 1 + 3 + // signature, width, height, alpha hint, version
		3 + // no transform, no colour cache, no meta Huffman
		(1 + 4 + codeLengthsUsed*3 + 1 + greenAlphabet) + // green
		(1 + 4 + codeLengthsUsed*3 + 1 + literals) + // red
		(1 + 4 + codeLengthsUsed*3 + 1 + literals) + // blue
		(1 + 1 + 1 + 8) + // alpha, one symbol, eight bit
		(1 + 1 + 1 + 1) // distance, one symbol, one bit

	// bitsPerPixel is three channels of eight bits. Alpha costs nothing.
	bitsPerPixel = 24
)

// streamBytes is how long the bitstream is for a picture of this many pixels.
//
// The pixel part is a whole number of bytes, so the header is the only part
// that has to be rounded, and it is rounded once.
func streamBytes(pixels int64) int64 {
	return (headerBits+7)/8 + pixels*bitsPerPixel/8
}

// bitWriter writes least significant bit first, which is the order VP8L reads.
//
// It writes THROUGH to an io.Writer rather than collecting the file, because a
// generator here may be asked for gigabytes and the regression surface says
// none of them holds the whole file in memory. The first version of this
// package did collect it, with a comment explaining that a Huffman stream is
// not a sequence of whole bytes - which is true and is not a reason. The
// partial byte is one byte of state, not a file.
type bitWriter struct {
	to   io.Writer
	buf  []byte
	acc  uint64
	bits uint
	err  error
	// written counts the bytes handed to the writer, so the caller can check
	// the arithmetic that sized the chunk header it already sent.
	written int64
}

func newBitWriter(to io.Writer) *bitWriter {
	return &bitWriter{to: to, buf: make([]byte, 0, 32*1024)}
}

func (w *bitWriter) write(value uint32, n uint) {
	w.acc |= uint64(value&(1<<n-1)) << w.bits
	w.bits += n
	for w.bits >= 8 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc >>= 8
		w.bits -= 8
	}
	if len(w.buf) >= 32*1024 {
		w.drain()
	}
}

func (w *bitWriter) drain() {
	if w.err != nil || len(w.buf) == 0 {
		return
	}
	n, err := w.to.Write(w.buf)
	w.written += int64(n)
	w.buf = w.buf[:0]
	w.err = err
}

// flush empties the buffer and the partial byte, and reports the first error
// any write hit.
func (w *bitWriter) flush() (int64, error) {
	if w.bits > 0 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc, w.bits = 0, 0
	}
	w.drain()
	return w.written, w.err
}

// reverseByte returns the eight bits of v in the opposite order.
//
// Huffman codes travel most significant bit first through a stream that is
// otherwise least significant bit first, the same convention DEFLATE uses.
// With every literal eight bits long the canonical code for symbol s is s, so
// emitting one is emitting s backwards.
func reverseByte(v uint8) uint32 {
	var out uint32
	for i := uint(0); i < 8; i++ {
		out = out<<1 | uint32(v>>i)&1
	}
	return out
}

// writeFlatCode declares a code in which every literal is eight bits long and
// every symbol above the literals is unused.
//
// The code length alphabet needs two symbols to be a prefix code at all, so it
// carries 0 and 8 with one bit each. Symbol 0 sorts first and gets code 0,
// symbol 8 gets code 1.
func writeFlatCode(w *bitWriter, alphabet int) {
	w.write(0, 1) // not a simple code
	w.write(codeLengthsUsed-4, 4)
	for i := 0; i < codeLengthsUsed; i++ {
		length := uint32(0)
		if codeLengthOrder[i] == 0 || codeLengthOrder[i] == 8 {
			length = 1
		}
		w.write(length, 3)
	}
	w.write(0, 1) // a length is read for every symbol in the alphabet
	for s := 0; s < alphabet; s++ {
		if s < literals {
			w.write(1, 1) // symbol 8
		} else {
			w.write(0, 1) // symbol 0, unused
		}
	}
}

// writeSingleSymbolCode declares a code carrying one symbol, which then costs
// nothing to emit.
func writeSingleSymbolCode(w *bitWriter, symbol uint32, eightBit bool) {
	w.write(1, 1) // simple
	w.write(0, 1) // one symbol
	if eightBit {
		w.write(1, 1)
		w.write(symbol, 8)
	} else {
		w.write(0, 1)
		w.write(symbol, 1)
	}
}

// writeStream emits the whole bitstream. row is called once per row and hands
// back width*3 bytes in red, green, blue order.
func writeStream(w *bitWriter, width, height int, row func(y int) []byte) {
	w.write(0x2F, 8)
	w.write(uint32(width-1), 14)
	w.write(uint32(height-1), 14)
	w.write(0, 1) // no alpha in use
	w.write(0, 3) // version

	w.write(0, 1) // no transform
	w.write(0, 1) // no colour cache
	w.write(0, 1) // no meta Huffman

	writeFlatCode(w, greenAlphabet)
	writeFlatCode(w, literals)
	writeFlatCode(w, literals)
	writeSingleSymbolCode(w, 255, true) // alpha, opaque everywhere
	writeSingleSymbolCode(w, 0, false)  // distance, never used

	for y := 0; y < height; y++ {
		px := row(y)
		for x := 0; x < width; x++ {
			// Green first. That is the order the format reads a pixel in, and
			// getting it wrong produces a picture that decodes cleanly with
			// its channels swapped - which is why the guard compares pixels
			// rather than only opening the file.
			w.write(reverseByte(px[x*3+1]), 8)
			w.write(reverseByte(px[x*3]), 8)
			w.write(reverseByte(px[x*3+2]), 8)
		}
	}
}
