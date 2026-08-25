package recipe

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Reading one target, kept apart from reading the recipe around it.
//
// The split happened when the file ceiling in internal/guard went red at 567
// lines. The ceiling is a ratchet, so the answer is a split and never a higher
// number - raising it to turn a run green is the same act as editing a golden
// value for the same reason.
//
// The line drawn here is what a part does, not how long it is: recipe.go reads
// the document and its settings, this file reads one entry of the targets list
// and everything only that needs.

type rawTarget struct {
	ID     *scalar `yaml:"id"`
	Format *scalar `yaml:"format"`
	Count  *scalar `yaml:"count"`
	Name   *scalar `yaml:"name"`
	Label  *bool   `yaml:"label"`

	// Size accepts what a person writes: 2mb as text, or a plain byte count
	// as a number. Refusing one of the two would be a trap rather than a rule.
	Size *scalar `yaml:"size"`

	Properties map[string]scalar `yaml:"properties"`
	Expected   any               `yaml:"expected"`
	Group      *scalar           `yaml:"group"`

	Boundary  *scalar             `yaml:"boundary"`
	SizeRange *scalar             `yaml:"size-range"`
	Contains  []map[string]scalar `yaml:"contains"`
	Mutations []map[string]any    `yaml:"mutations"`
	Fill      *scalar             `yaml:"fill"`
}

// DefaultCount is how many files a target produces when it does not say.
//
// Exported so that the window can show it as what happens if the box is left
// empty, rather than keeping a second copy of the number - which is the shape
// that put one answer in tfg formats and another in the generator once already.
const DefaultCount = 1

func (rt rawTarget) validate(p *problems, index int, def Defaults) Target {
	t := Target{Label: def.Label}
	count := DefaultCount

	where := targetSpot(index, "")
	if id, ok := oneValue(p, where.of("id"), where.String()+" {setting}", "id: invoices", rt.ID); ok && id != "" {
		t.ID = id
		where = targetSpot(index, t.ID)
	} else {
		p.add(where.of("id"), fmt.Sprintf("%s has no {setting}", where),
			"{a} {setting} anchors the seed of a target, so editing one target never moves the bytes of another",
			"give it {a} {setting}, for example id: invoices")
	}

	format, formatGiven := oneValue(p, where.of("format"), where.String()+" format", "format: txt", rt.Format)
	if !formatGiven || format == "" {
		p.add(where.of("format"), fmt.Sprintf("%s has no format", where),
			"a target has to say what kind of file it produces",
			"add format: txt, or run \"tfg formats\" to see the whole list")
	} else {
		t.Format = format
	}

	if rt.Count != nil {
		n, ok := rt.Count.number()
		switch {
		case !ok:
			p.add(where.of("count"), fmt.Sprintf("%s has {a} {setting} of %q, which is not a whole number", where, rt.Count.text),
				"{a} {setting} is how many files this target produces, so it is read exactly as written",
				"write a decimal number such as count: 10")
		case n <= 0:
			p.add(where.of("count"), fmt.Sprintf("%s asks for %d files", where, n),
				"a target that produces nothing is almost always a mistake rather than an intention",
				"ask for at least one, or delete the target")
		// Judged before the list is built. The reader used to grow it one entry
		// at a time and reached a 13 GB allocation on a count of 2^63 - so this
		// has to refuse the number rather than the result of using it.
		case n > core.MaxFilesPerRun:
			p.add(where.of("count"), fmt.Sprintf("%s asks for %d files", where, n),
				core.ErrTooManyFiles.Error(),
				fmt.Sprintf("use {a} {setting} of %d or less, or split the target across several recipes", core.MaxFilesPerRun))
		default:
			count = int(n)
		}
	}

	rt.refuseSections(p, where)
	if rt.Contains != nil {
		t.Contains = contentGroups(p, where, rt.Contains)
	}
	rt.resolveSize(p, where, count, &t)

	if name, ok := oneValue(p, where.of("name"), where.String()+" {setting}", "name: invoice_{index:04}.pdf", rt.Name); ok {
		t.Name = name
	}
	if rt.Label != nil {
		t.Label = *rt.Label
	}

	t.Expected, t.ExpectedReason = expectation(p, where, rt.Expected)
	if group, ok := oneValue(p, where.of("group"), where.String()+" {setting}", "group: invoices", rt.Group); ok {
		t.Group = group
	}
	t.Properties = properties(p, where, t.Format, rt.Properties)
	return t
}

