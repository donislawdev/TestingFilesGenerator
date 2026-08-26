package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The manifest says which Go built the binary that wrote it.
//
// D11 promises the same bytes for the same recipe within a major version, and
// the compiler is one of the things that promise rests on - this project pins a
// toolchain in go.mod and in both CI workflows precisely because the standard
// library's flate, gzip, zip and png outputs are part of the contract.
//
// Without this, a hash that moved because somebody rebuilt with a newer Go
// looks exactly like a hash that moved because the recipe changed, and the
// manifest is the one artefact that outlives the run. That is what tool.name
// and tool.version are for, and the toolchain was the piece missing from them.
// Review item S1, added on 2026-08-27 with the owner's yes.
//
// It does NOT move manifest_version, and the second half of this guard is what
// makes that a decision rather than an oversight: docs/MANIFEST.md section 10
// says the schema grows by adding fields and that a consumer ignores the ones
// it does not know. A reader written against 1.0 has to still be handed 1.0.
//
// Asked of the JSON a consumer receives rather than of the struct. Decoding
// into manifest.Tool would pass by construction the moment the type changed,
// which is the trap the manifestShape helper in recipe_test.go was written
// against.
func TestTheManifestNamesTheToolchainThatBuiltTheRun(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")

	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1", "--out", out)
	if code != 0 {
		t.Fatalf("the run ended with %d: %s", code, errOut)
	}

	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}

	var got struct {
		ManifestVersion string `json:"manifest_version"`
		Tool            struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Go      string `json:"go"`
		} `json:"tool"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the manifest does not parse: %v", err)
	}

	if got.Tool.Go == "" {
		t.Error("the manifest does not say which Go built this. A hash that moved after a rebuild " +
			"then cannot be told apart from a hash that moved because the recipe changed")
	}
	if want := runtime.Version(); got.Tool.Go != want {
		t.Errorf("the manifest says it was built by %q and this binary was built by %q", got.Tool.Go, want)
	}

	// Naming the toolchain is an added field, so the schema a consumer checks
	// has not moved. If this ever has to change, it is a major and a changelog
	// entry rather than a line edited here.
	if got.ManifestVersion != "1.0" {
		t.Errorf("manifest_version is %q. Adding a field is not a breaking change under "+
			"docs/MANIFEST.md section 10, so moving this needs its own decision",
			got.ManifestVersion)
	}
	if got.Tool.Name == "" || got.Tool.Version == "" {
		t.Errorf("the fields that were already there are empty: name=%q version=%q", got.Tool.Name, got.Tool.Version)
	}
}
