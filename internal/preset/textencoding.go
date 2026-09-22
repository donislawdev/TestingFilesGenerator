package preset

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

const (
	encodingID = "text-encoding"
	// sampleParam is not called size, and that is forced rather than chosen: a
	// preset parameter is a flag, --size is already one, and the build refuses
	// a preset that shadows it. See clashingParameter in internal/cli.
	sampleParam   = "sample"
	defaultSample = "4kb"

	encodingGroup   = "encoding"
	lineEndingGroup = "line-endings"

	// lineEndingSetting is the second axis. Written here rather than imported,
	// because no package owns the name the way textenc owns the first one - csv
	// declares a constant for it and log writes the string. What keeps this
	// honest is not the spelling but the guard: the set is built from whatever
	// the registry says carries this setting, so a format that gains it joins
	// the set and a renamed setting empties the group and reddens.
	lineEndingSetting = "line_ending"

	// encodingQuestion is announced by the preset AND written into the header
	// of an ejected recipe. Named once rather than typed twice.
	encodingQuestion = "Does my reader know which encoding a file is in, or is it guessing?"
)

func init() {
	Register(Preset{
		ID:       encodingID,
		Title:    "Text encoding",
		Question: encodingQuestion,

		Parameters: []format.Property{
			{
				Name: sampleParam, Kind: format.PropertySize,
				Default: defaultSample,
				Detail: "How big each file of the set is. UTF-16 stores two bytes for every " +
					"character, so an odd number is refused.",
			},
		},

		Requires: []string{"MVP"},
		Catches: []string{
			"a reader that assumes UTF-8 and shows a UTF-16 file as one character in three, or as rows of boxes",
			"a byte order mark read as content, so the first field of an import starts with three stray characters",
			"an importer that guesses the encoding from the opening bytes and guesses differently for a longer file",
			"a CRLF file split into rows with an empty row after each one, or a carriage return kept inside the last field",
		},

		Says:   saidAboutTheEncodingSet,
		Expand: expandTextEncoding,
	})
}

// textFile is one file of the set: a format, the settings that make it what it
// is, and what a reasonably built reader should do with it.
type textFile struct {
	desc  format.Descriptor
	group string
	props map[string]string
	// label is what the file is named after - the setting that distinguishes it
	// from its neighbours, such as "utf-16le_bom" or "crlf".
	label            string
	expected, reason string
}

func (f textFile) id() string {
	return strings.ReplaceAll(f.group, "-", "_") + "_" + f.desc.ID + "_" + f.label
}

func (f textFile) name() string { return f.label + f.desc.Extension }

func (f textFile) draft(size int64) recipe.TargetDraft {
	return recipe.TargetDraft{
		ID: f.id(), Format: f.desc.ID, Count: "1",
		Size: strconv.FormatInt(size, 10), Name: f.name(), Group: f.group,
		Expected: f.expected, ExpectedReason: f.reason,
		Properties: f.props,
	}
}

// carrying is every format in this build that declares all of these settings.
//
// Asked of the registry rather than written down, which is the whole shape of
// this preset. The card for it, written on 2026-09-08, said the set was
// "utf-8 / utf-16 times a byte order mark times CRLF and LF on txt, md, csv and
// log" - and that set does not exist: txt has no line ending, csv has no
// encoding, and the two axes have no format in common. A list copied by hand
// goes stale green, so this one is not copied.
func carrying(settings ...string) []format.Descriptor {
	var out []format.Descriptor
	for _, id := range format.IDs() {
		desc, err := format.Get(id)
		if err == nil && declaresAll(desc, settings) {
			out = append(out, desc)
		}
	}
	return out
}

// declaresAll says whether one format has every one of these settings.
func declaresAll(desc format.Descriptor, settings []string) bool {
	for _, want := range settings {
		if _, ok := declared(desc, want); !ok {
			return false
		}
	}
	return true
}

// declared is one setting of a format as the format declares it.
func declared(desc format.Descriptor, name string) (format.Property, bool) {
	for _, p := range desc.Properties {
		if p.Name == name {
			return p, true
		}
	}
	return format.Property{}, false
}

// encodingCells is the encoding half of the set, with the combinations this
// build refuses left out and named.
//
// The product is not full and cannot be generated blind. XML in UTF-16 has to
// open with a byte order mark - the specification says so and the format
// refuses the combination - so two cells of eighteen do not exist. Generating
// them would refuse the whole set over a cell nobody asked for and nobody could
// remove, because we laid it out rather than them.
//
// Which cells those are is asked of the format rather than written here. The
// rule belongs to XML today and the next text format may have its own.
func encodingCells() (files []textFile, left []string) {
	for _, desc := range carrying(textenc.Setting) {
		kept, dropped := cellsOfFormat(desc)
		files = append(files, kept...)
		left = append(left, dropped...)
	}
	return files, left
}

