package guard

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// The window says the engine's sentences without the recipe's words - no
// target, no line of a file, no key written as key: value, no command to run.
// docs/SIZE-FIXES-2026-11-16.md section 6.1, M3b. The English window rewords
// them with the phrases of internal/gui/text/recipewords.go, and a translation
// says them in its own words.

// TestTheWindowSaysNoWordOfTheRecipe holds every sentence the code says:
//
//   - in English, as the window lays it out, it carries the values and the
//     slots of the command line's layout in the same order - a phrase that
//     dropped or moved one would print another value in its place;
//   - in English and in every translation, no sentence a window can show
//     carries a word of the recipe. A sentence only a recipe file can bring
//     about - the file's version, its sections, its separators, keys no screen
//     writes - is listed in notOnAScreen with the reason;
//   - every phrase of the table rewords some sentence, and every sentence of
//     the list still carries a word of the recipe on the command line - a list
//     nothing compares with the code goes stale while staying green.
func TestTheWindowSaysNoWordOfTheRecipe(t *testing.T) {
	sentences := engineSentences(t)
	words := recipeWordsOf(t)

	// The canary: the check has to find each kind of word in a sentence made
	// of them, and the layout check has to see a value dropped.
	planted := `target %q: leave the line out, or add size: 2mb or compression=none to the recipe, then run "tfg formats" for a dry run`
	if found := words.in(planted, text.English); len(found) < 7 {
		t.Fatalf("a sentence made of the recipe's words is found to carry only %v, so the check below would pass a sentence that carries them", found)
	}
	if keepsTheValues("target %q of %d", "batch %d") {
		t.Fatal("a layout that dropped a value is taken for one that keeps them, so a phrase could move a value unseen")
	}
	// A sentence said on its own - a note of a run, a refusal about a setting -
	// rather than inside a refusal of the recipe's, which every refusal below
	// is: the window lays its own layout out as well.
	own := core.Says("guard.Own", "target %q asks for 0 files", core.A("Target", "a"))
	for _, said := range []string{text.Sentence(own), text.Refusal(core.RefuseAbout("count", own), "")} {
		if found := words.in(said, text.English); len(found) > 0 {
			t.Errorf("a sentence said on its own reads %q in the English window, which carries %v", said, found)
		}
	}

	used := map[string]bool{}
	for _, id := range sortedIDs(sentences) {
		c := sentences[id]
		for _, layout := range layoutsOf(c) {
			window := text.InTheScreensWords(layout)
			if !keepsTheValues(layout, window) {
				t.Errorf("%s: the window lays %q out as %q, which does not keep its values and slots in order", id, layout, window)
			}
			for _, p := range text.RecipePhrases() {
				if standsWhole(layout, p) {
					used[p] = true
				}
			}
			if _, fileOnly := notOnAScreen[id]; fileOnly {
				continue
			}
			if found := words.in(window, text.English); len(found) > 0 {
				t.Errorf("%s: the English window says %q, which carries %v", id, window, found)
			}
		}
	}
	for _, p := range text.RecipePhrases() {
		if !used[p] {
			t.Errorf("the phrase %q rewords no sentence the code says, so it is a rule about nothing", p)
		}
	}
	for id, why := range notOnAScreen {
		c, known := sentences[id]
		switch {
		case !known:
			t.Errorf("notOnAScreen lists %s (%s), which the code no longer says", id, why)
		case len(words.in(c.other, text.English)) == 0:
			t.Errorf("notOnAScreen lists %s (%s), which carries no word of the recipe any more", id, why)
		}
	}

	// Then the window itself, in English: refusals of recipes written the way
	// Several batches writes them, and of batches handed over the way Single
	// batch hands them, said at the foot of the form and under a box - where
	// the place a refusal names is a sentence inside a type of the recipe's.
	shown := 0
	for _, err := range refusalsAWindowGets(t) {
		for _, label := range []string{"", text.FieldSize()} {
			said := text.Refusal(err, label)
			if testing.Verbose() && label == "" {
				t.Logf("the window says: %s", said)
			}
			if found := words.in(said, text.English); len(found) > 0 {
				t.Errorf("the English window says\n  %q\nwhich carries %v", said, found)
			}
			shown++
		}
	}
	if shown < 20 {
		t.Fatalf("the window was asked %d time(s), so the refusals below were not reached", shown)
	}

	translations := catalogueFilesIn(t, saidDir)
	if len(translations) < 2 {
		t.Fatal("no language but English has the engine's sentences, so no translation is asked")
	}
	for tag, entries := range translations {
		if tag == text.English {
			continue
		}
		for _, id := range sortedIDs(sentences) {
			if _, fileOnly := notOnAScreen[id]; fileOnly {
				continue
			}
			for form, said := range entries[id] {
				if form == "hash" || form == "description" {
					continue
				}
				if found := words.in(said, tag); len(found) > 0 {
					t.Errorf("%s %s (%s) says %q, which carries %v", tag, id, form, said, found)
				}
			}
		}
	}
}

