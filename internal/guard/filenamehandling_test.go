package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The preset of unusual file names asks for the fifty names of
// docs/NAMES-PRESET-2026-09-24.md, and each of them in every format.
//
// A name here is the whole point of its file, and a name that arrives a
// character short is a file that tests something else while its manifest
// entry still says what it was meant to test. That happened before this
// preset existed: an ideographic space at the front of a name was trimmed on
// its way through the recipe, in every format (O243). So each name is asked
// after the recipe has been written and read back, which is the name a run
// actually gets - and each is asked by what makes it the name it is, worked out
// here rather than copied from the preset.
func TestTheFileNamePresetAsksForEveryNameInEveryFormat(t *testing.T) {
	checked := 0
	for _, id := range format.IDs() {
		desc, _ := format.Get(id)
		names := namesOfTheSet(t, preset.Args{"format": id})
		if len(names) != 50 {
			t.Errorf("%s: the set holds %d names and the list has 50", id, len(names))
		}
		folded := map[string]string{}
		for target, name := range names {
			if !utf8.ValidString(name) || name == "" || len(name) > core.MaxNameBytes {
				t.Errorf("%s: %s is %d bytes, or empty, or not UTF-8: %+q", id, target, len(name), name)
			}
			if other, taken := folded[core.FoldName(name)]; taken {
				t.Errorf("%s: %s and %s are one file on a system that folds names", id, target, other)
			}
			folded[core.FoldName(name)] = target
			if fault := nameFault(target, name, desc.Extension); fault != "" {
				t.Errorf("%s: %s is %+q, and %s", id, target, name, fault)
			}
			checked++
		}
	}
	if checked < 50*20 {
		t.Fatalf("only %d names were checked", checked)
	}
	t.Logf("%d names in %d formats", checked, len(format.IDs()))
}

// namesOfTheSet expands the preset, reads the recipe back and gives each
// target's name by its id.
func namesOfTheSet(t *testing.T, args preset.Args) map[string]string {
	t.Helper()
	expanded, err := preset.Expand("filename-handling", args)
	if err != nil {
		t.Fatalf("the preset refused %v: %v", args, err)
	}
	rec, err := recipe.Parse(expanded.Source, "filename-handling")
	if err != nil {
		t.Fatalf("the preset's recipe at %v was refused: %v", args, err)
	}
	out := map[string]string{}
	for _, target := range rec.Targets {
		out[target.ID] = target.Name
	}
	return out
}

// nameFault says what is wrong with a name of the set, or "" when nothing is.
func nameFault(target, name, ext string) string {
	exact := map[string]string{
		"htaccess": ".htaccess", "web_config": "web.config", "dotenv": ".env",
		"ds_store": ".DS_Store", "desktop_ini": "desktop.ini", "no_extension": "README",
	}
	marked := map[string]rune{
		"bidi_override": 0x202E, "zero_width": 0x200B, "no_break_space": 0xA0,
		"homoglyph": 0x430, "line_separator": 0x2028, "emoji_zwj": 0x200D,
	}
	stem := strings.TrimSuffix(name, ext)
	switch {
	case exact[target] != "":
		if name != exact[target] {
			return "it means something only as " + exact[target]
		}
		return ""
	case target == "upper_extension":
		if name != "REPORT"+strings.ToUpper(ext) {
			return "its extension is not the format's in capitals"
		}
		return ""
	case target == "fullwidth_extension":
		if name == "report"+ext || norm.NFKC.String(name) != "report"+ext {
			return "it is not the format's extension in full width letters"
		}
		return ""
	case !strings.HasSuffix(name, ext):
		return "it does not end with the format's extension " + ext
	}
	switch target {
	case "only_extension":
		return unless(stem == "", "it is more than the extension")
	case "ustar_101":
		return unless(len(name) == 101 && strings.Trim(stem, "u") == "", "it is not 101 bytes of u with the extension")
	case "max_ascii":
		return unless(len(name) == 255 && strings.Trim(stem, "a") == "", "it is not 255 bytes of a with the extension")
	case "cjk_bytes":
		return unless(strings.Trim(stem, "日") == "" && len(name) <= 255 && len(name)+3 > 255, "it is not as many ideographs as fit in 255 bytes")
	case "emoji_bytes":
		return unless(strings.Trim(stem, "🎉") == "" && len(name) <= 255 && len(name)+4 > 255, "it is not as many emoji as fit in 255 bytes")
	case "nfd", "hangul_nfd":
		return unless(norm.NFD.IsNormalString(name) && !norm.NFC.IsNormalString(name), "it is not in the decomposed form")
	case "leading_bom":
		return unless(strings.HasPrefix(name, string(rune(0xFEFF))), "it does not begin with a byte order mark")
	case "leading_space":
		return unless(strings.HasPrefix(name, " ") && !strings.HasPrefix(name, "  "), "it does not begin with one plain space")
	case "double_space":
		return unless(strings.Contains(stem, "  "), "it does not hold two spaces in a row")
	case "leading_dot":
		return unless(strings.HasPrefix(name, ".") && !strings.HasPrefix(name, ".."), "it does not begin with one dot")
	case "leading_double_dot":
		return unless(strings.HasPrefix(name, ".."), "it does not begin with two dots")
	case "leading_dash":
		return unless(strings.HasPrefix(name, "-"), "it does not begin with a dash")
	case "leading_ideographic_space":
		return unless(strings.HasPrefix(name, string(rune(0x3000))), "it does not begin with an ideographic space")
	case "unicode_tags":
		return unless(untagged(stem) == "hidden note", "it carries no hidden note in tag characters")
	}
	if r, ok := marked[target]; ok && !strings.ContainsRune(name, r) {
		return "it lacks the character it is about"
	}
	return ""
}

