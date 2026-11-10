package guard

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// witnessSize is the comfortable size every pair is also judged at, beside the
// smallest one it can be given. Measured 2026-11-09: the set of witnesses was
// the same at 20 kB and at 1 MiB in every format, while the smallest size is
// where the answer moved (docs/DAMAGE-ETAP-2-2026-11-09.md section 2.1).
const witnessSize = 20 << 10

// damagePoint is one damage with one set of values.
type damagePoint struct {
	id     string
	values damage.Values
	label  string
}

// witnessCase is one damaged file and the healthy file of the same format
// and size beside it.
type witnessCase struct {
	format string
	point  damagePoint
	size   int64
	good   string
	broken string
}

// TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat is the witness
// rule as a guard rather than a measurement taken once.
//
// A damage is offered for a format only when a judge refuses the result, so a
// test written against the file can tell an upload validator that let it
// through. Until 2026-11-09 that was a sweep from September, and the offer
// covered every format, including one added the day after the sweep. Now
// every format in the registry meets every damage in the registry, with its
// default values and each parameter at both ends, at the smallest size the
// pair can be given and at 20 kB. Each file comes from the real engine with
// its label, as both surfaces write by default, and the structural checker
// judges it beside a healthy file of the same format and size.
//
// Red when a damaged file is accepted, and red when the checker falls over
// instead of answering: a crash is a fault in the checker, never a witness. A
// format the checker does not know is red too. Nobody has decided whether a
// damage can be offered there, and the offer must not reach it by default.
//
// Measured at the default settings of each format. A format whose settings
// change the answer (the shape of a log line, the separator of a table) is
// judged here only as it comes by default, and that limit is written down
// in docs/DAMAGE-ETAP-2-2026-11-09.md section 2.2.
func TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat(t *testing.T) {
	dir := t.TempDir()

	var points []damagePoint
	for _, d := range damage.All() {
		points = append(points, damagePointsOf(t, d)...)
	}
	if len(points) == 0 {
		t.Fatal("the damage registry is empty, so this guard would pass without judging anything")
	}

	var cases []witnessCase
	formats := format.All()
	for _, f := range formats {
		if !oracle.StrictKnows(f.ID) {
			t.Errorf("%s has no structural check, so no damage has a witness there.\n"+
				"Reason: the offer of a damage reaches every format by default, so a format\n"+
				"nobody can judge would be handed damaged files nothing refuses. Give it a\n"+
				"check in internal/oracle/strict.py before it is registered.", f.ID)
			continue
		}
		cases = append(cases, witnessCasesOf(t, dir, f, points)...)
	}

	reqs := make([]oracle.StrictRequest, 0, 2*len(cases))
	for _, c := range cases {
		reqs = append(reqs,
			oracle.StrictRequest{Format: c.format, Path: c.good},
			oracle.StrictRequest{Format: c.format, Path: c.broken})
	}
	got, available, err := oracle.StrictBatch(reqs)
	if !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatal(err)
	}

	witnessed := 0
	for i, c := range cases {
		good, broken := got[2*i], got[2*i+1]
		where := fmt.Sprintf("%s at %d B with %s", c.format, c.size, c.point.label)
		if good.Verdict != oracle.StrictOK {
			// The control. A checker that refused everything would witness
			// every damage, so a pair whose healthy file is refused says
			// nothing about the damage.
			t.Errorf("%s: the healthy file was answered %s %q, so this pair says nothing about the damage",
				where, good.Verdict, good.Reason)
			continue
		}
		switch broken.Verdict {
		case oracle.StrictFail:
			witnessed++
		case oracle.StrictOK:
			t.Errorf("%s: the damaged file was ACCEPTED (%s).\n"+
				"Reason: a damaged file nothing refuses is one no test can be written against,\n"+
				"and its manifest calls it expected: reject. Either the damage is not offered\n"+
				"for this format, or the structural check is missing a rule of the format.",
				where, broken.Reason)
		default:
			t.Errorf("%s: the checker answered %s %q about the damaged file.\n"+
				"Reason: that is a fault in the checker, not a refusal of the file. Counted as\n"+
				"a witness, a checker that falls over on every damaged file would witness\n"+
				"every damage.", where, broken.Verdict, broken.Reason)
		}
	}
	if want := len(formats) * len(points); len(cases) < want {
		t.Errorf("%d pair(s) judged, and %d formats by %d damage points need at least %d",
			len(cases), len(formats), len(points), want)
	}
	t.Logf("%d damaged file(s) refused, across %d formats and %d damage point(s)",
		witnessed, len(formats), len(points))
}

