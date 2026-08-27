package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Every message this tool prints is English. The neighbouring scan proves that
// for the text inside the binary, and it cannot prove it for text that is not
// in the binary at all - the sentence an operating system hands back when a
// read fails arrives at run time.
//
// Measured on this machine 2026-08-01, and the result is narrower than it first
// looked. Go asks Windows for the English message explicitly before it asks for
// the machine's own, so `syscall.Errno(1).Error()` gives "Incorrect function."
// even here, where the system speaks Polish and FormatMessage on its own
// answers "Niepoprawna funkcja.". The leak is the fallback Go keeps for the
// case where the English resource is missing.
//
// So this guard is not only about language. The system's sentence is opaque
// whatever language it is in - "Incorrect function." for reading a directory
// tells nobody anything. Ours says what happened and carries the number, which
// is the part that means the same thing everywhere.

// TestASystemErrorReachesTheUserInOurWords drives real commands into real
// system errors and checks that what the system said does not survive to the
// terminal.
func TestASystemErrorReachesTheUserInOurWords(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "not-here", "manifest.json")

	// The exact sentence this operating system would have produced, taken from
	// the same error the commands below will hit. Written down here rather than
	// hard coded, because it differs between systems and between languages -
	// which is the whole point.
	osText := systemSentence(t, missing)

	cases := []struct {
		name string
		args []string
	}{
		{"verify", []string{"verify", missing}},
		{"cleanup", []string{"cleanup", missing, "--yes"}},
		{"recipe fmt", []string{"recipe", "fmt", filepath.Join(dir, "not-here", "recipe.yaml")}},
		{"validate", []string{"validate", filepath.Join(dir, "not-here", "recipe.yaml")}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), c.args, &out, &errOut)
			if code == cli.ExitOK {
				t.Fatalf("the command succeeded against a path that does not exist")
			}
			said := errOut.String()
			if said == "" {
				t.Fatal("the command failed and said nothing")
			}

			if strings.Contains(said, osText) {
				t.Errorf("the operating system's own sentence reached the terminal:\n  %q\n  in: %s",
					osText, strings.TrimSpace(said))
			}

			// The number is what survives translation and what somebody puts
			// into a search box, so it has to be there.
			if !strings.Contains(said, "system error ") {
				t.Errorf("the message carries no system error number: %s", strings.TrimSpace(said))
			}

			for i, r := range said {
				if r > 127 {
					t.Errorf("byte %d of the message is not ASCII: %q", i, said)
					break
				}
			}
		})
	}
}

// TestPointingACommandAtADirectorySaysSo covers the mistake somebody actually
// makes. Four commands take a file while the ones beside them take directories,
// and left alone the mistake surfaces as whatever the system says about reading
// a directory - "Incorrect function." on Windows, which reads as a defect in
// this tool rather than as a wrong argument.
func TestPointingACommandAtADirectorySaysSo(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		args []string
		file string
	}{
		{"verify", []string{"verify", dir}, "manifest.json"},
		{"cleanup", []string{"cleanup", dir, "--yes"}, "manifest.json"},
		{"validate", []string{"validate", dir}, "recipe.yaml"},
		{"recipe fmt", []string{"recipe", "fmt", dir}, "recipe.yaml"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), c.args, &out, &errOut)
			if code != cli.ExitUsage {
				t.Errorf("ended with %d, expected the usage code %d", code, cli.ExitUsage)
			}
			said := strings.TrimSpace(errOut.String())

			// It has to name what was wrong and what to write instead. A
			// message saying only "that failed" sends somebody looking for a
			// defect in the tool.
			if !strings.Contains(said, "is a directory") {
				t.Errorf("the message does not say the path is a directory: %s", said)
			}
			if !strings.Contains(said, c.file) {
				t.Errorf("the message does not name %s, so it does not say what to write instead: %s", c.file, said)
			}
			if out.Len() > 0 {
				t.Errorf("a failed run wrote to standard output: %s", out.String())
			}
		})
	}
}

// A run that produced nothing still writes a manifest, and that manifest is
// read by somebody's script. Every collection in it has to render as an empty
// collection rather than as null, because a reader looping over the entries has
// no reason to expect a value that is not a list.
//
// Found by a probe on 2026-08-01: a run interrupted before its first file
// finished rendered "files": null, while "generators", "by_format" and
// "by_expected" beside it all rendered as {}. The document shows a list.
func TestAManifestWithNoFilesRendersAnEmptyListNotNull(t *testing.T) {
	m := manifest.New("testing-files-generator", "0.0.0-test", "run_test",
		"tfg generate", 7741, "windows", "amd64")

	var rendered bytes.Buffer
	if err := m.Encode(&rendered); err != nil {
		t.Fatalf("encoding: %v", err)
	}
	text := rendered.String()

	// The raw bytes, because that is what a consumer parses. Decoding first
	// would hide exactly the difference this is about.
	for _, field := range []string{"files", "generators", "by_format", "by_expected", "by_target"} {
		if strings.Contains(text, `"`+field+`": null`) {
			t.Errorf("%q renders as null in a manifest describing no files - a reader looping over it meets a value that is not a collection", field)
		}
	}
	if !strings.Contains(text, `"files": []`) {
		t.Errorf("the manifest does not carry an empty file list:\n%s", text)
	}

	// And it has to survive the round trip, so the empty case reads back the
	// same way a full one does.
	var back struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rendered.Bytes(), &back); err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	if back.Files == nil {
		t.Error("the file list reads back as nothing rather than as an empty list")
	}
	if len(back.Files) != 0 {
		t.Errorf("the manifest describes %d files and the run produced none", len(back.Files))
	}
}

// systemSentence produces the exact text this operating system attaches to a
// missing path, by provoking the same error the commands under test will hit.
//
// Taken from the system rather than written down, because a hard coded English
// string would make this guard pass on the very machine where it should fail.
func systemSentence(t *testing.T, path string) string {
	t.Helper()
	_, err := os.Open(path)
	if err == nil {
		t.Fatal("the path meant to be missing exists")
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		t.Skipf("opening a missing path gave %T rather than a system error, so there is nothing to compare against", err)
	}
	text := errno.Error()
	if text == "" {
		t.Skip("this system attaches no text to the error")
	}
	return text
}
