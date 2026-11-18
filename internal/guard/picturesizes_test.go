package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The promise of the preset of picture sizes is in the name of every file:
// 1920x1080.jpg is a picture 1920 wide and 1080 high. A set whose names and
// pictures disagree would test somebody's system against sizes nobody asked
// for, and every one of its expectations would read as certain.
//
// So the set at its defaults is written and every file is read back by the
// standard library's own decoder - a reader this tool did not write - and its
// size compared with its name. The manifest is asked as well, because an
// expectation is half of what a preset hands over (PR4).
func TestEveryPictureOfTheSizesSetIsTheSizeItIsNamed(t *testing.T) {
	// The canary: a picture whose name says another size has to be told
	// apart, or the loop below could pass by comparing nothing with nothing.
	if misnamed := sizeDisagrees("100x100.jpg", 98, 98); misnamed == "" {
		t.Fatal("canary: a picture of 98 x 98 named 100x100 was not caught, so this guard cannot fail")
	}
	if fine := sizeDisagrees("98x98.jpg", 98, 98); fine != "" {
		t.Fatalf("canary: a picture of 98 x 98 named 98x98 was refused: %s", fine)
	}

	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"generate", "--preset", "picture-sizes", "--out", dir,
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the preset at its defaults was refused: exit %d\n%s", code, errOut.String())
	}

	pictures := 0
	for _, name := range namesIn(t, dir) {
		if isRecord(name) {
			continue
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("opening %s: %v", name, err)
		}
		cfg, kind, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			t.Errorf("%s does not read as a picture: %v", name, err)
			continue
		}
		if kind != "jpeg" {
			t.Errorf("%s reads as %s, and the set at its defaults is JPEG", name, kind)
		}
		if bad := sizeDisagrees(name, cfg.Width, cfg.Height); bad != "" {
			t.Error(bad)
		}
		pictures++
	}
	// The set has a size for every screen, avatar and platform it names, so a
	// run that wrote a handful would still pass the loop. Twenty-one files at
	// the defaults on 2026-11-17 - a floor below that rather than the number,
	// so a size added from a platform's page is not a red guard.
	const floor = 20
	if pictures < floor {
		t.Fatalf("the set at its defaults wrote %d pictures, and it holds at least %d", pictures, floor)
	}

	var m struct {
		Files []struct {
			Path     string `json:"path"`
			Purpose  string `json:"purpose"`
			Expected struct {
				Outcome string `json:"outcome"`
				Reason  string `json:"reason"`
			} `json:"expected"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(manifestText(t, dir)), &m); err != nil {
		t.Fatalf("the manifest does not read: %v", err)
	}
	if len(m.Files) != pictures {
		t.Fatalf("the manifest lists %d files and %d pictures were written", len(m.Files), pictures)
	}
	for _, f := range m.Files {
		if f.Expected.Outcome != "accept" || f.Expected.Reason != "dimensions_limit" {
			t.Errorf("%s expects %q with %q, and every picture of a common size expects accept with dimensions_limit",
				f.Path, f.Expected.Outcome, f.Expected.Reason)
		}
		w, h, _ := sizeInName(filepath.Base(f.Path))
		if !strings.Contains(f.Purpose, fmt.Sprintf("%d x %d", w, h)) {
			t.Errorf("%s says what it is for without saying its size: %q", f.Path, f.Purpose)
		}
	}
	t.Logf("%d pictures, each the size its name says, read by the standard library", pictures)
}

// sizeDisagrees is why a picture of w by h does not match its name, or
// nothing when it does.
func sizeDisagrees(name string, w, h int) string {
	nw, nh, ok := sizeInName(name)
	if !ok {
		return fmt.Sprintf("%s does not say its size in its name", name)
	}
	if nw != w || nh != h {
		return fmt.Sprintf("%s is a picture of %d x %d", name, w, h)
	}
	return ""
}

// sizeInName reads WIDTHxHEIGHT from the front of a file name.
func sizeInName(name string) (w, h int, ok bool) {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if _, err := fmt.Sscanf(stem, "%dx%d", &w, &h); err != nil {
		return 0, 0, false
	}
	return w, h, fmt.Sprintf("%dx%d", w, h) == stem
}

// A size the format cannot draw is left out of the set and said out loud,
// and a set left with nothing is refused - PR7, a set missing what it was
// about still looking like a set. Asked of the format rather than of a list:
// an icon is at most 256 pixels a side, which leaves three avatars.
func TestThePictureSizesSetSaysWhatTheFormatCannotDraw(t *testing.T) {
	icons, err := preset.Expand("picture-sizes", preset.Args{"format": "ico"})
	if err != nil {
		t.Fatalf("a set of icons was refused, and its smallest avatars fit an icon: %v", err)
	}
	notes := strings.Join(icons.Notes(), "\n")
	if !strings.Contains(notes, "1920x1080") || !strings.Contains(notes, "256") {
		t.Errorf("a set of icons left the large sizes out without saying which, or why. It says:\n%s", notes)
	}
	src := string(icons.Source)
	if strings.Contains(src, "1920x1080") {
		t.Error("a set of icons holds a size an icon cannot be")
	}
	if !strings.Contains(src, "98x98") {
		t.Errorf("a set of icons lost the avatar an icon can be:\n%s", src)
	}

	// Each refusal names the setting to change. An avif placeholder refused
	// as "no size fits" would be true and would send somebody to the groups.
	for _, refused := range []struct {
		args    preset.Args
		setting string
	}{
		{preset.Args{"format": "ico", "groups": "screens"}, "groups"},
		{preset.Args{"format": "pdf"}, "format"},
		{preset.Args{"format": "avif", "content": "placeholder"}, "content"},
	} {
		_, err := preset.Expand("picture-sizes", refused.args)
		var value *format.PropertyValueError
		if !errors.As(err, &value) {
			t.Errorf("%v was answered with %v, and a value the set cannot be built from is the caller's to change", refused.args, err)
			continue
		}
		if value.Key != refused.setting {
			t.Errorf("%v was refused about %s, and the value to change is %s: %v", refused.args, value.Key, refused.setting, err)
		}
	}

	// The default content says nothing about content, so a format with no
	// placeholder still builds the set.
	if _, err := preset.Expand("picture-sizes", preset.Args{"format": "avif", "groups": "avatars"}); err != nil {
		t.Errorf("a set of AVIF avatars with the test pattern was refused: %v", err)
	}
}

// One preset, both surfaces, the same bytes - with every parameter stated on
// the screen, because the defaults are the one value a box that drops what was
// typed into it cannot be told apart by.
func TestThePresetScreenBuildsThePictureSizesTheCommandLineBuilds(t *testing.T) {
	root := t.TempDir()
	fromCLI := filepath.Join(root, "cli")
	fromWindow := filepath.Join(root, "window")

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"generate", "--preset", "picture-sizes", "--groups", "avatars", "--content", "placeholder",
		"--format", "png", "--out", fromCLI,
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the command line refused the preset: exit %d\n%s", code, errOut.String())
	}

	host, content := presetScreen(t)
	choosePreset(t, content, "picture-sizes")
	fill(t, content, text.FieldOutputDir(), fromWindow)
	fill(t, content, text.SettingLabel("groups"), "avatars")
	choose(t, content, text.SettingLabel("content"), "placeholder")
	choose(t, content, text.SettingLabel("format"), "png")
	press(t, content, "Generate")
	waitForManifest(t, host, fromWindow)
	join(host)

	cliNames, windowNames := namesIn(t, fromCLI), namesIn(t, fromWindow)
	if strings.Join(cliNames, " ") != strings.Join(windowNames, " ") {
		t.Fatalf("the two surfaces produced different files.\n  command line: %v\n  window:       %v",
			cliNames, windowNames)
	}
	// Five avatars on 2026-11-17. Asserted rather than logged, because an
	// equality between two empty sets proves nothing.
	const wanted = 5
	if len(cliNames) != wanted+2 {
		t.Fatalf("the preset produced %d thing(s), %d pictures, a manifest and its instructions were expected: %v",
			len(cliNames), wanted, cliNames)
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
		if !isRecord(name) {
			compared++
		}
	}
	if compared != wanted {
		t.Fatalf("%d pictures were compared and %d were expected", compared, wanted)
	}
	if a, b := presetBlock(manifestText(t, fromCLI)), presetBlock(manifestText(t, fromWindow)); a != b {
		t.Errorf("the preset blocks differ.\n  command line: %s\n  window:       %s", a, b)
	}
	if !strings.Contains(manifestText(t, fromWindow), `"content": "placeholder"`) {
		t.Error("the window's manifest does not record the placeholder chosen on the screen")
	}
}
