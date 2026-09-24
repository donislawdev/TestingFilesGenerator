package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// Long file names, O239 (docs/O239-LONG-NAMES-2026-09-24.md).
//
// Every file is written under a temporary name beside it and renamed once it
// is whole. Until 2026-09-24 that name was the file's name with eighteen bytes
// after it, so a name from 238 bytes up was never written on any system, while
// every file system took the name itself. And a name over 255 bytes was
// written on Windows and macOS and could not be on Linux - one recipe, two
// answers. Every guard of the write path used short names, so neither had a
// guard at all.

// TestANameAsLongAsEverySystemStoresIsWritten writes a name of 255 bytes and
// one of 253 bytes in 87 characters, and asks verify about both.
func TestANameAsLongAsEverySystemStoresIsWritten(t *testing.T) {
	suffix := fmt.Sprintf("%s%d", core.PartialMarker, os.Getpid())
	for _, name := range []string{
		strings.Repeat("a", core.MaxNameBytes-len(".txt")) + ".txt",
		strings.Repeat("日", 83) + ".txt",
	} {
		// The case in question: the name fits, and its temporary name joined
		// the plain way would not.
		if len(name) > core.MaxNameBytes || len(name)+len(suffix) <= core.MaxNameBytes {
			t.Fatalf("a name of %d bytes is not the case this guard asks about", len(name))
		}
		dir := t.TempDir()
		code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1", "--name", name, "--out", dir)
		if code != cli.ExitOK {
			t.Errorf("a name of %d bytes, which every system stores, ended with exit %d:\n%s", len(name), code, errOut)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("a name of %d bytes is not on the disk: %v", len(name), err)
		}
		if code, _, errOut := run(t, "verify", filepath.Join(dir, engine.DefaultManifestName)); code != cli.ExitOK {
			t.Errorf("verify on a run with a name of %d bytes gave exit %d:\n%s", len(name), code, errOut)
		}
	}
}

// TestTwoLongNamesThatBeginAlikeGetTwoSiblings asks the one place a name beside
// a file is made. A short name keeps the plain join, so what an ordinary run
// leaves behind looks the same as ever. Two long names sharing their first 240
// bytes - a numbered template at the end of a long name - get two different
// siblings, each short enough, each whole characters, each ending in the
// marker that says what it is.
func TestTwoLongNamesThatBeginAlikeGetTwoSiblings(t *testing.T) {
	// The longest process id a system hands out, a Windows DWORD.
	suffix := core.PartialMarker + "4294967295"
	if short := "report.txt"; core.SiblingName(short, suffix) != short+suffix {
		t.Errorf("a short name's sibling is %q rather than the plain join, so every leftover of an ordinary run "+
			"changed its shape", core.SiblingName(short, suffix))
	}

	// Three bytes to a character, so a cut made by the byte lands inside one.
	begin := strings.Repeat("日", 80)
	seen := map[string]string{}
	for _, name := range []string{begin + "_0001.txt", begin + "_0002.txt"} {
		if len(name)+len(suffix) <= core.MaxNameBytes {
			t.Fatalf("a name of %d bytes is short enough to keep the plain join, so nothing is asked", len(name))
		}
		got := core.SiblingName(name, suffix)
		if len(got) > core.MaxNameBytes {
			t.Errorf("the sibling of a name of %d bytes is %d bytes, longer than every system stores", len(name), len(got))
		}
		if !utf8.ValidString(got) {
			t.Errorf("the sibling %q was cut part way through a character", got)
		}
		if !strings.HasSuffix(got, suffix) || !core.IsPartialName(got) {
			t.Errorf("the sibling %q does not end in the marker, so a leftover would not be known for what it is", got)
		}
		if other, taken := seen[got]; taken {
			t.Errorf("%q and %q share the sibling %q, so two files of one run meet on it", other, name, got)
		}
		seen[got] = name
	}
}

