package guard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/commandline"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The Copy command button promises one thing: pasted into a terminal, the line
// makes what the screen would have made. These guards hold that promise rather
// than the presence of flags - each one pastes the copied line into the command
// line and compares the files with the files the window writes from the same
// form. A flag the command line no longer takes, a default that moved, a value
// written so the command line reads it differently: each makes different files
// or none, and turns these red. See docs, the analysis of 2026-11-25.

func TestACommandCopiedFromSingleBatchMakesTheFilesTheWindowMakes(t *testing.T) {
	host := newFakeHost(t)
	gen := window.NewGenerate(host)
	content := gen.Object()
	t.Cleanup(func() { join(host) })

	// Every box the line writes a flag for is moved off its starting value, so
	// a flag the line forgets is a difference in the files rather than a
	// default that happened to agree.
	fields := gen.Fields()
	chooserIn(t, fields, "format").SetSelected("txt")
	chooserIn(t, fields, "encoding").SetSelected("utf-16le")
	setBox(t, fields, "size", "4kb")
	setBox(t, fields, "count", "3")
	setBox(t, fields, "id", "copied")
	setBox(t, fields, "name", "copied_{index:04}.txt")
	setBox(t, fields, "seed", "42")
	chooser, ok := controlUnder(content, text.FieldDamage()).(*parts.Chooser)
	if !ok {
		t.Fatalf("the damage field is not a list to choose from")
	}
	chooser.SetSelected(damage.ZeroHead)
	setBox(t, fields, damage.SettingBytes, "16")
	label, ok := findField(fields, engine.SettingLabel).Control.(*parts.Toggle)
	if !ok {
		t.Fatalf("the label field is not a switch")
	}
	label.SetChecked(false)

	fromLine := t.TempDir()
	entryUnder(t, content, text.FieldOutputDir()).SetText(fromLine)
	line := copiedFrom(t, host, content)
	for _, flag := range []string{commandline.Format, commandline.Set, commandline.Count, commandline.ID,
		commandline.NameTemplate, commandline.Seed, commandline.Damage, commandline.Clean, commandline.Out} {
		if !strings.Contains(line, flag) {
			t.Fatalf("the copied line has no %s, so this guard would compare a form it did not set up:\n%s", flag, line)
		}
	}
	runCopied(t, line)

	fromWindow := t.TempDir()
	entryUnder(t, content, text.FieldOutputDir()).SetText(fromWindow)
	press(t, content, text.ButtonGenerate())
	join(host)

	sameRun(t, line, fromLine, fromWindow)
}

func TestACommandCopiedFromPresetsMakesTheFilesTheWindowMakes(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewPreset(host)
	content := screen.Object()
	t.Cleanup(func() { join(host) })

	fields := screen.Fields()
	chooserIn(t, fields, "preset").SetSelected(choiceNamed(t, chooserIn(t, fields, "preset"), "size-boundaries"))
	setBox(t, fields, "limit", "2kb")
	setBox(t, fields, "spread", "1B")
	putIn(t, fields, "format", "txt")
	setBox(t, fields, engine.SettingSeed, "7")

	fromLine := t.TempDir()
	entryUnder(t, content, text.FieldOutputDir()).SetText(fromLine)
	line := copiedFrom(t, host, content)
	for _, flag := range []string{commandline.Preset, "--limit", "--spread", commandline.Format, commandline.Seed} {
		if !strings.Contains(line, flag) {
			t.Fatalf("the copied line has no %s:\n%s", flag, line)
		}
	}
	runCopied(t, line)

	fromWindow := t.TempDir()
	entryUnder(t, content, text.FieldOutputDir()).SetText(fromWindow)
	press(t, content, text.ButtonGenerate())
	join(host)

	sameRun(t, line, fromLine, fromWindow)
}