// refusalsAWindowGets is refusals a screen gets back: of documents written the
// way Several batches writes them, read the way it reads them, and of batches
// planned the way Single batch plans them. Each has to be refused.
func refusalsAWindowGets(t *testing.T) []error {
	t.Helper()
	png := func(id, size string) recipe.TargetDraft { return recipe.TargetDraft{ID: id, Format: "png", Size: size} }
	documents := []recipe.Document{
		{Targets: []recipe.TargetDraft{{Format: "png"}}},
		{Targets: []recipe.TargetDraft{png("a", "1kb"), png("a", "1kb")}},
		{Seed: "abc", Targets: []recipe.TargetDraft{{ID: "a", Format: "png", Size: "1kb", Count: "abc"}}},
		{Targets: []recipe.TargetDraft{{ID: "a", Format: "png", Count: "0", Size: "1kb"}}},
		{Targets: []recipe.TargetDraft{{ID: "a", Format: "png", SizeRange: "zz"}}},
		{Targets: []recipe.TargetDraft{{ID: "a", Format: "png", Boundary: "zz"}}},
		{Targets: []recipe.TargetDraft{{ID: "a", Format: "zip", Size: "1kb", Contains: []recipe.ContentDraft{{Count: "x"}, {Format: "txt", Count: "-1"}}}}},
		{Targets: []recipe.TargetDraft{{ID: "a", Format: "png", Size: "1kb", Purpose: "a\x01b"}}},
	}
	var out []error
	for _, d := range documents {
		src, err := recipe.Compose(d)
		if err == nil {
			_, err = preset.ReadRecipe(src, text.TabRecipe())
		}
		if err == nil {
			t.Fatalf("%+v was read without a refusal, so the refusal it stands for is not asked", d)
		}
		out = append(out, spreadAll(t, err)...)
	}
	for _, targets := range [][]engine.Target{
		{{ID: "files", Format: "png", Sizes: engine.Uniform(0, 1024)}},
		{{Format: "png", Sizes: engine.Uniform(1, 1024)}},
		{{ID: "a", Format: "txt", Sizes: engine.Uniform(1, 10), NameTmpl: "same.txt"}, {ID: "b", Format: "txt", Sizes: engine.Uniform(1, 10), NameTmpl: "same.txt"}},
		{{ID: "a", Format: "txt", Sizes: engine.Uniform(1, 10), Smallest: true}},
	} {
		_, err := engine.Plan(targets, engine.Options{OutDir: t.TempDir(), DryRun: true})
		if err == nil {
			t.Fatalf("%+v was planned, so the refusal it stands for is not asked", targets)
		}
		out = append(out, spreadAll(t, err)...)
	}
	return out
}

