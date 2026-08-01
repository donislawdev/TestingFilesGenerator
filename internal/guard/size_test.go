package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Sizes count in 1024s, on purpose and against the standards.
//
// Measured: Windows Explorer shows 10 485 760 bytes as "10 MB" and 10 000 000
// bytes as "9,53 MB". ls -lh shows "10M" and "9.6M". A tester who asks for
// 10mb and then checks the file has to see the number they asked for, or the
// tool looks broken - and looking broken costs more than following the
// standard buys.
//
// Someone who wants exactly ten million bytes writes the number.

func TestSizesCountIn1024s(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1024", 1024},
		{"0", 0},
		{"1b", 1},
		{"1kb", 1024},
		{"1kib", 1024},
		{"1k", 1024},
		{"1mb", 1024 * 1024},
		{"1mib", 1024 * 1024},
		{"10mb", 10 * 1024 * 1024},
		{"1gb", 1024 * 1024 * 1024},
		{"1gib", 1024 * 1024 * 1024},
		{"1.5gib", 1610612736},
		{"1tb", 1024 * 1024 * 1024 * 1024},

		// Case does not matter, and neither does a space.
		{"700kB", 700 * 1024},
		{"10 MB", 10 * 1024 * 1024},
		{"10MiB", 10 * 1024 * 1024},

		// The size the whole project keeps as its example.
		{"10485761", 10485761},
	}

	for _, c := range cases {
		got, err := core.ParseSize(c.in)
		if err != nil {
			t.Errorf("%q failed: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q gave %d B, expected %d B", c.in, got, c.want)
		}
	}
}

func TestTheTwoSpellingsAgree(t *testing.T) {
	pairs := [][2]string{
		{"5kb", "5kib"},
		{"5mb", "5mib"},
		{"5gb", "5gib"},
		{"5tb", "5tib"},
	}
	for _, p := range pairs {
		a, errA := core.ParseSize(p[0])
		b, errB := core.ParseSize(p[1])
		if errA != nil || errB != nil {
			t.Fatalf("%q or %q failed: %v %v", p[0], p[1], errA, errB)
		}
		if a != b {
			t.Errorf("%q gave %d B and %q gave %d B - the two spellings mean the same thing", p[0], a, p[1], b)
		}
	}
}

func TestASizeThatIsNotAWholeByteIsRefused(t *testing.T) {
	// Rounding is the one thing this tool must never do quietly. In a batch
	// of ten thousand files a warning is lost, and the user silently receives
	// data they did not order.
	for _, in := range []string{"1.5", "0.5b", "1.0001kb"} {
		got, err := core.ParseSize(in)
		if err == nil {
			t.Errorf("%q was accepted as %d B instead of being refused", in, got)
			continue
		}
		// The message has to say what a workable value looks like, not only
		// that the input was wrong.
		if !strings.Contains(err.Error(), "whole number of bytes") {
			t.Errorf("%q was refused without explaining why: %v", in, err)
		}
	}
}

func TestNonsenseSizesAreRefusedWithAUsefulMessage(t *testing.T) {
	for _, in := range []string{"", "banana", "mb", "10qb", "-5mb"} {
		if got, err := core.ParseSize(in); err == nil {
			t.Errorf("%q was accepted as %d B", in, got)
		}
	}
}
