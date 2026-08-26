package guard

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A generator must not need as much memory as the file it produces.
//
// Measured before this guard existed: a 600 MiB PNG peaked at 613 MB of
// memory while the same size of text peaked at 42 MB, because the padding was
// built as one buffer. At the sizes the presets declare, that ends the run.

// allocCeiling is how many objects a generator may allocate for one 64 MiB
// file, whatever the format.
//
// Measured 2026-08-02, after the two defects this number was introduced to
// catch were fixed: png 56, zip 39, wav 11, everything else at most 7. The
// ceiling sits at more than twice the worst of those, because the runtime
// moves a little between runs - png was seen at 55, 56 and 59 - and a guard
// that reddens on noise gets switched off.
//
// The slack costs nothing, because the failure this catches is not a few
// objects over. It is allocating once per item: png allocated 786443 objects
// for one image before SetRGBA replaced Set, and md 163842 for one document
// before a one character ToUpper stopped building a string per word. Four
// orders of magnitude, not a few percent.
//
// A ratchet. It goes down when work makes it lowerable, never up to turn a run
// green - the same rule as the coverage threshold and the pinned values.
const allocCeiling = 128

func TestNoGeneratorHoldsTheWholeFileInMemory(t *testing.T) {
	const size = 64 << 20 // 64 MiB, large enough that holding it would show

	for _, d := range format.All() {
		t.Run(d.ID, func(t *testing.T) {
			plan, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
			if err != nil {
				t.Fatalf("planning %d B failed: %v", size, err)
			}

			runtime.GC()
			var before runtime.MemStats
			runtime.ReadMemStats(&before)

			counted := &countingSink{}
			if err := d.Generator.Write(context.Background(), counted, plan); err != nil {
				t.Fatalf("writing failed: %v", err)
			}

			var after runtime.MemStats
			runtime.ReadMemStats(&after)

			if counted.n != size {
				t.Fatalf("produced %d B, expected %d B", counted.n, size)
			}

			// Allocated over the whole write, not held at once, so the budget
			// is generous. What it catches is a generator that builds the
			// entire file before handing it over.
			grew := int64(after.TotalAlloc - before.TotalAlloc)
			if grew > size/2 {
				t.Errorf("%s allocated %d B while producing a %d B file - it is building the file in memory rather than streaming it",
					d.ID, grew, size)
			}

			// How many objects, not how many bytes. The two answer different
			// questions: a generator can stream perfectly and still allocate
			// once per pixel, which is what PNG did - 786443 objects for one
			// image, because Set takes a color.Color interface and boxes every
			// colour. The byte budget above never noticed, because the boxes
			// are small and short lived.
			//
			// This is the dimension worth gating. Allocation counts are
			// deterministic - measured identical across runs - while wall clock
			// speed is not, so a time based gate would be flaky on CI runners
			// and a flaky guard gets switched off.
			//
			// Deterministic across runs of the SAME binary, which is the part
			// that needed saying. The race detector allocates on its own
			// account, so the number it produces is not the number this ceiling
			// was measured against. Found on 2026-08-20, the first race run
			// this project has had since the toolkit arrived: pptx counted 130
			// against a ceiling of 128, while the byte figure above stayed at
			// 871 KiB for a 64 MiB file - so the property this guard exists for
			// was never in doubt, only the instrument.
			//
			// The byte check is deliberately left running under the detector.
			// It is the half that answers "is the whole file in memory", and it
			// answers it just as well with instrumentation in the way.
			objects := int64(after.Mallocs - before.Mallocs)
			switch {
			case raceEnabled:
				t.Logf("%s: object ceiling not applied under the race detector, which allocates on its own account - %d objects seen",
					d.ID, objects)
			case objects > allocCeiling:
				t.Errorf("%s allocated %d objects producing a %d B file, ceiling is %d - something in the loop is allocating per item",
					d.ID, objects, size, allocCeiling)
			}
			t.Logf("%s: allocated %d KiB in %d objects, producing %d KiB",
				d.ID, grew/1024, objects, int64(size)/1024)
		})
	}
}

