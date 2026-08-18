package recipe

import (
	"fmt"

	"github.com/goccy/go-yaml"
)

// Writing a recipe from values somebody typed, rather than reading one.
//
// This exists for the window. A screen that edits a recipe has to end up with a
// document, because the document is what gets judged - Parse holds every rule
// about sizes, counts, clashing keys and closed lists, and a surface that
// decided any of them itself would be a second opinion the engine never agreed
// to. So the screen collects text and this turns the text into a recipe.
//
// It lives here rather than in the window for two reasons. The recipe package
// owns the shape of the document, so a key added to the schema is added in one
// place. And nothing above layer 2 has any business knowing this is YAML.
//
// Hand written YAML would have been the obvious way and it is the wrong one.
// The size-boundaries preset writes its document with fmt.Fprintf, and the
// comment above it explains that this is safe because every value is one the
// package built itself. That comment was wrong until 2026-08-05: the id carried
// the caller's text, so "1\rB" reached the document raw and broke it, because a
// carriage return sits in the middle where the size parser trims only the ends.
// Found by fuzzing rather than by reading.
//
// Here EVERY value is somebody's typing. A person can put a colon, a quote, a
// newline or a carriage return in a file name template, and any of those would
// end a hand written line early - producing either a syntax error blamed on the
// user or, far worse, a document that parses into something they did not ask
// for. So this marshals, and the library does the quoting.

// Document is a recipe as a screen holds it: every value as the text somebody
// typed, none of it judged yet.
//
// Text rather than parsed values throughout, and that is the point rather than
// laziness. A size is "2mb" until Parse says otherwise, a count is "ten" until
// Parse refuses it, and the refusal that comes back names the setting - so the
// box that was wrong gets marked without the window ever having parsed
// anything. See internal/gui/window and docs/GUI.md section 2, G1.
//
// An empty string is left out of the document entirely, which is how a surface
// says "I did not state this". Untouchable rule 5 needs that difference to
// exist: a window that filled every key in could never record which values were
// its own invention.
type Document struct {
	Seed     string
	OutDir   string
	Manifest string
	// Label is defaults.label, and a pointer because a switch has no third
	// position for silence. Nil leaves the defaults section out.
	Label   *bool
	Targets []TargetDraft
}

// TargetDraft is one entry of the targets list, as typed.
type TargetDraft struct {
	ID     string
	Format string
	Count  string
	Name   string
	Group  string
	// Three ways of saying how big, and this type deliberately accepts all
	// three at once. Two of them together is a refusal Parse already words and
	// addresses - "states both a size and a size-range" - so a screen offering
	// three boxes gets that refusal placed under the right one, and does not
	// need a mode of its own to keep in step with the rules.
	Size      string
	SizeRange string
	Boundary  string

	Expected       string
	ExpectedReason string

	Properties map[string]string
	Contains   []ContentDraft
}

// ContentDraft is one entry of a contains list, as typed.
type ContentDraft struct {
	Format string
	Count  string
	Size   string
}

// Compose writes the document these values describe.
//
// The result is source rather than a parsed recipe, because source is what the
// rest of the tool consumes: Parse judges it, Hash identifies it for the
// manifest, and a run started from a window is then the same run as one started
// from a file.
func Compose(d Document) ([]byte, error) {
	// Refused before anything is written, and addressed, so a window marks the
	// box rather than showing a sentence about a character nobody can see.
	if err := refuseUnwritable(d); err != nil {
		return nil, err
	}

	doc := yaml.MapSlice{{Key: "version", Value: SchemaVersion}}

	if d.Seed != "" {
		doc = append(doc, yaml.MapItem{Key: "seed", Value: d.Seed})
	}
	if d.Label != nil {
		doc = append(doc, yaml.MapItem{Key: "defaults",
			Value: yaml.MapSlice{{Key: "label", Value: *d.Label}}})
	}
	if out := outputSection(d); len(out) > 0 {
		doc = append(doc, yaml.MapItem{Key: "output", Value: out})
	}

	// Always present, even when empty. A document with no targets is refused by
	// Parse with a sentence about asking for no files, which is the answer a
	// screen with nothing on it should get - better than a syntax error, and
	// better than silence.
	targets := make([]yaml.MapSlice, 0, len(d.Targets))
	for _, t := range d.Targets {
		targets = append(targets, targetEntry(t))
	}
	doc = append(doc, yaml.MapItem{Key: "targets", Value: targets})

	return yaml.Marshal(doc)
}

