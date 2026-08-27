package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A preset can be built out of any format, from the window as well as from the
// command line.
//
// Reported from the screen on 2026-08-12 as "why can the preset screen only
// make PDFs", and it was exactly that. size-boundaries declares Reads: format,
// which is how a preset supplies a value for a global flag rather than
// declaring a parameter that would collide with it - CLI.md section 6 - so
// "tfg generate --preset size-boundaries --format png" has always worked and
// the window had no control for it at all. Measured that day: seven PNGs from
// the command line, seven PDFs from the window, whatever anybody did.
//
// The parity guard could not see this and still cannot. It counts the
// capabilities of the engine, and a preset supplying a default for --format
// adds none - the format is on that list already. What was missing is the pair
// "this preset, in that format", which is a capability of a SCREEN. So this
// guard is the one that watches it, and it watches by running the thing.
func TestThePresetScreenCanBuildTheSetInAnyFormat(t *testing.T) {
	dir := t.TempDir()

	host, content := presetScreen(t)
	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.SettingLabel("limit"), "2mb")
	choose(t, content, text.SettingLabel("format"), "png")
	press(t, content, "Generate")
	waitForManifest(t, host, dir)
	join(host)

	written := namesIn(t, dir)
	pictures := 0
	for _, name := range written {
		if name == "manifest.json" {
			continue
		}
		if !strings.HasSuffix(name, ".png") {
			t.Errorf("the set was asked for in png and %s came out", name)
			continue
		}
		// The name is not the file. A window that put the extension on and
		// generated a PDF anyway would pass every check made on the listing,
		// and this is a tool whose whole thesis is that the bytes are right.
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("%s is named as a picture and does not begin like one", name)
			continue
		}
		pictures++
	}
	if pictures == 0 {
		t.Fatal("the run produced no files, so this guard compared nothing")
	}
	t.Logf("%d file(s) built by the preset in a format chosen on the screen", pictures)
}

// The same set, in the same format, from both surfaces, byte for byte.
//
// The guard above says the window can ask for another format. This says the
// answer is the same one the command line gives, which is a different claim and
// the one D1 is about: the window reaching the setting through a different path
// is exactly how two surfaces come to produce two things that look alike.
func TestChoosingTheFormatGivesTheSameSetOnBothSurfaces(t *testing.T) {
	fromCLI, fromWindow := t.TempDir(), t.TempDir()

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"generate", "--preset", "size-boundaries", "--limit", "2mb", "--format", "png", "--out", fromCLI,
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the command line refused the preset in png: exit %d\n%s", code, errOut.String())
	}

	host, content := presetScreen(t)
	fill(t, content, text.FieldOutputDir(), fromWindow)
	fill(t, content, text.SettingLabel("limit"), "2mb")
	choose(t, content, text.SettingLabel("format"), "png")
	press(t, content, "Generate")
	waitForManifest(t, host, fromWindow)
	join(host)

	cliNames, windowNames := namesIn(t, fromCLI), namesIn(t, fromWindow)
	if strings.Join(cliNames, " ") != strings.Join(windowNames, " ") {
		t.Fatalf("the two surfaces produced different files.\n  command line: %v\n  window:       %v",
			cliNames, windowNames)
	}
	if len(cliNames) != 8 {
		t.Fatalf("the preset produced %d thing(s) and seven files plus a manifest was expected: %v",
			len(cliNames), cliNames)
	}

	compared := 0
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
		compared++
	}
	if compared == 0 {
		t.Fatal("nothing was compared, so this guard would pass on two empty directories")
	}

	// And the record says the format was ours to choose rather than the
	// preset's to assume. Untouchable rule 5 is about telling those apart.
	if manifest := manifestText(t, fromWindow); !strings.Contains(manifest, `"format": "png"`) {
		t.Error("the manifest does not record that the format was given, so the run reads as though the preset chose it")
	}
}

// The preview says what kind of files it would write.
//
// Asked for on 2026-08-12. On the generate screen the answer is in a menu three
// inches up, and on the preset screen it is nowhere: a preset states its own
// targets, so before this the window could tell somebody it was about to write
// seven files and 70 MiB without saying what they were. G6 makes the preview
// the thing somebody presses instead of finding out by writing gigabytes.
func TestThePreviewSaysWhatKindOfFilesItWouldWrite(t *testing.T) {
	generateHost, generate := screen(t)
	choose(t, generate, text.FieldFormat(), "png")
	fill(t, generate, text.FieldOutputDir(), t.TempDir())
	press(t, generate, "Preview")
	join(generateHost)
	if shown := allText(generate); !strings.Contains(shown, "png") {
		t.Errorf("the preview does not say what it would write. It says:\n%s", shown)
	}

	presetHost, presets := presetScreen(t)
	fill(t, presets, text.FieldOutputDir(), t.TempDir())
	choose(t, presets, text.SettingLabel("format"), "wav")
	press(t, presets, "Preview")
	join(presetHost)
	shown := allText(presets)
	if !strings.Contains(shown, "wav") {
		t.Errorf("the preset preview does not say what it would write. It says:\n%s", shown)
	}
	// Not merely the word somewhere on a screen that has a menu holding it.
	// The line under the buttons is the one somebody reads after pressing.
	if !strings.Contains(shown, "7 files · wav") {
		t.Errorf("the preview line does not name the kind beside the count. It says:\n%s", shown)
	}
}

// choose picks a value from a labelled menu, the way somebody does.
func choose(t *testing.T, o fyne.CanvasObject, label, value string) {
	t.Helper()
	control := controlUnder(o, label)
	menu, ok := control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the field %q is %T rather than a menu to choose from", label, control)
	}
	menu.SetSelected(value)
	if menu.Selected != value {
		t.Fatalf("choosing %q from %q left it showing %q, so the value is not one of its options",
			value, label, menu.Selected)
	}
}
