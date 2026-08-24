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

	bar     *parts.Progress
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

	// stop ends the run in progress and waits for it, and is only ever touched
	// on the interface thread.
	//
	// It is deliberately left in place once a run has ended rather than
	// cleared, and that is a threading decision rather than an oversight.
	// Clearing it would be a write from the worker under the toolkit's test
	// driver, where fyne.Do runs on the calling goroutine - so -race would
	// report a race that production does not have, and the honest way out is a
	// handle that is safe to hold on to. Calling it after the run has ended
	// cancels a finished context and receives from a closed channel, both of
	// which return at once.
	//
	// So it says nothing about whether a run is going. It used to be asked
	// that, and the answer was wrong from the second run onwards - see running.
	stop func()

	// running is whether a run owns the screen, and it exists because stop
	// cannot answer that. Asking stop was a real defect and a quiet one: it is
	// set on the first Generate and never cleared, so from then on every live
	// check returned at its first line. Fields stopped being checked while
	// being typed in and stopped being unmarked once corrected, and the screen
	// looked exactly the same doing it.
	//
	// Only ever touched on the interface thread, which is what makes a plain
	// bool enough - setRunning is called from there, and the worker gets back
	// through fyne.Do before it reaches this.
	running bool

	// alsoDisabled are controls that are neither fields nor run buttons and
	// still have no business being pressed while a run is going. The batch
	// screen's "add a batch" is one: pressing it rebuilds the form under a run.
	alsoDisabled []fyne.Disableable

	// scroll is the part of this screen that moves, so a refusal can bring the
	// box it is about into view. Set by the screen, because only the screen
	// that built it knows which scroll holds its form.
	scroll *container.Scroll

	// destination is where this screen would write, asked for rather than
	// stored, because the box it comes from is edited after this is wired.
	// Nil on a screen that has no such box, and the status line simply stays
	// empty there.
	destination func() string

	// resting is true while nothing pressed has spoken. The status line
	// carries the destination until then and whatever was pressed afterwards.
	//
	// One way only, and that is worth knowing before relying on it: nothing
	// sets it back. So the destination line is live until the first press and
	// never again, and the call to sayDestination in the live check is doing
	// something only for that first stretch.
	//
	// It was tempting to make typing restore it, and that is wrong for a
	// measured reason - see sayDestination. A refusal puts its sentence on this
	// same line, and the live check runs on the next keystroke, so restoring
	// here would wipe the sentence somebody had just been given.
	//
	// The consequence left standing: after a preview, editing a field leaves a
	// cost on the line that was worked out for the old values. The answer to
	// that is a summary that is always live rather than a sentence that has to
	// be defended - the report calls it the permanent line in the action bar,
	// and this whole mechanism goes when that lands.
	resting bool

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
	// The first box a refusal lands on, so the form can be brought to it. A
	// refusal that marks a box the person cannot see reads as a button that did
	// nothing - see parts.Reveal and O107.
	first := ""
	for _, one := range spread(err) {
		// An interface rather than a case per error type, so a screen shown a
		// kind of refusal nobody thought about here still gets it placed. The
		// engine, the format registry and the preset package all answer this
		// and none of them had to be imported for the question to be asked.
		var about interface{ AboutSetting() string }
		if errors.As(one, &about) && about.AboutSetting() != "" &&
			r.fields.Mark(about.AboutSetting(), one) {
			if first == "" {
				first = about.AboutSetting()
			}
			continue
		}
		// About the run rather than about one box, or about a setting this
		// screen does not draw. The foot of the form is where those belong.
		loose = append(loose, one.Error())
	}
	if len(loose) > 0 {
		r.problem.Say(strings.Join(loose, "\n\n"))
	}
	if first == "" {
		return
	}
	// Every problem went onto a box, so the foot of the form says nothing and
	// the only sign the press was even received is a red box that may be off
	// the screen. Both halves of the answer are here: a sentence where the
	// button is, and the form moved to the first box that needs attention.
	if len(loose) == 0 {
		r.say(text.RefusedBeforeWriting)
	}
	if field := r.fields.Lookup(first); field != nil {
		parts.Reveal(r.scroll, field.Control)
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
	if joined, several := err.(interface{ Unwrap() []error }); several {
		var out []error
		for _, one := range joined.Unwrap() {
			out = append(out, spread(one)...)
		}
		return out
	}
	// A join under a single wrapper is still a join. The type assertion above
	// only sees the outermost layer, so one fmt.Errorf("%s: %w", ...) anywhere
	// on the way here turns five marked boxes back into one paragraph at the
	// foot of the form - which is the defect this function exists to prevent.
	//
	// Nothing wraps a join today: settle returns errors.Join straight out on
	// both screens. So this is not fixing anything that is broken, it is
	// removing the way it comes back - and it comes back silently, because the
	// guards for marking all build their errors with a bare join.
	if inner := errors.Unwrap(err); inner != nil {
		if _, several := inner.(interface{ Unwrap() []error }); several {
			return spread(inner)
		}
	}
	return []error{err}
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
	if r.running {
		return
	}
	// Typing in the destination box moves the destination, and the line saying
	// where the files go is worth nothing if it names the old one.
	//
	// This only reaches the line while nothing pressed has spoken - see
	// resting. Said here because the call reads as though it always does.
	r.sayDestination()
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
			r.fields.Mark(setting, one)
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

// say puts a sentence on the status line, above whatever settling had to say.
//
// A line with nothing on it takes no room, the same rule the error area
// follows. A label holding the empty string still reserves its height, so an
// idle screen was paying for a sentence nobody had written yet - measured at
// roughly fifty pixels of the action bar on a screen that scrolls, which is
// space taken from the form to say nothing.
//
// The notes used to come first, and that was right while this box grew to fit
// whatever was in it. It stopped being right on 2026-08-20, when the room here
// became a ceiling and the message started scrolling inside it: the first line
// is the only one certain to be read, and a note about a default we invented is
// not the line somebody is waiting for. Looked at rather than reasoned about -
// a finished run showed "no limit was given, so this set is built around
// 10mb..." with "7 files written." out of sight below it.
func (r *runner) say(lines ...string) {
	said := strings.Join(append(append([]string{}, lines...), r.notes...), "\n")
	if said == "" {
		r.sayDestination()
		return
	}
	r.resting = false
	// Back to the ordinary colour unless the caller says otherwise. Anything
	// coloured is coloured about one run, so it has to be cleared by the next
	// thing said - otherwise a green line from a finished run stays green over
	// the progress of the one after it.
	r.status.Importance = widget.MediumImportance
	r.status.SetText(said)
	r.status.Show()
}

// toneOfOutcome colours what a finished run said.
//
// The strongest moment this program has was drawn more weakly than anything
// else on the screen. Measured off a render on 2026-08-20: "3 files written."
// came out at #E6E6E6, the same grey, the same size and the same weight as
// the neutral line that names the output folder - while a refusal is four
// lines of red. The screen shouted about a mistake and whispered about
// success.
//
// The colours were already there and already measured. ColorNameSuccess and
// ColorNameWarning have been in both palettes since the palette was written
// and were used by nothing at all - checked across the whole tree on
// 2026-08-20, the only semantic colour reaching the screen was the error red.
//
// Three outcomes rather than two, because "written with failures" is not
// success and is not a refusal either. Silence is banned here (untouchable
// rule 6), so a run that skipped files has to look different from one that did
// not, and amber is the palette's word for that.
//
// Colour is never the only carrier - UX1 - and it is not one here either: the
// sentences already differ. What colour adds is which of the three a person is
// looking at, before reading it.
//
// A mark in front of the words was considered and left out. It would either go
// into the message, where it meets the ASCII rules and the guard that pins
// what the first line says, or beside it, where it pushes the line off the
// left edge every other line on the screen stands on.
func (r *runner) toneOfOutcome(res *engine.Result, runErr error) {
	switch {
	case runErr != nil:
		r.status.Importance = widget.WarningImportance
	case res == nil || res.Manifest == nil:
		r.status.Importance = widget.WarningImportance
	case res.Failures > 0:
		r.status.Importance = widget.WarningImportance
	default:
		r.status.Importance = widget.SuccessImportance
	}
	r.status.Refresh()
}

// sayDestination puts where the files will go on the status line, while there
// is nothing louder to put there.
//
// The line is kept clear for a run whether or not there is one, so at rest it
// was empty space in the one part of the screen that never scrolls away - and
// the destination is the one field that is off the bottom of every form when
// the window opens. It is also the only field that decides where somebody
// else's disk gets written to, so the cost of not seeing it is not symmetric
// with the cost of not seeing the others (O102).
//
// It gives way to anything a run has to say and does not come back, because a
// preview and an outcome are answers to something that was just pressed. A
// preview names the same directory in its own sentence anyway.
func (r *runner) sayDestination() {
	// Once something louder has spoken, this says nothing at all - including
	// not clearing what was said. Clearing was the first version and it was
	// wrong in a way only the stored screens caught: a refusal put its sentence
	// at the foot, the next keystroke ran the live check, and the check called
	// this, which wiped the sentence somebody had just been given.
	if !r.resting {
		return
	}
	if r.destination == nil {
		r.status.SetText("")
		r.status.Hide()
		return
	}
	dir := strings.TrimSpace(r.destination())
	if dir == "" {
		r.status.SetText("")
		r.status.Hide()
		return
	}
	r.status.SetText(text.WritingTo(dir))
	r.status.Show()
}

func newRunner() *runner {
	r := &runner{fields: parts.NewFields(), resting: true}
	// Wired once, here, so that a field added later is covered without anybody
	// remembering to wire it. See Fields.WhenTypedIn and recheck.
	r.fields.WhenTypedIn(r.recheck)
	r.bar = parts.NewProgress()
	// Counted as a percentage rather than as bytes, so the arithmetic that keeps
	// a very large run inside the range of its own type is the one the command
	// line already uses.
	r.bar.Max = 100
	// Nothing is written inside the track, which is now a property of the
	// control rather than a formatter turned off: the line under it ends with
	// the same percentage already (text.Progress), so the number stood on the
	// screen twice.
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
func (r *runner) actions() fyne.CanvasObject {
	// Centred, on the owner's decision of 2026-08-19, which reverses the one of
	// 2026-08-18 that put them at the right edge. Both were reports from
	// looking at the built window, and the reasoning for the first is kept
	// above rather than deleted because it was not wrong - it was a choice, and
	// this is a different one.
	//
	// A spacer at each end rather than one, because a single greedy spacer only
	// pushes: it can put the group at one end or the other and never in the
	// middle.
	//
	// Everything that is not one of these buttons went to the bar's rail on
	// 2026-08-19 - see parts.ActionBar. It used to be laid over this row, which
	// kept it inside the form's column and so a margin away from the edge.
	return container.NewHBox(
		layout.NewSpacer(), r.previewBtn, r.generateBtn, r.cancelBtn, layout.NewSpacer())
}

// rail is what stands at the left edge of the action bar: the buttons that are
// not about this run. See parts.ActionBar.
func rail(items ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(items...)
}

// progress is where a run says what it is doing, and it keeps its height
// whether or not there is a run. See parts.WithRoomForARun for why.
func (r *runner) progress() fyne.CanvasObject {
	return parts.WithRoomForARun(container.NewVBox(r.bar, r.status))
}

// onPreview says what the run would cost and writes nothing.
//
// It goes all the way through engine.Run with DryRun set rather than stopping
// after the plan, and that is the difference between a preview and a guess: the
// checks for free space and for a name already taken live in the same preflight
// the real run goes through, so the answer here is the answer there. Stopping
// early used to promise success to a run that refused to start on the next line.
// The preflight it goes through is disk work - free space, and every name it
// would take - so this crosses to a worker rather than doing it here. It used
// to run on the interface thread, and on a large set or a directory on a
// network share that is a window which stops drawing: no bar, no way out, and
// both buttons still looking pressable because nothing had said the screen was
// busy. Nobody measured how long it takes, which is the point - the answer
// depends on somebody else's disk.
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

	// Occupied, but with nothing to cancel and nothing to put on the bar. See
	// setBusy - both of those are properties of a dry run that were measured.
	r.setBusy(true, false)
	r.say(text.WorkingOutTheCost)

	done := make(chan struct{})

	// Waited for on the way out, even though a preview writes nothing and there
	// would be no half written file to protect. What it does do is touch
	// widgets when it comes back, so a preview still in flight after the window
	// has gone is a worker drawing into a screen that is being torn down.
	//
	// Waiting is all this can do. preflight takes no context, so there is
	// nothing to cancel - and that is why the button is not offered either.
	// Before this change the same wait happened on every preview and blocked
	// the whole window rather than only the closing of it.
	//
	// Set before the goroutine starts, for the reason startRun spells out.
	r.stop = func() { <-done }

	go func() {
		_, runErr := engine.Run(context.Background(), planned, opt)
		// Do rather than DoAndWait, for the same reason startRun gives: the
		// interface thread must never be left waiting on a worker.
		fyne.Do(func() { r.previewFinished(planned, opt, runErr) })
		close(done)
	}()
}

// previewFinished is the end of a preview, back on the interface thread.
func (r *runner) previewFinished(planned []engine.PlannedFile, opt engine.Options, runErr error) {
	r.setBusy(false, false)
	if runErr != nil {
		r.refuse(runErr)
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

	// Cancelling and waiting, in that order, is the whole of G7. Closing the
	// window is not a signal, so nothing else brings the run to a stop - and
	// ending the process without waiting would leave it somewhere inside a file
	// with no manifest, which is the one thing the output directory is promised
	// never to hold.
	//
	// Set BEFORE the goroutine starts, and that ordering is the whole point.
	// The other way round there is a window - short, and real - where files are
	// being written and this is still nil, so closing the window in it finds
	// nothing to call, waits for nothing, and ends the process somewhere inside
	// a file. ctx, cancel and done all exist already, so there is nothing to
	// gain by waiting.
	r.stop = func() {
		cancel()
		<-done
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

// runFinished is the end of a run, back on the interface thread.
//
// Note what it does not do: clear stop. That is deliberate and the reason is at
// the declaration of the field.
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
	r.toneOfOutcome(res, runErr)
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
func (r *runner) setRunning(running bool) { r.setBusy(running, running) }

// setBusy is the same thing with the two halves told apart: whether the screen
// is occupied, and whether there is anything to interrupt.
//
// They came apart when the preview stopped blocking the interface thread. A
// preview occupies the screen exactly as a run does - both buttons off, the
// form frozen - but it has nothing to show on the bar and nothing to cancel,
// and both of those are measured rather than assumed:
//
//   - A dry run returns before the writing loop, so OnProgress never fires. A
//     bar shown for it would sit at nought until the answer arrived, which is
//     what a stuck run looks like.
//   - preflight takes no context. It is where a preview spends its time, and it
//     cannot be interrupted, so a Cancel offered here would be a button that
//     does nothing while looking like the way out.
func (r *runner) setBusy(busy, stoppable bool) {
	// The state itself, before any of the controls. Everything below is what
	// the state looks like - this is the state, and it is what the live check
	// asks. See the running field.
	r.running = busy
	// Cancel is hidden rather than greyed when there is nothing to cancel, asked
	// for on 2026-08-11 after looking at the window. A permanently dead control
	// is a question the screen keeps asking and answering itself, and it sat
	// beside the two buttons that do work - so the row read as three choices
	// where there were two. It appears with the run and goes with it.
	// The form goes with them. It stayed editable through a run, so somebody
	// could change the output directory while files were going into the old
	// one and nothing said which run that applied to - almost certainly none of
	// them, which is exactly the answer a person cannot reach from looking
	// (O106).
	r.fields.Freeze(busy)
	for _, control := range r.alsoDisabled {
		if busy {
			control.Disable()
			continue
		}
		control.Enable()
	}

	if busy {
		r.previewBtn.Disable()
		r.generateBtn.Disable()
	} else {
		r.previewBtn.Enable()
		r.generateBtn.Enable()
	}

	// Only a run gets these two. See the note above for why a preview gets
	// neither, and note that both are put away whenever the screen goes idle -
	// so a preview started after a run cannot leave a stale bar behind.
	if busy && stoppable {
		r.cancelBtn.Enable()
		r.cancelBtn.Show()
		r.bar.Show()
		return
	}
	r.cancelBtn.Disable()
	r.cancelBtn.Hide()
	r.bar.Hide()
}

// keepScroll remembers the scrolling area on the way past, so that a refusal
// can bring the box it is about into view. It hands the scroll straight back,
// so a screen wires it by wrapping the call it was already making rather than
// by finding somewhere to put an extra statement.
func (r *runner) keepScroll(scroll *container.Scroll) *container.Scroll {
	r.scroll = scroll
	return scroll
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
