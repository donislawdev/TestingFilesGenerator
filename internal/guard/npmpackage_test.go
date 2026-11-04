package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The npm package (npm/testing-files-generator) runs this program and reads
// what it writes, from JavaScript. Nothing in Go calls it, so nothing in Go
// notices when the two drift apart - a platform the release builds and the
// package cannot find, an outcome the program declares and the package refuses,
// a manifest field the package's types promise and the manifest does not have.
// These guards read the package's files the way the release and the manifest
// are read here, and hold one to the other. docs/PLAYWRIGHT-2026-11-04.md.

const npmPackage = "npm/testing-files-generator"

func npmFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// releaseTargets is the list of GOOS/GOARCH pairs the release loop builds a
// command line for.
func releaseTargets(t *testing.T) []string {
	t.Helper()
	body := npmFile(t, ".github/workflows/release.yml")
	loop := regexp.MustCompile(`(?s)for target in \\\n(.*?)\n\s*do\n`).FindStringSubmatch(body)
	if loop == nil {
		t.Fatal("the release workflow has no loop of command line targets - this guard no longer reads it")
	}
	targets := regexp.MustCompile(`[a-z0-9]+/[a-z0-9]+`).FindAllString(loop[1], -1)
	if len(targets) == 0 {
		t.Fatal("the release loop names no targets this guard can read")
	}
	sort.Strings(targets)
	return targets
}

func TestThePackageKnowsEveryPlatformTheReleaseBuilds(t *testing.T) {
	node := map[string]string{"windows": "win32", "linux": "linux", "darwin": "darwin", "amd64": "x64", "arm64": "arm64"}
	var want []string
	for _, target := range releaseTargets(t) {
		goos, goarch, _ := strings.Cut(target, "/")
		if node[goos] == "" || node[goarch] == "" {
			t.Fatalf("the release builds %s, and this guard does not know what Node calls it", target)
		}
		want = append(want, node[goos]+" "+node[goarch])
	}

	body := npmFile(t, npmPackage+"/lib/binary.js")
	entries := regexp.MustCompile(`'([a-z0-9]+ [a-z0-9]+)': '(@testing-files-generator/[a-z0-9-]+)'`).FindAllStringSubmatch(body, -1)
	var got []string
	for _, e := range entries {
		got = append(got, e[1])
		if name := "@testing-files-generator/" + strings.ReplaceAll(e[1], " ", "-"); e[2] != name {
			t.Errorf("the binary for %s comes in %s, and the name the release publishes for it is %s", e[1], e[2], name)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the package looks for a binary on %v, and the release builds one for %v", got, want)
	}
}

// unionOf reads the members of a TypeScript union of string literals.
func unionOf(t *testing.T, body, name string) []string {
	t.Helper()
	decl := regexp.MustCompile(`(?s)export type ` + name + ` =(.*?);`).FindStringSubmatch(body)
	if decl == nil {
		t.Fatalf("index.d.ts has no type %s", name)
	}
	var out []string
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(decl[1], -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestThePackageHoldsTheManifestsClosedLists(t *testing.T) {
	types := npmFile(t, npmPackage+"/index.d.ts")
	if got, want := unionOf(t, types, "Outcome"), recipe.Outcomes(); !reflect.DeepEqual(got, want) {
		t.Errorf("index.d.ts says an outcome is one of %v, and the program declares %v", got, want)
	}
	if got, want := unionOf(t, types, "Reason"), recipe.Reasons(); !reflect.DeepEqual(got, want) {
		t.Errorf("index.d.ts says a reason is one of %v, and the program declares %v", got, want)
	}

	runtime := regexp.MustCompile(`const outcomes = \[([^\]]*)\]`).FindStringSubmatch(npmFile(t, npmPackage+"/lib/testfile.js"))
	if runtime == nil {
		t.Fatal("lib/testfile.js has no list of outcomes this guard can read")
	}
	var got []string
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(runtime[1], -1) {
		got = append(got, m[1])
	}
	sort.Strings(got)
	if want := recipe.Outcomes(); !reflect.DeepEqual(got, want) {
		t.Errorf("the package accepts the outcomes %v, and the program declares %v", got, want)
	}
}

// jsonFields is the set of names a manifest type writes.
func jsonFields(typ reflect.Type) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out[name] = true
		}
	}
	return out
}

