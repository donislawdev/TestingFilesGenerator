package parts

import (
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// ByteCount says what the size in a box comes to, counted out in bytes.
//
// "10mb" is two different numbers depending on who reads it, and which one this
// tool means was settled long ago and written down - units count in 1024s,
// RECIPE.md section 9. What had never happened is anybody being TOLD on the
// screen. The sentence behind the little i said it, in an example about 10mb
// rather than about whatever is in the box, and only to somebody who clicked.
//
// It is not a second opinion about the units. It asks core.ParseSize, which is
// the same function the run parses the box with, so it cannot come to a
// different number than the files do - and if the box holds something that is
// not a size it says nothing at all rather than guessing.
//
// It stands beside the box, on the same line, since 2026-09-14. It lived at
// the far right of the field's name line before that - the cheap place while a
// name stood over its box, and 190 px from the box it was counting for. A
// field is one line now and the count is the thing after the box on it.
//
// Spelled by core.ExactBytes, which is what the command line prints. The
// window had a spelling of its own without the grouping until 2026-09-14, so
// one number came out as 10485760 B here and 10 485 760 B there.
//
// Quiet words since 2026-09-23, rather than a low importance label under a
// theme override - see QuietText for what the override cost.
type ByteCount struct {
	QuietText
}

func newByteCount() *ByteCount {
	c := &ByteCount{}
	c.start("")
	c.ExtendBaseWidget(c)
	// The same rank as the line explaining a field, because it is the same kind
	// of thing: something quiet beside a control that the eye should skip until
	// it wants it.
	c.SizeName = theme.SizeNameCaptionText
	return c
}

// show puts the count for one size on screen, or takes it away.
//
// Empty for anything that is not a size, which covers the box somebody is
// halfway through typing. A count that guessed at a half-typed value would
// change three times per keystroke and be wrong twice.
func (c *ByteCount) show(size string) {
	bytes, err := core.ParseSize(size)
	if err != nil {
		c.SetText("")
		return
	}
	c.SetText(core.ExactBytes(bytes))
}