// refuseSections names the parts of a target this build cannot honour.
func (rt rawTarget) refuseSections(p *problems, where spot) {
	if rt.Mutations != nil {
		p.notYetIn(where, "mutations", "damaged files arrive with the Chaos Lab",
			"remove the section")
	}
	if rt.Fill != nil {
		p.notYetIn(where, "fill", "the fill mode is not settable yet",
			"remove the line - content is generated from the seed")
	}
}

// resolveSize settles how big each file of this target is.
//
// One of four ways to state a size. Two of them work here, and the rule itself
// is not negotiable: the plan knows the size of every file before anything is
// written, which is what lets --dry-run report exact numbers and refuses an
// impossible size before the first file exists.
func (rt rawTarget) resolveSize(p *problems, where spot, count int, t *Target) {
	switch {
	// Two ways of stating a size in one target is two answers to one question,
	// and picking one of them quietly is how a recipe stops meaning what it
	// says. Every pairing is named rather than resolved.
	case rt.SizeRange != nil && rt.Size != nil:
		p.add(where.of("size"), fmt.Sprintf("%s states both {a} {setting} and a size-range", where),
			"one names an exact {setting} and the other draws one, so together they say two different things",
			"keep {setting} for identical files, or keep size-range for a different {setting} each")

	case rt.SizeRange != nil && rt.Boundary != nil:
		p.add(where.of("boundary"), fmt.Sprintf("%s states both {a} {setting} and a size-range", where),
			"{a} {setting} is three chosen sizes around a limit, so drawing sizes as well would throw away the reason for choosing them",
			"keep {setting} to test a limit, or keep size-range for files of varying size")

	case rt.Boundary != nil && rt.Size != nil:
		p.add(where.of("size"), fmt.Sprintf("%s states both {a} {setting} and a boundary", where),
			"a boundary already decides the sizes, so {a} {setting} beside it means two different things at once",
			"keep boundary for the three sizes around a limit, or keep {setting} for one exact {setting}")

	case rt.Boundary != nil && rt.Count != nil:
		p.add(where.of("count"), fmt.Sprintf("%s states both {a} {setting} and a boundary", where),
			"a boundary set is exactly three files, one below the limit, one at it and one above",
			"remove {setting}, or use size with {setting} to ask for identical files")

	case rt.Boundary != nil:
		t.Size = boundaryText(p, where, rt.Boundary)
		t.Sizes = boundarySizes(p, where, t.Size)
		if len(t.Sizes) == 3 {
			// Kept so the three files can name which of them they are. Without
			// it they arrive as an ordinary group of three and the only way to
			// tell the limit from one byte either side is to read the sizes
			// back off the disk - which is exactly what somebody uploading them
			// to a form cannot do.
			t.BoundaryLimit = t.Sizes[1]
		}

	case rt.Size != nil:
		s, ok := rt.Size.value()
		if !ok {
			p.add(where.of("size"), fmt.Sprintf("%s has {a} {setting} that is neither text nor a number", where),
				"{a} {setting} is written as 2mb or as a plain byte count",
				"use size: 2mb or size: 2097152")
			break
		}
		t.Size = s
		n, err := core.ParseSize(s)
		if err != nil {
			p.add(where.of("size"), fmt.Sprintf("%s: %v", where, err),
				"units count in 1024s, so 10mb is 10485760 bytes",
				"use {a} {setting} such as 2mb, 512kb or a plain byte count")
			break
		}
		for i := 0; i < count; i++ {
			t.Sizes = append(t.Sizes, n)
		}

	case rt.SizeRange != nil:
		// Placed above contains for the same reason size is: a stated size
		// wins over one worked out from the contents, and the difference is
		// closed by padding. A range is a stated size that happens to differ
		// per file.
		text, ok := rt.SizeRange.value()
		if !ok {
			p.add(where.of("size-range"), fmt.Sprintf("%s has {a} {setting} that is not text", where),
				"a range is two sizes with a hyphen between them",
				"use size-range: 1kb-8kb")
			break
		}
		t.Size = text
		low, high, ok := parseSizeRange(p, where, text)
		if !ok {
			break
		}
		t.SizeIsRange, t.SizeMin, t.SizeMax = true, low, high
		// The values stay zero here and the engine replaces them. What this
		// list carries at this point is the number of files, exactly as it
		// does for contains.
		for i := 0; i < count; i++ {
			t.Sizes = append(t.Sizes, 0)
		}

	case rt.Contains != nil:
		// The fourth way of declaring a size, and not an exception to the rule.
		// Every member has its own size, so the total follows from the parts
		// and a dry run still reports a number without generating anything.
		// See ARCHITECTURE.md section 9.
		t.SizeFromContents = true
		for i := 0; i < count; i++ {
			t.Sizes = append(t.Sizes, 0)
		}

	default:
		p.add(where.of("size"), fmt.Sprintf("%s has no {setting}", where),
			"every target declares its {setting}, which is what lets a dry run report exact numbers before anything reaches the disk",
			"add size: 2mb, size-range: 1kb-8kb, a boundary, contains, or a plain number of bytes")
	}
}

