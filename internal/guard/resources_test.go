package guard

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A generator must not need as much memory as the file it produces.
//
// Measured before this guard existed: a 600 MiB PNG peaked at 613 MB of
// memory while the same size of text peaked at 42 MB, because the padding was
// built as one buffer. At the sizes the presets declare, that ends the run.

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
			t.Logf("%s: allocated %d KiB producing %d KiB", d.ID, grew/1024, int64(size)/1024)
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
		[]engine.Target{{ID: "many", Format: "txt", Count: count, Bytes: 1024, Label: true}},
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
			accepted := map[string]string{}
			for _, k := range d.Properties {
				accepted[k] = "1"
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
		ID: "images", Format: "png", Count: 1, Bytes: 200 << 10, Label: true,
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
