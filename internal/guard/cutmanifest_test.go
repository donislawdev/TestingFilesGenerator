package guard

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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
// The shapes a cut takes (docs/TRUNCATE-2026-11-11.md sections 2 and 7). A
// GIF of 1 MiB is nearly all padding after its picture, so 90% missing still
// leaves the whole picture. A PNG splits its padding around the picture for a
// cut, so the cut takes the share asked for. An icon keeps its padding BEFORE
// its picture, so a cut there takes all of it. A text is its words, so the cut
// falls in them. Two cuts in a row end the file where the second one does, and
// the cut is said of that one. A damage that cuts nothing has no cut to report.
//
// Canary: the check below must refuse a cut that names the wrong role, and
// one that claims content it does not have.
func TestACutFileSaysWhereTheCutFellAndWhatIsLeft(t *testing.T) {
	cases := []cutCase{
		{name: "behind", format: "gif", size: 1 << 20, missing: "90", in: "padding", kept: "all", note: "after the content"},
		{name: "picture", format: "png", size: 1 << 20, missing: "90", in: "content", kept: "share"},
		// At half: the picture of 256x256 takes about 260 KB at the end of the
		// file, and a tenth of a megabyte cut away falls inside it.
		{name: "icon", format: "ico", size: 1 << 20, missing: "50", in: "padding", kept: "none", note: "before the content"},
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
		cut("picture", "png", 1<<20, "90"), cut("icon", "ico", 1<<20, "50"), cut("text", "txt", 1000, "50"),
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
