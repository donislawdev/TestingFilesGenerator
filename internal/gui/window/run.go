package window

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The one place in the window where work happens off the interface thread, and
// the only file of this tree in the concurrency guard's list.
//
// It is one goroutine and one channel. The goroutine exists because engine.Run
// writes files and a window that waits for it is a window the desktop reports
// as not responding. The channel exists because closing the window has to wait
// for that goroutine to wind down - cancelling and exiting immediately would
// end the process somewhere inside a file, which is the invariant G7 is about.
//
// Everything the interface is told from in here crosses back through fyne.Do.
// Measured on 2026-08-05 and worth writing down, because it decides what a test
// can prove: under the toolkit's test driver fyne.Do runs the function on the
// calling goroutine, and a widget touched straight from a worker does not
// complain either. So neither the test driver nor -race can catch a missing
// fyne.Do, and the guard for it is a static one that reads this file. See O63.

// progressInterval is how often the bar may be redrawn. The engine reports far
// more often than this, once per write inside a file, and thinning that out
// before it reaches a screen is the caller's job.
const progressInterval = 100 * time.Millisecond

// throttle passes the first report through and then at most one per interval.
//
// The first one matters and is the part that was got wrong elsewhere: a limiter
// started with "now" suppresses everything until the interval has passed, so a
// run shorter than that draws nothing at all - which is both a bar that never
// appears and a guard that passes while proving nothing. The command line hit
// exactly that, and it was mutation that found it rather than reading.
type throttle struct {
	last time.Time
}

func (t *throttle) allow(now time.Time) bool {
	if !t.last.IsZero() && now.Sub(t.last) < progressInterval {
		return false
	}
	t.last = now
	return true
}

// settler is how a screen says what it wants produced.
//
// It is the only thing that differs between the screens: one builds a single
// target from its fields, the other expands a named preset. Everything after it
// - the plan, the preview, the run, the bar, cancelling, the manifest - is the
// same work and lives once, in the runner below.
type settler func() ([]engine.Target, engine.Options, error)

// runner is the half of a screen that runs the engine and shows how it is
// going. Both screens have one.
//
// Written as a shared piece the moment there was a second screen rather than
// after the two had drifted. The parts that would have drifted are named in G7
// and are exactly the parts nobody would think to compare: whether closing the
// window waits, whether a preview writes, whether a manifest is saved when the
// run was cut short.
type runner struct {
	settle settler

	previewBtn  *widget.Button
	generateBtn *widget.Button
	cancelBtn   *widget.Button

	bar     *widget.ProgressBar
	status  *widget.Label
	problem *parts.ErrorArea

	// fields is every box on the screen, by the setting the engine names it by.
	// A refusal that says which setting it is about lands under that box
	// instead of at the foot of the form - UX8, and O73, where the message
	// about "how many" sat 748 px below the field it named.
	//
	// A registry rather than a map filled in at the call sites, changed on
	// 2026-08-12. The map was real and it was filled in for five fields of
	// eight, with every setting a format declares left out - so whether a box
	// could be marked depended on which of two functions somebody typed when
	// they added it. Fields.Add is the only way to build a field now.
	fields *parts.Fields

	// stop ends the run in progress and waits for it. Nil while nothing is
	// running, and only ever touched on the interface thread.
	stop func()

	// notes is what settling had to say out loud, set by settle rather than
	// worked out here. Silence is banned: a set built around a limit we
	// invented carries expectations that read exactly like a set built around
	// the real one, so the run says which number it made up. The command line
	// prints these as "note:" lines and this is the window's half of it.
	notes []string
}

// Fields is every box on this screen, for a guard to compare against the tree.
//
// Exported for one question and it is the question this design exists to
// answer: is there a control on the screen that is not in here. A control the
// registry does not know about is a control whose refusal has nowhere to go,
// and counting them from the registry alone could never find one.
func (r *runner) Fields() *parts.Fields { return r.fields }

// refuse shows a refusal, under the field it is about where it names one.
//
// The choice is the engine's rather than the window's. It sets the setting on
// the error, and this looks it up - the alternative was matching the wording
// here, which is a second copy of rules the engine owns and the copy that
// drifts.
func (r *runner) refuse(err error) {
	var loose []string
	for _, one := range spread(err) {
		// An interface rather than a case per error type, so a screen shown a
		// kind of refusal nobody thought about here still gets it placed. The
		// engine, the format registry and the preset package all answer this
		// and none of them had to be imported for the question to be asked.
		var about interface{ AboutSetting() string }
		if errors.As(one, &about) && about.AboutSetting() != "" &&
			r.fields.Mark(about.AboutSetting(), one.Error()) {
			continue
		}
		// About the run rather than about one box, or about a setting this
		// screen does not draw. The foot of the form is where those belong.
		loose = append(loose, one.Error())
	}
	if len(loose) > 0 {
		r.problem.Say(strings.Join(loose, "\n\n"))
	}
}

