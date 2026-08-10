// Package text holds the sentences the window shows.
//
// Why a package rather than literals where they are used. D9 gives the window
// translations and keeps the command line English forever, so this is the one
// surface here that will ever have a second language. Nothing in this project
// enforced that: until 2026-08-10 every label and message was a literal spread
// across five files, and nobody could say how many there were.
//
// What this is NOT. It is not i18n. There is no catalogue, no locale and no
// lookup, and adding them is still the separate decision docs/GUI.md section 6
// calls it. What this buys is an inventory and one seam - when a catalogue
// arrives it goes underneath these functions, and no place that calls them has
// to change.
//
// Two rules decide the shape here, and both come from measuring what would
// otherwise be painful later.
//
// Text carrying a value is a FUNCTION, never a format string held as a
// constant. Seven messages in this window say "file(s)", which is an English
// dodge around the plural - Polish needs three forms and picks one from the
// number. A constant holding "%d file(s)" leaves that decision at the call
// site, and there are dozens of those. A function keeps it here, where one
// change reaches all of them.
//
// Every entry says where it appears. A translator reading a bare sentence
// cannot tell a field label from a column heading, and the same English word
// is often two different words elsewhere. Written now, while somebody knows.
//
// Names are the catalogue keys, so they are English (D9) and they describe the
// place rather than the wording - renaming a sentence should not rename a key.
package text

import "fmt"

// Buttons on the run controls, in the order G6 puts them: preview before the
// thing it previews.
const (
	ButtonPreview  = "Preview"
	ButtonGenerate = "Generate"
	ButtonCancel   = "Cancel"
)

// PreviewCost is the line under the buttons after Preview, saying what the run
// would cost and that nothing exists yet.
func PreviewCost(files int, total string) string {
	return fmt.Sprintf("%d file(s), %s in total. Nothing has been written.", files, total)
}

// PreviewFreeSpace follows PreviewCost when the disk could be measured. It is
// a separate sentence because a disk we cannot read has to say nothing at all
// rather than invent a number.
func PreviewFreeSpace(dir, free string) string {
	return fmt.Sprintf(" %s has %s free.", dir, free)
}

// WritingFiles is the line under the bar at the moment a run starts, before
// the first progress report arrives.
func WritingFiles(files int) string {
	return fmt.Sprintf("writing %d file(s)...", files)
}

// Progress is the line under the bar during a run. Bytes as well as files,
// because one large file is a run where the file count says nothing for
// minutes.
func Progress(filesDone, filesTotal int, bytesDone, bytesTotal string, percent int) string {
	return fmt.Sprintf("%d/%d files  %s of %s  %d%%",
		filesDone, filesTotal, bytesDone, bytesTotal, percent)
}

// TimeLeft is appended to Progress once the estimate is worth showing.
func TimeLeft(roughly string) string {
	return "  " + roughly + " left"
}

// ManifestNotSaved is the error when the files exist and their record does
// not. The caller wraps the underlying reason onto the end.
func ManifestNotSaved(path string) string {
	return fmt.Sprintf("the files were written and the manifest could not be saved to %s", path)
}

// NothingProduced is the outcome when a run ended with no manifest at all.
const NothingProduced = "nothing was produced."

// StoppedAfter is the outcome of a run that was cancelled or failed part way.
func StoppedAfter(written int) string {
	return fmt.Sprintf("stopped after %d file(s). The manifest describes exactly those.", written)
}

// WrittenWithFailures is the outcome when some files could not be produced.
// Untouchable rule 6: this has to be visible in the window and not only in the
// manifest, because "the manifest says which ones" is an answer in a terminal
// and an instruction to open a ten thousand entry file in a window.
func WrittenWithFailures(written, failed int) string {
	return fmt.Sprintf("%d file(s) written, %d could not be produced.", written, failed)
}

// Written is the outcome of a run where everything asked for was produced.
func Written(written int) string {
	return fmt.Sprintf("%d file(s) written.", written)
}
