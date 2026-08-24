package recipe

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
)

// SchemaVersion is the recipe schema this build understands. It is versioned
// separately from the tool - an older recipe keeps working on a newer tool,
// and a recipe can require a range of tool versions instead.
const SchemaVersion = 1

// DefaultSeed is the seed a run uses when the recipe does not name one.
//
// It is the zero value of the field, and that is exactly why it is written
// down rather than left implicit. The window shows what happens if the box is
// left empty, and reading that off a zero value would be a second copy of the
// number with nothing holding it to the first - the shape that once put one
// answer in tfg formats and another in the generator.
//
// Measured before it was written: a recipe with no seed and a recipe with
// seed: 0 produce the same bytes.
//
// Exported for the same reason DefaultCount is, in target.go.
const DefaultSeed = 0

// Recipe is a validated recipe. Nothing outside this package builds one, so
// holding an instance means every check in this file has already passed.
type Recipe struct {
	Version  int
	Seed     int64
	SeedSet  bool
	Defaults Defaults
	Targets  []Target
	Output   Output
}

// Defaults are inherited by every target. One level of inheritance and no
// deep merging - a chain is unreadable in a diff, and the diff is the product.
type Defaults struct {
	Label bool
}

// Target is one group of files.
type Target struct {
	ID string
	// Size is what the recipe wrote, kept for the message a person reads.
	Size string
	// Sizes is one exact size per file, resolved and known before anything is
	// written. A boundary set puts three consecutive sizes here under one id.
	Sizes []int64
	// BoundaryLimit is the limit a boundary set was built around, and zero
	// when this target is not one. It is what lets the three files say which
	// of them is under the limit, on it and over it.
	BoundaryLimit int64
	Format        string
	Name          string
	Label         bool
	// Expected is what the system under test should do with these files, and
	// ExpectedReason is why, from the closed list in docs/MANIFEST.md.
	Expected       string
	ExpectedReason string
	// Group names the class of case these files belong to, and it reaches the
	// manifest so a test can assert about a whole class at once rather than
	// file by file - "every file in extension-content-mismatch was rejected".
	//
	// Presets are what fill it in practice, and it is a plain recipe key
	// rather than something a preset passes around the recipe on purpose: PR5
	// says an ejected preset is an ordinary recipe, and a field only a preset
	// could set would make the ejected copy produce something different from
	// the preset it came from.
	Group      string
	Properties map[string]string
	// Contains is what a container holds, one entry per group.
	Contains []Content
	// SizeFromContents is set when contains was given without a size, so the
	// container works the size out from what it holds.
	SizeFromContents bool
	// SizeMin and SizeMax hold the range when the target asked for one, and
	// SizeIsRange says it did. The sizes themselves are not settled here.
	//
	// They cannot be. A range is drawn from the seed, and the --seed flag
	// overrides the recipe after this package has finished reading it, so a
	// size drawn at validation time would belong to a different run than the
	// one the manifest describes. The engine draws them, which is still before
	// anything reaches the disk, so AR10 holds and --dry-run stays exact.
	SizeIsRange bool
	SizeMin     int64
	SizeMax     int64
}

// Content is one group of files inside a container.
//
// It is declared here rather than reused from the format package so that the
// recipe stays a description of a recipe. The format layer sits below this one
// and the mapping is one line in the caller.
type Content struct {
	Format string
	Count  int
	Bytes  int64
}

// Count is how many files this target produces.
func (t Target) Count() int { return len(t.Sizes) }

// Output says where the run lands.
type Output struct {
	Dir      string
	Manifest string
}

