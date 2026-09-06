package guard

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The ladder ceiling is above every picture the top rung can make.
//
// Planning used to encode the picture only to learn its length, throw that away
// and let the writer encode it again. That confirmation was 38 to 53% of a PNG
// run. It is skipped now whenever the request is far enough above the top rung
// that the search cannot come out any other way (P7 in the performance review
// of 2026-09-05), which measured 2.04x less processor time over 300 files.
//
// The whole of that rests on one claim: the top rung never encodes to more than
// ladderCeiling. If it did, planning would pick a rung whose picture does not
// leave room for the padding, and the writer would refuse a file that planning
// had already accepted - which breaks the promise that a preview gives the same
// verdict as the run.
//
// So this sweeps it. The number is read out of the source rather than copied
// here, because a copy is a second place to update and this repository has a
// guard whose whole job is finding numbers copied into prose.
func TestTheLadderCeilingIsAboveEveryPictureTheTopRungMakes(t *testing.T) {
	// The top rung of every ladder, and what the format spends on top of the
	// picture before any padding can start. Both are read from the source
	// below, so this table only names what to sweep.
	cases := []struct {
		id       string
		file     string
		topWidth int
		topHigh  int
		// extra sweeps the setting that moves the encoded size most: frames
		// for GIF, nothing for PNG.
		extra []map[string]string
	}{
		{
			id: "png", file: "../format/png/png.go", topWidth: 640, topHigh: 480,
			extra: []map[string]string{{}},
		},
		{
			id: "gif", file: "../format/gif/gif.go", topWidth: 640, topHigh: 480,
			extra: []map[string]string{
				{"frames": "1"}, {"frames": "3"}, {"frames": "10"},
				{"frames": "30"}, {"frames": "60"},
			},
		},
	}

	// Seeds picked to spread the gradient offset, which is seed%256, rather
	// than to look random.
	seeds := []uint64{0, 1, 7, 42, 127, 128, 255, 256, 777, 7741, 99991, 123456}

	for _, c := range cases {
		ceiling := ceilingFromSource(t, c.file)
		d, err := format.Get(c.id)
		if err != nil {
			t.Fatalf("%s is not registered: %v", c.id, err)
		}

		worst := int64(0)
		var worstAt string
		for _, extra := range c.extra {
			for _, seed := range seeds {
				for _, label := range []bool{true, false} {
					props := map[string]string{
						"width":  strconv.Itoa(c.topWidth),
						"height": strconv.Itoa(c.topHigh),
					}
					for k, v := range extra {
						props[k] = v
					}

					// Asking for one byte at the top rung makes the format
					// state its own floor, and that floor is the encoded
					// picture plus whatever it always carries. Nothing else
					// reports the encoded size from outside the package.
					_, err := d.Generator.Plan(format.Request{
						Bytes: 1, Seed: seed, Label: label, Properties: props,
					})
					var below *format.BelowMinimumError
					if !errors.As(err, &below) {
						t.Fatalf("%s at %dx%d refused one byte with %T rather than a BelowMinimumError: %v",
							c.id, c.topWidth, c.topHigh, err, err)
					}
					if below.Minimum > worst {
						worst = below.Minimum
						worstAt = fmt.Sprintf("seed %d, label %v, %v", seed, label, extra)
					}
				}
			}
		}

		if worst > ceiling {
			t.Errorf("%s: the top rung reaches %d B (%s) but ladderCeiling is %d.\n"+
				"Planning skips the encoding for any request above the ceiling and takes the top rung "+
				"on trust. A picture bigger than the ceiling leaves less room than planning assumed, so "+
				"the writer refuses a size planning accepted - a preview and a run disagreeing, which is "+
				"a row on the regression surface. Raise ladderCeiling in %s past %d.",
				c.id, worst, worstAt, ceiling, c.file, worst)
		}
		t.Logf("%s: worst top rung %d B against a ceiling of %d, %.1fx of headroom (%s)",
			c.id, worst, ceiling, float64(ceiling)/float64(worst), worstAt)
	}
}

// ceilingFromSource reads the ladderCeiling constant out of a format package,
// so the number lives in exactly one place.
func ceilingFromSource(t *testing.T, path string) int64 {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	var found int64
	seen := false
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if name.Name != "ladderCeiling" || i >= len(spec.Values) {
				continue
			}
			lit, ok := spec.Values[i].(*ast.BasicLit)
			if !ok || lit.Kind != token.INT {
				continue
			}
			v, err := strconv.ParseInt(lit.Value, 0, 64)
			if err != nil {
				continue
			}
			found, seen = v, true
		}
		return true
	})
	if !seen {
		t.Fatalf("%s has no ladderCeiling constant. If the fast path was removed, remove this guard "+
			"with it rather than leaving it passing on nothing.", path)
	}
	return found
}

// Planning a picture format does not code the picture.
//
// The companion to the ceiling guard above: that one says the fast path is
// SAFE, this one says it is actually being taken. Without it the ceiling could
// be perfectly correct while planning encodes anyway, and the only symptom
// would be a preview of a large run costing what the run costs - which is the
// defect AVIF was rebuilt for and the same shape PNG and GIF carried until
// 2026-09-06.
//
// Fifty plans against one write, because a plan that codes is within a factor
// of one of a write and the gap is otherwise enormous. It asks the ALLOCATOR
// rather than the clock: allocation counts are deterministic, and a time based
// gate is flaky on a loaded runner.
func TestPlanningAPictureDoesNotCodeIt(t *testing.T) {
	for _, id := range []string{"png", "gif"} {
		t.Run(id, func(t *testing.T) {
			d, err := format.Get(id)
			if err != nil {
				t.Fatal(err)
			}

			const plans = 50
			// Comfortably above both ceilings, so the fast path is the one
			// under test rather than the ladder walk.
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
				t.Errorf("%d plans of %s allocated %d B and one write allocated %d B.\n"+
					"Planning is coding the picture, which is what makes a preview of a large run cost what "+
					"the run costs. Planning takes the top rung from ladderCeiling and the encode belongs in Write.",
					plans, id, planning, writing)
			}
		})
	}
}
