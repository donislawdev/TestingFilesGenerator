package preset

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

const (
	minimalID = "empty-and-minimal"
	// everyFormat is what the formats parameter says when it means all of them.
	//
	// A word rather than the list written out, because the list is a property of
	// the build and this declaration is read at init, before the format registry
	// has necessarily finished filling. Global() above carries the same note for
	// the same reason. It also means the default stays one short word on a
	// screen instead of a hundred characters of ids.
	everyFormat = "all"

	minimalGroup = "minimal"
	emptyGroup   = "empty"

	// minimalQuestion is announced by the preset AND written into the header of
	// an ejected recipe. Named once rather than typed twice, because two copies
	// of one sentence are how the recipe and the list drift apart.
	minimalQuestion = "Does a file that is valid and as small as the format allows get through?"
)

func init() {
	Register(Preset{
		ID:       minimalID,
		Title:    "Empty and minimal",
		Question: minimalQuestion,

		Parameters: []format.Property{
			{
				Name: "formats", Kind: format.PropertyText,
				Shape:   "format ids separated by commas, or all",
				Default: everyFormat,
				Detail: "Which formats the set is built from. Leave it at all for every format " +
					"this build has, or name the ones your system accepts.",
			},
		},

		// The preset the window opens on, and the reason is a number rather
		// than a preference. Pressing Generate on it without touching anything
		// writes 32 214 B, where size-boundaries at its defaults writes
		// 73 400 320 - and somebody who has just opened the program should not
		// have seventy megabytes as their first result. It also means
		// something without being told anything: size-boundaries says out loud
		// that its 10mb is our placeholder and not the limit of the system
		// under test, so an untouched run of it describes nothing.
		Landing: true,

		Requires: []string{"MVP"},
		Catches: []string{
			"a valid file turned away for being too small, where the check counts bytes instead of reading them",
			"an empty file that brings the reader down rather than being reported",
			"a picture one pixel wide that divides by zero on the way to a thumbnail",
			"storage that reads nought bytes as a failed upload and keeps retrying",
		},

		Says:   saidAboutTheMinimalSet,
		Expand: expandEmptyAndMinimal,
	})
}

// formatsList is how the formats parameter is written.
var formatsList = commaList{
	preset:    minimalID,
	param:     "formats",
	empty:     "no formats were given, so there is nothing to build the set from",
	check:     checkSetFormat,
	keep:      lower,
	duplicate: repeatedFormat,
}

func repeatedFormat(first string) string {
	return fmt.Sprintf(
		"it is the same format as %q and the set would hold that file twice. Every format appears once, because each one stands for one path through your reader",
		first)
}

// checkSetFormat lets the keyword through and asks the registry about the rest.
//
// Whether the keyword is allowed to stand BESIDE a named format is decided
// after the list is parsed rather than here, because here the answer would be
// wrong for "all," - a trailing comma leaves one item, the keyword is alone,
// and a refusal saying it cannot stand beside a named format would be about a
// format nobody wrote.
func checkSetFormat(item string) string {
	if strings.EqualFold(item, everyFormat) {
		return ""
	}
	return knownFormat(item)
}

// chosenFormats is the formats this set covers, in registry order.
//
// Registry order rather than the order somebody typed, and the registry is
// walked here rather than the typing, which is the difference between the
// sentence being true and merely being written down. It was written down and
// false until 2026-09-22: this loop ran over the ids as they arrived, so
// "--formats png,zip" and "--formats zip,png" asked for one set and produced
// two - different recipe text, a different recipe_hash in the manifest, and the
// files listed the other way round. The bytes of the files themselves never
// moved, because a seed comes from the id of a target rather than from its
// place in the list. Found by review, not by a guard, and there is one now.
//
// Why it matters at all: the manifest answers "did this record come from that
// recipe", and two people asking for the same thing have to be able to compare
// their answers. SortChoices holds the same rule for a closed set of values one
// level down.
func chosenFormats(raw string) ([]format.Descriptor, error) {
	ids, err := formatsList.parse(raw)
	if err != nil {
		return nil, err
	}
	ids, err = spelledOut(ids)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}

	// The keyword shadows a format of the same name, and no format is called
	// all - TestNoFormatIsCalledByTheWordThatMeansAllOfThem holds that, so the
	// shadow cannot appear without something going red.
	out := make([]format.Descriptor, 0, len(ids))
	for _, id := range format.IDs() {
		if !wanted[id] {
			continue
		}
		desc, err := format.Get(id)
		if err != nil {
			return nil, err
		}
		out = append(out, desc)
	}
	return out, nil
}

