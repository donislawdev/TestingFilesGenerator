package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/commandline"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The Several batches screen copies two commands: the recipe of the screen,
// which the shell saves as a file, and tfg generate on that file. They are
// pasted into the real shells of this machine rather than read by a model,
// because what they lean on - a here-string in PowerShell, a here-document in
// sh, and the file each saves - is the shell's to get right. The class and
// the purpose hold what a shell reads as code when a text is not taken as it
// stands, so a saved recipe that went through the shell's hands changed is a
// difference in the hash and in the manifest.
//
// The saved file is held to the name the window gave it, which is the start
// of the hash of the recipe the window composed, and the files the second
// command makes are held to the files Generate makes from the same form.
// The canary hands the shell the same text with quoting that expands, and
// asks that what it saves is NOT the recipe - without that, a value the shell
// leaves alone either way would prove nothing about the quoting.
func TestACommandCopiedFromSeveralBatchesMakesTheFilesTheWindowMakes(t *testing.T) {
	dialect := commandline.ShellFor(runtime.GOOS)
	shells := shellsFor(dialect)
	if len(shells) == 0 {
		t.Skip("NOT CHECKED: no shell of this system on this machine. CI runs this on Windows, Linux and macOS.")
	}
	stand := echoBuilt(t)

	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	body := screen.Object()
	t.Cleanup(func() { join(host) })
	pressNamed(t, body, text.ButtonAddBatch())
	fields := screen.Fields()
	set := func(position int, setting, value string) {
		setBox(t, fields, recipe.TargetAddress(position, setting), value)
	}
	r := func(code rune) string { return string(code) }
	read := `it's "$HOME" $(echo x) ` + "`echo y`" + ` @' '@ TFG_EOF z` + r(0x017C) + r(0x00F3) + r(0x0142) + "w " + r(0x2019)
	set(1, recipe.KeyID, "first")
	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("txt")
	set(1, recipe.KeySize, "1kb")
	set(1, recipe.KeyCount, "2")
	set(1, recipe.KeyGroup, read)
	set(1, recipe.KeyPurpose, read)
	set(2, recipe.KeyID, "second")
	chooserIn(t, fields, recipe.TargetAddress(2, recipe.KeyFormat)).SetSelected("csv")
	set(2, recipe.KeySize, "2kb")
	setBox(t, fields, recipe.KeySeed, "4242")

	// An emptied folder is refused under its box and nothing is copied: the
	// recipe would leave output.dir out, and the command reads that as the
	// folder the terminal is in, where Generate refuses.
	setBox(t, fields, recipe.KeyOutputDir, "")
	press(t, body, text.ButtonCopyCommand())
	if host.copied != "" || findField(fields, recipe.KeyOutputDir).Saying() == "" {
		t.Errorf("with the output folder emptied the clipboard holds %q and the box says %q", host.copied, findField(fields, recipe.KeyOutputDir).Saying())
	}

	fromWindow := t.TempDir()
	setBox(t, fields, recipe.KeyOutputDir, fromWindow)
	press(t, body, text.ButtonGenerate())
	join(host)

	for _, shell := range shells {
		t.Run(filepath.Base(shell), func(t *testing.T) {
			fromScript := t.TempDir()
			setBox(t, fields, recipe.KeyOutputDir, fromScript)
			script := copiedFrom(t, host, body)
			last := script[strings.LastIndex(script, "\n")+1:]
			if !strings.HasPrefix(last, commandline.Name+" ") {
				t.Fatalf("the last command does not start tfg by its name, so the stand in put first on PATH is not what runs:\n%s", script)
			}

			cwd := t.TempDir()
			args := ranIn(t, shell, dialect, cwd, script, stand)
			if len(args) < 2 || args[0] != commandline.Generate {
				t.Fatalf("the script ran tfg with %q:\n%s", args, script)
			}
			file := args[1]
			saved := savedRecipe(t, cwd, file, script)
			if !strings.Contains(textIn(body), text.CommandSavesRecipe(file)) {
				t.Errorf("the screen does not say the recipe is saved as %s. It says: %s", file, textIn(body))
			}

			// The canary, in a folder of its own.
			elsewhere := t.TempDir()
			ranIn(t, shell, dialect, elsewhere, expanding(t, script, dialect), stand)
			if bytes.Equal(savedRecipe(t, elsewhere, file, ""), saved) {
				t.Errorf("CANARY: %s saved the recipe whole from text it was told to expand, so the values on this form cannot tell a quoted text from one that is not", shell)
			}

			t.Chdir(cwd)
			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
				t.Fatalf("the saved recipe ended with code %d:\n%s\n%s", code, script, errOut.String())
			}
			sameRun(t, script, fromScript, fromWindow)
		})
	}
}

// savedRecipe reads the file the shell saved and holds it to its name. The
// name carries the start of the hash of the recipe the window composed, so a
// file that does not hash to it is not the recipe that was copied. hold is
// the script, or empty for a file that is expected to differ.
func savedRecipe(t *testing.T, dir, file, hold string) []byte {
	t.Helper()
	saved, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatalf("the shell saved no %s: %v", file, err)
	}
	if hold == "" {
		return saved
	}
	hash, err := recipe.Hash(saved)
	if err != nil {
		t.Fatalf("the saved recipe does not read: %v\n%s", err, hold)
	}
	if want := strings.TrimPrefix(hash, "sha256:")[:12]; !strings.Contains(file, want) {
		t.Fatalf("the saved recipe hashes to %s, which is not the %s it was copied as:\n%s", hash, file, hold)
	}
	return saved
}

// expanding is the script with the quoting that keeps the text as it stands
// taken away: a here-string in double quotes in PowerShell, a here-document
// whose end is not quoted in sh. Both expand a variable and run a command
// written inside the text.
func expanding(t *testing.T, script string, s commandline.Shell) string {
	t.Helper()
	open, close, opened, closed := "<<'TFG_EOF'", "", "<<TFG_EOF", ""
	if s == commandline.PowerShell {
		open, close, opened, closed = "@'\n", "\n'@ |", "@\"\n", "\n\"@ |"
	}
	if !strings.Contains(script, open) || (close != "" && !strings.Contains(script, close)) {
		t.Fatalf("the script is not written the way the canary expects, so it would break nothing:\n%s", script)
	}
	bad := strings.Replace(script, open, opened, 1)
	if close != "" {
		bad = strings.Replace(bad, close, closed, 1)
	}
	return bad
}
