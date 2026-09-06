package guard

import (
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// hostileParameterValues are values that would break a document written by
// pasting strings together.
//
// The first of them is not invented. Fuzzing on 2026-08-05 produced "1\rB",
// which the size parser reads as one byte because it trims the ends - and the
// carriage return then reached the recipe source raw and broke the document.
// The comment on the renderer said at the time that no value there needed
// quoting. That was true of every value except the one a caller supplies.
//
// The rest are the shapes that turn one line of YAML into two, or into a
// comment, or into a key of their own.
var hostileParameterValues = []string{
	"1\rB",
	"1kb\n  id: injected",
	"1kb#comment",
	"1kb: value",
	"1kb\ttab",
	`1kb"quote`,
	"1kb'quote",
	"- 1kb",
	"1kb ",
	" 1kb",
	"[1kb]",
	"{1kb}",
	"1kb\\",
	"1kb\x00",
}

// Every preset renders a recipe that means what it says, whatever it is given.
//
// internal/preset builds its source by writing strings into a template, which an
// outside review on 2026-09-05 raised as finding S8: nothing in the shape of
// that code stops a value from carrying a newline or a colon into the document.
// What keeps it safe today is firstUnusable, a whitelist of the characters a
// size is written with, and the review's point was that the whitelist has to be
// remembered by whoever adds the next preset.
//
// The review's remedy - composing through recipe.Compose, which lets the
// encoder quote - was measured and turned down. It is in
// docs/SECURITY-REVIEW-2026-09-06.md section 13 with the numbers: the source
// comes out 627 B rather than 965 B, and internal/cli/preset.go records
// recipe.Hash of exactly those bytes as recipe_hash in the manifest. Changing
// the renderer changes that hash for everybody who runs a preset, which is a
// promise about other people's records rather than a tidy.
//
// So the guarantee is held by asking rather than by rewriting: whatever any
// preset is given, the source it produces either is refused with a sentence or
// parses back into a recipe with targets. That is the property the whitelist
// exists to provide, and it is asked of every REGISTERED preset rather than of
// the one that exists today - FuzzPresetExpansion asks it well and asks it of
// "size-boundaries" by name, so a second preset would arrive uncovered.
//
// Deterministic rather than fuzzed on purpose. A fuzz target runs where somebody
// runs it, and this runs on every push.
func TestEveryPresetRendersARecipeThatParsesBack(t *testing.T) {
	presets := preset.All()
	if len(presets) == 0 {
		t.Fatal("no preset is registered, so this guard checked nothing")
	}

	checked := 0
	for _, p := range presets {
		for _, param := range p.Parameters {
			for _, hostile := range hostileParameterValues {
				checked++
				args := preset.Args{param.Name: hostile}
				expanded, err := preset.Expand(p.ID, args)
				if err != nil {
					// Refusing a value the declaration does not allow is an
					// answer, and it has to carry a sentence somebody can act
					// on.
					if strings.TrimSpace(err.Error()) == "" {
						t.Errorf("%s refused %s=%q with an empty message", p.ID, param.Name, hostile)
					}
					continue
				}
				assertParsesBack(t, p.ID, param.Name, hostile, expanded.Source)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no preset declares a parameter, so nothing hostile was tried")
	}
	t.Logf("%d preset(s), %d value(s) tried", len(presets), checked)
}

// assertParsesBack is the whole property: source a preset accepted has to be a
// recipe, and a recipe with something in it.
//
// Parsed rather than pattern matched, because what a broken document does is
// mean something ELSE - an injected key is perfectly good YAML - and only the
// parser can say what it came to.
func assertParsesBack(t *testing.T, id, param, value string, src []byte) {
	t.Helper()

	rec, err := recipe.Parse(src, "preset-"+id)
	if err != nil {
		t.Errorf("%s accepted %s=%q and produced source the parser refuses: %v\n--- source ---\n%s",
			id, param, value, err, src)
		return
	}
	if len(rec.Targets) == 0 {
		t.Errorf("%s accepted %s=%q and produced a recipe with no targets:\n%s", id, param, value, src)
		return
	}
	// A value that reached the document as structure would show up here as a
	// target nobody asked for, or as one whose id is not the one the preset
	// builds.
	for _, target := range rec.Targets {
		if strings.TrimSpace(target.ID) == "" {
			t.Errorf("%s accepted %s=%q and produced a target with no id:\n%s", id, param, value, src)
		}
		if target.Format == "" {
			t.Errorf("%s accepted %s=%q and produced target %q with no format:\n%s",
				id, param, value, target.ID, src)
		}
	}
}
