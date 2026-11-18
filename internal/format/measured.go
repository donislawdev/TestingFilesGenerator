package format

// Measured is what one walk to the smallest file of a format has measured,
// kept for the rounds after it.
//
// The walk (Descriptor.smallest) asks Plan at nought, takes the minimum the
// refusal names and asks again there. A picture with its width and height set
// encodes itself in Plan to know its length, and its label does not carry the
// size, so both rounds encoded the same picture: two encodes where one would
// do, 94% of planning the picture-sizes preset (docs/SMALLEST-ONE-ENCODE-2026-11-18.md).
// The second round now takes what the first measured.
//
// One per walk, made by the walk, and gone with it. A walk is one goroutine -
// the files of a batch plan side by side, each with a walk of its own - so
// nothing here is shared and nothing needs a lock. A request outside a walk
// carries none, and Measure then measures every time, as every plan did before.
type Measured struct {
	kept []measurement
}

// measurement is one value measured for one key.
type measurement struct {
	of, is any
}

// Measure is measure(of), from what an earlier round of the same walk kept for
// an equal of, or measured now and kept for the rounds after. m may be nil.
//
// of has to hold everything measure reads, and that is what makes a value kept
// for it safe to hand back: the same function of the same input. So measure is
// a function of the format's package named here, never a closure that could
// read something of not holding - internal/guard holds every call to that.
// A format whose plan writes the bytes it measured (avif, jxl, ico) would
// otherwise write another picture without a word.
//
// A failed measurement is not kept, so the next round tries again and fails
// the same way.
func Measure[K comparable, V any](m *Measured, of K, measure func(K) (V, error)) (V, error) {
	if v, ok := recall[V](m, of); ok {
		return v, nil
	}
	v, err := measure(of)
	if err != nil || m == nil {
		return v, err
	}
	m.kept = append(m.kept, measurement{of: of, is: v})
	return v, nil
}

// recall is what m kept for an of equal to this one, if it kept anything.
// A nil m kept nothing.
func recall[V any](m *Measured, of any) (V, bool) {
	var none V
	if m == nil {
		return none, false
	}
	for _, k := range m.kept {
		if v, ok := k.is.(V); ok && k.of == of {
			return v, true
		}
	}
	return none, false
}
