package guard

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	pictures "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// The walk to the smallest file plans one picture a round at a time, and a
// picture with its width and height set encodes itself to know its length. It
// encoded the same picture in every round until 2026-11-18 - two encodes where
// one would do, 94% of planning the picture-sizes preset (O357,
// docs/SMALLEST-ONE-ENCODE-2026-11-18.md). Now a round takes what the one
// before it measured (format.Measure). The guards below hold the three things
// that has to stay true: the picture is encoded once, a walk that remembers
// writes the bytes of a walk that measures again, and what is remembered is
// remembered under everything it was measured from.

// TestTheSmallestPictureIsEncodedOnceWhileItIsPlanned plans the smallest file
// of every picture format and asks that the walk cost, in bytes allocated, not
// much more than planning that one file at its size once.
//
// Allocation rather than time, because it is the same number on every run of
// one build: measured on 2026-11-18 at 256x192, a walk was 2.00 plans in each
// of the six formats that encode while planning, and 1.00 to 1.02 after. The
// four that work their length out by arithmetic (bmp, svg, tiff, webp) plan in
// a few kilobytes, which the 16 KiB of room covers.
//
// The canary is the same walk with every round measuring afresh, a generator
// that drops what the walk kept: it has to be caught in jpg and png, the two
// formats the presets of pictures default to, or the check cannot see a second
// encode at all.
func TestTheSmallestPictureIsEncodedOnceWhileItIsPlanned(t *testing.T) {
	ctx := context.Background()
	var caught []string
	for _, id := range pictures.Formats() {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		r := smallestPictureRequest(false)
		p, err := d.PlanSmallest(ctx, r)
		if err != nil {
			t.Fatalf("%s: the smallest file at 256x192 was refused, so there is no walk to measure: %v", id, err)
		}
		at := r
		at.Bytes = p.Bytes
		one := leastAllocated(t, id, func() error { _, err := d.Generator.Plan(at); return err })
		walk := leastAllocated(t, id, func() error { _, err := d.PlanSmallest(ctx, r); return err })
		forgetful := d
		forgetful.Generator = forgettingWalk{d.Generator}
		again := leastAllocated(t, id, func() error { _, err := forgetful.PlanSmallest(ctx, r); return err })

		if tooDear(walk, one) {
			t.Errorf("%s: the walk to the smallest file allocated %d B where planning that file once allocated %d B - "+
				"the picture is encoded in more than one round. A plan that encodes measures through format.Measure "+
				"with the request's Measured, keyed by everything the picture is drawn from", id, walk, one)
		}
		if tooDear(again, one) {
			caught = append(caught, id)
		}
	}
	for _, id := range []string{"jpg", "png"} {
		if !slices.Contains(caught, id) {
			t.Errorf("canary - a walk of %s whose every round measures afresh was not caught (caught: %v), so this check "+
				"cannot see a second encode", id, caught)
		}
	}
}

// tooDear says a walk cost more than one plan and half another, with 16 KiB of
// room for a format whose plan is arithmetic.
func tooDear(walk, one uint64) bool {
	return walk > one+one/2+16<<10
}

// forgettingWalk plans every round with nothing the walk kept, which is how a
// walk planned before it kept anything.
type forgettingWalk struct{ format.Generator }

func (f forgettingWalk) Plan(r format.Request) (format.Plan, error) {
	r.Measured = nil
	return f.Generator.Plan(r)
}

// leastAllocated is the fewest bytes two calls allocated, after one call that
// fills whatever a first call fills.
func leastAllocated(t *testing.T, id string, call func() error) uint64 {
	t.Helper()
	if err := call(); err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	least := ^uint64(0)
	for i := 0; i < 2; i++ {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if err := call(); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		runtime.ReadMemStats(&after)
		least = min(least, after.TotalAlloc-before.TotalAlloc)
	}
	return least
}

// smallestPictureRequest asks for a 256x192 picture with its label, or for a
// placeholder when placeholder is set, as the walk to the smallest file is
// asked.
func smallestPictureRequest(placeholder bool) format.Request {
	props := map[string]string{"width": "256", "height": "192"}
	if placeholder {
		props["content"] = "placeholder"
	}
	return format.Request{Label: true, Seed: 7, Number: 3, Properties: props}
}