// notOnAScreen is every sentence that carries a word of the recipe and that no
// screen can bring about, with the reason. A screen writes its recipe through
// recipe.Compose, which writes the version, the sections and the separators
// itself and only the keys of recipe.Document, one value to a key - and Single
// batch and Presets write no recipe at all.
var notOnAScreen = map[string]string{
	"placeholder.ColourIsEmpty": commentCutIt,

	"preset.ARecipeCanBuildOnlyOn":   presetFromAList,
	"preset.RunTfgPresetListForThe":  presetFromAList,
	"preset.RunTfgFormatsForTheList": "with.format naming a format this build lacks - a screen picks the format from a list",
	"preset.RunTfgFormatsToSeeWhat":  "a build in which no format carries an encoding or a line ending, which this build is not",

	"recipe.ARecipeCanBuildOnA":         presetFromAList,
	"recipe.AddExtendsPresetIdAboveIt":  presetFromAList,
	"recipe.ReadTheFileWithParseOr":     presetFromAList,
	"recipe.WriteExtendsPresetIdAndRun": presetFromAList,

	"recipe.APresetSTargetsWereSupplied": readerMisused,
	"recipe.ARecipeThatBuildsOnA":        readerMisused,
	"recipe.ReadTheFileWithTfgGenerate":  readerMisused,
	"recipe.ThisRecipeBuildsOnPresetAnd": readerMisused,
	"recipe.ThisRecipeDoesNotBuildOn":    readerMisused,

	"recipe.PresetCarries":       presetAtFault,
	"recipe.ThisIsAMistakeInThe": presetAtFault,

	"recipe.ARecipeIsOneDocumentAnd":          theFileItself,
	"recipe.RemoveTheSeparatorsOrSplitThe":    theFileItself,
	"recipe.SettlingItTwiceGivesTwoDifferent": theFileItself,
	"recipe.Unreadable":                       theFileItself,

	"recipe.EveryRecipeDeclaresTheSchemaVersion": theVersion,
	"recipe.TheRecipeHasNoVersion":               theVersion,
	"recipe.UpgradeTheToolOrWriteThe":            theVersion,

	"recipe.ARecipeWithoutTargetsHasNothing": "no batch at all - Several batches keeps its last batch unless a preset brings the files",
	"recipe.TheRecipeAsksForNoFiles":         "no batch at all - Several batches keeps its last batch unless a preset brings the files",

	"recipe.AllowNondeterministicTrueHasNothingTo": "allow_nondeterministic, a key no screen writes",
	"recipe.TheToolVersionThisRecipeRequires":      "engine, a key no screen writes",
	"recipe.UseExpectedAcceptOrABlock":             "expected in a shape of its own - a screen writes the outcome from a list",
	"recipe.UseFormatPdfOrRunTfg":                  "a format that is not a name - a screen writes the format from a list",
	"recipe.UseSize2mbOrSize2097152":               "a size that is not text - a screen writes every box as text",
	"recipe.GiveItOneValueForExample":              oneValueABox,
	"recipe.GiveItOneValueForExample2":             oneValueABox,

	"format.RemoveTheLine":                       "a setting the format does not have - a screen offers only the settings a format declares",
	"preset.RemoveTheLine":                       "with naming a parameter the preset does not take - a screen writes only the preset's own",
	"recipe.RemoveTheKeyOrMoveIt":                "a key inside contains other than its format, count and size - a screen writes only those three",
	"recipe.RemoveTheLineOrPutThe":               "expected with keys of its own - a screen writes the outcome and the reason only",
	"recipe.RemoveTheLine":                       "output.split_threshold, a key no screen writes",
	"recipe.RemoveTheLineContentIsGenerated":     "fill, a key no screen writes",
	"recipe.RemoveTheLineItWillBe":               "allow_nondeterministic, a key no screen writes",
	"recipe.RemoveTheLineTheManifestRecords":     "engine, a key no screen writes",
	"recipe.RemoveTheSectionTheExpectedField":    "policy, a key no screen writes",
	"recipe.UseLocaleEnOrLeaveThe":               "locale, a key no screen writes",
	"recipe.AddAtLeastOneEntryUnder":             "no batch at all - Several batches keeps its last batch unless a preset brings the files",
	"recipe.APresetContributesTargetsAndNothing": presetAtFault,
	"recipe.ATargetThatDamagesNothingProduces":   damageInARecipe,
	"recipe.NameADamageOrRemoveThe":              damageInARecipe,
	"recipe.TooDeep":                             theFileItself,
	"damage.UnknownInsteadRemove":                "a damage this build does not have - Single batch offers the damages from a list",
}

// The reasons notOnAScreen gives more than once.
const (
	commentCutIt    = "a colour cut off by #, which starts a comment only in a recipe file - a screen hands the colour over whole"
	presetFromAList = "extends or with written by hand - a screen writes extends from its list of this build's presets, and with only beside it"
	readerMisused   = "a recipe handed to the reader that does not expand presets, or presets to one that does not build on them - a fault of the code calling it"
	presetAtFault   = "a preset whose own recipe carries more than targets, or is refused - a fault of the preset, and every preset of this build is read by the guards"
	theFileItself   = "the file as a document - a screen writes one document of its own boxes, settled, through recipe.Compose"
	theVersion      = "the version of the file - a screen writes the version this build reads"
	oneValueABox    = "a setting written as a list - a screen writes one value to a box"
	damageInARecipe = "a damage in a recipe - Several batches offers none, and Single batch writes no recipe"
)

// layoutsOf is every layout of a sentence: the one form, or both.
func layoutsOf(c saidCall) []string {
	if c.plural {
		return []string{c.one, c.other}
	}
	return []string{c.other}
}

