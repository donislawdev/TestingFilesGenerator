package window

import (
	"context"
	"fmt"
	"path/filepath"
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

	previewBtn  *widget.Button
	generateBtn *widget.Button
	cancelBtn   *widget.Button

	bar     *widget.ProgressBar
	status  *widget.Label
	problem *parts.ErrorArea

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

// say puts a sentence on the status line, under whatever settling had to say.
func (r *runner) say(lines ...string) {
	r.status.SetText(strings.Join(append(append([]string{}, r.notes...), lines...), "\n"))
}

func newRunner() *runner {
	r := &runner{}
	r.bar = widget.NewProgressBar()
	// Counted as a percentage rather than as bytes, so the arithmetic that keeps
	// a very large run inside the range of its own type is the one the command
	// line already uses.
	r.bar.Max = 100
	r.bar.Hide()

	r.status = widget.NewLabel("")
	r.status.Wrapping = fyne.TextWrapWord
	r.problem = parts.NewErrorArea()

	r.previewBtn = widget.NewButton(text.ButtonPreview, r.onPreview)
	r.generateBtn = widget.NewButton(text.ButtonGenerate, r.onGenerate)
	r.generateBtn.Importance = widget.HighImportance
	r.cancelBtn = widget.NewButton(text.ButtonCancel, r.onCancel)
	r.cancelBtn.Disable()
	return r
}

// actions puts Preview before Generate, and that order is G6.
//
// It is the one thing a window does better than a command line rather than
// merely as well: --dry-run has to be known about and remembered, and this is
// on the way to the button beside it. With presets running to several gigabytes
// and disks that are not always emptier than that, it is not decoration.
func (r *runner) actions(extra ...fyne.CanvasObject) fyne.CanvasObject {
	all := []fyne.CanvasObject{r.previewBtn, r.generateBtn, r.cancelBtn}
	return container.NewHBox(append(all, extra...)...)
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
	r.problem.Clear()
	targets, opt, err := r.settle()
	if err != nil {
		r.problem.Say(err.Error())
		return
	}
	opt.DryRun = true

	planned, err := engine.Plan(targets, opt)
	if err != nil {
		r.problem.Say(err.Error())
		return
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		r.problem.Say(err.Error())
		return
	}
	r.say(previewText(planned, opt.OutDir))
}

// previewText is the cost, before anything exists. G6: how many files, how many
// bytes, and how much room there is for them.
func previewText(planned []engine.PlannedFile, outDir string) string {
	total := engine.TotalBytes(planned)
	line := text.PreviewCost(len(planned), core.HumanBytes(total))

	// A disk we cannot measure is not the same as a disk that is full, so a
	// failure to read it says nothing rather than inventing a number.
	if free, err := core.AvailableBytes(outDir); err == nil {
		line += text.PreviewFreeSpace(outDir, core.HumanBytes(free))
	}
	return line
}

// onGenerate plans on the interface thread and writes off it.
//
// Planning is fast enough to do here - measured at 15.7 ms for ten thousand
// files - and doing it here is what lets a refusal appear with nothing started
// and no buttons to put back.
func (r *runner) onGenerate() {
	r.problem.Clear()
	targets, opt, err := r.settle()
	if err != nil {
		r.problem.Say(err.Error())
		return
	}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		r.problem.Say(err.Error())
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
		r.problem.Say(runErr.Error())
	case saveErr != nil:
		r.problem.Say(saveErr.Error())
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
	if running {
		r.previewBtn.Disable()
		r.generateBtn.Disable()
		r.cancelBtn.Enable()
		r.bar.Show()
		return
	}
	r.previewBtn.Enable()
	r.generateBtn.Enable()
	r.cancelBtn.Disable()
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
func wholeNumber(what, text string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"%s is %q, which is not a whole number. Write the digits out, such as 1 or 500", what, text)
	}
	return n, nil
}