// Planning works out every file before anything is written, which is what
// buys the refusal before the first byte and the free space check. The price
// is that the whole plan sits in memory.
//
// Measured: about 2.3 kB per planned file, so ten thousand files cost 12 MB
// and a million would cost a couple of gigabytes. Ten thousand is the stated
// design point, and this guard keeps the cost there from creeping up.
func TestPlanningTenThousandFilesStaysCheap(t *testing.T) {
	const count = 10000

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	planned, err := engine.Plan(
		[]engine.Target{{ID: "many", Format: "txt", Sizes: engine.Uniform(count, 1024), Label: true}},
		engine.Options{OutDir: t.TempDir(), Seed: 7741, Command: "test"},
	)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if len(planned) != count {
		t.Fatalf("planned %d files, expected %d", len(planned), count)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	perFile := int64(after.TotalAlloc-before.TotalAlloc) / count

	const budget = 8 << 10
	if perFile > budget {
		t.Errorf("planning costs %d B per file, budget is %d B - the whole plan is held in memory, so this sets the ceiling on how many files a run can have",
			perFile, budget)
	}
	t.Logf("planning cost %d B per file", perFile)
}

// Planning a container must not generate what the container holds.
//
// This project tells people to run --dry-run before anything large, so the
// preview is the step that has to be cheap. It was not: measuring an archive
// meant building it, and building it meant running every generator inside, two
// or three times over. Measured on 2026-08-03, a 256 MB archive from contents:
// 960 ms to preview against 56 ms for a plain file of the same size, and 1585
// ms to actually write it. The preview cost about what the run costs.
//
// Asserted on the clock, which this project otherwise refuses to gate on
// because the spread between runners is real. It is legitimate here and only
// here, because the gap is not a percentage: the declared contents below come
// to 10 GB, so the old path had to push about 30 GB through a CRC before
// answering - tens of seconds on any machine - while the arithmetic answers in
// about a millisecond however large the number is. Four orders of magnitude,
// so no runner sits anywhere near the line.
//
// Ten gigabytes rather than a hundred on purpose. The mutation that proves this
// guard restores the old path and has to run to completion, and the owner's
// mutation runs are already long enough.
func TestPlanningAContainerDoesNotGenerateWhatItHolds(t *testing.T) {
	const (
		ceiling  = 20 * time.Second
		declared = 30 * (100 << 20) // 30 files of 100 MB, see the note below
	)

	started := time.Now()
	planned, err := engine.Plan(
		[]engine.Target{{
			ID:     "huge",
			Format: "zip",
			Sizes:  engine.Uniform(1, 0),
			// No size of its own, so the archive works it out from what it
			// holds and nothing is padded.
			SizeFromContents: true,
			Contains: []format.Content{
				// Three gigabytes rather than the ten this asked for until
				// 2026-08-26. ZIP now refuses an archive past four, because
				// the arithmetic that works out its size cannot see the zip64
				// records - measured, the writer produced 112 B more than the
				// plan above that line. Three still makes the point of this
				// guard as plainly as ten did: a build that generated what the
				// archive holds would be writing three gigabytes here, and the
				// clock below would say so.
				{Format: "txt", Count: 30, Bytes: 100 << 20},
			},
			Label: true,
		}},
		engine.Options{OutDir: t.TempDir(), Seed: 7741, Command: "test"},
	)
	elapsed := time.Since(started)

	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if len(planned) != 1 {
		t.Fatalf("planned %d files, expected 1", len(planned))
	}

	// The number still has to be right. A fast answer that is wrong would pass
	// the clock and break the one promise this tool makes.
	if got := planned[0].Plan.Bytes; got < declared {
		t.Errorf("the archive plans %d B while holding %d B, so the contents are not being counted", got, int64(declared))
	}
	if elapsed > ceiling {
		t.Errorf("planning an archive of %d B of declared contents took %s, ceiling is %s - the contents are being generated in order to measure them",
			int64(declared), elapsed.Round(time.Millisecond), ceiling)
	}
	t.Logf("planned %d B of declared contents in %s", int64(declared), elapsed.Round(time.Millisecond))
}

type countingSink struct{ n int64 }

func (c *countingSink) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

var _ io.Writer = (*countingSink)(nil)

// A property key nobody recognises is a typo, and a typo taken in silence
// gives a file with default settings and an hour spent wondering why the test
// passes when it should not.
func TestAnUnknownPropertyIsRefused(t *testing.T) {
	for _, d := range format.All() {
		t.Run(d.ID, func(t *testing.T) {
			err := d.CheckProperties(map[string]string{"definitely-not-a-real-property": "1"})
			if err == nil {
				t.Fatalf("%s accepted a property it does not have", d.ID)
			}
			var unknown *format.UnknownPropertyError
			if !errors.As(err, &unknown) {
				t.Errorf("refused with %T, expected an UnknownPropertyError", err)
			}

			// Everything the format does declare has to pass, otherwise this
			// guard would be satisfied by refusing everything.
			//
			// The value comes from the declaration rather than being a fixed
			// "1", which used to work only because nothing checked values. Now
			// it also proves the declaration agrees with itself: a default the
			// format advertises and then refuses would be a worse trap than no
			// default at all.
			accepted := map[string]string{}
			for _, p := range d.Properties {
				accepted[p.Name] = acceptableValue(p)
			}
			if len(accepted) > 0 {
				if err := d.CheckProperties(accepted); err != nil {
					t.Errorf("%s refused its own declared properties: %v", d.ID, err)
				}
			}
		})
	}
}

// A near typo is the case that matters. "widht" instead of "width" is what
// people actually write.
func TestATypoInAPropertyReachesTheUser(t *testing.T) {
	targets := []engine.Target{{
		ID: "images", Format: "png", Sizes: engine.Uniform(1, 200<<10), Label: true,
		Properties: map[string]string{"widht": "100"},
	}}

	_, err := engine.Plan(targets, engine.Options{OutDir: t.TempDir(), Seed: 1, Command: "test"})
	if err == nil {
		t.Fatal("a misspelled property was accepted, and the file would have been produced with default dimensions")
	}
	var unknown *format.UnknownPropertyError
	if !errors.As(err, &unknown) {
		t.Errorf("refused with %T, expected an UnknownPropertyError", err)
	}
}

// The picture is held in memory while it is encoded, so there is a size past
// which a request has to be refused rather than ending the run with an out of
// memory error nobody can act on.
func TestAnImageTooLargeToHoldIsRefused(t *testing.T) {
	d, err := format.Get("png")
	if err != nil {
		t.Skip("png is not registered")
	}
	_, err = d.Generator.Plan(format.Request{
		Bytes: 5 << 20, Seed: 1, Label: true,
		Properties: map[string]string{"width": "20000", "height": "20000"},
	})
	if err == nil {
		t.Error("a 400 megapixel picture was accepted - measured at 4.65 GB of memory before this was capped")
	}
}

// acceptableValue is a value the declaration says it takes, worked out from
// the declaration alone.
//
// The default is used where there is one, because a format advertising a
// default it then refuses is a trap rather than a mistake, and this is the one
// place able to notice.
func acceptableValue(p format.Property) string {
	if p.Default != "" {
		return p.Default
	}
	switch p.Kind {
	case format.PropertyChoice:
		if len(p.Choices) > 0 {
			return p.Choices[0]
		}
		return ""
	case format.PropertyBool:
		return "true"
	case format.PropertySize:
		return "1kb"
	case format.PropertyInt:
		if p.Min != 0 || p.Max != 0 {
			return strconv.FormatInt(p.Min, 10)
		}
		return "1"
	default:
		return "x"
	}
}