// typeScriptFields reads the field names of one block of index.d.ts - an
// interface, or an object written inside one - leaving out the index
// signature that lets a manifest carry more than the package reads.
func typeScriptFields(block string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`(?m)(?:^\s*|[{;]\s*)([a-z_]+)\??:`).FindAllStringSubmatch(block, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestThePackageTypesOnlyPromiseFieldsTheManifestHas(t *testing.T) {
	types := npmFile(t, npmPackage+"/index.d.ts")
	iface := func(name string) string {
		m := regexp.MustCompile(`(?s)export interface ` + name + ` \{\n(.*?)\n\}`).FindStringSubmatch(types)
		if m == nil {
			t.Fatalf("index.d.ts has no interface %s", name)
		}
		return m[1]
	}
	inline := func(block, name string) string {
		m := regexp.MustCompile(`(?m)^\s*` + name + `: \{(.*)\};$`).FindStringSubmatch(block)
		if m == nil {
			t.Fatalf("the Manifest interface in index.d.ts has no %s block on one line", name)
		}
		return m[1]
	}
	whole := iface("Manifest")
	top := regexp.MustCompile(`(?m)^\s*\w+: \{.*\};$`).ReplaceAllString(whole, "")
	cases := []struct {
		where string
		block string
		typ   reflect.Type
	}{
		{"ManifestFile", iface("ManifestFile"), reflect.TypeOf(manifest.File{})},
		{"Expected", iface("Expected"), reflect.TypeOf(manifest.Expected{})},
		{"Manifest", top + "\ntool: x;\nrun: x;\nsummary: x;", reflect.TypeOf(manifest.Manifest{})},
		{"Manifest.tool", inline(whole, "tool"), reflect.TypeOf(manifest.Tool{})},
		{"Manifest.run", inline(whole, "run"), reflect.TypeOf(manifest.Run{})},
		{"Manifest.summary", inline(whole, "summary"), reflect.TypeOf(manifest.Summary{})},
	}
	for _, c := range cases {
		have := jsonFields(c.typ)
		fields := typeScriptFields(c.block)
		if len(fields) == 0 {
			t.Errorf("%s in index.d.ts declares no field this guard can read", c.where)
		}
		for _, f := range fields {
			if !have[f] {
				t.Errorf("%s in index.d.ts promises %s, and the manifest's %s writes no such field", c.where, f, c.typ.Name())
			}
		}
	}
}

// The text of the package and of its example is what a person reads - an
// error thrown into their test, a comment their editor shows, a line the
// example prints. It is repository text, so it is English, ASCII and rule 13.
var npmTextRoots = []string{npmPackage + "/", "examples/playwright/"}

// npmLiterals are the files whose strings are sentences somebody reads. In
// the rest a string is a test's data, a script for a child process or a
// header value, and judging it as prose would be crying wolf - the same line
// goFileFaults draws for Go.
var npmLiterals = map[string]bool{
	npmPackage + "/index.js":          true,
	npmPackage + "/bin/tfg.js":        true,
	npmPackage + "/lib/binary.js":     true,
	npmPackage + "/lib/fileset.js":    true,
	npmPackage + "/lib/run.js":        true,
	npmPackage + "/lib/testfile.js":   true,
	"examples/playwright/app/page.js": true,
	"examples/playwright/check.mjs":   true,
}

func npmTextFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, name := range strings.Fields(gitOutput(t, "ls-files", "--cached", "--others", "--exclude-standard")) {
		for _, root := range npmTextRoots {
			if strings.HasPrefix(name, root) {
				out = append(out, name)
			}
		}
	}
	return out
}

