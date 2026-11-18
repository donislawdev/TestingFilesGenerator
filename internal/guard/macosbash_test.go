package guard

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestAJobOnMacOSUsesOnlyWhatItsBashHas reads every job of every workflow that
// runs on macOS, and the scripts and local actions its steps name, for what
// the bash of a macOS runner cannot run.
//
// A macOS runner's bash is 3.2, and what bash 4 brought works on the two
// other systems of a matrix and ends the step 127 on the third, which nobody
// writing on Windows or Linux sees. mapfile ran on the Linux and Windows
// runners and ended the step of the test job that runs the other packages 127
// on macOS (2026-11-18), with the trap already written down in
// docs/MACOS-SIGNING.md section 11.
//
// A job runs on macOS when its text, comments aside, names a macos- runner,
// in runs-on or in its matrix. A job that names one and keeps a step off it
// with an if is counted too, which only asks more of it.
func TestAJobOnMacOSUsesOnlyWhatItsBashHas(t *testing.T) {
	for _, c := range []struct {
		name     string
		workflow string
		files    map[string]string
		want     []string
	}{
		{"mapfile on macOS", macJob("mapfile -t others < others.txt"), nil, []string{"canary.yml mac: mapfile"}},
		{"readarray on macOS", macJob("readarray -t others < others.txt"), nil, []string{"canary.yml mac: readarray"}},
		{"an associative array", macJob("declare -A seen"), nil, []string{"canary.yml mac: an associative array"}},
		{"lower case by expansion", macJob(`echo "${name,,}"`), nil, []string{"canary.yml mac: case changed by expansion"}},
		{"the timeout command", macJob("timeout 30 ./tfg version"), nil, []string{"canary.yml mac: the timeout command"}},
		{"a script the job names", macJob("bash .github/scripts/x.sh"), map[string]string{".github/scripts/x.sh": "readarray -t a < f\n"}, []string{"canary.yml mac, .github/scripts/x.sh: readarray"}},
		{"a local action the job uses", macJob("true") + "      - uses: ./.github/actions/y\n", map[string]string{".github/actions/y/action.yml": "runs:\n  steps:\n    - run: mapfile -t a < f\n"}, []string{"canary.yml mac, .github/actions/y/action.yml: mapfile"}},
		{"the same on Linux", "on: push\njobs:\n  linux:\n    runs-on: ubuntu-latest\n    steps:\n      - run: mapfile -t a < f\n", nil, nil},
		{"what is not the construct", macJob("go test ./... -timeout 18m # mapfile here would end 127") + "    timeout-minutes: 5\n", map[string]string{}, nil},
	} {
		read := func(path string) (string, error) {
			if text, ok := c.files[path]; ok {
				return text, nil
			}
			return "", fs.ErrNotExist
		}
		faults, _, err := macOSBashFaults(map[string]string{"canary.yml": c.workflow}, read)
		if err != nil || !slices.Equal(faults, c.want) {
			t.Fatalf("canary %q: found %q (%v), want %q - the check cannot be trusted with the real workflows", c.name, faults, err, c.want)
		}
	}

	root := repoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no workflow under .github/workflows (%v)", err)
	}
	workflows := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		workflows[filepath.Base(path)] = string(data)
	}
	read := func(path string) (string, error) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		return string(data), err
	}
	faults, reached, err := macOSBashFaults(workflows, read)
	if err != nil {
		t.Fatal(err)
	}
	// The test job of ci.yml runs on macOS and hands its part to a script,
	// so a reading that missed either would be green over nothing.
	for _, want := range []string{"ci.yml test", ".github/scripts/guard_part.sh"} {
		if !slices.Contains(reached, want) {
			t.Fatalf("the check did not reach %s, and it runs on macOS. It reached %q", want, reached)
		}
	}
	if len(faults) > 0 {
		t.Fatalf("a job on macOS runs what bash 3.2 does not have:\n  %s\nWrite it for bash 3.2 - a while read loop, xargs, tr for case, gtimeout or a deadline in the program - or in Python.", strings.Join(faults, "\n  "))
	}
}

// macJob is a workflow of one job on macOS whose one step runs line.
func macJob(line string) string {
	return "on: push\njobs:\n  mac:\n    runs-on: macos-latest\n    steps:\n      - run: " + line + "\n"
}

// bash4Only is what a macOS runner's bash 3.2 does not have, each with the
// name a fault is reported by.
var bash4Only = []struct {
	name string
	is   *regexp.Regexp
}{
	{"mapfile", regexp.MustCompile(`\bmapfile\b`)},
	{"readarray", regexp.MustCompile(`\breadarray\b`)},
	{"an associative array", regexp.MustCompile(`\b(declare|local|typeset)\s+-[A-Za-z]*A`)},
	{"case changed by expansion", regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*(,,?|\^\^?)\}`)},
	// coreutils, not part of macOS. The flag -timeout of go test and the key
	// timeout-minutes are neither preceded by a space nor followed by one.
	{"the timeout command", regexp.MustCompile(`(^|[\s;&|(])timeout\s+[0-9]`)},
}

// localScript is a script of the repository a step names, and localAction a
// composite action of the repository a step uses.
var (
	localScript = regexp.MustCompile(`\.github/scripts/[A-Za-z0-9_.-]+\.sh\b`)
	localAction = regexp.MustCompile(`uses:\s*\./(\.github/actions/[A-Za-z0-9_.-]+)`)
)

// macOSBashFaults reads every job of workflows that runs on macOS, with the
// scripts and local actions it names, and returns each construct of bash4Only
// it finds, as "workflow job[, file]: name", in the order read. reached is every job and
// file it read, for the caller to ask that it reached what it has to.
func macOSBashFaults(workflows map[string]string, read func(path string) (string, error)) (faults, reached []string, err error) {
	for _, workflow := range slices.Sorted(maps.Keys(workflows)) {
		jobs := ciJobs(workflows[workflow])
		for _, job := range slices.Sorted(maps.Keys(jobs)) {
			block := jobs[job]
			if !strings.Contains(block, "macos-") {
				continue
			}
			where := workflow + " " + job
			reached = append(reached, where)
			texts := map[string]string{"": block}
			for _, path := range localScript.FindAllString(block, -1) {
				texts[path] = ""
			}
			for _, m := range localAction.FindAllStringSubmatch(block, -1) {
				texts[m[1]+"/action.yml"] = ""
			}
			for _, path := range slices.Sorted(maps.Keys(texts)) {
				text := texts[path]
				at := where
				if path != "" {
					got, err := read(path)
					if errors.Is(err, fs.ErrNotExist) && strings.HasSuffix(path, "/action.yml") {
						got, err = read(strings.TrimSuffix(path, ".yml") + ".yaml")
					}
					if err != nil {
						return nil, nil, fmt.Errorf("%s names %s, and it cannot be read: %w", where, path, err)
					}
					text, at = got, where+", "+path
					reached = append(reached, path)
				}
				text = withoutYamlComments(text)
				for _, b := range bash4Only {
					if b.is.MatchString(text) {
						faults = append(faults, at+": "+b.name)
					}
				}
			}
		}
	}
	return faults, reached, nil
}
