package guard

import (
	"path/filepath"
	"testing"
)

// The manifest says which target each file came from.
//
// Nothing else in the entry answers that. id is the handle a test refers to one
// file by (MF2) and it is numbered across the whole run, so two targets produce
// f_0001 to f_0005 with nothing marking where one ends. format is shared the
// moment two targets ask for one format. group is optional and free text. The
// file name carries the target id only while nobody supplies a name template of
// their own - and somebody who did is exactly the person who would want to ask.
//
// So the only way to count "how many files did which target produce" was to
// parse file names, which is the work a manifest exists to remove. Found on
// 2026-08-18 while writing a guard for a run started from the recipe screen,
// where the assertion grouping files by target returned zeroes. Observation
// O97, closed on 2026-08-27 with the owner's yes.
//
// Both halves are asked of the JSON a consumer receives rather than of the
// struct. Decoding into manifest.File would pass by construction the moment the
// type changed, which is the trap manifestShape in recipe_test.go was written
// against.
//
// The two targets share one format on purpose. A build that filled this in from
// the descriptor rather than from the target would produce one key covering
// both, and a check written against two formats would have called that correct.
func TestTheManifestSaysWhichTargetEachFileCameFrom(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
seed: 5
targets:
  - id: small-invoices
    format: txt
    count: 2
    size: 1024
    name: "a_{index:04}.txt"
  - id: big-invoices
    format: txt
    count: 3
    size: 4096
    name: "b_{index:04}.txt"
output:
  dir: `+filepath.ToSlash(out)+`
`)

	if code, _, errOut := run(t, "generate", path); code != 0 {
		t.Fatalf("the run ended with %d: %s", code, errOut)
	}

	m := parsedManifest(t, out)

	// Size is the independent axis. It comes from the target too, so an entry
	// carrying the wrong target id disagrees with its own byte count - which a
	// constant, or the format id, or the group cannot satisfy.
	wantBytes := map[string]float64{"small-invoices": 1024, "big-invoices": 4096}
	wantCount := map[string]int{"small-invoices": 2, "big-invoices": 3}

	files, ok := m["files"].([]any)
	if !ok {
		t.Fatalf("the manifest carries no file list: %T", m["files"])
	}
	if len(files) != 5 {
		t.Fatalf("the run wrote %d entries where the recipe asked for 5", len(files))
	}

	counted := map[string]int{}
	for _, raw := range files {
		f, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("an entry is not an object: %T", raw)
		}
		id, _ := f["target_id"].(string)
		if id == "" {
			t.Fatalf("entry %v does not say which target produced it, so counting files per "+
				"target means parsing names again", f["path"])
		}
		want, known := wantBytes[id]
		if !known {
			t.Errorf("entry %v names target %q, and the recipe has no such target", f["path"], id)
			continue
		}
		if got, _ := f["bytes"].(float64); got != want {
			t.Errorf("entry %v says it came from %q, whose files are %.0f B, and it is %.0f B",
				f["path"], id, want, got)
		}
		counted[id]++
	}

	for id, want := range wantCount {
		if counted[id] != want {
			t.Errorf("%d entries name target %q where the recipe asked for %d", counted[id], id, want)
		}
	}

	// The summary carries the same answer without walking the entries, and it
	// has to agree with them. Two numbers drifting apart is worse than one
	// missing, because the cheap one is the one a person reads by eye.
	summary, ok := m["summary"].(map[string]any)
	if !ok {
		t.Fatalf("the manifest carries no summary: %T", m["summary"])
	}
	byTarget, ok := summary["by_target"].(map[string]any)
	if !ok {
		t.Fatalf("the summary does not break the run down by target: %T", summary["by_target"])
	}
	if len(byTarget) != len(wantCount) {
		t.Errorf("the summary names %d targets where the run had %d: %v", len(byTarget), len(wantCount), byTarget)
	}
	for id, want := range wantCount {
		if got, _ := byTarget[id].(float64); int(got) != want {
			t.Errorf("the summary says target %q produced %.0f files and the entries say %d", id, got, want)
		}
	}

	// Naming the target is an added field, so the schema a consumer checks has
	// not moved. docs/MANIFEST.md section 10: the schema grows by adding fields
	// and a reader written against 1.0 is handed 1.0. If this ever has to
	// change it is a major and a changelog entry, not a line edited here.
	if got, _ := m["manifest_version"].(string); got != "1.0" {
		t.Errorf("manifest_version is %q. Adding a field is not a breaking change under "+
			"docs/MANIFEST.md section 10, so moving this needs its own decision", got)
	}
}