// The Tools screen makes no files, so its line is held to the answer: the
// checksum the command prints is the one worked out here, of the file the
// box named - not of a file the terminal happened to find under the same
// relative name.
func TestACommandCopiedFromToolsReadsTheFileTheBoxNames(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewTools(host)
	content := screen.Object()
	t.Cleanup(func() { join(host) })

	file := filepath.Join(t.TempDir(), "read me.bin")
	body := []byte("the file the box names")
	if err := os.WriteFile(file, body, 0o600); err != nil {
		t.Fatal(err)
	}
	fields := screen.Fields()
	pick := chooserIn(t, fields, "tool")
	pick.SetSelected(choiceNamed(t, pick, "checksum"))
	setBox(t, fields, "file", file)
	setBox(t, fields, "algorithm", "sha256")

	line := copiedFrom(t, host, content)
	out := runCopied(t, line)
	sum := sha256.Sum256(body)
	if !strings.Contains(out, hex.EncodeToString(sum[:])) {
		t.Errorf("the copied line printed\n%s\nwhich does not hold the sha256 of the file the box names.\nThe line: %s", out, line)
	}
}

// A form Generate refuses is refused by Copy command under the same box, and
// nothing reaches the clipboard. The same form with the box put right copies a
// line - the canary that the press reaches the screen at all, without which an
// empty clipboard proves nothing.
//
// The comma in a damage setting is refused while the line is put together,
// before it is written for a shell, so it is a refusal of its own here: it
// once reached the foot of the form in the words of the commandline package
// rather than under its box.
func TestCopyCommandRefusesWhatGenerateRefusesAndCopiesNothing(t *testing.T) {
	cases := []struct {
		label, damage, box, bad, good string
		says                          string
	}{
		{"a size that is not a size", "", "size", "ten", "10kb", ""},
		{"a name template with a tab in it", "", "name", "a\tb_{index}.txt", "ab_{index}.txt", text.CommandControlCharacter()},
		{"a damage setting with a comma in it", damage.ZeroHead, damage.SettingBytes, "1,6", "16", text.CommandCommaInDamage()},
		// The command does not refuse this one itself: without --out it writes
		// into the folder the terminal is in, where the window refuses.
		{"an output folder left empty", "", engine.SettingOutDir, "", os.TempDir(), ""},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			host := newFakeHost(t)
			gen := window.NewGenerate(host)
			content := gen.Object()
			t.Cleanup(func() { join(host) })
			fields := gen.Fields()
			entryUnder(t, content, text.FieldOutputDir()).SetText(t.TempDir())
			if c.damage != "" {
				chooser, ok := controlUnder(content, text.FieldDamage()).(*parts.Chooser)
				if !ok {
					t.Fatalf("the damage field is not a list to choose from")
				}
				chooser.SetSelected(c.damage)
			}

			setBox(t, fields, c.box, c.bad)
			if got := boxText(t, fields, c.box); got != c.bad {
				t.Fatalf("the box holds %q rather than %q, so there is nothing to refuse", got, c.bad)
			}
			press(t, content, text.ButtonCopyCommand())
			if host.copied != "" {
				t.Errorf("a refused form put %q on the clipboard", host.copied)
			}
			said := findField(fields, c.box).Saying()
			if said == "" || (c.says != "" && !strings.Contains(said, c.says)) {
				t.Errorf("the %s box says %q. Marked: %v", c.box, said, fields.Marked())
			}

			setBox(t, fields, c.box, c.good)
			press(t, content, text.ButtonCopyCommand())
			if host.copied == "" {
				t.Fatalf("the form put right copied nothing either, so the refusal above proves nothing")
			}
			if said := findField(fields, c.box).Saying(); said != "" {
				t.Errorf("the refusal stayed under the box after the copy worked: %q", said)
			}
		})
	}
}

// copiedFrom presses Copy command and answers the line, after asking that the
// status line says which shell it was written for.
func copiedFrom(t *testing.T, host *fakeHost, o fyne.CanvasObject) string {
	t.Helper()
	host.copied = ""
	press(t, o, text.ButtonCopyCommand())
	if host.copied == "" {
		t.Fatalf("Copy command put nothing on the clipboard. The screen says: %s", textIn(o))
	}
	said := text.CommandCopiedTerminal()
	if commandline.ShellFor(runtime.GOOS) == commandline.PowerShell {
		said = text.CommandCopiedPowerShell()
	}
	if !strings.Contains(textIn(o), said) {
		t.Errorf("after a copy the screen does not say %q. It says: %s", said, textIn(o))
	}
	return host.copied
}

