package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A length of time is written the way a person writes it and counted in whole
// milliseconds - 10s, 1m30s, 1h, 500ms. It is the value a video's duration is
// asked for in, and later a sound's (docs/WIDEO-2026-10-06.md section 12).
//
// The same promise as a size, for the same reason: exact or refused. A film
// asked for at 59.9 seconds that comes out at 60 is a test of the wrong limit,
// and nothing in a passing run would say so.

func TestDurationsCountInWholeMilliseconds(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0s", 0},
		{"1ms", 1},
		{"500ms", 500},
		{"1s", 1_000},
		{"10s", 10_000},
		{"59.9s", 59_900},
		{"0.1s", 100},
		{".5s", 500},
		{"1m", 60_000},
		{"1m30s", 90_000},
		{"1h", 3_600_000},
		{"1.5h", 5_400_000},
		{"1h30m", 5_400_000},
		{"2h3m4s5ms", 7_384_005},
		{"1h0.033s", 3_600_033},

		// Case does not matter, and a space between the parts is how people
		// write it by hand.
		{"10S", 10_000},
		{"1H 30M", 5_400_000},
		{"  1m 30s  ", 90_000},
		{"1 h", 3_600_000},
	}
	for _, c := range cases {
		got, err := core.ParseDuration(c.in)
		if err != nil {
			t.Errorf("%q was refused: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q gave %d ms, expected %d ms", c.in, got, c.want)
		}
	}
}

func TestEveryWayOfWritingOneLengthGivesTheSameMilliseconds(t *testing.T) {
	// The arithmetic is done on the digits, not through a float. 0.025h is
	// 90 000 ms exactly only if nothing on the way rounds.
	for _, group := range [][]string{
		{"90s", "1m30s", "1.5m", "90000ms", "0.025h", "1m 30s"},
		{"1h30m", "90m", "5400s", "1.5h", "5400000ms"},
		{"0.1s", "100ms", "0.100s"},
	} {
		first, err := core.ParseDuration(group[0])
		if err != nil {
			t.Fatalf("%q was refused: %v", group[0], err)
		}
		for _, other := range group[1:] {
			got, err := core.ParseDuration(other)
			if err != nil {
				t.Errorf("%q was refused: %v", other, err)
				continue
			}
			if got != first {
				t.Errorf("%q gave %d ms and %q gave %d ms - one length written two ways", group[0], first, other, got)
			}
		}
	}
}

func TestADurationThatIsNotAWholeMillisecondIsRefused(t *testing.T) {
	for _, in := range []string{"0.0001s", "1.0005s", "0.5ms", "0.0000001h"} {
		got, err := core.ParseDuration(in)
		if err == nil {
			t.Errorf("%q was accepted as %d ms instead of being refused", in, got)
			continue
		}
		if !strings.Contains(err.Error(), "whole millisecond") {
			t.Errorf("%q was refused without saying why: %v", in, err)
		}
	}
}

func TestNonsenseDurationsAreRefusedWithTheReason(t *testing.T) {
	// Each refusal names what is actually wrong. A bare number is the one that
	// matters most: reading 10 as ten seconds would be a guess, and so would
	// ten milliseconds.
	cases := []struct{ in, says string }{
		{"", "empty"},
		{"10", "no unit"},
		{"-5s", "not a length of time"},
		{"1,5s", "not a length of time"},
		{"5min", "unknown unit"},
		{"30s1m", "out of order"},
		{"1m1m", "out of order"},
		{"99999999999999999999h", "too long"},
	}
	for _, c := range cases {
		got, err := core.ParseDuration(c.in)
		if err == nil {
			t.Errorf("%q was accepted as %d ms", c.in, got)
			continue
		}
		if got != 0 {
			t.Errorf("%q was refused and still came back as %d ms", c.in, got)
		}
		if !strings.Contains(err.Error(), c.says) {
			t.Errorf("%q was refused as %q, which does not say %q", c.in, err.Error(), c.says)
		}
	}
}

func TestAWrittenDurationReadsBackAsTheSameMilliseconds(t *testing.T) {
	// FormatDuration is what a refusal and a window show, and a value shown
	// to somebody has to be one they can type back in.
	values := []int64{0, 1, 33, 999, 1_000, 1_033, 59_999, 60_000, 90_500, 3_599_999, 3_600_000, 3_600_033, 86_400_000, 1<<40 + 7}
	for v := int64(0); v < 200_000; v += 7 {
		values = append(values, v)
	}
	for _, v := range values {
		written := core.FormatDuration(v)
		got, err := core.ParseDuration(written)
		if err != nil {
			t.Errorf("%d ms was written as %q, which is refused: %v", v, written, err)
			continue
		}
		if got != v {
			t.Errorf("%d ms was written as %q, which reads back as %d ms", v, written, got)
		}
	}
	for v, want := range map[int64]string{0: "0s", 500: "500ms", 10_000: "10s", 90_000: "1m30s", 3_600_000: "1h", 1_033: "1.033s"} {
		if got := core.FormatDuration(v); got != want {
			t.Errorf("%d ms was written as %q, expected the shortest form %q", v, got, want)
		}
	}
}

// A setting of this kind takes a length inside its range, refuses one outside
// it or one it cannot read, and says what it takes in the same words empty or
// refused, on both surfaces.
//
// Asked of a declaration made up here, because on the day the kind arrived no
// format declared one yet - the registry-wide guards (the window against tfg
// formats, empty against refused) take over the moment one does, and this one
// keeps the kind covered if that format ever goes.
func TestADurationSettingTakesALengthInsideItsRangeAndSaysSo(t *testing.T) {
	p := format.Property{Name: "duration", Kind: format.PropertyDuration, Min: 33, Max: 86_400_000, Default: "10s"}

	for _, ok := range []string{"10s", "33ms", "24h", "1m30s", "59.9s"} {
		if bad := p.Allows(ok); !bad.IsZero() {
			t.Errorf("%q is inside 33ms to 24h and was refused: %s", ok, bad)
		}
	}
	const want = "a length of time from 33ms to 24h, such as 10s, 1m30s or 500ms"
	if got := p.Allowed(); got != want+", default 10s" {
		t.Errorf("an empty field says %q, expected %q", got, want+", default 10s")
	}
	for _, bad := range []string{"32ms", "24h1ms", "10", "banana", "-1s"} {
		why := p.Allows(bad)
		if why.IsZero() {
			t.Errorf("%q is outside 33ms to 24h or not a length and was accepted", bad)
			continue
		}
		if why.String() != "it takes "+want {
			t.Errorf("%q was refused as %q, and an empty field says %q - one setting described two ways", bad, why.String(), want)
		}
	}

	if got := text.AllowedWholeNumber(); got != "whole number" {
		t.Fatalf("this process speaks another language (%q), so nothing here is English to compare", got)
	}
	if got := parts.Allowed(p); got != p.Allowed() {
		t.Errorf("the window says %q and tfg formats says %q", got, p.Allowed())
	}
	unbounded := format.Property{Name: "duration", Kind: format.PropertyDuration}
	if got := parts.Allowed(unbounded); got != unbounded.Allowed() || got != "a length of time such as 10s, 1m30s or 500ms" {
		t.Errorf("without a range the window says %q and tfg formats says %q", got, unbounded.Allowed())
	}
}
