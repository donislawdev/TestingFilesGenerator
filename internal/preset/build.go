package preset

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The machinery every preset shares, so that a preset file holds its question
// and its plan and nothing else.
//
// It exists because of what the second preset showed. size-boundaries carried
// its own list parser, its own character check and its own YAML writer, and
// writing empty-and-minimal beside it would have copied all three - with the
// fourth and fifth copies already named in the queue, because upload-validation
// takes two lists and text-encoding takes a third. A problem that comes back is
// a missing primitive rather than a missing tidy-up.

// commaList is the one parser behind every list a preset takes.
//
// Lists are written as one scalar with commas - "1B,1kb,1mb", "jpg,png,pdf" -
// and that is a decision rather than a habit: a preset parameter is a
// map[string]string all the way through, so the same value reads the same way
// as a flag, under "with:" in a recipe and in a field on a screen. A second
// spelling for one value is where eject and extends would drift apart. See
// docs/EXTENDS-WITH-2026-09-22.md section 2.2 e.
type commaList struct {
	// preset and param name the setting a refusal is about, so the message
	// carries the name both surfaces use - the flag without its dashes and the
	// label on the field.
	preset, param string
	// empty is the reason for a list that named nothing at all.
	empty string
	// check judges one item and answers with why it is not allowed, or "" when
	// it is. It receives the item trimmed, never blank.
	check func(item string) string
	// same is what makes two items one, for the duplicate that would otherwise
	// reach the recipe as two targets of one id. Nil compares the items
	// themselves. size-boundaries compares byte counts instead, so that 1024
	// and 1kb collide - found by fuzzing on 2026-08-05, where the collision
	// surfaced as a recipe the parser refused, complaining about target ids
	// nobody typed.
	same func(item string) string
	// keep is what to store when an item is accepted. Nil keeps the item as it
	// was trimmed.
	keep func(item string) string
	// duplicate words the refusal for an item the list is already holding,
	// given the item it repeats.
	//
	// A field rather than one sentence written here, because the general
	// sentence is worse than the one it would replace. A list of distances says
	// "the same distance, and each one names one file either side of the
	// limit", a list of formats says something else, and a primitive that
	// flattens both into "the same item" makes the message vaguer for everyone
	// in order to save a line. The text rules in CLAUDE.md ask for the name the
	// reader sees, not the name the code uses.
	duplicate func(first string) string
}

// refuse is the shape a bad value in a preset parameter takes.
//
// The type the format registry raises for a value outside its declaration,
// rather than a plain error, and that is a repair rather than a preference. A
// plain error falls through the classifier to RUNTIME, so "--spread notasize"
// told CI this program had a bug instead of saying the value was wrong -
// measured on 2026-08-05, exit 1.
func (l commaList) refuse(value, reason string) error {
	return &format.PropertyValueError{
		Format: l.preset, Key: l.param, Value: value, Reason: reason,
	}
}

// parse splits the value and judges every item of it.
//
// The order of the three steps is the whole of this function, and it was wrong
// once. Normalising has to happen BEFORE two items are compared, because being
// the same is a property of the value rather than of the typing: "PNG,png"
// passed a check made on what was typed, then became two identical ids and
// surfaced as "target id minimal_png is used twice" - a refusal about an id
// nobody had written. Measured 2026-09-22, and it is the same shape as the
// collision fuzzing found in the spread on 2026-08-05.
func (l commaList) parse(raw string) ([]string, error) {
	var out []string
	seen := map[string]string{}
	for _, piece := range strings.Split(raw, ",") {
		typed := strings.TrimSpace(piece)
		if typed == "" {
			continue
		}
		// Judged as it was typed, so the message quotes what the reader can see
		// in their own command line.
		if bad := l.check(typed); bad != "" {
			return nil, l.refuse(typed, bad)
		}
		item := typed
		if l.keep != nil {
			item = l.keep(typed)
		}
		key := item
		if l.same != nil {
			key = l.same(item)
		}
		if first, repeated := seen[key]; repeated {
			return nil, l.refuse(typed, l.duplicate(first))
		}
		seen[key] = typed
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, l.refuse(raw, l.empty)
	}
	return out, nil
}

// knownFormat is the check for a list whose items name formats.
//
// The registry answers rather than a list written here, which is the rule
// pulled from CLAUDE.md the hard way: a list copied by hand goes stale green.
// The refusal names what this build has, because a person who typed heic has no
// other way to find out what it does have.
func knownFormat(item string) string {
	if _, err := format.Get(strings.ToLower(item)); err != nil {
		return fmt.Sprintf("this build has no format called that. It has: %s",
			strings.Join(format.IDs(), ", "))
	}
	return ""
}

