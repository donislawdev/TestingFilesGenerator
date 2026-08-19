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

import (
	"fmt"
	"strings"
)

// Buttons on the run controls, in the order G6 puts them: preview before the
// thing it previews.
const (
	ButtonPreview  = "Preview"
	ButtonGenerate = "Generate"
	ButtonCancel   = "Cancel"
)

// files is a count with its noun, in the right number.
//
// The doc above this package has described "file(s)" as a dodge since the day
// it was written, and said a function is where the decision belongs. Every
// message here still wrote the brackets until 2026-08-12, so the seam existed
// and nothing had ever gone through it. This is that function. When a
// catalogue arrives, Polish picks one of three forms from the number and it
// picks it here.
func files(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// separator divides the facts on a status line.
//
// They are separate facts and were one sentence: how many files, how big, and
// whether anything exists yet ran together with a full stop between them, so
// three unrelated things read as prose and none of them could be found at a
// glance. Divided, the line is scanned rather than read.
const separator = " · "

// PreviewCost is the line under the buttons after Preview, saying what the run
// would cost and that nothing exists yet.
//
// It names the kinds of file as well as the count, added on 2026-08-12. On the
// generate screen that is on the screen anyway, and on the preset screen it is
// the answer to a question the screen could not otherwise be asked: a preset
// supplies the format itself unless somebody says otherwise, so "seven files,
// 70 MiB" left out the one fact that says what they are.
//
// The list is left out entirely when there is nothing to list, rather than
// shown empty. A plan that produces no files is a legal outcome here and it
// already says so in the count.
func PreviewCost(count int, formats []string, total string) string {
	line := files(count)
	if len(formats) > 0 {
		line += separator + strings.Join(formats, ", ")
	}
	return line + separator + total + separator + "nothing written yet"
}

// PreviewFreeSpace follows PreviewCost when the disk could be measured. It is
// a separate fact because a disk we cannot read has to say nothing at all
// rather than invent a number.
func PreviewFreeSpace(dir, free string) string {
	return separator + free + " free in " + dir
}

// WritingTo is what the status line says when a run has not said anything yet.
//
// The destination is the one field on these forms that is off the screen when
// the window opens, and it is the only one that decides where somebody else's
// disk gets written to. It sits here because the line is kept clear for a run
// whether or not there is one, so saying this costs no room at all - and a
// preview replaces it with a sentence that names the same directory.
func WritingTo(dir string) string {
	return "Files will go to " + dir
}

// WritingFiles is the line under the bar at the moment a run starts, before
// the first progress report arrives.
func WritingFiles(count int) string {
	return "Writing " + files(count) + "..."
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

// WindowTitle is what the desktop shows in the title bar and in the task
// switcher, with the version so that a screenshot says which build it is.
//
// Here since 2026-08-13, and it had been a literal in the file that opens the
// window since the day there was a window. The guard could not see it: it
// worked from a list of the calls that show text, and nobody had thought to put
// the toolkit's NewWindow on that list. Which is the argument for the rule
// being the other way round.
func WindowTitle(version string) string {
	return "Testing Files Generator " + version
}

// NoWindowInThisBuild is what the window binary says when it was compiled
// without the C support its toolkit needs.
//
// Four parts, D6: what cannot be done, why, what works instead, and what to do
// about it.
const NoWindowInThisBuild = "tfg-gui: this build has no window in it. It was compiled without C support, " +
	"which the graphics toolkit needs to reach OpenGL. Every feature is available " +
	"from the command line, which needs neither - run \"tfg --help\". " +
	"To get a window, use a tfg-gui built for your system rather than this one."

// NotAWholeNumber refuses a box that should hold digits and does not.
//
// The field is named by its label rather than by its key, because this is read
// by somebody looking at the screen - and it arrives from here rather than as a
// literal at the call site, which is where "how many" and "seed" were until
// 2026-08-13.
func NotAWholeNumber(field, value string) string {
	return fmt.Sprintf("%s is %q, which is not a whole number. Write the digits out, such as 1 or 500",
		field, value)
}

// ManifestNotSaved is the error when the files exist and their record does
// not. The caller wraps the underlying reason onto the end.
func ManifestNotSaved(path string) string {
	return fmt.Sprintf("the files were written and the manifest could not be saved to %s", path)
}

// NothingProduced is the outcome when a run ended with no manifest at all.
const NothingProduced = "Nothing was produced."

// StoppedAfter is the outcome of a run that was cancelled or failed part way.
func StoppedAfter(written int) string {
	return fmt.Sprintf("Stopped after %s. The manifest describes exactly those.", files(written))
}

// WrittenWithFailures is the outcome when some files could not be produced.
// Untouchable rule 6: this has to be visible in the window and not only in the
// manifest, because "the manifest says which ones" is an answer in a terminal
// and an instruction to open a ten thousand entry file in a window.
func WrittenWithFailures(written, failed int) string {
	return fmt.Sprintf("%s written, %d could not be produced.", files(written), failed)
}

// Written is the outcome of a run where everything asked for was produced.
func Written(written int) string {
	return files(written) + " written."
}