// MaxBytes is the largest recipe this build will read.
//
// A recipe comes from somebody else's repository - it can arrive in a pull
// request - so its size is chosen by a stranger. Reading time grows with it,
// and the cost sits inside the YAML parser where nothing we do afterwards can
// reduce it. The only lever before parsing is the size of the input.
//
// Measured on 2026-08-02: a deliberately nested document of 80 kB cost about
// 1.3 s of parsing over the baseline, and the growth is faster than linear. A
// megabyte caps the worst case at seconds rather than minutes, and is far above
// any recipe a person writes - ten thousand targets spelled out in full come to
// roughly half of it.
//
// What this does not do: it does not defend against a recipe that fits and is
// still expensive. Seconds on a hostile file are acceptable, minutes were not.
const MaxBytes = 1 << 20

// TooLargeError is returned for a recipe past MaxBytes.
type TooLargeError struct {
	Name  string
	Bytes int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf(
		"%s is %d B and the limit is %d B. A recipe is a document somebody writes, and reading time grows with its size, so an unbounded one is a way to hang a build. Split it into several recipes, or generate the targets with a loop in your own script",
		e.Name, e.Bytes, MaxBytes)
}

// Parse reads a recipe and returns it only when every check passes.
//
// Nothing is written before this succeeds, and it reports every problem at
// once rather than the first one. Fixing a recipe one error per run is the
// cheapest way to make someone stop using the tool.
func Parse(src []byte, name string) (*Recipe, error) {
	// Checked here as well as before the read, because this is the door every
	// caller comes through - including the fuzz target, which hands over bytes
	// that never were a file.
	if int64(len(src)) > MaxBytes {
		return nil, &TooLargeError{Name: name, Bytes: int64(len(src))}
	}

	var raw rawRecipe

	// A recipe is UTF-8, and anything else is refused rather than read as best
	// it can be.
	//
	// Measured on 2026-08-04: a recipe saved as cp1250 with Polish letters in a
	// file name was accepted with exit 0, and the file arrived called "za"
	// followed by four replacement characters. The decoder had turned every
	// byte it could not read into U+FFFD, the manifest recorded the same
	// mangled name, so verify agreed with the disk and nothing anywhere said
	// the name was not the one that was asked for.
	//
	// Notepad on Windows still offers ANSI when saving, and this tool is aimed
	// at testers on Windows. The same reasoning as the byte order mark below,
	// one step earlier: what somebody typed is what they get, or they are told
	// why not.
	if !utf8.Valid(src) {
		return nil, &SyntaxError{Name: name, Detail: "this file is not valid UTF-8. Every character that could not be read would come back as a replacement mark, so a name written with accents would produce a file called something else. Save the file as UTF-8 and try again"}
	}

	// An editor that writes a byte order mark would otherwise hand the decoder
	// a first key nobody typed. Dropped here rather than in one of the two
	// readers below, so both see the same bytes.
	src = withoutBOM(src)

	// One file is one recipe. Everything after a document separator would be
	// dropped by the decoder, which means somebody gets half the fixtures they
	// asked for and a run that says it went fine.
	if _, err := oneDocument(src, name); err != nil {
		return nil, err
	}

	// Strict decoding turns an unknown key into an error. A typo in
	// "siez: 10mb" accepted in silence gives a file of the default size and an
	// hour spent wondering why the test passes when it should not.
	if err := decodeStrict(src, &raw); err != nil {
		return nil, &SyntaxError{Name: name, Detail: strings.TrimRight(err.Error(), "\n")}
	}

	p := &problems{name: name}
	rec := raw.validate(p)
	if err := p.err(); err != nil {
		return nil, err
	}
	return rec, nil
}

// decodeStrict runs the YAML decoder and turns a crash inside it into an error.
//
// The decoder is the one dependency in this project, and a recipe is a file
// somebody else wrote - so a crash in there is a crash on ordinary user input.
// Found by fuzzing on 2026-08-02: "targets: ! " is a tag indicator with nothing
// after it, and goccy/go-yaml v1.19.2 dereferences nil on it. What reached the
// user was a Go stack trace on standard error and exit code 2, which the frozen
// table says means a usage error - an ending nothing downstream can tell apart
// from a mistyped flag.
//
// Scoped to this one call rather than to the whole of Parse. A crash in our own
// validation should still arrive as a crash, not be quietly relabelled as a
// problem with the user's file.
func decodeStrict(src []byte, raw *rawRecipe) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// The panic value itself says "invalid memory address", which tells
			// a person testing an upload form nothing at all. What helps is the
			// shape of the thing that does it.
			err = fmt.Errorf("this file could not be read as YAML. Look for a tag or anchor marker such as ! or & with nothing after it")
		}
	}()
	return yaml.UnmarshalWithOptions(src, raw, yaml.Strict())
}

