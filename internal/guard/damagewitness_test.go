package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
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

// witnessCase is one damaged file and the healthy file of the same format,
// settings and size beside it.
type witnessCase struct {
	format string
	point  damagePoint
	size   int64
	// setting is the one format setting moved from its default, empty for a
	// file at its defaults.
	setting map[string]string
	kind    damage.Kind
	good    string
	broken  string
}

// policyRefusals is, for a pair the manifest calls policy dependent, every
// reason the structural check gives when it refuses one of these files - read
// and judged by a person as the shape this tool writes, or as a rule the
// format states and readers skip, rather than a rule a reader has to enforce.
//
// Written out rather than allowed wholesale, because the list is the second
// direction of the witness rule. A refusal for a reason NOT here may be the
// format's own rule broken, and then the pair should be certain - so it turns
// the guard red until somebody reads it and decides. A reason here that no
// file gives any more turns it red too, because a list nothing hits is a list
// nobody can trust (pitfall 15). Read from the measurements of 2026-11-10
// (docs/DAMAGE-ETAP-2-2026-11-09.md sections 12.1 to 12.3).
var policyRefusals = map[[2]string][]string{
	// The shape this tool writes: the last line ends with a line break.
	{damage.Truncate, "csv"}:  {"the file does not end with"},
	{damage.Truncate, "log"}:  {"the file does not end with a newline"},
	{damage.Truncate, "toml"}: {"the document does not end with a newline"},
	{damage.Truncate, "yaml"}: {"the document does not end with a newline"},
	{damage.Truncate, "html"}: {"was ordered and the page is not", "the document does not end with a closing"},
	{damage.Truncate, "md"}:   {"so one block is never closed"},
	{damage.Truncate, "eml"}: {"the message has no blank line after its header lines", "layout=newsletter and the plain text",
		// A multipart message cut inside its body has no closing boundary,
		// which RFC 2046 asks for and readers skip: Python's email reads such
		// a cut multipart/alternative with its text part whole and notes a
		// CloseBoundaryNotFoundDefect (measured 2026-11-12). Until the label
		// of a damaged file named its damage, the cut at this size fell
		// between a CR and its LF, and that reason came first.
		"does not open with its boundary or has no closing one"},
	{damage.Truncate, "txt"}: {},
	// Bytes past the format's own end, which its rules forbid and readers skip.
	{damage.AppendJunk, "png"}:   {"declares"},
	{damage.AppendJunk, "gif"}:   {"the trailer is at"},
	{damage.AppendJunk, "jpg"}:   {"the file does not end with the end of image marker"},
	{damage.AppendJunk, "pdf"}:   {"the end of file marker is not near the end"},
	{damage.AppendJunk, "rtf"}:   {"outside 7 bit text"},
	{damage.AppendJunk, "zip"}:   {"the comment declares"},
	{damage.AppendJunk, "docx"}:  {"the comment declares"},
	{damage.AppendJunk, "xlsx"}:  {"the comment declares"},
	{damage.AppendJunk, "pptx"}:  {"the comment declares"},
	{damage.AppendJunk, "targz"}: {"of trailer should follow the stream"},
	{damage.AppendJunk, "eml"}: {"a part is not shaped the way this tool writes one", "RFC 5322 asks for",
		"after its closing boundary", "is not a line of base64",
		// A rule the junk breaks only when its random bytes hold a CR or an
		// LF on its own - about four seeds in ten, so not every file.
		"a carriage return or a line feed on its own"},
	// UTF-16 in the next two: the junk breaks the encoding only when its
	// random bytes form a lone surrogate, which is a matter of the seed.
	{damage.AppendJunk, "txt"}: {"that are not printable ASCII", "does not decode as utf-16"},
	{damage.AppendJunk, "md"}:  {"that are not printable ASCII", "does not decode as utf-16"},
	// Bytes the format allows: nothing points at them.
	// A reader of lines meets the damage on one line and carries on with the
	// next, measured 2026-11-11 (docs/OTLP-2026-11-10.md section 12.3). The
	// checker names the line the damage is on.
	{damage.ZeroHead, "log"}: {"line 1 is not a whole entry in any shape this tool writes", "the file does not end with a newline"},
	// The head the damage zeroes also held the byte order mark or the empty
	// envelope the pitfalls put first, and the checker names that before it
	// reaches line 1.
	{damage.ZeroHead, "otlp"}: {"line 1 is not JSON", "the file does not end with a newline",
		"does not start with a byte order mark", "does not open with the line"},
	{damage.Truncate, "otlp"}:   {"the file does not end with a newline"},
	{damage.AppendJunk, "otlp"}: {"not valid UTF-8"},
	{damage.AppendJunk, "ico"}:  {},
	{damage.AppendJunk, "tiff"}: {},
}

// TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat is the witness
// rule as a guard rather than a measurement taken once.
//
// Every format meets every damage, and the manifest says one of two things
// about the result (owner's call, 2026-11-10). Where the damage breaks the
// format's own rules it says reject with certainty, and that is only honest
// if a judge refuses the file - so a damaged file the structural check
// accepts is red. Where the file's fate depends on the reader it says so, and
// then the check has to accept the file or refuse it for one of the reasons
// in policyRefusals - a new reason is red until a person decides which side
// of the line it stands on.
//
// Two sweeps, each file from the real engine with its label, as both surfaces
// write by default, and each judged beside a healthy file of the same format,
// settings and size. The first takes every damage at its defaults and with
// each parameter at both ends, at the smallest size the pair can be given and
// at 20 kB. The second takes every setting of every format one at a time - a
// choice, or the other side of a flag - with each damage at its defaults at
// 20 kB, because whether a damage is noticed moves with the settings as well
// as the size: a cut line of one log shape is still a line and of another is
// not, and random bytes are text in UTF-16 and not in UTF-8 (section 12.2).
//
// Red as well when the checker falls over instead of answering - a crash is a
// fault in the checker, never a witness - and for a format the checker does
// not know: nobody has decided what a damage does there.
//
// A setting the format refuses on its own (a pair that needs another one
// beside it) is skipped, and so are numbers, which are dimensions and counts
// rather than shapes. Chains of several damages are not swept: the manifest
// is certain about a chain when one member is, by the reasoning in
// damage.Chain.Certain, and that is the limit written down in section 12.9.
func TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat(t *testing.T) {
	dir := t.TempDir()

	var points, defaults []damagePoint
	for _, d := range damage.All() {
		points = append(points, damagePointsOf(t, d)...)
		defaults = append(defaults, damagePoint{id: d.ID, values: damage.Values{}, label: d.ID + " at its defaults"})
	}
	if len(points) == 0 {
		t.Fatal("the damage registry is empty, so this guard would pass without judging anything")
	}
	told := strictTakesSettings(t)

	var cases []witnessCase
	formats := format.All()
	swept := 0
	for _, f := range formats {
		if !oracle.StrictKnows(f.ID) {
			t.Errorf("%s has no structural check, so nobody can say what a damage does there.\n"+
				"Reason: every damage reaches every format, so a format nobody can judge would be\n"+
				"handed damaged files the manifest says something about that nothing measured.\n"+
				"Give it a check in internal/oracle/strict.py before it is registered.", f.ID)
			continue
		}
		atDefaults := witnessCasesOf(t, dir, f, points)
		bySetting := settingCasesOf(t, dir, f, defaults)
		swept += len(bySetting)
		cases = append(cases, atDefaults...)
		cases = append(cases, bySetting...)
	}

	reqs := make([]oracle.StrictRequest, 0, 2*len(cases))
	for _, c := range cases {
		settings := settingsTold(told, c.format, c.setting)
		reqs = append(reqs,
			oracle.StrictRequest{Format: c.format, Path: c.good, Settings: settings},
			oracle.StrictRequest{Format: c.format, Path: c.broken, Settings: settings})
	}
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatal(err)
	}

	j := witnessJudge{t: t, hit: map[[2]string]map[string]bool{}}
	for i, c := range cases {
		j.judge(c, got[2*i], got[2*i+1])
	}
	j.staleReasons()

	if want := len(formats) * len(points); len(cases)-swept < want {
		t.Errorf("%d pair(s) judged at the defaults, and %d formats by %d damage points need at least %d",
			len(cases)-swept, len(formats), len(points), want)
	}
	// Both answers reached, so neither half of this guard is green by never
	// being asked.
	if j.certain == 0 || j.depends == 0 {
		t.Errorf("%d pair(s) judged certain and %d policy dependent - a guard that never meets one of the two proves nothing about it",
			j.certain, j.depends)
	}
	if swept == 0 {
		t.Error("no format setting was swept, so the second half of this guard judged nothing")
	}
	t.Logf("%d certain pair(s) refused and %d policy dependent pair(s) judged, %d of them with a setting moved, across %d formats",
		j.certain, j.depends, swept, len(formats))
}

