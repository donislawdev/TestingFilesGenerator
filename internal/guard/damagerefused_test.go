package guard

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A damaged file has to be REFUSED, which is the reverse of everything else in
// this package.
//
// Every other oracle guard here asks "does a judge accept what we wrote". This
// one asks the opposite, and it is the whole reason the damage axis is worth
// having: a broken file no reader complains about is a file nobody can write a
// test against, so it is a file this tool has no business producing.
//
// The formats are the text ones plus png, because those are the ones whose
// judges run without an external tool being installed. That is a limit of what
// can be asked on any machine rather than of the damage: the full sweep across
// all twenty four lives in tools/probes/corruptwitness, which is where the
// witness matrix is measured.
func TestADamagedFileIsRefusedByAJudge(t *testing.T) {
	dir := t.TempDir()

	for _, id := range []string{"txt", "md", "json", "xml", "csv", "html", "png"} {
		t.Run(id, func(t *testing.T) {
			good, broken := writeGoodAndBroken(t, dir, id)

			// The control first. Without it this test passes for a build whose
			// judge refuses everything, which would be the loudest possible
			// way of proving nothing.
			if res := oracle.Strict(id, good); res.Available && res.Err != nil {
				t.Fatalf("the undamaged file was refused, so this row says nothing about damage: %v\n%s",
					res.Err, res.Output)
			}

			res := oracle.Strict(id, broken)
			if !res.Available {
				t.Skipf("no structural check for %s on this machine", id)
			}
			if res.Err == nil {
				t.Errorf("the damaged file was ACCEPTED. A broken file every reader takes is one "+
					"nobody can write a test against, and the manifest calls it expected: reject.\n%s",
					res.Output)
			}
		})
	}
}

// The manifest of a damaged run says what was broken and expects a rejection.
//
// Read from the file on disk rather than from the plan, because the whole
// point of the record is what a consumer finds in it.
func TestTheManifestOfADamagedRunSaysSoAndExpectsRejection(t *testing.T) {
	dir := t.TempDir()
	man := runDamaged(t, dir, "png", damage.Chain{{
		ID: damage.ZeroHead, Values: damage.Values{damage.SettingBytes: "16"},
	}})

	if len(man.Files) != 1 {
		t.Fatalf("expected one file in the manifest, got %d", len(man.Files))
	}
	f := man.Files[0]

	if len(f.Damage) != 1 {
		t.Fatalf("the manifest records %d damage(s) and one was applied", len(f.Damage))
	}
	if f.Damage[0].Type != damage.ZeroHead {
		t.Errorf("the manifest names %q rather than the damage that was applied", f.Damage[0].Type)
	}
	// Resolved rather than as written, so a consumer does not have to know
	// what the default was in the build that wrote it.
	if got := f.Damage[0].Settings[damage.SettingBytes]; got != "16" {
		t.Errorf("the manifest records bytes=%q rather than what was asked for", got)
	}
	if f.Expected.Outcome != manifest.OutcomeReject {
		t.Errorf("a damaged file is expected to be %q rather than %q",
			manifest.OutcomeReject, f.Expected.Outcome)
	}
	// Stated with certainty rather than as a guess, and allowed to be: the
	// witness rule says a damage nothing can refuse is not offered, so this is
	// measured rather than invented. Untouchable rule 5 is about the second.
	if f.Expected.Confidence != "certain" {
		t.Errorf("the expectation is %q confident and it is measured, not guessed", f.Expected.Confidence)
	}
}

// An undamaged run is untouched by any of this.
//
// D11 in the narrowest place it can be asked: the bytes of a file nobody
// damaged, and the manifest entry beside it, both have to be what they were
// before the axis existed. The damage key is ABSENT rather than empty, so a
// manifest written by an older build is still the same document.
func TestARunWithNoDamageIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	man := runDamaged(t, dir, "png", nil)

	f := man.Files[0]
	if f.Damage != nil {
		t.Errorf("a run with no damage recorded %v", f.Damage)
	}
	if f.Expected.Outcome != manifest.OutcomeUnspecified {
		t.Errorf("a file nobody damaged expects %q rather than %q",
			manifest.OutcomeUnspecified, f.Expected.Outcome)
	}

	body, err := os.ReadFile(filepath.Join(dir, f.Name))
	if err != nil {
		t.Fatal(err)
	}
	// The signature, which is the first thing zero-head would have taken.
	if len(body) < 8 || body[0] != 0x89 || string(body[1:4]) != "PNG" {
		t.Errorf("the file does not open with the PNG signature: % x", body[:min(8, len(body))])
	}
}

