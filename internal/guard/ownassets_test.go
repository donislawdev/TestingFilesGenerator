package guard

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// Bytes that a package of THIS project embeds and somebody else wrote.
//
// Until 2026-09-15 every entry in the licence registry came from inside some
// other module, so every question about what a binary carries was a question
// about modules: a font travels with the package that embeds it, and a build's
// own record names the modules it links. That day the window was set in Inter
// (internal/gui/font), and the question stopped having an answer for that
// entry - both binaries are one module, so "is the module linked" is yes for
// the command line binary too, and it carries no font at all.
//
// So an entry of ours is answered by PACKAGE instead: go list when a document
// is rendered, and the package's own init (legal.Carrying) when a running
// binary is asked. The guards here hold the three halves of that to the build:
// the spelling the registry uses is the spelling go list uses, the command
// line links none of these packages, and every such package does announce
// itself.

// The registry spells our module and our packages by hand, and the whole
// mechanism compares those spellings with what go list reports. A typo would
// not be an error anywhere - it would be a font quietly dropping off every
// list, which is the failure this registry exists to prevent.
func TestEveryAssetOfOurOwnIsSpelledAsGoListSpellsIt(t *testing.T) {
	out, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != legal.OurModule() {
		t.Errorf("the registry calls this module %q and go list calls it %q", legal.OurModule(), got)
	}
	for _, asset := range ours(t) {
		out, err := exec.Command("go", "list", "-f", "{{.ImportPath}}", asset.Package).Output()
		if err != nil {
			t.Errorf("%s names the package %s, which go list cannot find: %v", asset.Name, asset.Package, err)
			continue
		}
		if got := strings.TrimSpace(string(out)); got != asset.Package {
			t.Errorf("%s names the package %q and go list spells it %q", asset.Name, asset.Package, got)
		}
	}
}

// The command line binary has no toolkit and no font, and the notices say so
// in as many words. Nothing structural keeps it that way: a package of ours
// that embeds a font is a package any other package may import, and the day
// one on the command line's side does, "tfg license" would go on saying
// nothing about it - the runtime answer comes from the package's own init,
// which would then run there too, but the sentence in the notices would be
// wrong and this is the guard that would say so first.
func TestTheCommandLineLinksNoBytesOfSomebodyElsesThatWeEmbed(t *testing.T) {
	linked := ourPackagesLinkedBy(t, "../../cmd/tfg", false)
	for _, asset := range ours(t) {
		if linked[asset.Package] {
			t.Errorf("the command line binary links %s, which embeds %s - the notices say tfg carries "+
				"none of this, and tfg license would have to start saying otherwise", asset.Package, asset.Name)
		}
	}

	// And the built binary, asked the way a user asks it. The line for an
	// embedded entry starts with its name and two spaces (legal.Item.Line).
	binary := buildCommandLine(t)
	out, err := exec.Command(binary, "license").Output()
	if err != nil {
		t.Fatalf("running the licence command: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, asset := range ours(t) {
			if strings.HasPrefix(strings.TrimSpace(line), asset.Name+"  ") {
				t.Errorf("tfg license names %q, and the command line binary carries no such bytes", asset.Name)
			}
		}
	}
}

// A running binary learns which of our packages it holds from the packages
// themselves, at start-up. A package that forgot to say so would ship its
// bytes and drop off the licence command without a word - the one failure a
// registry cannot notice about itself.
//
// Asked in this binary, which links the window's parts and therefore every
// package the window embeds through. A package this binary does not link
// cannot be measured here and is reported rather than skipped: a blank import
// in this file is the remedy, and a skip is how a guard passes on nothing.
func TestEveryPackageOfOursThatEmbedsSomebodyElsesBytesAnnouncesItself(t *testing.T) {
	linked := ourPackagesLinkedBy(t, ".", true)
	for _, asset := range ours(t) {
		if !linked[asset.Package] {
			t.Errorf("this guard binary does not link %s, so it cannot ask whether the package "+
				"announces %s - import it here so it can", asset.Package, asset.Name)
			continue
		}
		if !legal.LinkedHere(asset.Package) {
			t.Errorf("%s is linked into this binary and never called legal.Carrying - %s would ship "+
				"and be named on no list a running binary prints", asset.Package, asset.Name)
		}
	}
}

// ours is every registry entry embedded by a package of this project. At
// least one, or the guards above are proving things about an empty set.
func ours(t *testing.T) []legal.Asset {
	t.Helper()
	var out []legal.Asset
	for _, asset := range legal.Assets() {
		if asset.Ours() {
			out = append(out, asset)
		}
	}
	if len(out) == 0 {
		t.Fatal("the registry holds no entry embedded by a package of ours, and these guards exist for those")
	}
	return out
}

// ourPackagesLinkedBy asks the compiler which packages of this module one
// target links, on every system a release is built for. CGO is on for the
// reason written beside the notices guard: with it off the window's toolkit
// hides its dependencies and the answer is a stub.
//
// withTests asks about the target's TEST binary rather than the package: the
// guard package itself is one doc.go, and everything it links it links from
// its test files.
func ourPackagesLinkedBy(t *testing.T, target string, withTests bool) map[string]bool {
	t.Helper()
	args := []string{"list", "-deps"}
	if withTests {
		args = append(args, "-test")
	}
	args = append(args, "-f", "{{if .Module}}{{.Module.Path}}|{{.ImportPath}}{{end}}", target)
	linked := map[string]bool{}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		cmd := exec.Command("go", args...)
		cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=1")
		out, err := cmd.Output()
		if err != nil {
			t.Skipf("go list for %s is not available here: %v", goos, err)
		}
		for _, line := range strings.Split(string(out), "\n") {
			module, pkg, found := strings.Cut(strings.TrimSpace(line), "|")
			if found && module == legal.OurModule() {
				linked[pkg] = true
			}
		}
	}
	if len(linked) == 0 {
		t.Fatalf("%s links no package of this module at all, which cannot be right", target)
	}
	return linked
}
