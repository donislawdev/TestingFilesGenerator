package core

import (
	"math"
	"math/big"
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
		return 0, RefuseAbout(SettingSize, Says("core.SizeEmpty", "empty {setting}: write a number of bytes or a size such as 10mb, 1.5gib or 700kB"))
	}

	digits := strings.TrimRight(raw, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ")
	unit := strings.ToLower(strings.TrimSpace(raw[len(digits):]))
	digits = strings.TrimSpace(digits)

	if digits == "" {
		return 0, RefuseAbout(SettingSize, Says("core.SizeNoNumber", "{setting} %q has no number: write something like 10mb or 1048576", A("Value", s)))
	}

	multiplier, ok := unitBytes(unit)
	if !ok {
		// Every unit here counts in 1024s, kb and kib alike. An earlier version
		// of this sentence said kb was for thousands, which is the opposite of
		// what the tool does and appeared in the one place somebody has
		// already got their units wrong.
		return 0, RefuseAbout(SettingSize, Says("core.SizeUnknownUnit", "{setting} %q uses an unknown unit %q: use b, kb, mb, gb or tb, all counting in 1024s, and kib, mib, gib or tib for the same thing spelled out", A("Value", s), A("Unit", unit)))
	}

	// A whole number with no unit, or with a unit, is the common case and
	// stays in integer arithmetic so nothing is lost on the way.
	if n, err := strconv.ParseInt(digits, 10, 64); err == nil {
		if n < 0 {
			return 0, sizeNegative(s)
		}
		if multiplier != 1 && n > math.MaxInt64/multiplier {
			return 0, sizeTooLarge(s)
		}
		return n * multiplier, nil
	}

	// Only the spellings a person writes. ParseFloat, which read these until
	// 2026-10-27, also took exponents, hex floats and the words for infinity,
	// and none of those describes a file.
	//
	// Measured on 2026-08-03: "--size 1e5" quietly meant 100000 bytes, while
	// the recipe refuses "1_000" and "0x10" outright, with the reasoning that
	// guessing at a spelling is the behaviour that type was written to remove.
	// Two doors into one idea applying opposite rules, and the difference was
	// nobody's decision - it fell out of ParseFloat being permissive. In a size
	// field, "1e5" is far likelier to be a typo than an intention.
	//
	// The decimal point stays, because 1.5gib is a real thing people write.
	if !plainDecimal(digits) {
		return 0, RefuseAbout(SettingSize, Says("core.SizeNotPlain",
			"{setting} %q is not a plain number: write the digits out, as 10mb, 1.5gib or 1048576. Spellings such as 1e5, 0x10 or 1_000 are refused rather than guessed at, the same as in a recipe", A("Value", s)))
	}
	return parseDecimal(s, digits, multiplier)
}

// parseDecimal is ParseSize for a number with a decimal point, s being what
// was written and digits its number, already known to be plain.
func parseDecimal(s, digits string, multiplier int64) (int64, error) {
	negative, whole, fraction := splitDecimal(digits)
	if negative && (whole != "" || fraction != "") {
		return 0, sizeNegative(s)
	}
	// Twenty digits before the point is ten to the nineteenth at the least,
	// already past the largest int64 before any unit multiplies it.
	if len(whole) > 19 {
		return 0, sizeTooLarge(s)
	}
	n, exact := decimalBytes(whole, fraction, multiplier)
	if !n.IsInt64() || (!exact && n.Int64() == math.MaxInt64) {
		return 0, sizeTooLarge(s)
	}
	if !exact {
		return 0, RefuseAbout(SettingSize, Says("core.SizeNotWhole",
			"{setting} %q is not a whole number of bytes: it falls between %d and %d. Ask for one of the two instead",
			A("Value", s), A("Below", n.Int64()), A("Above", n.Int64()+1)))
	}
	return n.Int64(), nil
}

// splitDecimal takes a plain decimal apart into its sign, the digits before the
// point without the zeros that lead them and the digits after it without the
// zeros that end them - so a part that is nought is empty.
func splitDecimal(digits string) (negative bool, whole, fraction string) {
	negative = strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(strings.TrimPrefix(digits, "+"), "-")
	whole, fraction, _ = strings.Cut(digits, ".")
	return negative, strings.TrimLeft(whole, "0"), strings.TrimRight(fraction, "0")
}

