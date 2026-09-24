package guard

import (
	"bytes"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// A flag a preset reads stands in with one value, whichever surface asks.
//
// Until 2026-09-24 the window took the value from preset.Global, which knew
// one default for --format - pdf, the one size-boundaries uses - while each
// preset's Expand applied its own. The second preset to read --format was the
// preset of unusual file names, made of text files by default, so the command
// line would have written text files and the window PDFs from one preset (D1).
// The default is declared by the preset now, and this asks both surfaces for
// it: the set a run makes with the flag left out, and the value the window's
// menu opens on.
func TestEveryFlagAPresetReadsDefaultsToOneValueOnBothSurfaces(t *testing.T) {
	_, content := presetScreen(t)

	checked := 0
	for _, p := range preset.All() {
		for _, name := range p.Reads {
			declared := p.ReadDefaults[name]

			bare, err := preset.Expand(p.ID, preset.Args{})
			if err != nil {
				t.Fatalf("%s refused its own defaults: %v", p.ID, err)
			}
			stated, err := preset.Expand(p.ID, preset.Args{name: declared})
			if err != nil {
				t.Fatalf("%s refused --%s %s: %v", p.ID, name, declared, err)
			}
			if !bytes.Equal(bare.Source, stated.Source) {
				t.Errorf("%s declares --%s %s, and leaving the flag out makes a different set", p.ID, name, declared)
			}
			// The comparison above says nothing if the flag changes nothing, so
			// another value has to make another set.
			if !anotherValueChangesTheSet(t, p, name, declared, bare.Source) {
				t.Errorf("%s makes the same set whatever --%s says, so this guard cannot tell the defaults apart", p.ID, name)
			}

			choosePreset(t, content, p.ID)
			menu, ok := controlUnder(content, text.SettingLabel(name)).(*parts.Chooser)
			if !ok {
				t.Errorf("the window draws no menu for --%s of %s", name, p.ID)
				continue
			}
			if menu.Selected != declared {
				t.Errorf("the window opens --%s of %s on %q, and the command line uses %q", name, p.ID, menu.Selected, declared)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no preset reads a flag, so this guard checked nothing")
	}
	t.Logf("%d flag(s) read by a preset, each defaulting to one value on both surfaces", checked)
}

// anotherValueChangesTheSet is whether some other format makes a different
// set - the first one the preset accepts.
func anotherValueChangesTheSet(t *testing.T, p preset.Preset, name, declared string, bare []byte) bool {
	t.Helper()
	for _, id := range format.IDs() {
		if id == declared {
			continue
		}
		other, err := preset.Expand(p.ID, preset.Args{name: id})
		if err != nil {
			continue
		}
		return !bytes.Equal(other.Source, bare)
	}
	return false
}
