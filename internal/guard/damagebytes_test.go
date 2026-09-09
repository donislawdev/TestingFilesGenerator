package guard

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
)

// The byte layer of damage, asked directly.
//
// These are the narrowest tests in the feature and they are here rather than
// left to the end to end ones on purpose: a damage that writes the wrong bytes
// still produces a file an oracle refuses, so the guard that asks "is it
// refused" would stay green while the damage did something other than what it
// says. What it DOES has to be pinned separately from what it ACHIEVES.

// zero-head zeroes exactly the bytes it says and leaves the rest alone.
//
// The stream is written in pieces of three, because that is the shape the real
// caller has - a generator writes in whatever chunks it likes, and a damage
// that only works when the head arrives in one write is a damage that works in
// a test and not in the product.
func TestZeroHeadZeroesTheBytesItSaysAndNoOthers(t *testing.T) {
	for _, head := range []int{4, 8, 16} {
		t.Run(strconv.Itoa(head), func(t *testing.T) {
			const size = 64
			source := make([]byte, size)
			for i := range source {
				source[i] = byte(i + 1)
			}

			var out bytes.Buffer
			chain := damage.Chain{{
				ID:     damage.ZeroHead,
				Values: damage.Values{damage.SettingBytes: strconv.Itoa(head)},
			}}
			w, streams, err := chain.Open(&out)
			if err != nil {
				t.Fatalf("opening the chain: %v", err)
			}

			for i := 0; i < len(source); i += 3 {
				end := min(i+3, len(source))
				if n, err := w.Write(source[i:end]); err != nil || n != end-i {
					t.Fatalf("writing bytes %d to %d gave %d, %v", i, end, n, err)
				}
			}

			got := out.Bytes()
			if len(got) != size {
				t.Fatalf("zero-head changed the length: %d B in, %d B out", size, len(got))
			}
			for i := 0; i < head; i++ {
				if got[i] != 0 {
					t.Errorf("byte %d is %#x and should have been zeroed", i, got[i])
				}
			}
			for i := head; i < size; i++ {
				if got[i] != source[i] {
					t.Errorf("byte %d is %#x and the source had %#x - past the head nothing may move",
						i, got[i], source[i])
				}
			}
			if !streams[0].Touched() {
				t.Error("every one of those bytes was non zero and the damage says it touched nothing")
			}
		})
	}
}

// A damage that changed nothing says so.
//
// This is the half the whole feature stands on. A file whose head is already
// zero comes out identical to a good one, every reader accepts it, and the
// manifest would call it broken - the tool lying in the one place its value
// lives. The engine refuses the run on this answer, so the answer has to be
// right in both directions, and the case above is the other direction.
func TestADamageThatMovedNothingSaysSo(t *testing.T) {
	source := make([]byte, 32) // all zero already

	var out bytes.Buffer
	chain := damage.Chain{{ID: damage.ZeroHead}}
	w, streams, err := chain.Open(&out)
	if err != nil {
		t.Fatalf("opening the chain: %v", err)
	}
	if _, err := w.Write(source); err != nil {
		t.Fatalf("writing: %v", err)
	}

	if streams[0].Touched() {
		t.Error("the head was already zero and the damage claims it changed something")
	}
	if idle := chain.Idle(streams); idle != damage.ZeroHead {
		t.Errorf("the chain names %q as the idle damage rather than %q", idle, damage.ZeroHead)
	}
	if !bytes.Equal(out.Bytes(), source) {
		t.Error("nothing was supposed to change and the bytes moved")
	}
}

// The floor is the smallest file the damage can be given, and it follows the
// parameter rather than being a constant.
//
// It has two jobs and that is why one number does both: it is what the plan
// refuses below, and it is the size the witness matrix is measured at. A floor
// that ignored the parameter would answer for one of those and not the other.
func TestTheFloorFollowsThePairOfSettings(t *testing.T) {
	for _, c := range []struct {
		head string
		want int64
	}{
		{"", 8}, // the declared default
		{"4", 4},
		{"64", 64},
	} {
		chain := damage.Chain{{
			ID:     damage.ZeroHead,
			Values: damage.Values{damage.SettingBytes: c.head},
		}}
		got, owner, err := chain.Floor()
		if err != nil {
			t.Fatalf("asking the floor for head %q: %v", c.head, err)
		}
		if got != c.want {
			t.Errorf("head %q has a floor of %d B and should be %d B", c.head, got, c.want)
		}
		if owner != damage.ZeroHead {
			t.Errorf("the floor is credited to %q rather than to the damage that set it", owner)
		}
	}
}

// A chain applies its damages in the order it lists them.
//
// Order is part of the contract because two damages can produce different
// bytes in different orders, and D11 promises those bytes do not move. Asked
// with two zero-heads of different lengths, where the wrong order is visible
// in the result: the longer one first leaves sixteen zeros, the shorter one
// first leaves the same sixteen only if the second really ran after it.
func TestAChainAppliesItsDamagesInTheOrderItListsThem(t *testing.T) {
	source := make([]byte, 32)
	for i := range source {
		source[i] = 0xFF
	}

	var out bytes.Buffer
	chain := damage.Chain{
		{ID: damage.ZeroHead, Values: damage.Values{damage.SettingBytes: "4"}},
		{ID: damage.ZeroHead, Values: damage.Values{damage.SettingBytes: "16"}},
	}
	w, streams, err := chain.Open(&out)
	if err != nil {
		t.Fatalf("opening the chain: %v", err)
	}
	if _, err := w.Write(source); err != nil {
		t.Fatalf("writing: %v", err)
	}

	got := out.Bytes()
	for i := 0; i < 16; i++ {
		if got[i] != 0 {
			t.Fatalf("byte %d is %#x - the second damage did not run after the first", i, got[i])
		}
	}
	if got[16] != 0xFF {
		t.Errorf("byte 16 is %#x and nothing should have reached it", got[16])
	}

	// The second one meets a head whose first four bytes are already zero and
	// twelve that are not, so it still moved something. The first met all
	// ones. Both have to say they were busy, because the idle check runs per
	// damage rather than once at the end.
	for i, s := range streams {
		if !s.Touched() {
			t.Errorf("damage %d says it changed nothing and it had bytes to change", i)
		}
	}
	if idle := chain.Idle(streams); idle != "" {
		t.Errorf("the chain names %q as idle and neither damage was", idle)
	}
}
