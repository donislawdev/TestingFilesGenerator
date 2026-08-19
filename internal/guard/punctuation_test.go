package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// Rule 13 of this project asks for one dash and no semicolons in the prose that
// ships: the flat hyphen, and a full stop where a semicolon would go. Until now
// nothing held it. The tree happened to obey it, which is a different thing
// from being made to.
//
// What the two ASCII guards already cover, so that this one is not read as
// doing more than it does: any character above 127 is refused in README and
// both changelogs, and in the non test files of internal/cli and cmd/tfg. Every
// dash this guard rejects is above 127, so for those places the dash half is a
// second opinion rather than new ground. It is worth having anyway, because
// "holds a character the command line does not allow" and "holds an en dash
// where D17 asks for a hyphen" send an author to two different fixes.
//
// New ground is the rest: comments in every other package, comments in test
// files, and the semicolon half everywhere. Nothing checked the semicolon at
// all.
//
// Measured before writing, on 2026-08-02: zero offending dashes anywhere, zero
// semicolons in comments, and fifteen semicolons in string literals of which
// every single one is syntax rather than prose - HTML and XML entities, the
// User-Agent strings the log generator emits, and the Python and JavaScript
// oracles carried in raw strings. That measurement is what fixed the scope
// below. A guard that read those fifteen as prose would have been switched off
// inside a week.

// Prose is what a person reads as a sentence. Code inside prose is not prose,
// and this file draws that line in three places with one rule.
var (
	// entityRef matches a character entity reference. The semicolon closing
	// one belongs to the entity, not to the sentence around it. Measured: this
	// tree already writes &amp; inside prose, in CHANGELOG.md and in a message
	// of textformats_test.go.
	entityRef = regexp.MustCompile(`&(#[0-9]+|#[xX][0-9a-fA-F]+|[a-zA-Z][a-zA-Z0-9]*);`)

	// inlineCode matches a markdown code span.
	inlineCode = regexp.MustCompile("`[^`\n]*`")
)

// proseFaults reports what is wrong with one stretch of prose, in the words an
// author needs to fix it.
// symbols says whether a symbol such as an emoji is allowed in this text. It is
// true for the shop window and false everywhere else - see symbolsAllowed.
func proseFaults(text string, symbols bool) []string {
	text = entityRef.ReplaceAllString(text, "")

	var out []string
	if strings.Contains(text, ";") {
		out = append(out, "a semicolon - rule 13 asks for a full stop, or a comma "+
			"when the second half is only an aside")
	}
	for _, r := range text {
		if r == '-' {
			continue
		}
		// Pd holds every dash Unicode calls punctuation, the flat one included,
		// so that one is let past above. The minus sign is not in Pd and is
		// named here because it is the dash a document copied out of a word
		// processor tends to carry.
		if !unicode.Is(unicode.Pd, r) && r != '\u2212' {
			continue
		}
		out = append(out, fmt.Sprintf("%q - rule 13 asks for the flat hyphen -", r))
		break
	}
	for _, r := range text {
		// ASCII is left alone here. The dash and semicolon rules above already
		// cover what matters in it, and a circumflex or a backtick inside a
		// comment about a regular expression is ordinary writing.
		if r < 128 {
			continue
		}
		// Two things slip past a rule written only about dashes, and this tree
		// carried one of each until 2026-08-04.
		//
		// A symbol is note taking that escaped. The red circle this project
		// uses to mark a warning in its own documents had reached a comment in
		// internal/core, where it means nothing to anybody reading the code.
		//
		// The rest are the characters a word processor substitutes: an ellipsis
		// and curly quotes. They read the same and cannot be typed back, so a
		// person searching for the line does not find it.
		//
		// Letters are deliberately not touched. One comment in this package
		// annotates raw cp1250 bytes and has to be able to write the letters
		// those bytes stand for - a rule that forbade it would make the one
		// comment that needs them say less.
		switch {
		case unicode.Is(unicode.So, r) && symbols:
			continue
		case unicode.Is(unicode.So, r):
			out = append(out, fmt.Sprintf("%q - a symbol belongs in the project's own notes, not in text that ships", r))
		case r == '…' || r == '‘' || r == '’' || r == '“' || r == '”':
			out = append(out, fmt.Sprintf("%q - write it the way somebody can type it", r))
		default:
			continue
		}
		break
	}
	return out
}

