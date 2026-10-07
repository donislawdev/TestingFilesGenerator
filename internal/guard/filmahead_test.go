package guard

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"slices"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// The pictures of one film are coded by several goroutines at once and
// painted a strip at a time (internal/format/video, ahead.go and picture.go,
// docs/WEBM-WYDAJNOSC-2026-10-06.md). Neither may change a byte of any film,
// and neither may leave anything running once the film is done with.

// A film paints only the rows that can change - the clock's band and the
// square's, widened to whole pairs of rows - onto planes made once from the
// gradient. It has to come out as the picture painted whole, the way every
// film was painted before, in every plane and every sample.
//
// The sizes are the ones where strips go wrong: one pixel, odd sides that
// leave a chroma sample half outside, a picture short enough that the square
// meets the clock's band, the smallest that still shows the clock, and the
// ladder's top rung. Each with the label and without, and each with the clock
// in whole seconds and with milliseconds, because those are bands of two
// lengths.
func TestAFilmPaintsOnlyWhatChangesAndGetsTheWholePicture(t *testing.T) {
	sizes := [][2]int{{1, 1}, {2, 2}, {3, 3}, {16, 9}, {17, 11}, {48, 32}, {52, 20}, {64, 48}, {97, 41}, {160, 90}, {161, 91}, {640, 360}}
	timelines := map[string]video.Timeline{}
	for name, ms := range map[string][2]int64{"seconds": {3_600_000, 1_000}, "milliseconds": {60_000, 100}} {
		tl, err := video.NewTimeline("webm", ms[0], 60_000, ms[1], 30)
		if err != nil {
			t.Fatal(err)
		}
		timelines[name] = tl
	}
	labels := map[string]string{"labelled": core.Label("webm", 1<<20, 7), "unlabelled": ""}

	compared, clocked, moved := 0, 0, 0
	for _, size := range sizes {
		w, h := size[0], size[1]
		for tName, tl := range timelines {
			for lName, label := range labels {
				changes := []int64{0, 1, 2, 9, 10, 11, tl.Changes() / 2, tl.Changes() - 1}
				var first video.Planes
				for i, c := range changes {
					got := video.Picture(w, h, 7, label, tl, c)
					want := video.WholePicture(w, h, 7, label, tl, c)
					for plane, pair := range map[string][2][]uint8{"Y": {got.Y, want.Y}, "U": {got.U, want.U}, "V": {got.V, want.V}} {
						if !bytes.Equal(pair[0], pair[1]) {
							t.Errorf("%dx%d %s %s, picture %d: the %s plane painted a strip at a time differs from the picture painted whole", w, h, tName, lName, c, plane)
						}
					}
					compared++
					if i == 0 {
						first = got
						first.Y = slices.Clone(got.Y)
					} else if !bytes.Equal(first.Y, got.Y) {
						moved++
					}
				}
				if video.ClockShown(w, h, label, tl) {
					clocked++
				}
			}
		}
	}
	// Asserted, not assumed: a strip painter that drew nothing would agree with
	// a whole painter that drew nothing, so the pictures have to have moved,
	// and the clock has to have been in some of them.
	if compared < 300 || moved < 100 || clocked < 10 {
		t.Fatalf("%d pictures compared, %d of them different from their film's first, %d films with a clock - the guard did not see pictures that change",
			compared, moved, clocked)
	}
}

