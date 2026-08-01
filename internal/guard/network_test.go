package guard

import (
	"strings"
	"testing"
)

// The tool works fully offline. No telemetry, no update checks, no cloud
// clients, nothing downloaded while it runs.
//
// This guard works on the import graph, so it also catches whatever a
// dependency drags in behind it. What it does not do is prove that no traffic
// leaves the machine - that needs a traffic monitor, and that requirement
// stands. See docs/PRODUCT.md section 9.
func networkBanned(rel string) bool {
	switch rel {
	case "internal/gui", "cmd/tfg-gui":
		// The graphics toolkit brings its own dependency tree that we do not
		// control. Putting the window under this rule risks a red build for a
		// reason we cannot fix, and the usual answer to that is dropping the
		// rule everywhere. Isolation of the window rides on the traffic
		// monitor instead.
		return false
	}
	if l, ok := layer[rel]; ok {
		return l <= 3 || rel == "internal/cli" || rel == "cmd/tfg"
	}
	return false
}

// execBanned marks packages that must not start external processes. Layer 3
// gets a narrow exception on the day the system encoder arrives, and that
// exception will name one package rather than a whole layer.
func execBanned(rel string) bool {
	l, ok := layer[rel]
	return ok && l <= 2
}

func TestNoNetworkImports(t *testing.T) {
	checked := 0
	for _, p := range packages(t) {
		if !networkBanned(p.rel) {
			continue
		}
		checked++
		for _, imp := range rawImports(t, p) {
			if imp == "net" || strings.HasPrefix(imp, "net/") {
				t.Errorf("%s imports %q - the tool makes no network calls", p.rel, imp)
			}
			if strings.HasPrefix(imp, "golang.org/x/net") {
				t.Errorf("%s imports %q - the tool makes no network calls", p.rel, imp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no package was examined - this guard would pass without checking anything")
	}
}

func TestNoProcessExecutionInLowerLayers(t *testing.T) {
	checked := 0
	for _, p := range packages(t) {
		if !execBanned(p.rel) {
			continue
		}
		checked++
		for _, imp := range rawImports(t, p) {
			if imp == "os/exec" {
				t.Errorf("%s imports os/exec - layers 0 to 2 start no external processes", p.rel)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no package was examined - this guard would pass without checking anything")
	}
}
