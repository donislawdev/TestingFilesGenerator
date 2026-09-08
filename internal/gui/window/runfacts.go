package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// runFacts is what the panel beside the form says about the run the settings
// add up to.
//
// It exists because that panel had nothing in it. Measured on 2026-09-08: at
// 1120x760 it drew 414 px of panel around one sentence, and the sentence only
// named the output directory. The window could not answer "what am I about to
// do" until somebody pressed a button, so the answer to G6 - say what a run
// costs before it starts - was behind a press that a person may never make.
//
// Every number here is READ rather than worked out, which is G1. A count is the
// length of the list of sizes the engine built, a format is the one the target
// carries, and a size is what core.ParseSize already turned the box into. The
// two that cannot be had without planning - the total, and the room left on the
// disk - arrive from a preview and are absent until one happens. That is the
// owner's decision of 2026-09-08, taken against the alternative of planning on
// every keystroke, which at 25 000 files is a plan per keypress.
type runFacts struct {
	batches *parts.Fact
	files   *parts.Fact
	format  *parts.Fact
	each    *parts.SizeFact
	total   *parts.SizeFact
	free    *parts.SizeFact
	// rule is the hairline under the group. It travels with the lines rather
	// than standing in the panel on its own, because a rule above nothing is a
	// rule separating one thing from itself.
	rule fyne.CanvasObject
}

// newRunFacts builds the lines, all of them off the screen until there is
// something to put on them.
func newRunFacts() *runFacts {
	return &runFacts{
		batches: parts.NewFact(text.FactBatches()),
		files:   parts.NewFact(text.FactFiles()),
		format:  parts.NewFact(text.FactFormat()),
		each:    parts.NewSizeFact(text.FactEachFile()),
		total:   parts.NewSizeFact(text.FactTotal()),
		free:    parts.NewSizeFact(text.FactFreeOnDisk()),
		rule:    hiddenRule(),
	}
}

// hiddenRule is the separator, off the screen until there is something above it.
func hiddenRule() fyne.CanvasObject {
	rule := parts.Rule()
	rule.Hide()
	return rule
}

// objects are the lines in the order they are read, for the panel to hold.
func (f *runFacts) objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		f.batches.Object(), f.files.Object(), f.format.Object(),
		f.each.Object(), f.total.Object(), f.free.Object(), f.rule,
	}
}

// From says what a settled set of targets comes to.
//
// Called on every keystroke through recheck, so it does no work a plan would
// do. What it walks is the targets themselves, once.
func (f *runFacts) From(targets []engine.Target) {
	if len(targets) == 0 {
		f.Nothing()
		return
	}
	// Only when there is more than one. A line reading "Batches 1" on a screen
	// that draws exactly one batch is a line that answers a question nobody
	// asked, and the panel is short enough to notice.
	f.rule.Show()
	f.batches.Say(countOf(len(targets) > 1, len(targets)))
	f.files.Say(core.Grouped(int64(filesIn(targets))))
	f.format.Say(oneFormat(targets))

	if size, same := oneSize(targets); same {
		f.each.Say(size)
	} else {
		// Batches of different sizes, a size range, or a container working its
		// own size out. Each of those has a per file size and none of them has
		// ONE, so the honest answer is not to draw the line - the same rule
		// ByteCount follows for a box holding something that is not a size.
		f.each.Nothing()
	}
	// A preview's answers describe the plan that was previewed. The moment a
	// field changes they describe a run nobody asked for any more, so they go.
	f.total.Nothing()
	f.free.Nothing()
}

// Previewed adds what only a plan can say.
//
// Free space is separate from the total because a disk that cannot be measured
// has to say nothing rather than invent a number - the same distinction
// PreviewFreeSpace draws in the sentence under these lines.
func (f *runFacts) Previewed(total int64, free int64, measured bool) {
	f.total.Say(total)
	if !measured {
		f.free.Nothing()
		return
	}
	f.free.Say(free)
}

// Nothing takes every line off the screen, for a form that does not settle.
//
// A form holding a half typed size does not describe a run, so a panel that
// kept the last numbers would be reporting a run that is no longer the one on
// the screen. Silence is the only honest state.
func (f *runFacts) Nothing() {
	f.rule.Hide()
	f.batches.Say("")
	f.files.Say("")
	f.format.Say("")
	f.each.Nothing()
	f.total.Nothing()
	f.free.Nothing()
}

// countOf spells a number, or nothing at all when the line is not wanted.
func countOf(wanted bool, n int) string {
	if !wanted {
		return ""
	}
	return core.Grouped(int64(n))
}

// filesIn is how many files these targets describe.
//
// The length of a list rather than a sum of counts, because a target carries
// one size per file - engine.Target.Sizes - so the engine has already said how
// many there are and this only reads it off.
func filesIn(targets []engine.Target) int {
	files := 0
	for _, t := range targets {
		files += len(t.Sizes)
	}
	return files
}

// oneFormat is the format these targets share, or nothing when they differ.
//
// A panel that named the first batch's format while three batches wrote three
// kinds of file would be stating something false about two of them.
func oneFormat(targets []engine.Target) string {
	shared := targets[0].Format
	for _, t := range targets[1:] {
		if t.Format != shared {
			return ""
		}
	}
	return shared
}

// oneSize is the size every file in these targets has, and whether they all
// have one.
//
// False for a range and for a container that works its own size out, because
// both of those carry only a count until planning happens - see
// engine.Target.SizeIsRange and SizeFromContents. False as well for a boundary
// set, where three consecutive sizes sit under one id on purpose.
func oneSize(targets []engine.Target) (int64, bool) {
	for _, t := range targets {
		if t.SizeIsRange || t.SizeFromContents || len(t.Sizes) == 0 {
			return 0, false
		}
	}
	shared := targets[0].Sizes[0]
	for _, t := range targets {
		for _, size := range t.Sizes {
			if size != shared {
				return 0, false
			}
		}
	}
	return shared, true
}
