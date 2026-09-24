package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// A module list cannot see a font.
//
// Measured on 2026-08-27: the window binary carries seven font files and
// ninety-seven images that arrive from inside fyne.io/fyne/v2, under the Open
// Font License, the Bitstream Vera licence and MIT - and the notices named none
// of them. Nothing could have caught it. Every mechanism this project had asked
// about MODULES: go list -deps reports modules, the dependency gate reviews the
// module graph a pull request adds, a scanner reads the modules recorded in the
// binary, and the notices guard compares that list with a table. A font is not
// a module. It is a file inside one, with a licence of its own, and no question
// about modules reaches it. See docs/OBSERVATIONS.md, O150.
//
// So this asks a different question, and asks the compiler rather than the
// source: which files does each package compile into the binary. The first
// version of that finding read one source file and missed a font, which is the
// second reason this guard exists rather than a careful reader.
func TestEveryFileEmbeddedFromSomebodyElseIsAccountedFor(t *testing.T) {
	seen := map[string]bool{}
	matched := map[string]bool{}
	total := 0
	// Every key of ownWork has to be spent on a file some build embeds, or it
	// is an exemption that outlived its file - and an exemption nobody uses
	// today is the one that exempts somebody else's bytes tomorrow, at the
	// same package and path, without a word. Asked the same way the registry
	// is asked below, since an unused entry and an unused exemption are the
	// same drift facing two ways. An outside review of the pull request
	// pointed at the missing half on 2026-09-16.
	usedExemptions := map[string]bool{}

	for _, target := range []string{"../../cmd/tfg-gui", "../../cmd/tfg"} {
		for _, goos := range []string{"windows", "linux", "darwin"} {
			total += accountForBuild(t, target, goos, seen, matched, usedExemptions)
		}
	}

	// The toolkit alone embeds 133 files, so a run that finds a handful found
	// a shell with cgo switched off rather than a tree with nothing in it.
	// Without this the guard would pass by seeing nothing, which is the failure
	// it is here to prevent.
	if total < 50 {
		t.Fatalf("only %d embedded file(s) were found across both binaries and three systems, "+
			"which is too few to be the real set - check that cgo is enabled in this shell", total)
	}

	var stale []string
	for _, asset := range legal.Assets() {
		if !matched[asset.Name] {
			stale = append(stale, asset.Name+" (declared in "+asset.Package+")")
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("%d entr(y/ies) in the licence registry match nothing any build embeds:\n  %s\n"+
			"An entry for bytes that no longer ship is a notice nobody needs, and it hides the day "+
			"the real thing was replaced by something else.", len(stale), strings.Join(stale, "\n  "))
	}
	var unspent []string
	for key := range ownWork {
		if !usedExemptions[key] {
			unspent = append(unspent, key)
		}
	}
	if len(unspent) > 0 {
		sort.Strings(unspent)
		t.Errorf("%d ownWork exemption(s) name a file no build embeds:\n  %s\n"+
			"An exemption without a file is a licence check switched off for whatever lands at that "+
			"path next. Take it off the list.", len(unspent), strings.Join(unspent, "\n  "))
	}
	t.Logf("%d embedded file(s) across every package, all accounted for by %d registry entr(y/ies) or named as our own work",
		total, len(legal.Assets()))
}

// accountForBuild walks one platform's build and returns how many files it
// added. Split out rather than nested inside the test because the depth ceiling
// said so, and the ceiling is a measurement rather than a preference - see
// docs/QUALITY.md.
func accountForBuild(t *testing.T, target, goos string, seen, matched, usedExemptions map[string]bool) int {
	t.Helper()
	added := 0
	for pkg, files := range embeddedFiles(t, target, goos) {
		added += accountForPackage(t, pkg, files, seen, matched, usedExemptions)
	}
	return added
}

// accountForPackage does one package, skipping what another platform already
// answered for. Three systems are asked and they agree about most of the tree,
// so without seen the same font would be reported six times.
func accountForPackage(t *testing.T, pkg string, files []string, seen, matched, usedExemptions map[string]bool) int {
	t.Helper()
	added := 0
	for _, file := range files {
		if seen[pkg+" "+file] {
			continue
		}
		seen[pkg+" "+file] = true
		added++
		if ownWork[pkg+" "+file] {
			usedExemptions[pkg+" "+file] = true
			continue
		}
		accountFor(t, pkg, file, matched)
	}
	return added
}

// ownWork is every file a package of THIS module embeds that this project
// drew or wrote itself, so that no licence but our own applies to it.
//
// Until 2026-09-15 the walk skipped our module altogether, on the reasoning
// that what we embed is our own work. That day the window took a font of
// somebody else's (Inter, internal/gui/font) and the reasoning stopped being
// true - and a skip would have let those bytes ship with a registry entry
// that matched nothing, which the stale check below would have reported as
// the ENTRY being wrong. So the module is walked like any other, and the
// exceptions are named here, one by one, with the reason each is ours.
//
// A file added to any of our packages that is on neither list makes this
// guard red, which is the direction it should fail in: the person adding it
// says whose it is, rather than a guard assuming.
var ownWork = map[string]bool{
	// Drawn from shapes by tools/appicon.py. docs/LICENSING.md.
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/icon chickpea.png": true,
	// Drawn from two circles and a point, the geometry written beside it in
	// heart.go. The owner's choice over an icon set's heart. docs/LICENSING.md.
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts heart.svg": true,
	// The window's own words.
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text locale/en.json": true,
}

// accountFor requires exactly one registry entry to claim a file. None means
// bytes ship unnamed. Two means the notices could say two different licences
// for one file and nothing would notice which one a reader believed.
func accountFor(t *testing.T, pkg, file string, matched map[string]bool) {
	t.Helper()
	var claims []string
	for _, asset := range legal.Assets() {
		if asset.Package == pkg && asset.Covers(file) {
			claims = append(claims, asset.Name)
			matched[asset.Name] = true
		}
	}
	switch len(claims) {
	case 1:
	case 0:
		t.Errorf("%s embeds %s into a binary and no entry in internal/legal accounts for it.\n"+
			"Read the licence that ships with it, add an entry, and put it in THIRD-PARTY-NOTICES.md. "+
			"Bytes that travel with a release and are named nowhere are the thing this guard exists for.",
			pkg, file)
	default:
		sort.Strings(claims)
		t.Errorf("%s embeds %s and %d registry entries claim it: %s.\n"+
			"One file, one licence, one entry - otherwise the notices can carry two answers "+
			"and nobody can tell which one is true.", pkg, file, len(claims), strings.Join(claims, ", "))
	}
}

// The notices are what actually travels with a release, so an entry that is
// only in the code is an entry nobody downloading a binary can read.
func TestEveryEmbeddedAssetIsNamedInTheNotices(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Skipf("no notices file here: %v", err)
	}
	text := string(body)
	for _, asset := range legal.Assets() {
		if !strings.Contains(text, asset.Name) {
			t.Errorf("the registry carries %q and the notices never name it - "+
				"the notices are the copy that travels with the binary", asset.Name)
		}
		if !strings.Contains(text, asset.SPDX) {
			t.Errorf("%s ships under %s and the notices never state that licence", asset.Name, asset.SPDX)
		}
		if asset.Copyright == "" {
			t.Errorf("%s has no copyright line, and every licence here requires one to travel", asset.Name)
		}
	}
}

// embeddedFiles asks one platform's build which files each package embeds.
//
// CGO_ENABLED is set rather than inherited, for the reason written beside the
// notices guard: the toolkit hides its real dependencies behind cgo build
// constraints, so a shell with cgo off reports a tree with almost nothing in
// it. Our own module is walked with the rest since 2026-09-15 - see ownWork
// for what changed and why.
func embeddedFiles(t *testing.T, target, goos string) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f",
		"{{if and .EmbedFiles .Module}}{{.Module.Path}}|{{.ImportPath}}|{{range .EmbedFiles}}{{.}} {{end}}{{end}}",
		target)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list for %s is not available here: %v", goos, err)
	}
	found := map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(parts) != 3 {
			continue
		}
		found[parts[1]] = strings.Fields(parts[2])
	}
	return found
}