func TestTheTextOfThePackageIsAsciiWithPlainPunctuation(t *testing.T) {
	files := npmTextFiles(t)
	sources, literals := 0, 0
	for _, name := range files {
		body := npmFile(t, name)
		for n, line := range strings.Split(body, "\n") {
			if i := strings.IndexFunc(line, func(r rune) bool { return r > 127 }); i >= 0 {
				t.Errorf("%s:%d holds %q - text in the repository is English", name, n+1, []rune(line[i:])[0])
			}
		}
		switch filepath.Ext(name) {
		case ".js", ".mjs", ".ts":
		default:
			continue
		}
		sources++
		prose, err := jsProse(body, npmLiterals[name])
		if err != nil {
			t.Errorf("%s cannot be read for its prose: %v", name, err)
			continue
		}
		if npmLiterals[name] {
			literals++
		}
		for _, p := range prose {
			for _, fault := range proseFaults(p.text, false) {
				t.Errorf("%s:%d holds %s", name, p.line, fault)
			}
		}
	}
	if sources == 0 || literals < 6 {
		t.Fatalf("read %d sources and the strings of %d - the package is not where this guard looks", sources, literals)
	}
}

type jsText struct {
	line int
	text string
}

// jsProse returns what a person reads in a JavaScript or TypeScript file: its
// comments, and with literals the words of its strings, the code inside a
// template's ${} left out. It reads only as much of the language as that
// takes, and says so rather than guess when a file ends inside a string.
func jsProse(src string, literals bool) ([]jsText, error) {
	var out []jsText
	line := 1
	add := func(at int, text string) {
		if strings.TrimSpace(text) != "" {
			out = append(out, jsText{at, text})
		}
	}
	// last is the last character of code that is not space, which tells a
	// regular expression from a division.
	last := byte('\n')
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\n':
			line++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				end = len(src) - i
			}
			add(line, src[i+2:i+end])
			i += end - 1
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, errorAt(line, "a comment that never ends")
			}
			body := src[i+2 : i+2+end]
			add(line, strings.NewReplacer("\n *", "\n", "\n*", "\n").Replace(body))
			line += strings.Count(body, "\n")
			i += end + 3
			continue
		case c == '/' && strings.IndexByte("(,=:[!&|?{};+\n", last) >= 0:
			j := i + 1
			class := false
			for ; j < len(src) && (src[j] != '/' || class); j++ {
				switch src[j] {
				case '\\':
					j++
				case '[':
					class = true
				case ']':
					class = false
				case '\n':
					return nil, errorAt(line, "a regular expression that never ends")
				}
			}
			i = j
			last = '/'
			continue
		case c == '\'' || c == '"' || c == '`':
			start, j := line, i+1
			var text strings.Builder
			for ; j < len(src) && src[j] != c; j++ {
				switch {
				case src[j] == '\\':
					j++
				case c == '`' && src[j] == '$' && j+1 < len(src) && src[j+1] == '{':
					// The code of an interpolation is skipped to its closing
					// brace. A brace inside a string in there would end it
					// early, and the package writes none.
					j++
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
				case src[j] == '\n':
					if c != '`' {
						return nil, errorAt(line, "a string that never ends on its line")
					}
					line++
					text.WriteByte('\n')
				default:
					text.WriteByte(src[j])
				}
			}
			if j >= len(src) {
				return nil, errorAt(start, "a string that never ends")
			}
			if literals {
				add(start, text.String())
			}
			i = j
			last = c
			continue
		}
		if c != ' ' && c != '\t' && c != '\r' {
			last = c
		}
	}
	return out, nil
}

func errorAt(line int, what string) error {
	return fmt.Errorf("line %d starts %s", line, what)
}
