package guard

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A boundary set exists so three files can be told apart, and until 2026-08-03
// they could not be. They arrived as files_0001, files_0002 and files_0003,
// and the only way to learn which one sat on the limit was to read the sizes
// back off the disk - which is exactly what somebody dragging one into an
// upload form is not going to do.
//
// Reported from manual testing. The sizes were always right.

func boundaryTarget(t *testing.T, id string, limit int64) engine.Target {
	t.Helper()
	sizes, err := core.BoundarySizes(limit)
	if err != nil {
		t.Fatalf("building the boundary set for %d: %v", limit, err)
	}
	return engine.Target{
		ID:            id,
		Format:        "txt",
		Sizes:         sizes,
		BoundaryLimit: limit,
		Label:         true,
	}
}

func TestABoundarySetSaysWhichFileIsWhich(t *testing.T) {
	const limit = 1 << 20
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{boundaryTarget(t, "files", limit)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	// The name has to match the size it carries. A set that labels the files
	// in the wrong order is worse than one that does not label them, because
	// it is believed.
	want := map[string]int64{
		"files_under_limit.txt": limit - 1,
		"files_at_limit.txt":    limit,
		"files_over_limit.txt":  limit + 1,
	}
	for name, size := range want {
		info, err := os.Stat(dir + string(os.PathSeparator) + name)
		if err != nil {
			t.Errorf("no file called %s - a boundary set that does not name its "+
				"three files leaves the sizes as the only way to tell them apart", name)
			continue
		}
		if info.Size() != size {
			t.Errorf("%s is %d B and its name says it should be %d B", name, info.Size(), size)
		}
	}
}

// A name the user wrote still wins. Otherwise the fix for one complaint
// quietly takes away a feature somebody was already relying on.
func TestABoundarySetStillObeysANameTemplate(t *testing.T) {
	dir := t.TempDir()

	target := boundaryTarget(t, "files", 1<<20)
	target.NameTmpl = "invoice_{index:04}.txt"

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "_limit") {
			t.Errorf("%s was named by the boundary set even though a template was given", e.Name())
		}
	}
	for _, want := range []string{"invoice_0001.txt", "invoice_0002.txt", "invoice_0003.txt"} {
		if _, err := os.Stat(dir + string(os.PathSeparator) + want); err != nil {
			t.Errorf("no file called %s, so the template was ignored", want)
		}
	}
}

// An ordinary group is still numbered. Naming everything by role would be the
// other way to break this.
func TestAnOrdinaryGroupIsStillNumbered(t *testing.T) {
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 3, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	if _, err := os.Stat(dir + string(os.PathSeparator) + "files_0001.txt"); err != nil {
		t.Error("an ordinary group stopped being numbered")
	}
}
