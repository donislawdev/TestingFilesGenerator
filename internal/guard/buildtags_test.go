package guard

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"os/exec"
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
const buildTagsFile = "../../" + buildTagsFileName

// buildTagsFileName is the same file as a person types it, which is what
// belongs in a message telling somebody what to do about it.
const buildTagsFileName = ".github/build-tags"

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
// module.
//
// It used to say that lines fetching and running somebody else's tool are not
// ours to tag, and that sentence hid three jobs. A tool fetched with "go run"
// and then pointed at ./... loads and type checks our packages exactly as the
// compiler does, and the tag selects different files inside two of our
// dependencies - so govulncheck was doing its reachability analysis through a
// build we do not ship, and staticcheck and golangci-lint were reading files no
// released binary contains and not reading the ones it does. Found by an
// outside review on 2026-09-05 and confirmed here.
//
// So "go run" is two commands wearing one name. Running a tool to read a
// configuration file - "golangci-lint config verify" - never opens a package
// and needs no tag. Running one over our packages does, and so does running one
// of our own commands. Both of those say so by naming a package pattern of
// ours, which is what the last line asks.
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
	return strings.Contains(trimmed, "go run") && strings.Contains(trimmed, "./")
}

// A build without the tags does not compile, and says which tags.
//
// Everything above keeps the tag on the commands this project runs. None of it
// reaches somebody who builds the program themselves, and until 2026-09-06
// README.md offered three ways to do that and not one carried the tag. So
// "go install ...@latest", a distribution packager and a contributor running
// from a checkout all got the AVX2 path that reads past the end of a buffer -
// while every workflow here was careful not to.
//
// The refusal is at build time on purpose. The alternative is a binary that
// works until it meets a picture size that takes the process down, which is the
// same defect arriving months later and somewhere nobody can act on it.
//
// Asked of the compiler rather than of the source, because what matters is that
// the build FAILS. A guard reading internal/format/avif for a build constraint
// would stay green against a file that had stopped failing.
func TestABuildWithoutTheBuildTagsRefusesAndSaysWhy(t *testing.T) {
	out := filepath.Join(t.TempDir(), "untagged.bin")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/tfg")
	cmd.Dir = filepath.Join("..", "..")
	combined, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("cmd/tfg built with no build tags at all.\n" +
			"Without them the AVIF encoder's AVX2 path reads past the end of a buffer and takes " +
			"the process down on some picture sizes, so a build that gets that far is a build " +
			"somebody will run. internal/format/avif is where the refusal lives.")
	}
	said := string(combined)
	if !strings.Contains(said, "build tags") {
		t.Errorf("the build failed without the tags, which is right, but what it said does not "+
			"name build tags:\n%s\nSomebody meeting this has to be able to act on it.", said)
	}
}

// The install instructions carry the tags the build needs.
//
// One file names the tags and everything else reads it, which works for the
// commands this project runs and cannot work for a command somebody types
// before they have the repository. So README.md carries the tag itself, and
// this is what keeps that copy honest.
func TestTheInstallInstructionsCarryTheBuildTags(t *testing.T) {
	tags := buildTags()
	raw, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}

	checked := 0
	for i, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "go install") && !strings.Contains(line, "go build ") {
			continue
		}
		checked++
		if strings.Contains(line, buildTagsFileName) || strings.Contains(line, "-tags "+tags) {
			continue
		}
		t.Errorf("README.md line %d tells somebody to build without the build tags:\n  %s\n"+
			"What to do: add -tags %q to it, or read them from %s where the command is run "+
			"inside a checkout. Without them the AVIF encoder reads past the end of a buffer.",
			i+1, strings.TrimSpace(line), tags, buildTagsFileName)
	}
	if checked == 0 {
		t.Fatal("no line of README.md was recognised as an install or build command, so this " +
			"guard checked nothing")
	}
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
