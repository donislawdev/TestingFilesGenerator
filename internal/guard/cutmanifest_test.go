package guard

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// cutCase is one file cut short and what its manifest has to say about it.
type cutCase struct {
	name    string
	format  string
	size    int64
	missing string
	// in is the role of the first byte cut away.
	in string
	// kept says how much of the content is left: "all", "none", "some" (a
	// text, all of it on disk) or "share" (the share missing leaves).
	kept string
	// note is the word that tells the two notes of a cut in the padding
	// apart, empty when the cut gets none.
	note string
	// cuts is how many truncates the chain holds, one when not said. The
	// last of them keeps the least and carries the cut.
	cuts int
}

// A file cut short says in its manifest where the cut fell and how much of
// the content is left, and gets a note when the cut missed the content.
//
// The shapes a cut takes (docs/TRUNCATE-2026-11-11.md sections 2, 7 and 8).
// A PDF of 1 MiB is nearly all padding after its pages, so 90% missing still
// leaves every page. A PNG splits its padding around the picture
// for a cut, so the cut takes the share asked for, and so does a tar.gz. A
// text is its words, so the cut falls in them. Two cuts in a row end the file where the second one does, and
// the cut is said of that one. A damage that cuts nothing has no cut to report.
//
// Canary: the check below must refuse a cut that names the wrong role, and
// one that claims content it does not have.
func TestACutFileSaysWhereTheCutFellAndWhatIsLeft(t *testing.T) {
	cases := []cutCase{
		// A GIF was this case until it split its padding for the cut.
		{name: "behind", format: "pdf", size: 1 << 20, missing: "90", in: "padding", kept: "all", note: "after the content"},
		{name: "picture", format: "png", size: 1 << 20, missing: "90", in: "content", kept: "share"},
		// 4 KiB on disk is 40 KiB whole. It was the case of a cut in the padding
		// in front of the content, with an icon and a JPEG before it, until it
		// split its padding for the cut.
		{name: "front", format: "targz", size: 4 << 10, missing: "90", in: "content", kept: "share"},
		{name: "text", format: "txt", size: 1000, missing: "50", in: "content", kept: "some"},
		{name: "twice", format: "txt", size: 1000, missing: "50", in: "content", kept: "some", cuts: 2},
	}
	targets := []engine.Target{{ID: "untouched", Format: "txt", Sizes: engine.Uniform(1, 1000), Label: true,
		Damage: damage.Chain{{ID: "zero-head"}}}}
	for _, c := range cases {
		var chain damage.Chain
		for range max(1, c.cuts) {
			chain = append(chain, damage.Spec{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: c.missing}})
		}
		targets = append(targets, engine.Target{ID: c.name, Format: c.format, Sizes: engine.Uniform(1, c.size), Label: true,
			Damage: chain})
	}
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	byTarget := map[string]manifest.File{}
	for _, f := range res.Manifest.Files {
		byTarget[f.TargetID] = f
	}

	if f := byTarget["untouched"]; len(f.Damage) != 1 || f.Damage[0].Cut != nil {
		t.Errorf("a damage that cuts nothing reported a cut: %+v", f.Damage)
	}
	for _, c := range cases {
		f, ok := byTarget[c.name]
		if !ok || f.Failed {
			t.Errorf("%s: no file was written (%s)", c.name, f.Error)
			continue
		}
		for _, problem := range cutProblems(c, f) {
			t.Errorf("%s: %s", c.name, problem)
		}
	}

	// Canary: the GIF's entry, told the cut fell in its content, and the
	// text's, told none of its content is left.
	behind := byTarget["behind"]
	if len(behind.Damage) == 1 && behind.Damage[0].Cut != nil {
		wrong := cases[0]
		wrong.in = "content"
		if len(cutProblems(wrong, behind)) == 0 {
			t.Errorf("canary: a cut in the padding passed for a cut in the content")
		}
	}
	text := cases[3]
	text.kept = "none"
	if len(cutProblems(text, byTarget["text"])) == 0 {
		t.Errorf("canary: a text that kept half its words passed for one that kept none")
	}
}

