package text

import (
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// RegistryWord is one sentence of the registries the window shows, under the
// key it is looked up by, with where it stands - which a translator reading
// the bare sentence could not tell.
type RegistryWord struct {
	Key     string
	English string
	Where   string
}

// RegistryWords is every sentence of the registries the window shows, sorted
// by key - the list registry/en.json is written from and every other language
// is held to.
//
// Walked from the registries rather than written down, so a format added
// tomorrow brings its sentences with it, and a language without them turns a
// guard red instead of an English sentence arriving in a Polish window. Asked
// by the guards and never by the window, which looks each sentence up where it
// draws it - under the same keys, built by the same functions.
func RegistryWords() []RegistryWord {
	w := registryWords{}
	for _, d := range format.All() {
		w.format(d)
	}
	for _, p := range preset.All() {
		w.preset(p)
	}
	for _, d := range damage.All() {
		w.settings(DamageOwner(d.ID), "the "+d.ID+" damage", d.Parameters)
	}
	for _, d := range tool.All() {
		w.tool(d)
	}
	w.lists()
	for i, p := range LicenceParagraphs(version.LicenceNotice) {
		w.add(LicenceKey(i+1), p, "One paragraph of the licence notice on the About screen. The first is the name and the copyright line and stays as it is. The program's name and the licence's name stay as they are.")
	}
	return w.sorted()
}

// lists is the values of the two closed lists a window offers under "notes for
// the manifest": what the system under test should do, and the rule tested.
func (w registryWords) lists() {
	for _, o := range recipe.Outcomes() {
		w.add(ChoiceKey(recipe.KeyExpected, o), o, "One entry of the list of what the system under test should do with a file. The manifest writes the value itself.")
	}
	for _, r := range recipe.Reasons() {
		w.add(ChoiceKey(recipe.KeyExpectedReason, r), r, "One entry of the list of the rule a file tests. The manifest writes the value itself.")
	}
}

// registryWords collects by key. A key reached twice - a width declared by
// four formats has one label - holds the sentence it was first reached with.
type registryWords map[string]RegistryWord

func (w registryWords) add(key, english, where string) {
	if _, have := w[key]; english == "" || have {
		return
	}
	w[key] = RegistryWord{Key: key, English: english, Where: where}
}

func (w registryWords) format(d format.Descriptor) {
	owner := FormatOwner(d.ID)
	w.add(FormatNameKey(d.ID), d.Name, "What the "+d.ID+" format is called, beside its id in a list of formats. The name of a standard stays as it is - only a name that describes the file is translated.")
	w.settings(owner, "the "+d.ID+" format", d.Properties)
	for _, j := range d.JointLimits {
		w.add(JointKey(owner, j.Of, j.By), j.Why, "Why "+j.Of+" times "+j.By+" of the "+d.ID+
			" format have a ceiling, at the end of the note under the two, after the word because.")
		w.unit(j.Unit)
	}
}

func (w registryWords) preset(p preset.Preset) {
	whose := "the " + p.ID + " preset"
	w.add(TitleKey(p.ID), p.Title, "What "+whose+" is called, beside its id in the list of presets.")
	declared := append(append([]format.Property{}, p.Parameters...), p.Globals()...)
	w.settings(PresetOwner(p.ID), whose, declared)
	w.add(QuestionKey(p.ID), p.Question, "The question "+whose+" answers, at the top of the preset screen.")
	for i, line := range p.Catches {
		w.add(CatchKey(p.ID, i+1), line, "One line of what "+whose+" typically finds, on the preset screen.")
	}
	for name, said := range p.SaidWhenDefaulted {
		w.add(NoteKey(p.ID, name), said, "What a run of "+whose+" says when nobody gave its "+name+".")
	}
}

func (w registryWords) tool(d tool.Descriptor) {
	whose := "the " + d.ID + " tool"
	w.add(ToolQuestionKey(d.ID), d.Question, "The question "+whose+" answers - its name in the list of tools and the title over it.")
	w.add(ToolDetailKey(d.ID), d.Detail, "One sentence under the question of "+whose+", saying what it does.")
	for _, in := range d.Inputs {
		w.add(LabelKey(in.Name), EnglishLabel(in.Name), "The name beside the box of what "+whose+" works on.")
		w.add(InputKey(d.ID, in.Name), in.Detail, "The sentence under the "+in.Name+" box of "+whose+".")
		w.add(core.KindKey(string(in.Kind)), string(in.Kind), "What a tool works on, as a word, in a refusal that one was not given: no file was given.")
	}
	for _, n := range d.Notes {
		w.add(ToolNoteKey(d.ID, n.ID), n.Says, "A line of the result of "+whose+", ending in a colon - what follows it is a list of names or one number.")
	}
	w.settings(ToolOwner(d.ID), whose, d.Settings)
}

func (w registryWords) settings(owner Owner, whose string, declared []format.Property) {
	for _, p := range declared {
		w.add(LabelKey(p.Name), EnglishLabel(p.Name), "The name beside the box of a setting a recipe writes as "+p.Name+".")
		w.add(DetailKey(owner, p.Name), p.Detail, "The sentence under the "+p.Name+" setting of "+whose+", after what it takes.")
		w.add(GroupKey(owner, p.Group), p.Group, "The heading over a block of settings of "+whose+".")
		w.add(ShapeKey(p.Shape), p.Shape, "What the text in a box has to look like, in the sentence under it.")
		for _, c := range p.Choices {
			w.add(ChoiceKey(p.Name, c), c, "One entry of the "+p.Name+" list, as the list shows it. A recipe and the command line write the value itself, so a token such as utf-8, a4, sha256 or 644 stays as it is.")
		}
		w.unit(p.Unit)
	}
}

func (w registryWords) unit(unit string) {
	w.add(UnitKey(unit), unit, "What a number counts, after it, as in: from 1 to 20000 "+unit+
		". Written the way the language puts a noun after a number.")
}

func (w registryWords) sorted() []RegistryWord {
	out := make([]RegistryWord, 0, len(w))
	for _, word := range w {
		out = append(out, word)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
