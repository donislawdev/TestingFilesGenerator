package guard

import (
	"bytes"
	"context"
	"runtime"
	"slices"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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
func TestAFilmCodedBySeveralGoroutinesHasTheBytesOfOne(t *testing.T) {
	targets := []engine.Target{
		filmTarget(2<<20, map[string]string{"width": "64", "height": "48", "duration": "40s"}),
		filmTarget(1<<20, map[string]string{"duration": "30s", "change_interval": "500ms"}),
	}
	for _, target := range targets {
		alone, helped := filmUnder(t, 1, target), filmUnder(t, 8, target)
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
	target := filmTarget(8<<20, map[string]string{"width": "160", "height": "90", "duration": "10m"})
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opt := engine.Options{OutDir: dir, Seed: goldenSeed, Command: "test"}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatal(err)
	}
	if changes, _ := planned[0].Plan.Properties["change_count"].(int64); changes < 100 {
		t.Fatalf("the film has %d pictures, too few to be stopped in the middle of them", changes)
	}
	_, codedBefore := video.Helpers()
	opt.OnProgress = func(p engine.Progress) {
		// Some forty pictures in, of six hundred.
		if p.BytesDone > 100_000 {
			cancel()
		}
	}
	res, runErr := engine.Run(ctx, planned, opt)
	running, codedAfter := video.Helpers()
	if runErr == nil && (res == nil || len(res.Manifest.Files) > 0) {
		t.Fatalf("the run finished the film, so it was never stopped half way")
	}
	if codedAfter == codedBefore {
		t.Fatalf("no helper coded a picture before the stop, so there was nothing that could have been left running")
	}
	if running != 0 {
		t.Errorf("%d helpers are still running after the stopped run returned", running)
	}
}
