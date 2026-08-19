package guard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// Untouchable rule 3, D11: the bytes of a generated file do not change inside a
// major version. Somebody's CI asserts on a hash we produced, and a drift shows
// up there as a wall of red with nothing to explain it.
//
// Nothing was guarding that. The neighbouring file pins the STANDARD LIBRARY -
// flate, gzip, zip, png - which catches a Go upgrade moving the ground under
// us. It says nothing about our own code. And the determinism tests compare two
// runs of one binary, so they stay green when every byte of a format changes,
// because both runs changed together.
//
// So this is the guard for the half nobody was watching: a refactor of a
// generator that quietly produces different files. It is the same class as the
// canonical form of a recipe, and it takes the same remedy - a pinned value.
//
// If this goes red, that is a breaking change. It needs a major version bump,
// a changelog entry and a decision by the owner. Never edit the golden file to
// make it green. That single move turns a guard into decoration.

const generatorGoldenFile = "testdata/generator-golden.json"

// The cases are fixed on purpose: one per format, plus the switches that change
// bytes rather than just size. The seed is written here rather than defaulted,
// because a default that moved would move every hash below.
const goldenSeed = 7741

func goldenCases() map[string]engine.Target {
	return map[string]engine.Target{
		// Every format at a size comfortably above its minimum.
		"txt_4kib": {ID: "g", Format: "txt", Sizes: engine.Uniform(1, 4096), Label: true},
		"png_64kib": {ID: "g", Format: "png", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},
		"pdf_16kib": {ID: "g", Format: "pdf", Sizes: engine.Uniform(1, 16384), Label: true},
		"bmp_64kib": {ID: "g", Format: "bmp", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},

		// The one format whose picture is grown to fill the request, so the
		// dimensions are arithmetic rather than a setting. Pinned without them,
		// because that arithmetic is what a refactor would move.
		"bmp_100kib_sized_to_fit": {ID: "g", Format: "bmp", Sizes: engine.Uniform(1, 102400), Label: true},
		"gif_64kib": {ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},
		"ico_32kib": {ID: "g", Format: "ico", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "32", "height": "32"}},

		// What sits inside an icon changes every byte of it, so both answers
		// are pinned rather than only the one the default picks.
		"ico_32kib_png_inside": {ID: "g", Format: "ico", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "32", "height": "32", "embed": "png"}},
		"wav_32kib": {ID: "g", Format: "wav", Sizes: engine.Uniform(1, 32768), Label: true},
		"zip_16kib": {ID: "g", Format: "zip", Sizes: engine.Uniform(1, 16384), Label: true},
		"md_8kib":   {ID: "g", Format: "md", Sizes: engine.Uniform(1, 8192), Label: true},
		"log_8kib":  {ID: "g", Format: "log", Sizes: engine.Uniform(1, 8192), Label: true},
		"csv_8kib":  {ID: "g", Format: "csv", Sizes: engine.Uniform(1, 8192), Label: true},
		"json_8kib": {ID: "g", Format: "json", Sizes: engine.Uniform(1, 8192), Label: true},
		"xml_8kib":  {ID: "g", Format: "xml", Sizes: engine.Uniform(1, 8192), Label: true},
		"html_8kib": {ID: "g", Format: "html", Sizes: engine.Uniform(1, 8192), Label: true},
		"svg_8kib":  {ID: "g", Format: "svg", Sizes: engine.Uniform(1, 8192), Label: true},

		// XML is the only one of the three that carries the label in the file,
		// as a comment, so it is the only one where the switch moves bytes. For
		// CSV and JSON the label never reaches the content, and both positions
		// would pin the same file.
		"xml_8kib_no_label": {ID: "g", Format: "xml", Sizes: engine.Uniform(1, 8192), Label: false},

		// The label is a byte affecting switch, not a cosmetic one, so it is
		// pinned in both positions.
		"txt_4kib_no_label": {ID: "g", Format: "txt", Sizes: engine.Uniform(1, 4096), Label: false},

		// An archive holding real files of another format. This is the path
		// "contains" rewrites, and the one case where a refactor could change
		// the bytes of every archive anybody has generated.
		"zip_with_three_pdfs": {ID: "g", Format: "zip", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"entries": "3", "entry_format": "pdf", "entry_size": "8kb"}},

		// Padding above the archive comment limit moves into a stored entry.
		// That is the second stage of a padding channel with a ceiling, and it
		// has its own arithmetic to get wrong.
		"zip_past_the_comment_limit": {ID: "g", Format: "zip", Sizes: engine.Uniform(1, 262144), Label: true},

		// TAR.GZ, pinned from three sides for the same reasons as ZIP. Its size
		// does not come from measuring the structure the way ZIP's does - it
		// comes from arithmetic over the stored block framing of gzip - so a
		// drift here can be a wrong size rather than only different bytes.
		"targz_16kib": {ID: "g", Format: "targz", Sizes: engine.Uniform(1, 16384), Label: true},
		"targz_with_three_pdfs": {ID: "g", Format: "targz", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"entries": "3", "entry_format": "pdf", "entry_size": "8kb"}},

		// Above the comment limit the padding moves into a tar entry, and a tar
		// entry is aligned to 512 bytes. This is the case where the two stages
		// have to agree to the byte.
		"targz_past_the_comment_limit": {ID: "g", Format: "targz", Sizes: engine.Uniform(1, 262144), Label: true},
	}
}

