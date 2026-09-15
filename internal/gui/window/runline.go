package window

import (
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The line at the foot of a form that says what the form comes to.
//
// It answers G6 before anything is pressed: how many files, how many bytes,
// what kind and where, read off the form as it is typed. The bar used to say
// one of those four - the directory - on a line that gave way to the first
// thing a run said and never came back (see the runner's history of
// resting). The line is worked out again on every change of the form, so
// what a run says stands in place of a summary that is still true rather
// than in place of one that was.
//
// A line and not a panel of rows. Rows were built first, on 2026-09-14, and
// the owner's verdict on the render was that four short values in a wide
// strip read as a panel with nothing in it. The same four facts fit on the
// line the bar already keeps clear for a run, and cost the form no height.

// runLine turns what the form comes to into the sentence under the buttons,
// and remembers what a worker found out about the disk.
type runLine struct {
	// freeIn is the directory a preview or a run measured, and free is the
	// room it found there. Said only while the directory on the form is
	// still that one: a number measured for one disk says nothing about the
	// disk somebody has typed since.
	freeIn    string
	free      int64
	freeKnown bool
}

// said is the line for a form that settles.
func (l *runLine) said(sum summary, dir string) string {
	return text.RunLine(int(sum.files), sum.totalText(), sum.formats, l.where(dir))
}

// fallback is the line for a form that does not settle: where the files
// would go, which is read off its own box and is true whatever the other
// boxes say. Empty when even that is not known, so the line takes no room.
func (l *runLine) fallback(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	return text.WritingTo(l.where(dir))
}

// where is the directory, with the room left on its disk when that has
// been measured for this very directory.
func (l *runLine) where(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir != "" && l.freeKnown && dir == l.freeIn {
		return text.DirectoryWithFreeSpace(dir, core.HumanBytes(l.free))
	}
	return dir
}

// measured records what a preview or a run found out about the disk under a
// directory. Recorded rather than read here, because reading it is disk work
// and this runs on the interface thread - the worker that went to the disk
// hands the answer over.
func (l *runLine) measured(dir string, free int64) {
	l.freeIn = strings.TrimSpace(dir)
	l.free = free
	l.freeKnown = true
}
