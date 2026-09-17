// Package text holds the sentences the window shows.
//
// Why a package rather than literals where they are used. D9 gives the window
// translations and keeps the command line English forever, so this is the one
// surface here that will ever have a second language. Nothing in this project
// enforced that: until 2026-08-10 every label and message was a literal spread
// across five files, and nobody could say how many there were.
//
// What this was NOT, until it was. This paragraph said "it is not i18n, there
// is no catalogue, no locale and no lookup" and promised that when one arrived
// it would go underneath these functions without a single caller changing. The
// catalogue arrived on 2026-08-25 and the promise held: not one call site moved.
// The paragraph is rewritten rather than deleted, because the shape it argued
// for is the reason the change cost nothing.
//
// So what is here now is the inventory AND the lookup. Every entry states its
// English on the spot, beside the reason it is worded that way, and hands it to
// the catalogue as the default - see say, sayf and sayN in catalogue.go. A
// sentence that states its words itself instead reaches no translator at all,
// and since 2026-08-26 a guard says so by name.
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
	"strconv"
	"strings"
)

// Buttons on the run controls, in the order G6 puts them: preview before the
// thing it previews.
func ButtonPreview() string  { return say("ButtonPreview", "Preview") }
func ButtonGenerate() string { return say("ButtonGenerate", "Generate") }

// ButtonOpenFolder shows the directory a finished run wrote into. It is on the
// bar only while there is something to open.
func ButtonOpenFolder() string { return say("ButtonOpenFolder", "Open folder") }
func ButtonCancel() string     { return say("ButtonCancel", "Cancel") }

// files is a count with its noun, in the right number.
//
// The doc above this package has described "file(s)" as a dodge since the day
// it was written, and said a function is where the decision belongs. Every
// message here still wrote the brackets until 2026-08-12, so the seam existed
// and nothing had ever gone through it. This is that function.
//
// The catalogue arrived underneath it on 2026-08-26, and the sentence above
// came true: BOTH English forms go over rather than one sentence with a number
// in it, so a translation file may carry as many forms as its language has and
// the library picks between them. Polish gets its three. See sayN.
func files(n int) string {
	return sayN("Files", "1 file", "{{.Count}} files", n, nil)
}

// separator divides the facts on one line.
//
// They are separate facts and were one sentence: a folded section's stated
// values ran together with a full stop between them, so unrelated things read
// as prose and none of them could be found at a glance. Divided, the line is
// scanned rather than read.
const separator = " · "

// RunLine is the line under the buttons while nothing pressed has spoken:
// what the form comes to, worked out from the form as it is typed.
//
// It grew out of the line that named only the destination. G6 says the window
// says what a run will cost before anything is pressed, and until 2026-09-14
// that answer was behind the Preview button - the line at rest said where the
// files would go and nothing else. Now it says how many, how big, what kind
// and where, and Preview turns the estimate into a measurement.
//
// One line rather than a panel of rows, and that is the owner's verdict on
// the rows: four short values in a wide strip read as a panel with nothing
// in it. A line holds the same four facts in the room the bar already keeps.
//
// The facts are divided the way FoldedSummary divides them, so the line is
// scanned rather than read. The destination comes last and is left off when
// there is none, so a form with the directory cleared still says what it
// comes to.
func RunLine(count int, total string, formats []string, destination string) string {
	facts := []string{files(count)}
	if total != "" {
		facts = append(facts, total)
	}
	if len(formats) > 0 {
		facts = append(facts, Formats(formats))
	}
	if destination != "" {
		facts = append(facts, WillGoTo(destination))
	}
	return strings.Join(facts, separator)
}

// WillGoTo is the destination, as one fact among the others on the line.
func WillGoTo(dir string) string {
	return sayf("WillGoTo", "will go to {{.Directory}}", map[string]any{"Directory": dir})
}