// spread opens a refusal that carries several into the ones it carries.
//
// The window used to mark ONE box however many were wrong, because everything
// between the screen and the field registry was singular by type: settle
// returned at the first bad box, refuse took one error, Mark marked one field.
// Reported from the screen on 2026-08-18, and it is the window narrowing what
// the layer below already does - RC7 has the engine refuse a recipe with every
// problem it has rather than the first, on the grounds that fixing a file one
// error per run is the cheapest way to make somebody stop using the tool. The
// same argument applies to a form.
//
// errors.Join is what carries them, so nothing here has to be a new error type
// and a single refusal still arrives as itself. Walked rather than flattened
// once, because a join can hold a join - the preset screen collects its own and
// hands on whatever the recipe parser gave it.
func spread(err error) []error {
	if err == nil {
		return nil
	}
	joined, several := err.(interface{ Unwrap() []error })
	if !several {
		return []error{err}
	}
	var out []error
	for _, one := range joined.Unwrap() {
		out = append(out, spread(one)...)
	}
	return out
}

// recheck says what is wrong with the screen while somebody is still typing.
//
// Asked for from the screen on 2026-08-18: a bad value should turn its box red
// and give the reason straight away, rather than waiting for a button. It runs
// the SAME settle the buttons run, which is the whole of the design - there is
// no second set of rules to write, nothing to keep in step, and a box that can
// be refused is refused here because it is refused there. A field nobody has
// added a rule for needs no rule added.
//
// Two things it does not do. It leaves the foot of the form alone, because a
// complaint about the run rather than about a box is not something to shout
// while somebody is mid-word. And it says nothing about an empty box - see
// Fields.Blank.
func (r *runner) recheck(setting string) {
	// Nothing to check against yet, during the screen being built.
	if r.settle == nil {
		return
	}
	// A run owns the screen while it lasts. Its progress and its refusals are
	// not to be wiped by a keystroke.
	if r.stop != nil {
		return
	}
	// Only this box, in both directions. What the other boxes were told is
	// about values nobody has just changed, and it is still true - including
	// the parts of it this cannot see, because a format minimum and a name
	// already taken are the engine's answers rather than settle's.
	r.fields.Clear(setting)
	if r.fields.Blank(setting) {
		return
	}
	_, _, err := r.settle()
	for _, one := range spread(err) {
		var about interface{ AboutSetting() string }
		if errors.As(one, &about) && about.AboutSetting() == setting {
			r.fields.Mark(setting, one.Error())
			return
		}
	}
}

// clearProblems empties every place a refusal can appear, not just the last one
// used. Clearing only the foot of the form would leave a message under a field
// after the value that caused it was fixed.
func (r *runner) clearProblems() {
	r.problem.Clear()
	r.fields.ClearAll()
}

// say puts a sentence on the status line, under whatever settling had to say.
//
// A line with nothing on it takes no room, the same rule the error area
// follows. A label holding the empty string still reserves its height, so an
// idle screen was paying for a sentence nobody had written yet - measured at
// roughly fifty pixels of the action bar on a screen that scrolls, which is
// space taken from the form to say nothing.
func (r *runner) say(lines ...string) {
	said := strings.Join(append(append([]string{}, r.notes...), lines...), "\n")
	r.status.SetText(said)
	if said == "" {
		r.status.Hide()
		return
	}
	r.status.Show()
}

func newRunner() *runner {
	r := &runner{fields: parts.NewFields()}
	// Wired once, here, so that a field added later is covered without anybody
	// remembering to wire it. See Fields.WhenTypedIn and recheck.
	r.fields.WhenTypedIn(r.recheck)
	r.bar = widget.NewProgressBar()
	// Counted as a percentage rather than as bytes, so the arithmetic that keeps
	// a very large run inside the range of its own type is the one the command
	// line already uses.
	r.bar.Max = 100
	r.bar.Hide()

	r.status = widget.NewLabel("")
	r.status.Wrapping = fyne.TextWrapWord
	// Nothing to say yet, so nothing takes up room. See say.
	r.status.Hide()
	r.problem = parts.NewErrorArea()

	r.previewBtn = widget.NewButton(text.ButtonPreview, r.onPreview)
	r.generateBtn = widget.NewButton(text.ButtonGenerate, r.onGenerate)
	r.generateBtn.Importance = widget.HighImportance
	// Three ranks, so the eye lands on the one that does the work: Generate
	// filled, Preview plain beside it, Cancel receding until there is something
	// to cancel. They were three identical buttons in a row, which is a choice
	// presented as no choice.
	// Cancel is a button and looks like one. LowImportance draws no surface at
	// all, so it arrived as bare words beside two filled buttons - which reads
	// as a disabled label rather than as the way to stop a run, and it is the
	// one control on the screen somebody reaches for in a hurry. Measured on
	// the running screen on 2026-08-12: no fill, no border, nothing to aim at
	// but the text.
	//
	// Medium rather than high, which is the zero value and the plain filled
	// button. The rank it needs is "as pressable as Preview and not competing
	// with Generate", and Generate is disabled while this one is showing
	// anyway.
	r.cancelBtn = widget.NewButton(text.ButtonCancel, r.onCancel)
	r.cancelBtn.Disable()
	r.cancelBtn.Hide()
	return r
}

