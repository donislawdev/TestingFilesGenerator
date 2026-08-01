package guard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// This tool writes large amounts of data and it runs in directories that
// belong to the user. These guards cover the three ways it could damage
// something rather than merely fail.

func TestNothingIsEverWrittenOverInSilence(t *testing.T) {
	dir := t.TempDir()

	// Something of the user's, sitting where a generated file would land.
	victim := filepath.Join(dir, "files_0001.txt")
	const precious = "work that took an afternoon"
	if err := os.WriteFile(victim, []byte(precious), 0o644); err != nil {
		t.Fatalf("preparing the file: %v", err)
	}

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("the run went ahead over an existing file")
	}
	var collision *engine.CollisionError
	if !errors.As(runErr, &collision) {
		t.Errorf("refused with %T, expected a CollisionError so the caller can answer with the right exit code", runErr)
	}

	after, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("reading the file back: %v", err)
	}
	if string(after) != precious {
		t.Error("the existing file was destroyed - this is the one failure that cannot be undone by running again")
	}
}

func TestTwoFilesCannotHeadForOneName(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}

	// A name template with no index means every file of the target wants the
	// same name. Without this check one file survives, the manifest describes
	// three, and the suite reads a manifest that quietly lost two of them.
	targets := []engine.Target{{
		ID: "files", Format: "txt", Sizes: engine.Uniform(3, 512),
		NameTmpl: "same.txt", Label: true,
	}}

	_, err := engine.Plan(targets, opt)
	if err == nil {
		t.Fatal("planning accepted three files heading for one name")
	}
	var recipeErr *engine.RecipeError
	if !errors.As(err, &recipeErr) {
		t.Errorf("refused with %T, expected a RecipeError", err)
	}
	if !strings.Contains(err.Error(), "same.txt") {
		t.Errorf("the message does not name the clashing file: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("planning left %d entries on disk - it must never touch it", len(entries))
	}
}

func TestARunTooBigForTheDiskIsRefusedBeforeTheFirstByte(t *testing.T) {
	dir := t.TempDir()

	// The free space probe is injected rather than the run being made huge.
	// Asking for a petabyte would prove the same thing, but when the guard is
	// broken that test writes until the disk fills - on a CI runner and on
	// the machine of whoever runs it. Measured: that mutation took 50 seconds
	// before this was changed.
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		AvailableBytes: func(string) (int64, error) { return 1000, nil },
	}

	planned, err := engine.Plan([]engine.Target{txtTarget("bigger-than-the-disk", 4, 8192)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("a run needing a petabyte was allowed to start")
	}
	var spaceErr *engine.SpaceError
	if !errors.As(runErr, &spaceErr) {
		t.Fatalf("refused with %T, expected a SpaceError", runErr)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("a refused run left %d entries behind, expected none", len(entries))
	}
}

func TestAnInterruptedRunLeavesNoPartialFileAndStillWritesAManifest(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}

	planned, err := engine.Plan([]engine.Target{txtTarget("files", 40, 8192)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	// Cancel immediately. This stands in for Ctrl+C, a kill and a CI timeout,
	// and it costs no disk to test.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, runErr := engine.Run(ctx, planned, opt)
	if runErr == nil {
		t.Fatal("a cancelled run reported success")
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Errorf("a cancelled run ended with %v, expected a cancellation", runErr)
	}

	// The invariant: the output directory never holds an incomplete file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tfg-partial") {
			t.Errorf("%s was left behind - a half written file that looks finished reaches a test suite as a false truth", e.Name())
		}
	}

	// A manifest exists even so, otherwise cleanup has nothing to work with
	// and the leftovers stay for good.
	if res.Manifest == nil {
		t.Fatal("a cancelled run produced no manifest")
	}
	if res.Manifest.Run.Complete {
		t.Error("the manifest of a cancelled run claims the run finished")
	}
}

func TestAFreshRunIntoAnEmptyDirectoryStillWorks(t *testing.T) {
	// The guards above refuse things. This one exists so that refusing
	// everything would not pass as success.
	dir := t.TempDir()
	opt := engine.Options{OutDir: filepath.Join(dir, "nested", "out"), Seed: 7741, Command: "test"}

	planned, err := engine.Plan([]engine.Target{txtTarget("files", 3, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("a normal run into an empty directory failed: %v", err)
	}
	if res.Manifest.Summary.Materialized != 3 {
		t.Errorf("produced %d files, expected 3", res.Manifest.Summary.Materialized)
	}
}
