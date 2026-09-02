package guard

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"unicode"
)

// What this defends. D9: what lives in the repository is English, whatever its
// audience. Comments included - the criterion is the place, not the reader.
//
// Why it needed a guard, and why this one rather than the one next door.
// ascii_test.go asks whether a character is above 127. Polish written without
// its accents is entirely ASCII, so it walks past that question, and past the
// punctuation guard, and past every linter here. Measured 2026-08-27: a
// six line Polish comment sat in internal/guard for a day and was found by
// accident, because misspell reported one word inside it as a typo. Nothing
// was looking at the other five lines, and nothing was looking at the file at
// all - ascii_test.go covers internal/cli and cmd/tfg, not this package.
//
// Why the sentence in ascii_test.go was too strong. It says no automated check
// ever will catch another language written in plain ASCII. That is a written
// impossibility with nothing holding it, which this project has been burned by
// before - the 7Z claim that four commands overturned, and the dropdown theme
// that six days of nobody rechecking left standing. The limit was in the
// MECHANISM, not in the problem: asking about characters cannot see a language,
// and asking about WORDS can. Not every language - this asks about Polish,
// because Polish is the only other language written here.
//
// Why comments and not string literals. A literal may legitimately hold another
// language as test data, which is why ascii_test.go exempts test files. A
// comment has no such excuse in any file, so this covers tests too - the defect
// that started this was in a test file.
//
// What this does NOT catch. Polish written entirely in words that are also
// English words, and every language that is not Polish. It narrows the reading
// that ascii_test.go leaves to a person, it does not remove it.

// polishWords are Polish words that are not also English words.
//
// Chosen for that property rather than for frequency, and it is the whole
// design. "to", "on", "we", "by", "do", "za", "ale", "co" and "pod" are all
// ordinary Polish and all ordinary English, so every one of them would report a
// perfectly good English comment. What is left is still dense enough that a
// sentence of Polish is very hard to write without one.
//
// Both spellings of the accented ones, because this project writes Polish both
// ways - the documents carry accents and the commit messages mostly do not.
var polishWords = map[string]bool{
	"jest": true, "sie": true, "się": true, "wiec": true, "więc": true,
	"ktory": true, "który": true, "ktora": true, "która": true,
	"ktore": true, "które": true, "zeby": true, "żeby": true,
	"dlatego": true, "poniewaz": true, "ponieważ": true, "czyli": true,
	"tylko": true, "takze": true, "także": true, "wszystko": true,
	"jednak": true, "przez": true, "bardzo": true, "moze": true, "może": true,
	"musi": true, "trzeba": true, "wtedy": true, "zawsze": true, "nigdy": true,
	"teraz": true, "nawet": true, "jako": true, "przy": true, "nad": true,
	"bez": true, "dla": true, "nie": true, "oraz": true, "albo": true,
	"kazdy": true, "każdy": true, "wlasnie": true, "właśnie": true,
	"zamiast": true, "rzeczy": true, "byla": true, "była": true,
	"bylo": true, "było": true, "mowi": true, "mówi": true, "robi": true,
	"jesli": true, "jeśli": true, "kiedy": true, "gdzie": true, "wszystkie": true,
}

// Translations are the one place another language is the subject.
//
// A comment there explaining what a Polish string says would be reporting
// itself. Named rather than guessed at, so a second package holding another
// language has to be added here on purpose.
const translationsPackage = "internal/gui/text"

func TestNoCommentInTheRepositoryIsWrittenInPolish(t *testing.T) {
	checked, comments := 0, 0

	for _, p := range packages(t) {
		if p.rel == translationsPackage || strings.HasPrefix(p.rel, translationsPackage+"/") {
			continue
		}

		// Tests as well as sources. The comment that started this was in a test
		// file, and a rule that skipped them would have been green on the day
		// it was written.
		for _, f := range concat(p.files, p.tests) {
			checked++

			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
			if err != nil {
				t.Errorf("parsing %s: %v", f, err)
				continue
			}

			for _, group := range parsed.Comments {
				comments++
				for _, line := range group.List {
					if word, found := firstPolishWord(line.Text); found {
						pos := fset.Position(line.Pos())
						t.Errorf("%s:%d holds the Polish word %q in a comment.\n"+
							"  %s\n"+
							"D9 makes everything in the repository English, comments included, "+
							"and the criterion is the place rather than the reader. The internal "+
							"documents are Polish because they live outside the repository.",
							trimRoot(t, pos.Filename), pos.Line, word, strings.TrimSpace(line.Text))
						break
					}
				}
			}
		}
	}

	// Both counters, because either being zero means a green test about nothing.
	// build.ImportDir honours build tags, so a shell with CGO_ENABLED=0 hides
	// every file behind //go:build cgo - the same environment noise that makes
	// the notices guard report no modules at all.
	if checked == 0 {
		t.Fatal("no Go file was read, so this proved nothing")
	}
	if comments == 0 {
		t.Fatalf("%d Go files were read and not one comment was found, which means the "+
			"comments were not reached rather than that they are all English", checked)
	}
}

// firstPolishWord reports the first word of a comment that is Polish and not
// also English.
//
// Split on anything that is not a letter, so the comment markers, the
// punctuation and any identifier with an underscore fall apart into words
// rather than hiding one. Done this way rather than with a word boundary in a
// pattern because Go's boundaries are ASCII only, and half of these words are
// not.
func firstPolishWord(text string) (string, bool) {
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r)
	}) {
		// A word in capitals is a quoted value, not prose. Two guards read the
		// Polish regression table in CLAUDE.md and name its verdict column in
		// their own comments - "JEST" with nothing behind it - and both were
		// reported by the first version of this. A comment naming a value it
		// works with is English prose about a Polish token, which is the
		// opposite of what this looks for.
		//
		// Safe because the emphasis this project puts in comments is on English
		// words. NOT, RUNS and DEFAULT collide with nothing here.
		if word == strings.ToUpper(word) && word != strings.ToLower(word) {
			continue
		}
		lowered := strings.ToLower(word)
		if polishWords[lowered] {
			return lowered, true
		}
	}
	return "", false
}

func trimRoot(t *testing.T, path string) string {
	t.Helper()
	root := repoRoot(t)
	if rel := strings.TrimPrefix(path, root); rel != path {
		return strings.TrimPrefix(strings.ReplaceAll(rel, "\\", "/"), "/")
	}
	return path
}
