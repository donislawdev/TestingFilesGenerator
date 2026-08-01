package recipe

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// SchemaVersion is the recipe schema this build understands. It is versioned
// separately from the tool - an older recipe keeps working on a newer tool,
// and a recipe can require a range of tool versions instead.
const SchemaVersion = 1

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
	Sizes  []int64
	Format string
	Name   string
	Label  bool
	// Expected is what the system under test should do with these files, and
	// ExpectedReason is why, from the closed list in docs/MANIFEST.md.
	Expected       string
	ExpectedReason string
	Properties     map[string]string
}

// Count is how many files this target produces.
func (t Target) Count() int { return len(t.Sizes) }

// Output says where the run lands.
type Output struct {
	Dir      string
	Manifest string
}

// Parse reads a recipe and returns it only when every check passes.
//
// Nothing is written before this succeeds, and it reports every problem at
// once rather than the first one. Fixing a recipe one error per run is the
// cheapest way to make someone stop using the tool.
func Parse(src []byte, name string) (*Recipe, error) {
	var raw rawRecipe

	// One file is one recipe. Everything after a document separator would be
	// dropped by the decoder, which means somebody gets half the fixtures they
	// asked for and a run that says it went fine.
	if _, err := oneDocument(src, name); err != nil {
		return nil, err
	}

	// Strict decoding turns an unknown key into an error. A typo in
	// "siez: 10mb" accepted in silence gives a file of the default size and an
	// hour spent wondering why the test passes when it should not.
	if err := yaml.UnmarshalWithOptions(src, &raw, yaml.Strict()); err != nil {
		return nil, &SyntaxError{Name: name, Detail: strings.TrimRight(err.Error(), "\n")}
	}

	p := &problems{name: name}
	rec := raw.validate(p)
	if err := p.err(); err != nil {
		return nil, err
	}
	return rec, nil
}

// rawRecipe carries every key the recipe document describes, including the
// ones this build cannot honour yet.
//
// Listing them is deliberate. Leaving them out would make "policy:" fail as an
// unknown key, which is misleading - it is a documented key that arrives
// later. A key we cannot honour has to say so in those words.
type rawRecipe struct {
	Version *int    `yaml:"version"`
	Engine  *string `yaml:"engine"`
	Seed    *int64  `yaml:"seed"`
	Locale  *string `yaml:"locale"`

	Defaults *rawDefaults `yaml:"defaults"`
	Targets  []rawTarget  `yaml:"targets"`

	AllowNondeterministic *bool `yaml:"allow_nondeterministic"`

	Policy  map[string]any `yaml:"policy"`
	Extends *string        `yaml:"extends"`
	With    map[string]any `yaml:"with"`

	Output *rawOutput `yaml:"output"`
}

type rawDefaults struct {
	Label *bool   `yaml:"label"`
	Fill  *string `yaml:"fill"`
}

type rawTarget struct {
	ID     *string `yaml:"id"`
	Format *string `yaml:"format"`
	Count  *int    `yaml:"count"`
	Name   *string `yaml:"name"`
	Label  *bool   `yaml:"label"`

	// Size accepts what a person writes: 2mb as text, or a plain byte count
	// as a number. Refusing one of the two would be a trap rather than a rule.
	Size any `yaml:"size"`

	Properties map[string]any `yaml:"properties"`
	Expected   any            `yaml:"expected"`

	Boundary  any              `yaml:"boundary"`
	SizeRange *string          `yaml:"size-range"`
	Contains  []map[string]any `yaml:"contains"`
	Mutations []map[string]any `yaml:"mutations"`
	Fill      *string          `yaml:"fill"`
}

type rawOutput struct {
	Dir            *string `yaml:"dir"`
	Manifest       *string `yaml:"manifest"`
	SplitThreshold *int    `yaml:"split_threshold"`
}

