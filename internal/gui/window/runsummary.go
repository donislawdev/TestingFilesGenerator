package window

import (
	"math"
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What a form comes to, worked out from the targets it settles into and from
// nothing else - no disk, no planning.
//
// Planning is the wrong tool for a number that changes on every keystroke:
// planning a picture encodes it, and two thousand PNGs cost 16 to 23 seconds
// of it (see onPreview). The arithmetic here costs what adding a few numbers
// costs, so it can be redone every time a box changes. What it cannot know
// it says it cannot know, rather than guessing: a size drawn from a range is
// drawn when the run is planned, and a container sized by its contents is
// sized when it is built.
//
// A type that knows nothing about the toolkit, on purpose - GUI rule 15 of
// CLAUDE.md. It can be asked without a window, and the guard that asks it
// does exactly that.

// summary is what a set of targets adds up to.
type summary struct {
	// files is how many files the targets ask for, across every batch.
	files int64
	// least and most bound the bytes the run will occupy. Equal when every
	// target states its sizes, apart when one draws them from a range.
	least, most int64
	// certainty says how far the two bounds can be trusted.
	certainty certainty
	// formats is every format the targets name, each once, sorted - the same
	// rule formatsOf applies to a plan, and for the same reason: the order
	// targets were typed in is not something to show as if it meant anything.
	formats []string
}

// certainty is how much a summary can say about its total.
type certainty int

const (
	// stated: every size is on the form, so the total is exact.
	stated certainty = iota
	// drawn: at least one target draws its sizes from a range, so the total
	// is between two numbers until the run is planned.
	drawn
	// fromContents: at least one container is sized by what it holds, which
	// nothing knows before it is built.
	fromContents
	// beyondCounting: the sizes add up to more than a number can hold. The
	// run will be refused for it (core.AddSizes) - the strip does not invent
	// a smaller number in the meantime.
	beyondCounting
)

// summarise adds the targets up.
func summarise(targets []engine.Target) summary {
	s := summary{certainty: stated}
	seen := map[string]bool{}
	for _, t := range targets {
		s.files += int64(len(t.Sizes))
		if t.Format != "" && !seen[t.Format] {
			seen[t.Format] = true
			s.formats = append(s.formats, t.Format)
		}
		s.add(t)
	}
	sort.Strings(s.formats)
	return s
}

// add folds one target's sizes into the bounds.
func (s *summary) add(t engine.Target) {
	switch {
	case t.SizeFromContents:
		s.lower(fromContents)
	case t.Range.Used:
		s.lower(drawn)
		n := int64(len(t.Sizes))
		s.least = s.grow(s.least, times(n, t.Range.Min))
		s.most = s.grow(s.most, times(n, t.Range.Max))
	default:
		for _, bytes := range t.Sizes {
			s.least = s.grow(s.least, bytes)
			s.most = s.grow(s.most, bytes)
		}
	}
}

// grow adds to a running total, and marks the summary when the sum has left
// the range a number can hold. The same arithmetic the engine refuses with,
// so the strip and the refusal agree about where counting stops.
func (s *summary) grow(total, bytes int64) int64 {
	if bytes < 0 {
		s.lower(beyondCounting)
		return total
	}
	sum, err := core.AddSizes(total, bytes)
	if err != nil {
		s.lower(beyondCounting)
		return total
	}
	return sum
}

// times is n files of one size, or a number too large to hold, which grow
// then refuses. Written out because n*bytes wraps silently.
func times(n, bytes int64) int64 {
	if n <= 0 || bytes <= 0 {
		return 0
	}
	if bytes > math.MaxInt64/n {
		return -1
	}
	return n * bytes
}

// lower moves the certainty down, never up - a form with one range and one
// stated size is a form with a range.
func (s *summary) lower(to certainty) {
	if to > s.certainty {
		s.certainty = to
	}
}

// totalText is the total as the strip says it.
//
// Empty when the total cannot be counted. Nothing is invented in its place:
// the run is refused for exactly this, and that refusal has the four parts a
// message here would not have room for.
func (s summary) totalText() string {
	switch s.certainty {
	case fromContents:
		return text.SizeFromContents()
	case beyondCounting:
		return ""
	case drawn:
		if s.least != s.most {
			return text.SizeBetween(core.HumanBytes(s.least), core.HumanBytes(s.most))
		}
	case stated:
		// Every size is on the form, so the exact total below is the answer.
	}
	return text.SizeAndBytes(core.HumanBytes(s.least), core.ExactBytes(s.least))
}

// exactly is the summary of a plan: the sizes are drawn by now, so the two
// bounds meet and the total is what the disk will hold.
func exactly(planned []engine.PlannedFile) summary {
	total := engine.TotalBytes(planned)
	return summary{
		files:     int64(len(planned)),
		least:     total,
		most:      total,
		certainty: stated,
		formats:   formatsOf(planned),
	}
}
