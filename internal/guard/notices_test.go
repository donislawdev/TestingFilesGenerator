package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

	built, on := shipped(t, "../../cmd/tfg-gui")
	if len(built) < 5 {
		t.Fatalf("the build reported %d modules, too few to be the real set", len(built))
	}
	t.Logf("%d module(s) across windows, linux and darwin", len(built))

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
		named := make([]string, 0, len(missing))
		for _, path := range missing {
			// Which systems ship it, because the answer decides how hard it is
			// to notice by hand: a module built everywhere is one somebody
			// meets, and a module built on one system is one that stayed
			// invisible for a week.
			named = append(named, path+"  (on "+on[path]+")")
		}
		t.Errorf("%d module(s) are compiled into the window and named nowhere in the notices:\n  %s\n"+
			"Read the licence out of the module's own source, add a row, and change the counts "+
			"in the paragraph above the table.", len(missing), strings.Join(named, "\n  "))
	}
	t.Logf("%d module version(s) in the notices agree with what the window binary links", listed)
}

// The paragraph above that table counts the licences, and until 2026-08-10
// nothing compared those counts with the rows they describe. The error above
// says "change the counts in the paragraph" out loud, which is the shape this
// project has a name for - a step a person has to remember. It was not
// remembered: the paragraph said twelve BSD 3-Clause and nine MIT while the
// table held thirteen and ten, and two internal documents copied the wrong
// pair onwards.
//
// This matters more than an ordinary stale number because the file is the one
// artefact here that exists to satisfy somebody else's licence. A reader who
// counts the rows and finds the sentence disagreeing has no way to tell which
// half is the mistake.

// numberWord reads the counts the way the paragraph writes them. They are
// words rather than digits because that paragraph is prose a person reads, so
// a guard that only understood digits would not be reading the sentence that
// is actually there.
var numberWord = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16,
	"seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
	"twenty-one": 21, "twenty-two": 22, "twenty-three": 23, "twenty-four": 24,
	"twenty-five": 25, "twenty-six": 26, "twenty-seven": 27,
	"twenty-eight": 28, "twenty-nine": 29, "thirty": 30,
}

// licenceRow reads a row of the window table: module, version, licence.
var licenceRow = regexp.MustCompile(
	`^\|\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|`)

// moduleCount is the sentence that opens the section.
var moduleCount = regexp.MustCompile(`These (\d+) modules`)

// windowOnlyTable splits the section describing the window binary into the
// prose that counts things and the rows that are the things counted.
func windowOnlyTable(body string) (prose, rows string) {
	_, after, found := strings.Cut(body, "## The window binary only")
	if !found {
		return "", ""
	}
	if end := strings.Index(after, "\n## "); end >= 0 {
		after = after[:end]
	}
	head, table, found := strings.Cut(after, "| module |")
	if !found {
		return after, ""
	}
	return head, table
}

// spokenCount finds how many of one licence the prose claims. The table writes
// BSD-3-Clause and the sentence writes BSD 3-Clause, so the separators are
// treated as the same character - insisting on one spelling would make this
// fail on the wording rather than on the count.
func spokenCount(prose, licence string) (int, bool) {
	loose := regexp.QuoteMeta(licence)
	loose = strings.ReplaceAll(loose, `-`, `[- ]`)
	pattern := regexp.MustCompile(`([a-z]+(?:-[a-z]+)?)\s+` + loose + `\b`)
	m := pattern.FindStringSubmatch(prose)
	if m == nil {
		return 0, false
	}
	n, ok := numberWord[m[1]]
	return n, ok
}

func TestTheNoticesCountsAgreeWithTheTableTheyDescribe(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Skipf("no notices file here: %v", err)
	}
	prose, rows := windowOnlyTable(string(body))
	if rows == "" {
		t.Fatal("the notices file has no window binary table, so there was nothing to count")
	}

	tally := map[string]int{}
	total := 0
	for _, line := range strings.Split(rows, "\n") {
		m := licenceRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		tally[strings.TrimSpace(m[3])]++
		total++
	}
	if total < 5 {
		t.Fatalf("only %d row(s) were counted, so this guard would pass on an empty table", total)
	}

	if m := moduleCount.FindStringSubmatch(prose); m == nil {
		t.Error("the section does not say how many modules it lists, so the number cannot be checked")
	} else if said, _ := strconv.Atoi(m[1]); said != total {
		t.Errorf("the notices say %d modules ship in the window and the table holds %d rows.\n"+
			"One of the two is wrong and a reader counting the rows cannot tell which.", said, total)
	}

	var licences []string
	for licence := range tally {
		licences = append(licences, licence)
	}
	sort.Strings(licences)
	for _, licence := range licences {
		said, ok := spokenCount(prose, licence)
		if !ok {
			t.Errorf("the table has %d module(s) under %s and the paragraph above it does not count them.\n"+
				"A licence that ships uncounted is the direction that went unnoticed before.", tally[licence], licence)
			continue
		}
		if said != tally[licence] {
			t.Errorf("the paragraph says %d module(s) are %s and the table holds %d.",
				said, licence, tally[licence])
		}
	}
	t.Logf("%d module(s) in the window table, counted correctly by licence in %d kind(s)", total, len(tally))
}

// builtVersions is what a binary really links, module by module.
// shipped is every module that ends up in a binary somebody is handed, on any
// of the systems this project builds for.
//
// Across the three rather than on the one the guard happens to run on, and that
// is a correction made on 2026-08-13 after the guard missed a module for a
// week. github.com/rymdport/portal is linked into the window on Linux and on
// no other system, so a listing taken on Windows does not name it - and this
// guard ran on Windows, said every listed module is built and every built
// module is listed, and was right about the wrong set. The defect showed up as
// a red guard under WSL2, which is to say by accident.
//
// The notices file travels with binaries for every system. So the question it
// answers has to be asked for every system, and the platform is carried
// alongside so that a message can say where a module comes from.
func shipped(t *testing.T, target string) (map[string]string, map[string]string) {
	t.Helper()
	versions := map[string]string{}
	where := map[string]string{}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		for path, version := range builtVersions(t, target, goos) {
			if seen, ok := versions[path]; ok && seen != version {
				t.Errorf("%s is %s on %s and %s elsewhere, so one binary ships a version the "+
					"notices cannot both describe", path, version, goos, seen)
			}
			versions[path] = version
			if where[path] == "" {
				where[path] = goos
			} else if !strings.Contains(where[path], goos) {
				where[path] += ", " + goos
			}
		}
	}
	return versions, where
}

// builtVersions asks the compiler what one platform's binary links.
//
// CGO_ENABLED is set explicitly rather than inherited. The window's toolkit
// hides its real dependencies behind cgo build constraints, so a shell with
// CGO_ENABLED=0 reports almost nothing - which is a known noise on this machine
// and would turn this guard into one that passes by seeing nothing.
func builtVersions(t *testing.T, target, goos string) map[string]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f",
		"{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}", target)
	cmd.Env = append(os.Environ(), "GOOS="+goos, "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list for %s is not available here: %v", goos, err)
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
