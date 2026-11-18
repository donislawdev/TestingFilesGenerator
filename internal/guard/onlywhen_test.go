package guard

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A setting that changes a file only while another setting has some values
// (format.Condition) - a placeholder's colours, while the content is not the
// test pattern. The owner's decision of 2026-11-17 hides it in the window
// until it applies (docs/SIZE-FIXES-2026-11-16.md section 6.5). These hold the
// three things that decision rests on. The command line, which hides nothing,
// is told no in words rather than ignored. The window shows exactly the rows
// that apply. And a value left in a hidden box never reaches the run.

// TestASettingThatChangesNothingIsRefusedRatherThanIgnored asks every format
// that declares a condition to plan each such setting while the setting it
// depends on is somewhere the condition does not hold. A recipe can say that,
// and a value read and then ignored is a colour somebody waits for in silence
// (rule 6), so each has to come back refused, about that setting.
func TestASettingThatChangesNothingIsRefusedRatherThanIgnored(t *testing.T) {
	asked := 0
	for _, d := range format.All() {
		for _, p := range d.Properties {
			if p.OnlyWhen.IsZero() {
				continue
			}
			asked++
			if wrong := ignoredWhileItChangesNothing(d, p); wrong != "" {
				t.Error(wrong)
			}
		}
	}
	// The four settings of a placeholder in each of png, jpg and svg.
	if asked < 12 {
		t.Fatalf("%d settings declare a condition, where twelve were known on 2026-11-17 - the "+
			"declaration has moved, and this guard asks less than it did", asked)
	}

	// The canary: the same question of a png that drops the colour of a
	// placeholder before planning has to find the silence.
	png, err := format.Get("png")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range png.Properties {
		if p.Name == "background" {
			png.Generator = dropping{Generator: png.Generator, name: p.Name}
			if ignoredWhileItChangesNothing(png, p) == "" {
				t.Fatal("canary: a png that ignores background on the test pattern passed, so this " +
					"guard cannot tell a refusal from silence")
			}
			return
		}
	}
	t.Fatal("canary: png declares no background, so the canary never ran")
}

// ignoredWhileItChangesNothing plans d with p asked for and the setting p
// depends on at a value under which p changes nothing, and says what is wrong
// when that is not refused about p.
func ignoredWhileItChangesNothing(d format.Descriptor, p format.Property) string {
	elsewhere := ""
	for _, q := range d.Properties {
		if q.Name == p.OnlyWhen.Setting {
			for _, v := range q.Choices {
				if !slices.Contains(p.OnlyWhen.Values, v) {
					elsewhere = v
					break
				}
			}
		}
	}
	if elsewhere == "" {
		return fmt.Sprintf("%s: %s depends on %s and no value of it leaves the condition unmet",
			d.ID, p.Name, p.OnlyWhen.Setting)
	}
	// The same plan without p is taken first, so the refusal below is about
	// p and not about anything else the request asks.
	ask := format.Request{Bytes: 64 << 10, Seed: 1, Label: true,
		Properties: map[string]string{p.OnlyWhen.Setting: elsewhere}}
	if _, err := d.Generator.Plan(ask); err != nil {
		return fmt.Sprintf("%s: 64 KiB with %s=%s and nothing else was refused, so nothing here "+
			"asks about %s: %v", d.ID, p.OnlyWhen.Setting, elsewhere, p.Name, err)
	}
	props := map[string]string{p.OnlyWhen.Setting: elsewhere, p.Name: askingFor(p)}
	ask.Properties = props
	_, err := d.Generator.Plan(ask)
	var refused *format.PropertyValueError
	if !errors.As(err, &refused) || refused.Key != p.Name {
		return fmt.Sprintf("%s: %s=%s with %s=%s, where it changes nothing, came back %v - "+
			"not a refusal about %s", d.ID, p.Name, props[p.Name], p.OnlyWhen.Setting, elsewhere, err, p.Name)
	}
	return ""
}

// askingFor is a value of p that asks for something: the other position of
// a switch, a value of a list other than its default, and for the rest the
// default itself, which a box only holds when somebody wrote it there.
func askingFor(p format.Property) string {
	switch p.Kind {
	case format.PropertyBool:
		if p.Default == "true" {
			return "false"
		}
		return "true"
	case format.PropertyChoice:
		for _, v := range p.Choices {
			if v != p.Default {
				return v
			}
		}
	default:
		// A number, a size, a duration or a text asks for its default below.
	}
	if p.Default != "" {
		return p.Default
	}
	return "1"
}

// dropping is a generator that throws one setting away before planning, which
// is what a format that forgot the refusal would do.
type dropping struct {
	format.Generator
	name string
}

func (g dropping) Plan(r format.Request) (format.Plan, error) {
	r.Properties = maps.Clone(r.Properties)
	delete(r.Properties, g.name)
	return g.Generator.Plan(r)
}

