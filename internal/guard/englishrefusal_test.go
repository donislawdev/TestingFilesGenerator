package guard

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The English window says the engine's sentences in the command line's
// English, with a value of a list by its name and a setting by its label -
// docs/SIZE-FIXES-2026-11-16.md section 6.1, M3a. Both surfaces render through
// one function in core, the command line with nothing reworded, so what these
// guards hold is the half a reading cannot: that rewording touches the names
// and the labels and not one other byte.

// nothingDifferent is a wording that answers for no value and keeps every
// layout. A sentence reworded by it has to be the command line's sentence to
// the byte.
type nothingDifferent struct{}

func (nothingDifferent) Word(any) (string, bool)     { return "", false }
func (nothingDifferent) Layout(layout string) string { return layout }

// sayNothingDifferent is the wording that says everything as the command line
// does.
var sayNothingDifferent core.Wording = nothingDifferent{}

// rewordable is an error that says itself in a surface's words.
type rewordable interface {
	Reworded(string, core.Wording) string
}

// TestTheEnglishWindowRewordsOnlyNamesAndLabels.
//
// First every kind of value a sentence can carry, planted, so a kind the
// refusals below happen not to use is still asked: reworded by a wording that
// answers for nothing, each has to come out as the command line prints it.
// Then the same of every refusal the registries give for a bad value. Then the
// window itself, in English: every value of a list a refusal carries is said
// by its name and every setting it names by its label, wherever the two
// differ from what the command line prints.
func TestTheEnglishWindowRewordsOnlyNamesAndLabels(t *testing.T) {
	if got := text.AllowedWholeNumber(); got != "whole number" {
		t.Fatalf("this process speaks another language (%q), so nothing here is English to ask", got)
	}
	for _, c := range plantedSentences() {
		if got := c.said.Reworded(c.name, sayNothingDifferent); got != c.said.InTheWordsOf(c.name) {
			t.Errorf("%s: reworded with nothing to reword it says\n  %q\nand the command line\n  %q", c.what, got, c.said.InTheWordsOf(c.name))
		}
	}
	// The canary: the same check has to see a sentence that IS reworded.
	canary := core.Says("guard.Canary", "set %s to %q", core.A("Key", core.LabelTerm("bbox")), core.A("Value", core.Choice{Of: "content", Value: "test"}))
	window := text.Sentence(canary)
	if window == canary.String() || !strings.Contains(window, text.SettingLabel("bbox")) ||
		!strings.Contains(window, text.ChoiceName("content", "test")) || strings.Contains(window, `"`) {
		t.Fatalf("the window says %q for %q, so it rewords nothing and the checks below would be green for nothing", window, canary.String())
	}
	// A refusal about a setting, which leaves the place of its name in the
	// sentence until a surface fills it, at the foot of the form: its own text
	// is not its sentence's, and it is still reworded.
	about := core.RefuseAbout("content", core.Says("guard.About", "{setting} cannot be %q here", core.A("Value", core.Choice{Of: "content", Value: "test"})))
	if foot := text.Refusal(about, ""); !strings.Contains(foot, text.ChoiceName("content", "test")) {
		t.Errorf("at the foot of the form a refusal about a setting says %q, where the command line says %q", foot, about.Error())
	}

	corpus := refusalCorpus(t)
	named := 0
	for _, err := range corpus {
		var r rewordable
		if errors.As(err, &r) {
			if got, cli := r.Reworded("", sayNothingDifferent), r.(error).Error(); got != cli {
				t.Errorf("reworded with nothing to reword it, a refusal says\n  %q\nand the command line\n  %q", got, cli)
			}
		}
		// Where the window puts it: at the foot of the form, and under the box
		// of the setting it is about.
		shown := []string{text.Refusal(err, "")}
		var about interface{ AboutSetting() string }
		if errors.As(err, &about) && about.AboutSetting() != "" {
			shown = append(shown, text.Refusal(err, text.SettingLabel(core.LastSettingSegment(about.AboutSetting()))))
		}
		for _, word := range windowWordsIn(err) {
			for _, one := range shown {
				if !strings.Contains(one, word) {
					t.Errorf("the English window does not say %q in\n  %q\nwhere the command line says\n  %q", word, one, err.Error())
				}
			}
			named++
		}
	}
	if len(corpus) < 300 || named < 15 {
		t.Fatalf("%d refusal(s), naming %d value(s) or setting(s) the window words differently, so the registries were not swept", len(corpus), named)
	}
	t.Logf("%d refusal(s), %d name(s) and label(s) the window words differently", len(corpus), named)
}