// markdownProse returns the prose of each line, with fenced blocks and code
// spans removed. The result is indexed the way a line number is, one behind.
func markdownProse(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := make([]string, len(lines))
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		out[i] = inlineCode.ReplaceAllString(line, "")
	}
	return out
}

// commentProse strips the marker from a comment and drops the lines a doc
// comment indents. Go marks an example by indenting it, so an indented line is
// code and is judged as code - the same rule as a fenced block in markdown.
func commentProse(text string) string {
	var keep []string
	for _, line := range strings.Split(text, "\n") {
		body := strings.TrimPrefix(strings.TrimSpace(line), "//")
		if strings.HasPrefix(body, "\t") || strings.HasPrefix(body, "    ") {
			continue
		}
		keep = append(keep, body)
	}
	return strings.Join(keep, "\n")
}

// markdownFaults checks the English text files of the repository.
func markdownFaults(t *testing.T, root string) ([]string, int) {
	t.Helper()

	var faults []string
	checked := 0
	for _, name := range englishFiles {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("reading %s: %v - it is listed as English text but is not there",
				name, err)
			continue
		}
		checked++
		for i, prose := range markdownProse(string(body)) {
			for _, fault := range proseFaults(prose, symbolsAllowed[name]) {
				faults = append(faults,
					fmt.Sprintf("%s:%d holds %s", name, i+1, fault))
			}
		}
	}
	return faults, checked
}

// goFileFaults checks one Go file. Comments are prose everywhere. String
// literals are prose only where they are what the user reads, which is the same
// two packages the ASCII guard names - elsewhere a literal is a byte the format
// needs, and judging it as a sentence is how this guard would start crying
// wolf.
func goFileFaults(t *testing.T, path, rel string, literals bool) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	var faults []string
	at := func(pos token.Pos, fault string) {
		faults = append(faults,
			fmt.Sprintf("%s:%d holds %s", rel, fset.Position(pos).Line, fault))
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			for _, fault := range proseFaults(commentProse(comment.Text), false) {
				at(comment.Pos(), fault)
			}
		}
	}
	if !literals {
		return faults
	}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		for _, fault := range proseFaults(lit.Value, false) {
			at(lit.Pos(), fault)
		}
		return true
	})
	return faults
}

// goFaults walks every Go file of the module, test files included. A comment in
// a test is prose in the repository like any other.
func goFaults(t *testing.T) ([]string, int) {
	t.Helper()
	root := repoRoot(t)

	var faults []string
	checked := 0
	for _, p := range packages(t) {
		paths, err := filepath.Glob(filepath.Join(p.dir, "*.go"))
		if err != nil {
			t.Fatalf("listing %s: %v", p.rel, err)
		}
		for _, path := range paths {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			checked++
			faults = append(faults,
				goFileFaults(t, path, filepath.ToSlash(rel), asciiRequired(p.rel))...)
		}
	}
	return faults, checked
}

func TestProseInTheRepositoryUsesFlatHyphensAndNoSemicolons(t *testing.T) {
	fromMarkdown, markdownFiles := markdownFaults(t, repoRoot(t))
	fromGo, goFiles := goFaults(t)

	if markdownFiles == 0 || goFiles == 0 {
		t.Fatal("nothing was examined - this guard would pass without checking anything")
	}

	// Every one at once rather than the first, for the reason RC7 gives about
	// recipes: being sent back seven times for seven answers is its own defect.
	faults := append(fromMarkdown, fromGo...)
	if len(faults) > 0 {
		sort.Strings(faults)
		t.Errorf("%d places break rule 13 on punctuation:\n  %s",
			len(faults), strings.Join(faults, "\n  "))
	}
}