// A damage that would change nothing stops the file rather than writing it.
//
// Reachable rather than theoretical, and that is measured: ico, avif and jxl
// all begin with zero bytes, so zeroing one or two of them moves nothing. Here
// it is asked of a file made of zeros, because the guard has to press the
// answer rather than depend on which formats happen to start that way.
func TestADamageThatWouldChangeNothingStopsTheFile(t *testing.T) {
	source := make([]byte, 64)
	chain := damage.Chain{{ID: damage.ZeroHead}}

	var sink discard
	w, streams, err := chain.Open(&sink)
	if err != nil {
		t.Fatalf("opening the chain: %v", err)
	}
	if _, err := w.Write(source); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if idle := chain.Idle(streams); idle == "" {
		t.Fatal("every byte was already zero and the chain reports nothing idle")
	}
}

// discard swallows bytes. The chain has to write somewhere and what it wrote
// is not the question here - whether it moved anything is.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// writeGoodAndBroken produces one file of this format twice, once whole and
// once damaged, and returns both paths.
func writeGoodAndBroken(t *testing.T, dir, id string) (good, broken string) {
	t.Helper()

	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	size := d.SmallestAccepted(format.Request{}) + 4096

	goodDir := filepath.Join(dir, id+"-good")
	brokenDir := filepath.Join(dir, id+"-broken")
	goodMan := runOne(t, goodDir, id, size, nil)
	brokenMan := runOne(t, brokenDir, id, size, damage.Chain{{ID: damage.ZeroHead}})

	return filepath.Join(goodDir, goodMan.Files[0].Name),
		filepath.Join(brokenDir, brokenMan.Files[0].Name)
}

// runDamaged is one png of a comfortable size, with the chain given.
func runDamaged(t *testing.T, dir, id string, chain damage.Chain) *manifest.Manifest {
	t.Helper()
	return runOne(t, dir, id, 20<<10, chain)
}

// runOne writes one file through the whole engine and hands back the manifest
// it wrote, read from disk.
func runOne(t *testing.T, dir, id string, size int64, chain damage.Chain) *manifest.Manifest {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	target := engine.Target{
		ID:     "d",
		Format: id,
		Sizes:  engine.Uniform(1, size),
		Damage: chain,
	}
	opt := engine.Options{OutDir: dir, Seed: 5, ManifestName: engine.DefaultManifestName}

	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatalf("planning %s: %v", id, err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running %s: %v", id, err)
	}
	for _, f := range res.Manifest.Files {
		if f.Failed {
			t.Fatalf("%s: the run reported a failure: %s", id, f.Error)
		}
	}
	return res.Manifest
}

// A file smaller than its damage is refused BEFORE anything is written.
//
// In the plan rather than at the moment of writing, because the fault is in
// the recipe and is visible before the first byte - which is what makes "an
// invalid recipe writes no files" true here as everywhere else.
//
// Reachable rather than theoretical: txt declares a minimum of 0 B and a zero
// byte file really is produced, so a person can ask for one and damage it.
func TestAFileSmallerThanItsDamageIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	dir := t.TempDir()
	target := engine.Target{
		ID:     "tiny",
		Format: "txt",
		Sizes:  engine.Uniform(1, 4),
		Damage: damage.Chain{{ID: damage.ZeroHead}}, // eight bytes by default
	}
	opt := engine.Options{OutDir: dir, ManifestName: engine.DefaultManifestName}

	_, err := engine.Plan([]engine.Target{target}, opt)
	if err == nil {
		t.Fatal("a 4 B file was planned with a damage that needs 8 B")
	}

	var small *damage.TooSmallError
	if !errors.As(err, &small) {
		t.Fatalf("refused with %T, which nothing can take apart: %v", err, err)
	}
	if small.Floor != 8 || small.Requested != 4 {
		t.Errorf("the refusal says floor %d and requested %d", small.Floor, small.Requested)
	}
	// The four parts D6 asks for, and the one that matters most: it names a
	// size that would work rather than only saying no.
	if !strings.Contains(small.Instead(), "8 B") {
		t.Errorf("the refusal does not name a size that would work: %q", small.Instead())
	}

	// Nothing on disk. The whole reason this is a planning refusal.
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("the run was refused and left %d file(s) behind", len(left))
	}
}

