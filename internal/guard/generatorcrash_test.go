package guard

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A generator that crashes costs one file, not the process and not the
// directory.
//
// There was one recover in this whole tree until 2026-09-03, around the yaml
// reader, and the argument that put it there is the same one here: a crash in
// there is a crash on ordinary user input. Two formats hand their pixels to
// somebody else's encoder, and every format runs on numbers that came from a
// recipe.
//
// The crash is not the expensive part. writeOne creates the file under a
// temporary name and renames it into place, so a panic left that name on the
// disk - and untouchable rule 7 makes the manifest the whole authority over
// what cleanup may delete, so a file that never reached one is a file nothing
// in this tool can ever take away. verify reports it from then on and no
// command ends it.
//
// So the assertion that earns this guard is the one about the leftover: after
// a crash the directory holds what it would have held if the file had been
// refused.
func TestAGeneratorThatCrashesWhileWritingCostsOneFile(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7, Command: "test"}
	targets := []engine.Target{
		txtTarget("broken", 1, 4096),
		txtTarget("good", 1, 4096),
	}

	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if len(planned) != 2 {
		t.Fatalf("planned %d files, expected 2", len(planned))
	}
	// The real descriptor carrying the real plan, with only the writing half
	// swapped for one that stops the way a defect stops a program.
	planned[0].Desc.Generator = crashingGenerator{Generator: planned[0].Desc.Generator}

	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("the run ended on the crash instead of carrying on: %v", err)
	}
	if res.Failures != 1 {
		t.Errorf("the run reports %d failures, expected 1", res.Failures)
	}

	if _, err := os.Stat(filepath.Join(dir, planned[0].Name)); !os.IsNotExist(err) {
		t.Errorf("%s is on the disk and the generator never finished writing it", planned[0].Name)
	}
	if _, err := os.Stat(filepath.Join(dir, planned[1].Name)); err != nil {
		t.Errorf("the file beside the crash was not produced: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if core.IsPartialName(e.Name()) {
			t.Errorf("%s was left behind, and nothing in this tool can ever remove it", e.Name())
		}
	}

	// The manifest says what happened in the words of what happened. Swallowing
	// the panic and letting the size check answer instead gives "generator for
	// txt produced 0 B where the plan said 4096 B", which describes a symptom
	// and blames the wrong thing.
	said := ""
	for _, f := range res.Manifest.Files {
		if f.ID != planned[0].ID {
			continue
		}
		said = f.Error
		if !f.Failed {
			t.Errorf("the manifest does not mark %s as failed", f.Name)
		}
	}
	if said == "" {
		t.Fatal("the manifest carries no error for the file that crashed")
	}
	for _, want := range []string{"txt", "internal error", crashWord} {
		if !strings.Contains(said, want) {
			t.Errorf("the manifest entry does not say %q: %s", want, said)
		}
	}
}

// Both places the engine hands work to a generator go through that wrapper.
//
// Asked of the source, because there is no way to ask it of a run. The
// planning side looks its descriptor up in the registry, so a test cannot hand
// it a generator that crashes without registering one - and a registered
// format is visible to every other guard in this package, which ask that every
// format carries a card, a layer, an oracle and a mutation. The write side is
// proven above by crashing it for real, and the planning side is held to going
// through the same wrapper.
//
// Planning is the likelier of the two, which is worth saying because the
// review that asked for this was looking at the other one: jxl and avif encode
// while PLANNING, walking a ladder of sizes and handing each rung to a
// borrowed encoder.
func TestEveryCallIntoAGeneratorGoesThroughTheCrashWrapper(t *testing.T) {
	const wrapper = "crash.go"
	calls := []string{"Generator.Plan(", "Generator.Write("}

	dir := filepath.Join(repoRoot(t), "internal", "engine")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	read := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || e.Name() == wrapper {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		read++
		for _, call := range calls {
			if strings.Contains(string(body), call) {
				t.Errorf("%s calls %s itself, so a panic in there ends the process",
					e.Name(), strings.TrimSuffix(call, "("))
			}
		}
	}
	if read == 0 {
		t.Fatal("the scan read no file, so this guard would pass against anything")
	}

	// And the wrapper still makes those calls, or the rule above is satisfied
	// by a build that never reaches a generator at all.
	body, err := os.ReadFile(filepath.Join(dir, wrapper))
	if err != nil {
		t.Fatalf("reading %s: %v", wrapper, err)
	}
	for _, call := range calls {
		if !strings.Contains(string(body), call) {
			t.Errorf("%s does not call %s, so nothing in the engine does", wrapper, call)
		}
	}
}

// crashWord is the panic the generator below raises, and the manifest has to
// carry it through. Spelled once so the assertion and the crash cannot drift.
const crashWord = "reachable only by crashing on purpose"

// crashingGenerator plans the way the real format does and stops the way a
// defect stops a program.
type crashingGenerator struct{ format.Generator }

func (crashingGenerator) Write(context.Context, io.Writer, format.Plan) error {
	panic(crashWord)
}