// rawRecipe carries every key the recipe document describes, including the
// ones this build cannot honour yet.
//
// Listing them is deliberate. Leaving them out would make "policy:" fail as an
// unknown key, which is misleading - it is a documented key that arrives
// later. A key we cannot honour has to say so in those words.
type rawRecipe struct {
	// Every number here is a scalar rather than an int, so that we and not
	// YAML decide what the digits mean. See scalar.go for the five spellings
	// this quietly got wrong before.
	Version *scalar `yaml:"version"`
	Engine  *scalar `yaml:"engine"`
	Seed    *scalar `yaml:"seed"`
	Locale  *scalar `yaml:"locale"`

	Defaults *rawDefaults `yaml:"defaults"`
	Targets  []rawTarget  `yaml:"targets"`

	AllowNondeterministic *scalar `yaml:"allow_nondeterministic"`

	Policy  map[string]any `yaml:"policy"`
	Extends *scalar        `yaml:"extends"`
	With    map[string]any `yaml:"with"`

	Output *rawOutput `yaml:"output"`
}

type rawDefaults struct {
	Label *scalar `yaml:"label"`
	Fill  *scalar `yaml:"fill"`
}

type rawOutput struct {
	Dir            *scalar `yaml:"dir"`
	Manifest       *scalar `yaml:"manifest"`
	SplitThreshold *scalar `yaml:"split_threshold"`
}

func (raw rawRecipe) validate(p *problems) *Recipe {
	rec := &Recipe{
		Version:  SchemaVersion,
		Defaults: Defaults{Label: true},
		Output:   Output{Dir: ".", Manifest: "manifest.json"},
	}

	version, versionIsNumber := int64(0), false
	if raw.Version != nil {
		version, versionIsNumber = raw.Version.number()
	}
	switch {
	case raw.Version == nil:
		p.add("version", "the recipe has no version",
			"every recipe declares the schema version it was written against",
			fmt.Sprintf("add version: %d as the first line", SchemaVersion))
	case !versionIsNumber:
		p.add("version", fmt.Sprintf("version %q is not a whole number", raw.Version.text),
			"the version decides how the rest of the file is read, so it is never guessed at",
			fmt.Sprintf("write version: %d", SchemaVersion))
	case version != SchemaVersion:
		p.add("version", fmt.Sprintf("version %d is not a schema this build knows", version),
			fmt.Sprintf("this build understands version %d", SchemaVersion),
			"upgrade the tool, or write the recipe against the version above")
	}

	raw.refuseUnsupported(p)
	raw.applySettings(p, rec)

	if len(raw.Targets) == 0 {
		p.add("targets", "the recipe asks for no files",
			"a recipe without targets has nothing to produce",
			"add at least one entry under targets:")
		return rec
	}

	seen := map[string]bool{}
	for i, rt := range raw.Targets {
		t := rt.validate(p, i, rec.Defaults)
		if t.ID != "" {
			if seen[t.ID] {
				p.add(targetSpot(i, t.ID).of("id"), fmt.Sprintf("target {setting} %q is used twice", t.ID),
					"{a} {setting} identifies a target, anchors its seed and links it to the manifest",
					"give one of them a different {setting}")
			}
			seen[t.ID] = true
		}
		rec.Targets = append(rec.Targets, t)
	}
	return rec
}

