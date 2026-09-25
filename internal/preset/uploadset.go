package preset

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The set upload-validation lays out, and the values it is settled on.
//
// Apart from the file beside it, which is what the preset ANNOUNCES - its
// question, its parameters and the sentences it says out loud about what the
// set came out as. This is how the eight groups of files are built and which of
// them a parameter can empty. The two were one file of 498 lines of code on
// 2026-09-22 and the ceiling is 408, so the split follows what the parts do
// rather than where the count happened to fall.

// allowList and denyList are the two lists this preset takes. They are not the
// same kind of value, which is the decision of 2026-09-22 written down.
//
// An allowed type has to become a real file of that type, so its values are
// formats the registry has. A denied one is about the EXTENSION: the measured
// set turns away .exe and .sh, and this build has no format called either -
// installer.exe would be a ZIP in any real system and deploy.sh is text. So a
// denied entry is a format id where the registry knows one and a bare extension
// otherwise, and the file it produces says out loud what is inside it.
var allowList = commaList{
	preset:    uploadID,
	param:     allowParam,
	empty:     "no types were allowed, so the set has no positive control and no file to name wrongly",
	check:     knownFormat,
	keep:      lower,
	duplicate: repeatedAllowed,
}

var denyList = commaList{
	preset:    uploadID,
	param:     denyParam,
	empty:     "no extensions were denied, so there is nothing for the form to turn away",
	check:     checkExtension,
	keep:      lower,
	duplicate: repeatedDenied,
}

func repeatedAllowed(first string) string {
	return fmt.Sprintf(
		"it is the same type as %q and the set would hold that file twice. Every allowed type appears once, because each one stands for one path through your form",
		first)
}

func repeatedDenied(first string) string {
	return fmt.Sprintf(
		"it is the same extension as %q and the set would hold that file twice. Every denied extension appears once, because each one stands for one rule your form has",
		first)
}

// checkExtension answers why a piece of the deny list is not an extension.
//
// An extension rather than a format, because this list is about the name. The
// value reaches a file name, so it is made of what a name is made of and
// nothing else - the same defence the boundary set's distances get, and for the
// same reason rather than by analogy.
func checkExtension(item string) string {
	if strings.HasPrefix(item, ".") {
		return fmt.Sprintf("an extension is written without its dot, so %q rather than %q",
			strings.TrimPrefix(item, "."), item)
	}
	if len(item) > longestExtension {
		return fmt.Sprintf("it is %d characters long and an extension here is at most %d",
			len(item), longestExtension)
	}
	if bad := firstNotAlphanumeric(item); bad != "" {
		return fmt.Sprintf(
			"it holds %s, and an extension is written with letters and digits - such as exe, sh or svg. Its text becomes the end of a file name", bad)
	}
	return ""
}

// firstNotAlphanumeric names the first character that cannot appear in an
// extension, quoted so a space or a control character is visible in the message.
func firstNotAlphanumeric(item string) string {
	for _, r := range item {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		default:
			return fmt.Sprintf("%q", r)
		}
	}
	return ""
}

// deniedEntry is one extension the form is meant to turn away, and what this
// build can put inside a file of that name.
type deniedEntry struct {
	ext string
	// desc writes the bytes. It is the format of that name where the registry
	// has one, and the filler otherwise.
	desc format.Descriptor
	// known says the file really is what its name claims. Where it is false the
	// run says so out loud, because a test of a content sniffer would otherwise
	// pass against a file that never held what it claimed to.
	known bool
}

// extension is what a file of this entry is named with, dot included.
//
// A format the registry has answers for itself, because an extension is not
// always a dot and an id: targz is written .tar.gz, and denied.targz would be
// a file no upload form has a rule about - which is the one thing this group
// exists to test.
func (e deniedEntry) extension() string {
	if e.known {
		return e.desc.Extension
	}
	return "." + e.ext
}

// uploadSet is one survey settled on its parameters.
type uploadSet struct {
	limit     int64
	limitText string
	allowed   []format.Descriptor
	denied    []deniedEntry
	// farOver is how many times the limit the one big file is, or nought when
	// that file was turned off.
	farOver int64
	bulk    int
	filler  format.Descriptor
}

func settleUpload(args Args) (uploadSet, error) {
	var s uploadSet
	limit, err := core.ParseSize(args[uploadLimitParam])
	if err != nil {
		return s, fmt.Errorf("%s: %w", uploadLimitParam, err)
	}
	// The limit leads the names of the files built around it, so its text
	// reaches a file name and gets the check a distance gets.
	s.limitText = strings.TrimSpace(args[uploadLimitParam])
	if bad := firstUnusable(s.limitText); bad != "" {
		return s, fmt.Errorf(
			"%s: it holds %s, and a limit is written with digits, letters and a dot - "+
				"such as 10mb, 512 or 1.5gb. Its text becomes part of every file name",
			uploadLimitParam, bad)
	}
	s.limit = limit

	if s.allowed, err = allowedFormats(args[allowParam]); err != nil {
		return s, err
	}
	if s.denied, err = deniedExtensions(args[denyParam]); err != nil {
		return s, err
	}
	if err := listsAgree(s.allowed, s.denied); err != nil {
		return s, err
	}
	if s.farOver, err = farOverTimes(args[farOverParam]); err != nil {
		return s, err
	}
	if s.bulk, err = strconv.Atoi(args[bulkParam]); err != nil {
		return s, fmt.Errorf("%s: %q is not a whole number of files", bulkParam, args[bulkParam])
	}
	s.filler, err = format.Get(fillerFormat)
	return s, err
}

