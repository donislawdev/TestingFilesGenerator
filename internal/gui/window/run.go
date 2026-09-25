package window

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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

	previewBtn  *parts.Button
	generateBtn *parts.Button
	// offer is what the finished run left and the way to each of it: the
	// folder, the record, and the rule about when each is on the bar.
	//
	// In the row of actions rather than beside the output box, so the bar keeps
	// its height and the form does not move - the property
	// TestTheFormDoesNotMoveWhenARunStarts holds. See offers.
	offer *offers

	// busy is whether work owns the screen and the face it wears for it -
	// the frozen form, Cancel, the bar. See runbusy.go.
	busy    *busy
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

	// hold, when a guard supplies one, is run on the worker just before the
	// screen is told the work has ended. Nil in the shipped program - see
	// HoldBeforeFinishing for what it is for and why nothing else can do it.
	hold func()

	// stop ends the run in progress and waits for the worker to finish, and is
	// only ever touched on the interface thread.
	//
	// "Waits for the worker" is as far as it reaches, and the limit is worth
	// writing down because G7 gets read out of this sentence. The worker closes
	// the channel this waits on immediately after handing its last piece of
	// work to fyne.Do, and fyne.Do QUEUES that work rather than running it -
	// DoAndWait there would be the interface thread waiting on a worker that is
	// waiting on the interface thread. So when stop returns, the worker is
	// finished and the widget writes it asked for may still be in the toolkit's
	// queue.
	//
	// What that buys is what G7 asks for: nothing of ours is still running and
	// nothing more is going to the disk. What it does not buy is the screen
	// having caught up - and Settled does not buy that either, because it waits
	// on the same channel.
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
	// settled is closed when the work in flight has finished, and is what
	// Settled waits on. Separate from stop because stop CANCELS first, which is
	// right for closing the window and for Esc and wrong for anybody who wants
	// to read the answer.
	settled chan struct{}

	// scroll is the part of this screen that moves, so a refusal can bring the
	// box it is about into view. Set by the screen, because only the screen
	// that built it knows which scroll holds its form.
	scroll *container.Scroll

	// readdress moves a refusal onto the box that is on the screen, for a
	// screen that draws one of several boxes for one question.
	//
	// The batch screen chooses between three ways of stating a size and shows
	// one of them. The recipe reader words a batch with none of the three as
	// "target 1 has no size" whichever way was meant, because the size is the
	// key it looks for first - so with the switch on a range, that refusal is
	// addressed to a box nobody can see, and the screen says nothing is wrong
	// while refusing to run. Measured on 2026-08-25 by the star guard, which
	// asked whether an empty box that the run refuses is marked and found it
	// was not.
	//
	// A hook rather than knowledge inside Fields, because it is the screen with
	// the switch that knows what the switch is on, and Fields is drawn by three
	// screens that mostly do not have one. nil means the address the engine
	// gave is the address to mark.
	readdress func(string) string

	// unfold opens whatever the screen has put a box away inside, so a refusal
	// about it can be seen. Nil on a screen that folds nothing.
	unfold func(string)

	// sections are the screen's panels, each of which folds away - see
	// sections.go. Opened by a refusal as unfold is.
	sections *sections

	// destination is where this screen would write, asked for rather than
	// stored, because the box it comes from is edited after this is wired.
	// Read for the line even when the rest of the form does not settle, so
	// the one box that decides where somebody else's disk gets written to is
	// on the line whatever the other boxes say.
	destination func() string

	// line is what the status line says while nothing pressed has spoken:
	// what the form comes to, live. It replaced a line that carried the
	// destination until the first press and never again - a one way flag
	// called resting, which meant that after a preview, editing a field left
	// a cost on the line worked out for the old values. The line is worked
	// out from the form on every change, so there is nothing on it to defend:
	// a change of the form puts the summary back over whatever a press said.
	line *runLine

	// notes is what settling had to say out loud, set by settle rather than
	// worked out here. Silence is banned: a set built around a limit we
	// invented carries expectations that read exactly like a set built around
	// the real one, so the run says which number it made up. The command line
	// prints these as "note:" lines and this is the window's half of it.
	notes []string

	// touched is told at every reading of the form - see watchedSettle - so
	// that the window gives memory back once it has been left alone. Every
	// change somebody makes and every run reads the form. Nil on a screen
	// built on its own. See tidy.go.
	touched func()
}

