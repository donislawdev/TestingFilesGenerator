package guard

import (
	"context"
	"runtime"
	"slices"
	"testing"

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

// heaviestJxlSeeds are, for each rung that draws a label, the seeds the
// thorough sweep (tools/probes/jxlladder, 768 seeds) found heaviest on
// 2026-11-12 - for the label of a file at the edge of the rung and for the
// label of a damaged one. The first few seeds alone stayed green with the
// ceiling for a damaged file set back to the number that was too low, so they
// could not see what the sweep found.
var heaviestJxlSeeds = map[int][]uint64{
	640: {0xc76d0104c41a71bd, 0x8f27bfdd1384288d},
	320: {0x29b351201f9ae4c5, 0xe162c2724da869d3},
	256: {0x3d, 0x80e38da90dd80935},
	160: {0xb9f4567924888695},
}

// Every rung's two ceilings still cover what that rung produces: the first
// with the label of a file at the edge of the rung, the second with the label
// of a damaged file.
//
// The guard above walks the ladder the way a run does, so it only ever sees
// the rung each size happens to select, at one seed. This one asks each rung
// directly, across seeds, which is the question the ceiling is an answer to.
//
// A file at the edge of a rung - asked for at its ceiling, where the picture
// leaves the least room - states a size with the digits of the ceiling, so the
// first ceiling is asked with those. A label with more digits is a file with
// far more room than its picture needs. A damaged file's label names its
// damage, and a longer label is more ink until it is too long for its scale, so
// the second is asked with the longest damaged label every scale still draws.
//
// A handful of seeds from zero and the heaviest the thorough sweep found
// (heaviestJxlSeeds): this is the version that runs on every push and would
// still catch a codec raised underneath us, because a new encoder moves every
// rung at once rather than one seed of one rung.
//
// Fewer seeds again under the race detector, and that needs saying rather than
// hiding. This encoder is slow and the detector makes it slower: measured on
// 2026-08-31, this one test took 80 s of the 102 s the four JPEG XL guards cost
// under -race, and the whole guard package went past the 25 minute timeout the
// race job allows. What that job exists to find is data races, and this test
// walks the same single threaded arithmetic whatever it is given - so the
// seeds it drops buy no race coverage at all. The full sweep still runs on all
// three operating system jobs, which is where a wrong ceiling would show.
func TestEveryJxlRungStillFitsInsideItsDeclaredCeiling(t *testing.T) {
	rungs := jxl.Rungs()
	if len(rungs) == 0 {
		t.Fatal("the ladder is empty, so this guard measured nothing")
	}

	first := uint64(8)
	if raceEnabled {
		first = 2
		t.Logf("the race detector is on, so this sweeps %d seeds from zero rather than 8 - it costs minutes here and finds no races either way", first)
	}

	type ask struct {
		label   string
		ceiling int64
	}
	drawn := 0
	for _, r := range rungs {
		w, h := int(r[0]), int(r[1])
		seeds := slices.Clone(heaviestJxlSeeds[w])
		for seed := range first {
			seeds = append(seeds, seed)
		}
		// The size the label states: the ceiling itself, a tenth above it,
		// and the lightest and the heaviest number with as many digits.
		eights, lightest := int64(0), int64(1)
		for n := r[2]; n > 0; n /= 10 {
			eights, lightest = eights*10+8, lightest*10
		}
		lightest /= 10
		var worst ask
		over, size, worstSeed := int64(0), int64(0), uint64(0)
		for _, seed := range seeds {
			asks := []ask{{"", r[2]}}
			for _, want := range []int64{r[2], r[2] + r[2]/10, lightest, eights} {
				asks = append(asks, ask{format.Request{Bytes: want, Seed: seed}.LabelText("jxl"), r[2]})
			}
			// A damaged file states a size of any length the damage leaves it.
			plain := format.Request{Bytes: 1e12, Seed: seed}
			for _, damage := range damagesDrawnAtEveryScale(t, plain, "jxl", w) {
				damaged := plain
				damaged.Damage = damage
				asks = append(asks, ask{damaged.LabelText("jxl"), r[3]})
				drawn++
			}
			for _, a := range asks {
				n, err := jxl.FileSizeFor(w, h, jxl.DefaultQuality, seed, a.label)
				if err != nil {
					t.Fatalf("%dx%d seed %d: %v", w, h, seed, err)
				}
				if int64(n)-a.ceiling > over {
					over, size, worst, worstSeed = int64(n)-a.ceiling, int64(n), a, seed
				}
			}
		}
		if over > 0 {
			t.Errorf("the %dx%d rung declares a ceiling of %d B and produced %d B at seed %#x, with the label %q.\n"+
				"A ceiling below what its rung produces lets planning pick a picture the file cannot hold.\n"+
				"What to do: run tools/probes/jxlladder and record what it measures.",
				w, h, worst.ceiling, size, worstSeed, worst.label)
		}
	}
	if drawn == 0 {
		t.Fatal("no rung is wide enough to draw a damaged label, so this guard asked nothing about one")
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

	// The sizes matter as much as the qualities, and the first version of this
	// guard had too few of them. It asked 2000, 20000 and 200000, and the full
	// mutation run of 2026-09-01 called it a HOLE: turning the slow road off
	// left it green. Measured afterwards by sweeping, the break shows at
	// quality 100 and 5000 B, where a 160x120 picture codes to 6047 B and the
	// file was to be 5000 - so the rung the table picked does not fit at all.
	//
	// The guard was right about what it watches and short on where it looked,
	// which is the same shape as O163 one file over: a measurement proves what
	// its sample covers and says nothing about the gap beside it.
	for _, quality := range []string{"1", "10", "70", "90", "100"} {
		for _, want := range []int64{2000, 5000, 20000, 60000, 200000} {
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