// witnessJudge holds what the guard above counts while it reads the answers.
type witnessJudge struct {
	t       *testing.T
	certain int
	depends int
	// hit is every policy refusal reason a file gave, by pair.
	hit map[[2]string]map[string]bool
}

func (j *witnessJudge) judge(c witnessCase, good, broken oracle.StrictJudgement) {
	t := j.t
	where := fmt.Sprintf("%s at %d B with %s", c.format, c.size, c.point.label)
	for name, value := range c.setting {
		where += fmt.Sprintf(" and %s=%s", name, value)
	}
	if good.Verdict != oracle.StrictOK {
		// The control. A checker that refused everything would witness every
		// damage, so a pair whose healthy file is refused says nothing.
		t.Errorf("%s: the healthy file was answered %s %q, so this pair says nothing about the damage",
			where, good.Verdict, good.Reason)
		return
	}
	if broken.Verdict != oracle.StrictOK && broken.Verdict != oracle.StrictFail {
		t.Errorf("%s: the checker answered %s %q about the damaged file.\n"+
			"Reason: that is a fault in the checker, not a refusal of the file. Counted as\n"+
			"a witness, a checker that falls over on every damaged file would witness\n"+
			"every damage.", where, broken.Verdict, broken.Reason)
		return
	}
	certain, _, err := damage.Chain{{ID: c.point.id, Values: c.point.values}}.Certain(c.kind)
	if err != nil {
		t.Errorf("%s: %v", where, err)
		return
	}
	if certain {
		j.certain++
		if broken.Verdict == oracle.StrictOK {
			t.Errorf("%s: the damaged file was ACCEPTED (%s).\n"+
				"Reason: its manifest says reject with certainty, and nothing refuses it. Either the\n"+
				"structural check is missing a rule of the format, or the damage leaves this file's\n"+
				"fate to the reader and belongs in the damage's policy cases.", where, broken.Reason)
		}
		return
	}
	j.depends++
	if broken.Verdict == oracle.StrictOK {
		return
	}
	pair := [2]string{c.point.id, c.format}
	for _, reason := range policyRefusals[pair] {
		if strings.Contains(broken.Reason, reason) {
			if j.hit[pair] == nil {
				j.hit[pair] = map[string]bool{}
			}
			j.hit[pair][reason] = true
			return
		}
	}
	t.Errorf("%s: the manifest says the outcome depends on the reader, and the structural check\n"+
		"refused the file for a reason nobody has read: %q.\n"+
		"Reason: if the damage breaks a rule of the format here, the pair is certain and the\n"+
		"manifest should say reject - take it out of the damage's policy cases. If the reason is\n"+
		"the shape this tool writes, or bytes past the format's end, add it to policyRefusals.",
		where, broken.Reason)
}