// A film coded by several goroutines has the bytes of the same film coded by
// one, and the several really did code it.
//
// The second half is the one that keeps the first honest. With one thread the
// process has no helper to give, so the run under one thread is the reference
// and has to have used none, and the run under eight has to have used some -
// otherwise both runs are the same goroutine and the comparison is of a film
// with itself. One film is sized by hand, so planning codes its sample through
// the helpers too, and one is chosen to fit.
//
// An MP4 codes its film twice, the second time taking what the first kept
// (video.Pictures.Again), and both passes take helpers, so it is asked too.
func TestAFilmCodedBySeveralGoroutinesHasTheBytesOfOne(t *testing.T) {
	var targets []engine.Target
	for _, id := range filmFormats {
		targets = append(targets,
			filmTarget(id, 2<<20, map[string]string{"width": "64", "height": "48", "duration": "40s"}),
			filmTarget(id, 1<<20, map[string]string{"duration": "30s", "change_interval": "500ms"}))
	}
	tiled := 0
	for _, target := range targets {
		alone, helped := filmUnder(t, 1, target), filmUnder(t, 8, target)
		if filmTileCount(t, target.Format, alone.bytes) > 1 {
			tiled++
		}
		if alone.coded != 0 {
			t.Errorf("%v: under one thread helpers coded %d pictures, so there is no film of one goroutine to compare with", target.Properties, alone.coded)
		}
		if helped.coded == 0 {
			t.Errorf("%v: under eight threads no helper coded anything, so the film of several goroutines was never made", target.Properties)
		}
		if !bytes.Equal(alone.bytes, helped.bytes) {
			t.Errorf("%v: the film coded with helpers differs from the film coded by one goroutine (%d B and %d B)", target.Properties, len(helped.bytes), len(alone.bytes))
		}
	}
	// Tiles are what helpers code, and a tile one helper coded is taken by
	// pictures others are waiting for - the film of tiles is the one where
	// the order of who coded what could show.
	if tiled < len(filmFormats) {
		t.Fatalf("%d films were cut into tiles, so helpers coding tiles were not compared in each container", tiled)
	}
}

type filmRun struct {
	bytes []byte
	coded int64
}

func filmUnder(t *testing.T, threads int, target engine.Target) filmRun {
	t.Helper()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(threads))
	_, before := video.Helpers()
	b, _ := filmOne(t, target)
	_, after := video.Helpers()
	return filmRun{bytes: b, coded: after - before}
}

// A film's progress moves with its pictures, not with its padding.
//
// The bytes of a film are mostly padding written in its last seconds, so a bar
// counting bytes stood at five percent while the pictures were coded and
// promised the owner three hours for a film that took ten minutes
// (docs/WEBM-WYDAJNOSC-2026-10-06.md). Here the pictures are about a
// twenty-fifth of the bytes and two thirds of the work: at the moment they are
// all written the work has to be far ahead of the bytes. And a run of files
// whose cost is their bytes has to report the same numbers as before - work
// equal to bytes at every report.
//
// The second film is cut into tiles, which are coded the first time their key
// comes up, so most of its work is the first picture and the square's ten
// places - and the bar has to move with that too.
func TestAFilmsProgressMovesWithItsPicturesNotItsPadding(t *testing.T) {
	for _, target := range []engine.Target{
		filmTarget("webm", 32<<20, map[string]string{"width": "160", "height": "90", "duration": "5m"}),
		filmTarget("webm", 32<<20, map[string]string{"width": "640", "height": "360", "duration": "5m"}),
		// An MP4 codes its pictures before it writes a byte, so its bar is all
		// work first - which a bar counting bytes would show standing still.
		filmTarget("mp4", 32<<20, map[string]string{"width": "640", "height": "360", "duration": "5m"}),
	} {
		props := target.Format + " " + fmt.Sprint(target.Properties)
		reports, planned := progressOf(t, target)
		if changes, _ := planned.Properties["change_count"].(int64); changes < 100 || planned.Work <= 0 {
			t.Fatalf("%v: the film has %d pictures and declares %d of work, so there were no pictures for the bar to move with", props, changes, planned.Work)
		}
		ahead := 0.0
		for _, r := range reports {
			ahead = max(ahead, float64(r.WorkDone)/float64(r.WorkTotal)-float64(r.BytesDone)/float64(r.BytesTotal))
		}
		if ahead < 0.4 {
			t.Errorf("%v: the work was never more than %.0f%% ahead of the bytes, so the bar counts the padding as the film", props, 100*ahead)
		}
		if last := reports[len(reports)-1]; last.WorkDone != last.WorkTotal || last.BytesDone != last.BytesTotal {
			t.Errorf("%v: the run ended at %d of %d work and %d of %d bytes", props, last.WorkDone, last.WorkTotal, last.BytesDone, last.BytesTotal)
		}
	}

	plain, _ := progressOf(t, engine.Target{ID: "plain", Format: "txt", Sizes: engine.Uniform(1, 1<<20)})
	for _, r := range plain {
		if r.WorkDone != r.BytesDone || r.WorkTotal != r.BytesTotal {
			t.Fatalf("a text file reported %d of %d work against %d of %d bytes, and its cost is its bytes", r.WorkDone, r.WorkTotal, r.BytesDone, r.BytesTotal)
		}
	}
}