// refuseUnsupported names every top level key the document describes and this
// build cannot honour.
//
// Accepting one in silence would produce a run that quietly ignores what was
// asked for, which is the one thing this tool must never do.
func (raw rawRecipe) refuseUnsupported(p *problems) {
	if raw.Engine != nil {
		p.notYet("engine", "the tool version this recipe requires is not checked yet",
			"remove the line - the manifest records the version that ran")
	}
	if locale, ok := oneValue(p, "locale", "locale", "locale: en", raw.Locale); ok && locale != "en" {
		p.add("locale", fmt.Sprintf("locale %q is not available in this build", locale),
			"generated content is English only so far",
			"use locale: en, or leave the line out")
	}
	if on, ok := oneFlag(p, "allow_nondeterministic", "allow_nondeterministic", raw.AllowNondeterministic); ok && on {
		p.add("allow_nondeterministic", "allow_nondeterministic: true has nothing to allow in this build",
			"every format here repeats to the byte, so no consent is needed",
			"remove the line - it will be needed by the formats that use a system encoder")
	}
	if raw.Policy != nil {
		p.notYet("policy", "unspecified expectations are left in the manifest for the consumer to settle",
			"remove the section - the expected field on a target already works")
	}
	// The reason these two give used to be "presets are not in this build",
	// and presets arrived on 2026-08-05 while this sentence stayed. A recipe
	// still cannot name one - that is what is missing - so the key is refused
	// for the same reason as before and the sentence now says which.
	if raw.Extends != nil {
		p.notYet("extends", "a recipe cannot build on a preset yet, though the command line can run one",
			"run \"tfg preset eject <id> > recipe.yaml\" and edit the targets, or write them out in full")
	}
	if raw.With != nil {
		p.notYet("with", "a recipe cannot build on a preset yet, though the command line can run one",
			"run \"tfg preset eject <id> > recipe.yaml\" and edit the targets, or write them out in full")
	}
}

// applySettings copies the seed, the defaults and the output settings onto the
// recipe, refusing the parts of each that are not honoured yet.
func (raw rawRecipe) applySettings(p *problems, rec *Recipe) {
	if raw.Seed != nil {
		n, ok := raw.Seed.number()
		if !ok {
			p.add("seed", fmt.Sprintf("seed %q is not a whole number", raw.Seed.text),
				"the seed decides every byte of the run, so it is read exactly as written and never guessed at",
				"write a decimal number such as seed: 20260802")
		} else {
			rec.Seed = n
			rec.SeedSet = true
		}
	}

	if raw.Defaults != nil {
		if raw.Defaults.Fill != nil {
			p.notYet("defaults.fill", "the fill mode is not settable yet",
				"remove the line - content is generated from the seed")
		}
		if on, ok := oneFlag(p, "defaults.label", "defaults.label", raw.Defaults.Label); ok {
			rec.Defaults.Label = on
		}
	}

	if raw.Output != nil {
		if raw.Output.SplitThreshold != nil {
			p.notYet("output.split_threshold", "the manifest is always written as one file so far",
				"remove the line")
		}
		if dir, ok := oneValue(p, "output.dir", "output.dir", "dir: ./fixtures", raw.Output.Dir); ok {
			rec.Output.Dir = dir
		}
		if name, ok := oneValue(p, "output.manifest", "output.manifest", "manifest: manifest.json", raw.Output.Manifest); ok {
			rec.Output.Manifest = name
		}
	}
}

// sortedKeys is map iteration with the randomness taken out, for the places
// where the keys become text somebody reads.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// scalarText renders a YAML scalar the way a person wrote it.
//
// YAML types numbers on its own, so 3 arrives as an integer and 1.7 as a
// float. Both have to come out as the text --set would have carried, or the
// two surfaces would disagree about the same recipe.
func scalarText(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	case uint64:
		return strconv.FormatUint(x, 10), true
	case float64:
		// -1 keeps the shortest form that reads back to the same number, so
		// 1.7 does not turn into 1.7000000000000002.
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return "", false
	}
}