// TestAWalkThatRemembersWritesTheBytesOfOneThatMeasuresAgain plans the
// smallest file of every picture format twice - the walk as it runs, and the
// walk with every round measuring afresh - and writes both. Same size, same
// bytes.
//
// avif, jxl and ico write the bytes their plan coded, so a value remembered
// under a key that left out something the picture is drawn from would be
// another picture written without a word. jpg, png and gif encode again while
// writing and stop on a length the plan did not expect. The golden values pin
// the smallest file of png, jpg and svg only (generatorbytes_test.go), so this
// asks the other seven too, and a placeholder wherever a format draws one.
//
// With the width and height set every round draws the same picture, so only
// the walk without them - whose rounds measure different rungs of a ladder -
// can show a value handed back for another picture. Measured by hand on
// 2026-11-18: format.Measure with its key comparison taken out reddened that
// walk in jpg, png, gif and ico.
func TestAWalkThatRemembersWritesTheBytesOfOneThatMeasuresAgain(t *testing.T) {
	ctx := context.Background()
	formats, placeholders := pictures.Formats(), 0
	for _, id := range formats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		forgetful := d
		forgetful.Generator = forgettingWalk{d.Generator}
		for _, shape := range []string{"256x192", "a placeholder", "no size"} {
			placeholder := shape == "a placeholder"
			if placeholder && !slices.Contains(d.PropertyNames(), "content") {
				continue
			}
			r := smallestPictureRequest(placeholder)
			if shape == "no size" {
				// The ladder of sizes: the first round measures every rung
				// down to one pixel, the second the rungs down to the one it
				// takes. Rounds asking for different pictures are what a value
				// handed back under another key would show in.
				r.Properties = nil
			}
			remembered, err := d.PlanSmallest(ctx, r)
			if err != nil {
				t.Fatalf("%s at %s: %v", id, shape, err)
			}
			measured, err := forgetful.PlanSmallest(ctx, r)
			if err != nil {
				t.Fatalf("%s at %s, every round measuring afresh: %v", id, shape, err)
			}
			got, want := writtenSum(t, d, remembered), writtenSum(t, d, measured)
			if remembered.Bytes != measured.Bytes || got != want {
				t.Errorf("%s at %s: the walk that remembers planned %d B and wrote %s, the walk that measures again "+
					"%d B and %s - a round took a value measured for another picture",
					id, shape, remembered.Bytes, got, measured.Bytes, want)
			}
			if placeholder {
				placeholders++
			}
		}
	}
	if len(formats) < 10 || placeholders < 3 {
		t.Fatalf("asked %d picture formats and %d placeholders - ten formats and the placeholders of png, jpg and svg "+
			"were registered when this was written, so this stopped reaching some of them", len(formats), placeholders)
	}
}