// sortedIDs is every id, in order, so a failure reads the same each time.
func sortedIDs(sentences map[string]saidCall) []string {
	out := make([]string, 0, len(sentences))
	for id := range sentences {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// keepsTheValues is whether window takes the values of layout in the same
// order and the same way. It may leave out a slot the setting's name goes in -
// a window says that name in a box's label - and may not add one, which would
// stand in the sentence unfilled wherever no name is given.
func keepsTheValues(layout, window string) bool {
	return strings.Join(core.DirectivesOf(layout), " ") == strings.Join(core.DirectivesOf(window), " ") &&
		strings.Count(window, core.SettingSlot) <= strings.Count(layout, core.SettingSlot) &&
		strings.Count(window, core.ArticleSlot) <= strings.Count(layout, core.ArticleSlot)
}

// recipeWords is what a sentence may not carry in a window, by language.
type recipeWords struct {
	// keyed is a key followed by ": " or "=", and what stands after it.
	keyed *regexp.Regexp
	// values is every value a key can be written with, so "a size: write one"
	// is prose and "size: 2mb" is the recipe.
	values map[string]bool
	words  map[string][]*regexp.Regexp
}

// in is every word of the recipe a layout or a translation carries, in a
// language. The fields of a translation are names of values, not words, and
// are left out.
func (w recipeWords) in(said, lang string) []string {
	said = fieldFound.ReplaceAllString(said, " ")
	var out []string
	for _, m := range w.keyed.FindAllStringSubmatch(said, -1) {
		if w.isAValue(m[1], strings.TrimRight(m[2], `.,"`)) {
			out = append(out, m[0])
		}
	}
	for _, rx := range append(w.words["*"], w.words[lang]...) {
		out = append(out, rx.FindAllString(said, -1)...)
	}
	return out
}

// isAValue is whether what stands after a key is a value written for it: a
// number or a size, a value some list declares, a preset named by its id, or
// any word given as an id.
func (w recipeWords) isAValue(key, v string) bool {
	return v != "" && (v[0] >= '0' && v[0] <= '9' || w.values[v] || strings.HasPrefix(v, "preset:") || key == "id")
}

// recipeWordsOf is the words, with the keys and their values read out of the
// registries: every key a recipe accepts and every setting a format, a damage,
// a preset or a tool declares.
func recipeWordsOf(t *testing.T) recipeWords {
	t.Helper()
	keys, values := map[string]bool{}, map[string]bool{"true": true, "false": true}
	for _, k := range recipe.Keys() {
		keys[k[strings.LastIndex(k, ".")+1:]] = true
	}
	add := func(props []format.Property) {
		for _, p := range props {
			keys[p.Name] = true
			for _, c := range p.Choices {
				values[c] = true
			}
		}
	}
	for _, d := range format.All() {
		values[d.ID] = true
		add(d.Properties)
	}
	for _, d := range damage.All() {
		values[d.ID] = true
		add(d.Parameters)
	}
	for _, p := range preset.All() {
		add(p.Parameters)
	}
	for _, d := range tool.All() {
		add(d.Settings)
	}
	for _, v := range append(recipe.Outcomes(), recipe.Reasons()...) {
		values[v] = true
	}
	if len(keys) < 100 || len(values) < 100 {
		t.Fatalf("%d key(s) and %d value(s) read out of the registries, so the check for key: value would find almost nothing", len(keys), len(values))
	}
	quoted := make([]string, 0, len(keys))
	for k := range keys {
		quoted = append(quoted, regexp.QuoteMeta(k))
	}
	sort.Slice(quoted, func(i, j int) bool { return len(quoted[i]) > len(quoted[j]) })
	return recipeWords{
		keyed:  regexp.MustCompile(`\b(` + strings.Join(quoted, "|") + `)(?:: |=)(\S+)`),
		values: values,
		words: map[string][]*regexp.Regexp{
			"*": {regexp.MustCompile(`\btfg\b`)},
			text.English: {
				regexp.MustCompile(`(?i)\btargets?\b`),
				regexp.MustCompile(`(?i)\brecipes?\b`),
				regexp.MustCompile(`(?i)\bthe line\b|\b(remove|leave) the \w+ line\b`),
				regexp.MustCompile(`(?i)\bdry[ -]run\b`),
			},
			// Go's \b is a boundary of ASCII words, and these end in a letter
			// outside it.
			"pl": {
				regexp.MustCompile(`(?i)\bprzepis\p{L}*`),
				regexp.MustCompile(`(?i)\b(usuń|pomiń) (ten |tę )?(wiersz|linię)`),
				regexp.MustCompile(`(?i)\bprzebieg\p{L}* próbn`),
			},
		},
	}
}

// standsWhole is whether phrase stands in layout with no letter or digit right
// before or right after it - the way the window replaces one.
func standsWhole(layout, phrase string) bool {
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9])` + regexp.QuoteMeta(phrase) + `(?:$|[^A-Za-z0-9])`).MatchString(layout)
}