// Fields is every box on this screen, for a guard to compare against the tree.
//
// Exported for one question and it is the question this design exists to
// answer: is there a control on the screen that is not in here. A control the
// registry does not know about is a control whose refusal has nowhere to go,
// and counting them from the registry alone could never find one.
func (r *runner) Fields() *parts.Fields { return r.fields }

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
	showOn(r.status, strings.Join(append(append([]string{}, lines...), r.notes...), "\n"))
}

// showOn puts words on the status line without the notes, or takes the
// line away when handed nothing. The line at rest goes through here: what
// settling had to say out loud belongs with a press, and at rest it made a
// second line that scrolled inside the room kept for one. A function rather
// than a method, because the runner stands at its ceiling of methods.
func showOn(status *widget.Label, said string) {
	if said == "" {
		status.SetText("")
		status.Hide()
		return
	}
	// Back to the ordinary colour unless the caller says otherwise. Anything
	// coloured is coloured about one run, so it has to be cleared by the next
	// thing said - otherwise a green line from a finished run stays green over
	// the progress of the one after it.
	status.Importance = widget.MediumImportance
	status.SetText(said)
	status.Show()
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

// refreshLine works out what the form comes to and puts it on the line.
//
// From settle rather than from a plan: settle is what the form parses to, and
// planning is what the engine does with it and can cost seconds (see
// onPreview).
//
// Settling is not free either, and this comment said it was until 2026-09-23.
// On the preset screen it expands the preset, which for upload-validation
// encodes images - 157-180 ms and 60 MB each time, measured that day
// (docs/GUI-MEMORY-2026-09-23.md section 4h). That is why a change of a box
// reads the form once for the line and for the box (recheck), and why the
// preset screen remembers what it expanded last (lastExpansion).
//
// Not while a run owns the screen: its progress is not to be overwritten by
// a summary, and the form is frozen then anyway.
func (r *runner) refreshLine() {
	if r.settle == nil || r.busy.occupied {
		return
	}
	targets, opt, err := r.settle()
	lineFrom(r, targets, opt, err)
}

// lineFrom puts on the line what one reading of the form came to. A form that
// does not settle falls back to naming the destination alone, which is read
// off its own box because it is the one fact worth having whatever the other
// boxes say.
//
// Apart from refreshLine so that recheck can hand it the reading it has
// already made. A function rather than a method, because the runner stands
// one method under its ceiling.
func lineFrom(r *runner, targets []engine.Target, opt engine.Options, err error) {
	if err != nil {
		dir := ""
		if r.destination != nil {
			dir = r.destination()
		}
		showOn(r.status, r.line.fallback(dir))
		return
	}
	showOn(r.status, r.line.said(summarise(targets), opt.OutDir))
}

func newRunner(wait later) *runner {
	r := &runner{fields: parts.NewFields(), line: &runLine{}, sections: newSections()}
	// Wired once, here, so that a field added later is covered without anybody
	// remembering to wire it. See Fields.WhenTypedIn and recheck.
	r.fields.WhenTypedIn(r.recheck)
	bar := parts.NewProgress()
	// Counted as a percentage rather than as bytes, so the arithmetic that keeps
	// a very large run inside the range of its own type is the one the command
	// line already uses.
	bar.Max = 100
	// Nothing is written inside the track, which is now a property of the
	// control rather than a formatter turned off: the line under it ends with
	// the same percentage already (text.Progress), so the number stood on the
	// screen twice.
	bar.Hide()

	r.status = widget.NewLabel("")
	r.status.Wrapping = fyne.TextWrapWord
	// Nothing to say yet, so nothing takes up room. See say.
	r.status.Hide()
	r.problem = parts.NewErrorArea()

	r.previewBtn = parts.NewButton(parts.Secondary, text.ButtonPreview(), r.onPreview).InTheBar()
	r.generateBtn = parts.NewButton(parts.Primary, text.ButtonGenerate(), r.onGenerate).InTheBar()
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
	cancel := parts.NewButton(parts.Secondary, text.ButtonCancel(), r.onCancel).InTheBar()
	cancel.Disable()
	cancel.Hide()
	r.busy = &busy{fields: r.fields, preview: r.previewBtn, generate: r.generateBtn,
		cancel: cancel, bar: bar, later: wait}

	r.offer = newOffers(r.busy.relay)
	return r
}

// rail is what stands at the left edge of the action bar: the buttons that are
// not about this run. See parts.ActionBar.
func rail(items ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(items...)
}

// roomToSpeak is where a run says what it is doing, and it keeps its height
// whether or not there is a run. See parts.WithRoomForARun for why.
//
// The refusal about the run as a whole is in here too since 2026-09-14. It
// stood outside the reserved room until then, so a refusal grew the bar and
// moved the form - the same movement the room exists to prevent, allowed for
// one kind of message because it was the kind nobody had measured.
func roomToSpeak(bar *parts.Progress, status *widget.Label, problem *parts.ErrorArea) fyne.CanvasObject {
	return parts.WithRoomForARun(container.NewVBox(bar, parts.Flush(status), problem.Object()))
}

// footer is the bar at the foot of a screen: the buttons, and under them the
// room a run speaks in - which at rest carries what the form comes to.
func (r *runner) footer(rail fyne.CanvasObject) fyne.CanvasObject {
	return parts.ActionBar(rail, r.actions(), roomToSpeak(r.busy.bar, r.status, r.problem))
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
	// The same refusal as onGenerate, for the same moment.
	if r.busy.occupied {
		return
	}
	r.clearProblems()
	targets, opt, err := r.settle()
	if err != nil {
		r.refuse(err)
		return
	}
	opt.DryRun = true

	// Occupied, and stoppable. Both of those changed on 2026-08-26.
	//
	// Planning used to happen here, on the interface thread, before the screen
	// had said it was busy. That was justified by a measurement - "15.7 ms for
	// ten thousand files" - taken on txt. Measured across formats that day, two
	// thousand files: txt 380-416 ms and png 16.5-22.8 s, because png, jpg and
	// gif encode the picture while planning. Ten thousand pictures is a minute
	// and a half of a window that does not draw.
	//
	// And it can be stopped now. preflight took no context, so there was
	// nothing to cancel and the button was deliberately not offered - which
	// also meant closing the window waited for the whole of it on the interface
	// thread. Both halves are gone: preflight checks its context per file, and
	// planning does too.
	r.busy.set(true, busyFace{stoppable: true})
	r.say(text.WorkingOutTheCost())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.settled = done

	// Waited for on the way out, even though a preview writes nothing and there
	// would be no half written file to protect. What it does do is touch
	// widgets when it comes back, so a preview still in flight after the window
	// has gone is a worker drawing into a screen that is being torn down.
	//
	// Set before the goroutine starts, for the reason startRun spells out.
	r.stop = func() {
		cancel()
		<-done
	}

	go func() {
		planned, planErr := engine.PlanContext(ctx, targets, opt)
		if planErr != nil {
			// Do rather than DoAndWait, for the same reason startRun gives: the
			// interface thread must never be left waiting on a worker.
			r.holdBeforeFinishing()
			fyne.Do(func() { r.previewFinished(nil, nil, opt, diskRoom{}, planErr) })
			close(done)
			return
		}
		res, runErr := engine.Run(ctx, planned, opt)
		// The disk is asked here, on the worker, for the reason the whole
		// preview is: on a network share the answer can take as long as the
		// share takes, and the interface thread is the one thing that must
		// not wait for it. It used to be asked after crossing back.
		room := roomOn(opt.OutDir)
		r.holdBeforeFinishing()
		fyne.Do(func() { r.previewFinished(res, planned, opt, room, runErr) })
		close(done)
	}()
}

// diskRoom is what a worker found out about the room under a directory.
// known is false when the disk could not be asked, and then nothing is said
// rather than a number invented - a disk we cannot measure is not a full one.
type diskRoom struct {
	free  int64
	known bool
}

// roomOn asks the disk under a directory how much room it has. Disk work,
// so it belongs on a worker and never on the interface thread.
func roomOn(dir string) diskRoom {
	if dir == "" {
		return diskRoom{}
	}
	free, err := core.AvailableBytes(dir)
	if err != nil {
		return diskRoom{}
	}
	return diskRoom{free: free, known: true}
}

// previewFinished is the end of a preview, back on the interface thread.
//
// The result is carried across as well as the plan, and that is what lets a
// preview warn about a record too big to read back. A dry run builds the whole
// document - see manifestReachNote - so the answer is there for the asking
// rather than something the window would have to work out for itself.
func (r *runner) previewFinished(res *engine.Result, planned []engine.PlannedFile, opt engine.Options, room diskRoom, runErr error) {
	r.busy.set(false, busyFace{})
	if runErr != nil {
		r.refuse(runErr)
		return
	}
	// The line goes exact: the plan has drawn every size a range left open,
	// and the disk has been asked. What it adds is the one thing the numbers
	// cannot say, which is that none of it exists yet.
	if room.known {
		r.line.measured(opt.OutDir, room.free)
	}
	said := r.line.said(exactly(planned), opt.OutDir) + text.AndNothingWrittenYet()
	r.say(append([]string{said}, manifestReachNote(res)...)...)
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

// onGenerate settles the form on the interface thread and does the rest off it.
//
// Planning used to happen here, and the comment saying that was fine carried a
// measurement: "15.7 ms for ten thousand files". That number is a txt number.
// Measured on 2026-08-26 with --dry-run at two thousand files: txt 380-416 ms
// against png 16.5-22.8 s, because png, jpg and gif encode the picture while
// planning and walk a ladder of sizes doing it when none is given. Ten thousand
// pictures is eighty to a hundred and ten seconds of a window that does not
// redraw, with no bar, no way out, and both buttons still looking pressable.
//
// What is left here is reading the form, which is the one thing that HAS to be
// here - the widgets belong to this thread.
func (r *runner) onGenerate() {
	// Refused by the state and not only by the button. The button is switched
	// off with the busy face, which follows the state by a moment - see
	// BusyFaceAfter - and a second press inside that moment would start a
	// second run into the directory the first is filling.
	if r.busy.occupied {
		return
	}
	r.clearProblems()
	targets, opt, err := r.settle()
	if err != nil {
		r.refuse(err)
		return
	}
	// Remembered here rather than read back afterwards, because the box on the
	// screen can be edited while a run is going and the button has to open
	// where the files ACTUALLY went. Hidden first, so a run that produces
	// nothing does not leave the offer from the run before it standing.
	r.offer.forget()
	r.offer.wroteInto = opt.OutDir
	r.startRun(targets, opt)
}

func (r *runner) startRun(targets []engine.Target, opt engine.Options) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.settled = done
	started := time.Now()
	limit := &throttle{}

	r.busy.set(true, busyFace{stoppable: true, progressing: true})
	r.busy.bar.SetValue(0)
	// The plan comes first now, so the first thing said is about working the
	// cost out rather than about writing files that are not being written yet.
	r.say(text.WorkingOutTheCost())

	// Called by the engine from the goroutine below, once per write inside a
	// file. Thinned out here, before anything crosses over, so the interface is
	// asked to redraw ten times a second rather than thousands.
	opt.OnProgress = func(p engine.Progress) {
		if !limit.allow(time.Now()) {
			return
		}
		elapsed := time.Since(started)
		fyne.Do(func() {
			r.busy.bar.SetValue(float64(core.Percent(p.BytesDone, p.BytesTotal)))
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
		planned, planErr := engine.PlanContext(ctx, targets, opt)
		if planErr != nil {
			// A refusal from planning leaves nothing started and nothing on the
			// disk, so it goes back as a refusal rather than as the end of a
			// run - runFinished would talk about files that never existed.
			r.holdBeforeFinishing()
			fyne.Do(func() {
				r.busy.set(false, busyFace{})
				r.refuse(planErr)
			})
			close(done)
			return
		}
		// Nothing is said from here, and that is deliberate rather than an
		// omission. A line saying how many files are being written would be a
		// widget touched from the worker in the middle of the run, and under
		// the test driver fyne.Do runs on the CALLING goroutine - so every
		// guard that looks at the screen while a run is going would be reading
		// a widget this goroutine is writing. The race detector found two of
		// them on CI. The progress callback already takes the line over within
		// a tenth of a second, and it is throttled, which is why it was never
		// the same problem.
		res, runErr := engine.Run(ctx, planned, opt)
		// The manifest is written here rather than after crossing back, because
		// it is disk work and the interface thread is the one thing that must
		// not wait on a disk.
		saved, saveErr := saveRecord(res, opt)
		// The room left on the disk is the room left AFTER the files, which
		// is not the number a preview measured before them.
		room := roomOn(opt.OutDir)
		// Do rather than DoAndWait. The interface thread may already be inside
		// stop, waiting on the channel closed below, and a worker waiting for
		// that thread to run something would be both of them waiting.
		r.holdBeforeFinishing()
		fyne.Do(func() { r.runFinished(res, runErr, saveErr, room, saved) })
		close(done)
	}()
}

// runFinished is the end of a run, back on the interface thread.
//
// Note what it does not do: clear stop. That is deliberate and the reason is at
// the declaration of the field.
//
// saved is where the record went, with nothing in it when no record was
// written - which is a refused run, a preview, and a run whose manifest could
// not be saved. The screen says nothing about a manifest in any of those.
func (r *runner) runFinished(res *engine.Result, runErr, saveErr error, room diskRoom, saved engine.Record) {
	r.busy.set(false, busyFace{})
	if room.known && r.offer.wroteInto != "" {
		r.line.measured(r.offer.wroteInto, room.free)
	}

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
	//
	// The warning about a record too big to read back comes SECOND, ahead of
	// the per file notes, and that order is the same lesson the command line
	// learned on 2026-09-06: it is the one line standing between somebody and a
	// directory nothing in this toolset can ever clean up, and it was being
	// buried under notes about a label that did not fit.
	//
	// The record is named on the same line as the outcome rather than on one of
	// its own, because it is part of the same fact: what this run produced. The
	// command line has printed it since there was a manifest, and the window
	// said only how many files - so the one thing this tool makes that others
	// do not was, from a window, something you found in the folder afterwards.
	outcome := text.SaidWithManifest(outcomeText(res, runErr), manifestNameOf(saved.Manifest))
	said := append([]string{outcome}, manifestReachNote(res)...)
	// Instructions that were due and are not there are said, not failed: the
	// manifest was saved and holds the same facts. Silence would leave a button
	// missing with no reason given.
	if saved.Missed != nil {
		// Escaped as the command line escapes it, the path and the system's
		// sentence both - the second carries the path again (review on #140).
		said = append(said, text.InstructionsNotSaved(core.Shown(saved.Missed.Path), core.ShownText(saved.Missed.Err.Error())))
	}
	r.say(append(said, notesOf(res)...)...)
	r.toneOfOutcome(res, runErr)
	r.offer.theFolder(res)
	r.offer.manifest.show(saved.Manifest)
	r.offer.instructions.show(saved.Instructions)
}

// manifestNameOf is the file's own name, for a sentence that stands beside a
// button opening the folder it is in. Nothing where no record was written.
func manifestNameOf(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
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

// HoldBeforeFinishing sets a hold the worker passes through on its way to
// reporting, so that a guard can read the screen WHILE a run is going.
//
// A seam for the guards, named as one, and the shipped program never sets it -
// the same kind of thing as Settled below and as Options.AvailableBytes in the
// engine.
//
// It exists because of O144, and the shape of that is worth stating because it
// is not a timing problem and cannot be fixed by waiting longer. A guard that
// reads a widget between pressing Generate and joining the worker is reading
// something the worker writes when the run ends, and NOTHING orders the two -
// so the race detector is right whatever the wall clock does, and three
// assertions about the middle of a run had to go or be skipped on 2026-08-26.
// In a real window there is no race at all: shortcuts and the end of a run are
// both delivered on the interface thread. Under the test driver fyne.Do runs on
// the CALLING goroutine, which is the worker (O124).
//
// The hold supplies the ordering that was missing, rather than papering over
// its absence. The worker calls it before it touches anything, so a guard can
// hand over a function that signals and then waits: the signal orders the
// worker's earlier writes before the guard's read, and the guard's release
// orders that read before the worker's later writes. There is then no
// concurrent access left to report, which is why this restores the assertions
// instead of hiding them.
func (r *runner) HoldBeforeFinishing(fn func()) { r.hold = fn }

// holdBeforeFinishing runs the hold, if a guard supplied one. Called on the
// worker, before anything on the screen is touched.
func (r *runner) holdBeforeFinishing() {
	if r.hold != nil {
		r.hold()
	}
}

// Settled waits for work in flight to finish, without stopping it.
//
// A seam for the guards, and named as one rather than dressed up - the same
// kind of thing as Options.AvailableBytes and Options.MaxPlanBytes in the
// engine, which exist so a test can describe a small disk or a small ceiling
// without owning one.
//
// It is here because of what changed on 2026-08-26. Until then a preview could
// not be cancelled - preflight took no context - so Stop only waited, and the
// guards used "close the window" as their way of waiting for an answer. Now
// closing really does cancel, which is the point of the change, and a guard
// that closed the window to read the preview would be cancelling the preview.
//
// It waits on the same channel stop does and carries the same limit: the worker
// is finished and its last fyne.Do may still be queued. Under the test driver
// that distinction disappears, because there fyne.Do runs on the calling
// goroutine, so the callback has already run by the time the channel closes -
// which is why a guard may read the screen the moment this returns, and why the
// limit is a production one rather than a test one.
func (r *runner) Settled() {
	if r.settled != nil {
		<-r.settled
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