func unless(ok bool, fault string) string {
	if ok {
		return ""
	}
	return fault
}

// untagged is the text the tag characters of s spell.
func untagged(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0xE0020 && r <= 0xE007E {
			b.WriteRune(r - 0xE0000)
		}
	}
	return b.String()
}

// The set promises acceptance only where refusing would be the system's
// fault, and says unspecified everywhere else (MF5), with a reason the
// manifest already has - the owner's decision of 2026-09-24.
func TestTheFileNamePresetPromisesOnlyWhatASystemMustDo(t *testing.T) {
	expanded, err := preset.Expand("filename-handling", preset.Args{})
	if err != nil {
		t.Fatal(err)
	}
	rec, err := recipe.Parse(expanded.Source, "filename-handling")
	if err != nil {
		t.Fatal(err)
	}
	var accepted []string
	for _, target := range rec.Targets {
		switch target.Expected {
		case "accept":
			accepted = append(accepted, target.ID)
		case "unspecified":
			switch target.ExpectedReason {
			case "filename_invalid", "filename_too_long", "filename_traversal":
			default:
				t.Errorf("%s is unspecified for %q, which is not one of the three name reasons", target.ID, target.ExpectedReason)
			}
		default:
			t.Errorf("%s expects %q, and a name is either accepted or left to the system's policy", target.ID, target.Expected)
		}
	}
	sort.Strings(accepted)
	if got := strings.Join(accepted, " "); got != "leading_zeros many_dots null_word upper_extension" {
		t.Errorf("the set promises acceptance for %q", got)
	}
}

// Every name is written as it was asked for, here - and CI runs this on
// Windows, Linux and macOS, which is the measurement of
// docs/NAMES-PRESET-2026-09-24.md section 4 kept.
//
// Asked in the default format and in one whose extension is a byte longer,
// because the names that are about length are made to a length with the
// extension, and only the default was ever measured by hand.
func TestTheFileNamePresetWritesEveryNameByteForByte(t *testing.T) {
	for _, args := range [][]string{nil, {"--format", "docx"}} {
		dir := t.TempDir()
		var out, errOut bytes.Buffer
		cmd := append([]string{"generate", "--preset", "filename-handling", "--out", dir}, args...)
		if code := cli.Run(context.Background(), cmd, &out, &errOut); code != cli.ExitOK {
			t.Fatalf("%v ended %d:\n%s", cmd, code, errOut.String())
		}
		m, err := manifest.Load(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var recorded []string
		for _, f := range m.Files {
			if !f.Materialized {
				t.Errorf("%v: %+q was not written", args, f.Name)
			}
			recorded = append(recorded, f.Name)
		}
		sort.Strings(recorded)
		var onDisk []string
		for _, name := range namesIn(t, dir) {
			if !isRecord(name) {
				onDisk = append(onDisk, name)
			}
		}
		if len(recorded) != 50 || strings.Join(onDisk, "\x00") != strings.Join(recorded, "\x00") {
			t.Errorf("%v: the directory holds %d names and the manifest %d, and they are not the same bytes:\n  disk     %+q\n  manifest %+q",
				args, len(onDisk), len(recorded), onDisk, recorded)
		}
		if code := cli.Run(context.Background(), []string{"verify", filepath.Join(dir, "manifest.json")}, &out, &errOut); code != cli.ExitOK {
			t.Errorf("%v: verify ended %d:\n%s", args, code, errOut.String())
		}
	}
}

// One preset, both surfaces, the same files - the names are the point, so the
// window has to write the same fifty.
func TestThePresetScreenWritesTheNamesTheCommandLineWrites(t *testing.T) {
	fromCLI, fromWindow := t.TempDir(), t.TempDir()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"generate", "--preset", "filename-handling", "--out", fromCLI}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the command line refused the preset: exit %d\n%s", code, errOut.String())
	}

	host, content := presetScreen(t)
	choosePreset(t, content, "filename-handling")
	fill(t, content, text.FieldOutputDir(), fromWindow)
	press(t, content, "Generate")
	waitForManifest(t, host, fromWindow)
	join(host)

	cliNames, windowNames := namesIn(t, fromCLI), namesIn(t, fromWindow)
	if len(cliNames) != 52 {
		t.Fatalf("the command line wrote %d things and fifty files, a manifest and its instructions were expected", len(cliNames))
	}
	if strings.Join(cliNames, "\x00") != strings.Join(windowNames, "\x00") {
		t.Fatalf("the two surfaces wrote different names:\n  command line %+q\n  window       %+q", cliNames, windowNames)
	}
	for _, name := range cliNames {
		if name == "manifest.json" {
			continue
		}
		a, errA := os.ReadFile(filepath.Join(fromCLI, name))
		b, errB := os.ReadFile(filepath.Join(fromWindow, name))
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			t.Errorf("%+q differs between the surfaces (%v, %v)", name, errA, errB)
		}
	}
}