// A size the damage CAN take is planned, which is the control.
//
// Without it the test above passes for a build that refuses every damaged
// target, and that would be a floor that has become a wall.
func TestASizeTheDamageCanTakeIsPlanned(t *testing.T) {
	target := engine.Target{
		ID:     "fine",
		Format: "txt",
		Sizes:  engine.Uniform(1, 8),
		Damage: damage.Chain{{ID: damage.ZeroHead}},
	}
	opt := engine.Options{OutDir: t.TempDir(), ManifestName: engine.DefaultManifestName}

	if _, err := engine.Plan([]engine.Target{target}, opt); err != nil {
		t.Fatalf("8 B is exactly what the damage needs and it was refused: %v", err)
	}
}

// Damage beside an expectation of accept is refused, and the other three
// outcomes are not.
//
// Only accept, and the narrowness is the decision. reject is what damage
// implies, and sanitize and unspecified are both sensible questions about a
// broken file - a system under test may be expected to repair it, or that may
// be the point of the test. Refusing more than the one that cannot be true
// would take away cases somebody has a right to build.
//
// The half that matters is the second loop. Without it this passes for a build
// that refuses damage beside ANY expectation, which is the wall version of the
// same rule and would be invisible from the first loop alone.
func TestDamageBesideAnExpectationOfAcceptIsRefused(t *testing.T) {
	body := func(outcome string) []byte {
		return []byte(`version: 1
targets:
  - id: broken
    format: png
    size: 20kb
    damage: [zero-head]
    expected: ` + outcome + `
`)
	}

	if _, err := recipe.Parse(body("accept"), "r.yaml"); err == nil {
		t.Error("a target that breaks the file and expects it to be accepted was accepted")
	} else if !strings.Contains(err.Error(), "accept") {
		t.Errorf("the refusal does not say which expectation it is about: %v", err)
	}

	for _, outcome := range []string{"reject", "sanitize", "unspecified"} {
		if _, err := recipe.Parse(body(outcome), "r.yaml"); err != nil {
			t.Errorf("expected %s beside damage is a legitimate question and was refused: %v",
				outcome, err)
		}
	}
}

