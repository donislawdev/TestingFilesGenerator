package window

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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

// settle turns what is on the screen into what the engine takes.
//
// It parses and it does not judge. Every rule about whether the numbers make
// sense - the minimum of the format, the ceiling on files, a name that is a
// path, two files heading for one name - belongs to the engine and is asked of
// it. That is G1, and it is why the window cannot come to accept something the
// command line refuses.
func (g *Generate) settle() ([]engine.Target, engine.Options, error) {
	var none engine.Options

	bytesWanted, err := core.ParseSize(g.size.Text)
	if err != nil {
		return nil, none, err
	}
	count, err := wholeNumber("how many", g.count.Text)
	if err != nil {
		return nil, none, err
	}
	seed, err := wholeNumber("seed", g.seed.Text)
	if err != nil {
		return nil, none, err
	}

	// Asked before the list is built, because building it is the failure. The
	// same ceiling and the same sentence the command line uses, from core, so
	// there is no second opinion about how many files is too many. A count past
	// it used to reach make and panic with a stack trace.
	if count > core.MaxFilesPerRun {
		return nil, none, fmt.Errorf("this run asks for %d files - %s", count, core.ErrTooManyFiles)
	}

	return []engine.Target{{
			ID:         g.id.Text,
			Format:     g.formatPick.Selected,
			Sizes:      engine.Uniform(int(count), bytesWanted),
			NameTmpl:   g.name.Text,
			Label:      g.label.Checked,
			Properties: g.properties(),
		}}, engine.Options{
			OutDir:       g.outDir.Text,
			Seed:         seed,
			Command:      "tfg-gui",
			ManifestName: engine.DefaultManifestName,
		}, nil
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

// onPreview says what the run would cost and writes nothing.
//
// It goes all the way through engine.Run with DryRun set rather than stopping
// after the plan, and that is the difference between a preview and a guess: the
// checks for free space and for a name already taken live in the same preflight
// the real run goes through, so the answer here is the answer there. Stopping
// early used to promise success to a run that refused to start on the next line.
func (g *Generate) onPreview() {
	g.problem.Clear()
	targets, opt, err := g.settle()
	if err != nil {
		g.problem.Say(err.Error())
		return
	}
	opt.DryRun = true

	planned, err := engine.Plan(targets, opt)
	if err != nil {
		g.problem.Say(err.Error())
		return
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		g.problem.Say(err.Error())
		return
	}
	g.status.SetText(previewText(planned, opt.OutDir))
}

// previewText is the cost, before anything exists. G6: how many files, how many
// bytes, and how much room there is for them.
func previewText(planned []engine.PlannedFile, outDir string) string {
	total := engine.TotalBytes(planned)
	text := fmt.Sprintf("%d file(s), %s in total. Nothing has been written.",
		len(planned), core.HumanBytes(total))

	// A disk we cannot measure is not the same as a disk that is full, so a
	// failure to read it says nothing rather than inventing a number.
	if free, err := core.AvailableBytes(outDir); err == nil {
		text += fmt.Sprintf(" %s has %s free.", outDir, core.HumanBytes(free))
	}
	return text
}

// onGenerate plans on the interface thread and writes off it.
//
// Planning is fast enough to do here - measured at 15.7 ms for ten thousand
// files - and doing it here is what lets a refusal appear with nothing started
// and no buttons to put back.
func (g *Generate) onGenerate() {
	g.problem.Clear()
	targets, opt, err := g.settle()
	if err != nil {
		g.problem.Say(err.Error())
		return
	}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		g.problem.Say(err.Error())
		return
	}
	g.startRun(planned, opt)
}

