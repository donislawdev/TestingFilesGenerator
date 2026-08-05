package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
