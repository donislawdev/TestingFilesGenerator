package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The preset screen, and the one question worth asking about it.
//
// A preset in a window is not "the same idea again" - it is the same run
// reached another way, and D1 says the two ways have to arrive at the same
// place. What makes that fragile here is concrete rather than theoretical:
// turning a parsed recipe into engine targets is written twice, once in
// internal/cli and once in internal/gui/window, because the two sit on the same
// layer and cannot share it. That exact conversion has drifted before. Measured
// 2026-08-04: validate and generate each had one, the one in validate had lost
// BoundaryLimit, and validate refused recipes generate accepted.
//
// So this does not compare the two lists of fields. It runs one preset from
// both surfaces and compares what lands on the disk.

// presetScreen opens the window and moves to the preset screen.
func presetScreen(t *testing.T) (*fakeHost, fyne.CanvasObject) {
	t.Helper()
	host, content := screen(t)
	press(t, content, "Presets")
	if host.content == nil {
		t.Fatal("pressing Presets put no screen in the window")
	}
	return host, host.content
}

// The window offers every preset this build registered.
func TestTheWindowOffersEveryPresetThereIs(t *testing.T) {
	_, content := presetScreen(t)

	control := controlUnder(content, "preset")
	picker, ok := control.(*widget.Select)
	if !ok {
		t.Fatalf("the preset field is %T rather than a list to choose from", control)
	}
	if strings.Join(picker.Options, ",") != strings.Join(preset.IDs(), ",") {
		t.Errorf("the window offers %v and the registry has %v", picker.Options, preset.IDs())
	}
	if picker.Selected == "" {
		t.Error("no preset is chosen when the screen opens")
	}
	t.Logf("%d preset(s) offered, all from the registry", len(picker.Options))
}