// fractionDigits is how many digits after the point decimalBytes reads.
//
// A fraction lands on a whole byte only when the unit cancels its
// denominator, and the largest unit is 2^40, which cancels 2^40 of the 10^k
// and none of the 5^k - so the fraction has to be a multiple of 5^k with k at
// most 40 once its trailing zeros are gone. A longer one is never whole, and
// only the byte below it is still asked for. That byte is decided by the
// first sixty digits: what they leave over is a multiple of the unit short of
// the next byte, and every digit after them adds less than the unit times
// 10^-60. Reading no further keeps the cost the same for a fraction of sixty
// digits and for one of a million, which is what an argument or a recipe
// written to be hostile can send (untouchable rule 23).
const fractionDigits = 60

// decimalBytes is whole.fraction times the unit, to the byte below when it is
// not a whole number of bytes, and whether it is.
//
// Until 2026-10-27 this went through float64, which carries 53 bits, and a
// fraction too fine for it was rounded before anyone asked whether it was
// whole: 255.99999999999999B became 256 B, 1.0000000000000001kb 1024 B and
// 9007199254740993.0B a byte smaller than asked, each without a word (D6).
// Exact arithmetic on the digits is the only way the question is answered
// about the number somebody wrote rather than about its nearest float.
func decimalBytes(whole, fraction string, unit int64) (*big.Int, bool) {
	long := len(fraction) > fractionDigits
	if long {
		fraction = fraction[:fractionDigits]
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction))), nil)
	// Both parts are digits only - plainDecimal let nothing else through - and
	// both are empty when the number is nought.
	n := new(big.Int)
	if both := whole + fraction; both != "" {
		n.SetString(both, 10)
	}
	n.Mul(n, big.NewInt(unit))
	rest := new(big.Int)
	n.QuoRem(n, scale, rest)
	return n, !long && rest.Sign() == 0
}

// sizeNegative and sizeTooLarge are two refusals ParseSize gives from two
// places each.
func sizeNegative(s string) error {
	return RefuseAbout(SettingSize, Says("core.SizeNegative", "{setting} %q is negative: a file cannot be smaller than zero bytes", A("Value", s)))
}

func sizeTooLarge(s string) error {
	return RefuseAbout(SettingSize, Says("core.SizeTooLarge", "{setting} %q is too large to express in bytes", A("Value", s)))
}

// plainDecimal reports whether the text is digits, with at most one decimal
// point and an optional leading sign. Nothing else counts as a number here.
//
// Leading zeros are allowed and mean nothing, which matches how the recipe
// reads a number: 010 is ten because that is what it looks like.
//
// One sign, not one of each. "+-5.5" passed here until 2026-10-27 and was
// refused only because the float parser behind it refused it - and the digits
// are read by splitDecimal now, which would have taken it for 5.5.
func plainDecimal(s string) bool {
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	dot, digits := false, 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return digits > 0
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

// ParseSizeRange reads the two ends of "1kb-8kb".
//
// It lives here rather than in the recipe package because two surfaces ask the
// same question. The recipe key and the --size-range flag have to agree on what
// a range is down to the byte, and two implementations of one rule is a place
// for them to drift apart - the reader would then meet a recipe the formatter
// calls clean and the generator refuses, or the reverse.
//
// The hyphen is not ambiguous here because a size is never negative.
func ParseSizeRange(text string) (low, high int64, err error) {
	parts := strings.Split(text, "-")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, Refuse(Says("core.RangeNotARange", "%q is not a range. A range is exactly two sizes with one hyphen between them, such as 1kb-8kb", A("Value", text)))
	}

	ends := [2]int64{}
	for i, part := range parts {
		n, perr := ParseSize(part)
		if perr != nil {
			return 0, 0, perr
		}
		ends[i] = n
	}

	// Equal ends are legal and mean every file is that size. It reads oddly and
	// is a reasonable thing to arrive at from a script that computes both ends,
	// so refusing it would be pedantry rather than protection.
	if ends[0] > ends[1] {
		return 0, 0, Refuse(Says("core.RangeBackwards",
			"the range runs backwards, from %d B down to %d B. The smaller end comes first, and a range that runs backwards holds no sizes at all. Write it as %d-%d",
			A("From", ends[0]), A("To", ends[1]), A("Low", ends[1]), A("High", ends[0])))
	}
	return ends[0], ends[1], nil
}