// TestARefusalWithNoBoxNamesItsSettingsByTheirLabels asks two settings of a log
// refused together, which the engine keys "level_mix and entry_format" - a key
// no box on any screen has, so the refusal stands at the foot of the form. It
// has to name both settings by their labels: the English window said the two
// keys, and the Polish one left the place empty. Asked in English here and in
// Polish in a process of its own.
func TestARefusalWithNoBoxNamesItsSettingsByTheirLabels(t *testing.T) {
	polish := os.Getenv(listWordsPolishChild) == "1"
	if polish {
		if err := text.LoadBuiltIn("pl"); err != nil {
			t.Fatalf("Polish would not load: %v", err)
		}
	}
	_, err := engine.Plan([]engine.Target{{ID: "a", Format: "log", Sizes: engine.Uniform(1, 4096),
		Properties: map[string]string{"entry_format": "apache-combined", "level_mix": "errors"}, Label: true}},
		engine.Options{OutDir: t.TempDir(), DryRun: true})
	if err == nil {
		t.Fatal("a log of an Apache shape with a mix of levels was planned, so there is no refusal to ask")
	}
	for _, one := range spreadAll(t, err) {
		shown := text.Refusal(one, "")
		for _, key := range []string{"level_mix", "entry_format"} {
			if label := text.SettingLabel(key); !strings.Contains(shown, label) || strings.Contains(shown, key) {
				t.Errorf("at the foot of the form the refusal does not name %s as %q:\n  %s", key, label, shown)
			}
		}
	}
	if !polish {
		runChild(t, "TestARefusalWithNoBoxNamesItsSettingsByTheirLabels", listWordsPolishChild)
	}
}

// plantedSentence is a sentence and the name its slots are filled with.
type plantedSentence struct {
	what string
	name string
	said core.Said
}

// plantedSentences is a sentence for each kind of value core keeps as it is
// until the sentence is read, and for each way a layout prints one.
func plantedSentences() []plantedSentence {
	inner := core.Says("guard.Inner", "the %s shape with %q", core.A("Shape", core.Choice{Of: "entry_format", Value: "json-lines"}), core.A("Typed", "a\tb"))
	wrapped := core.RefuseAbout("width", core.Says("guard.Wrapped", "{a} {setting} of %d is too wide", core.A("Value", 9)))
	return []plantedSentence{
		{"a value of a list, quoted", "", core.Says("guard.Choice", "expects %q", core.A("Outcome", core.Choice{Of: "expected", Value: "reject"}))},
		{"a setting and its slot", "bbox", core.Says("guard.Label", "{a} {setting} beside %s", core.A("Key", core.LabelTerm("bbox")))},
		{"a sentence inside, quoted and not", "", core.Says("guard.Nested", "%s, then %q", core.A("One", inner), core.A("Two", inner))},
		{"a refusal wrapped", "", core.Says("guard.Wraps", "refused: %w", core.A("Cause", wrapped))},
		{"an error that is not ours", "", core.Says("guard.Plain", "failed: %v", core.A("Cause", errors.New("disk said no")))},
		{"sentences joined three ways", "", core.Says("guard.Lists", "%s / %s / %s",
			core.A("And", core.Conjoined{inner, inner}), core.A("Comma", core.Sentences{inner, inner}), core.A("Lines", core.Lines{inner, inner}))},
		{"several values of a list", "", core.Says("guard.Choices", "one of %s", core.A("Values", core.Choices{Of: "time_order", Values: []string{"in-order", "out-of-order"}}))},
		{"names joined", "", core.Says("guard.Joined", "%s", core.A("Names", core.Joined{Items: []string{"a", "b", "c"}}))},
		{"a sentence under a type of its own", "", core.Says("guard.Place", "%s has no %s", core.A("At", placeNamed{said: inner}), core.A("Other", placeNamed{said: inner, adds: " size"}))},
		{"a size and a unit", "", core.Says("guard.Bytes", "%s of %d %s", core.A("Size", core.Bytes(10<<20)), core.A("Count", 3), core.A("Unit", core.UnitTerm("pixels")))},
		{"a width the layout asks for", "", core.Says("guard.Width", "[%-12s|%8q]", core.A("A", core.Choice{Of: "content", Value: "test"}), core.A("B", "x"))},
		{"one of a number", "", core.SaysN("guard.One", "%d file holds it", "%d files hold it", core.A("Count", 1))},
		{"several of a number", "", core.SaysN("guard.Many", "%d file holds it", "%d files hold it", core.A("Count", 2))},
		{"a count not printed", "", core.SaysN("guard.Unsaid", "the folder holds", "the folders hold", core.A("Count", 2))},
		{"no full stop, a capital", "", core.Says("guard.Shaped", "expects %s.", core.A("Outcome", core.Choice{Of: "expected", Value: "accept"})).WithoutFullStop().Capitalized()},
	}
}