func outputSection(d Document) yaml.MapSlice {
	var out yaml.MapSlice
	if d.OutDir != "" {
		out = append(out, yaml.MapItem{Key: "dir", Value: d.OutDir})
	}
	if d.Manifest != "" {
		out = append(out, yaml.MapItem{Key: "manifest", Value: d.Manifest})
	}
	return out
}

// targetEntry writes one target, in the order somebody fills a form in rather
// than the order the reader happens to check things.
func targetEntry(t TargetDraft) yaml.MapSlice {
	entry := yaml.MapSlice{}
	add := func(key, value string) {
		if value != "" {
			entry = append(entry, yaml.MapItem{Key: key, Value: value})
		}
	}

	add("id", t.ID)
	add("format", t.Format)
	add("count", t.Count)
	add("size", t.Size)
	add("size-range", t.SizeRange)
	add("boundary", t.Boundary)
	add("name", t.Name)
	add("group", t.Group)

	if e := expectationEntry(t); e != nil {
		entry = append(entry, yaml.MapItem{Key: "expected", Value: e})
	}
	if len(t.Properties) > 0 {
		props := yaml.MapSlice{}
		// Sorted, because map order in Go is randomised and this text is
		// hashed into the manifest. Two runs of one screen have to compose the
		// same bytes or recipe_hash would move on its own.
		for _, name := range sortedKeys(t.Properties) {
			props = append(props, yaml.MapItem{Key: name, Value: t.Properties[name]})
		}
		entry = append(entry, yaml.MapItem{Key: "properties", Value: props})
	}
	if len(t.Contains) > 0 {
		inside := make([]yaml.MapSlice, 0, len(t.Contains))
		for _, c := range t.Contains {
			one := yaml.MapSlice{}
			if c.Format != "" {
				one = append(one, yaml.MapItem{Key: "format", Value: c.Format})
			}
			if c.Count != "" {
				one = append(one, yaml.MapItem{Key: "count", Value: c.Count})
			}
			if c.Size != "" {
				one = append(one, yaml.MapItem{Key: "size", Value: c.Size})
			}
			inside = append(inside, one)
		}
		entry = append(entry, yaml.MapItem{Key: "contains", Value: inside})
	}
	return entry
}

// expectationEntry writes the short form when there is no reason and the long
// one when there is, which is the same choice a person writing the file by hand
// makes. Nil when nothing was stated.
//
// A reason with no outcome is written out as it stands rather than tidied away,
// because Parse refuses that pairing with a sentence saying an expectation needs
// an outcome. Dropping it here would turn a refusal into silence.
func expectationEntry(t TargetDraft) any {
	switch {
	case t.Expected == "" && t.ExpectedReason == "":
		return nil
	case t.ExpectedReason == "":
		return t.Expected
	default:
		return yaml.MapSlice{
			{Key: "outcome", Value: t.Expected},
			{Key: "reason", Value: t.ExpectedReason},
		}
	}
}

// refuseUnwritable rejects text that cannot make the round trip into a document
// and back out as itself.
//
// Measured on 2026-08-18 against goccy/go-yaml v1.19.2, and both findings are
// the library's rather than ours. A carriage return inside a value makes Marshal
// emit a literal block scalar that its OWN parser then rejects, so the tool
// would write a broken document and blame whoever typed it. A tab is dropped in
// silence: "one\ttwo" is composed and read back as "onetwo".
//
// Neither is acceptable and neither can be fixed here, so the third answer is to
// say no. That is the same choice parseSpread made for the size-boundaries
// preset after a carriage return broke its document - refuse the characters a
// value is not written with, rather than trusting the writer to escape them.
//
// Every control character goes, not only the two measured. A tab, a newline or a
// bell in a group name, an id or a file name is never what somebody meant, the
// two that are known to break are invisible on screen, and a rule covering the
// class does not have to be revisited the next time the library changes how it
// quotes. The refusals are addressed, so each lands under its own box.
func refuseUnwritable(d Document) error {
	p := &problems{name: composedName}

	check := func(at, value string) {
		if bad, found := firstControl(value); found {
			p.add(at,
				fmt.Sprintf("%s holds the character %q, which cannot be written to a recipe", at, bad),
				"a recipe is a text document, and a control character either breaks it or is dropped without a word",
				"remove it - it is usually a stray tab or a line break from pasting")
		}
	}

	check("seed", d.Seed)
	check("output.dir", d.OutDir)
	check("output.manifest", d.Manifest)

	for i, t := range d.Targets {
		where := targetSpot(i, t.ID)
		check(where.of("id"), t.ID)
		check(where.of("format"), t.Format)
		check(where.of("count"), t.Count)
		check(where.of("size"), t.Size)
		check(where.of("size-range"), t.SizeRange)
		check(where.of("boundary"), t.Boundary)
		check(where.of("name"), t.Name)
		check(where.of("group"), t.Group)
		check(where.of("expected"), t.Expected)
		check(where.of("expected.reason"), t.ExpectedReason)

		// Sorted, so a document with two bad properties reports them in the
		// same order twice.
		for _, name := range sortedKeys(t.Properties) {
			check(where.of("properties."+name), t.Properties[name])
		}
		for j, c := range t.Contains {
			inside := where.entry("contains", j)
			check(inside.of("format"), c.Format)
			check(inside.of("count"), c.Count)
			check(inside.of("size"), c.Size)
		}
	}

	return p.err()
}

