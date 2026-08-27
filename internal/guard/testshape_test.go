package guard

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Half this tree is guards, and until 2026-08-27 that half had no ceiling at
// all.
//
// The ceilings in codeshape_test.go read build.ImportDir(...).GoFiles, which
// excludes _test.go by construction. So the part of the repository that grows
// fastest - one guard per behaviour, and the count only goes up - was the part
// nothing was watching. Measured that day: 163 files and 877 functions, the
// largest file 176 lines OVER the ceiling production has never crossed, and
// one function at 214.
//
// The numbers are their own, not the production ones, and that is measured
// rather than lenient: guards nest to 7 where production reaches 4, and their
// worst branch count is 35 against 22. One ceiling over both sets would either
// tear production apart or watch nothing here.
//
// Frozen at today's measurement, so this cost nothing to introduce and stops
// tomorrow's growth. Same ratchet as everywhere else: down is routine, up is
// the owner's decision.
const (
	longestTestFunction = 214
	longestTestFile     = 726
	deepestTestNesting  = 7

	// The bands, as line counts rather than as a share of the ceiling. The
	// reason is written at crowdingFileLines: a band worked out from the
	// ceiling reshapes itself the moment the ceiling drops, so lowering one
	// number silently moves a second one.
	crowdingTestFileLines     = 545
	crowdingTestFunctionLines = 161

	crowdedTestFiles     = 3
	crowdedTestFunctions = 1
)

func TestNoGuardHasGrownPastWhatAPersonCanFollow(t *testing.T) {
	m := measureTestTree(t)
	if m.files == 0 {
		t.Fatal("the scan read no test file - this guard would pass against any ceiling ever set")
	}

	if m.biggestFile > longestTestFile {
		t.Errorf("%s holds %d lines of code and the ceiling is %d - split it by what the parts do",
			m.biggestFileName, m.biggestFile, longestTestFile)
	}
	if m.biggestFunc > longestTestFunction {
		t.Errorf("%s is %d lines of code and the ceiling is %d",
			m.biggestFuncName, m.biggestFunc, longestTestFunction)
	}
	if m.deepest > deepestTestNesting {
		t.Errorf("%s nests %d deep and the ceiling is %d",
			m.deepestName, m.deepest, deepestTestNesting)
	}
	if m.crowdedFiles > crowdedTestFiles {
		t.Errorf("%d test file(s) are %d lines of code or longer and the cap is %d",
			m.crowdedFiles, crowdingTestFileLines, crowdedTestFiles)
	}
	if m.crowdedFuncs > crowdedTestFunctions {
		t.Errorf("%d guard(s) are %d lines of code or longer and the cap is %d",
			m.crowdedFuncs, crowdingTestFunctionLines, crowdedTestFunctions)
	}
	t.Logf("%d test file(s): biggest %d, longest guard %d, deepest %d",
		m.files, m.biggestFile, m.biggestFunc, m.deepest)
}

// The same second half the production ceilings have: every number here has to
// BE the measurement. A ceiling parked above the truth grants headroom nobody
// decided to grant, and this set has more room to drift than any other.
func TestTheGuardCeilingsAreTodaysMeasurementAndNotALooserNumber(t *testing.T) {
	m := measureTestTree(t)
	if m.files == 0 {
		t.Fatal("the scan read no test file - this guard would pass against any number ever set")
	}

	for _, a := range []struct {
		what           string
		ceiling, found int
		where          string
	}{
		{"longestTestFile", longestTestFile, m.biggestFile, m.biggestFileName},
		{"longestTestFunction", longestTestFunction, m.biggestFunc, m.biggestFuncName},
		{"deepestTestNesting", deepestTestNesting, m.deepest, m.deepestName},
		{"crowdedTestFiles", crowdedTestFiles, m.crowdedFiles, "files in the band"},
		{"crowdedTestFunctions", crowdedTestFunctions, m.crowdedFuncs, "guards in the band"},
	} {
		if a.ceiling == a.found {
			continue
		}
		t.Errorf("%s is %d and the measurement is %d (%s) - move it to %d.\n"+
			"  A number standing above what it measures lets the next arrival in without a word.",
			a.what, a.ceiling, a.found, a.where, a.found)
	}
}

func measureTestTree(t *testing.T) shape {
	t.Helper()
	var m shape
	for _, p := range packages(t) {
		for _, path := range p.tests {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			src := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, body, parser.ParseComments)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			m.files++
			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			measureFile(&m, fset, file, src, filepath.ToSlash(rel), crowdBands{
				file:     crowdingTestFileLines,
				function: crowdingTestFunctionLines,
				// The branching axes are watched on production only. A guard
				// spelling out twenty cases is doing its job, and a ceiling
				// there would argue with the thing this package is for.
				cyclo: noBand, args: noBand, depth: noBand,
			})
		}
	}
	return m
}