// ParseBoundary reads the limit of a boundary set.
//
// It counts in 1024s, the same as every other size this tool reads, and that is
// the whole of it since 2026-08-18.
//
// It used to REFUSE a spelling like 15mb rather than guess, on the grounds that
// a limit belongs to the system under test and "15 MB" on an upload form means
// 15000000 B far more often than not. The owner withdrew that on 2026-08-18 out
// of their own experience: a limit written "15 MB" is worked out in 1024s in
// almost every system, and the decimal reading is the rare one. The premise was
// backwards, so the refusal built on it went.
//
// What made the refusal safe to remove is that the run already answers the
// question it was asking. A boundary set prints the limit it was built around
// in bytes - "boundary "files" around 15728640 B" - above the three files, so
// anybody whose system meant the other number sees it at once, in a dry run,
// before a byte is written. A plain byte count still passes through for them.
//
// Here rather than in the flag for the same reason as ParseSizeRange: the
// recipe key and the flag have to mean the same thing, and one implementation
// is how that stays true.
func ParseBoundary(s string) (int64, error) {
	return ParseSize(s)
}

// The two parts of ErrBoundaryTooLarge, which a preset's refusal carries
// apart - the same way the other end's are kept.
var (
	BoundaryTooLargeWhy = Says("core.BoundaryTooLargeWhy", "a boundary set needs one size above the limit, and there is no number above this one")
	BoundaryTooLargeFix = Says("core.BoundaryTooLargeFix", "Use a limit at least one byte below the largest, or check that the number is the one you meant")
)

// ErrBoundaryTooLarge is a limit with no room above it for the third file.
//
// Measured on 2026-08-03: --boundary 9223372036854775807 wrapped the file above
// the limit to the smallest number there is, and the refusal that followed said
// "TXT cannot be smaller than 0 B. Requested: -9223372036854775808 B" - an
// answer about the bottom of the range to a question about the top of it.
var ErrBoundaryTooLarge = Refuse(Says("core.BoundaryTooLarge", "%s. %s", A("Why", BoundaryTooLargeWhy), A("Fix", BoundaryTooLargeFix)))

// The other end, in the two parts a report keeps apart. Built from them rather
// than beside them, the same way ErrTooManyFiles is, so the sentence and the
// parts cannot come to disagree.
var (
	BoundaryTooSmallWhy = Says("core.BoundaryTooSmallWhy", "the set needs a size one byte below the limit, and there is nothing below zero")
	BoundaryTooSmallFix = Says("core.BoundaryTooSmallFix", "Use a limit of at least 1 B")
)

// ErrBoundaryTooSmall is a limit with no room below it for the first file.
//
// The rule is here rather than in the callers, and that is the whole point of
// this error existing. Until 2026-08-25 BoundarySizes(0) returned {-1, 0, 1} -
// a file of minus one byte - and nothing went wrong only because both callers
// happened to check the low end themselves, in two places, in two sentences
// that had already drifted apart by a comma. An outside review named it, and
// this file already argued the same thing about ParseSizeRange and
// ParseBoundary: two implementations of one rule are a place for them to
// disagree.
var ErrBoundaryTooSmall = Refuse(Says("core.BoundaryTooSmall", "%s. %s", A("Why", BoundaryTooSmallWhy), A("Fix", BoundaryTooSmallFix)))

// BoundarySizes turns a limit into the three sizes a boundary set means: one
// byte under it, the limit itself, and one byte over.
//
// Here for the same reason as ParseSizeRange - the recipe key and the flag have
// to mean the same thing, and one implementation is how that stays true.
func BoundarySizes(limit int64) ([]int64, error) {
	// Equality rather than "at or above", because nothing of this type sits
	// above the largest value it can hold. The wider comparison read as the
	// more careful one and its upper half was unreachable.
	if limit < 1 {
		return nil, ErrBoundaryTooSmall
	}
	if limit == math.MaxInt64 {
		return nil, ErrBoundaryTooLarge
	}
	return []int64{limit - 1, limit, limit + 1}, nil
}
