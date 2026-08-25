package core

import (
	"fmt"
	"strings"
)

// SettingSlot stands in a refusal where the sentence names the setting it is
// about, so each surface can put its own name there.
//
// The engine words a refusal once for both surfaces and names the setting by
// the recipe key - "a target has no id". A window labels that box Group name,
// so the sentence was naming something the screen does not have. From
// 2026-08-20 the window closed that by swapping the key for the label in the
// finished sentence, and on 2026-08-25 that swap was measured on screen and
// found to come apart three ways at once:
//
//   - the key landed inside longer words. The key behind Output directory is
//     "dir", so the refusal read "the output Output directoryectory is empty".
//     Also identifies, formats, sizes and size-range.
//   - a recipe key quoted as an example was relabelled too, so the window
//     offered "for example Group name: invoices" - a key no recipe has.
//   - the article in front of it stayed, giving "an Group name".
//
// A slot ends the whole class rather than the three symptoms. The sentence
// says where the name goes, so nothing has to be found by searching prose, and
// a key quoted as a key is left alone because it is not a slot.
//
// The token is written the way it is because it cannot occur in a refusal
// somebody reads. Every one of these sentences is English we wrote.
const SettingSlot = "{setting}"

// ArticleSlot stands immediately before a SettingSlot that needs "a" or "an"
// in front of it.
//
// It exists because the two surfaces disagree about which article the same
// sentence needs. "an id anchors the seed" is what the command line has always
// printed and must go on printing, and the box that setting fills in a window
// is called Group name, which takes "a". A sentence that hard codes either one
// is wrong on the other surface, and dropping the article to sidestep that
// would change what the command line prints.
const ArticleSlot = "{a}"

// InTheWordsOf puts one name into every slot of a message.
//
// The command line asks for the recipe key, which is what it printed before
// slots existed, so its output does not move. A window asks for the label
// above the box.
func InTheWordsOf(message, name string) string {
	if name == "" {
		return message
	}
	return strings.NewReplacer(
		ArticleSlot, articleFor(name),
		SettingSlot, name,
	).Replace(message)
}

// articleFor is "a" or "an" in front of one name.
//
// The rule is the written letter rather than the sound, which is not the whole
// of English - "an hour" and "a university" both break it. It is right for
// every name either surface can put here, and those names are ours rather than
// anybody's input: the recipe keys are a closed list and so are the labels.
// TestEveryNameARefusalCanBeGivenTakesTheArticleThisRuleGivesIt walks both
// lists and would fail on the first name that needs the other answer.
func articleFor(name string) string {
	if name == "" {
		return ""
	}
	switch name[0] {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return "an"
	}
	return "a"
}

// SettingSize is the recipe key every refusal about a size is about.
//
// It lives here rather than in the format registry because a size is parsed
// here, one layer below, and a refusal that cannot name its own setting cannot
// be placed beside a box. format.SettingSize is this, so there is still one
// spelling of it. See format.SettingSize for why it is the recipe key rather
// than either surface's own word for the same thing.
const SettingSize = "size"

// SettingErrorf is a refusal that names the setting it is about through a slot,
// so each surface reads it in its own words.
//
// The format string is kept unrendered rather than the finished message,
// because the values put into it come from whoever ran the tool. A size typed
// as "{setting}" would otherwise be read back as a slot and answered with the
// name of the box, which is the shape of defect this whole mechanism exists to
// remove rather than to move somewhere quieter.
func SettingErrorf(setting, format string, a ...any) error {
	return &settingError{setting: setting, format: format, args: a}
}

type settingError struct {
	setting string
	format  string
	args    []any
}

func (e *settingError) Error() string { return e.InTheWordsOf(e.setting) }

func (e *settingError) InTheWordsOf(name string) string {
	if name == "" {
		name = e.setting
	}
	return fmt.Sprintf(InTheWordsOf(e.format, name), e.args...)
}

// AboutSetting lets a window put this refusal beside the box it came from,
// through the same interface the engine, the format registry and the preset
// package already answer.
func (e *settingError) AboutSetting() string { return e.setting }

// KeyTargets and KeyContains are the two list names a settings address is built
// from. The rest of the vocabulary lives in internal/recipe, with the reader
// that produces it - these two are here because the shape of an address is now
// built in two layers rather than one.
const (
	KeyTargets  = "targets"
	KeyContains = "contains"
)

// TargetAddress is where one setting of one target lives.
//
// The position counts from one, matching the prose: a refusal about the second
// target says "target 2", and a screen numbering its blocks from zero would
// send somebody to the wrong one.
//
// This lived in internal/recipe until 2026-08-25, next to the key vocabulary,
// which was right while the recipe reader was the only thing that produced an
// address. It is not any more. Measured that day: a recipe with three batches
// of four hundred thousand files passes the reader, because the reader bounds
// each target on its own, and is refused by the engine on the total - with a
// sentence and no address, so "validate --json" carried no "at" and a window
// had nothing to mark. Three more refusals reach a person the same way.
//
// So the shape moved down to where both layers already are. The alternative
// was the engine importing the recipe reader for a function that joins two
// strings, which would put a YAML parser inside the layer that is meant not to
// know what a recipe file looks like.
func TargetAddress(position int, setting string) string {
	return TargetPrefix(position) + "." + setting
}

// TargetPrefix is one target without a setting named yet, which is what a
// refusal about the target as a whole is addressed to.
func TargetPrefix(position int) string {
	return fmt.Sprintf("%s[%d]", KeyTargets, position)
}

// ContentAddress is where one setting of one contains entry lives. Both
// positions count from one, for the reason above.
func ContentAddress(target, entry int, setting string) string {
	return fmt.Sprintf("%s.%s[%d].%s", TargetPrefix(target), KeyContains, entry, setting)
}

// AddressNamesATarget says whether an address carries the position of the
// target it is about, rather than a setting name on its own.
//
// It exists because the two are not interchangeable in a report. A refusal from
// below the recipe reader knows its setting - "size" - and not which entry of
// the list it happened in, and "size" in a report about a recipe with twenty
// targets points at all of them at once. A reader grouping by field would put
// them in one bucket and a window would have no box to mark, so an address
// without its position is worse than none: it looks actionable.
//
// Not every address needs one. A setting of the document itself, output.dir or
// version, is complete as it stands and is not what this asks about.
func AddressNamesATarget(address string) bool {
	return strings.HasPrefix(address, KeyTargets+"[")
}

// LastSettingSegment is the plain name inside a settings address.
//
// A refusal about a batch addresses its setting as targets[1].properties.width
// and both surfaces name that box Width, so the address is not the word a
// sentence wants. Whatever follows the final dot is.
func LastSettingSegment(address string) string {
	if at := strings.LastIndex(address, "."); at >= 0 {
		return address[at+1:]
	}
	return address
}