// WritingTo is what the line says when the form cannot be added up yet - a
// batch with no name, a count that is not a number. The one fact that is
// still true is where the files would go, and it is the one field that
// decides where somebody else's disk gets written to, so it stays on the
// line on its own rather than the line going blank.
func WritingTo(dir string) string {
	return sayf("WritingTo", "Files will go to {{.Directory}}", map[string]any{"Directory": dir})
}

// AndNothingWrittenYet follows the line after a preview, which has made
// every number on it exact and put nothing on the disk. It carries its own
// divider, so a caller appends it to a line rather than assembling one.
func AndNothingWrittenYet() string {
	return separator + say("PreviewNothingWritten", "nothing written yet")
}

// Formats is a list of kinds of file, said on one line. Here rather than
// joined at the call site because the mark between two items is something a
// person reads, and one language's comma is another's ideograph.
func Formats(kinds []string) string { return strings.Join(kinds, listSeparator) }

// listSeparator divides the items of a list said on one line.
const listSeparator = ", "

// SizeAndBytes is a size a person can read with the exact count after it,
// because a limit under test is exact and "10 MB" does not say whether it is
// 10 000 000 or 10 485 760. The exact count is left off when the readable
// form already is it.
func SizeAndBytes(human, exact string) string {
	if human == exact {
		return human
	}
	return sayf("SizeAndBytes", "{{.Human}} ({{.Exact}})",
		map[string]any{"Human": human, "Exact": exact})
}

// SizeBetween is the total of a form that draws its sizes from a range. The
// draw happens when the run is planned, so before that the honest answer is
// the two ends - and Preview turns it into one number.
func SizeBetween(least, most string) string {
	return sayf("SizeBetween", "between {{.Least}} and {{.Most}}",
		map[string]any{"Least": least, "Most": most})
}

// SizeFromContents is the total of a form holding a container whose size is
// whatever the files inside it come to, which nothing knows until the
// container is built.
func SizeFromContents() string {
	return say("SizeFromContents", "decided by the files inside, known after Preview")
}

// DirectoryWithFreeSpace is the output directory with the room left on its
// disk after it, once a preview has measured that room.
func DirectoryWithFreeSpace(dir, free string) string {
	return sayf("DirectoryWithFreeSpace", "{{.Directory}} ({{.Free}} free)",
		map[string]any{"Directory": dir, "Free": free})
}

// ManifestTooLargeToRead is said when a run will write a record this build
// cannot read back.
//
// The command line has printed this since 2026-08-26 and the window said
// nothing at all, which is the parity gap observation O184 names. A person who
// generates 25 000 files from a window gets a directory that tfg verify and
// tfg cleanup both refuse - and the manifest is the only authority over what
// may be deleted, so nothing in this toolset can ever remove those files.
//
// A note rather than a refusal, which is the owner's decision from that day and
// is unchanged here. The run works. What was missing was that nobody was told.
func ManifestTooLargeToRead(size, limit string) string {
	return sayf("ManifestTooLargeToRead",
		"this run's record is about {{.Size}} and this build reads at most {{.Limit}}, so Verify and Clean up will not be able to read it. Split the run to keep each record readable.",
		map[string]any{"Size": size, "Limit": limit})
}

// WorkingOutTheCost is what a preview says while it is going.
//
// A preview does disk work - it asks how much room there is and whether any of
// the names are taken - and on a slow or a networked directory that is not
// instant. It says so rather than leaving the screen looking idle while both
// buttons are greyed out with no explanation.
func WorkingOutTheCost() string {
	return say("WorkingOutTheCost", "Working out what this would cost...")
}

// Progress is the line under the bar during a run. Bytes as well as files,
// because one large file is a run where the file count says nothing for
// minutes.
func Progress(filesDone, filesTotal int, bytesDone, bytesTotal string, percent int) string {
	return sayf("Progress", "{{.Done}}/{{.Total}} files  {{.BytesDone}} of {{.BytesTotal}}  {{.Percent}}%",
		map[string]any{"Done": filesDone, "Total": filesTotal,
			"BytesDone": bytesDone, "BytesTotal": bytesTotal, "Percent": percent})
}

