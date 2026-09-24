package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// What this tool prints about a name a person cannot read shows that name,
// rather than letting it act (O241).
//
// Measured on 2026-09-24: verify reported a missing file named "photo", a
// right to left override, "gpj.txt" - and the terminal drew it as
// "phototxt.jpg" - and an extra "in", a zero width space, "voice.txt" that
// printed as "invoice.txt". The report named files other than the ones on the
// disk, and two of them could not be told apart. The preset of unusual file
// names writes exactly these, so every report about its files would lie in
// the same way.
//
// Asked from outside, through the commands, because the defect is a place that
// prints a name without going through core.Shown, and a new place tomorrow is
// the one nobody will remember. Every guard here asks two things: nothing in
// the text is a character nobody can see, and the escape is there - since
// printing nothing at all would pass the first alone.

var (
	rightToLeft   = string(rune(0x202E))
	zeroWidth     = string(rune(0x200B))
	lineSeparator = string(rune(0x2028))
)

// escapeOf is how a character is written once it is shown.
func escapeOf(s string) string {
	q := strconv.QuoteRune([]rune(s)[0])
	return q[1 : len(q)-1]
}

// saysNothingUnseen fails when said holds a character nobody can see, other
// than the line breaks that separate what it says.
func saysNothingUnseen(t *testing.T, what, said string) {
	t.Helper()
	for _, r := range said {
		if r != '\n' && r != '\r' && unseenHere(r) {
			t.Errorf("%s printed U+%04X raw:\n%s", what, r, said)
			return
		}
	}
}

// saysEscaped fails when said does not carry the name with its escape.
func saysEscaped(t *testing.T, what, said, stem, unseen, rest string) {
	t.Helper()
	if want := stem + escapeOf(unseen) + rest; !strings.Contains(said, want) {
		t.Errorf("%s does not show %q:\n%s", what, want, said)
	}
}

// unseenRun writes the three names into a directory and gives back the
// directory and the manifest.
func unseenRun(t *testing.T) (string, string) {
	t.Helper()
	// The directory has such a character too, so every line naming it is asked
	// as well - verify and cleanup printed it raw until a review of 2026-09-25.
	dir := filepath.Join(t.TempDir(), "run"+zeroWidth)
	var drafts []recipe.TargetDraft
	for i, name := range []string{"photo" + rightToLeft + "gpj.txt", "in" + zeroWidth + "voice.txt", "report" + lineSeparator + "ERROR.txt"} {
		drafts = append(drafts, recipe.TargetDraft{ID: "t" + strconv.Itoa(i), Format: "txt", Size: "1kb", Name: name})
	}
	src, err := recipe.Compose(recipe.Document{Targets: drafts})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "names.yaml")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"generate", path, "--out", dir}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the run ended %d:\n%s", code, errOut.String())
	}
	// Joined here rather than read off the "manifest:" line, which shows the
	// directory's character as an escape and so is not a path any more.
	if !regexp.MustCompile(`(?m)^manifest: `).MatchString(errOut.String()) {
		t.Fatalf("the run did not say where its manifest is:\n%s", errOut.String())
	}
	return dir, filepath.Join(dir, "manifest.json")
}

// carriesExactly fails unless some string in a JSON report is want, byte for
// byte - the half a program reads, which must not be escaped.
func carriesExactly(t *testing.T, what string, report []byte, want string) {
	t.Helper()
	var v any
	if err := json.Unmarshal(report, &v); err != nil {
		t.Errorf("%s is not JSON: %v\n%s", what, err, report)
		return
	}
	if !holdsString(v, func(s string) bool { return strings.HasSuffix(s, want) }) {
		t.Errorf("%s does not carry %+q exactly:\n%s", what, want, report)
	}
}