func (raw rawRecipe) validate(p *problems) *Recipe {
	rec := &Recipe{
		Version:  SchemaVersion,
		Defaults: Defaults{Label: true},
		Output:   Output{Dir: ".", Manifest: "manifest.json"},
	}

	switch {
	case raw.Version == nil:
		p.add("the recipe has no version",
			"every recipe declares the schema version it was written against",
			fmt.Sprintf("add version: %d as the first line", SchemaVersion))
	case *raw.Version != SchemaVersion:
		p.add(fmt.Sprintf("version %d is not a schema this build knows", *raw.Version),
			fmt.Sprintf("this build understands version %d", SchemaVersion),
			"upgrade the tool, or write the recipe against the version above")
	}

	// Keys the document describes and this build cannot honour. Accepting them
	// in silence would produce a run that quietly ignores what was asked for,
	// which is the one thing this tool must never do.
	if raw.Engine != nil {
		p.notYet("engine", "the tool version this recipe requires is not checked yet",
			"remove the line - the manifest records the version that ran")
	}
	if raw.Locale != nil && *raw.Locale != "en" {
		p.add(fmt.Sprintf("locale %q is not available in this build", *raw.Locale),
			"generated content is English only so far",
			"use locale: en, or leave the line out")
	}
	if raw.AllowNondeterministic != nil && *raw.AllowNondeterministic {
		p.add("allow_nondeterministic: true has nothing to allow in this build",
			"every format here repeats to the byte, so no consent is needed",
			"remove the line - it will be needed by the formats that use a system encoder")
	}
	if raw.Policy != nil {
		p.notYet("policy", "unspecified expectations are left in the manifest for the consumer to settle",
			"remove the section - the expected field on a target already works")
	}
	if raw.Extends != nil {
		p.notYet("extends", "presets are not in this build",
			"write the targets out in full")
	}
	if raw.With != nil {
		p.notYet("with", "presets are not in this build",
			"write the targets out in full")
	}

	if raw.Seed != nil {
		rec.Seed = *raw.Seed
		rec.SeedSet = true
	}

	if raw.Defaults != nil {
		if raw.Defaults.Fill != nil {
			p.notYet("defaults.fill", "the fill mode is not settable yet",
				"remove the line - content is generated from the seed")
		}
		if raw.Defaults.Label != nil {
			rec.Defaults.Label = *raw.Defaults.Label
		}
	}

	if raw.Output != nil {
		if raw.Output.SplitThreshold != nil {
			p.notYet("output.split_threshold", "the manifest is always written as one file so far",
				"remove the line")
		}
		if raw.Output.Dir != nil {
			rec.Output.Dir = *raw.Output.Dir
		}
		if raw.Output.Manifest != nil {
			rec.Output.Manifest = *raw.Output.Manifest
		}
	}

	if len(raw.Targets) == 0 {
		p.add("the recipe asks for no files",
			"a recipe without targets has nothing to produce",
			"add at least one entry under targets:")
		return rec
	}

	seen := map[string]bool{}
	for i, rt := range raw.Targets {
		t := rt.validate(p, i, rec.Defaults)
		if t.ID != "" {
			if seen[t.ID] {
				p.add(fmt.Sprintf("target id %q is used twice", t.ID),
					"an id identifies a target, anchors its seed and links it to the manifest",
					"give one of them a different id")
			}
			seen[t.ID] = true
		}
		rec.Targets = append(rec.Targets, t)
	}
	return rec
}

