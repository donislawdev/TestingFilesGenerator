package guard

import (
	"strings"
	"testing"
)

// Layers, straight from docs/ARCHITECTURE.md section 2. A package may import
// packages of a strictly lower layer and nothing else.
//
// Nothing points upwards. The engine must not learn about the command line,
// and the command line binary must not learn about the window.
var layer = map[string]int{
	"internal/version": 0,
	"internal/core":    0,

	"internal/format":            1,
	"internal/format/all":        1,
	"internal/format/imagelabel": 1,
	"internal/format/txt":        1,
	"internal/format/png":        1,
	"internal/format/pdf":        1,
	"internal/format/zip":        1,
	"internal/format/wav":        1,

	"internal/recipe":   2,
	"internal/preset":   2,
	"internal/manifest": 2,

	"internal/engine": 3,
	"internal/audit":  3,

	"internal/cli": 4,
	"internal/gui": 4,

	"cmd/tfg":     5,
	"cmd/tfg-gui": 5,
}

// Same layer edges that are intended. Everything else inside one layer is a
// violation, which is what keeps cli and gui apart.
var sameLayerAllowed = map[string][]string{
	// The registration package exists to pull every format in, so it is the
	// one place allowed to reach sideways across the whole layer.
	"internal/format/all": {
		"internal/format",
		"internal/format/txt",
		"internal/format/png",
		"internal/format/pdf",
		"internal/format/zip",
		"internal/format/wav",
	},
	"internal/format/imagelabel": {"internal/format"},
	"internal/format/txt":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/png":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/pdf":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/zip":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/wav":        {"internal/format", "internal/format/imagelabel"},
	"internal/preset":            {"internal/recipe"},
}

// Edges that a plain layer number would allow but that must never exist.
var denied = map[string][]string{
	// Importing the window would pull in the graphics toolkit and with it
	// CGO, and the command line binary would stop cross compiling.
	"cmd/tfg": {"internal/gui"},
}

// testOnly packages sit outside the ladder. Only tests import them.
var testOnly = map[string]bool{
	"internal/guard":  true,
	"internal/oracle": true,
}

func TestLayeringHoldsForEveryPackage(t *testing.T) {
	pkgs := packages(t)

	// Every package is either placed on the ladder or explicitly test only.
	// A new package that is neither fails here rather than escaping the rule
	// by being forgotten.
	for _, p := range pkgs {
		if p.rel == "" {
			continue
		}
		if _, ok := layer[p.rel]; !ok && !testOnly[p.rel] {
			t.Errorf("package %s has no layer - add it to docs/ARCHITECTURE.md section 2 and to this map", p.rel)
		}
	}

	checked := 0
	for _, p := range pkgs {
		from, ok := layer[p.rel]
		if !ok {
			continue
		}
		for _, imp := range p.imports {
			to, ok := layer[imp]
			if !ok {
				if testOnly[imp] {
					t.Errorf("%s imports %s in non test code - that package is for tests only", p.rel, imp)
				}
				continue
			}
			checked++

			if isDenied(p.rel, imp) {
				t.Errorf("%s must never import %s", p.rel, imp)
				continue
			}
			if to < from {
				continue
			}
			if to == from && isSameLayerAllowed(p.rel, imp) {
				continue
			}
			if to == from {
				t.Errorf("%s imports %s - same layer %d and not an allowed edge", p.rel, imp, from)
				continue
			}
			t.Errorf("%s (layer %d) imports %s (layer %d) - nothing points upwards", p.rel, from, imp, to)
		}
	}

	if checked == 0 {
		t.Fatal("no internal import was examined - this guard would pass without checking anything")
	}
}

// TestOracleStaysOutOfProductionCode keeps the reference tool wrappers where
// they belong. They shell out to 7z and ffprobe, which has no business in a
// shipped binary.
func TestOracleStaysOutOfProductionCode(t *testing.T) {
	for _, p := range packages(t) {
		if p.rel == "internal/oracle" {
			continue
		}
		for _, imp := range p.imports {
			if imp == "internal/oracle" || strings.HasPrefix(imp, "internal/oracle/") {
				t.Errorf("%s imports %s outside of tests", p.rel, imp)
			}
		}
	}
}

func isSameLayerAllowed(from, to string) bool {
	for _, a := range sameLayerAllowed[from] {
		if a == to {
			return true
		}
	}
	return false
}

func isDenied(from, to string) bool {
	for _, d := range denied[from] {
		if d == to {
			return true
		}
	}
	return false
}
