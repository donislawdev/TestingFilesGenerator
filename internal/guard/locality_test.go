package guard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// Editing one place in a recipe must not move bytes somewhere else.
//
// The naive design is one random stream every target draws from in order.
// Adding a file at the top then shifts everything below it, every hash in
// someone's CI goes red at once, and after the second time that happens the
// team stops trusting the tool and goes back to keeping binaries in the
// repository. Deriving seeds instead of consuming them is what prevents it,
// and this is the test that says so.

func hashesOf(t *testing.T, targets []engine.Target, seed int64) map[string]string {
	t.Helper()
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: seed, Command: "test"}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("running: %v", err)
	}

	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "manifest.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		sum := sha256.Sum256(b)
		out[e.Name()] = hex.EncodeToString(sum[:])
	}
	if len(out) == 0 {
		t.Fatal("no file was produced - this guard would pass without checking anything")
	}
	return out
}

func txtTarget(id string, count int, bytes int64) engine.Target {
	return engine.Target{ID: id, Format: "txt", Sizes: engine.Uniform(count, bytes), Label: true}
}

func TestAddingATargetLeavesTheOthersUntouched(t *testing.T) {
	before := hashesOf(t, []engine.Target{
		txtTarget("invoices", 3, 2048),
	}, 7741)

	after := hashesOf(t, []engine.Target{
		txtTarget("invoices", 3, 2048),
		txtTarget("reports", 2, 4096),
	}, 7741)

	for name, want := range before {
		got, ok := after[name]
		if !ok {
			t.Errorf("%s disappeared after another target was added", name)
			continue
		}
		if got != want {
			t.Errorf("%s changed after another target was added - editing one place moved bytes in another", name)
		}
	}
}

func TestReorderingTargetsChangesNothing(t *testing.T) {
	a := hashesOf(t, []engine.Target{
		txtTarget("invoices", 2, 2048),
		txtTarget("reports", 2, 4096),
	}, 7741)

	b := hashesOf(t, []engine.Target{
		txtTarget("reports", 2, 4096),
		txtTarget("invoices", 2, 2048),
	}, 7741)

	for name, want := range a {
		if b[name] != want {
			t.Errorf("%s changed when the targets were listed in a different order", name)
		}
	}
}

func TestRaisingTheCountLeavesTheEarlierFilesIdentical(t *testing.T) {
	before := hashesOf(t, []engine.Target{txtTarget("invoices", 3, 2048)}, 7741)
	after := hashesOf(t, []engine.Target{txtTarget("invoices", 6, 2048)}, 7741)

	if len(after) <= len(before) {
		t.Fatalf("raising the count from 3 to 6 gave %d files, not more than %d", len(after), len(before))
	}
	for name, want := range before {
		if after[name] != want {
			t.Errorf("%s changed when the count went from 3 to 6 - the earlier files must stay byte for byte identical", name)
		}
	}
}

func TestChangingATargetIdChangesThatTargetOnly(t *testing.T) {
	a := hashesOf(t, []engine.Target{
		txtTarget("invoices", 2, 2048),
		txtTarget("reports", 2, 2048),
	}, 7741)
	b := hashesOf(t, []engine.Target{
		txtTarget("invoices", 2, 2048),
		txtTarget("summaries", 2, 2048),
	}, 7741)

	// The untouched target keeps its bytes.
	for name, want := range a {
		if name[:8] != "invoices" {
			continue
		}
		if b[name] != want {
			t.Errorf("%s changed when a different target was renamed", name)
		}
	}

	// The renamed one is genuinely different, because the id is the identity
	// of the target rather than a label on it.
	var renamed []string
	for name := range b {
		if len(name) >= 9 && name[:9] == "summaries" {
			renamed = append(renamed, name)
		}
	}
	sort.Strings(renamed)
	if len(renamed) == 0 {
		t.Fatal("renaming a target produced no files under the new id")
	}
	for _, name := range renamed {
		old := "reports" + name[len("summaries"):]
		if a[old] == b[name] {
			t.Errorf("%s has the same bytes as %s - changing an id has to change that target's files", name, old)
		}
	}
}

func TestDryRunWritesNothingAtAll(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, DryRun: true, Command: "test"}

	targets := []engine.Target{txtTarget("invoices", 5, 4096)}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	if len(entries) != 0 {
		t.Errorf("a dry run left %d entries behind: %v", len(entries), entries)
	}

	// It still has to report what would happen, otherwise it counts nothing
	// and shows nothing.
	if res.Manifest.Summary.FileCount != 5 {
		t.Errorf("a dry run described %d files, expected 5", res.Manifest.Summary.FileCount)
	}
	if got := engine.TotalBytes(planned); got != 5*4096 {
		t.Errorf("a dry run totalled %d B, expected %d B", got, 5*4096)
	}
}

func TestASkippedLabelIsVisibleInTheManifest(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}

	// Twelve bytes cannot hold the label. Silence is banned, so the entry has
	// to say so rather than simply carrying no label.
	planned, err := engine.Plan([]engine.Target{txtTarget("tiny", 1, 12)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}

	if len(res.Manifest.Files) != 1 {
		t.Fatalf("expected one entry, got %d", len(res.Manifest.Files))
	}
	e := res.Manifest.Files[0]
	if e.LabelEmbedded {
		t.Error("the entry claims a label the file is too small to hold")
	}
	found := false
	for _, n := range e.Notes {
		if n.Code == "label_omitted" {
			found = true
		}
	}
	if !found {
		t.Error("the label was dropped without a note - a file that quietly differs from what was ordered is exactly what must never happen")
	}
}
