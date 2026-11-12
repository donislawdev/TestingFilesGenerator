package format

import (
	"io"
	"math/bits"
)

// PaddingInFront is how much padding goes in front of a file's content so that
// a cut after its first cutAt bytes keeps the same share of the content as of
// the whole file (Request.CutAt): cutAt, less the before bytes that stand in
// front of that padding, less content*cutAt/whole rounded down.
//
// It is the answer before a format rounds it to what it can write, and zero or
// less means that even with no padding in front the cut keeps no more than the
// share. Each format that splits its padding around its content for a cut asks
// this, so the share is one rule (docs/TRUNCATE-2026-11-11.md sections 7.4 and
// 8.5).
//
// content*cutAt is worked out in 128 bits, because a large picture times a cut
// near the top of what a file can be passes an int64. The quotient fits as
// long as 0 < cutAt < whole and content <= whole, which the caller sees to.
func PaddingInFront(before, content, cutAt, whole int64) int64 {
	hi, lo := bits.Mul64(uint64(content), uint64(cutAt))
	kept, _ := bits.Div64(hi, lo, uint64(whole))
	return cutAt - before - int64(kept)
}

// TailHolder passes a stream on while keeping its last Keep bytes back, so a
// format can write something in front of them once the stream ends - padding
// in front of the end marker an encoder writes last.
type TailHolder struct {
	W    io.Writer
	Keep int
	// Tail is the bytes held back, Keep of them once that many have come.
	Tail []byte
	// Written is how many bytes went on to W.
	Written int64
}

func (t *TailHolder) Write(p []byte) (int, error) {
	n := len(p)
	buf := append(t.Tail, p...)
	if len(buf) > t.Keep {
		emit := buf[:len(buf)-t.Keep]
		if _, err := t.W.Write(emit); err != nil {
			return 0, err
		}
		t.Written += int64(len(emit))
		buf = buf[len(buf)-t.Keep:]
	}
	t.Tail = append([]byte(nil), buf...)
	return n, nil
}