// spelledOut turns the keyword into the list of ids it stands for, and refuses
// it standing beside a named format.
//
// Its own function rather than a branch inside chosenFormats, because the two
// questions are separate: this one is about what the words mean, the one above
// is about what the registry has. Keeping them apart also keeps each of them
// two levels deep instead of one of them three, which is the shape the crowding
// gate asks for and the reason it asks.
func spelledOut(ids []string) ([]string, error) {
	if len(ids) == 1 && ids[0] == everyFormat {
		return format.IDs(), nil
	}
	for _, id := range ids {
		if id == everyFormat {
			return nil, formatsList.refuse(everyFormat, fmt.Sprintf(
				"%q already means every format, so it cannot stand beside a named one. Write it on its own, or name only the formats you want",
				everyFormat))
		}
	}
	return ids, nil
}

// minimalFile is one file of the set.
type minimalFile struct {
	desc format.Descriptor
	size int64
	// empty marks the file that is nought bytes, which is the half of this set
	// nobody can promise an answer for.
	empty bool
}

func (f minimalFile) id() string {
	if f.empty {
		return emptyGroup + "_" + f.desc.ID
	}
	return minimalGroup + "_" + f.desc.ID
}

func (f minimalFile) name() string {
	if f.empty {
		return emptyGroup + f.desc.Extension
	}
	return minimalGroup + f.desc.Extension
}

// draft is this file as a target of the recipe.
//
// A method rather than a branch inside the loop that builds them, because a
// file of this set knows which half it belongs to and the loop does not need
// to ask. id and name above it are here for the same reason.
func (f minimalFile) draft() recipe.TargetDraft {
	if f.empty {
		return recipe.TargetDraft{
			ID: f.id(), Format: f.desc.ID, Count: "1",
			Size: strconv.FormatInt(f.size, 10), Name: f.name(), Group: emptyGroup,
			// Unspecified rather than accept, and the reason says which rule is
			// in play rather than what anybody did with it. A file of nought
			// bytes is legal and what a system should do with it is its own
			// decision - storage keeps it, an upload form usually turns it
			// away, and both are defensible.
			Expected: "unspecified", ExpectedReason: "size_zero",
		}
	}
	return recipe.TargetDraft{
		ID: f.id(), Format: f.desc.ID, Count: "1",
		Size: strconv.FormatInt(f.size, 10), Name: f.name(), Group: minimalGroup,
		Expected: "accept",
	}
}

// layOut is the set: the smallest legal file of every format, then the empty
// ones.
//
// The two halves are two groups rather than one, because they carry two
// different expectations and only one of them can be stated with any
// confidence. A file that is valid should be accepted, and that is a positive
// control - if those fail, every refusal the rest of the tool reports means
// nothing. A file of nought bytes is another matter: it is legal, and what a
// system ought to do with it is a policy its owner decides. MF5 and untouchable
// rule 5 both say we do not invent that answer.
//
// Which is why txt and md appear twice. Their smallest legal file IS nought
// bytes, so one entry each would mean either dropping two formats out of the
// positive control or handing two empty files an expectation nobody can back.
// A second file of one byte costs two bytes and keeps both halves honest.
func layOut(descs []format.Descriptor) []minimalFile {
	minimal := make([]minimalFile, 0, len(descs))
	var empty []minimalFile
	for _, d := range descs {
		floor := smallest(d)
		if floor > 0 {
			minimal = append(minimal, minimalFile{desc: d, size: floor})
			continue
		}
		minimal = append(minimal, minimalFile{desc: d, size: 1})
		empty = append(empty, minimalFile{desc: d, size: 0, empty: true})
	}
	return append(minimal, empty...)
}