func (rt rawTarget) validate(p *problems, index int, def Defaults) Target {
	t := Target{Label: def.Label}
	count := 1

	where := fmt.Sprintf("target %d", index+1)
	if rt.ID != nil && *rt.ID != "" {
		t.ID = *rt.ID
		where = fmt.Sprintf("target %q", t.ID)
	} else {
		p.add(fmt.Sprintf("%s has no id", where),
			"an id anchors the seed of a target, so editing one target never moves the bytes of another",
			"give it an id, for example id: invoices")
	}

	if rt.Format == nil || *rt.Format == "" {
		p.add(fmt.Sprintf("%s has no format", where),
			"a target has to say what kind of file it produces",
			"add format: txt, or run \"tfg formats\" to see the whole list")
	} else {
		t.Format = *rt.Format
	}

	if rt.Count != nil {
		if *rt.Count <= 0 {
			p.add(fmt.Sprintf("%s asks for %d files", where, *rt.Count),
				"a target that produces nothing is almost always a mistake rather than an intention",
				"ask for at least one, or delete the target")
		} else {
			count = *rt.Count
		}
	}

	// One of four ways to state a size. Two of them work here, and the rule
	// itself is not negotiable: the plan knows the size of every file before
	// anything is written, which is what lets --dry-run report exact numbers
	// and refuses an impossible size before the first file exists.
	if rt.SizeRange != nil {
		p.notYet(where+": size-range", "a random size from a range is not in this build yet",
			"give an exact size instead")
	}
	if rt.Contains != nil {
		p.notYet(where+": contains", "declaring archive members is not in this build yet",
			"use size together with the entries, entry_format and entry_size properties of the zip format")
	}
	if rt.Mutations != nil {
		p.notYet(where+": mutations", "damaged files arrive with the Chaos Lab",
			"remove the section")
	}
	if rt.Fill != nil {
		p.notYet(where+": fill", "the fill mode is not settable yet",
			"remove the line - content is generated from the seed")
	}

	switch {
	case rt.Boundary != nil && rt.Size != nil:
		p.add(fmt.Sprintf("%s states both a size and a boundary", where),
			"a boundary already decides the sizes, so a size beside it means two different things at once",
			"keep boundary for the three sizes around a limit, or keep size for one exact size")

	case rt.Boundary != nil && rt.Count != nil:
		p.add(fmt.Sprintf("%s states both a count and a boundary", where),
			"a boundary set is exactly three files, one below the limit, one at it and one above",
			"remove count, or use size with count to ask for identical files")

	case rt.Boundary != nil:
		t.Size = boundaryText(p, where, rt.Boundary)
		t.Sizes = boundarySizes(p, where, t.Size)

	case rt.Size != nil:
		s, ok := scalarText(rt.Size)
		if !ok {
			p.add(fmt.Sprintf("%s has a size that is neither text nor a number", where),
				"a size is written as 2mb or as a plain byte count",
				"use size: 2mb or size: 2097152")
			break
		}
		t.Size = s
		n, err := core.ParseSize(s)
		if err != nil {
			p.add(fmt.Sprintf("%s: %v", where, err),
				"units count in 1024s, so 10mb is 10485760 bytes",
				"use a size such as 2mb, 512kb or a plain byte count")
			break
		}
		for i := 0; i < count; i++ {
			t.Sizes = append(t.Sizes, n)
		}

	case rt.SizeRange == nil && rt.Contains == nil:
		p.add(fmt.Sprintf("%s has no size", where),
			"every target declares its size, which is what lets a dry run report exact numbers before anything reaches the disk",
			"add size: 2mb, a boundary, or a plain number of bytes")
	}

	if rt.Name != nil {
		t.Name = *rt.Name
	}
	if rt.Label != nil {
		t.Label = *rt.Label
	}

	t.Expected, t.ExpectedReason = expectation(p, where, rt.Expected)
	t.Properties = properties(p, where, rt.Properties)
	return t
}

// reasons is the closed list from docs/MANIFEST.md section 5.
//
// It is closed on purpose. A reason nobody recognises is a typo, and a typo
// accepted in silence becomes an expectation no test will ever check - the
// same failure as an unknown outcome, one level down.
var reasons = map[string]bool{
	"size_limit": true, "size_zero": true, "count_limit": true,
	"extension_denied": true, "mime_mismatch": true, "content_malformed": true,
	"filename_invalid": true, "filename_too_long": true, "filename_traversal": true,
	"dimensions_limit": true, "nesting_depth": true, "encoding_invalid": true,
	"malware_signature": true, "duplicate": true, "none": true,
}

