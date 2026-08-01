package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseSize turns a human written size into an exact number of bytes.
//
//	10mb     -> 10 485 760
//	10mib    -> 10 485 760  (the same, spelled out)
//	1.5gib   -> 1 610 612 736
//	700kb    -> 716 800
//	10485761 -> 10 485 761
//
// Case does not matter. A plain number is a count of bytes.
//
// Every unit counts in 1024s, and that is a deliberate departure from the
// standards. The reason is what the person checking the file will see.
// Measured on Windows and on Linux: a file of 10 485 760 bytes is shown as
// "10 MB" by Windows Explorer and as "10M" by ls, while a file of 10 000 000
// bytes is shown as "9,53 MB" and "9.6M". Someone who asks for 10mb, gets ten
// million bytes and then looks at the file concludes the generator is broken
// - and for a tool whose whole promise is an exact size, that costs more than
// following the standard buys.
//
// Someone who genuinely wants ten million bytes writes the number.
//
// A size that does not land on a whole byte is refused rather than rounded.
// Rounding is the one thing this tool must never do quietly - a batch of ten
// thousand files would bury the warning and hand back data nobody asked for.
func ParseSize(s string) (int64, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, fmt.Errorf("empty size: write a number of bytes or a size such as 10mb, 1.5gib or 700kB")
	}

	digits := strings.TrimRight(raw, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ")
	unit := strings.ToLower(strings.TrimSpace(raw[len(digits):]))
	digits = strings.TrimSpace(digits)

	if digits == "" {
		return 0, fmt.Errorf("size %q has no number: write something like 10mb or 1048576", s)
	}

	multiplier, ok := unitBytes(unit)
	if !ok {
		return 0, fmt.Errorf("size %q uses an unknown unit %q: use b, kb, mb, gb, tb for thousands or kib, mib, gib, tib for 1024s", s, unit)
	}

	// A whole number with no unit, or with a unit, is the common case and
	// stays in integer arithmetic so nothing is lost on the way.
	if n, err := strconv.ParseInt(digits, 10, 64); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("size %q is negative: a file cannot be smaller than zero bytes", s)
		}
		if multiplier != 1 && n > math.MaxInt64/multiplier {
			return 0, fmt.Errorf("size %q is too large to express in bytes", s)
		}
		return n * multiplier, nil
	}

	f, err := strconv.ParseFloat(digits, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q is not a number: write something like 10mb, 1.5gib or 1048576", s)
	}
	if f < 0 {
		return 0, fmt.Errorf("size %q is negative: a file cannot be smaller than zero bytes", s)
	}

	exact := f * float64(multiplier)
	if exact > math.MaxInt64 {
		return 0, fmt.Errorf("size %q is too large to express in bytes", s)
	}
	n := int64(exact)
	if exact != math.Trunc(exact) {
		return 0, fmt.Errorf(
			"size %q is not a whole number of bytes: it works out to %.4f bytes. Ask for %d or %d instead",
			s, exact, n, n+1)
	}
	return n, nil
}

func unitBytes(unit string) (int64, bool) {
	const (
		ki = 1024
		mi = ki * 1024
		gi = mi * 1024
		ti = gi * 1024
	)
	// The two spellings mean the same thing. kib and mib exist because people
	// who know the difference reach for them, not because they behave
	// differently here.
	switch unit {
	case "", "b":
		return 1, true
	case "k", "kb", "ki", "kib":
		return ki, true
	case "m", "mb", "mi", "mib":
		return mi, true
	case "g", "gb", "gi", "gib":
		return gi, true
	case "t", "tb", "ti", "tib":
		return ti, true
	}
	return 0, false
}
