package guard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	pictures "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder"
)

// What a placeholder promises somebody who sets it up (docs/PLACEHOLDER-2026-11-14.md):
// a setting it cannot draw is refused before any file exists, a setting that
// would change nothing is refused rather than ignored, the words of each file
// carry that file's numbers, and smallest is the smallest file there is. The
// pixels are pinned by placeholderglyphs_test.go, the bytes by the golden
// cases in generatorbytes_test.go.

// TestAPlaceholderSettingRefusesWhatItCannotDraw walks the settings one value
// at a time. Each refusal names the setting, so a window marks the right box.
//
// The canary is the accepted half of the table: a Read that refused every
// value would pass every refusal below, and it cannot pass those.
func TestAPlaceholderSettingRefusesWhatItCannotDraw(t *testing.T) {
	long := strings.Repeat("a", placeholder.MaxText)
	cases := []struct {
		props   map[string]string
		refused string // the setting refused, empty for accepted
	}{
		{map[string]string{}, ""},
		{map[string]string{"content": "placeholder", "background": "#ABC", "foreground": "123456", "guides": "true", "text": long}, ""},
		{map[string]string{"content": "test", "guides": "false"}, ""},
		{map[string]string{"content": "placeholder", "text": "{index}/{index:04} {width} \xc3\x97 {height} Zdj\xc4\x99cie \xd0\x96 \xce\xa9"}, ""},
		{map[string]string{"content": "test", "background": "cccccc"}, "background"},
		{map[string]string{"content": "test", "guides": "true"}, "guides"},
		{map[string]string{"text": "Hero"}, "text"},
		{map[string]string{"content": "Placeholder"}, "content"},
		{map[string]string{"content": "placeholder", "background": ""}, "background"},
		{map[string]string{"content": "placeholder", "foreground": "#12345"}, "foreground"},
		{map[string]string{"content": "placeholder", "background": "ggg"}, "background"},
		{map[string]string{"content": "placeholder", "text": "{idx}"}, "text"},
		{map[string]string{"content": "placeholder", "text": "open {width"}, "text"},
		{map[string]string{"content": "placeholder", "text": "a}b"}, "text"},
		{map[string]string{"content": "placeholder", "text": "\xe4\xb8\xad"}, "text"},
		{map[string]string{"content": "placeholder", "text": "tab\there"}, "text"},
		{map[string]string{"content": "placeholder", "text": long + "a"}, "text"},
		{map[string]string{"content": "placeholder", "guides": "yes"}, "guides"},
	}
	for _, c := range cases {
		_, err := placeholder.Read("png", c.props)
		var bad *format.PropertyValueError
		switch {
		case c.refused == "" && err != nil:
			t.Errorf("%v was refused and is a placeholder somebody can draw: %v", c.props, err)
		case c.refused != "" && !errors.As(err, &bad):
			t.Errorf("%v was not refused as a setting, it came back %v", c.props, err)
		case c.refused != "" && bad.Key != c.refused:
			t.Errorf("%v was refused about %s, and the setting at fault is %s", c.props, bad.Key, c.refused)
		}
	}
}

// TestEachPlaceholderSaysItsOwnNumber plans three files and reads the words
// each was given, as the manifest records them.
func TestEachPlaceholderSaysItsOwnNumber(t *testing.T) {
	for _, id := range []string{"png", "jpg", "svg"} {
		target := engine.Target{ID: "hero", Format: id, Sizes: engine.Uniform(3, 0), Smallest: true, Label: true,
			Properties: map[string]string{"content": "placeholder", "width": "320", "height": "180",
				"text": "{index}/{index:04} {width}x{height}", "background": "#ABC"}}
		planned, err := engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 1})
		if err != nil {
			t.Fatalf("%s: planning three placeholders: %v", id, err)
		}
		if len(planned) != 3 {
			t.Fatalf("%s: three files were asked for and %d planned", id, len(planned))
		}
		for i, f := range planned {
			want := fmt.Sprintf("%d/%04d 320x180", i+1, i+1)
			props := f.Plan.Properties
			if props["text"] != want || props["content"] != "placeholder" || props["background"] != "aabbcc" {
				t.Errorf("%s file %d says text %v, content %v, background %v - want %q, placeholder, aabbcc",
					id, i+1, props["text"], props["content"], props["background"], want)
			}
			if props["text_drawn"] != true || props[format.PropertyLabelEmbedded] != true {
				t.Errorf("%s file %d: a 320x180 placeholder has room for its words and its label, and says text_drawn %v, label %v",
					id, i+1, props["text_drawn"], props[format.PropertyLabelEmbedded])
			}
		}
	}
}