// composedName stands where a file name stands for a recipe read off the disk.
// A screen has no file, and the whole of this refusal is placed under the boxes
// it names, so this is what is left when something has to be called something.
const composedName = "the settings on screen"

// firstControl is the first character in a value that a document cannot carry.
//
// Runes rather than bytes, so a multi byte character is never mistaken for a
// control code by looking at one of its halves. DEL goes with the C0 block: it
// is equally invisible and equally never meant.
func firstControl(value string) (rune, bool) {
	for _, r := range value {
		if r < 0x20 || r == 0x7F {
			return r, true
		}
	}
	return 0, false
}

// The names of the settings a surface has to address, and the shape of an
// address itself.
//
// Exported for the window. A screen that edits a recipe draws a field per
// setting and has to register each under the name a refusal will arrive with, so
// without these it would hold fifteen string literals that agree with this
// package by coincidence - and the day one of them was misspelled, the refusal
// would land at the foot of the form with the box beside it unmarked. Silent,
// and green in every test that did not press that exact field.
//
// So the vocabulary lives here, where the reader that produces the addresses
// lives too. What that still cannot prove is that a screen registers the RIGHT
// name for a given box: both sides can agree and both be wrong. That is what
// TestEveryRefusalAboutABatchMarksThatBatchsBox is for, and the two together are
// what make the pairing safe rather than merely consistent.
const (
	KeyTargets  = "targets"
	KeyVersion  = "version"
	KeySeed     = "seed"
	KeyLocale   = "locale"
	KeyPolicy   = "policy"
	KeyEngine   = "engine"
	KeyExtends  = "extends"
	KeyWith     = "with"
	KeyContains = "contains"

	KeyID             = "id"
	KeyFormat         = "format"
	KeyCount          = "count"
	KeySize           = "size"
	KeySizeRange      = "size-range"
	KeyBoundary       = "boundary"
	KeyName           = "name"
	KeyGroup          = "group"
	KeyLabel          = "label"
	KeyFill           = "fill"
	KeyMutations      = "mutations"
	KeyProperties     = "properties"
	KeyExpected       = "expected"
	KeyExpectedReason = "expected.reason"

	KeyOutputDir      = "output.dir"
	KeyOutputManifest = "output.manifest"
	KeyDefaultsLabel  = "defaults.label"
)

// TargetAddress is where one setting of one target lives.
//
// The position counts from one, matching the prose: a refusal about the second
// target says "target 2", and a screen numbering its blocks from zero would send
// somebody to the wrong one.
func TargetAddress(position int, setting string) string {
	return targetPrefix(position) + "." + setting
}

// targetPrefix is one target without a setting named yet, which is what a
// refusal about the target as a whole is addressed to.
func targetPrefix(position int) string {
	return fmt.Sprintf("%s[%d]", KeyTargets, position)
}

// ContentAddress is where one setting of one contains entry lives. Both
// positions count from one, for the reason above.
func ContentAddress(target, entry int, setting string) string {
	return fmt.Sprintf("%s.%s[%d].%s", targetPrefix(target), KeyContains, entry, setting)
}