// actions puts Preview before Generate, and that order is G6.
//
// It is the one thing a window does better than a command line rather than
// merely as well: --dry-run has to be known about and remembered, and this is
// on the way to the button beside it. With presets running to several gigabytes
// and disks that are not always emptier than that, it is not decoration.
// actions is the bar at the foot of the form. The buttons that run something go
// at the RIGHT edge of the column, and anything else stays at the left.
//
// Measured from the stored tree on 2026-08-18 before this changed: the bar is
// 820 px wide and the two buttons stood at x=0 and x=73. Nobody had chosen that
// - it was where an HBox puts things - and the owner asked why they sat over
// there. Decided on 2026-08-18: the end of the reading path, which is where a
// form with a fixed action bar puts the thing it wants pressed last. Cancel is
// hidden until a run starts, so at rest the rightmost button is Generate.
//
// The spacer is what does it. An HBox gives every child its minimum width and
// leaves the rest empty at the end, so without something greedy in front the
// buttons cannot move.
func (r *runner) actions(extra ...fyne.CanvasObject) fyne.CanvasObject {
	all := append([]fyne.CanvasObject{}, extra...)
	all = append(all, layout.NewSpacer(), r.previewBtn, r.generateBtn, r.cancelBtn)
	return container.NewHBox(all...)
}

func (r *runner) progress() fyne.CanvasObject {
	return container.NewVBox(r.bar, r.status)
}

// onPreview says what the run would cost and writes nothing.
//
// It goes all the way through engine.Run with DryRun set rather than stopping
// after the plan, and that is the difference between a preview and a guess: the
// checks for free space and for a name already taken live in the same preflight
// the real run goes through, so the answer here is the answer there. Stopping
// early used to promise success to a run that refused to start on the next line.
func (r *runner) onPreview() {
	r.clearProblems()
	targets, opt, err := r.settle()
	if err != nil {
		r.refuse(err)
		return
	}
	opt.DryRun = true

	planned, err := engine.Plan(targets, opt)
	if err != nil {
		r.refuse(err)
		return
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		r.refuse(err)
		return
	}
	r.say(previewText(planned, opt.OutDir))
}

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

// formatsOf is what kinds of file the run would produce, each named once.
//
// Read off the plan rather than off the screen, which is what makes it true on
// both screens. The generate screen has the answer in a menu, and the preset
// screen does not have it anywhere: a preset states its own targets, so the
// only place that knows is the plan they came to. A set built in a format
// nobody chose is exactly the thing somebody wants to catch before it is
// written.
//
// Sorted, so a run of several kinds reads the same way twice. Plans keep the
// order of the targets they came from, and that order is not something to show
// somebody as if it meant anything.
func formatsOf(planned []engine.PlannedFile) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range planned {
		if id := p.Desc.ID; id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// onGenerate plans on the interface thread and writes off it.
//
// Planning is fast enough to do here - measured at 15.7 ms for ten thousand
// files - and doing it here is what lets a refusal appear with nothing started
// and no buttons to put back.
func (r *runner) onGenerate() {
	r.clearProblems()
	targets, opt, err := r.settle()
	if err != nil {
		r.refuse(err)
		return
	}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		r.refuse(err)
		return
	}
	r.startRun(planned, opt)
}

