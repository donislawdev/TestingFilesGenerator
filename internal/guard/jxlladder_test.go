package guard

import (
	"context"
	"runtime"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/jxl"
)

// JPEG XL is the second format here whose file size is decided by a compressor
// rather than by arithmetic, so like AVIF it cannot work its picture size out
// and chooses from a ladder instead. Each rung carries a CEILING - the largest
// that rung has ever coded to - and planning takes the largest rung whose
// ceiling fits the file.
//
// Two things have to hold, and they pull in opposite directions.
//
// A ceiling set too LOW lets planning pick a picture that does not leave room
// for the padding, and then the file cannot come out the size it promised.
// A ceiling set too HIGH costs a smaller picture than the file had room for -
// which nothing else notices, because the size is still exact and the run still
// succeeds.
//
// The sizes below span the whole ladder and reach well past it, so a rung that
// stopped being reachable at any size shows up as a picture that never grows.
func TestJxlDrawsAPictureThatGrowsWithTheFileAndAlwaysFits(t *testing.T) {
	d, err := format.Get("jxl")
	if err != nil {
		t.Fatal(err)
	}

	sizes := []int64{
		int64(jxl.MinimumBytes), int64(jxl.MinimumBytes) + 1, 200, 300, 400, 500, 700, 1000,
		1500, 2000, 2500, 3000, 4000, 6000, 8000, 12000, 20000, 65536,
		100_000, 500_000, 1_500_000,
	}

	lastArea := 0
	biggest := 0
	for _, want := range sizes {
		p, err := d.Generator.Plan(format.Request{Bytes: want, Seed: 7741, Label: true})
		if err != nil {
			t.Fatalf("planning %d B: %v", want, err)
		}
		w := p.Properties["width"].(int)
		h := p.Properties["height"].(int)

		// The picture has to fit, which is the half a wrong ceiling breaks.
		counted := &countingSink{}
		if err := d.Generator.Write(context.Background(), counted, p); err != nil {
			t.Errorf("%d B: writing a %dx%d picture failed: %v\n"+
				"That is what a ceiling set too low looks like - planning chose a picture the file has no room for.",
				want, w, h, err)
			continue
		}
		if counted.n != want {
			t.Errorf("%d B was asked for and %d B came out, with a %dx%d picture", want, counted.n, w, h)
		}

		// And more bytes must never buy a smaller picture.
		if w*h < lastArea {
			t.Errorf("%d B gets a %dx%d picture, which is smaller than the one the size below it got - asking for more took picture away",
				want, w, h)
		}
		lastArea = w * h
		if w*h > biggest {
			biggest = w * h
		}
	}

	// A ladder nothing ever climbs would satisfy everything above.
	if biggest <= 1 {
		t.Fatalf("the largest picture over the whole sweep was %d pixels, so no rung of the ladder is reachable and this guard measured nothing", biggest)
	}
	t.Logf("largest picture reached over the sweep: %d pixels", biggest)
}

// Every rung's declared ceiling still covers what that rung produces.
//
// The guard above walks the ladder the way a run does, so it only ever sees
// the rung each size happens to select, at one seed. This one asks each rung
// directly, across seeds, which is the question the ceiling is an answer to.
//
// A handful of seeds rather than all 256: the thorough sweep is
// tools/probes/jxlladder, which takes minutes. This is the version that runs
// on every push and would still catch a codec raised underneath us, because a
// new encoder moves every rung at once rather than one seed of one rung.
func TestEveryJxlRungStillFitsInsideItsDeclaredCeiling(t *testing.T) {
	rungs := jxl.Rungs()
	if len(rungs) == 0 {
		t.Fatal("the ladder is empty, so this guard measured nothing")
	}

	// The label carries the byte count, so its length moves with the size
	// asked for, and a longer label is more ink in the picture.
	requested := []int64{int64(jxl.MinimumBytes), 1 << 14, 1 << 30}

	for _, r := range rungs {
		w, h, ceiling := int(r[0]), int(r[1]), r[2]
		worst := int64(0)
		var worstSeed uint64
		for seed := uint64(0); seed < 8; seed++ {
			for _, want := range requested {
				for _, withLabel := range []bool{true, false} {
					label := ""
					if withLabel {
						label = core.Label("jxl", want, seed)
					}
					n, err := jxl.FileSizeFor(w, h, jxl.DefaultQuality, seed, label)
					if err != nil {
						t.Fatalf("%dx%d seed %d: %v", w, h, seed, err)
					}
					if int64(n) > worst {
						worst, worstSeed = int64(n), seed
					}
				}
			}
		}
		if worst > ceiling {
			t.Errorf("the %dx%d rung declares a ceiling of %d B and produced %d B at seed %d.\n"+
				"A ceiling below what its rung produces lets planning pick a picture the file cannot hold.\n"+
				"What to do: run tools/probes/jxlladder and record what it measures.",
				w, h, ceiling, worst, worstSeed)
		}
	}
}