// cellsOfFormat is the encoding half for one format.
func cellsOfFormat(desc format.Descriptor) (files []textFile, left []string) {
	enc, _ := declared(desc, textenc.Setting)
	for _, name := range enc.Choices {
		kept, dropped := cellsOfEncoding(desc, name)
		files = append(files, kept...)
		left = append(left, dropped...)
	}
	return files, left
}

// cellsOfEncoding is one format in one encoding, with and without a mark.
func cellsOfEncoding(desc format.Descriptor, encoding string) (files []textFile, left []string) {
	for _, mark := range marks(desc) {
		props := map[string]string{textenc.Setting: encoding}
		label := encoding
		if mark != "" {
			props[textenc.SettingBOM] = mark
		}
		if mark == "true" {
			label += "_bom"
		}
		if why := refusedOutright(desc, props); why != "" {
			left = append(left, fmt.Sprintf("%s as %s (%s)", desc.ID, label, why))
			continue
		}
		files = append(files, textFile{
			desc: desc, group: encodingGroup, props: props, label: label,
			expected: outcomeFor(encoding), reason: reasonFor(encoding),
		})
	}
	return files, left
}

// marks is the byte order mark axis for one format, or one empty entry for a
// format that has an encoding and no mark to go with it.
//
// Both spellings rather than the declared choices, because a bool declares no
// closed set and a window draws it as a switch with two positions.
func marks(desc format.Descriptor) []string {
	if _, ok := declared(desc, textenc.SettingBOM); !ok {
		return []string{""}
	}
	return []string{"false", "true"}
}

// outcomeFor and reasonFor are the owner's decision of 2026-09-22, and the
// line they draw is between what every reader has to handle and what is
// somebody's declared policy.
//
// UTF-8 is the one encoding a modern reader cannot decline, with or without a
// mark: the mark is legal there, and a reader showing it as stray characters
// has a defect we can name. Whether a system handles UTF-16 at all is its own
// policy - "text uploads, UTF-8 only" is a correct system, not a broken one -
// and MF5 says we do not invent that answer.
func outcomeFor(encoding string) string {
	if encoding == textenc.UTF8 {
		return "accept"
	}
	return "unspecified"
}

func reasonFor(encoding string) string {
	if encoding == textenc.UTF8 {
		return ""
	}
	return "encoding_invalid"
}

// lineEndingCells is the other half, and it is a separate half rather than a
// second axis of the first.
//
// No format in this build carries both settings, measured 2026-09-22 against
// the registry: an encoding belongs to md, txt and xml, a line ending to csv
// and log. The set says so out loud, because "where is the UTF-16 CSV" is the
// first question somebody reading it asks.
func lineEndingCells() []textFile {
	var out []textFile
	for _, desc := range carrying(lineEndingSetting) {
		ending, _ := declared(desc, lineEndingSetting)
		for _, name := range ending.Choices {
			out = append(out, textFile{
				desc:  desc,
				group: lineEndingGroup,
				props: map[string]string{lineEndingSetting: name},
				label: name,
				// Both endings are legal in both formats and a reader that
				// handles one has no excuse for the other, so this half of the
				// set is stated rather than left open.
				expected: "accept",
			})
		}
	}
	return out
}

// refusedOutright is why this format will not take these settings at any size,
// or empty when it will.
//
// The format answers rather than this file. Asked at the size the format itself
// names as its smallest for these settings, so a refusal that comes back is
// about the settings and not about the room they need.
func refusedOutright(desc format.Descriptor, props map[string]string) string {
	r := format.Request{Label: true, Properties: props}
	r.Bytes = desc.SmallestAccepted(r)
	_, err := desc.Generator.Plan(r)
	var bad *format.PropertyValueError
	if errors.As(err, &bad) {
		return bad.Reason
	}
	return ""
}

