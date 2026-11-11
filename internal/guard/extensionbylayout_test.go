package guard

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/otlp"
)

// An otlp file in a layout of one request is one JSON document, and until
// 2026-11-11 its name ended in .jsonl whatever the layout - which tells a reader
// that picks its parser by the name to read the file line by line. A setting
// now declares the endings its values give (format.Property.Extensions), and
// the engine names every file by Descriptor.ExtensionFor.
// docs/ROZSZERZENIE-UKLADU-2026-11-11.md.

// TestEveryEndingASettingGivesIsDeclaredSoItCanBeTrusted holds the declarations
// to what ExtensionFor, tfg formats and the site lean on.
func TestEveryEndingASettingGivesIsDeclaredSoItCanBeTrusted(t *testing.T) {
	declaring := 0
	for _, d := range format.All() {
		settings := 0
		for _, p := range d.Properties {
			if len(p.Extensions) == 0 {
				continue
			}
			settings++
			declaring++
			endingsAreDeclaredWell(t, d, p)
		}
		if settings > 1 {
			t.Errorf("%s: %d settings change what a file name ends in, and ExtensionFor takes the first that matches - "+
				"one setting may, so the name never depends on the order the settings are declared in", d.ID, settings)
		}
	}
	// otlp's formatting gives one. Without this a rule taken out leaves the
	// loop above with nothing to look at, green.
	if declaring == 0 {
		t.Fatal("no setting of any format changes what a file name ends in, so this guard checked nothing - otlp's formatting did")
	}
}

func endingsAreDeclaredWell(t *testing.T, d format.Descriptor, p format.Property) {
	t.Helper()
	if p.Kind != format.PropertyChoice {
		t.Errorf("%s %s: only a choice can give a file name its ending, and this is a %s", d.ID, p.Name, p.Kind)
	}
	for value, ext := range p.Extensions {
		switch {
		case !slices.Contains(p.Choices, value):
			t.Errorf("%s %s: %q gives %s and is not one of the values it takes, %v", d.ID, p.Name, value, ext, p.Choices)
		case value == p.Default:
			t.Errorf("%s %s: the default %q is listed with %s - the default's ending is Extension, and a second place for it is one that goes stale",
				d.ID, p.Name, value, ext)
		case !strings.HasPrefix(ext, "."):
			t.Errorf("%s %s: %q gives %q, which does not start with a dot", d.ID, p.Name, value, ext)
		case ext == d.Extension:
			t.Errorf("%s %s: %q gives %s, the format's own ending, so the entry changes nothing", d.ID, p.Name, value, ext)
		}
	}
	// The sentence under the setting is where a window says it, and it is
	// written by hand beside the map - so it is held to the map. Matched as a
	// whole ending, because .json is also the first five letters of .jsonl.
	for _, ext := range d.Extensions() {
		if !regexp.MustCompile(regexp.QuoteMeta(ext) + `\b`).MatchString(p.Detail) {
			t.Errorf("%s %s: a file can end in %s, and the sentence under the setting never says so: %q", d.ID, p.Name, ext, p.Detail)
		}
	}
}

// TestEveryFileIsNamedForTheSettingsItWasMadeWith plans files the way a run
// plans them and reads the names the engine gave.
//
// otlp is asked first with its endings written out. The loop after it compares
// with ExtensionFor, so it is what catches an engine that stops asking - and
// only the written endings catch an ExtensionFor that stops answering.
func TestEveryFileIsNamedForTheSettingsItWasMadeWith(t *testing.T) {
	for layout, want := range map[string]string{
		"":                 ".jsonl",
		otlp.RecordPerLine: ".jsonl",
		otlp.Minified:      ".json",
		otlp.Indented:      ".json",
	} {
		props := map[string]string{}
		if layout != "" {
			props[otlp.Formatting] = layout
		}
		for _, name := range plannedNames(t, "otlp", props, "") {
			if got := filepath.Ext(name); got != want {
				t.Errorf("otlp with formatting %q: %s ends in %s, and a file of this layout ends in %s", layout, name, got, want)
			}
		}
	}

	// A template somebody wrote wins, ending and all - a test of a form that
	// reads only the ending asks for exactly that.
	for _, name := range plannedNames(t, "otlp", map[string]string{otlp.Formatting: otlp.Indented}, "mine_{index:04}.jsonl") {
		if !strings.HasPrefix(name, "mine_") || filepath.Ext(name) != ".jsonl" {
			t.Errorf("a template asked for mine_{index:04}.jsonl and the file was named %s", name)
		}
	}

	checked := 0
	for _, d := range format.All() {
		for _, p := range d.Properties {
			if len(p.Extensions) == 0 {
				continue
			}
			for _, value := range append([]string{""}, p.Choices...) {
				props := map[string]string{}
				if value != "" {
					props[p.Name] = value
				}
				want := d.ExtensionFor(props)
				for _, name := range plannedNames(t, d.ID, props, "") {
					if !strings.HasSuffix(name, want) {
						t.Errorf("%s with %s %q: %s does not end in %s", d.ID, p.Name, value, name, want)
					}
					checked++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no file was named under a setting that gives an ending, so the loop proved nothing")
	}
}

// plannedNames plans two files of format id and, with no template, a boundary
// set beside them - the two ways the engine names a file itself - and gives
// back every name.
func plannedNames(t *testing.T, id string, props map[string]string, tmpl string) []string {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	size := d.SmallestAccepted(format.Request{Label: true, Properties: props})
	targets := []engine.Target{{ID: "files", Format: id, Sizes: []int64{size, size}, Label: true, Properties: props, NameTmpl: tmpl}}
	if tmpl == "" {
		targets = append(targets, engine.Target{ID: "edge", Format: id, Sizes: []int64{size, size + 1, size + 2},
			BoundaryLimit: size + 1, Label: true, Properties: props})
	}
	planned, err := engine.Plan(targets, engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test"})
	if err != nil {
		t.Fatalf("%s with %v: %v", id, props, err)
	}
	names := make([]string, 0, len(planned))
	for _, f := range planned {
		names = append(names, f.Name)
	}
	if want := 2 + 3*min(1, len(targets)-1); len(names) != want {
		t.Fatalf("%s with %v: %d files were planned and %d were asked for", id, props, len(names), want)
	}
	return names
}