func (r *runner) startRun(planned []engine.PlannedFile, opt engine.Options) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	started := time.Now()
	limit := &throttle{}

	r.setRunning(true)
	r.bar.SetValue(0)
	r.say(text.WritingFiles(len(planned)))

	// Called by the engine from the goroutine below, once per write inside a
	// file. Thinned out here, before anything crosses over, so the interface is
	// asked to redraw ten times a second rather than thousands.
	opt.OnProgress = func(p engine.Progress) {
		if !limit.allow(time.Now()) {
			return
		}
		elapsed := time.Since(started)
		fyne.Do(func() {
			r.bar.SetValue(float64(core.Percent(p.BytesDone, p.BytesTotal)))
			r.status.SetText(progressText(p, elapsed))
		})
	}

	go func() {
		res, runErr := engine.Run(ctx, planned, opt)
		// The manifest is written here rather than after crossing back, because
		// it is disk work and the interface thread is the one thing that must
		// not wait on a disk.
		saveErr := saveManifest(res, opt)
		// Do rather than DoAndWait. The interface thread may already be inside
		// stop, waiting on the channel closed below, and a worker waiting for
		// that thread to run something would be both of them waiting.
		fyne.Do(func() { r.runFinished(res, runErr, saveErr) })
		close(done)
	}()

	// Cancelling and waiting, in that order, is the whole of G7. Closing the
	// window is not a signal, so nothing else brings the run to a stop - and
	// ending the process without waiting would leave it somewhere inside a file
	// with no manifest, which is the one thing the output directory is promised
	// never to hold.
	r.stop = func() {
		cancel()
		<-done
	}
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
	path := filepath.Join(opt.OutDir, opt.ManifestName)
	if err := res.Manifest.Save(path); err != nil {
		return fmt.Errorf("%s: %w", text.ManifestNotSaved(path), err)
	}
	return nil
}

// stop is deliberately left in place rather than cleared here, and that is a
// threading decision rather than an oversight. Clearing it would be a write
// from the worker under the toolkit's test driver, where fyne.Do runs on the
// calling goroutine - so -race would report a race that production does not
// have, and the honest way out is a handle that is safe to hold on to. Calling
// it after the run has ended cancels a finished context and receives from a
// closed channel, both of which return at once.
func (r *runner) runFinished(res *engine.Result, runErr, saveErr error) {
	r.setRunning(false)

	switch {
	case runErr != nil:
		r.refuse(runErr)
	case saveErr != nil:
		r.refuse(saveErr)
	}

	// Silence is banned. A file that was not produced has to be visible here
	// and not only in the manifest - "the manifest says which ones" is an
	// answer in a terminal and an instruction to open a file with ten thousand
	// entries in a window.
	r.say(append([]string{outcomeText(res, runErr)}, notesOf(res)...)...)
}

func outcomeText(res *engine.Result, runErr error) string {
	if res == nil || res.Manifest == nil {
		return text.NothingProduced
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

// setRunning is the screen in one state or the other. Two buttons that both
// look pressable during a run is a window that invites a second run into a
// directory the first one is still filling.
func (r *runner) setRunning(running bool) {
	// Cancel is hidden rather than greyed when there is nothing to cancel, asked
	// for on 2026-08-11 after looking at the window. A permanently dead control
	// is a question the screen keeps asking and answering itself, and it sat
	// beside the two buttons that do work - so the row read as three choices
	// where there were two. It appears with the run and goes with it.
	if running {
		r.previewBtn.Disable()
		r.generateBtn.Disable()
		r.cancelBtn.Enable()
		r.cancelBtn.Show()
		r.bar.Show()
		return
	}
	r.previewBtn.Enable()
	r.generateBtn.Enable()
	r.cancelBtn.Disable()
	r.cancelBtn.Hide()
	r.bar.Hide()
}

func (r *runner) onCancel() { r.Stop() }

// Stop ends a run in progress and waits for it, or does nothing when there is
// none. Safe to call at any time, which is what lets one close intercept ask
// every screen without knowing which of them is busy.
func (r *runner) Stop() {
	if r.stop != nil {
		r.stop()
	}
}

// wholeNumber reads a plain count out of a box.
//
// Turning text into a number is not one of this tool's rules - the command line
// gets it from the flag package - so this is not a second copy of anything G1
// protects. What the number then has to satisfy stays with the engine.
func wholeNumber(setting, field, value string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, &aboutField{setting: setting, detail: text.NotAWholeNumber(field, value)}
	}
	return n, nil
}

// aboutField is a refusal the WINDOW made, carrying the box it is about.
//
// The engine says which setting its own refusals are about and the window threw
// that away for its own: reading a size or a count out of a box is the window's
// work, so "abc is not a whole number" arrived as a plain error with no
// subject, went to the foot of the form, and marked nothing. Measured on
// 2026-08-12 - the size field could be marked when a format refused the number
// and not when the number was not a number, which is the more common mistake of
// the two.
type aboutField struct {
	setting string
	detail  string
}

func (e *aboutField) Error() string        { return e.detail }
func (e *aboutField) AboutSetting() string { return e.setting }

// saying wraps a refusal from somewhere that does not know which box was read.
// core.ParseSize is shared with the command line, where there are no boxes.
func saying(setting string, err error) error {
	if err == nil {
		return nil
	}
	var already interface{ AboutSetting() string }
	if errors.As(err, &already) && already.AboutSetting() != "" {
		return err
	}
	return &aboutField{setting: setting, detail: err.Error()}
}
