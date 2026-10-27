package guard

import (
	"math/big"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

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

// A size with a decimal point is read to the byte, not through a float.
//
// Until 2026-10-27 the decimal went through float64, which carries 53 bits, so
// a fraction too fine for it was rounded before anyone asked whether it was
// whole: 255.99999999999999B planned a file of 256 B, 1.0000000000000001kb one
// of 1024 B, and 9007199254740993.0B one a byte smaller than asked - three
// sizes the tool did not ask about, contrary to D6 and to RECIPE.md, which
// already said such a fraction is an error. The refusal it did give read
// "works out to 0.0000 bytes" for a value that is not nought.
// docs/STABILITY-REVIEW-2026-10-26.md, R17.
func TestASizeWithADecimalPointIsReadToTheByte(t *testing.T) {
	const largest = int64(9223372036854775807)
	for _, c := range []struct {
		in   string
		want int64
	}{
		{"9007199254740993.0B", 9007199254740993},
		{"9223372036854775807.0B", largest},
		{".5kb", 512},
		{"5.kb", 5120},
		{"+1.5kb", 1536},
		{"007.50kb", 7680},
		{"-0.0", 0},
		{"0.0kb", 0},
		{"8191.9990234375tb", (8191<<10 + 1023) << 30},
	} {
		got, err := core.ParseSize(c.in)
		if err != nil || got != c.want {
			t.Errorf("%q gave %d B and %v, expected %d B", c.in, got, err, c.want)
		}
	}

	for _, c := range []struct{ in, between string }{
		{"255.99999999999999B", "between 255 and 256"},
		{"1.0000000000000001kb", "between 1024 and 1025"},
		{"0.0000000000000000001tb", "between 0 and 1"},
		// Whole in its first sixty digits and not in its sixty first.
		{"0.5" + strings.Repeat("0", 59) + "1kb", "between 512 and 513"},
	} {
		got, err := core.ParseSize(c.in)
		if err == nil {
			t.Errorf("%.40q was accepted as %d B instead of being refused", c.in, got)
			continue
		}
		if !strings.Contains(err.Error(), c.between) {
			t.Errorf("%.40q was refused without naming the whole numbers around it (%s): %.300v", c.in, c.between, err)
		}
	}

	for _, in := range []string{"9223372036854775808.0B", "8388608.0tb", "8388607.9999999999999999999tb"} {
		if got, err := core.ParseSize(in); err == nil || !strings.Contains(err.Error(), "too large") {
			t.Errorf("%q gave %d B and %v, expected too large", in, got, err)
		}
	}

	// Four million digits on either side of the point, which a recipe can
	// carry. Exact arithmetic reads digits in quadratic time - 23 s for this
	// many on the machine of 2026-10-27 - so the answer is owed without
	// reading them all (untouchable rule 23).
	many := strings.Repeat("7", 4_000_000)
	answersAtOnce(t, many+".5b", "too large")
	answersAtOnce(t, "0."+many+"b", "between 0 and 1")
}

// answersAtOnce asks ParseSize and wants its refusal within five seconds.
func answersAtOnce(t *testing.T, in, want string) {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := core.ParseSize(in)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("a size of %d digits gave %.200v, expected %q", len(in), err, want)
		}
	case <-time.After(5 * time.Second):
		t.Errorf("a size of %d digits had no answer after five seconds", len(in))
	}
}

// Every whole number of bytes, written as the exact decimal of a unit, reads
// back as itself - below 2^53, where a float held it and the answer must not
// have moved, and above, where it did not and the answer was wrong.
func TestEveryExactDecimalOfAUnitReadsBackAsItself(t *testing.T) {
	units := []struct {
		name  string
		shift uint
	}{{"b", 0}, {"kb", 10}, {"mb", 20}, {"gb", 30}, {"tb", 40}}
	rng := rand.New(rand.NewPCG(27, 10))
	for _, u := range units {
		for i := 0; i < 2000; i++ {
			w := rng.Int64N(1 << 53)
			if i%2 == 1 {
				w = rng.Int64()
			}
			text := exactDecimal(w, u.shift) + u.name
			got, err := core.ParseSize(text)
			if err != nil || got != w {
				t.Fatalf("%q gave %d B and %v, expected %d B", text, got, err, w)
			}
		}
	}
}

// exactDecimal writes w / 2^shift with every digit it has, and always with a
// decimal point, so the value goes the way a fraction goes rather than the way
// a whole number does.
func exactDecimal(w int64, shift uint) string {
	whole := w >> shift
	rest := w - whole<<shift
	digits := new(big.Int).Mul(big.NewInt(rest), new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(shift)), nil)).String()
	if shift == 0 {
		return strconv.FormatInt(whole, 10) + ".0"
	}
	return strconv.FormatInt(whole, 10) + "." + strings.Repeat("0", int(shift)-len(digits)) + digits
}

func TestNonsenseSizesAreRefusedWithAUsefulMessage(t *testing.T) {
	for _, in := range []string{"", "banana", "mb", "10qb", "-5mb", "-1.5kb", "+-5.5", "+-1.5kb", "-+1.5kb", "--1.5"} {
		if got, err := core.ParseSize(in); err == nil {
			t.Errorf("%q was accepted as %d B", in, got)
		}
	}
}
