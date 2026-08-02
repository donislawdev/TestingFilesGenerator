package recipe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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

// contentGroups reads the contains list.
//
// Every problem is collected rather than returned on the first one, because a
// contains list written by hand usually has more than one thing wrong with it
// and fixing a recipe one error per run is how people stop using a tool.
func contentGroups(p *problems, where string, raw []map[string]scalar) []Content {
	if len(raw) == 0 {
		// An empty list is not the same as no list. It says "an archive
		// holding nothing", which is a legitimate thing to test, and the size
		// then comes from the container overhead alone.
		return []Content{}
	}

	out := make([]Content, 0, len(raw))
	for i, item := range raw {
		at := fmt.Sprintf("%s: contains entry %d", where, i+1)
		g := Content{Count: 1}

		// Sorted, because Go randomises map order and these keys become
		// problems in the report. Unsorted, the same broken recipe would print
		// its problems in a different order on two runs.
		for _, key := range sortedKeys(item) {
			switch key {
			case "format", "count", "size":
			default:
				p.add(fmt.Sprintf("%s has the key %q", at, key),
					"a contains entry describes files with a format, a count and a size, and anything else would be dropped on the way",
					"remove the key, or move it to the target itself")
			}
		}

		if v, ok := item["format"]; ok {
			if s, ok := v.value(); ok && s != "" {
				g.Format = s
			} else {
				p.add(fmt.Sprintf("%s has a format that is not a name", at),
					"a format is the id of a format this build supports",
					"use format: pdf, or run tfg formats to see the list")
			}
		} else {
			p.add(fmt.Sprintf("%s has no format", at),
				"a container holds real files, so each group says which format its files are",
				"add format: pdf")
		}

		if v, ok := item["count"]; ok {
			n, ok := v.number()
			switch {
			case !ok:
				p.add(fmt.Sprintf("%s has a count that is not a whole number", at),
					"a count is how many files of this group the container holds",
					"use count: 50")
			case n < 0:
				p.add(fmt.Sprintf("%s has a negative count", at),
					"a container cannot hold fewer than no files",
					"use count: 0 for a group that contributes nothing, or drop the entry")
			default:
				g.Count = int(n)
			}
		}

		if v, ok := item["size"]; ok {
			s, ok := v.value()
			if !ok {
				p.add(fmt.Sprintf("%s has a size that is neither text nor a number", at),
					"a size is written as 2mb or as a plain byte count",
					"use size: 2mb or size: 2097152")
			} else if n, err := core.ParseSize(s); err != nil {
				p.add(fmt.Sprintf("%s: %v", at, err),
					"units count in 1024s, so 10mb is 10485760 bytes",
					"use a size such as 2mb, 512kb or a plain byte count")
			} else {
				g.Bytes = n
			}
		} else {
			// The same rule as AR10 one level down. Without it the size of the
			// container could not be worked out before writing, which is the
			// whole reason contains counts as a way of declaring a size.
			p.add(fmt.Sprintf("%s has no size", at),
				"the size of the container follows from the size of what it holds, so every group states one",
				"add size: 2mb")
		}

		out = append(out, g)
	}
	return out
}

type rawTarget struct {
	ID     *string `yaml:"id"`
	Format *string `yaml:"format"`
	Count  *scalar `yaml:"count"`
	Name   *string `yaml:"name"`
	Label  *bool   `yaml:"label"`

	// Size accepts what a person writes: 2mb as text, or a plain byte count
	// as a number. Refusing one of the two would be a trap rather than a rule.
	Size *scalar `yaml:"size"`

	Properties map[string]scalar `yaml:"properties"`
	Expected   any               `yaml:"expected"`

	Boundary  *scalar             `yaml:"boundary"`
	SizeRange *scalar             `yaml:"size-range"`
	Contains  []map[string]scalar `yaml:"contains"`
	Mutations []map[string]any    `yaml:"mutations"`
	Fill      *string             `yaml:"fill"`
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
		n, ok := rt.Count.number()
		switch {
		case !ok:
			p.add(fmt.Sprintf("%s has a count of %q, which is not a whole number", where, rt.Count.text),
				"a count is how many files this target produces, so it is read exactly as written",
				"write a decimal number such as count: 10")
		case n <= 0:
			p.add(fmt.Sprintf("%s asks for %d files", where, n),
				"a target that produces nothing is almost always a mistake rather than an intention",
				"ask for at least one, or delete the target")
		default:
			count = int(n)
		}
	}

	rt.refuseSections(p, where)
	if rt.Contains != nil {
		t.Contains = contentGroups(p, where, rt.Contains)
	}
	rt.resolveSize(p, where, count, &t)

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