// listsAgree refuses an extension that is allowed and denied at once.
//
// Neither list can see the other, so "--allow pdf --deny pdf" laid a set out
// holding allowed_pdf.pdf expecting accept and denied.pdf expecting reject for
// extension_rule. Both expectations reach the manifest, so any suite running
// that set contradicts itself whatever the system under test does - and the run
// said nothing, which is untouchable rule 6 on the worst kind of silence: the
// one where nothing fails.
//
// The whole set is refused rather than one half dropped, because dropping a
// half is choosing for somebody which of the two they meant. Found by review on
// 2026-09-22, not by a guard, and there is one now.
func listsAgree(allowed []format.Descriptor, denied []deniedEntry) error {
	turned := make(map[string]string, len(denied))
	for _, entry := range denied {
		turned[entry.extension()] = entry.ext
	}
	for _, desc := range allowed {
		written, both := turned[desc.Extension]
		if !both {
			continue
		}
		return &ImpossibleError{
			Preset: uploadID, Setting: denyParam,
			Detail: fmt.Sprintf(
				"%s is allowed and %s is denied, and both name a file ending %s - so the set would hold one of them to be taken and one to be turned away",
				desc.ID, written, desc.Extension),
			Hint: fmt.Sprintf("Take %s out of the %s list, or %s out of the %s list.",
				desc.ID, allowParam, written, denyParam),
		}
	}
	return nil
}

// allowedFormats is the allow list in registry order.
//
// Registry order rather than the order somebody typed, for the reason the
// minimal set found the hard way on 2026-09-22: walking the typing makes
// "--allow jpg,png" and "--allow png,jpg" two different recipes, two different
// recipe hashes and two file lists nobody can compare.
func allowedFormats(raw string) ([]format.Descriptor, error) {
	ids, err := allowList.parse(raw)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var out []format.Descriptor
	for _, id := range format.IDs() {
		if desc, err := format.Get(id); err == nil && wanted[id] {
			out = append(out, desc)
		}
	}
	return out, nil
}

// deniedExtensions is the deny list in alphabetical order.
//
// Alphabetical rather than the registry's, because half these entries are not
// in the registry and an order that covers only half of a list is not an order.
func deniedExtensions(raw string) ([]deniedEntry, error) {
	items, err := denyList.parse(raw)
	if err != nil {
		return nil, err
	}
	sort.Strings(items)
	filler, err := format.Get(fillerFormat)
	if err != nil {
		return nil, err
	}
	out := make([]deniedEntry, 0, len(items))
	for _, item := range items {
		entry := deniedEntry{ext: item, desc: filler}
		if desc, err := format.Get(item); err == nil {
			entry.desc, entry.known = desc, true
		}
		out = append(out, entry)
	}
	return out, nil
}

// farOverTimes reads the multiplier, or nought for the file turned off.
func farOverTimes(raw string) (int64, error) {
	if raw == farOverOff {
		return 0, nil
	}
	times, err := strconv.ParseInt(strings.TrimSuffix(raw, "x"), 10, 64)
	if err != nil || times < 2 {
		return 0, &format.PropertyValueError{
			Format: uploadID, Key: farOverParam, Value: raw,
			Reason: "it has to be a number of times the limit, written with an x - 2x or 10x - or off",
		}
	}
	return times, nil
}

func expandUploadValidation(args Args) ([]byte, error) {
	s, err := settleUpload(args)
	if err != nil {
		return nil, err
	}

	// The three files around the limit come from the same code the boundary set
	// uses, so "under the limit is accepted, over it is refused for size_limit"
	// exists once rather than in two presets that could drift.
	around := limitSet{
		preset: uploadID, setting: uploadLimitParam, group: sizeGroup,
		desc: s.allowed[0], limit: s.limit, limitText: s.limitText,
		spread: []offset{{text: "1b", bytes: 1}},
	}
	steps := around.steps()
	if err := around.reachable(steps); err != nil {
		return nil, err
	}

	files := s.files()
	if err := s.reachable(files); err != nil {
		return nil, err
	}
	targets := append(around.drafts(steps), draftsOf(files)...)
	return plan{preset: uploadID, question: uploadQuestion, targets: targets}.source()
}

