package window

import (
	"fmt"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
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

// manifestReachNote is the window's half of the warning the command line prints
// before the first byte.
//
// Observation O184: the command line has said this since 2026-08-26 and the
// window said nothing at all, so a person generating 25 000 files from a window
// was left with a directory that neither Verify nor Clean up can read - and the
// manifest is the only authority over what may be removed. A parity gap in
// quality rather than in what the engine can do, which is the kind D1 is
// easiest to lose.
//
// Read off the document rather than worked out here, and off the SAME predicate
// the command line uses, which is what manifest.TooLargeToReadBack exists for.
// The two surfaces cannot come to different conclusions about one run.
//
// A preview reaches this too. engine.Run with DryRun adds an entry for every
// planned file, so the document a preview produces is the document the run
// would produce, minus the bytes on the disk. That is why one shape serves
// both, and why the window can answer before anything is written even though it
// cannot say a word in the middle of a run.
func manifestReachNote(res *engine.Result) []string {
	if res == nil || res.Manifest == nil {
		return nil
	}
	size, over := res.Manifest.ReadBackReach()
	if !over {
		return nil
	}
	return []string{text.ManifestTooLargeToRead(
		core.HumanBytes(size), core.HumanBytes(manifest.MaxBytes))}
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
