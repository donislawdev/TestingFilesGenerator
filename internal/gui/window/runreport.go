package window

import (
	"fmt"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What a run tells the person while it goes and when it ends.
//
// Split out of run.go on 2026-08-26, when moving the planning off the interface
// thread took that file past three quarters of the ceiling on how long a file
// may be. The ceiling is a ratchet, so the answer is a split and never a higher
// number. The split is by subject: everything here turns what the engine
// reports into the sentence under the buttons, and nothing here drives a run.

// previewText is the cost, before anything exists. G6: how many files, what
// kind, how many bytes, and how much room there is for them.
func previewText(planned []engine.PlannedFile, outDir string) string {
	total := engine.TotalBytes(planned)
	line := text.PreviewCost(len(planned), formatsOf(planned), core.HumanBytes(total))

	// A disk we cannot measure is not the same as a disk that is full, so a
	// failure to read it says nothing rather than inventing a number.
	if free, err := core.AvailableBytes(outDir); err == nil {
		line += text.PreviewFreeSpace(outDir, core.HumanBytes(free))
	}
	return line
}

// progressText is the line under the bar. Bytes rather than files, because one
// large file is a run where the file count says nothing for minutes.
func progressText(p engine.Progress, elapsed time.Duration) string {
	line := text.Progress(p.FilesDone, p.FilesTotal,
		core.HumanBytes(p.BytesDone), core.HumanBytes(p.BytesTotal),
		core.Percent(p.BytesDone, p.BytesTotal))

	// The estimate stays quiet until it has enough to go on. A number that
	// swings wildly for the first second is worse than no number.
	if elapsed < time.Second || p.BytesDone <= 0 || p.BytesDone >= p.BytesTotal {
		return line
	}
	left := time.Duration(float64(elapsed) *
		float64(p.BytesTotal-p.BytesDone) / float64(p.BytesDone))
	return line + text.TimeLeft(core.Roughly(left))
}

// saveManifest writes the record of what the run did.
//
// A run refused before it wrote anything gets none. Writing one would replace
// the record of whatever was already in that directory, and that record is the
// only thing cleanup can work from.
func saveManifest(res *engine.Result, opt engine.Options) error {
	if opt.DryRun || res == nil || !res.Started {
		return nil
	}
	// Asked of the engine rather than joined here. This used to be
	// filepath.Join(opt.OutDir, opt.ManifestName), which is the same answer
	// only while the name is filled in - and with it blank it named the output
	// directory itself, so saving would have tried to rename a file onto a
	// directory. All three screens do fill it in, which is why nothing ever
	// reached it.
	path := engine.ManifestPath(opt)
	if err := res.Manifest.Save(path); err != nil {
		return fmt.Errorf("%s: %w", text.ManifestNotSaved(path), err)
	}
	return nil
}
