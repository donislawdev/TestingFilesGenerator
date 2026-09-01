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

	// The site renderer imports nothing of ours. It is on the bottom layer
	// because it has no reason to be anywhere else: the facts it needs are
	// handed to it by the guard that renders the site, which is external to
	// everything and already allowed to look anywhere. See internal/site.
	"internal/site": 0,

	// The licence registry imports nothing of ours either, and for the same
	// kind of reason: it is a list of facts about what we ship, read by the
	// guards, by the licence command and by whatever renders an SBOM. A list
	// that could reach into the engine would be a list that could disagree
	// with itself depending on who asked.
	"internal/legal": 0,

	// The generator of the bill of materials. A main package, so nothing can
	// import it and it cannot reach a binary by accident. It sits with the
	// other commands because it imports the registry and the version, and
	// because it is a program somebody runs rather than a library.
	"internal/legal/cmd/sbom": 5,

	"internal/format":            1,
	"internal/format/all":        1,
	"internal/format/imagelabel": 1,
	"internal/format/archive":    1,
	"internal/format/txt":        1,
	"internal/format/md":         1,
	"internal/format/logfile":    1,
	"internal/format/csvfile":    1,
	"internal/format/jsonfile":   1,
	"internal/format/xmlfile":    1,
	"internal/format/htmlfile":   1,
	"internal/format/svgfile":    1,
	"internal/format/bmp":        1,
	"internal/format/docx":       1,
	"internal/format/opc":        1,
	"internal/format/pptx":       1,
	"internal/format/xlsx":       1,
	"internal/format/gif":        1,
	"internal/format/ico":        1,
	"internal/format/jpg":        1,
	"internal/format/png":        1,
	"internal/format/pdf":        1,
	"internal/format/zip":        1,
	"internal/format/targz":      1,
	"internal/format/tiff":       1,
	"internal/format/webp":       1,
	"internal/format/avif":       1,
	"internal/format/jxl":        1,
	"internal/format/wav":        1,

	"internal/recipe":   2,
	"internal/preset":   2,
	"internal/manifest": 2,

	"internal/engine": 3,
	"internal/audit":  3,

	"internal/cli":        4,
	"internal/gui":        4,
	"internal/gui/parts":  4,
	"internal/gui/icon":   4,
	"internal/gui/text":   4,
	"internal/gui/window": 4,

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
		"internal/format/md",
		"internal/format/logfile",
		"internal/format/csvfile",
		"internal/format/jsonfile",
		"internal/format/xmlfile",
		"internal/format/htmlfile",
		"internal/format/svgfile",
		"internal/format/bmp",
		"internal/format/docx",
		"internal/format/pptx",
		"internal/format/xlsx",
		"internal/format/gif",
		"internal/format/ico",
		"internal/format/jpg",
		"internal/format/png",
		"internal/format/pdf",
		"internal/format/zip",
		"internal/format/targz",
		"internal/format/tiff",
		"internal/format/webp",
		"internal/format/avif",
		"internal/format/jxl",
		"internal/format/wav",
	},
	"internal/format/imagelabel": {"internal/format"},
	"internal/format/archive":    {"internal/format"},
	"internal/format/txt":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/md":         {"internal/format"},
	"internal/format/logfile":    {"internal/format"},
	"internal/format/csvfile":    {"internal/format"},
	"internal/format/jsonfile":   {"internal/format"},
	"internal/format/xmlfile":    {"internal/format"},
	"internal/format/htmlfile":   {"internal/format"},
	"internal/format/svgfile":    {"internal/format"},
	"internal/format/bmp":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/opc":        {"internal/format"},
	"internal/format/docx":       {"internal/format", "internal/format/opc"},
	"internal/format/xlsx":       {"internal/format", "internal/format/opc"},
	"internal/format/pptx":       {"internal/format", "internal/format/opc"},
	"internal/format/gif":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/ico":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/jpg":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/png":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/pdf":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/zip":        {"internal/format", "internal/format/imagelabel", "internal/format/archive"},
	"internal/format/targz":      {"internal/format", "internal/format/archive"},
	"internal/format/tiff":       {"internal/format", "internal/format/imagelabel"},
	"internal/format/webp":       {"internal/format", "internal/format/imagelabel"},
	"internal/format/avif":       {"internal/format", "internal/format/imagelabel"},
	"internal/format/jxl":        {"internal/format", "internal/format/imagelabel"},
	"internal/format/wav":        {"internal/format", "internal/format/imagelabel"},
	"internal/preset":            {"internal/recipe"},

	// The window is composed of parts and the parts know nothing about
	// windows. That direction is what lets a part be rendered on its own,
	// which is where the golden images sit - an image of a whole screen
	// changes with every layout change and stops being looked at.
	"internal/gui/window": {"internal/gui/parts", "internal/gui/text"},
	// The sentences the window shows. It imports nothing of ours - a text
	// package that reached for the engine to word a message would put half a
	// message here and half where the engine says it, which is how two
	// wordings for one thing start.
	"internal/gui/parts": {"internal/gui/text"},
	// And the package that opens a real window reaches all three. It is the
	// only one that touches the toolkit's app package, so it is the only one
	// that needs a C compiler.
	//
	// The text package joined that list on 2026-08-13, when the two sentences
	// this package says were moved into it: the title on the window itself, and
	// what the build without C support prints instead of opening one. They had
	// been literals here since there was a window, and no guard could see them
	// - the one that watches for words outside the text package worked from a
	// list of the calls that show text, and nobody had put the toolkit's
	// NewWindow on it. The rule is the other way round now.
	"internal/gui": {"internal/gui/window", "internal/gui/parts", "internal/gui/text", "internal/gui/icon"},
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
