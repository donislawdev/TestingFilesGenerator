package window

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What a finished run says about itself.
//
// Split out of run.go on 2026-08-25 for the same reason the buttons were: the
// file went past three quarters of its ceiling, and the ceiling is a ratchet.

func outcomeText(res *engine.Result, runErr error) string {
	if res == nil || res.Manifest == nil {
		return text.NothingProduced()
	}
	written := len(res.Manifest.Files) - res.Failures
	switch {
	case runErr != nil:
		return text.StoppedAfter(written)
	case res.Failures > 0:
		return text.WrittenWithFailures(written, res.Failures)
	}
	return text.Written(written)
}

// notesOf is what a run says about its files, in the window's language. A
// note the run did not remember as a sentence - there is none today - is said
// as the manifest holds it.
func notesOf(res *engine.Result) []string {
	if res == nil || res.Manifest == nil {
		return nil
	}
	groups := res.Manifest.NoteGroups()
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		note, known := res.NotesSaid[g.Detail]
		if !known {
			note = g.AsHeld()
		}
		out = append(out, text.Sentence(g.Line(note)))
	}
	return out
}