// The command line refuses that pair as well, and writes nothing.
//
// The guard above asks recipe.Parse and only recipe.Parse, and that was enough
// to be green through a build where this was broken. Measured on 2026-09-09 on
// the 0.3.0 binary: the recipe was refused with code 3 while
// --damage zero-head --expected accept ended with code 0 and left a manifest
// on disk saying a deliberately damaged file should be accepted - the tool
// lying in the one place its value lives. O199.
//
// Both halves are needed. Asking only the refusal would pass for a build that
// refuses damage beside any expectation at all, and asking only that files are
// written would pass for one that refuses nothing - so the second loop is the
// wall detector and the first is the hole detector.
//
// The count of files is asked rather than the exit code alone: a refusal that
// arrives after the writing has started is a refusal that came too late, and
// the code by itself cannot tell those apart.
func TestTheCommandLineRefusesDamageBesideAcceptToo(t *testing.T) {
	run := func(t *testing.T, extra ...string) (int, string, int) {
		t.Helper()
		dir := t.TempDir()
		var out, errOut bytes.Buffer
		args := append([]string{
			"generate", "--format", "txt", "--size", "100",
			"--damage", damage.ZeroHead, "--out", dir,
		}, extra...)
		code := cli.Run(context.Background(), args, &out, &errOut)
		written, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading the output directory: %v", err)
		}
		return code, errOut.String(), len(written)
	}

	code, said, files := run(t, "--expected", damage.RuledOutExpectation)
	if code != cli.ExitUsage {
		t.Errorf("damage beside %q ended with %d, expected %d - two flags that cancel each other are a fault in the invocation\nstderr: %s",
			damage.RuledOutExpectation, code, cli.ExitUsage, said)
	}
	if files != 0 {
		t.Errorf("the run was refused and still left %d file(s) behind", files)
	}
	if !strings.Contains(said, damage.RuledOutExpectation) {
		t.Errorf("the refusal does not name the word it turned down: %s", said)
	}

	// The other three are legitimate questions about a broken file, and a
	// build refusing them would be a wall rather than this rule.
	for _, outcome := range []string{"reject", "sanitize", "unspecified"} {
		if code, said, _ := run(t, "--expected", outcome); code != cli.ExitOK {
			t.Errorf("--expected %s beside damage ended with %d rather than %d: %s",
				outcome, code, cli.ExitOK, said)
		}
	}
	// And the expectation on its own is untouched. Every case above carries a
	// damage, so a build that refused accept for every run whatsoever would
	// look correct from all of them.
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"generate", "--format", "txt", "--size", "100",
		"--expected", damage.RuledOutExpectation, "--out", dir,
	}, &out, &errOut); code != cli.ExitOK {
		t.Errorf("--expected %s with nothing damaged ended with %d rather than %d, which makes this a wall rather than a rule about damage: %s",
			damage.RuledOutExpectation, code, cli.ExitOK, errOut.String())
	}
}

// The outcome damage rules out is the one the manifest and the recipe know.
//
// Three spellings of one word live in three packages that cannot import each
// other - damage sits beside manifest rather than under it - so this compares
// them rather than leaving them to drift. A rename in one place turns this red
// instead of quietly producing a build where nothing is ever refused.
func TestTheOutcomeDamageRulesOutIsTheOneTheManifestKnows(t *testing.T) {
	if damage.RuledOutExpectation != manifest.OutcomeAccept {
		t.Errorf("damage rules out %q and the manifest calls it %q, so nothing would ever match",
			damage.RuledOutExpectation, manifest.OutcomeAccept)
	}
	if !slices.Contains(recipe.Outcomes(), damage.RuledOutExpectation) {
		t.Errorf("damage rules out %q and a recipe does not accept that word at all: %v",
			damage.RuledOutExpectation, recipe.Outcomes())
	}
}

// A damage the build does not know is refused while the recipe is read, and
// the refusal names what there is.
//
// While the recipe is read rather than when the file is written, because an
// invalid recipe produces no files at all - and the names come from the
// registry, so a second damage appears in that sentence on the day it is
// registered.
func TestADamageNobodyRegisteredIsRefusedWithTheNamesThereAre(t *testing.T) {
	src := []byte(`version: 1
targets:
  - id: broken
    format: png
    size: 20kb
    damage: [shred-it]
`)

	_, err := recipe.Parse(src, "r.yaml")
	if err == nil {
		t.Fatal("a damage nobody registered was accepted")
	}
	for _, want := range []string{"shred-it", damage.ZeroHead} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}

// A damage setting outside its declaration is refused in the registry's own
// words.
//
// The wording is not written in the damage package and must not be: bytes is
// declared as a number with a range, and format.Property.Allows builds the
// sentence - so a damage parameter and a format setting refuse alike, and
// neither copies the other. Four is the floor and it is measured: below it,
// four of the twenty four formats come out with damage no reader complains
// about.
func TestADamageSettingOutsideItsDeclarationIsRefused(t *testing.T) {
	src := []byte(`version: 1
targets:
  - id: broken
    format: png
    size: 20kb
    damage:
      - type: zero-head
        bytes: 2
`)

	_, err := recipe.Parse(src, "r.yaml")
	if err == nil {
		t.Fatal("zeroing two bytes was accepted and four formats have no witness for it")
	}
	if !strings.Contains(err.Error(), "bytes") {
		t.Errorf("the refusal does not name the setting: %v", err)
	}
}
