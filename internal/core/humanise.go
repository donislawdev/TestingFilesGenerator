package core

import (
	"fmt"
	"math"
	"time"
)

// How a number is put in front of a person, for the two surfaces that both have
// to do it.
//
// These sat in the command line's progress bar until the window needed a
// progress bar of its own. Two surfaces cannot import each other, so leaving
// them there meant copying them - and a copy of "how big is that in words"
// drifts quietly: the bar in one surface would start rounding differently from
// the bar in the other, and nobody compares two progress bars.

// HumanBytes counts in 1024s, the same as every size this tool accepts and the
// same as what Explorer and ls show. See docs/RECIPE.md section 9.
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// Percent divides before multiplying where it has to, so a very large run does
// not wrap on the way to a number between nought and a hundred.
//
// done*100 leaves the range of an int64 above about 92 PB. No disk holds that
// today, and the arithmetic that produces it is free to be right anyway.
func Percent(done, total int64) int {
	switch {
	case total <= 0:
		return 100
	case done >= total:
		return 100
	case done > math.MaxInt64/100:
		return int(done / (total / 100))
	}
	return int(done * 100 / total)
}

// Count puts a number in front of a person with its noun in the right number:
// "1 file", "7 files".
//
// Every message on the command line wrote "%d file(s)" until 2026-08-13. That
// is an English dodge around the plural, and the window had stopped using it a
// day earlier - so the same run was described two ways, and the surface a
// person meets first described it worse.
//
// The caller hands over both words instead of a rule that adds an "s". A rule
// gets "difference" right and "box" wrong, and it would be wrong quietly, in a
// line nobody reads twice.
//
// A sentence built on this must not put a verb after the count. "1 file were
// removed" is worse than the dodge it replaces, so write the sentence with
// nothing in it that agrees with the number - a participle ("1 file removed"),
// a modal ("1 file would be removed") or a noun phrase ("the only record of 1
// file") all read the same at every count. Two messages in cleanup were
// reshaped for exactly this reason.
func Count(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// Roughly keeps an estimate at the precision it deserves. Seconds on a two
// minute estimate are noise that changes every redraw.
func Roughly(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds())+1)
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes())+1)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