func (g *Generate) startRun(planned []engine.PlannedFile, opt engine.Options) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	started := time.Now()
	limit := &throttle{}

	g.setRunning(true)
	g.bar.SetValue(0)
	g.status.SetText(fmt.Sprintf("writing %d file(s)...", len(planned)))

	// Called by the engine from the goroutine below, once per write inside a
	// file. Thinned out here, before anything crosses over, so the interface is
	// asked to redraw ten times a second rather than thousands.
	opt.OnProgress = func(p engine.Progress) {
		if !limit.allow(time.Now()) {
			return
		}
		elapsed := time.Since(started)
		fyne.Do(func() {
			g.bar.SetValue(float64(core.Percent(p.BytesDone, p.BytesTotal)))
			g.status.SetText(progressText(p, elapsed))
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
		fyne.Do(func() { g.runFinished(res, runErr, saveErr) })
		close(done)
	}()

	// Cancelling and waiting, in that order, is the whole of G7. Closing the
	// window is not a signal, so nothing else brings the run to a stop - and
	// ending the process without waiting would leave it somewhere inside a file
	// with no manifest, which is the one thing the output directory is promised
	// never to hold.
	g.stop = func() {
		cancel()
		<-done
	}
}

// progressText is the line under the bar. Bytes rather than files, because one
// large file is a run where the file count says nothing for minutes.
func progressText(p engine.Progress, elapsed time.Duration) string {
	text := fmt.Sprintf("%d/%d files  %s of %s  %d%%",
		p.FilesDone, p.FilesTotal,
		core.HumanBytes(p.BytesDone), core.HumanBytes(p.BytesTotal),
		core.Percent(p.BytesDone, p.BytesTotal))

	// The estimate stays quiet until it has enough to go on. A number that
	// swings wildly for the first second is worse than no number.
	if elapsed < time.Second || p.BytesDone <= 0 || p.BytesDone >= p.BytesTotal {
		return text
	}
	left := time.Duration(float64(elapsed) *
		float64(p.BytesTotal-p.BytesDone) / float64(p.BytesDone))
	return text + "  " + core.Roughly(left) + " left"
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
		return fmt.Errorf("the files were written and the manifest could not be saved to %s: %w", path, err)
	}
	return nil
}

// runFinished puts the screen back and says what happened. On the interface
// thread, because everything it touches is a widget.
// stop is deliberately left in place rather than cleared here, and that is a
// threading decision rather than an oversight. Clearing it would be a write
// from the worker under the toolkit's test driver, where fyne.Do runs on the
// calling goroutine - so -race would report a race that production does not
// have, and the honest way out is a handle that is safe to hold on to. Calling
// it after the run has ended cancels a finished context and receives from a
// closed channel, both of which return at once.
func (g *Generate) runFinished(res *engine.Result, runErr, saveErr error) {
	g.setRunning(false)

	switch {
	case runErr != nil:
		g.problem.Say(runErr.Error())
	case saveErr != nil:
		g.problem.Say(saveErr.Error())
	}

	// Silence is banned. A file that was not produced has to be visible here
	// and not only in the manifest - "the manifest says which ones" is an
	// answer in a terminal and an instruction to open a file with ten thousand
	// entries in a window.
	g.status.SetText(outcomeText(res, runErr))
	for _, note := range notesOf(res) {
		g.status.SetText(g.status.Text + "\n" + note)
	}
}

func outcomeText(res *engine.Result, runErr error) string {
	if res == nil || res.Manifest == nil {
		return "nothing was produced."
	}
	written := len(res.Manifest.Files) - res.Failures
	switch {
	case runErr != nil:
		return fmt.Sprintf("stopped after %d file(s). The manifest describes exactly those.", written)
	case res.Failures > 0:
		return fmt.Sprintf("%d file(s) written, %d could not be produced.", written, res.Failures)
	}
	return fmt.Sprintf("%d file(s) written.", written)
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
func (g *Generate) setRunning(running bool) {
	if running {
		g.previewBtn.Disable()
		g.generateBtn.Disable()
		g.cancelBtn.Enable()
		g.bar.Show()
		return
	}
	g.previewBtn.Enable()
	g.generateBtn.Enable()
	g.cancelBtn.Disable()
	g.bar.Hide()
}

func (g *Generate) onCancel() {
	if g.stop != nil {
		g.stop()
	}
}

// onClose is what closing the window does while a run is going, G7.
//
// The run is stopped and waited for, and only then does the window go. What is
// already finished stays on disk and the manifest describes exactly that, with
// run.complete false - the same ending Ctrl+C gives the command line, reached
// through the one gesture that is not a signal.
func (g *Generate) onClose() {
	if g.stop != nil {
		g.stop()
	}
	g.host.Close()
}