// writtenSum writes the file p plans and returns its SHA-256.
func writtenSum(t *testing.T, d format.Descriptor, p format.Plan) string {
	t.Helper()
	h := sha256.New()
	if err := d.Generator.Write(context.Background(), h, p); err != nil {
		t.Fatalf("%s: writing the plan of %d B: %v", d.ID, p.Bytes, err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestAMeasurementIsKeptForAnEqualKeyOnly asks format.Measure the four things
// a walk leans on: an equal key is measured once, another key is measured on
// its own, a failure is not kept, and a request outside a walk measures every
// time.
func TestAMeasurementIsKeptForAnEqualKeyOnly(t *testing.T) {
	type key struct {
		side int
		seed uint64
	}
	calls := 0
	double := func(k key) (int, error) {
		calls++
		if k.side < 0 {
			return 0, errors.New("no picture of a negative side")
		}
		return k.side * 2, nil
	}
	walk := &format.Measured{}
	for _, c := range []struct {
		name  string
		in    *format.Measured
		of    key
		want  int
		calls int
		fails bool
	}{
		{"first", walk, key{8, 1}, 16, 1, false},
		{"the same key again", walk, key{8, 1}, 16, 1, false},
		{"another seed", walk, key{8, 2}, 16, 2, false},
		{"another side", walk, key{9, 1}, 18, 3, false},
		{"a failure", walk, key{-1, 1}, 0, 4, true},
		{"the failure again", walk, key{-1, 1}, 0, 5, true},
		{"outside a walk", nil, key{8, 1}, 16, 6, false},
		{"outside a walk again", nil, key{8, 1}, 16, 7, false},
	} {
		got, err := format.Measure(c.in, c.of, double)
		if got != c.want || calls != c.calls || (err != nil) != c.fails {
			t.Errorf("%s: got %d (%v) after %d measurements, want %d after %d", c.name, got, err, calls, c.want, c.calls)
		}
	}
}

// TestEveryMeasurementIsAFunctionOfItsKeyAlone reads every call of
// format.Measure in the shipped code and asks that what it measures with is a
// function of its package named there - not a closure, not a variable, not a
// method value.
//
// A value is remembered under the key it was measured from, and that is safe
// only while the measuring reads nothing but the key. A closure can read a
// setting the key left out, and then a second round with another value of that
// setting takes the first round's picture. The types hold the rest: a format's
// drawing is what its picture and encode functions take, so a function of the
// drawing alone cannot see more.
//
// Read from the syntax, so a local variable holding a closure under the name
// of a function of the package would pass. Telling the two apart needs the
// type checker, which costs this guard more than that case is worth.
func TestEveryMeasurementIsAFunctionOfItsKeyAlone(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      int
	}{
		{"a function of the package", "package p\nfunc f(d int) (int, error) { return d, nil }\nfunc g() { format.Measure(nil, 1, f) }", 0},
		{"a closure", "package p\nfunc g(q int) { format.Measure(nil, 1, func(d int) (int, error) { return d + q, nil }) }", 1},
		{"a variable", "package p\nfunc f(d int) (int, error) { return d, nil }\nfunc g() { h := f\n format.Measure(nil, 1, h) }", 1},
		{"a method value", "package p\ntype s struct{ q int }\nfunc (v s) f(d int) (int, error) { return d + v.q, nil }\nfunc g(v s) { format.Measure(nil, 1, v.f) }", 1},
		{"inside package format", "package format\nfunc g(q int) { Measure(nil, 1, func(d int) (int, error) { return q, nil }) }", 1},
		{"a type argument named", "package p\nfunc f(d int) (int, error) { return d, nil }\nfunc g() { format.Measure[int, int](nil, 1, f) }", 0},
	} {
		faults, _, err := measureFaults("canary", []string{c.src})
		if err != nil || len(faults) != c.want {
			t.Fatalf("canary %q: %d faults %q (%v), want %d - the check cannot be trusted with the real code", c.name, len(faults), faults, err, c.want)
		}
	}

	var faults, reached []string
	for _, p := range packages(t) {
		var srcs []string
		for _, f := range p.files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			srcs = append(srcs, string(data))
		}
		found, calls, err := measureFaults(p.rel, srcs)
		if err != nil {
			t.Fatal(err)
		}
		faults = append(faults, found...)
		if calls > 0 {
			reached = append(reached, p.rel)
		}
	}
	for _, want := range []string{"internal/format/jpg", "internal/format/avif"} {
		if !slices.Contains(reached, want) {
			t.Fatalf("no call of format.Measure found in %s, which plans through one - this reached %v", want, reached)
		}
	}
	if len(faults) > 0 {
		t.Errorf("format.Measure is handed something other than a function of its package:\n  %s\nName a function "+
			"that takes the format's drawing and nothing else, so what is remembered is what was measured.",
			strings.Join(faults, "\n  "))
	}
}

// measureFaults parses the files of one package and returns each call of
// Measure - format.Measure, or Measure inside package format - whose third
// argument is not the name of a function declared in the package, and how many
// calls it read.
func measureFaults(rel string, srcs []string) (faults []string, calls int, err error) {
	fset := token.NewFileSet()
	var files []*ast.File
	funcs := map[string]bool{}
	for i, src := range srcs {
		f, err := parser.ParseFile(fset, fmt.Sprintf("%s#%d", rel, i), src, 0)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, f)
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				funcs[fn.Name.Name] = true
			}
		}
	}
	for _, f := range files {
		inFormat := f.Name.Name == "format"
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 || !isMeasure(call.Fun, inFormat) {
				return true
			}
			calls++
			if id, ok := call.Args[2].(*ast.Ident); !ok || !funcs[id.Name] {
				faults = append(faults, fmt.Sprintf("%s: Measure at %s", rel, fset.Position(call.Pos())))
			}
			return true
		})
	}
	return faults, calls, nil
}

// isMeasure says fun names format.Measure, written from outside the package
// or from inside it.
func isMeasure(fun ast.Expr, inFormat bool) bool {
	if idx, ok := fun.(*ast.IndexListExpr); ok {
		fun = idx.X
	}
	if idx, ok := fun.(*ast.IndexExpr); ok {
		fun = idx.X
	}
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		pkg, ok := f.X.(*ast.Ident)
		return ok && pkg.Name == "format" && f.Sel.Name == "Measure"
	case *ast.Ident:
		return inFormat && f.Name == "Measure"
	}
	return false
}
