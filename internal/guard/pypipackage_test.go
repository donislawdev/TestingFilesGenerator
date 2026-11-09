package guard

import (
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The Python package (python/testing-files-generator) runs this program and
// reads what it writes, from Python, as the npm package does from
// JavaScript. Nothing in Go calls it, so nothing in Go notices when the two
// drift apart - a system the release builds and the package cannot find, a
// system the package knows that reaches PyPI untried, an outcome the program
// declares and the package refuses. These guards read the package's files
// and hold them to the release, the workflow and the manifest.
// docs/PYTEST-2026-11-09.md.

const pyPackage = "python/testing-files-generator"

// pyPlatforms reads PLATFORMS from _binary.py, such as "linux x64".
func pyPlatforms(t *testing.T) []string {
	t.Helper()
	decl := regexp.MustCompile(`(?m)^PLATFORMS = \(([^)]*)\)`).FindStringSubmatch(npmFile(t, pyPackage+"/src/testing_files_generator/_binary.py"))
	if decl == nil {
		t.Fatal("_binary.py has no PLATFORMS tuple on one line - this guard no longer reads it")
	}
	var out []string
	for _, m := range regexp.MustCompile(`"([a-z0-9]+ [a-z0-9]+)"`).FindAllStringSubmatch(decl[1], -1) {
		out = append(out, m[1])
	}
	if len(out) == 0 {
		t.Fatal("PLATFORMS in _binary.py names no system this guard can read")
	}
	sort.Strings(out)
	return out
}

func TestThePythonPackageKnowsEveryPlatformTheReleaseBuilds(t *testing.T) {
	name := map[string]string{"windows": "win32", "linux": "linux", "darwin": "darwin", "amd64": "x64", "arm64": "arm64"}
	var want []string
	for _, target := range releaseTargets(t) {
		goos, goarch, _ := strings.Cut(target, "/")
		if name[goos] == "" || name[goarch] == "" {
			t.Fatalf("the release builds %s, and this guard does not know what the package calls it", target)
		}
		want = append(want, name[goos]+" "+name[goarch])
	}
	sort.Strings(want)
	if got := pyPlatforms(t); !reflect.DeepEqual(got, want) {
		t.Errorf("the Python package looks for a binary on %v, and the release builds one for %v", got, want)
	}
}

// Every wheel with a binary is installed and used on its own system before
// anything goes to PyPI (.github/workflows/pypi.yml), the wheels for musl on
// Alpine beside those for glibc, and nothing is published until every one of
// them has. The systems tried are written out in the workflow, so they are
// held to _binary.py here: a system missing there goes to PyPI untried, and
// one the package does not know is a run that tries nothing.
func TestEverySystemThePythonPackageKnowsIsTriedBeforeAnythingIsPublished(t *testing.T) {
	workflow := npmFile(t, ".github/workflows/pypi.yml")
	try := regexp.MustCompile(`(?ms)^  try:\n(.*?)(?:^  [a-z][a-z-]*:\n|\z)`).FindStringSubmatch(workflow)
	if try == nil {
		t.Fatal("pypi.yml has no try job - this guard no longer reads it")
	}
	entries := regexp.MustCompile(`(?m)^          - name: .*\n((?:            .*\n)+)`).FindAllStringSubmatch(try[1], -1)
	glibc, musl := map[string]bool{}, map[string]bool{}
	for _, e := range entries {
		system := regexp.MustCompile(`(?m)^            system: ([a-z0-9-]+)$`).FindStringSubmatch(e[1])
		if system == nil {
			t.Errorf("an entry of the try matrix in pypi.yml names no system:\n%s", e[1])
			continue
		}
		if strings.Contains(e[1], "            alpine: ") {
			musl[system[1]] = true
		} else {
			glibc[system[1]] = true
		}
	}
	var tried, onAlpine, known, linux []string
	for s := range glibc {
		tried = append(tried, s)
	}
	for s := range musl {
		onAlpine = append(onAlpine, s)
	}
	for _, p := range pyPlatforms(t) {
		known = append(known, strings.ReplaceAll(p, " ", "-"))
		if strings.HasPrefix(p, "linux ") {
			linux = append(linux, strings.ReplaceAll(p, " ", "-"))
		}
	}
	sort.Strings(tried)
	sort.Strings(onAlpine)
	sort.Strings(known)
	sort.Strings(linux)
	if len(tried) == 0 {
		t.Fatal("read no system from the try matrix of pypi.yml - this guard no longer reads it")
	}
	if !reflect.DeepEqual(tried, known) {
		t.Errorf("pypi.yml tries the wheels of %v, and the package installs a binary for %v", tried, known)
	}
	if !reflect.DeepEqual(onAlpine, linux) {
		t.Errorf("pypi.yml tries the wheels for musl on %v, and the package has a binary for %v", onAlpine, linux)
	}

	publish := regexp.MustCompile(`(?ms)^  publish:\n(.*?)(?:^  [a-z][a-z-]*:\n|\z)`).FindStringSubmatch(workflow)
	if publish == nil {
		t.Fatal("pypi.yml has no publish job - this guard no longer reads it")
	}
	needs := regexp.MustCompile(`(?m)^    needs: \[([^\]]*)\]`).FindStringSubmatch(publish[1])
	if needs == nil || !strings.Contains(", "+needs[1]+",", " try,") {
		t.Errorf("the publish job of pypi.yml does not wait for the try job, so a wheel can reach PyPI untried: needs %v", needs)
	}
	if !strings.Contains(publish[1], "    environment:\n      name: ${{ needs.which.outputs.index }}\n") {
		t.Error("the publish job of pypi.yml does not run in the environment the owner approves, so nothing waits for the approval")
	}
}

// pyTuple reads a tuple of string literals assigned to name on one line.
func pyTuple(t *testing.T, body, file, name string) []string {
	t.Helper()
	decl := regexp.MustCompile(`(?m)^` + name + ` = \(([^)]*)\)`).FindStringSubmatch(body)
	if decl == nil {
		t.Fatalf("%s has no %s tuple on one line", file, name)
	}
	var out []string
	for _, m := range regexp.MustCompile(`"([a-z_]+)"`).FindAllStringSubmatch(decl[1], -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestThePythonPackageHoldsTheManifestsClosedLists(t *testing.T) {
	if got, want := pyTuple(t, npmFile(t, pyPackage+"/src/testing_files_generator/_testfile.py"), "_testfile.py", "OUTCOMES"), recipe.Outcomes(); !reflect.DeepEqual(got, want) {
		t.Errorf("the Python package accepts the outcomes %v, and the program declares %v", got, want)
	}

	plugin := npmFile(t, pyPackage+"/src/testing_files_generator/pytest_plugin.py")
	var marked []string
	for _, m := range regexp.MustCompile(`(?m)^    "([a-z]+)": "files `).FindAllStringSubmatch(plugin, -1) {
		marked = append(marked, m[1])
	}
	sort.Strings(marked)
	if want := recipe.Outcomes(); !reflect.DeepEqual(marked, want) {
		t.Errorf("the pytest plugin registers a marker for the outcomes %v, and the program declares %v", marked, want)
	}

	readme := npmFile(t, pyPackage+"/README.md")
	sentence := regexp.MustCompile(`The reasons a file can name are ([^.]*)\.`).FindStringSubmatch(readme)
	if sentence == nil {
		t.Fatal("the Python package's README has no sentence listing the reasons - this guard no longer reads it")
	}
	var reasons []string
	for _, m := range regexp.MustCompile("`([a-z_]+)`").FindAllStringSubmatch(sentence[1], -1) {
		reasons = append(reasons, m[1])
	}
	sort.Strings(reasons)
	if want := recipe.Reasons(); !reflect.DeepEqual(reasons, want) {
		t.Errorf("the Python package's README lists the reasons %v, and the program declares %v", reasons, want)
	}
}

// pyLiterals are the Python files whose strings are sentences somebody reads.
// In the rest a string is a test's data or a script for a child process.
var pyLiterals = map[string]bool{
	pyPackage + "/src/testing_files_generator/__init__.py":      true,
	pyPackage + "/src/testing_files_generator/__main__.py":      true,
	pyPackage + "/src/testing_files_generator/_binary.py":       true,
	pyPackage + "/src/testing_files_generator/_fileset.py":      true,
	pyPackage + "/src/testing_files_generator/_run.py":          true,
	pyPackage + "/src/testing_files_generator/_testfile.py":     true,
	pyPackage + "/src/testing_files_generator/pytest_plugin.py": true,
	pyPackage + "/release/assemble.py":                          true,
}

func TestTheTextOfThePythonPackageIsAsciiWithPlainPunctuation(t *testing.T) {
	sources, literals := 0, 0
	for _, name := range strings.Fields(gitOutput(t, "ls-files", "--cached", "--others", "--exclude-standard")) {
		if !strings.HasPrefix(name, "python/") && !strings.HasPrefix(name, "testdata/wrappers/") {
			continue
		}
		body := npmFile(t, name)
		for n, line := range strings.Split(body, "\n") {
			if i := strings.IndexFunc(line, func(r rune) bool { return r > 127 }); i >= 0 {
				t.Errorf("%s:%d holds %q - text in the repository is English", name, n+1, []rune(line[i:])[0])
			}
		}
		if filepath.Ext(name) != ".py" {
			continue
		}
		sources++
		prose, err := pyProse(body, pyLiterals[name])
		if err != nil {
			t.Errorf("%s cannot be read for its prose: %v", name, err)
			continue
		}
		if pyLiterals[name] {
			literals++
		}
		for _, p := range prose {
			for _, fault := range proseFaults(p.text, false) {
				t.Errorf("%s:%d holds %s", name, p.line, fault)
			}
		}
	}
	if sources == 0 || literals < len(pyLiterals) {
		t.Fatalf("read %d sources and the strings of %d of %d - the package is not where this guard looks", sources, literals, len(pyLiterals))
	}
}

// pyProse returns what a person reads in a Python file: its comments, and
// with literals the words of its strings, the code inside an f-string's
// braces left out. It reads only as much of the language as that takes, and
// says so rather than guess when a file ends inside a string.
func pyProse(src string, literals bool) ([]jsText, error) {
	var out []jsText
	line := 1
	add := func(at int, text string) {
		if strings.TrimSpace(text) != "" {
			out = append(out, jsText{at, text})
		}
	}
	isLetter := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\n':
			line++
		case c == '#':
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			add(line, src[i+1:i+end])
			i += end - 1
		case c == '\'' || c == '"':
			p := i
			for p > 0 && isLetter(src[p-1]) {
				p--
			}
			prefix := strings.ToLower(src[p:i])
			raw, format := strings.Contains(prefix, "r"), strings.Contains(prefix, "f")
			quote := string(c)
			if strings.HasPrefix(src[i:], strings.Repeat(quote, 3)) {
				quote = strings.Repeat(quote, 3)
			}
			start, j := line, i+len(quote)
			var text strings.Builder
			for ; j < len(src) && !strings.HasPrefix(src[j:], quote); j++ {
				switch ch := src[j]; {
				case ch == '\\' && j+1 < len(src):
					if src[j+1] == '\n' {
						line++
					}
					if raw {
						text.WriteByte(ch)
						text.WriteByte(src[j+1])
					} else {
						text.WriteByte(' ')
					}
					j++
				case format && ch == '{' && j+1 < len(src) && src[j+1] == '{':
					text.WriteByte('{')
					j++
				case format && ch == '{':
					// The code of a replacement field is skipped to its
					// closing brace. A brace inside a string in there would
					// end it early, and the package writes none.
					for open := 1; open > 0 && j+1 < len(src); {
						j++
						switch src[j] {
						case '{':
							open++
						case '}':
							open--
						}
					}
					text.WriteByte(' ')
				case ch == '\n':
					if len(quote) == 1 {
						return nil, errorAt(line, "a string that never ends on its line")
					}
					line++
					text.WriteByte('\n')
				default:
					text.WriteByte(ch)
				}
			}
			if j >= len(src) {
				return nil, errorAt(start, "a string that never ends")
			}
			if literals {
				add(start, text.String())
			}
			i = j + len(quote) - 1
		}
	}
	return out, nil
}