// staleReasons turns a reason no file gave into a failure, and a pair listed
// here that the registry no longer calls policy dependent.
func (j *witnessJudge) staleReasons() {
	t := j.t
	for pair, reasons := range policyRefusals {
		d, err := damage.Get(pair[0])
		if err != nil {
			t.Errorf("policyRefusals lists %s, which is not a damage", pair[0])
			continue
		}
		if !slices.ContainsFunc(d.Policy, func(c damage.PolicyCase) bool { return c.Format == pair[1] }) {
			t.Errorf("policyRefusals lists %s on %s, and %s has no policy case for %s",
				pair[0], pair[1], pair[0], pair[1])
		}
		for _, reason := range reasons {
			if !j.hit[pair][reason] {
				t.Errorf("policyRefusals expects %s on %s to be refused with %q, and no file was.\n"+
					"Reason: a reason nothing gives any more is a list going stale - take it out.",
					pair[0], pair[1], reason)
			}
		}
	}
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

// witnessCasesOf writes, for one format at its defaults, a healthy and a
// damaged file of each damage point at the two sizes, all in one run of the
// engine.
func witnessCasesOf(t *testing.T, dir string, f format.Descriptor, points []damagePoint) []witnessCase {
	t.Helper()
	var targets []engine.Target
	var cases []witnessCase
	sized := map[int64]bool{}
	for n, p := range points {
		chain := damage.Chain{{ID: p.id, Values: p.values}}
		// With the label, because that is how both surfaces write by default.
		smallest := engine.SmallestDamaged(f, chain, format.Request{Label: true})
		for _, size := range witnessSizes(smallest) {
			id := fmt.Sprintf("p%d-%d", n, size)
			targets = append(targets, engine.Target{ID: id, Format: f.ID, Sizes: engine.Uniform(1, size), Damage: chain, Label: true})
			// A file cut short can be smaller than any healthy file of its
			// format, so the control is the nearest size the format writes.
			good := healthySizeFrom(t, f, nil, size)
			cases = append(cases, witnessCase{format: f.ID, point: p, size: size, kind: damage.KindOf(f, nil),
				good: fmt.Sprintf("good-%d", good), broken: id})
			if !sized[good] {
				sized[good] = true
				targets = append(targets, engine.Target{ID: fmt.Sprintf("good-%d", good), Format: f.ID, Sizes: engine.Uniform(1, good), Label: true})
			}
		}
	}
	paths := writeWitnessFiles(t, filepath.Join(dir, f.ID), f.ID, targets)
	for i := range cases {
		c := &cases[i]
		c.good, c.broken = paths[c.good], paths[c.broken]
		if c.good == "" || c.broken == "" {
			t.Fatalf("%s: the run wrote no file for %s at %d B", f.ID, c.point.label, c.size)
		}
	}
	return cases
}

// settingCasesOf writes, for one format, every setting moved from its default
// on its own - with a healthy file and one damaged by each damage at its
// defaults, at 20 kB or the smallest size the setting allows.
func settingCasesOf(t *testing.T, dir string, f format.Descriptor, defaults []damagePoint) []witnessCase {
	t.Helper()
	var targets []engine.Target
	var cases []witnessCase
	for n, setting := range settingVariants(f) {
		healthy := engine.Target{ID: fmt.Sprintf("s%d-good", n), Format: f.ID, Properties: setting, Label: true}
		size := max(witnessSize, f.SmallestAccepted(format.Request{Label: true, Properties: setting}))
		healthy.Sizes = engine.Uniform(1, size)
		// Planned alone first: a setting the format refuses without another
		// beside it is skipped here rather than taking the whole run down.
		if _, err := engine.Plan([]engine.Target{healthy}, engine.Options{OutDir: dir, Seed: 5}); err != nil {
			continue
		}
		targets = append(targets, healthy)
		for m, p := range defaults {
			chain := damage.Chain{{ID: p.id, Values: p.values}}
			at := max(size, engine.SmallestDamaged(f, chain, format.Request{Label: true, Properties: setting}))
			id := fmt.Sprintf("s%d-d%d", n, m)
			good := healthy.ID
			if at != size {
				good = id + "-good"
				targets = append(targets, engine.Target{ID: good, Format: f.ID, Properties: setting,
					Sizes: engine.Uniform(1, healthySizeFrom(t, f, setting, at)), Label: true})
			}
			targets = append(targets, engine.Target{ID: id, Format: f.ID, Properties: setting, Sizes: engine.Uniform(1, at), Damage: chain, Label: true})
			cases = append(cases, witnessCase{format: f.ID, point: p, size: at, setting: setting,
				kind: damage.KindOf(f, setting), good: good, broken: id})
		}
	}
	if len(targets) == 0 {
		return nil
	}
	paths := writeWitnessFiles(t, filepath.Join(dir, f.ID+"-settings"), f.ID, targets)
	for i := range cases {
		c := &cases[i]
		c.good, c.broken = paths[c.good], paths[c.broken]
		if c.good == "" || c.broken == "" {
			t.Fatalf("%s: the run wrote no file for %s with %v", f.ID, c.point.label, c.setting)
		}
	}
	return cases
}

// settingVariants is every setting of a format moved from its default on its
// own: each other choice of a closed set, and the other side of a flag.
func settingVariants(f format.Descriptor) []map[string]string {
	var out []map[string]string
	for _, p := range f.Properties {
		var values []string
		switch p.Kind {
		case format.PropertyChoice:
			values = p.Choices
		case format.PropertyBool:
			values = []string{"true", "false"}
		case format.PropertyInt, format.PropertySize, format.PropertyDuration, format.PropertyText:
			// A number, a size, a length of time and free text are how much
			// of the file there is, not which shape it takes. Not swept.
		}
		for _, v := range values {
			if v != p.Default {
				out = append(out, map[string]string{p.Name: v})
			}
		}
	}
	return out
}

// writeWitnessFiles plans and writes the targets in one run, and answers the
// path of each target's file by its id.
func writeWitnessFiles(t *testing.T, out, formatID string, targets []engine.Target) map[string]string {
	t.Helper()
	opt := engine.Options{OutDir: out, Seed: 5, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("%s: planning the witness files: %v", formatID, err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("%s: writing the witness files: %v", formatID, err)
	}
	paths := map[string]string{}
	for _, m := range res.Manifest.Files {
		if m.Failed {
			t.Fatalf("%s: target %s failed: %s", formatID, m.TargetID, m.Error)
		}
		path := filepath.Join(out, filepath.FromSlash(m.Path))
		if !strings.HasPrefix(path, out) {
			t.Fatalf("%s: target %s was written outside the run: %s", formatID, m.TargetID, path)
		}
		paths[m.TargetID] = path
	}
	return paths
}

// healthySizeFrom is the first size at or above from that the format writes
// as a healthy file, jumping along the sizes its refusals name.
func healthySizeFrom(t *testing.T, f format.Descriptor, setting map[string]string, from int64) int64 {
	t.Helper()
	at := from
	for round := 0; round < 64; round++ {
		_, err := f.Generator.Plan(format.Request{Bytes: at, Label: true, Properties: setting})
		if err == nil {
			return at
		}
		var below *format.BelowMinimumError
		if !errors.As(err, &below) || below.Minimum <= at {
			t.Fatalf("%s writes no healthy file at or above %d B: %v", f.ID, from, err)
		}
		at = below.Minimum
	}
	t.Fatalf("%s kept naming larger sizes from %d B", f.ID, from)
	return 0
}

// witnessSizes is the smallest size a pair can be given, and the comfortable
// one when that is larger.
func witnessSizes(smallest int64) []int64 {
	if smallest >= witnessSize {
		return []int64{smallest}
	}
	return []int64{smallest, witnessSize}
}

// strictTakesSettings is the formats whose structural check is told the
// settings of the file, read from the checker itself rather than copied, so a
// format that starts taking them is told them here the same day.
func strictTakesSettings(t *testing.T) map[string]bool {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "oracle", "strict.py"))
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^TAKES_SETTINGS = \{([^}]*)\}`).FindSubmatch(source)
	if line == nil {
		t.Fatal("strict.py no longer declares TAKES_SETTINGS on one line, so this guard cannot tell which checks take settings")
	}
	out := map[string]bool{}
	for _, name := range regexp.MustCompile(`"([a-z0-9]+)"`).FindAllSubmatch(line[1], -1) {
		out[string(name[1])] = true
	}
	if !out["csv"] {
		t.Fatalf("read %v as the checks that take settings, and csv - which always has - is not among them", out)
	}
	return out
}

// settingsTold is the setting the checker is told about a file, for a check
// that takes them. The attachments of a message are not told, the way the
// mail guards leave them out (emlSettings).
func settingsTold(told map[string]bool, formatID string, setting map[string]string) map[string]string {
	if !told[formatID] || len(setting) == 0 {
		return nil
	}
	out := map[string]string{}
	for name, value := range setting {
		if formatID == "eml" && (name == eml.AttachmentFormat || name == eml.AttachmentSize) {
			continue
		}
		out[name] = value
	}
	return out
}
