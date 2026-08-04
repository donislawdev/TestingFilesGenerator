package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// group names the class of case a file belongs to, and it reaches the manifest
// so a test can assert about a whole class at once rather than file by file.
//
// The manifest document has described the field since it was written. Nothing
// filled it: there was no recipe key, so it was a promise in a document with no
// mechanism behind it - the silence in the other direction that CLI.md 8.1
// lists as one of this project's own gaps.
//
// Presets are what will fill it in practice, and it is a plain recipe key
// rather than something a preset hands to the engine on purpose. PR5 says an
// ejected preset is an ordinary recipe, and a field only a preset could set
// would make the ejected copy produce something different from the preset.

// Named apart from manifestOf next door, which returns the raw text.
func parsedManifest(t *testing.T, dir string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}
	return m
}

func TestAGroupInTheRecipeReachesTheManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
seed: 5
targets:
  - id: under
    format: txt
    count: 1
    size: 1024
    group: size-boundaries
    expected: accept
  - id: plain
    format: txt
    count: 1
    size: 512
output:
  dir: `+filepath.ToSlash(out)+`
`)

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}

	files, _ := parsedManifest(t, out)["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(files))
	}

	first, _ := files[0].(map[string]any)
	if got := first["group"]; got != "size-boundaries" {
		t.Errorf("the first entry carries group %v and the recipe said size-boundaries", got)
	}

	// Absent rather than empty. A consumer reading this can then tell "no
	// class" from "a class called nothing", which is the difference an empty
	// string would throw away.
	second, _ := files[1].(map[string]any)
	if _, present := second["group"]; present {
		t.Errorf("the second target named no group and its entry carries one: %v", second["group"])
	}

	// On the raw bytes as well, because that is what somebody's script reads
	// and a key rendered as null would satisfy the check above.
	body, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"group": null`) {
		t.Error("an entry without a group renders it as null rather than leaving it out")
	}
}
