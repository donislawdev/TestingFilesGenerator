package guard

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/avif"
)

// AVIF is the one format here whose file size is decided by a compressor rather
// than by arithmetic, so it cannot work its picture size out and has to choose
// from a ladder of sizes instead. Each step of that ladder carries a CEILING -
// the largest that step has ever coded to - and planning takes the largest step
// whose ceiling fits the file.
//
// Two things have to hold, and they pull in opposite directions.
//
// A ceiling set too LOW lets planning pick a picture that does not leave room
// for the padding, and then the file cannot come out the size it promised.
// A ceiling set too HIGH costs a smaller picture than the file had room for -
// which nothing else notices, because the size is still exact and the run still
// succeeds.
//
// The sizes below span the whole ladder and reach well past it, so a step that
// stopped being reachable at any size shows up as a picture that never grows.
func TestAvifDrawsAPictureThatGrowsWithTheFileAndAlwaysFits(t *testing.T) {
	d, err := format.Get("avif")
	if err != nil {
		t.Fatal(err)
	}

	sizes := []int64{
		311, 312, 320, 400, 500, 600, 700, 800, 1000, 1200,
		1500, 1800, 2000, 2500, 3000, 4000, 6000, 8000, 20000, 65536,
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
		t.Fatalf("the largest picture over the whole sweep was %d pixels, so no step of the ladder is reachable and this guard measured nothing", biggest)
	}
	t.Logf("largest picture reached over the sweep: %d pixels", biggest)
}

// heaviestAvifSeeds are the seeds the thorough sweep (tools/probes/avifladder,
// 768 seeds) found heaviest on 2026-11-12, asked beside the first few from
// zero. The first few alone stayed green with both ceilings set back to the
// numbers that were too low, so they could not see what the sweep found. A
// codec raised underneath moves every rung at once, so the heaviest seeds of
// today are a fair sample of tomorrow's.
var heaviestAvifSeeds = []uint64{
	0x65a47abe4364edd2, 0xfb0ac60c4be71cd6, 0x949ac3a5a267fad4, 0xf3fc5ddbb41fd92,
	0xd2, 0xea, 0xf4, 0xfb, 0xfe, 0xff,
}