// TimeLeft is appended to Progress once the estimate is worth showing.
func TimeLeft(roughly string) string {
	return "  " + sayf("TimeLeft", "{{.Roughly}} left", map[string]any{"Roughly": roughly})
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

// CatalogueNotLoaded is what the window binary says when the translations it
// carries could not be read.
//
// English rather than translated, and that is not an oversight: this is the one
// message that cannot ask the catalogue for its own words, because the
// catalogue is what failed. It also goes to a terminal rather than to a window,
// which is where D9 puts English anyway.
//
// The window opens regardless. Every message states its English beside itself
// and answers with it when nothing is loaded, so this costs a language and not
// a program.
func CatalogueNotLoaded(err error) string {
	return "tfg-gui: the translations built into this program could not be read, " +
		"so the window is in English: " + err.Error()
}

// WindowRefusedTitle is over the one dialog this program can show without a
// window of its own: the toolkit could not open one.
func WindowRefusedTitle() string {
	return say("WindowRefusedTitle", "Testing Files Generator could not open its window")
}

// WindowRefused is what the window binary says when the toolkit could not
// create a window at all - measured on a virtual machine without 3D
// acceleration, where the graphics driver offers no OpenGL (O218).
//
// Four parts, D6: what did not happen, why, what works instead, and what to
// do about it. The cause is the toolkit's own sentence about the driver,
// quoted rather than said, so it stays in English whatever the language -
// and it is left out rather than quoted empty when the toolkit gave none.
//
// Through the catalogue although it may reach a terminal: it is the same
// sentence in the dialog and on standard error, and the dialog is read by
// the person the window was for.
func WindowRefused(cause string) string {
	// One literal each, however long: the catalogue is written from the
	// calls a script can see, and a sentence built from pieces is invisible
	// to it.
	if cause == "" {
		return say("WindowRefusedNoCause", "The window could not be opened. It draws through OpenGL 2.1, and the graphics toolkit could not get that from the driver on this computer. Everything the window does is also on the command line - run \"tfg --help\" - and that needs no graphics driver. To get the window, use a graphics driver that provides OpenGL 2.1.")
	}
	return sayf("WindowRefused", "The window could not be opened. It draws through OpenGL 2.1, and the graphics toolkit could not get that from the driver on this computer. The toolkit said: {{.Cause}}. Everything the window does is also on the command line - run \"tfg --help\" - and that needs no graphics driver. To get the window, use a graphics driver that provides OpenGL 2.1.", map[string]any{"Cause": cause})
}

// The software renderer shipped beside the window binary on Windows: what
// the window says when it starts again with it, when it draws with it, and
// when it could not. See internal/gui/software.go for the mechanism. The
// sentences below name the renderer's files and folder, which are file names
// rather than words and are not translated.

// StartingAgainWithSoftwareRenderer is the line the first process writes to
// standard error before it starts the second one: the driver refused, and
// this is what is being done about it.
func StartingAgainWithSoftwareRenderer() string {
	return say("RendererStartingAgain", "The graphics driver on this computer offers no OpenGL 2.1. Starting again with the software renderer shipped beside the program.")
}

// DrawingWithSoftwareRenderer is what a window drawn in software says about
// itself, on standard error when it starts and on the About screen while it
// is open. True whether the window got there by itself or was asked for it
// with --software-gl, which is why it says what is happening and not why.
func DrawingWithSoftwareRenderer() string {
	return say("RendererDrawing", "This window is drawn by the software renderer shipped beside the program (Mesa llvmpipe), not by the graphics driver. Everything works. Drawing is slower than with a driver that provides OpenGL 2.1.")
}

// RendererNotBeside is what the refusal adds on Windows when the renderer's
// file is not where the archive put it.
func RendererNotBeside(path string) string {
	return sayf("RendererNotBeside", "The software renderer that ships with the Windows archive was not found beside the program - {{.Path}} is missing. Put the opengl folder from the archive back next to tfg-gui.exe, or use a graphics driver that provides OpenGL 2.1.", map[string]any{"Path": path})
}

// RendererStartFailed is what the refusal adds when the second process,
// the one that would have loaded the renderer, could not be started.
func RendererStartFailed(err error) string {
	return sayf("RendererStartFailed", "Starting the program again with the software renderer did not work: {{.Error}}.", map[string]any{"Error": withoutFullStop(err)})
}

// RendererNotLoaded is what the window says when it was asked to draw with
// the renderer and could not load it. On standard error as it happens, and
// in the refusal if the driver then refuses too.
func RendererNotLoaded(err error) string {
	return sayf("RendererNotLoaded", "The software renderer shipped beside the program could not be loaded: {{.Error}}.", map[string]any{"Error": withoutFullStop(err)})
}

// withoutFullStop is an error's words without the full stop Windows ends
// its own with, for a sentence that supplies its own. Measured on
// 2026-09-17: "The specified module could not be found." arrived with one
// and the sentence around it ended with two.
func withoutFullStop(err error) string {
	return strings.TrimSuffix(strings.TrimSpace(err.Error()), ".")
}

// RendererDidNotHelp is what the refusal adds when the renderer was loaded
// and the toolkit still could not open a window with it.
func RendererDidNotHelp() string {
	return say("RendererDidNotHelp", "The software renderer shipped beside the program was loaded, and the graphics toolkit still could not open a window with it.")
}

// RendererNotShipped is what --software-gl answers on a system the
// renderer does not ship for. Said rather than ignored, because a flag
// that does nothing in silence is one somebody waits on.
func RendererNotShipped() string {
	return say("RendererNotShipped", "No software renderer ships for this system. The window is drawn by the graphics driver as usual.")
}

// NotAWholeNumber refuses a box that should hold digits and does not.
//
// The field is named by its label rather than by its key, because this is read
// by somebody looking at the screen - and it arrives from here rather than as a
// literal at the call site, which is where "how many" and "seed" were until
// 2026-08-13.
func NotAWholeNumber(field, value string) string {
	return sayf("NotAWholeNumber",
		"{{.Field}} is {{.Value}}, which is not a whole number. Write the digits out, such as 1 or 500",
		map[string]any{"Field": field, "Value": strconv.Quote(value)})
}

// ManifestNotSaved is the error when the files exist and their record does
// not. The caller wraps the underlying reason onto the end.
func ManifestNotSaved(path string) string {
	return sayf("ManifestNotSaved",
		"the files were written and the manifest could not be saved to {{.Path}}",
		map[string]any{"Path": path})
}

// NothingProduced is the outcome when a run ended with no manifest at all.
func NothingProduced() string { return say("NothingProduced", "Nothing was produced.") }

// StoppedAfter is the outcome of a run that was cancelled or failed part way.
func StoppedAfter(written int) string {
	return sayf("StoppedAfter", "Stopped after {{.Files}}. The manifest describes exactly those.",
		map[string]any{"Files": files(written)})
}

// WrittenWithFailures is the outcome when some files could not be produced.
// Untouchable rule 6: this has to be visible in the window and not only in the
// manifest, because "the manifest says which ones" is an answer in a terminal
// and an instruction to open a ten thousand entry file in a window.
func WrittenWithFailures(written, failed int) string {
	return sayf("WrittenWithFailures", "{{.Files}} written, {{.Failed}} could not be produced.",
		map[string]any{"Files": files(written), "Failed": failed})
}

// Written is the outcome of a run where everything asked for was produced.
func Written(written int) string {
	return sayf("Written", "{{.Files}} written.", map[string]any{"Files": files(written)})
}