// parseSizeRange asks core for the two ends and turns a refusal into the four
// part message a recipe problem carries.
//
// The maths is not repeated here on purpose: the flag asks the same question,
// and one implementation is what keeps the two surfaces meaning the same thing.
func parseSizeRange(p *problems, where spot, text string) (low, high int64, ok bool) {
	lo, hi, err := core.ParseSizeRange(text)
	if err != nil {
		p.add(where.of("size-range"), fmt.Sprintf("%s: %v", where, err),
			"both ends of a range are sizes, and units count in 1024s",
			"use size-range: 1kb-8kb or a pair of plain byte counts")
		return 0, 0, false
	}
	return lo, hi, true
}

// reasons is the closed list from docs/MANIFEST.md section 5.
//
// It is closed on purpose. A reason nobody recognises is a typo, and a typo
// accepted in silence becomes an expectation no test will ever check - the
// same failure as an unknown outcome, one level down.
// The names are neutral about the verdict on purpose, and one of them was not
// until 2026-08-19.
//
// A reason says which rule a case is ABOUT, not what the system did - which is
// what lets the same reason sit under any outcome. MANIFEST.md's own example for
// an unspecified outcome carries size_zero, and a boundary set expects the file
// a byte under the limit to be ACCEPTED with size_limit as the rule in play.
//
// extension_denied broke that: it was the only entry of fifteen whose name
// carried a verdict, so "accept with extension_denied" read as a contradiction
// while "accept with size_limit" read fine. Reported by the owner, who picked
// the one entry that did not fit. Renamed to extension_rule rather than the
// field being restricted, because restricting it would have broken the boundary
// set - the tool's own flagship case.
var reasons = map[string]bool{
	"size_limit": true, "size_zero": true, "count_limit": true,
	"extension_rule": true, "mime_mismatch": true, "content_malformed": true,
	"filename_invalid": true, "filename_too_long": true, "filename_traversal": true,
	"dimensions_limit": true, "nesting_depth": true, "encoding_invalid": true,
	"malware_signature": true, "duplicate": true, "none": true,
}

func reasonList() string {
	return strings.Join(Reasons(), ", ")
}