// TestSmallestIsTheSmallestFile asks every picture format for its smallest
// file at a size it can draw, with no label, and then for one byte less: the
// first has to be planned and the second refused, or smallest is a size
// somebody could have beaten by typing a number.
//
// The canary asks the same of a size a hundred bytes larger, which every
// format plans along with the byte below it - so the check has to call it
// not the smallest, or it is a check that cannot say no.
func TestSmallestIsTheSmallestFile(t *testing.T) {
	formats := pictures.Formats()
	if len(formats) < 10 {
		t.Fatalf("%d picture formats registered, %v - this would stop asking about the ones that left", len(formats), formats)
	}
	for _, id := range formats {
		size, err := smallestOf(t, id, map[string]string{"width": "48", "height": "32"})
		if err != nil {
			t.Errorf("%s: smallest at 48x32 was refused: %v", id, err)
			continue
		}
		if !isSmallest(t, id, size) {
			t.Errorf("%s: smallest planned %d B and %d B is planned too", id, size, size-1)
		}
		if isSmallest(t, id, size+100) {
			t.Errorf("%s: canary - %d B is a hundred over the smallest and was called the smallest, so this check cannot fail", id, size+100)
		}
	}
}

// smallestOf is the size the engine plans for one file of id at smallest.
func smallestOf(t *testing.T, id string, props map[string]string) (int64, error) {
	t.Helper()
	target := engine.Target{ID: "s", Format: id, Sizes: engine.Uniform(1, 0), Smallest: true, Properties: props}
	planned, err := engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 1})
	if err != nil {
		return 0, err
	}
	return planned[0].Plan.Bytes, nil
}

// isSmallest says whether size is planned for a 48x32 picture of id and the
// byte below it is not.
func isSmallest(t *testing.T, id string, size int64) bool {
	t.Helper()
	at := func(n int64) error {
		target := engine.Target{ID: "s", Format: id, Sizes: engine.Uniform(1, n),
			Properties: map[string]string{"width": "48", "height": "32"}}
		_, err := engine.Plan([]engine.Target{target}, engine.Options{OutDir: t.TempDir(), Seed: 1})
		return err
	}
	return at(size) == nil && at(size-1) != nil
}

// TestSmallestSaysWhyItCannotBeAnswered asks for it where there is no smallest
// picture: a format that is not a picture, a picture with no size, and a
// damage that would make the file some other size than the smallest.
func TestSmallestSaysWhyItCannotBeAnswered(t *testing.T) {
	cases := []struct {
		target engine.Target
		says   string
	}{
		{engine.Target{ID: "s", Format: "pdf", Sizes: engine.Uniform(1, 0), Smallest: true}, "smallest is for pictures"},
		{engine.Target{ID: "s", Format: "png", Sizes: engine.Uniform(1, 0), Smallest: true}, "does not say how large the picture is"},
		{engine.Target{ID: "s", Format: "png", Sizes: engine.Uniform(1, 0), Smallest: true,
			Properties: map[string]string{"width": "8", "height": "8"},
			Damage:     damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "50"}}}}, "asks for the smallest file"},
	}
	for _, c := range cases {
		_, err := engine.Plan([]engine.Target{c.target}, engine.Options{OutDir: t.TempDir(), Seed: 1})
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s smallest with %v: want a refusal saying %q, got %v", c.target.Format, c.target.Properties, c.says, err)
		}
	}
	// The canary: the same picture with a size is planned, so the refusals
	// above are about smallest and not about the picture.
	if _, err := smallestOf(t, "png", map[string]string{"width": "8", "height": "8"}); err != nil {
		t.Errorf("canary - an 8x8 png at smallest was refused: %v", err)
	}
}

// TestAColourAfterAHashInARecipeIsNotLostInSilence writes the recipe somebody
// writes first - background: #ff0000 - where YAML reads everything after the
// hash as a comment. The recipe has to be refused with the reason, not run
// with a grey picture.
func TestAColourAfterAHashInARecipeIsNotLostInSilence(t *testing.T) {
	dir := t.TempDir()
	recipe := func(colour string) string {
		path := filepath.Join(dir, "r"+colour[len(colour)-1:]+".yaml")
		body := "version: 1\nseed: 1\ntargets:\n  - id: hero\n    format: png\n    size: smallest\n" +
			"    properties:\n      content: placeholder\n      width: 64\n      height: 32\n      background: " + colour + "\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	code, _, errOut := run(t, "generate", recipe("#ff0000"), "--dry-run", "--out", filepath.Join(dir, "a"))
	if code == cli.ExitOK || !strings.Contains(errOut, "background") || !strings.Contains(errOut, "# starts a comment") {
		t.Errorf("background: #ff0000 came back with exit %d and %q - it has to be refused with the reason", code, errOut)
	}
	// The canary: the same colour without the hash is a recipe that runs.
	if code, _, errOut := run(t, "generate", recipe("ff0000"), "--dry-run", "--out", filepath.Join(dir, "b")); code != cli.ExitOK {
		t.Errorf("canary - background: ff0000 was refused with exit %d: %s", code, errOut)
	}
}