// runCopied reads the line the way the shell of this system would, leaves out
// the word that starts tfg and hands the rest to the command line. It answers
// what the command printed.
func runCopied(t *testing.T, line string) string {
	t.Helper()
	words, err := readLine(line, commandline.ShellFor(runtime.GOOS))
	if err != nil {
		t.Fatalf("reading the copied line: %v\n%s", err, line)
	}
	args := withoutProgram(words)
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the copied line ended with code %d:\n%s\n%s", code, line, errOut.String())
	}
	return out.String()
}

// sameRun compares what two runs wrote: every file of the set, byte for byte,
// and what the manifests say about each.
func sameRun(t *testing.T, line, fromLine, fromWindow string) {
	t.Helper()
	a := damageManifest(t, filepath.Join(fromLine, engine.DefaultManifestName))
	b := damageManifest(t, filepath.Join(fromWindow, engine.DefaultManifestName))
	if len(a.Files) == 0 {
		t.Fatalf("the copied line wrote no files, so there is nothing to compare:\n%s", line)
	}
	if !reflect.DeepEqual(a.Files, b.Files) {
		left, _ := json.MarshalIndent(a.Files, "", "  ")
		right, _ := json.MarshalIndent(b.Files, "", "  ")
		t.Fatalf("the copied line and the window wrote different sets.\nThe line: %s\nFrom the line:\n%s\nFrom the window:\n%s", line, left, right)
	}
	for _, f := range a.Files {
		x, errX := os.ReadFile(filepath.Join(fromLine, f.Path))
		y, errY := os.ReadFile(filepath.Join(fromWindow, f.Path))
		if errX != nil || errY != nil || !bytes.Equal(x, y) {
			t.Errorf("%s differs between the copied line and the window (%v, %v)", f.Path, errX, errY)
		}
	}
}

// putIn sets a field whether it is a box or a list.
func putIn(t *testing.T, fields *parts.Fields, at, value string) {
	t.Helper()
	f := findField(fields, at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	if c, ok := f.Control.(*parts.Chooser); ok {
		c.SetSelected(choiceNamed(t, c, value))
		return
	}
	setBox(t, fields, at, value)
}

// choiceNamed is the entry of a list that is, or starts with, an id - a list of
// presets or tools shows the id with its question after it.
func choiceNamed(t *testing.T, c *parts.Chooser, id string) string {
	t.Helper()
	for _, o := range c.Options {
		if o == id || strings.HasPrefix(o, id+" ") {
			return o
		}
	}
	t.Fatalf("the list has no entry for %q: %v", id, c.Options)
	return ""
}

// withoutProgram drops the word that starts tfg: the name, a path, or the
// call operator of PowerShell and the path after it.
func withoutProgram(words []string) []string {
	if len(words) > 1 && words[0] == "&" {
		return words[2:]
	}
	if len(words) > 0 {
		return words[1:]
	}
	return nil
}

// readLine splits a line the way the shell would, for the quoting this
// project writes: single quotes in both, a doubled quote inside them in
// PowerShell, and in sh a quote closed, a backslashed quote and a quote opened
// again. Anything outside that is refused rather than
// guessed at, so a line written in some other way fails here.
//
// A model of the shell, which is why it is not the only reader: the guard in
// copyquoting_test.go hands lines to the real shells.
func readLine(line string, s commandline.Shell) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord, quoted := false, false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case quoted && isSingleQuote(c, s) && s == commandline.PowerShell && i+1 < len(runes) && runes[i+1] == c:
			word.WriteRune(c)
			i++
		case quoted && isSingleQuote(c, s):
			quoted = false
		case quoted:
			word.WriteRune(c)
		case c == '\'' || (s == commandline.PowerShell && isSingleQuote(c, s)):
			quoted, inWord = true, true
		case c == '\\' && s == commandline.POSIX && i+1 < len(runes):
			word.WriteRune(runes[i+1])
			inWord = true
			i++
		case c == ' ':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(c)
			inWord = true
		}
	}
	if quoted {
		return nil, errUnclosedQuote
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

// isSingleQuote is a character the shell reads as a single quote: the plain
// one, and in PowerShell four typographic ones as well.
func isSingleQuote(c rune, s commandline.Shell) bool {
	if c == '\'' {
		return true
	}
	return s == commandline.PowerShell && (c == 0x2018 || c == 0x2019 || c == 0x201A || c == 0x201B)
}

var errUnclosedQuote = errors.New("a quote is opened and never closed")