// lower is the keep for a list of names the registry spells in lower case.
func lower(item string) string { return strings.ToLower(item) }

// setFile is one file of a preset's set: which format writes it, what it is
// called, what settings make it what it is, and what a reasonably built system
// should do with it.
//
// Shared because two presets lay their sets out this way and a third is the
// point at which a shape becomes a type. What it is NOT is every file of every
// preset: the minimal set and the encoding set each name their files from one
// varying part and compute the rest, which is a rule of their own rather than a
// field here.
type setFile struct {
	id, name, group string
	desc            format.Descriptor
	props           map[string]string
	// count is how many files this one entry stands for. Nought and one both
	// mean a single file, because a set with one of something says "1" and a
	// set with none of it leaves the entry out entirely.
	count int
	size  int64
	// atFloor asks for the smallest size this format takes for these settings,
	// whatever that turns out to be. A separate field rather than a nought in
	// size, because nought is a real size and a file of no bytes is a case two
	// of these presets are about - the sentinel that collides with a legal
	// value is how a guard ends up testing the wrong thing.
	atFloor          bool
	expected, reason string
}

// bytes is the size this file is asked for.
//
// A floor is the remembered one, because this is asked more than once for
// every file - by refused and by draft - and working it out plans the format
// at growing sizes. See format.SmallestRemembered.
func (f setFile) bytes() int64 {
	if f.atFloor {
		return format.SmallestRemembered(f.desc, format.Request{Label: true, Properties: f.props})
	}
	return f.size
}

// refused is what the format says about this file before anything is written,
// or nil when it will produce it.
//
// Every set built from setFile asks this of every file it holds, which is PR7
// one level down: a set missing the three files the run was about still looks
// like a set.
func (f setFile) refused() error {
	_, err := f.desc.Generator.Plan(f.request())
	return err
}

// request is what refused asks the format - one place, so that a set that
// skips a question it has already asked keys it by the question itself.
func (f setFile) request() format.Request {
	return format.Request{Label: true, Properties: f.props, Bytes: f.bytes()}
}

func (f setFile) draft() recipe.TargetDraft {
	count := "1"
	if f.count > 1 {
		count = strconv.Itoa(f.count)
	}
	return recipe.TargetDraft{
		ID: f.id, Format: f.desc.ID, Count: count,
		Size: strconv.FormatInt(f.bytes(), 10), Name: f.name, Group: f.group,
		Expected: f.expected, ExpectedReason: f.reason,
		Properties: f.props,
	}
}

// draftsOf is a whole set as targets, ready for the composer.
func draftsOf(files []setFile) []recipe.TargetDraft {
	out := make([]recipe.TargetDraft, 0, len(files))
	for _, f := range files {
		out = append(out, f.draft())
	}
	return out
}

// plan is the set a preset lays out, ready to be written.
//
// A preset describes targets and this turns them into source. PR5 asks for
// source rather than a structure, so that what eject prints and what a run
// consumes are the same bytes and cannot drift apart.
type plan struct {
	// preset and question go in the header, so an ejected recipe says where it
	// came from and what it was for.
	preset, question string
	targets          []recipe.TargetDraft
}

// source writes the recipe.
//
// recipe.Compose marshals with the YAML library rather than printing lines,
// and that is the whole reason this exists. The hand written writer in
// sizeboundaries.go carries a comment saying it is safe because every value is
// one the package built itself - and that comment was wrong until 2026-08-05,
// when fuzzing found "1\rB" reaching the document raw through a value the
// caller had typed. Compose owns the quoting, and it owns the shape of the
// document, so a key added to the schema is added in one place rather than in
// every preset.
//
// The header is written here rather than composed, because a comment is not
// part of the document and a marshaller has nowhere to put one. Every byte of
// it is this package's own text - an id and a question, both constants - so
// there is nothing here for a caller's typing to reach.
func (p plan) source() ([]byte, error) {
	body, err := recipe.Compose(recipe.Document{Targets: p.targets})
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated by: tfg preset eject %s\n", p.preset)
	fmt.Fprintf(&b, "# %s\n", p.question)
	b.WriteString("#\n")
	b.WriteString("# Edit it, commit it, it is an ordinary recipe from here on.\n\n")
	b.Write(body)
	return []byte(b.String()), nil
}
