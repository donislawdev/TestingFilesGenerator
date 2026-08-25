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

func notesOf(res *engine.Result) []string {
	if res == nil || res.Manifest == nil {
		return nil
	}
	return res.Manifest.Notes()
}
