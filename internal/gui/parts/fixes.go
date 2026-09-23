package parts

import (
	"errors"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// fixFor is the button a refusal can carry under a field, when the refusal
// itself holds the value that would put it right - or nothing.
//
// One refusal today, and it is the one the owner reported from the running
// window on 2026-09-23: a size below what a format can make says the smallest
// it CAN make as an exact count of bytes, "14707223 B", and the only way to
// act on it was to copy eight digits into the box by hand. The refusal is a
// format.BelowMinimumError, which carries the minimum as a number rather than
// only as words, so the button is worked out from the number and not read out
// of the sentence.
//
// Nothing is filled in by itself. Untouchable rule 1 says a size is exact or
// refused, never quietly rounded, and a button somebody presses is the
// opposite of quiet: the value goes in the box, the box is checked again like
// any typed value, and the byte count under it says what it is.
//
// Only on a box holding one size. A range of two sizes ("1kb-8kb") refused
// for its lower end would be replaced by a single number, which is a
// different request, so it gets the sentence and no button.
//
// And only on a box that HOLDS the size that was refused, and that is not the
// same thing as a box holding one size. Under "Around a limit" the box holds
// the limit and the set is built one byte below it, on it and one above - so
// a limit of 74 for a PNG refuses a file of 73, and the button put 74 back in
// the box: the same 73, the same refusal, forever. Found by an outside review
// of #126 and measured on 2026-09-23 (docs/REVIEW-126-2026-09-23.md).
//
// Raising the limit by the shortfall was tried and is not the answer either.
// PNG makes exactly 74 B or 86 B and more, nothing between, so a limit of 75
// builds 74, 75 and 76 and two of them fall in the gap - the button walked
// 74, 75, 86, 87 and called every step "the smallest size". The smallest limit
// whose three files all exist is a question about the format, and only the
// engine can answer it (G1). Until it can, the limit keeps the sentence and
// gets no button, because a button that promises the smallest value and does
// not deliver it is worse than none.
func fixFor(f *Field, err error) (string, func()) {
	var below *format.BelowMinimumError
	if !errors.As(err, &below) || below.Minimum <= 0 {
		return "", nil
	}
	boxes := boxesIn(f.Control)
	if len(boxes) != 1 || strings.Contains(boxes[0].Text, "-") {
		return "", nil
	}
	if held, parsed := core.ParseSize(boxes[0].Text); parsed != nil || held != below.Requested {
		return "", nil
	}
	box, value := boxes[0], strconv.FormatInt(below.Minimum, 10)
	return text.UseSmallestSize(core.ExactBytes(below.Minimum)), func() { box.SetText(value) }
}
