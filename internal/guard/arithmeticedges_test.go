package guard

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Free space too large to count comes back as the largest number rather than as
// a negative one.
//
// Both platform files ended in the same shape: a count the system reports
// unsigned, turned into an int64 and handed to the free space check. At eight
// exbibytes that conversion wraps, so the check would read the largest disk it
// will ever meet as smaller than any run and refuse to write to it.
//
// Nobody has such a disk, and that is the argument for how this is written
// rather than a reason to leave it alone. A condition inside the syscall is one
// nothing could ever turn red - this project has taken seven of those back out
// - so the arithmetic is a function that can be asked directly, and this asks
// it.
func TestFreeSpaceTooLargeToCountIsNotANegativeNumber(t *testing.T) {
	cases := []struct {
		blocks, size uint64
		want         int64
	}{
		{0, 4096, 0},
		{1, 4096, 4096},
		{1024, 4096, 4 << 20},
		// Where int64 stops, in blocks of four kilobytes.
		{math.MaxUint64 / 4096, 4096, math.MaxInt64},
		{math.MaxUint64, math.MaxUint64, math.MaxInt64},
		// A block size no filesystem reports. The answer is that we know
		// nothing about the space rather than that there is none, which is
		// what a failing syscall already says, and it is here so the overflow
		// check above does not divide by nought.
		{1024, 0, 0},
	}

	for _, c := range cases {
		got := core.AvailableFrom(c.blocks, c.size)
		if got < 0 {
			t.Errorf("AvailableFrom(%d, %d) is %d, and the free space check reads a negative as a disk too small for anything",
				c.blocks, c.size, got)
			continue
		}
		if got != c.want {
			t.Errorf("AvailableFrom(%d, %d) = %d, expected %d", c.blocks, c.size, got, c.want)
		}
	}
}

// A filler with nothing to add says so instead of spinning for ever.
//
// AppendFiller ends its loop only by adding bytes, so a vocabulary of empty
// strings never reaches the length asked for - and with a separator that
// returns nothing either, the loop neither ends nor grows. That is the one
// failure shape a size guard cannot see, because no file is ever produced to
// measure. FillRecords three functions up names exactly this and says so in a
// comment. Its sister did not, which made it a difference between two halves of
// one primitive rather than a hypothetical.
func TestAFillerWithNothingToAddRefusesRatherThanSpinning(t *testing.T) {
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		core.AppendFiller(nil, []string{"", ""}, 16, func(int) string { return "" })
	}()

	select {
	case v := <-done:
		if v == nil {
			t.Fatal("AppendFiller came back without refusing, so it either padded with nothing or lost the length it was asked for")
		}
		if said := fmt.Sprint(v); !strings.Contains(said, "empty") {
			t.Errorf("the refusal does not say what was wrong with the words it was handed: %s", said)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AppendFiller did not come back in five seconds, and its loop ends only by adding bytes")
	}
}

// A vocabulary that merely CONTAINS an empty word still pads to the byte.
//
// The refusal above has to be about the vocabulary having nothing at all to
// say, not about any one word being empty. One word with bytes in it ends the
// loop whatever the separator does, because every cycle through the vocabulary
// appends it - so a check per word would refuse something legal, and this is
// the guard that says so.
func TestAFillerWithOneRealWordAmongEmptyOnesStillPadsToTheByte(t *testing.T) {
	out := core.AppendFiller(nil, []string{"", "ab", ""}, 16, func(int) string { return "" })
	if len(out) != 16 {
		t.Fatalf("the padding came to %d B and 16 were asked for", len(out))
	}
}
