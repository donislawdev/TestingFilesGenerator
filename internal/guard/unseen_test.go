package guard

import (
	"testing"
	"unicode"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// unseenHere is a character a person reading a line cannot see, worked out
// from the Unicode tables rather than asked of the code under test: anything
// that is not a letter, a mark, a number, punctuation or a symbol, apart from
// the plain space. A guard that imported the class from the code would agree
// with it whatever it did.
func unseenHere(r rune) bool {
	return r != ' ' && !unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S)
}

// unseenSample is the characters these guards ask about: every format
// character, every separator and every control character above the ones a
// recipe refuses outright, from the tables, and one each from private use and
// from the unassigned range.
func unseenSample() []rune {
	var out []rune
	add := func(t *unicode.RangeTable) {
		for _, r16 := range t.R16 {
			for r := rune(r16.Lo); r <= rune(r16.Hi); r += rune(r16.Stride) {
				out = append(out, r)
			}
		}
		for _, r32 := range t.R32 {
			for r := rune(r32.Lo); r <= rune(r32.Hi); r += rune(r32.Stride) {
				out = append(out, r)
			}
		}
	}
	add(unicode.Cf)
	add(unicode.Zl)
	add(unicode.Zp)
	add(unicode.Zs)
	for r := rune(0x80); r <= 0x9F; r++ {
		out = append(out, r)
	}
	out = append(out, 0xE000, 0x0378)

	kept := out[:0]
	for _, r := range out {
		if unseenHere(r) {
			kept = append(kept, r)
		}
	}
	return kept
}

// A character nobody can see goes into a composed recipe as an escape and
// comes back out as itself.
//
// Measured on 2026-09-24 (O244): the library wrote a right to left override, a
// zero width space, a byte order mark and a line separator raw into unquoted
// values. A person editing the recipe could not see what was in it, and PyYAML
// refused the document at the line separator. The preset of unusual file names
// ejects exactly such a recipe, and a recipe composed in the window can hold one
// too.
//
// Both halves are asked, because either alone is satisfied by the wrong code:
// nothing raw in the source is what writing the value as an empty string would
// also give, and the value coming back is what writing it raw gave before.
func TestACharacterNobodyCanSeeIsWrittenAsAnEscapeAndReadBackAsItself(t *testing.T) {
	sample := unseenSample()
	// The ones the defect was found with have to be among them, or this is
	// asking about some other table.
	for _, must := range []rune{0x202E, 0x200B, 0xFEFF, 0x2028, 0x3000, 0xA0, 0xE0068} {
		if !containsRune(sample, must) {
			t.Fatalf("U+%04X is not in the sample of %d characters", must, len(sample))
		}
	}

	checked := 0
	for _, r := range sample {
		for _, value := range []string{string(r) + "b.txt", "a" + string(r) + "b.txt", "ab.txt" + string(r)} {
			src, err := recipe.Compose(recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "t", Format: "txt", Size: "1kb", Name: value, Group: value,
			}}})
			if err != nil {
				t.Errorf("U+%04X: composing was refused: %v", r, err)
				continue
			}
			for _, c := range string(src) {
				if c != '\n' && unseenHere(c) {
					t.Errorf("U+%04X: the composed recipe carries U+%04X raw\n%s", r, c, src)
					break
				}
			}
			rec, err := recipe.Parse(src, "composed")
			if err != nil {
				t.Errorf("U+%04X: the composed recipe was refused: %v\n%s", r, err, src)
				continue
			}
			if got := rec.Targets[0].Name; got != value {
				t.Errorf("U+%04X: asked for the name %+q and read %+q\n%s", r, value, got, src)
			}
			if got := rec.Targets[0].Group; got != value {
				t.Errorf("U+%04X: asked for the group %+q and read %+q\n%s", r, value, got, src)
			}
			checked++
		}
	}
	if checked < 3*100 {
		t.Fatalf("only %d values were checked - the tables gave less than they should", checked)
	}
	t.Logf("%d characters, %d values composed and read back", len(sample), checked)
}