func reasonList() string {
	out := make([]string, 0, len(reasons))
	for r := range reasons {
		out = append(out, r)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// boundaryText reads the limit a boundary set is built around.
func boundaryText(p *problems, where string, v any) string {
	s, ok := scalarText(v)
	if !ok {
		p.add(fmt.Sprintf("%s has a boundary that is neither text nor a number", where),
			"a boundary is one size, and the set is built either side of it",
			"use boundary: 10mb or a plain byte count")
		return ""
	}
	return s
}

// boundarySizes turns a limit into the three sizes a boundary set means.
//
// This is the case the whole tool is pointed at: an application says it
// accepts files up to 10 MB, and a test needs one file just under, one exactly
// on it and one just over. The three sizes have to be consecutive, which is
// why WAV pads the way it does - a format that could only hit even sizes would
// make two of the three unreachable.
func boundarySizes(p *problems, where, text string) []int64 {
	if text == "" {
		return nil
	}
	limit, err := core.ParseSize(text)
	if err != nil {
		p.add(fmt.Sprintf("%s: %v", where, err),
			"units count in 1024s, so 10mb is 10485760 bytes",
			"use a boundary such as 10mb, 512kb or a plain byte count")
		return nil
	}
	if limit < 1 {
		p.add(fmt.Sprintf("%s has a boundary of %d B", where, limit),
			"the set needs a size one byte below the limit, and there is nothing below zero",
			"use a boundary of at least 1 B")
		return nil
	}
	return []int64{limit - 1, limit, limit + 1}
}

// expectation accepts the short form and the long one. The short form is what
// most recipes use, and the long form carries a reason.
func expectation(p *problems, where string, v any) (string, string) {
	if v == nil {
		return "", ""
	}

	outcome, reason := "", ""
	switch x := v.(type) {
	case string:
		outcome = x
	case map[string]any:
		o, ok := x["outcome"]
		if !ok {
			p.add(fmt.Sprintf("%s declares an expectation with no outcome", where),
				"an expectation says what the system under test should do with the file",
				"add outcome: accept, reject, sanitize or unspecified")
			return "", ""
		}
		s, ok := scalarText(o)
		if !ok {
			p.add(fmt.Sprintf("%s declares an outcome that is not a word", where),
				"an outcome is one of four words",
				"use accept, reject, sanitize or unspecified")
			return "", ""
		}
		outcome = s

		// Any key other than the two we carry would be dropped on the way to
		// the manifest, and a dropped expectation is one nobody ever checks.
		for k := range x {
			switch k {
			case "outcome", "reason":
			default:
				p.add(fmt.Sprintf("%s declares %q inside its expectation", where, k),
					"an expectation carries an outcome and a reason, and anything else would be dropped without a word",
					"remove the line, or put the explanation in reason")
			}
		}

		if r, ok := x["reason"]; ok {
			s, ok := scalarText(r)
			if !ok {
				p.add(fmt.Sprintf("%s declares a reason that is not a word", where),
					"a reason is one value from a closed list",
					"use one of: "+reasonList())
			} else if !reasons[s] {
				p.add(fmt.Sprintf("%s gives the reason %q, which is not on the list", where, s),
					"the list is closed so that a report can group by reason, and a typo would make a category of one",
					"use one of: "+reasonList())
			} else {
				reason = s
			}
		}
	default:
		p.add(fmt.Sprintf("%s declares an expectation this build cannot read", where),
			"an expectation is either one word or a block with an outcome",
			"use expected: accept, or a block with outcome:")
		return "", ""
	}

	switch outcome {
	case "accept", "reject", "sanitize", "unspecified":
		return outcome, reason
	default:
		p.add(fmt.Sprintf("%s expects %q, which is not a known outcome", where, outcome),
			"a typo accepted in silence becomes an expectation no test will ever check",
			"use accept, reject, sanitize or unspecified")
		return "", ""
	}
}

// properties are handed to the format as text, exactly as --set does, so both
// surfaces speak one vocabulary. Whether a key exists at all is the format's
// answer, not ours.
func properties(p *problems, where string, in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))

	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		s, ok := scalarText(in[k])
		if !ok {
			p.add(fmt.Sprintf("%s: property %q is a list or a block", where, k),
				"a format property is a single value",
				"give it one value, for example pages: 3")
			continue
		}
		out[k] = s
	}
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