// linesWith is the lines of said that hold word, so a name is looked for on
// the line that says what happened to it.
func linesWith(said, word string) string {
	var out []string
	for _, line := range strings.Split(said, "\n") {
		if strings.Contains(line, word) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func holdsString(v any, match func(string) bool) bool {
	switch x := v.(type) {
	case string:
		return match(x)
	case []any:
		for _, e := range x {
			if holdsString(e, match) {
				return true
			}
		}
	case map[string]any:
		for _, e := range x {
			if holdsString(e, match) {
				return true
			}
		}
	}
	return false
}

func TestANoteAboutAFileShowsANameNobodyCanReadAsAnEscape(t *testing.T) {
	// A text file of one byte cannot carry its label, which is a note about
	// the file - the same trigger as the guard of notes about one file.
	one := runCLI(t, "generate", "--format", "txt", "--size", "1b", "--count", "1",
		"--name", "photo"+rightToLeft+"gpj.txt", "--dry-run", "--out", t.TempDir())
	if !strings.Contains(one, "note:") {
		t.Fatalf("the run printed no note, so this guard checked nothing:\n%s", one)
	}
	saysNothingUnseen(t, "a note about one file", one)
	saysEscaped(t, "a note about one file", one, "note: photo", rightToLeft, "gpj.txt: ")

	many := runCLI(t, "generate", "--format", "txt", "--size", "1b", "--count", "3",
		"--name", "in"+zeroWidth+"voice_{index:04}.txt", "--dry-run", "--out", t.TempDir())
	if !strings.Contains(many, "3 files:") {
		t.Fatalf("the run printed no grouped note, so this guard checked nothing:\n%s", many)
	}
	saysNothingUnseen(t, "a note about three files", many)
	saysEscaped(t, "a note about three files", many, "in", zeroWidth, "voice_0001.txt")
}

func TestVerifyShowsANameNobodyCanReadAsAnEscape(t *testing.T) {
	dir, manifestPath := unseenRun(t)
	if err := os.Remove(filepath.Join(dir, "photo"+rightToLeft+"gpj.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "extra"+zeroWidth+".txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"verify", manifestPath}, &out, &errOut); code != cli.ExitVerify {
		t.Fatalf("verify ended %d rather than with a mismatch:\n%s%s", code, out.String(), errOut.String())
	}
	said := out.String() + errOut.String()
	saysNothingUnseen(t, "verify", said)
	saysEscaped(t, "verify, the missing file", said, "photo", rightToLeft, "gpj.txt")
	saysEscaped(t, "verify, the extra file", said, "extra", zeroWidth, ".txt")

	out.Reset()
	errOut.Reset()
	cli.Run(context.Background(), []string{"verify", "--json", manifestPath}, &out, &errOut)
	carriesExactly(t, "verify --json", []byte(strings.TrimSpace(out.String()+errOut.String())), "photo"+rightToLeft+"gpj.txt")
}

func TestCleanupShowsANameNobodyCanReadAsAnEscape(t *testing.T) {
	dir, manifestPath := unseenRun(t)
	// Changed since it was written, so cleanup keeps it and says why.
	if err := os.WriteFile(filepath.Join(dir, "in"+zeroWidth+"voice.txt"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}

	preview := runCLI(t, "cleanup", manifestPath)
	saysNothingUnseen(t, "cleanup", preview)
	saysEscaped(t, "cleanup, a file it would remove", linesWith(preview, "remove"), "photo", rightToLeft, "gpj.txt")
	saysEscaped(t, "cleanup, a file it would keep", linesWith(preview, "keep"), "in", zeroWidth, "voice.txt")

	removed := runCLI(t, "cleanup", "--yes", manifestPath)
	saysNothingUnseen(t, "cleanup --yes", removed)
	saysEscaped(t, "cleanup --yes, the file it kept", linesWith(removed, "kept"), "in", zeroWidth, "voice.txt")
	if _, err := os.Stat(filepath.Join(dir, "in"+zeroWidth+"voice.txt")); err != nil {
		t.Errorf("the changed file was not kept: %v", err)
	}
}

func TestARefusalShowsANameNobodyCanReadAsAnEscape(t *testing.T) {
	// Two names that are one file on most systems, told apart only by case.
	src, err := recipe.Compose(recipe.Document{Targets: []recipe.TargetDraft{
		{ID: "lower", Format: "txt", Size: "1kb", Name: "a" + zeroWidth + ".txt"},
		{ID: "upper", Format: "txt", Size: "1kb", Name: "A" + zeroWidth + ".txt"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "collide.yaml")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	refused := runCLI(t, "validate", path)
	saysNothingUnseen(t, "a refusal of two names that collide", refused)
	saysEscaped(t, "a refusal of two names that collide", refused, "A", zeroWidth, ".txt")

	// A file already there under the name asked for.
	dir := t.TempDir()
	name := "photo" + rightToLeft + "gpj.txt"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	taken := runCLI(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1", "--name", name, "--out", dir)
	if !strings.Contains(taken, "already exists") {
		t.Fatalf("the run did not refuse the taken name, so this guard checked nothing:\n%s", taken)
	}
	saysNothingUnseen(t, "a refusal of a taken name", taken)
	saysEscaped(t, "a refusal of a taken name", taken, "photo", rightToLeft, "gpj.txt already exists")
}

// The rest of what a run says about a place: the files of a boundary set, a
// neighbouring run's files, and an output directory that cannot be used.
func TestTheLinesAboutARunShowANameNobodyCanReadAsAnEscape(t *testing.T) {
	boundary := runCLI(t, "generate", "--format", "txt", "--boundary", "2kb",
		"--name", "b"+rightToLeft+"_{index:04}.txt", "--dry-run", "--out", t.TempDir())
	if !strings.Contains(boundary, "boundary") {
		t.Fatalf("the run printed no boundary set, so this guard checked nothing:\n%s", boundary)
	}
	saysNothingUnseen(t, "a boundary set", boundary)
	saysEscaped(t, "a boundary set", linesWith(boundary, "0002"), "b", rightToLeft, "_0002.txt")

	// A second run beside the first, recording itself under its own name.
	dir, firstManifest := unseenRun(t)
	src, err := recipe.Compose(recipe.Document{Manifest: "record" + zeroWidth + ".json", Targets: []recipe.TargetDraft{
		{ID: "second", Format: "txt", Size: "1kb", Name: "second" + zeroWidth + ".txt"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "second.yaml")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	second := runCLI(t, "generate", path, "--out", dir)
	if !strings.Contains(second, "manifest:") {
		t.Fatalf("the second run did not record itself:\n%s", second)
	}
	saysNothingUnseen(t, "the line naming a run's manifest", second)
	saysEscaped(t, "the line naming a run's manifest", linesWith(second, "manifest:"), "record", zeroWidth, ".json")

	neighbour := runCLI(t, "verify", firstManifest)
	if !strings.Contains(neighbour, "another run's record") {
		t.Fatalf("verify said nothing about the other run, so this guard checked nothing:\n%s", neighbour)
	}
	saysNothingUnseen(t, "a note about another run", neighbour)
	saysEscaped(t, "a note about another run, its file", linesWith(neighbour, "another run"), "second", zeroWidth, ".txt")
	saysEscaped(t, "a note about another run, its record", linesWith(neighbour, "another run"), "record", zeroWidth, ".json")

	// An output directory that is a file, and one another run is holding.
	parent := t.TempDir()
	file := filepath.Join(parent, "out"+rightToLeft+"file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	notADir := runCLI(t, "generate", "--format", "txt", "--size", "1kb", "--out", file)
	saysNothingUnseen(t, "a refusal of an output directory that is a file", notADir)
	saysEscaped(t, "a refusal of an output directory that is a file", notADir, "out", rightToLeft, "file is a file")

	held := filepath.Join(parent, "held"+zeroWidth+"dir")
	if err := os.MkdirAll(held, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(held, ".tfg-run-lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	busy := runCLI(t, "generate", "--format", "txt", "--size", "1kb", "--out", held)
	if !strings.Contains(busy, "another run is already writing") {
		t.Fatalf("the run did not refuse a held directory, so this guard checked nothing:\n%s", busy)
	}
	saysNothingUnseen(t, "a refusal of a held directory", busy)
	saysEscaped(t, "a refusal of a held directory", busy, "held", zeroWidth, "dir, so this one")

	// A directory inside that file cannot be made, and the system's own error
	// under ours names the path again - raw, until a review of 2026-09-25.
	underAFile := runCLI(t, "generate", "--format", "txt", "--size", "1kb", "--out", filepath.Join(file, "sub"))
	if !strings.Contains(underAFile, "cannot create the output directory") {
		t.Fatalf("the run did not refuse a directory inside a file, so this guard checked nothing:\n%s", underAFile)
	}
	saysNothingUnseen(t, "a refusal of a directory inside a file", underAFile)
}

// The window says the same about an output directory as the command line: a
// folder named with a character nobody can see is shown with the escape,
// under the box it is about and at the foot of the form.
func TestTheWindowShowsADirectoryNobodyCanReadAsAnEscape(t *testing.T) {
	parent := t.TempDir()
	file := filepath.Join(parent, "out"+rightToLeft+"file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{file, filepath.Join(file, "sub")} {
		host, content := presetScreen(t)
		choosePreset(t, content, "filename-handling")
		fill(t, content, text.FieldOutputDir(), out)
		press(t, content, "Generate")
		join(host)

		said := everythingSaid(content)
		// The box itself holds what was typed, raw, which is right for a box.
		said = strings.ReplaceAll(said, out, "")
		if !strings.Contains(said, "out"+escapeOf(rightToLeft)+"file") {
			t.Fatalf("the window did not refuse %+q, or refused it without naming it:\n%s", out, said)
		}
		saysNothingUnseen(t, "the window's refusal of "+out, said)
	}
}