// TestEveryFormatHasAGoldenValue asks whether the pinning below covers what
// this build actually registers.
//
// The check inside that test compares two counts - how many cases this file
// lists against how many the golden file records - and catches one of the pair
// being changed alone. It says nothing about whether a format appears at all,
// because the cases are a hand written map and nothing walks the registry.
//
// So a format could be added, pass every guard in this package, and ship with
// its bytes pinned nowhere at all. D11 says those bytes do not move inside a
// major version, and for that format the promise would quietly be empty.
//
// Found on 2026-08-04 while adding the thirteenth format: its three cases went
// in because the pattern says to add them, not because anything asked. That is
// the same shape this project keeps finding - a guard that checks the rule was
// followed where it was followed, and is silent where it was not.
func TestEveryFormatHasAGoldenValue(t *testing.T) {
	descriptors := format.All()
	if len(descriptors) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	covered := map[string]bool{}
	for _, target := range goldenCases() {
		covered[target.Format] = true
	}

	for _, d := range descriptors {
		if !covered[d.ID] {
			t.Errorf("%s is registered and no golden case produces one, so its bytes are pinned "+
				"nowhere and D11 does not reach it. Add a case to goldenCases and record what it measures",
				d.ID)
		}
	}

	// The other direction, so a case naming a format this build does not have
	// is a mistake rather than a line nobody reads.
	for id := range covered {
		if _, err := format.Get(id); err != nil {
			t.Errorf("a golden case names the format %q and nothing registers it", id)
		}
	}
}

func TestOurOwnGeneratorsHaveNotDrifted(t *testing.T) {
	want := loadGeneratorGolden(t)
	cases := goldenCases()

	// A case added without a golden value, or a value left behind after a case
	// was dropped, both mean the pair stopped describing each other.
	if len(want.Files) != len(cases) {
		t.Fatalf("the golden file describes %d cases and the test produces %d - one of them was changed alone",
			len(want.Files), len(cases))
	}

	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		target := cases[name]
		bytesOut := generateOne(t, target)

		expected, ok := want.Files[name]
		if !ok {
			t.Errorf("case %q has no golden value", name)
			continue
		}
		sum := sha256.Sum256(bytesOut)
		actual := hex.EncodeToString(sum[:])
		if actual != expected.SHA256 || len(bytesOut) != expected.Bytes {
			t.Errorf("%s drifted\n  want %d bytes, sha256 %s\n  got  %d bytes, sha256 %s\n"+
				"  this is a breaking change - bump the major version, write the changelog entry,\n"+
				"  and do not edit the golden file to make this green",
				name, expected.Bytes, expected.SHA256, len(bytesOut), actual)
		}
	}
}

// generateOne runs the real path - plan then write - and returns the bytes that
// landed on the disk.
//
// Going through the engine rather than calling a generator directly is the
// point. The label, the padding and the naming are all part of what somebody's
// hash covers, and a shortcut past them would pin something no user receives.
func generateOne(t *testing.T, target engine.Target) []byte {
	t.Helper()
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: goldenSeed, Command: "test"}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatalf("planning %s: %v", target.Format, err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("running %s: %v", target.Format, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the output directory: %v", err)
	}
	var produced []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "manifest.json" {
			produced = append(produced, e.Name())
		}
	}
	if len(produced) != 1 {
		t.Fatalf("expected exactly one file for %s, got %d - the case is not pinning what it claims to",
			target.Format, len(produced))
	}
	b, err := os.ReadFile(filepath.Join(dir, produced[0]))
	if err != nil {
		t.Fatalf("reading %s: %v", produced[0], err)
	}
	return b
}

type generatorGolden struct {
	Note       string                 `json:"note"`
	MeasuredOn string                 `json:"measured_on"`
	Seed       int64                  `json:"seed"`
	Files      map[string]goldenEntry `json:"files"`
}

func loadGeneratorGolden(t *testing.T) generatorGolden {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(generatorGoldenFile))
	if err != nil {
		t.Fatalf("reading %s: %v\nthe golden values are part of the repository - see .gitignore", generatorGoldenFile, err)
	}
	var g generatorGolden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("parsing %s: %v", generatorGoldenFile, err)
	}
	if len(g.Files) == 0 {
		t.Fatalf("%s describes no files - this guard would pass without checking anything", generatorGoldenFile)
	}
	if g.Seed != goldenSeed {
		t.Fatalf("the golden file was measured with seed %d and the test uses %d - every value below would be wrong",
			g.Seed, goldenSeed)
	}
	return g
}