// Every rung's two ceilings still cover what that rung codes to: the first
// with the label of a file at the edge of the rung, the second with the label
// of a damaged file.
//
// A file at the edge of a rung - asked for at its ceiling, where the picture
// leaves the least room - states a size with the digits of the ceiling, so the
// first ceiling is asked with those. A damaged file's label names its damage,
// and a longer label is more ink until it is too long for its scale, so the
// second is asked with the longest damaged label every scale still draws.
// Asking one byte of a named picture size makes the format state the file
// that picture needs, which is what a ceiling is a bound on (2026-11-12, when
// both were found too low - O273 and the review of #234).
//
// A handful of seeds from zero, as the JPEG XL guard takes, and the heaviest
// the thorough sweep found (heaviestAvifSeeds).
func TestEveryAvifRungStillFitsInsideItsDeclaredCeilings(t *testing.T) {
	d, err := format.Get("avif")
	if err != nil {
		t.Fatal(err)
	}
	rungs := avif.Rungs()
	if len(rungs) == 0 {
		t.Fatal("the ladder is empty, so this guard measured nothing")
	}
	first := uint64(8)
	if raceEnabled {
		first = 2
	}
	seeds := slices.Clone(heaviestAvifSeeds)
	for seed := range first {
		seeds = append(seeds, seed)
	}

	type ask struct {
		r       format.Request
		ceiling int64
	}
	drawn := 0
	for _, rung := range rungs {
		w, h := int(rung[0]), int(rung[1])
		named := map[string]string{"width": strconv.Itoa(w), "height": strconv.Itoa(h)}
		over, needs, worst := int64(0), int64(0), ask{}
		for _, seed := range seeds {
			asks := []ask{{format.Request{Bytes: 1, Seed: seed, Properties: named}, rung[2]}}
			// The size on disk is what the label states: the ceiling itself,
			// a tenth above it, and the lightest and the heaviest number with
			// as many digits.
			eights, lightest := int64(0), int64(1)
			for n := rung[2]; n > 0; n /= 10 {
				eights, lightest = eights*10+8, lightest*10
			}
			lightest /= 10
			for _, size := range []int64{rung[2], rung[2] + rung[2]/10, lightest, eights} {
				asks = append(asks, ask{format.Request{Bytes: 1, Seed: seed, Label: true, OnDisk: size, Properties: named}, rung[2]})
			}
			// A damaged file states a size of any length the damage leaves it.
			plain := format.Request{Bytes: 1, Seed: seed, Label: true, OnDisk: 1e12, Properties: named}
			for _, damage := range damagesDrawnAtEveryScale(t, plain, "avif", w) {
				r := plain
				r.Damage = damage
				asks = append(asks, ask{r, rung[3]})
				drawn++
			}
			for _, a := range asks {
				_, err := d.Generator.Plan(a.r)
				var below *format.BelowMinimumError
				if !errors.As(err, &below) {
					t.Fatalf("a %dx%d picture refused one byte with %T rather than a BelowMinimumError: %v", w, h, err, err)
				}
				if below.Minimum-a.ceiling > over {
					over, needs, worst = below.Minimum-a.ceiling, below.Minimum, a
				}
			}
		}
		if over > 0 {
			t.Errorf("the %dx%d rung declares a ceiling of %d B and its picture needs %d B (seed %d, label %t, size %d, damage %q).\n"+
				"A ceiling below what its rung produces lets planning pick a picture the file cannot hold, and writing then refuses a size planning accepted.\n"+
				"What to do: run tools/probes/avifladder and record what it measures.",
				w, h, worst.ceiling, needs, worst.r.Seed, worst.r.Label, worst.r.OnDisk, worst.r.Damage)
		}
	}
	if drawn == 0 {
		t.Fatal("no rung is wide enough to draw a damaged label, so this guard asked nothing about one")
	}
}

// Planning must not code the picture.
//
// It used to, and the price was measured on 2026-08-29: a preview of a thousand
// files took 51 seconds where BMP took a fifth of one, and twenty thousand -
// which is what one guard on the window asks for - never finished inside the
// test timeout. This project has already named that shape a defect once, for
// archives, and the rule it wrote then is that the preview is the step that has
// to be cheap.
//
// Asked in ALLOCATIONS rather than on the clock, because allocation counts are
// deterministic and a time based gate would be flaky on a busy runner - the
// same reasoning the memory guards use. One encode allocates on the order of
// half a megabyte, so fifty plans coming in under a single write is not a close
// call.
func TestAvifPlanningDoesNotCodeThePicture(t *testing.T) {
	d, err := format.Get("avif")
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
			"What to do: planning picks a step of the ladder from its ceiling, and the encode belongs in Write.",
			plans, planning, writing)
	}
}

// A quality the ceilings were not measured at still produces a file.
//
// This one was found by reading the code, which is worth saying out loud: the
// ceiling table speaks for the default quality and nothing else, and a request
// for quality 100 without a named picture size took a step off that table whose
// picture is several times larger than the ceiling claims. Measured on
// 2026-08-29 before the repair: `--size 20kb --set quality=100` produced no
// file at all, and the run said so rather than writing a short one.
//
// The repair is that a named quality walks the ladder coding each step, which
// is the slow road, taken only by runs that asked for it.
func TestAvifStillFitsWhenTheQualityIsNotTheOneTheCeilingsWereMeasuredAt(t *testing.T) {
	d, err := format.Get("avif")
	if err != nil {
		t.Fatal(err)
	}

	for _, quality := range []string{"1", "10", "60", "90", "100"} {
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