// TestARegistryRefusesAConditionItCannotKeep asks the check every format's
// declaration passes through, with one declaration for each way a condition
// can be one nothing could keep, and one it can.
func TestARegistryRefusesAConditionItCannotKeep(t *testing.T) {
	content := format.Property{Name: "content", Kind: format.PropertyChoice, Choices: []string{"placeholder", "test"}}
	on := func(setting string, values ...string) format.Property {
		return format.Property{Name: "background", Kind: format.PropertyText,
			OnlyWhen: format.Condition{Setting: setting, Values: values}}
	}
	chained := on("content", "placeholder")
	chained.Name, chained.Kind, chained.Choices = "guides", format.PropertyChoice, []string{"on", "off"}
	cases := []struct {
		name  string
		props []format.Property
	}{
		{"declared after", []format.Property{on("content", "placeholder"), content}},
		{"not declared", []format.Property{content, on("contents", "placeholder")}},
		{"not a choice", []format.Property{{Name: "content", Kind: format.PropertyText}, on("content", "placeholder")}},
		{"no values", []format.Property{content, on("content")}},
		{"every value", []format.Property{content, on("content", "placeholder", "test")}},
		{"a value it cannot have", []format.Property{content, on("content", "picture")}},
		{"a chain", []format.Property{content, chained, on("guides", "on")}},
	}
	for _, c := range cases {
		if format.ConditionProblem(c.props) == "" {
			t.Errorf("%s: the declaration was taken, and no window could keep its condition", c.name)
		}
	}
	// The canary: one it can keep is taken, or a check refusing everything
	// would pass every case above.
	if problem := format.ConditionProblem([]format.Property{content, on("content", "placeholder")}); problem != "" {
		t.Errorf("a condition on a choice declared before it was refused: %s", problem)
	}
}

// TestAPlaceholderSettingIsOnTheScreenExactlyWhileItApplies walks both
// screens through the content of a png and asks, after each step, which rows
// are on the screen - read off the declaration, not off a list of names - and
// what the run is told.
//
// One value in the background box all the way through, and a bad one: with
// the content on placeholder the run refuses it, which proves the refusal
// reaches the box, and with the content on the test pattern the same value
// has to reach nothing at all. Back on placeholder the box still holds it.
func TestAPlaceholderSettingIsOnTheScreenExactlyWhileItApplies(t *testing.T) {
	png, err := format.Get("png")
	if err != nil {
		t.Fatal(err)
	}
	heading := ""
	for _, p := range png.Properties {
		if p.Group != "" && !p.OnlyWhen.IsZero() {
			heading = text.SettingGroup(text.FormatOwner("png"), p.Group)
		}
	}
	if heading == "" {
		t.Fatal("png declares no block of settings with a condition, so this guard asks nothing")
	}

	single := window.NewGenerate(newFakeHost(t))
	several := window.NewRecipe(newFakeHost(t))
	for _, s := range []struct {
		name    string
		object  fyne.CanvasObject
		fields  *parts.Fields
		format  string
		at      func(string) string
		settled func()
	}{
		{"the single batch screen", single.Object(), single.Fields(), engine.SettingFormat,
			func(name string) string { return name }, single.Settled},
		{"the batch screen", several.Object(), several.Fields(), recipe.TargetAddress(1, recipe.KeyFormat),
			func(name string) string { return recipe.TargetAddress(1, core.KeyProperties+"."+name) }, several.Settled},
	} {
		chooserIn(t, s.fields, s.format).SetSelected("png")
		openFold(t, s.object, "", text.SettingsFor("png"))
		content := chooserIn(t, s.fields, s.at("content"))
		rows := func(step, value string) {
			t.Helper()
			for _, p := range png.Properties {
				want := p.Applies(png.Properties, func(name string) string {
					if name == "content" {
						return value
					}
					return ""
				})
				if got := s.fields.Lookup(s.at(p.Name)).Object().Visible(); got != want {
					t.Errorf("%s, %s: the row of %s is shown=%v, where %v was due", s.name, step, p.Name, got, want)
				}
			}
			shown := slices.Contains(strings.Split(shownText(s.object), "\n"), heading)
			if want := value == "placeholder"; shown != want {
				t.Errorf("%s, %s: the heading %q is shown=%v, where %v was due", s.name, step, heading, shown, want)
			}
		}
		preview := func() string {
			t.Helper()
			pressNamed(t, s.object, text.ButtonPreview())
			s.settled()
			return anyRefusal(s.object)
		}

		rows("the format just chosen", "test")
		content.SetSelected("placeholder")
		rows("the content on placeholder", "placeholder")
		setBox(t, s.fields, s.at("background"), "zzz")
		if saidBy(t, s.fields, s.at("background")) == "" && preview() == "" {
			t.Fatalf("%s: the background zzz on a placeholder was not refused, so nothing below can "+
				"tell a value sent from a value kept", s.name)
		}
		content.SetSelected("test")
		rows("the content back on the test pattern", "test")
		if said := preview(); said != "" {
			t.Errorf("%s: with the background box hidden the run was refused, so what it held was "+
				"sent:\n%s", s.name, said)
		}
		content.SetSelected("placeholder")
		rows("the content on placeholder again", "placeholder")
		if got := boxText(t, s.fields, s.at("background")); got != "zzz" {
			t.Errorf("%s: the background box came back holding %q - what was typed in it went with "+
				"the row", s.name, got)
		}
	}
}
