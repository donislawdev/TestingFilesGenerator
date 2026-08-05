package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// THIRD-PARTY-NOTICES.md names a version for every module it lists, and those
// numbers were typed once from a command's output.
//
// Three of them were stale within hours of being written: the toolkit pins an
// old golang.org/x/image, govulncheck found four vulnerabilities in it that our
// code could reach, and raising it moved x/image, x/net and x/sys without the
// table noticing. Found by reading, which is the expensive way.
//
// That is the shape this project has already paid for three times - a number
// beside the code rather than derived from it, drifting quietly. The budget of
// a preset drifted by a factor of three and a half, the default entry size of
// an archive disagreed with what the tool advertised, and the index of probes
// described six of seventeen. So this asks the build what it links and compares.
//
// It checks versions rather than the set of modules. Which modules are listed
// is a licence question and belongs to a person reading each licence, and a
// guard that added rows automatically would defeat the reason the file exists.

// unlisted is every module the binary carries that the notices do not name.
//
// The standard library is not a module and does not appear in the build
// list, so nothing has to be excluded here - what comes back is third party
// code that ships.
func unlisted(body string, built map[string]string) []string {
	// Two ways of being named, because this file uses both. Most modules get
	// a row in a table. The two that carry the command line get a section of
	// their own with the whole licence text under a heading, which is a
	// stronger notice rather than a weaker one - reading only the rows called
	// them missing the first time this ran.
	named := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if m := noticeRow.FindStringSubmatch(line); m != nil {
			named[m[1]] = true
		}
		if heading, found := strings.CutPrefix(line, "## "); found {
			named[strings.TrimSpace(heading)] = true
		}
	}
	var missing []string
	for path := range built {
		if !named[path] {
			missing = append(missing, path)
		}
	}
	return missing
}

var noticeRow = regexp.MustCompile(`^\| ` + "`" + `([^` + "`" + `]+)` + "`" + ` \| (v[^ |]+) \|`)

func TestTheNoticesFileNamesTheVersionsThatAreActuallyBuilt(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Skipf("no notices file here: %v", err)
	}

	built := builtVersions(t, "../../cmd/tfg-gui")
	if len(built) < 5 {
		t.Fatalf("the build reported %d modules, too few to be the real set", len(built))
	}

	listed := 0
	for _, line := range strings.Split(string(body), "\n") {
		m := noticeRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		path, version := m[1], m[2]
		want, ok := built[path]
		if !ok {
			t.Errorf("the notices file lists %s and nothing links it any more - "+
				"a notice for code that does not ship is a notice nobody needs", path)
			continue
		}
		listed++
		if version != want {
			t.Errorf("the notices file says %s is %s and the build links %s.\n"+
				"Their licences require the notice to travel with the code that is actually "+
				"shipped, so the number has to come from the build rather than from memory.",
				path, version, want)
		}
	}
	if listed < 5 {
		t.Fatalf("only %d rows were compared, so this guard would pass on an empty table", listed)
	}

	// And the other direction, which is the one that was missing. Until
	// 2026-08-05 this only asked whether every listed module is still built,
	// so a module that ARRIVED went unlisted and unnoticed - and it was the
	// folder picker arriving that showed it. Their licences require the
	// notice to travel with the code that ships, and an absent notice fails
	// that in a way a wrong version number does not.
	if missing := unlisted(string(body), built); len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d module(s) are compiled into the window and named nowhere in the notices:\n  %s\n"+
			"Read the licence out of the module's own source, add a row, and change the counts "+
			"in the paragraph above the table.", len(missing), strings.Join(missing, "\n  "))
	}
	t.Logf("%d module version(s) in the notices agree with what the window binary links", listed)
}

// builtVersions is what a binary really links, module by module.
func builtVersions(t *testing.T, target string) map[string]string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-f",
		"{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}", target).Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	versions := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		path, version, found := strings.Cut(line, "@")
		if !found || strings.Contains(path, "donislawdev") {
			continue
		}
		versions[path] = version
	}
	return versions
}