// cutProblems is everything about f's cut that is not what c says it is.
func cutProblems(c cutCase, f manifest.File) []string {
	last := max(1, c.cuts) - 1
	if len(f.Damage) != last+1 || f.Damage[last].Cut == nil {
		return []string{fmt.Sprintf("the last damage entry has no cut: %+v", f.Damage)}
	}
	for _, d := range f.Damage[:last] {
		if d.Cut != nil {
			return []string{fmt.Sprintf("a cut that does not end the file reported one: %+v", f.Damage)}
		}
	}
	cut := f.Damage[last].Cut
	var out []string
	if cut.In != c.in {
		out = append(out, fmt.Sprintf("the cut fell in %q, not in %q", cut.In, c.in))
	}
	if cut.ContentBytes == nil {
		return append(out, "the content's length is not stated")
	}
	whole, kept := *cut.ContentBytes, cut.ContentKeptBytes
	switch {
	case whole <= 0:
		out = append(out, fmt.Sprintf("the content is said to be %d B long, and every case here cuts beside content", whole))
	case c.kept == "all" && kept != whole:
		out = append(out, fmt.Sprintf("%d B of %d B of content kept, not all of it", kept, whole))
	case c.kept == "none" && kept != 0:
		out = append(out, fmt.Sprintf("%d B of content kept, not none", kept))
	case c.kept == "some" && (kept != f.Bytes || whole <= kept):
		out = append(out, fmt.Sprintf("%d B of %d B of content kept, where a %d B file of nothing but content keeps all of it and loses the rest", kept, whole, f.Bytes))
	case c.kept == "share":
		missing, _ := strconv.ParseInt(c.missing, 10, 64)
		if problem := shareProblem(missing, f); problem != "" {
			out = append(out, problem)
		}
	}
	var said []string
	for _, n := range f.Notes {
		if n.Code == "cut_in_padding" {
			said = append(said, n.Detail)
		}
	}
	switch {
	case c.note == "" && len(said) > 0:
		out = append(out, fmt.Sprintf("a note about the padding where the cut fell in %s: %q", c.in, said))
	case c.note != "" && (len(said) != 1 || !strings.Contains(said[0], c.note)):
		out = append(out, fmt.Sprintf("the note about the padding says %q, which should say %q", said, c.note))
	}
	return out
}

