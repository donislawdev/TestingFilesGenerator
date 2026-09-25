package guard

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// O245, measured on 2026-09-25: the help of "tfg preset eject" said "> my.yaml",
// and Windows PowerShell 5.1 saves that as UTF-16, which the tool then refused.
// Every other way through PowerShell 5.1 - Out-File included - decodes the
// output with the console's code page first, so on a stock console the letters
// outside ASCII were changed before anything reached the file. The answer is
// that the tool writes the file itself, and that the refusal of a UTF-16 file
// says where it came from. docs/O245-EJECT-2026-09-25.md.

// ejectedPreset has names in Polish, Korean and emoji, so a byte that moved on
// the way into the file has letters to move.
const ejectedPreset = "filename-handling"

// The file -o writes is byte for byte what eject would have printed, and
// nothing goes to standard output.
func TestEjectWritesTheFileByteForByteWhatItWouldPrint(t *testing.T) {
	code, printed, errOut := run(t, "preset", "eject", ejectedPreset)
	if code != cli.ExitOK || !bytes.ContainsFunc([]byte(printed), func(r rune) bool { return r > 0x7f }) {
		t.Fatalf("eject to standard output ended %d with %d bytes, none outside ASCII, so this guard compares nothing worth comparing: %s",
			code, len(printed), errOut)
	}
	path := filepath.Join(t.TempDir(), "my.yaml")
	code, out, errOut := run(t, "preset", "eject", ejectedPreset, "-o", path)
	if code != cli.ExitOK {
		t.Fatalf("eject -o ended %d: %s", code, errOut)
	}
	if out != "" {
		t.Errorf("eject -o put %d bytes on standard output as well as into the file", len(out))
	}
	if !strings.Contains(errOut, "recipe: "+path) {
		t.Errorf("eject -o does not say where the recipe went:\n%s", errOut)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, []byte(printed)) {
		t.Errorf("the file holds %d bytes and eject prints %d - they differ, so the file is not the recipe PR5 promises",
			len(written), len(printed))
	}
}

// A file already at the name is refused and left exactly as it was - it may
// be a recipe somebody ejected and then edited. Nothing else is left behind,
// and a directory that is not there is refused the same way.
func TestEjectLeavesAFileAlreadyThereAsItIs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my.yaml")
	edited := []byte("# somebody's edits\n")
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "preset", "eject", ejectedPreset, "-o", path)
	if code != cli.ExitIO {
		t.Errorf("eject -o over a file ended %d rather than %d: %s", code, cli.ExitIO, errOut)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, edited) {
		t.Errorf("eject -o changed a file that was already there: %q, %v", got, err)
	}
	if left := namesIn(t, dir); len(left) != 1 {
		t.Errorf("the refused eject left %v in the directory", left)
	}

	missing := filepath.Join(dir, "not-there", "my.yaml")
	if code, _, errOut := run(t, "preset", "eject", ejectedPreset, "-o", missing); code != cli.ExitIO {
		t.Errorf("eject -o into a directory that is not there ended %d rather than %d: %s", code, cli.ExitIO, errOut)
	}
}

// A write that fails after the name was claimed takes the claim back, so no
// empty my.yaml is left for somebody to run and wonder at.
func TestEjectThatCannotWriteLeavesNoEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my.yaml")
	// A directory where the recipe is written first, under its temporary
	// name, so the write fails after the claim and before the rename.
	if err := os.MkdirAll(core.SiblingPath(path, core.WritingMarker), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, "preset", "eject", ejectedPreset, "-o", path)
	if code != cli.ExitIO {
		t.Errorf("an eject that could not write ended %d rather than %d: %s", code, cli.ExitIO, errOut)
	}
	if _, err := os.Lstat(path); err == nil {
		t.Error("the eject that could not write left the claimed name behind as an empty file")
	}
}

// An empty name and "-" are refused as usage, and nothing is written - "-" is
// not standard output here, leaving -o out is.
func TestEjectRefusesAFileNameThatIsNotOne(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"", "-"} {
		code, out, errOut := run(t, "preset", "eject", ejectedPreset, "-o", name)
		if code != cli.ExitUsage || out != "" {
			t.Errorf("eject -o %q ended %d with %d bytes on standard output: %s", name, code, len(out), errOut)
		}
	}
	if left := namesIn(t, dir); len(left) != 0 {
		t.Errorf("a refused eject wrote %v", left)
	}
}

// A recipe in UTF-16 is refused, in either byte order, and the refusal says
// where such a file comes from and what to do instead. The general refusal of
// a file that is not UTF-8 says none of that.
func TestARecipeSavedAsUTF16IsRefusedAndSaysWhere(t *testing.T) {
	_, printed, _ := run(t, "preset", "eject", ejectedPreset)
	units := utf16.Encode([]rune(printed))
	for _, order := range []struct {
		name string
		mark []byte
		as   binary.AppendByteOrder
	}{
		{"little endian, as PowerShell 5.1 writes it", []byte{0xff, 0xfe}, binary.LittleEndian},
		{"big endian", []byte{0xfe, 0xff}, binary.BigEndian},
	} {
		body := append([]byte{}, order.mark...)
		for _, u := range units {
			body = order.as.AppendUint16(body, u)
		}
		path := filepath.Join(t.TempDir(), "my.yaml")
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errOut := run(t, "validate", path)
		if code != cli.ExitRecipe {
			t.Errorf("%s: a UTF-16 recipe ended %d rather than %d", order.name, code, cli.ExitRecipe)
		}
		for _, want := range []string{"UTF-16", "PowerShell 5.1", "-o my.yaml"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("%s: the refusal does not say %q:\n%s", order.name, want, errOut)
			}
		}
	}

	cp1250 := filepath.Join(t.TempDir(), "r.yaml")
	body := append([]byte("version: 1\ntargets:\n  - id: t\n    format: txt\n    size: 1kb\n    name: za"), 0xbf, 0xf3, 0xb3, 0xe6)
	if err := os.WriteFile(cp1250, append(body, ".txt\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, errOut := run(t, "validate", cp1250); strings.Contains(errOut, "UTF-16") {
		t.Errorf("a cp1250 recipe is refused as UTF-16:\n%s", errOut)
	}
}