// evenEnough refuses a size no file of this set could have.
//
// A whole file in UTF-16 has an even number of bytes, so an odd sample is not a
// file too small - it is a value that cannot be written, and the difference
// matters to the person reading the refusal. Left to the run it would arrive as
// a complaint about one file of twenty, with a floor a byte above what was
// asked for and advice to raise it.
//
// The codec answers, and it is asked without a mark so that the only thing left
// that can refuse is the width. The numbers in the message are its own.
func evenEnough(size int64) error {
	for _, name := range encodingsInTheSet() {
		codec, err := textenc.Parse(encodingID, map[string]string{textenc.Setting: name})
		if err != nil {
			continue
		}
		var below *format.BelowMinimumError
		if errors.As(codec.Check(encodingID, size), &below) {
			return &ImpossibleError{
				Preset:  encodingID,
				Setting: sampleParam,
				Detail:  fmt.Sprintf("the set holds files in %s, and %s", name, below.Reason),
				Hint: fmt.Sprintf("Set the {setting} to %d B or %d B.",
					size-1, below.Minimum),
			}
		}
	}
	return nil
}

// encodingsInTheSet is every encoding any format of this set is written in,
// once each and in the order the formats declare them.
func encodingsInTheSet() []string {
	var out []string
	seen := map[string]bool{}
	for _, desc := range carrying(textenc.Setting) {
		enc, _ := declared(desc, textenc.Setting)
		out = appendUnseen(out, seen, enc.Choices)
	}
	return out
}

func appendUnseen(out []string, seen map[string]bool, more []string) []string {
	for _, name := range more {
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// roomEnough refuses the whole set when any one file of it is out of reach.
//
// The same rule as the boundary set, and the same reason: a set missing four of
// its twenty files still looks like a set, and the four that are missing are
// whichever ones the reader was weakest at. The floor named is the highest of
// them, because raising the sample to the first one would only produce the next
// refusal.
func roomEnough(files []textFile, size int64) error {
	var floor int64
	var tallest textFile
	for _, f := range files {
		r := format.Request{Label: true, Properties: f.props}
		r.Bytes = size
		if _, err := f.desc.Generator.Plan(r); err == nil {
			continue
		}
		if need := f.desc.SmallestAccepted(r); need > floor {
			floor, tallest = need, f
		}
	}
	if floor == 0 {
		return nil
	}
	return &ImpossibleError{
		Preset:  encodingID,
		Setting: sampleParam,
		Detail: fmt.Sprintf("%s written as %s cannot be smaller than %d B, and every file of this set is the same size",
			strings.ToUpper(tallest.desc.ID), tallest.label, floor),
		Hint: fmt.Sprintf("Set the {setting} to %d B or more.", floor),
	}
}

// saidAboutTheEncodingSet names what the set does not hold.
//
// Two silences, and untouchable rule 6 is about both. The combinations this
// build refuses are left out, so a set of sixteen arrives where eighteen were
// described. And the two halves look like one grid and are not - somebody
// reading the file list will look for the UTF-16 CSV that no format in this
// build can produce.
func saidAboutTheEncodingSet(Args) []string {
	var out []string
	cells, left := encodingCells()
	if len(left) > 0 {
		out = append(out, fmt.Sprintf(
			"this build refuses %s of the encoding set, so it holds %s rather than %d. Left out: %s.",
			core.Count(len(left), "combination", "combinations"),
			core.Count(len(cells), "file", "files"),
			len(cells)+len(left), strings.Join(left, ", ")))
	}
	if both := carrying(textenc.Setting, lineEndingSetting); len(both) == 0 {
		out = append(out, fmt.Sprintf(
			"no format in this build carries an encoding and a line ending at once, so the two halves of this set are separate files rather than one grid. Encodings: %s. Line endings: %s.",
			strings.Join(idsOf(carrying(textenc.Setting)), ", "),
			strings.Join(idsOf(carrying(lineEndingSetting)), ", ")))
	}
	return out
}

func idsOf(descs []format.Descriptor) []string {
	out := make([]string, 0, len(descs))
	for _, d := range descs {
		out = append(out, d.ID)
	}
	return out
}

func expandTextEncoding(args Args) ([]byte, error) {
	size, err := core.ParseSize(args[sampleParam])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sampleParam, err)
	}
	// Before the files are laid out, because this refusal is about the value
	// somebody typed and the one below it is about the set that value asks for.
	if err := evenEnough(size); err != nil {
		return nil, err
	}

	cells, _ := encodingCells()
	files := append(cells, lineEndingCells()...)
	if len(files) == 0 {
		return nil, &ImpossibleError{
			Preset: encodingID,
			Detail: "no format in this build carries an encoding or a line ending, so there is no set to build",
			Hint:   "Run \"tfg formats\" to see what this build has.",
		}
	}
	if err := roomEnough(files, size); err != nil {
		return nil, err
	}

	targets := make([]recipe.TargetDraft, 0, len(files))
	for _, f := range files {
		targets = append(targets, f.draft(size))
	}
	return plan{preset: encodingID, question: encodingQuestion, targets: targets}.source()
}