// TestALeftoverUnderAShortenedNameIsNamedForWhatItIs puts both kinds of
// leftover, in the shortened form, into the output of a run - a file being
// produced and a record being replaced - and asks verify what they are.
func TestALeftoverUnderAShortenedNameIsNamedForWhatItIs(t *testing.T) {
	long := strings.Repeat("a", 250) + ".txt"
	for _, marker := range []string{core.PartialMarker + "99999", core.WritingMarker} {
		out, mf := generated(t)
		name := core.SiblingName(long, marker)
		if name == long+marker {
			t.Fatalf("the leftover %q is in the plain form, so the shortened one is not asked about", name)
		}
		if err := os.WriteFile(filepath.Join(out, name), []byte("half a file\n"), 0o644); err != nil {
			t.Fatalf("writing the leftover: %v", err)
		}
		code, _, errOut := run(t, "verify", mf)
		if code != cli.ExitVerify {
			t.Errorf("exit %d, expected %d - a leftover is still a difference:\n%s", code, cli.ExitVerify, errOut)
		}
		// The kind in the report's first column, the same word for both
		// markers. The sentence under it differs between them.
		if strings.Contains(errOut, "extra     "+name) || !strings.Contains(errOut, "leftover  "+name) {
			t.Errorf("a leftover in the shortened form, %q, is not reported as a leftover:\n%s", name, errOut)
		}
	}
}

// TestANameLongerThanEverySystemStoresIsRefusedBeforeAnythingIsWritten asks for
// a name one byte over, and one of 86 characters that is 262 bytes, and then
// for one exactly at the line.
func TestANameLongerThanEverySystemStoresIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, name := range []string{
		strings.Repeat("a", core.MaxNameBytes+1-len(".txt")) + ".txt",
		strings.Repeat("日", 86) + ".txt",
	} {
		if len(name) <= core.MaxNameBytes {
			t.Fatalf("a name of %d bytes is not over the line", len(name))
		}
		dir := t.TempDir()
		code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1", "--name", name, "--out", dir)
		if code != cli.ExitRecipe {
			t.Errorf("a name of %d bytes ended with exit %d rather than a refusal of the recipe:\n%s", len(name), code, errOut)
		}
		if !strings.Contains(errOut, fmt.Sprintf("%d bytes", len(name))) || !strings.Contains(errOut, fmt.Sprint(core.MaxNameBytes)) {
			t.Errorf("the refusal does not say how long the name is and what the limit is:\n%s", errOut)
		}
		if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
			t.Errorf("a refused name left %d entries in the output directory (%v)", len(entries), err)
		}
	}

	atTheLine := strings.Repeat("a", core.MaxNameBytes-len(".txt")) + ".txt"
	if code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1",
		"--name", atTheLine, "--out", t.TempDir(), "--dry-run"); code != cli.ExitOK {
		t.Errorf("a name of exactly %d bytes was refused, a byte too early:\n%s", len(atTheLine), errOut)
	}
}

// TestAManifestNamedAsLongAsEverySystemStoresIsSaved names the record of a run
// 250 bytes long, which the plain join with its temporary marker put over the
// line.
func TestAManifestNamedAsLongAsEverySystemStoresIsSaved(t *testing.T) {
	name := strings.Repeat("m", 245) + ".json"
	if len(name) > core.MaxNameBytes || len(name)+len(core.WritingMarker) <= core.MaxNameBytes {
		t.Fatalf("a manifest name of %d bytes is not the case this guard asks about", len(name))
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: a
    format: txt
    count: 1
    size: 1kb
output:
  dir: `+filepath.ToSlash(out)+`
  manifest: `+name+`
`)
	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("a run with a manifest name of %d bytes ended with exit %d:\n%s", len(name), code, errOut)
	}
	if _, err := os.Stat(filepath.Join(out, name)); err != nil {
		t.Errorf("the manifest of %d bytes is not on the disk: %v", len(name), err)
	}
}

// TestARecipeNamedAsLongAsEverySystemStoresCanBeReplacedInPlace is the same
// question for "recipe fmt -w", asked of the one function it replaces a file
// through.
func TestARecipeNamedAsLongAsEverySystemStoresCanBeReplacedInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), strings.Repeat("r", 245)+".yaml")
	if base := filepath.Base(path); len(base)+len(core.WritingMarker) <= core.MaxNameBytes {
		t.Fatalf("a recipe name of %d bytes is not the case this guard asks about", len(base))
	}
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	want := "version: 1\ntargets: []\n"
	if err := core.ReplaceFile(path, []byte(want)); err != nil {
		t.Fatalf("a recipe named %d bytes long could not be replaced in place: %v", len(filepath.Base(path)), err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != want {
		t.Errorf("after the replace the recipe reads %q (%v)", body, err)
	}
}
