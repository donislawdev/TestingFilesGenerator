package guard

import (
	"bytes"
	"errors"
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
			w, streams, err := chain.Open(&out, size, 1)
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
	w, streams, err := chain.Open(&out, int64(len(source)), 1)
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
	w, streams, err := chain.Open(&out, int64(len(source)), 1)
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

// A cut keeps exactly the bytes it was told to, and tells the generator to
// stop on the write that crosses the line.
//
// The stop is what saves cutting 90% of a file from generating all of it, and
// nothing outside the damage can see whether it happened - the file comes out
// the same either way. So it is asked here, of the stream itself: the write
// that crosses the line takes only the bytes kept and answers ErrKeptEnough,
// and so does every write after it.
func TestACutKeepsItsBytesAndStopsTheGenerator(t *testing.T) {
	source := make([]byte, 100)
	for i := range source {
		source[i] = byte(i + 1)
	}
	var out bytes.Buffer
	chain := damage.Chain{{ID: damage.Truncate}}
	w, streams, err := chain.Open(&out, 30, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := w.Write(source[:20]); n != 20 || err != nil {
		t.Fatalf("a write inside the bytes kept gave %d, %v", n, err)
	}
	if streams[0].Touched() {
		t.Error("nothing was cut yet and the cut says it touched the file")
	}
	n, err := w.Write(source[20:])
	if n != 10 || !errors.Is(err, damage.ErrKeptEnough) {
		t.Errorf("the write crossing the line gave %d, %v - it should take the 10 B kept and tell the generator to stop", n, err)
	}
	if n, err := w.Write(source[:5]); n != 0 || !errors.Is(err, damage.ErrKeptEnough) {
		t.Errorf("a write after the line gave %d, %v", n, err)
	}
	if !bytes.Equal(out.Bytes(), source[:30]) {
		t.Errorf("the cut passed on %d B, and should have passed the first 30 B unchanged", out.Len())
	}
	if !streams[0].Touched() {
		t.Error("70 B were dropped and the cut says it touched nothing")
	}
}

// Junk is written after the file, once, and nothing before it moves.
func TestJunkComesAfterTheFileAndOnlyThere(t *testing.T) {
	source := []byte("a whole file, ending here")
	var out bytes.Buffer
	chain := damage.Chain{{ID: damage.AppendJunk, Values: damage.Values{damage.SettingBytes: "100"}}}
	size := int64(len(source) + 100)
	w, streams, err := chain.Open(&out, size, 7)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(source); err != nil {
		t.Fatal(err)
	}
	if out.Len() != len(source) || streams[0].Touched() {
		t.Fatal("junk arrived, or was claimed, before the file had ended")
	}
	if err := chain.Finish(streams); err != nil {
		t.Fatal(err)
	}
	if int64(out.Len()) != size || !bytes.Equal(out.Bytes()[:len(source)], source) {
		t.Errorf("after the junk the file is %d B, and should be the %d B before it and 100 B more", out.Len(), len(source))
	}
	if !streams[0].Touched() {
		t.Error("100 B were appended and the damage says it touched nothing")
	}
	// The same seed gives the same junk, a different one different junk.
	again := func(seed uint64) []byte {
		var b bytes.Buffer
		w, s, err := chain.Open(&b, size, seed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(source); err != nil {
			t.Fatal(err)
		}
		if err := chain.Finish(s); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if !bytes.Equal(again(7), out.Bytes()) || bytes.Equal(again(8), out.Bytes()) {
		t.Error("the junk does not follow the seed: the same seed has to give the same bytes and another seed others")
	}
}

// The floor of a cut follows its share, and a chain works its floor out from
// what each damage needs to leave.
//
// A cut has to drop at least four bytes, so the smallest file it can leave
// grows as the share shrinks: half gone needs 4 B left, one percent gone
// needs 298 B left before 1% of the whole is four bytes. In a chain, junk
// after a cut needs its own 64 B on top of what the cut leaves.
func TestTheFloorOfACutFollowsItsShareAndAChainAddsUp(t *testing.T) {
	for _, c := range []struct {
		chain damage.Chain
		want  int64
		owner string
	}{
		{damage.Chain{{ID: damage.Truncate}}, 4, damage.Truncate},
		{damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "1"}}}, 298, damage.Truncate},
		{damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "90"}}}, 1, damage.Truncate},
		{damage.Chain{{ID: damage.AppendJunk}}, 64, damage.AppendJunk},
		{damage.Chain{{ID: damage.Truncate}, {ID: damage.AppendJunk}}, 68, damage.Truncate},
		{damage.Chain{{ID: damage.ZeroHead}, {ID: damage.Truncate}}, 4, damage.Truncate},
	} {
		got, owner, err := c.chain.Floor()
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want || owner != c.owner {
			t.Errorf("%s has a floor of %d B set by %q, and should be %d B set by %q", c.chain, got, owner, c.want, c.owner)
		}
		if _, err := c.chain.Before(got); err != nil {
			t.Errorf("%s refuses its own floor of %d B: %v", c.chain, got, err)
		}
		if got > 0 {
			if _, err := c.chain.Before(got - 1); err == nil {
				t.Errorf("%s takes %d B, one under its floor", c.chain, got-1)
			}
		}
	}
}