// refuseSections names the parts of a target this build cannot honour.
func (rt rawTarget) refuseSections(p *problems, where string) {
	if rt.Mutations != nil {
		p.notYet(where+": mutations", "damaged files arrive with the Chaos Lab",
			"remove the section")
	}
	if rt.Fill != nil {
		p.notYet(where+": fill", "the fill mode is not settable yet",
			"remove the line - content is generated from the seed")
	}
}

// resolveSize settles how big each file of this target is.
//
// One of four ways to state a size. Two of them work here, and the rule itself
// is not negotiable: the plan knows the size of every file before anything is
// written, which is what lets --dry-run report exact numbers and refuses an
// impossible size before the first file exists.
func (rt rawTarget) resolveSize(p *problems, where string, count int, t *Target) {
	switch {
	// Two ways of stating a size in one target is two answers to one question,
	// and picking one of them quietly is how a recipe stops meaning what it
	// says. Every pairing is named rather than resolved.
	case rt.SizeRange != nil && rt.Size != nil:
		p.add(fmt.Sprintf("%s states both a size and a size-range", where),
			"one names an exact size and the other draws one, so together they say two different things",
			"keep size for identical files, or keep size-range for a different size each")

	case rt.SizeRange != nil && rt.Boundary != nil:
		p.add(fmt.Sprintf("%s states both a boundary and a size-range", where),
			"a boundary is three chosen sizes around a limit, so drawing sizes as well would throw away the reason for choosing them",
			"keep boundary to test a limit, or keep size-range for files of varying size")

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
		s, ok := rt.Size.value()
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

	case rt.SizeRange != nil:
		// Placed above contains for the same reason size is: a stated size
		// wins over one worked out from the contents, and the difference is
		// closed by padding. A range is a stated size that happens to differ
		// per file.
		text, ok := rt.SizeRange.value()
		if !ok {
			p.add(fmt.Sprintf("%s has a size-range that is not text", where),
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
		p.add(fmt.Sprintf("%s has no size", where),
			"every target declares its size, which is what lets a dry run report exact numbers before anything reaches the disk",
			"add size: 2mb, size-range: 1kb-8kb, a boundary, contains, or a plain number of bytes")
	}
}

// parseSizeRange asks core for the two ends and turns a refusal into the four
// part message a recipe problem carries.
//
// The maths is not repeated here on purpose: the flag asks the same question,
// and one implementation is what keeps the two surfaces meaning the same thing.
func parseSizeRange(p *problems, where, text string) (low, high int64, ok bool) {
	lo, hi, err := core.ParseSizeRange(text)
	if err != nil {
		p.add(fmt.Sprintf("%s: %v", where, err),
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
func boundaryText(p *problems, where string, v *scalar) string {
	s, ok := v.value()
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
	return core.BoundarySizes(limit)
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
		for _, k := range sortedKeys(x) {
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
func properties(p *problems, where string, in map[string]scalar) map[string]string {
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
			p.add(fmt.Sprintf("%s: property %q is a list or a block", where, k),
				"a format property is a single value",
				"give it one value, for example pages: 3")
			continue
		}
		out[k] = s
	}
	return out
}