// smallest is the smallest size this build will actually take for the format.
//
// The label is on, because it is on unless somebody passes --clean, and the
// number "tfg formats" prints under MINIMUM is this one. The preset and the
// table have to agree: a set built from a number the table does not show is a
// set nobody can check by hand.
//
// MinBytes beside it is the structural floor with no label, and it is NOT the
// same number - docx, pdf, targz, wav and zip all differ, measured 2026-09-22.
// Asking for MinBytes is refused.
//
// Worked out once per format per process and remembered. The request is the
// same every time, so the answer is too - and finding it means planning the
// format at growing sizes, which for the pictures means encoding them. The
// window expands this set on every change while the batch screen builds on
// it, and measured on 2026-09-23 that was ~95 ms and ~95 MB of garbage per
// expansion, a keystroke costing 380 ms (docs/GUI-MEMORY-2026-09-23.md
// section 2.3). Keyed by id, which is safe because format.Register refuses a
// second descriptor under one id. A sync.Map, because the window settles
// from its worker as well as from its own goroutine.
func smallest(d format.Descriptor) int64 {
	if known, ok := smallestKnown.Load(d.ID); ok {
		return known.(int64)
	}
	size := d.SmallestAccepted(format.Request{Label: true})
	smallestKnown.Store(d.ID, size)
	return size
}

// smallestKnown is what smallest has worked out, by format id.
var smallestKnown sync.Map

// saidAboutTheMinimalSet says when the set came out with only one of its halves.
//
// A preset named after both halves that quietly ships one is a promise it did
// not keep, and the whole of untouchable rule 6 is about that kind of silence.
// It happens for a real choice rather than a strange one: "--formats zip" is a
// sensible thing to ask for, and no archive has a legal empty form.
//
// Nothing is said about the other direction. A set that is ALL empty files
// cannot happen, because every format has a smallest legal file and this set
// always holds it.
func saidAboutTheMinimalSet(args Args) []string {
	descs, err := chosenFormats(args["formats"])
	if err != nil {
		// Expand is about to refuse these same values with a message that names
		// the item. A sentence here would be a second opinion on one question.
		return nil
	}
	for _, d := range descs {
		if smallest(d) == 0 {
			return nil
		}
	}
	return []string{fmt.Sprintf(
		"no format in this set has a legal empty form, so the set holds no empty files - only the smallest valid one of each. The formats that go down to nought bytes in this build: %s.",
		strings.Join(zeroCapable(), ", "))}
}

// zeroCapable is every format whose smallest legal file is nought bytes.
//
// Asked of the registry rather than written down. Two formats answer today and
// a third would join them without this sentence noticing, which is the failure
// CLAUDE.md calls a list copied by hand going stale green.
func zeroCapable() []string {
	var out []string
	for _, id := range format.IDs() {
		desc, err := format.Get(id)
		if err != nil {
			continue
		}
		if smallest(desc) == 0 {
			out = append(out, id)
		}
	}
	return out
}

func expandEmptyAndMinimal(args Args) ([]byte, error) {
	descs, err := chosenFormats(args["formats"])
	if err != nil {
		return nil, err
	}

	files := layOut(descs)
	targets := make([]recipe.TargetDraft, 0, len(files))
	for _, f := range files {
		targets = append(targets, f.draft())
	}

	return plan{preset: minimalID, question: minimalQuestion, targets: targets}.source()
}
