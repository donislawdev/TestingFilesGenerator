package guard

import (
	"strings"
	"testing"
	"unicode"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A space from outside ASCII at either end of a name is part of the name.
//
// Measured on 2026-09-24 (O243): a recipe asking for a file whose name starts
// with an ideographic space, U+3000, got a file without it, and nothing said
// so. The parser took each value with strings.TrimSpace, which knows every
// white space character Unicode has, while YAML itself counts only the space
// and the tab. The library handed the character over intact and the reading
// code threw it away. It surfaced through the preset of unusual file names,
// which asks for exactly such a name and came back one short on every format.
//
// The guard beside it in compose_test.go puts every character in the MIDDLE
// of a value, on purpose, because the ends are trimmed. That is why it never
// saw this, and why this one asks about nothing but the ends.
//
// The characters come from the Unicode table rather than from a list typed
// here, so this asks about every one the language knows and not the few
// somebody thought of. ASCII ones are left out: a space or a tab at the end of
// an unquoted YAML value is not part of it by the rules of YAML.
func TestAUnicodeSpaceAtEitherEndOfANameIsKept(t *testing.T) {
	var spaces []rune
	for _, r16 := range unicode.White_Space.R16 {
		for r := rune(r16.Lo); r <= rune(r16.Hi); r += rune(r16.Stride) {
			if r > unicode.MaxASCII {
				spaces = append(spaces, r)
			}
		}
	}
	for _, r32 := range unicode.White_Space.R32 {
		for r := rune(r32.Lo); r <= rune(r32.Hi); r += rune(r32.Stride) {
			spaces = append(spaces, r)
		}
	}
	// The ideographic space the defect was found with, and the no break space
	// every keyboard layout can type, have to be among them, or this is asking
	// about some other table.
	if !containsRune(spaces, 0x3000) || !containsRune(spaces, 0xA0) {
		t.Fatalf("the table gave %d characters and not the two this is about: %U", len(spaces), spaces)
	}

	checked := 0
	for _, r := range spaces {
		for _, name := range []string{string(r) + "report.txt", "report.txt" + string(r)} {
			// The state this guard is about: a name that a reader trimming
			// Unicode white space would shorten. Asserted, not assumed.
			if strings.TrimSpace(name) == name {
				t.Fatalf("%q is not a name that trimming would change, so it tests nothing", name)
			}

			written := "version: 1\ntargets:\n  - id: t\n    format: txt\n    size: 1kb\n    name: " + name + "\n"
			if got, ok := nameReadFrom(t, []byte(written)); ok && got != name {
				t.Errorf("a recipe written by hand asked for %+q and read it as %+q", name, got)
			}

			composed, err := recipe.Compose(recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "t", Format: "txt", Size: "1kb", Name: name,
			}}})
			if err != nil {
				t.Errorf("composing a recipe with the name %+q was refused: %v", name, err)
				continue
			}
			if got, ok := nameReadFrom(t, composed); ok && got != name {
				t.Errorf("a composed recipe asked for %+q and read it as %+q\n%s", name, got, composed)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no name was checked - this guard would pass without asking anything")
	}
	t.Logf("%d names, %d characters at the start and at the end", checked, len(spaces))
}

// nameReadFrom parses a one target recipe and gives back the name it asks for.
func nameReadFrom(t *testing.T, src []byte) (string, bool) {
	t.Helper()
	rec, err := recipe.Parse(src, "guard")
	if err != nil {
		t.Errorf("the recipe was refused: %v\n%s", err, src)
		return "", false
	}
	if len(rec.Targets) != 1 {
		t.Errorf("the recipe parsed to %d targets\n%s", len(rec.Targets), src)
		return "", false
	}
	return rec.Targets[0].Name, true
}

func containsRune(rs []rune, want rune) bool {
	for _, r := range rs {
		if r == want {
			return true
		}
	}
	return false
}