// A field per declared parameter, drawn by the same code that draws a format's
// settings - because a preset parameter is a format.Property. One declaration
// type, one set of controls, one wording for a refusal.
func TestTheWindowDrawsAFieldForEveryPresetParameter(t *testing.T) {
	_, content := presetScreen(t)
	picker := controlUnder(content, "preset").(*widget.Select)

	checked := 0
	for _, p := range preset.All() {
		picker.SetSelected(p.ID)

		// The question is what somebody chooses by. A list of ids says nothing
		// about which one answers what they came to ask, and the question is the
		// whole reason a preset is not just a recipe with a name.
		if shown := textIn(content); !strings.Contains(shown, p.Question) {
			t.Errorf("the screen does not show the question %s closes: %q", p.ID, p.Question)
		}

		for _, param := range p.Parameters {
			control := controlUnder(content, param.Name)
			if control == nil {
				t.Errorf("%s declares the parameter %q and the window draws no field for it", p.ID, param.Name)
				continue
			}
			if bad := wrongKindOfControl(param, control); bad != "" {
				t.Errorf("%s.%s is %s", p.ID, param.Name, bad)
			}
			if shown := textIn(content); !strings.Contains(shown, param.Allowed()) {
				t.Errorf("the field for %s.%s does not say what it takes (%q)",
					p.ID, param.Name, param.Allowed())
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no parameter was examined - this guard would pass without checking anything")
	}
	t.Logf("%d declared parameter(s), each with a field drawn from the declaration", checked)
}

// One preset, both surfaces, the same bytes and the same record.
//
// This is the guard the screen exists to earn. It compares three things that
// drift separately: the files, the recipe hash, and the preset block of the
// manifest - which says WHICH numbers were ours rather than only which preset
// this came from, and is untouchable rule 5 in the contract.
func TestThePresetScreenAndTheCommandLineProduceTheSameRun(t *testing.T) {
	fromCLI := t.TempDir()
	fromWindow := t.TempDir()

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"generate", "--preset", "size-boundaries", "--limit", "2mb", "--out", fromCLI,
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the command line refused the preset: exit %d\n%s", code, errOut.String())
	}

	host, content := presetScreen(t)
	fill(t, content, "output directory", fromWindow)
	fill(t, content, "limit", "2mb")
	press(t, content, "Generate")
	waitForManifest(t, fromWindow)
	join(host)

	cliNames, windowNames := namesIn(t, fromCLI), namesIn(t, fromWindow)
	if strings.Join(cliNames, " ") != strings.Join(windowNames, " ") {
		t.Fatalf("the two surfaces produced different files.\n  command line: %v\n  window:       %v",
			cliNames, windowNames)
	}

	// Byte for byte, because "the same set" and "the same files" are different
	// claims and only the second one is worth anything to somebody whose test
	// suite hashes them.
	for _, name := range cliNames {
		if name == "manifest.json" {
			continue
		}
		a, err := os.ReadFile(filepath.Join(fromCLI, name))
		if err != nil {
			t.Fatalf("reading %s from the command line run: %v", name, err)
		}
		b, err := os.ReadFile(filepath.Join(fromWindow, name))
		if err != nil {
			t.Fatalf("reading %s from the window run: %v", name, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between the surfaces: %d B from the command line, %d B from the window",
				name, len(a), len(b))
		}
	}

	cliManifest := manifestText(t, fromCLI)
	windowManifest := manifestText(t, fromWindow)

	for _, part := range []string{
		`"id": "size-boundaries"`,
		`"limit": "2mb"`,
		`"defaulted"`,
		`"spread"`,
	} {
		if !strings.Contains(windowManifest, part) {
			t.Errorf("the window's manifest is missing %s from its preset block", part)
		}
	}

	if a, b := presetBlock(cliManifest), presetBlock(windowManifest); a != b {
		t.Errorf("the preset blocks differ.\n  command line: %s\n  window:       %s", a, b)
	}
	if a, b := hashLine(cliManifest), hashLine(windowManifest); a != b {
		t.Errorf("the recipe hashes differ, so the two surfaces expanded the preset differently.\n"+
			"  command line: %s\n  window:       %s", a, b)
	}
	t.Logf("%d file(s) identical, one preset block, one recipe hash", len(cliNames)-1)
}

// A parameter the caller left alone is recorded as ours rather than theirs.
//
// The reverse case, and the one that carries the rule: a limit somebody stated
// must NOT be marked as invented, or the sentence loses its meaning by being
// said about everything.
func TestThePresetScreenSaysWhichNumbersWereOurs(t *testing.T) {
	dir := t.TempDir()
	host, content := presetScreen(t)

	fill(t, content, "output directory", dir)
	// limit left at its declared default, spread stated by hand.
	fill(t, content, "spread", "1kb")
	press(t, content, "Generate")
	waitForManifest(t, dir)
	join(host)

	recorded := manifestText(t, dir)
	if !strings.Contains(recorded, `"limit"`) {
		t.Error("the manifest does not record the limit the run used")
	}
	block := presetBlock(recorded)
	if !strings.Contains(block, "limit") || strings.Contains(defaultedIn(block), "spread") {
		t.Errorf("the run marked the wrong parameters as ours. Its preset block:\n%s", block)
	}
	if shown := textIn(content); !strings.Contains(shown, "placeholder") {
		t.Errorf("the window does not say out loud that the limit is a number we invented. It says:\n%s", shown)
	}
}

func manifestText(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest from %s: %v", dir, err)
	}
	return string(raw)
}

// presetBlock is the preset object of a manifest, as text. Compared as written
// rather than decoded, because that is what somebody's script reads.
func presetBlock(recorded string) string {
	i := strings.Index(recorded, `"preset"`)
	if i < 0 {
		return ""
	}
	depth := 0
	for j := i; j < len(recorded); j++ {
		switch recorded[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.Join(strings.Fields(recorded[i:j+1]), " ")
			}
		}
	}
	return ""
}

func defaultedIn(block string) string {
	i := strings.Index(block, `"defaulted"`)
	if i < 0 {
		return ""
	}
	end := strings.Index(block[i:], "]")
	if end < 0 {
		return block[i:]
	}
	return block[i : i+end]
}

func hashLine(recorded string) string {
	for _, line := range strings.Split(recorded, "\n") {
		if strings.Contains(line, "recipe_hash") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}