// Planning must not code the picture.
//
// AVIF paid for this once and the price was measured on 2026-08-29: a preview
// of a thousand files took 51 seconds where BMP took a fifth of one. This
// encoder is several times slower than that one, so the same defect here would
// cost more, not less.
//
// Asked in ALLOCATIONS rather than on the clock, because allocation counts are
// deterministic and a time based gate would be flaky on a busy runner - the
// same reasoning the memory guards use. One encode allocates megabytes, so
// fifty plans coming in under a single write is not a close call.
func TestJxlPlanningDoesNotCodeThePicture(t *testing.T) {
	d, err := format.Get("jxl")
	if err != nil {
		t.Fatal(err)
	}

	const plans = 50
	const size = 300 << 10

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var last format.Plan
	for i := 0; i < plans; i++ {
		p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: uint64(i), Label: true})
		if err != nil {
			t.Fatalf("planning: %v", err)
		}
		last = p
	}
	var afterPlan runtime.MemStats
	runtime.ReadMemStats(&afterPlan)
	planning := int64(afterPlan.TotalAlloc - before.TotalAlloc)

	runtime.GC()
	var beforeWrite runtime.MemStats
	runtime.ReadMemStats(&beforeWrite)
	if err := d.Generator.Write(context.Background(), &countingSink{}, last); err != nil {
		t.Fatalf("writing: %v", err)
	}
	var afterWrite runtime.MemStats
	runtime.ReadMemStats(&afterWrite)
	writing := int64(afterWrite.TotalAlloc - beforeWrite.TotalAlloc)

	t.Logf("%d plans allocated %d B, one write allocated %d B", plans, planning, writing)

	if planning >= writing {
		t.Errorf("%d plans allocated %d B and one write allocated %d B.\n"+
			"Planning is coding the picture again, which is what makes a preview of a large run cost what the run costs.\n"+
			"What to do: planning picks a rung of the ladder from its ceiling, and the encode belongs in Write.",
			plans, planning, writing)
	}
}

// A quality the ceilings were not measured at still produces a file.
//
// Carried over from AVIF, where it was a real defect rather than a worry: the
// ceiling table speaks for the default quality and nothing else, and a request
// for another one without a named picture size took a rung off that table
// whose picture is several times larger than the ceiling claims. Measured on
// 2026-08-29 before that repair, `--size 20kb --set quality=100` produced no
// file at all.
//
// The repair is that a named quality walks the ladder coding each rung, which
// is the slow road, taken only by runs that asked for it.
func TestJxlStillFitsWhenTheQualityIsNotTheOneTheCeilingsWereMeasuredAt(t *testing.T) {
	d, err := format.Get("jxl")
	if err != nil {
		t.Fatal(err)
	}

	for _, quality := range []string{"1", "10", "70", "90", "100"} {
		for _, want := range []int64{2000, 20000, 200000} {
			p, err := d.Generator.Plan(format.Request{
				Bytes: want, Seed: 7741, Label: true,
				Properties: map[string]string{"quality": quality},
			})
			if err != nil {
				t.Errorf("quality %s at %d B was refused: %v", quality, want, err)
				continue
			}
			counted := &countingSink{}
			if err := d.Generator.Write(context.Background(), counted, p); err != nil {
				t.Errorf("quality %s at %d B: writing a %vx%v picture failed: %v\n"+
					"The ceiling table was measured at the default quality and does not speak for this one.",
					quality, want, p.Properties["width"], p.Properties["height"], err)
				continue
			}
			if counted.n != want {
				t.Errorf("quality %s: %d B asked for, %d B came out", quality, want, counted.n)
			}
		}
	}
}
