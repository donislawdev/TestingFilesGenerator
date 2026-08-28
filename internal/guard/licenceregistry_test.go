package guard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// internal/legal is the reviewed list of what we ship, and a list nobody checks
// is a list that describes last month.
//
// The notices file has been held to the build since 2026-08-05 and that is not
// enough on its own: it is prose, so nothing can read a licence out of it, and
// the SBOM and the licence command both need one in a form a program can use.
// This is the same fact written where a program can reach it, so this guard
// asks the build the same question the notices guard asks, and then asks the
// two written answers to agree.
//
// "std" is here and comes back from no build, deliberately. The Go runtime is
// in every binary this project produces and is not a module, so no list of
// modules will ever name it - which is exactly why it would go missing.
const runtimeEntry = "std"

func TestEveryModuleTheBinariesLinkHasAReviewedLicence(t *testing.T) {
	built, _ := shipped(t, "../../cmd/tfg-gui")
	fromCLI, _ := shipped(t, "../../cmd/tfg")
	for path, version := range fromCLI {
		built[path] = version
	}
	if len(built) < 5 {
		t.Fatalf("the build reported %d modules, too few to be the real set", len(built))
	}

	registry := map[string]legal.Module{}
	for _, m := range legal.Modules() {
		if _, twice := registry[m.Path]; twice {
			t.Errorf("%s has two entries in the registry, so one of them is describing nothing", m.Path)
		}
		registry[m.Path] = m
	}

	var missing []string
	for path := range built {
		if _, ok := registry[path]; !ok {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d module(s) are compiled in and have no entry in internal/legal:\n  %s\n"+
			"Read the licence out of the module's own source and add it. An SBOM generated from "+
			"this list would otherwise claim to be complete while missing them.",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	for path := range registry {
		if path == runtimeEntry {
			continue
		}
		if _, ok := built[path]; !ok {
			stale = append(stale, path)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("%d entr(y/ies) in internal/legal name a module no binary links any more:\n  %s\n"+
			"A notice for code that does not ship is a notice nobody needs.",
			len(stale), strings.Join(stale, "\n  "))
	}
	t.Logf("%d module(s) linked, %d entries in the registry including %s",
		len(built), len(legal.Modules()), runtimeEntry)
}

// Every field that carries a licence question has to say something. An empty
// licence reads like an answer and is not one, and an empty copyright line is
// either a mistake or a fact that needs a sentence - one module here genuinely
// has none, because its licence file is the unmodified Apache template.
func TestNoEntryInTheRegistryIsSilentAboutItsLicence(t *testing.T) {
	for _, m := range legal.Modules() {
		if m.SPDX == "" {
			t.Errorf("%s has no licence identifier in the registry", m.Path)
		}
		if m.Copyright == "" && m.Note == "" {
			t.Errorf("%s has no copyright line and no note saying why.\n"+
				"A blank reads like an oversight, so the reason belongs beside it.", m.Path)
		}
	}
}

// The registry and the notices are two written copies of one fact, and this is
// what makes that safe: the licence in the table and the licence in the code
// have to be the same string. Written the other way round - a document nobody
// compares with the code - is how the version of golang.org/x/text sat at
// 0.40.0 in the notices while every binary linked 0.41.0.
func TestTheRegistryAndTheNoticesAgreeOnEveryLicence(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Skipf("no notices file here: %v", err)
	}

	registry := map[string]string{}
	for _, m := range legal.Modules() {
		registry[m.Path] = m.SPDX
	}

	compared := 0
	for _, line := range strings.Split(string(body), "\n") {
		m := licenceRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		path, licence := m[1], strings.TrimSpace(m[3])
		want, known := registry[path]
		if !known {
			continue // the other guard reports this, and reporting it twice buries it
		}
		compared++
		if licence != want {
			t.Errorf("the notices say %s is %s and internal/legal says %s.\n"+
				"One of them is wrong, and the SBOM will publish whichever the code says.",
				path, licence, want)
		}
	}
	if compared < 20 {
		t.Fatalf("only %d row(s) were compared, so this guard would pass on an empty table", compared)
	}
	t.Logf("%d licence(s) agree between the notices table and internal/legal", compared)
}
