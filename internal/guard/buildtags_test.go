package guard

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/gav1d/avif"
)

// The build tags this project ships with live in one file, and everything that
// compiles our code reads that file instead of carrying a copy.
//
// This is the same shape as .github/gui-ldflags, and it exists for the same
// reason: a flag written out in two places drifts, and the drift is silent.
//
// The tag is noasm, and it is not a preference. The AVIF encoder ships hand
// written AVX2 for the chroma from luma path, and that code reads past the end
// of a buffer: measured on 2026-08-29, a 640x256 picture killed the process
// with an access violation in cflAcMain8AVX2 at av1/cfl_amd64.s:281, and killed
// it in two runs out of three, which is worse than always - it depends on what
// the heap looks like. A sweep of 240 picture sizes crashed on one of them with
// the assembly and on none of them without it.
//
// The tag costs nothing that matters. The bytes are identical either way,
// measured across the whole size ladder, so D11 is untouched. Encoding a
// 320x240 picture goes from about 6 ms to about 10 ms, which is still several
// times faster than the road this project turned down.
const buildTagsFile = "../../.github/build-tags"

func buildTags() string {
	raw, err := os.ReadFile(buildTagsFile)
	if err != nil {
		panic("guard: cannot read " + buildTagsFile + ": " + err.Error())
	}
	return strings.TrimSpace(string(raw))
}

func TestTheBuildTagsFileNamesTheTagThatKeepsTheEncoderInsideItsBuffers(t *testing.T) {
	tags := buildTags()
	if tags == "" {
		t.Fatalf("%s is empty, so every build would take the assembly that reads past its buffers", buildTagsFile)
	}
	if !strings.Contains(tags, "noasm") {
		t.Errorf("the build tags are %q and do not include noasm.\n"+
			"Why it matters: the AVIF encoder's AVX2 path reads out of bounds and kills the process on some picture sizes - measured at 640x256, in two runs out of three.\n"+
			"What to do: put noasm back in %s. If a later version of the encoder fixes the assembly, retire the tag deliberately and say so here.", tags, buildTagsFile)
	}
}

// Every command in the workflows that compiles or tests our own code has to
// pass the tags, and has to take them from the file.
func TestEveryWorkflowCommandThatBuildsUsPassesTheBuildTags(t *testing.T) {
	dir := "../../.github/workflows"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no workflows here: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if !compilesOurCode(line) {
				continue
			}
			checked++
			if !strings.Contains(line, "-tags") {
				t.Errorf("%s line %d compiles our code without the build tags:\n  %s\n"+
					"What to do: add -tags \"$(cat .github/build-tags)\" to it, so the tag comes from the file rather than from memory.",
					e.Name(), i+1, strings.TrimSpace(line))
				continue
			}
			if !strings.Contains(line, ".github/build-tags") {
				t.Errorf("%s line %d passes build tags written out by hand:\n  %s\n"+
					"What to do: read them from .github/build-tags, so this command and the release cannot drift apart.",
					e.Name(), i+1, strings.TrimSpace(line))
			}
		}
	}

	if checked == 0 {
		t.Fatal("no workflow line was recognised as building or testing our code, so this guard checked nothing")
	}
	t.Logf("checked %d workflow commands", checked)
}

// compilesOurCode says whether a workflow line runs the compiler over this
// module. Lines that fetch and run somebody else's tool are not ours to tag.
func compilesOurCode(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	for _, verb := range []string{"go build", "go test", "go vet"} {
		if strings.Contains(trimmed, verb) {
			return true
		}
	}
	return false
}

// The size that crashed, encoded here so a build that lost the tag says so
// rather than waiting to be noticed by a user with unusual dimensions.
//
// It is loud rather than tidy: without the tag this does not fail, it takes the
// whole test binary down with an access violation. That is the honest shape for
// a guard against memory being read outside its buffer, and it is why the size
// is named here rather than left to chance.
func TestTheEncoderSurvivesTheSizeThatCrashedItsAssembly(t *testing.T) {
	const w, h = 640, 256

	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}

	var buf bytes.Buffer
	if err := avif.Encode(&buf, m, avif.EncodeOptions{Quality: 60, Speed: 10}); err != nil {
		t.Fatalf("encoding %dx%d failed: %v", w, h, err)
	}
	if buf.Len() == 0 {
		t.Fatalf("encoding %dx%d produced nothing", w, h)
	}
	t.Logf("%dx%d encoded to %d B without falling over", w, h, buf.Len())
}
