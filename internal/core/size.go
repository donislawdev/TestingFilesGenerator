package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseSize turns a human written size into an exact number of bytes.
//
// Decimal units count in thousands and binary units count in 1024s, the way
// the standards define them:
//
//	10mb   -> 10 000 000
//	10mib  -> 10 485 760
//	1.5gib -> 1 610 612 736
//	700kB  -> 700 000
//	10485761 -> 10 485 761
//
// Case does not matter. A plain number is a count of bytes.
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
		k = 1000
		m = k * 1000
		g = m * 1000
		t = g * 1000

		ki = 1024
		mi = ki * 1024
		gi = mi * 1024
		ti = gi * 1024
	)
	switch unit {
	case "", "b":
		return 1, true
	case "k", "kb":
		return k, true
	case "m", "mb":
		return m, true
	case "g", "gb":
		return g, true
	case "t", "tb":
		return t, true
	case "ki", "kib":
		return ki, true
	case "mi", "mib":
		return mi, true
	case "gi", "gib":
		return gi, true
	case "ti", "tib":
		return ti, true
	}
	return 0, false
}