// damagePointsOf is one damage with its defaults, and with each parameter at
// both ends of what it accepts.
func damagePointsOf(t *testing.T, d damage.Descriptor) []damagePoint {
	t.Helper()
	points := []damagePoint{{id: d.ID, values: damage.Values{}, label: d.ID + " at its defaults"}}
	for _, p := range d.Parameters {
		var values []string
		switch p.Kind {
		case format.PropertyInt:
			values = []string{strconv.FormatInt(p.Min, 10), strconv.FormatInt(p.Max, 10)}
		case format.PropertyChoice:
			values = p.Choices
		default:
			t.Errorf("%s has a parameter %s of kind %s, which this guard does not know how to sweep.\n"+
				"Reason: a value nobody judged is a value whose files may have no witness.",
				d.ID, p.Name, p.Kind)
		}
		for _, v := range values {
			points = append(points, damagePoint{
				id:     d.ID,
				values: damage.Values{p.Name: v},
				label:  fmt.Sprintf("%s %s=%s", d.ID, p.Name, v),
			})
		}
	}
	return points
}

// witnessCasesOf writes, for one format, a healthy and a damaged file of each
// damage point at the two sizes, all in one run of the engine.
func witnessCasesOf(t *testing.T, dir string, f format.Descriptor, points []damagePoint) []witnessCase {
	t.Helper()
	// With the label, because that is how both surfaces write by default.
	smallest := f.SmallestAccepted(format.Request{Label: true})

	var targets []engine.Target
	var cases []witnessCase
	sized := map[int64]bool{}
	for n, p := range points {
		chain := damage.Chain{{ID: p.id, Values: p.values}}
		floor, _, err := chain.Floor()
		if err != nil {
			t.Fatalf("%s: %v", p.label, err)
		}
		for _, size := range witnessSizes(max(smallest, floor)) {
			id := fmt.Sprintf("p%d-%d", n, size)
			targets = append(targets, engine.Target{ID: id, Format: f.ID, Sizes: engine.Uniform(1, size), Damage: chain, Label: true})
			cases = append(cases, witnessCase{format: f.ID, point: p, size: size, broken: id})
			if !sized[size] {
				sized[size] = true
				targets = append(targets, engine.Target{ID: fmt.Sprintf("good-%d", size), Format: f.ID, Sizes: engine.Uniform(1, size), Label: true})
			}
		}
	}

	out := filepath.Join(dir, f.ID)
	opt := engine.Options{OutDir: out, Seed: 5, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("%s: planning the witness files: %v", f.ID, err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("%s: writing the witness files: %v", f.ID, err)
	}
	paths := map[string]string{}
	for _, m := range res.Manifest.Files {
		if m.Failed {
			t.Fatalf("%s: target %s failed: %s", f.ID, m.TargetID, m.Error)
		}
		paths[m.TargetID] = filepath.Join(out, filepath.FromSlash(m.Path))
	}
	for i := range cases {
		c := &cases[i]
		c.good, c.broken = paths[fmt.Sprintf("good-%d", c.size)], paths[c.broken]
		if c.good == "" || c.broken == "" || !strings.HasPrefix(c.broken, out) {
			t.Fatalf("%s: the run wrote no file for %s at %d B", f.ID, c.point.label, c.size)
		}
	}
	return cases
}

// witnessSizes is the smallest size a pair can be given, and the comfortable
// one when that is larger.
func witnessSizes(smallest int64) []int64 {
	if smallest >= witnessSize {
		return []int64{smallest}
	}
	return []int64{smallest, witnessSize}
}