// The record a run that cuts its files is told beforehand is never shorter
// than the record it writes, and not much longer.
//
// The cut is known only once a file is written, so the forecast counts the
// longest one a file can be given (engine forecastCut). A forecast under the
// record lets the free space check pass a disk the record does not fit on,
// and one far over refuses a disk that had room.
//
// What it reaches, asserted rather than assumed: the record carries the note
// a cut in the padding gets, the longest thing the forecast allows for.
func TestTheRecordOfACutRunIsNeverLongerThanItsForecast(t *testing.T) {
	cut := func(id, format string, size int64, missing string) engine.Target {
		return engine.Target{ID: id, Format: format, Sizes: engine.Uniform(2, size), Label: true,
			Damage: damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: missing}}}}
	}
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test"}
	planned, err := engine.Plan([]engine.Target{
		cut("picture", "png", 1<<20, "90"), cut("behind", "pdf", 1<<20, "90"), cut("text", "txt", 1000, "50"),
	}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	forecast, err := engine.PredictRecord(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("forecasting: %v", err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	rec, err := engine.SaveRecord(res, opt)
	if err != nil {
		t.Fatalf("saving the record: %v", err)
	}
	body, err := os.ReadFile(rec.Manifest)
	if err != nil {
		t.Fatalf("reading the record: %v", err)
	}
	if !strings.Contains(string(body), `"cut_in_padding"`) {
		t.Fatalf("no file of the run was cut in its padding, so the longest entry was never written")
	}
	written := int64(len(body))
	if written > forecast.Manifest {
		t.Errorf("the record is %d B and the run was told %d B", written, forecast.Manifest)
	}
	// Each of the six files may be forecast a role word, two numbers and a
	// note it does not get - a few hundred bytes, not kilobytes.
	if slack := forecast.Manifest - written; slack > 6*256 {
		t.Errorf("the forecast is %d B over a record of %d B", slack, written)
	}
}

// A file cut at the smallest size it is cut at says how long its content was,
// as a larger one does: the share a cut took is measured against that length.
//
// At its smallest a file has the least padding, too little for the layout a
// cut gets in a larger one. A Word document's padding part in front of the
// others costs more than the padding there is, so the package is written as
// one nothing cuts, and its content is marked in a branch the share guard's
// larger files never reach (docs/REVIEW-239-2026-11-13.md). What it reaches,
// asserted rather than assumed: the ZIP packages, cut at their smallest, have
// no padding in front of their content and are cut inside it.
//
// Every format that takes the share (notYetCutAtTheShare lists the rest) is cut
// by half at its smallest. Where the cut falls in the content, its length has
// to be stated and to be how far the whole file's map lets the content run -
// up to the region behind it, not the length the generator marked it with.
//
// Canary: a cut whose content length is missing, and one whose length is a
// byte short, must not pass.
func TestACutAtTheSmallestSizeSaysHowLongTheContentWas(t *testing.T) {
	chain := damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "50"}}}
	var targets []engine.Target
	for _, f := range format.All() {
		if _, listed := notYetCutAtTheShare[f.ID]; listed {
			continue
		}
		size := engine.SmallestDamaged(f, chain, format.Request{Label: true})
		targets = append(targets, engine.Target{ID: f.ID, Format: f.ID, Sizes: engine.Uniform(1, size), Label: true, Damage: chain})
	}
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	plannedOf := map[string]engine.PlannedFile{}
	for _, f := range planned {
		plannedOf[f.Target.ID] = f
	}
	inContent := 0
	for _, m := range res.Manifest.Files {
		if m.Failed || len(m.Damage) != 1 || m.Damage[0].Cut == nil {
			t.Errorf("%s: no file with one cut was written (%s, %+v)", m.TargetID, m.Error, m.Damage)
			continue
		}
		cut := *m.Damage[0].Cut
		whole := wholeMap(t, plannedOf[m.TargetID])
		if _, zipped := endClosesTheFile[m.TargetID]; zipped && (paddingInFront(whole) || cut.In != string(format.RoleContent)) {
			t.Errorf("%s: at %d B the cut fell in the %s with padding in front of the content: %v - the layout too short for a padding part in front is not reached",
				m.TargetID, m.Bytes, cut.In, paddingInFront(whole))
		}
		if cut.In != string(format.RoleContent) {
			continue
		}
		inContent++
		if problem := contentLengthProblem(cut, whole); problem != "" {
			t.Errorf("%s: cut at %d B: %s", m.TargetID, m.Bytes, problem)
		}
	}
	if inContent == 0 {
		t.Errorf("no cut fell in the content, so no length was asked for")
	}

	// Canaries.
	regions := []format.Region{{Role: format.RoleContent, At: 0, Length: 10}, {Role: format.RolePadding, At: 10, Length: 5}}
	if contentLengthProblem(manifest.Cut{In: "content"}, regions) == "" {
		t.Errorf("canary: a cut that does not state the content's length passed")
	}
	short := int64(9)
	if contentLengthProblem(manifest.Cut{In: "content", ContentBytes: &short}, regions) == "" {
		t.Errorf("canary: content said to be a byte shorter than the map runs passed")
	}
}

// contentLengthProblem says how the content length cut states is not how far
// the content runs in the whole file's map, or nothing when it is. Each
// content region runs to the start of the region behind it, or to the end of
// the file - the length it was marked with is the claim being checked.
func contentLengthProblem(cut manifest.Cut, whole []format.Region) string {
	if cut.ContentBytes == nil {
		return "the length of the content is not stated"
	}
	var runs int64
	for i, r := range whole {
		if r.Role != format.RoleContent {
			continue
		}
		end := r.At + r.Length
		if i+1 < len(whole) {
			end = whole[i+1].At
		}
		runs += end - r.At
	}
	if *cut.ContentBytes != runs {
		return fmt.Sprintf("the content is said to be %d B long, where the whole file's map runs it for %d B", *cut.ContentBytes, runs)
	}
	return ""
}

// paddingInFront is whether padding of any length stands in front of the
// first content region of a map.
func paddingInFront(regions []format.Region) bool {
	for _, r := range regions {
		switch {
		case r.Role == format.RoleContent:
			return false
		case r.Role == format.RolePadding && r.Length > 0:
			return true
		}
	}
	return false
}