// files is every file of the set except the three around the limit.
func (s uploadSet) files() []setFile {
	var out []setFile
	out = append(out, s.farOverFiles()...)
	out = append(out, s.degenerateFiles()...)
	out = append(out, s.allowedFiles()...)
	out = append(out, s.deniedFiles()...)
	out = append(out, s.mismatchFiles()...)
	out = append(out, s.anomalyFiles()...)
	out = append(out, s.nameFiles()...)
	out = append(out, s.bulkFiles()...)
	return out
}

// reachable refuses the whole set when any one file of it is out of reach.
//
// Only the files measured from the limit can be, because everything else is
// asked for at the sample size or at the format's own floor, whichever is
// larger. So the refusal always has the same way out and can name the limit
// that would work: the floor, scaled back up by the share of the limit this
// file is.
func (s uploadSet) reachable(files []setFile) error {
	var worst shortfall
	// A file asking what an earlier one asked is not planned again. The set
	// holds one small picture under a dozen names - a name with spaces, one
	// with no extension, one outside ASCII - and planning is encoding it, so
	// that was a dozen encodings of one question (docs/GUI-MEMORY-2026-09-23.md
	// section 4j). Skipping changes no answer: the same request gets the same
	// refusal and the same need, and the deepest shortfall keeps the FIRST
	// file that reached it, because the comparison below is strict.
	asked := map[string]bool{}
	for _, f := range files {
		if !firstTimeAsked(asked, f) {
			continue
		}
		short, err := s.shortfallOf(f)
		if err != nil {
			return err
		}
		// The deepest shortfall rather than the first, so the limit the
		// refusal names fixes every file at once. Naming the first would send
		// somebody back for the next refusal, and RC7 says a file fixed one
		// error per run is the cheapest way to make them stop using the tool.
		if short.need > worst.need {
			worst = short
		}
	}
	if worst.need == 0 {
		return nil
	}
	return s.cannotReach(worst)
}

// firstTimeAsked says whether a set is asking this file's question for the
// first time, and remembers that it now has. A question with no key - a file
// with contents - counts as asked for the first time, every time.
func firstTimeAsked(asked map[string]bool, f setFile) bool {
	key, ok := format.RequestKey(f.desc.ID, f.request())
	if !ok {
		return true
	}
	if asked[key] {
		return false
	}
	asked[key] = true
	return true
}

// shortfall is one file that is smaller than its format will write, and the
// limit at which it would stop being.
type shortfall struct {
	file  setFile
	floor int64
	need  int64
}

// shortfallOf measures one file. The error beside it is a refusal raising the
// limit would not fix, which goes straight back rather than being weighed.
func (s uploadSet) shortfallOf(f setFile) (shortfall, error) {
	err := f.refused()
	if err == nil {
		return shortfall{}, nil
	}
	var below *format.BelowMinimumError
	if !errors.As(err, &below) {
		return shortfall{}, &ImpossibleError{
			Preset: uploadID, Setting: uploadLimitParam,
			Detail: strings.TrimPrefix(err.Error(), f.desc.ID+": "),
		}
	}
	return shortfall{
		file: f, floor: below.Minimum,
		need: wouldReach(below.Minimum, f.bytes(), s.limit),
	}, nil
}

func (s uploadSet) cannotReach(short shortfall) error {
	return &ImpossibleError{
		Preset: uploadID, Setting: uploadLimitParam,
		// The same sentence the boundary set writes for the same situation.
		// Splicing the format's own reason in instead read as a contradiction:
		// it ends "already needs that much", and after "would be 1024 B" the
		// words pointed at the wrong number.
		Detail: fmt.Sprintf("%s would be %d B and the smallest %s this build makes is %d B",
			short.file.id, short.file.bytes(), strings.ToUpper(short.file.desc.ID), short.floor),
		Hint: fmt.Sprintf(
			"Raise the {setting} to %d B or more, or take %s out of the allowed types. The {setting} asked for was %d B.",
			short.need, short.file.desc.ID, s.limit),
	}
}

// wouldReach is the limit at which a file this far below its floor would reach
// it, given that the file is a fixed share of the limit. Rounded up, because a
// limit that lands a byte short is advice that fails when it is followed.
func wouldReach(floor, size, limit int64) int64 {
	if size <= 0 {
		return limit
	}
	return (floor*limit + size - 1) / size
}

// sampleFor is how big a file that is about its name or its insides should be:
// the sample, or the format's own floor where that is larger.
//
// The larger of the two rather than the sample, so that a format with a floor
// above it cannot turn a file about a NAME into a refusal about a size.
//
// The remembered floor rather than one worked out here. It is the same
// question - the label on, nothing else - and it was asked once for every
// file about a name or an insides, each time encoding pictures to find the
// answer: 35% of expanding upload-validation, measured 2026-09-23
// (docs/GUI-MEMORY-2026-09-23.md section 4j).
func sampleFor(desc format.Descriptor) int64 {
	return sampleAtLeast(desc, uploadSample)
}
