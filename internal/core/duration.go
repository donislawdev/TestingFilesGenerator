package core

import (
	"math"
	"strconv"
	"strings"
)

// ParseDuration turns a written length of time into an exact number of
// milliseconds.
//
//	10s      -> 10 000
//	1m30s    -> 90 000
//	1h       -> 3 600 000
//	1.5s     -> 1 500
//	500ms    -> 500
//	1h 30m   -> 5 400 000
//
// The units are h, m, s and ms, and case does not matter. A length written in
// several parts goes from the largest unit down, each unit once, so 1h30m and
// 1m30s read the way a clock does. A space between the parts is allowed,
// because that is how people write it by hand.
//
// A number with no unit is refused rather than read as seconds. Ten could be
// ten seconds or ten milliseconds, and the tool picking one is exactly the
// guess this type exists to remove - the same reason a size refuses 1e5 and
// 0x10. Sizes can take a bare number because bytes are the only thing a size
// counts, and a length of time has no such single unit.
//
// A length that does not land on a whole millisecond is refused rather than
// rounded, as a size that does not land on a whole byte is. A video or a sound
// built on a rounded length would carry a duration nobody asked for.
//
// The arithmetic is done on the digits rather than through a float, so 0.1s is
// a hundred milliseconds exactly and not whatever 0.1 times a thousand happens
// to come to.
func ParseDuration(s string) (int64, error) {
	raw := strings.ToLower(strings.TrimSpace(s))
	if raw == "" {
		return 0, Refuse(Says("core.DurationEmpty", "an empty length of time: write one such as 10s, 1m30s or 500ms"))
	}

	var total int64
	previous := -1
	rest := raw
	for rest != "" {
		rest = strings.TrimLeft(rest, " ")
		number, afterNumber := leadingDecimal(rest)
		rest = strings.TrimLeft(afterNumber, " ")
		unit, afterUnit := leadingLetters(rest)
		rest = afterUnit

		// A number followed by something that is neither a unit nor the end -
		// 1,5s or -5s - is a spelling, not a missing unit, and is called one.
		if number == "" || (unit == "" && rest != "") {
			return 0, Refuse(Says("core.DurationNotALength",
				"%q is not a length of time: write one such as 10s, 1m30s or 500ms. A sign, an exponent or a comma is refused rather than guessed at", A("Value", s)))
		}
		if unit == "" {
			return 0, Refuse(Says("core.DurationNoUnit",
				"%q has no unit: write 10s for seconds or 10ms for milliseconds, because a bare number could be either", A("Value", s)))
		}
		index, per, ok := durationUnit(unit)
		if !ok {
			return 0, Refuse(Says("core.DurationUnknownUnit",
				"%q uses an unknown unit %q: use h, m, s or ms", A("Value", s), A("Unit", unit)))
		}
		if index <= previous {
			return 0, Refuse(Says("core.DurationOutOfOrder",
				"%q writes a unit twice or out of order: write the larger units first, each once, as 1h30m or 1m30s", A("Value", s)))
		}
		previous = index

		part, err := wholeMilliseconds(s, number, per)
		if err != nil {
			return 0, err
		}
		if total > math.MaxInt64-part {
			return 0, durationTooLong(s)
		}
		total += part
	}
	return total, nil
}

// FormatDuration writes a number of milliseconds the way ParseDuration reads
// it, in the shortest form a person would write: 10s, 1m30s, 1h, 500ms,
// 1.033s. ParseDuration of the result is the number again, which a guard holds
// for every value it tries.
func FormatDuration(ms int64) string {
	if ms <= 0 {
		return "0s"
	}
	hours := ms / 3_600_000
	minutes := ms % 3_600_000 / 60_000
	rest := ms % 60_000

	var b strings.Builder
	if hours > 0 {
		b.WriteString(strconv.FormatInt(hours, 10) + "h")
	}
	if minutes > 0 {
		b.WriteString(strconv.FormatInt(minutes, 10) + "m")
	}
	switch {
	case rest == 0:
	case rest%1000 == 0:
		b.WriteString(strconv.FormatInt(rest/1000, 10) + "s")
	case rest < 1000 && hours == 0 && minutes == 0:
		b.WriteString(strconv.FormatInt(rest, 10) + "ms")
	default:
		fraction := strings.TrimRight(strconv.FormatInt(1000+rest%1000, 10)[1:], "0")
		b.WriteString(strconv.FormatInt(rest/1000, 10) + "." + fraction + "s")
	}
	return b.String()
}

// durationUnit is where a unit stands among the four, largest first, and how
// many milliseconds one of it is.
func durationUnit(unit string) (index int, per int64, ok bool) {
	switch unit {
	case "h":
		return 0, 3_600_000, true
	case "m":
		return 1, 60_000, true
	case "s":
		return 2, 1_000, true
	case "ms":
		return 3, 1, true
	}
	return 0, 0, false
}

// leadingDecimal splits off digits with at most one decimal point. Anything
// else - a sign, an exponent, a comma - ends the number, and the caller then
// meets it where a unit should be.
func leadingDecimal(s string) (number, rest string) {
	i, dot := 0, false
	for i < len(s) {
		c := s[i]
		if c >= '0' && c <= '9' {
			i++
			continue
		}
		if c == '.' && !dot {
			dot = true
			i++
			continue
		}
		break
	}
	number = s[:i]
	if strings.Trim(number, ".") == "" {
		// A lone point is not a number, and saying so here keeps the message
		// the one about numbers rather than one about units.
		return "", s
	}
	return number, s[i:]
}

func leadingLetters(s string) (letters, rest string) {
	i := 0
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		i++
	}
	return s[:i], s[i:]
}

// wholeMilliseconds is one part of a length, worked out on the digits.
func wholeMilliseconds(s, number string, per int64) (int64, error) {
	whole, fraction, _ := strings.Cut(number, ".")
	fraction = strings.TrimRight(fraction, "0")

	var n int64
	if whole != "" {
		w, err := strconv.ParseInt(whole, 10, 64)
		if err != nil || w > math.MaxInt64/per {
			return 0, durationTooLong(s)
		}
		n = w * per
	}
	if fraction == "" {
		return n, nil
	}

	// Nine digits after the point is past anything a millisecond can come
	// out of: the largest unit is an hour, 36 times ten to the fifth
	// milliseconds, so a fraction whose last digit is not zero lands on a
	// whole millisecond only within seven places.
	notWhole := Refuse(Says("core.DurationNotWhole",
		"%q does not land on a whole millisecond: write it to the millisecond, as 1.5s or 1500ms", A("Value", s)))
	if len(fraction) > 9 {
		return 0, notWhole
	}
	f, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, notWhole
	}
	scale := int64(1)
	for range len(fraction) {
		scale *= 10
	}
	if f*per%scale != 0 {
		return 0, notWhole
	}
	part := f * per / scale
	if n > math.MaxInt64-part {
		return 0, durationTooLong(s)
	}
	return n + part, nil
}

func durationTooLong(s string) error {
	return Refuse(Says("core.DurationTooLong", "%q is too long to count in milliseconds", A("Value", s)))
}
