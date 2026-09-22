package recipe

import (
	"fmt"
	"sort"
	"strings"
)

// A recipe that builds on a preset.
//
// The recipe names the preset and fills its parameters, and its own targets
// are added after the preset's - so the file is the same run as "tfg preset
// eject" followed by editing, and shorter, and it says which test question
// the set came from. docs/RECIPE.md section 6, and the analysis in
// docs/EXTENDS-WITH-2026-09-22.md.
//
// This package cannot expand the preset, because it cannot import the preset
// package - the layer rule runs the other way. So reading such a file is two
// steps with the expansion between them, and both steps are here: ExtensionOf
// says what the file builds on, and ParseExtending reads the file with the
// preset's targets in front of its own. Whoever knows presets - internal/preset
// - joins the two, and every surface reads a file through that door.

// presetScheme is what an extends value starts with. The only scheme this
// build knows: a recipe cannot build on another file yet, and saying so is
// better than guessing which of the two a bare name meant.
const presetScheme = "preset:"

// Extension is what a recipe says it builds on.
type Extension struct {
	// Preset is the id named after "preset:".
	Preset string
	// With is the parameters the recipe fills, as the text somebody would
	// type into the flag of the same name. Absent when the recipe fills
	// none, so every parameter stands in from its declared default.
	With map[string]string
}

// ExtensionOf reads what a recipe builds on, and nil when it stands alone.
//
// It refuses what Parse would refuse about the document itself, so a file
// that is not a recipe is turned away here rather than expanded first. What
// it refuses about the two keys - with and no extends, a scheme this build
// does not know, a parameter written as a list - is worded the same way
// ParseExtending words it, because both call extension.
func ExtensionOf(src []byte, name string) (*Extension, error) {
	raw, err := decode(src, name)
	if err != nil {
		return nil, err
	}
	p := &problems{name: name}
	ext := raw.extension(p)
	if err := p.err(); err != nil {
		return nil, err
	}
	return ext, nil
}

// extension reads extends and with, refusing what cannot be read.
func (raw rawRecipe) extension(p *problems) *Extension {
	if raw.Extends == nil {
		if raw.With != nil {
			p.add(KeyWith, "with fills the parameters of a preset, and no preset is named",
				"with belongs beside extends: it says what to fill in, and extends says what to fill it into",
				"add extends: preset:<id> above it, or remove with")
		}
		return nil
	}
	value, ok := oneValue(p, KeyExtends, KeyExtends, "extends: preset:size-boundaries", raw.Extends)
	if !ok {
		return nil
	}
	id := strings.TrimPrefix(value, presetScheme)
	if id == value || id == "" {
		p.add(KeyExtends, fmt.Sprintf("extends %q does not name a preset", value),
			"a recipe can build on a preset, and on nothing else in this build - not on another file",
			"write extends: preset:<id>, and run \"tfg preset list\" for the ids this build has")
		return nil
	}
	ext := &Extension{Preset: id}
	if len(raw.With) == 0 {
		return ext
	}
	ext.With = make(map[string]string, len(raw.With))
	// In name order, so two runs over one file word their refusals in one
	// order - a map is walked in whatever order it likes.
	names := make([]string, 0, len(raw.With))
	for n := range raw.With {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := raw.With[n]
		text, ok := oneValue(p, KeyWith+"."+n, KeyWith+"."+n,
			fmt.Sprintf("%s: %s", n, "1B,1kb,1mb"), &s)
		if !ok {
			continue
		}
		ext.With[n] = text
	}
	return ext
}

// ParseExtending reads a recipe that builds on a preset, given the preset's
// expansion.
//
// The preset's targets come first and the file's own after them, and then the
// whole is judged once, by the same rules and in the same place as a recipe
// that stands alone - so an id used by both, a size under a format's floor,
// a name that cannot be written, are all refused the way they always were.
// Everything that is not a target comes from the file: the seed, the
// defaults, the output section. The preset may carry nothing else, and
// CheckBase is what holds a preset to that.
//
// Refusals about the file's own targets count from the file's first target,
// not the merged list's, so the position a screen registered a box under is
// the position the refusal names.
func ParseExtending(src []byte, name string, base []byte) (*Recipe, error) {
	own, err := decode(src, name)
	if err != nil {
		return nil, err
	}
	// The expansion is decoded strictly too. A preset that expanded into a
	// key this build does not know would be refused on the command line's
	// --preset path, and this path refuses it the same way rather than
	// merging the half it understands.
	from, err := decode(base, name)
	if err != nil {
		return nil, err
	}
	p := &problems{name: name}
	if own.extension(p) == nil {
		// A file that does not extend anything was handed a base. That is a
		// caller's mistake rather than the file's, and reading the file as
		// if it did would run targets the file never asked for.
		if p.err() == nil {
			p.add(KeyExtends, "this recipe does not build on a preset",
				"a preset's targets were supplied to the reader, and the recipe names none",
				"read the file with Parse, or add extends: preset:<id>")
		}
		return nil, p.err()
	}
	if err := from.onlyTargets(); err != nil {
		p.add(KeyExtends, err.Error(),
			"a preset contributes targets and nothing else, so that the seed, the defaults and the output section are always the file's own",
			"this is a mistake in the preset rather than in the recipe - run \"tfg preset eject\" and edit the result instead")
		return nil, p.err()
	}

	merged := own
	merged.Extends = nil
	merged.With = nil
	merged.Targets = append(append([]rawTarget{}, from.Targets...), own.Targets...)

	rec := merged.validate(p, len(from.Targets))
	if err := p.err(); err != nil {
		return nil, err
	}
	return rec, nil
}

// CheckBase says whether a preset's expansion is one a recipe can build on:
// a version and a list of targets, and nothing else.
//
// A guard asks this of every registered preset, so the rule that the file
// owns everything but the targets is held by every preset before any recipe
// extends it. ParseExtending asks it again of the one it was handed, because
// a layer that trusts the layer above it to have checked is a layer with a
// hole in it.
func CheckBase(src []byte, name string) error {
	raw, err := decode(src, name)
	if err != nil {
		return err
	}
	return raw.onlyTargets()
}

// onlyTargets is the rule behind CheckBase, on a decoded document.
func (raw rawRecipe) onlyTargets() error {
	var carries []string
	if raw.Seed != nil {
		carries = append(carries, KeySeed)
	}
	if raw.Engine != nil {
		carries = append(carries, KeyEngine)
	}
	if raw.Locale != nil {
		carries = append(carries, KeyLocale)
	}
	if raw.Defaults != nil {
		carries = append(carries, "defaults")
	}
	if raw.AllowNondeterministic != nil {
		carries = append(carries, "allow_nondeterministic")
	}
	if raw.Policy != nil {
		carries = append(carries, KeyPolicy)
	}
	if raw.Extends != nil {
		carries = append(carries, KeyExtends)
	}
	if raw.With != nil {
		carries = append(carries, KeyWith)
	}
	if raw.Output != nil {
		carries = append(carries, "output")
	}
	if len(carries) == 0 {
		return nil
	}
	return fmt.Errorf("the preset's recipe carries %s, which a recipe building on it cannot inherit",
		strings.Join(carries, ", "))
}