// Reasons is the closed list, for the surfaces that have to offer it.
//
// Exported because the command line needs the same list the recipe uses. It
// had no way to state a reason at all until 2026-08-03 - "--expected reject"
// worked and the reason beside it did not exist - so a run driven by flags
// could never fill the category the list exists to make countable. Two copies
// of a closed list is how the two surfaces drift, which is D1 one level down.
func Reasons() []string {
	out := make([]string, 0, len(reasons))
	for r := range reasons {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// KnownReason says whether a reason is on the list.
func KnownReason(r string) bool { return reasons[r] }

// outcomes is the closed list of what a system under test can be expected to do
// with a file, from docs/MANIFEST.md.
//
// A variable rather than four words repeated at each place that checks them,
// which is what this was until 2026-08-18: the sentence "accept, reject,
// sanitize or unspecified" appeared in the recipe reader twice more and in the
// command line twice, and the window was about to be the fifth copy. That is
// exactly the argument written above Reasons - two copies of a closed list is
// how the surfaces drift, which is D1 one level down.
var outcomes = []string{"accept", "reject", "sanitize", "unspecified"}

// Outcomes is the closed list, for the surfaces that have to offer it.
//
// Sorted, because a closed set has one order everywhere it is shown and a guard
// asks for exactly that. These four are alphabetical as written, and sorting
// says so rather than relying on it.
func Outcomes() []string {
	out := make([]string, len(outcomes))
	copy(out, outcomes)
	sort.Strings(out)
	return out
}

// KnownOutcome says whether an outcome is on the list.
func KnownOutcome(o string) bool {
	for _, known := range outcomes {
		if known == o {
			return true
		}
	}
	return false
}

// boundaryText reads the limit a boundary set is built around.
func boundaryText(p *problems, where spot, v *scalar) string {
	s, ok := v.value()
	if !ok {
		p.add(where.of("boundary"), fmt.Sprintf("%s has {a} {setting} that is neither text nor a number", where),
			"{a} {setting} is one size, and the set is built either side of it",
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
func boundarySizes(p *problems, where spot, text string) []int64 {
	if text == "" {
		return nil
	}
	limit, err := core.ParseBoundary(text)
	if err != nil {
		p.add(where.of("boundary"), fmt.Sprintf("%s: %v", where, err),
			"units count in 1024s, so 10mb is 10485760 bytes",
			"use {a} {setting} such as 10mb, 512kb or a plain byte count")
		return nil
	}
	if limit < 1 {
		p.add(where.of("boundary"), fmt.Sprintf("%s has {a} {setting} of %d B", where, limit),
			"the set needs a size one byte below the limit, and there is nothing below zero",
			"use {a} {setting} of at least 1 B")
		return nil
	}
	sizes, err := core.BoundarySizes(limit)
	if err != nil {
		p.add(where.of("boundary"), fmt.Sprintf("%s has {a} {setting} of %d B", where, limit),
			err.Error(),
			"use {a} {setting} at least one byte below the largest number")
		return nil
	}
	return sizes
}

// expectation accepts the short form and the long one. The short form is what
// most recipes use, and the long form carries a reason.
func expectation(p *problems, where spot, v any) (string, string) {
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
			p.add(where.of("expected"), fmt.Sprintf("%s declares an expectation with no outcome", where),
				"an expectation says what the system under test should do with the file",
				"add outcome: accept, reject, sanitize or unspecified")
			return "", ""
		}
		s, ok := scalarText(o)
		if !ok {
			p.add(where.of("expected"), fmt.Sprintf("%s declares an outcome that is not a word", where),
				"an outcome is one of four words",
				"use accept, reject, sanitize or unspecified")
			return "", ""
		}
		outcome = s

		// Any key other than the two we carry would be dropped on the way to
		// the manifest, and a dropped expectation is one nobody ever checks.
		for _, k := range sortedKeys(x) {
			switch k {
			case "outcome", "reason":
			default:
				p.add(where.of("expected"), fmt.Sprintf("%s declares %q inside its expectation", where, k),
					"an expectation carries an outcome and a reason, and anything else would be dropped without a word",
					"remove the line, or put the explanation in reason")
			}
		}

		if r, ok := x["reason"]; ok {
			s, ok := scalarText(r)
			if !ok {
				p.add(where.of("expected.reason"), fmt.Sprintf("%s declares a reason that is not a word", where),
					"a reason is one value from a closed list",
					"use one of: "+reasonList())
			} else if !reasons[s] {
				p.add(where.of("expected.reason"), fmt.Sprintf("%s gives the reason %q, which is not on the list", where, s),
					"the list is closed so that a report can group by reason, and a typo would make a category of one",
					"use one of: "+reasonList())
			} else {
				reason = s
			}
		}
	default:
		p.add(where.of("expected"), fmt.Sprintf("%s declares an expectation this build cannot read", where),
			"an expectation is either one word or a block with an outcome",
			"use expected: accept, or a block with outcome:")
		return "", ""
	}

	// Asked of the list rather than repeated here, so the window offering these
	// four and the reader accepting them cannot come apart.
	switch {
	case KnownOutcome(outcome):
		return outcome, reason
	default:
		p.add(where.of("expected"), fmt.Sprintf("%s expects %q, which is not a known outcome", where, outcome),
			"a typo accepted in silence becomes an expectation no test will ever check",
			"use accept, reject, sanitize or unspecified")
		return "", ""
	}
}

// properties are handed to the format as text, exactly as --set does, so both
// surfaces speak one vocabulary. Whether a key exists at all is the format's
// answer, not ours.
func properties(p *problems, where spot, formatID string, in map[string]scalar) map[string]string {
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
		s, ok := in[k].value()
		if !ok {
			p.add(where.of("properties."+k), fmt.Sprintf("%s: property %q is a list or a block", where, k),
				"a format property is a single value",
				"give it one value, for example pages: 3")
			continue
		}
		out[k] = s
	}
	askTheFormat(p, where, formatID, out)
	return out
}

// askTheFormat puts the declaration's own verdict on the boxes it is about.
//
// The rule itself is not repeated here and must not be: what a property takes
// lives in the declaration, and Allows words the refusal, so every format
// refuses in the same sentence and a new one gets it by declaring. What this
// adds is the ADDRESS. The engine has always asked the same question, one layer
// up, where a target is an entry in a list and not "targets[2]" - so a mistyped
// width came back as "bmp: width cannot be ..." with no way to tell which of
// twenty BMP batches meant it, no "at" in validate --json, and nothing for a
// form to mark. Measured on 2026-08-25 against a refusal about a size, which
// carries targets[2].size and all four parts of D6.
//
// The engine still asks. This does not replace that check and could not: the
// one-target path from the command line flags never comes through a recipe, and
// a layer that trusts the layer above it to have checked is a layer with a hole
// in it.
//
// An unknown format is left alone rather than reported here. The format is
// refused by name where formats are settled, and a second refusal about the
// properties of a format that does not exist would be noise on top of the
// answer.
func askTheFormat(p *problems, where spot, formatID string, stated map[string]string) {
	if len(stated) == 0 {
		return
	}
	d, err := format.Get(formatID)
	if err != nil {
		return
	}
	for _, bad := range d.CheckEachProperty(stated) {
		var about interface{ AboutSetting() string }
		if !errors.As(bad, &about) {
			// Every problem this can return names its key. Kept as a branch
			// rather than assumed, because a third kind added without one would
			// otherwise vanish instead of arriving unaddressed.
			p.add(where.of(KeyProperties), bad.Error(), "", "")
			continue
		}
		at := where.of(KeyProperties + "." + about.AboutSetting())
		var value *format.PropertyValueError
		if errors.As(bad, &value) {
			p.add(at, fmt.Sprintf("%s: %s cannot be %q", where, value.Key, value.Value),
				core.InTheWordsOf(value.Reason, value.Key), value.Instead)
			continue
		}
		p.add(at, fmt.Sprintf("%s: %s", where, bad.Error()), "",
			"use one of the properties the format declares")
	}
}
