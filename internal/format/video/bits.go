package video

// bitWriter writes most significant bit first, which is how every AV1 header
// is laid out.
type bitWriter struct {
	buf []byte
	n   uint
}

func (w *bitWriter) bit(b uint32) {
	if w.n%8 == 0 {
		w.buf = append(w.buf, 0)
	}
	if b&1 != 0 {
		w.buf[len(w.buf)-1] |= 0x80 >> (w.n % 8)
	}
	w.n++
}

func (w *bitWriter) bits(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit(v >> uint(i))
	}
}

// align is byte_alignment: zero bits to the next byte.
func (w *bitWriter) align() {
	for w.n%8 != 0 {
		w.bit(0)
	}
}

// ns is a value below n in the specification's non-symmetric code,
// 04.conventions.md lines 456-486, with w = FloorLog2(n) + 1 and
// m = (1 << w) - n. A value under m takes w - 1 bits, one at m or over takes
// one more, so that v = (x + m) >> 1 and the extra bit together read back as x.
func (w *bitWriter) ns(x, n int) {
	floor := 0
	for 2<<floor <= n {
		floor++
	}
	m := (1 << (floor + 1)) - n
	if x < m {
		w.bits(uint32(x), floor)
		return
	}
	w.bits(uint32((x+m)>>1), floor)
	w.bit(uint32((x + m) & 1))
}

// trailing is trailing_bits: a one, then zeros to the next byte.
func (w *bitWriter) trailing() {
	w.bit(1)
	w.align()
}

func (w *bitWriter) bytes() []byte { return w.buf }

// bitReader reads the same way, and only over bytes it was given - reading
// past the end is a defect in the caller, reported rather than panicking.
type bitReader struct {
	b       []byte
	pos     uint
	overrun bool
}

func (r *bitReader) bit() uint32 {
	if r.pos/8 >= uint(len(r.b)) {
		r.overrun = true
		return 0
	}
	v := uint32(r.b[r.pos/8]>>(7-r.pos%8)) & 1
	r.pos++
	return v
}

func (r *bitReader) bits(n int) uint32 {
	var v uint32
	for range n {
		v = v<<1 | r.bit()
	}
	return v
}

// leb128 is the length an OBU header carries.
func leb128(v int) []byte { return appendLeb128(nil, v) }

// appendLeb128 appends it to out.
func appendLeb128(out []byte, v int) []byte {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

// readLeb128 is the value and how many bytes it took, or 0 bytes when the
// input ends inside it.
func readLeb128(b []byte) (value, used int) {
	for i := 0; i < 8 && i < len(b); i++ {
		value |= int(b[i]&0x7f) << (7 * i)
		if b[i]&0x80 == 0 {
			return value, i + 1
		}
	}
	return 0, 0
}