// placeNamed is a sentence of ours under a type of its own, the way the recipe
// names a place in itself - printing its sentence, or adding to it.
type placeNamed struct {
	said core.Said
	adds string
}

func (p placeNamed) String() string  { return p.said.String() + p.adds }
func (p placeNamed) Said() core.Said { return p.said }

// refusalCorpus is every refusal the registries give for a value they do not
// take, opened into the ones each carries - what a window is really shown,
// rather than sentences built for the purpose.
func refusalCorpus(t *testing.T) []error {
	t.Helper()
	var out []error
	add := func(err error) {
		if err != nil {
			out = append(out, spreadAll(t, err)...)
		}
	}
	for _, err := range engineRefusals(t) {
		add(err)
	}
	bad := []string{"zzz", "-1", "0", "99999999999", "1.5"}
	for _, d := range format.All() {
		for _, p := range d.Properties {
			for _, v := range bad {
				add(d.CheckProperties(map[string]string{p.Name: v}))
			}
		}
	}
	// Settings that are each fine and refused together, planned the way a run
	// plans them - which is where a refusal names a value of a list or another
	// setting. Each has to be refused, or it asks nothing.
	for _, c := range []struct {
		format string
		size   int64
		props  map[string]string
	}{
		{"log", 4096, map[string]string{"entry_format": "apache-combined", "level_mix": "errors"}},
		{"zip", 20000, map[string]string{"password": "secret", "encryption": "aes-256", "compression": "best"}},
		{"zip", 20000, map[string]string{"depth": "0", "directory_entries": "true"}},
		{"otlp", 20000, map[string]string{"formatting": "indented", "batch": "5"}},
		{"html", 10, map[string]string{"nesting": "5", "styles": "inline"}},
		{"rtf", 10, map[string]string{"nesting": "5"}},
	} {
		_, err := engine.Plan([]engine.Target{{ID: "a", Format: c.format, Sizes: engine.Uniform(1, c.size), Properties: c.props, Label: true}},
			engine.Options{OutDir: t.TempDir(), DryRun: true})
		if err == nil {
			t.Fatalf("%s with %v was planned, so the refusal it stands for is not asked", c.format, c.props)
		}
		add(err)
	}
	for _, p := range preset.All() {
		for _, param := range p.Parameters {
			_, err := preset.Expand(p.ID, preset.Args{param.Name: "zzz"})
			add(err)
		}
	}
	return out
}

// windowWordsIn is every name and label a refusal carries that the English
// window says differently from the command line, read off the sentence inside
// it - the one a window rewords.
func windowWordsIn(err error) []string {
	var said interface{ Said() core.Said }
	if !errors.As(err, &said) {
		return nil
	}
	var out []string
	var walk func(s core.Said)
	walk = func(s core.Said) {
		for _, a := range s.Args() {
			switch v := a.Value.(type) {
			case core.Choice:
				if name := text.ChoiceName(v.Of, v.Value); name != v.Value {
					out = append(out, name)
				}
			case core.Term:
				if key, isLabel := v.Setting(); isLabel && text.SettingLabel(key) != key {
					out = append(out, text.SettingLabel(key))
				}
			case core.Said:
				walk(v)
			case interface{ Said() core.Said }:
				walk(v.Said())
			}
		}
	}
	walk(said.Said())
	return out
}
