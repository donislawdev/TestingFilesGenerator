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
		// The three Office packages, each at a size well above its floor. What
		// they pin is a whole OPC container: the parts, their order, the
		// compression of each one and the padding part that settles the size.
		"docx_32kib": {ID: "g", Format: "docx", Sizes: engine.Uniform(1, 32768), Label: true},
		"xlsx_32kib": {ID: "g", Format: "xlsx", Sizes: engine.Uniform(1, 32768), Label: true},
		"pptx_32kib": {ID: "g", Format: "pptx", Sizes: engine.Uniform(1, 32768), Label: true},

		// With content, because how many paragraphs, rows or slides there are
		// changes every byte of the package rather than only its length.
		"docx_32kib_many_paragraphs": {ID: "g", Format: "docx", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"paragraphs": "40"}},
		"xlsx_32kib_grid": {ID: "g", Format: "xlsx", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"rows": "25", "columns": "4"}},
		"pptx_32kib_five_slides": {ID: "g", Format: "pptx", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"slides": "5"}},

		"bmp_64kib": {ID: "g", Format: "bmp", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},

		// The two formats whose picture is grown to fill the request, so the
		// dimensions are arithmetic rather than a setting. Pinned without them,
		// because that arithmetic is what a refactor would move.
		//
		// It was one until 2026-08-29. TIFF stores its pixels uncompressed
		// too, so it is built the same way and needs the same pair of cases.
		"bmp_100kib_sized_to_fit": {ID: "g", Format: "bmp", Sizes: engine.Uniform(1, 102400), Label: true},
		"tiff_64kib": {ID: "g", Format: "tiff", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},
		"tiff_100kib_sized_to_fit": {ID: "g", Format: "tiff", Sizes: engine.Uniform(1, 102400), Label: true},

		// Naming one side and letting the other be worked out is its own
		// branch, and it had no case until 2026-08-29. The mutation that
		// removes that arithmetic came back NOT CAUGHT with the two cases
		// above in place: one names both sides and the other names neither,
		// so neither of them ever reached it. That was a hole in the evidence
		// rather than in the code, and this is what closes it.
		"tiff_64kib_width_only": {ID: "g", Format: "tiff", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "128"}},
		// WebP, pinned from three sides. Its size is arithmetic rather than
		// whatever a compressor decides, so a drift here can be a wrong SIZE and
		// not only different bytes - the same shape as BMP and TIFF.
		"webp_64kib": {ID: "g", Format: "webp", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "48"}},

		// One side named, so the other is worked out from the bytes that are
		// left. TIFF found by measurement that a case naming both sides and a
		// case naming neither both miss that arithmetic entirely.
		"webp_64kib_width_only": {ID: "g", Format: "webp", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "100"}},

		// An odd size, which is the case the padding was rebuilt for. Every RIFF
		// chunk block costs an even number of bytes, so without the tail after
		// the payload this request could not be answered at all.
		"webp_odd_size": {ID: "g", Format: "webp", Sizes: engine.Uniform(1, 65537), Label: true,
			Properties: map[string]string{"width": "64", "height": "48"}},

		// AVIF, and it is pinned for a reason the formats above do not have: the
		// pixels are coded by a library this project does not own. These hashes
		// are what would notice the codec being raised underneath us, which is
		// the whole reason it is pinned rather than followed.
		//
		// No width only case here. Every other picture format works the missing
		// side out from the bytes that are left, and AVIF cannot: what a picture
		// encodes to is whatever the coder decides, so there is no arithmetic to
		// run backwards and a named width simply keeps the default height.
		"avif_64kib": {ID: "g", Format: "avif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "48"}},

		// Quality is the one setting that changes how many bytes the coder
		// emits, so it reaches code the case above never does.
		"avif_quality": {ID: "g", Format: "avif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "48", "quality": "90"}},

		// An odd size. The free box takes any length at all, so this is the case
		// that would catch padding that could only step in twos.
		"avif_odd_size": {ID: "g", Format: "avif", Sizes: engine.Uniform(1, 65537), Label: true,
			Properties: map[string]string{"width": "64", "height": "48"}},

		"gif_64kib": {ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64"}},

		// The still GIF, which is a different encoder rather than the same one
		// with a smaller number. frames: 1 takes the plain path and writes no
		// control block, no loop block and no second image descriptor, so the
		// case above never reaches any of that arithmetic and a change to it
		// would move nobody's bytes that anything here measures.
		//
		// It is also the way back to the bytes this package wrote before it
		// could animate, which is a promise the package comment makes. Pinning
		// it is what stops that sentence from being a sentence.
		"gif_64kib_one_frame": {ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64", "frames": "1"}},

		// Enough frames that the marker lands somewhere different from either
		// end, so the arithmetic placing it is measured rather than assumed.
		// With three frames a wrong divisor still puts the square inside the
		// picture and nothing notices.
		"gif_64kib_eight_frames": {ID: "g", Format: "gif", Sizes: engine.Uniform(1, 65536), Label: true,
			Properties: map[string]string{"width": "64", "height": "64", "frames": "8"}},
		"ico_32kib": {ID: "g", Format: "ico", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "32", "height": "32"}},

		// What sits inside an icon changes every byte of it, so both answers
		// are pinned rather than only the one the default picks.
		"ico_32kib_png_inside": {ID: "g", Format: "ico", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "32", "height": "32", "embed": "png"}},

		// The first lossy format. Its bytes are pinned like every other, and
		// what that pins is different: not that the picture survives a round
		// trip, which JPEG never promises, but that the same request encodes
		// to the same file. Quality is named rather than left to the default,
		// because the default is a number somebody may change and this case
		// has to keep measuring the same thing if they do.
		// A picture too narrow for the label, so the omitted label path is
		// pinned too. Without it, drawing a label the picture has no room for
		// changes nothing any case measures - checked by breaking it.
		"jpg_narrow_label_omitted": {ID: "g", Format: "jpg", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "8", "height": "8", "quality": "90"}},
		"jpg_32kib": {ID: "g", Format: "jpg", Sizes: engine.Uniform(1, 32768), Label: true,
			Properties: map[string]string{"width": "64", "height": "64", "quality": "90"}},

		// Padding past what one comment segment can carry, which is the shape
		// no other format has - PNG refuses beyond one chunk and this one
		// writes as many segments as the size needs.
		"jpg_192kib_many_segments": {ID: "g", Format: "jpg", Sizes: engine.Uniform(1, 196608), Label: true,
			Properties: map[string]string{"width": "64", "height": "64", "quality": "90"}},
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