// progressOf runs one target with every progress report kept, and its plan.
func progressOf(t *testing.T, target engine.Target) ([]engine.Progress, format.Plan) {
	t.Helper()
	opt := engine.Options{OutDir: t.TempDir(), Seed: goldenSeed, Command: "test"}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatal(err)
	}
	var reports []engine.Progress
	opt.OnProgress = func(p engine.Progress) { reports = append(reports, p) }
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatal(err)
	}
	if len(reports) < 2 {
		t.Fatalf("%s: %d progress reports, too few to say how the bar moved", target.Format, len(reports))
	}
	return reports, planned[0].Plan
}

// A film stopped half way leaves no helper running when the run returns.
//
// Helpers are goroutines, and one left behind holds a picture's memory and a
// place the next film cannot take - and the window, which waits for the run
// before it closes (G7), would be waiting for something that no longer waits
// for anybody. The run is stopped once pictures are being written, which is
// asserted: stopped at the file's first bytes it would end before any helper
// was taken and pass without asking anything.
func TestAFilmStoppedHalfWayLeavesNoHelperRunning(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))
	writing := func(p engine.Progress) bool { return p.BytesDone > 100_000 } // some forty pictures in, of six hundred
	stops := []struct {
		format, where string
		now           func(engine.Progress) bool
	}{
		{"webm", "once its pictures are being written", writing},
		// An MP4 has two passes, each with helpers of its own: stopped while
		// the first codes and nothing of the file is written yet, and while
		// the second writes.
		{"mp4", "while its first pass codes", func(p engine.Progress) bool { return p.BytesDone == 0 && p.WorkDone > p.WorkTotal/4 }},
		{"mp4", "once its pictures are being written", writing},
	}
	for _, stop := range stops {
		target := filmTarget(stop.format, 8<<20, map[string]string{"width": "160", "height": "90", "duration": "10m"})
		stopFilmHalfWay(t, stop.format+" stopped "+stop.where, target, stop.now)
	}
}

// stopFilmHalfWay runs a film, stops it at the first report now says yes to,
// and asks that the run was stopped, that helpers had coded before it was,
// and that none is left running.
func stopFilmHalfWay(t *testing.T, name string, target engine.Target, now func(engine.Progress) bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opt := engine.Options{OutDir: t.TempDir(), Seed: goldenSeed, Command: "test"}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatal(err)
	}
	if changes, _ := planned[0].Plan.Properties["change_count"].(int64); changes < 100 {
		t.Fatalf("%s: the film has %d pictures, too few to be stopped in the middle of them", name, changes)
	}
	_, codedBefore := video.Helpers()
	opt.OnProgress = func(p engine.Progress) {
		if now(p) {
			cancel()
		}
	}
	res, runErr := engine.Run(ctx, planned, opt)
	running, codedAfter := video.Helpers()
	if runErr == nil && (res == nil || len(res.Manifest.Files) > 0) {
		t.Fatalf("%s: the run finished the film, so it was never stopped half way", name)
	}
	if codedAfter == codedBefore {
		t.Fatalf("%s: no helper coded a picture before the stop, so there was nothing that could have been left running", name)
	}
	if running != 0 {
		t.Errorf("%s: %d helpers are still running after the stopped run returned", name, running)
	}
}
