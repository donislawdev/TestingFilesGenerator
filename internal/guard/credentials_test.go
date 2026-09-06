package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The password of a locked archive appears in the manifest once, where it was
// meant to, and not in the line people copy.
//
// It is written down on purpose and that is not the finding. archive.go says
// why in the property's own Detail: a locked fixture whose password is not
// written down checks nothing. What it also did, until 2026-09-06, was appear a
// second time - inside run.command, the whole command line recorded verbatim.
//
// The two places are one accident apart and they are read differently.
// files[].properties is fixture data and a reader treats it as such.
// run.command is the reproduction line: pasted into a bug report, quoted in a
// README, committed beside a fixture set. Found by an outside review on
// 2026-09-05, confirmed here against the build.
//
// Both shapes are asked because the flag package takes both, and a fix that
// covered one would look complete.
func TestTheRecordedCommandDoesNotRepeatACredential(t *testing.T) {
	const secret = "hunter2SECRET"

	shapes := map[string][]string{
		"the value as its own argument": {"--set", "password=" + secret},
		"the value joined to the flag":  {"--set=password=" + secret},
	}

	for what, setArgs := range shapes {
		t.Run(what, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out")
			args := append([]string{"generate", "--format", "zip",
				"--set", "entries=2", "--set", "encryption=aes-256"}, setArgs...)
			args = append(args, "--size", "64kb", "--count", "1", "--out", out)

			if code, _, errOut := run(t, args...); code != 0 {
				t.Fatalf("the run ended %d rather than 0, so this guard never reached a manifest:\n%s", code, errOut)
			}

			raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
			if err != nil {
				t.Fatalf("reading the manifest: %v", err)
			}
			var m struct {
				Run struct {
					Command string `json:"command"`
				} `json:"run"`
				Files []struct {
					Properties map[string]any `json:"properties"`
				} `json:"files"`
			}
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("reading the manifest as JSON: %v", err)
			}

			if strings.Contains(m.Run.Command, secret) {
				t.Errorf("run.command repeats the password:\n  %s\n"+
					"That field is the line people copy into a bug report. The password belongs in "+
					"the file's own properties, where a reader expects fixture data.", m.Run.Command)
			}
			if !strings.Contains(m.Run.Command, "password=***") {
				t.Errorf("run.command does not show that a value was taken out:\n  %s\n"+
					"A line with the setting missing altogether would not reproduce the run and "+
					"would not say why.", m.Run.Command)
			}

			// The deliberate copy is still there. A fix that took the password
			// out of both places would leave a locked archive nothing can open,
			// which is the whole reason it is recorded.
			if len(m.Files) == 0 {
				t.Fatal("the manifest lists no files, so the half about the deliberate copy asks nothing")
			}
			if got := m.Files[0].Properties["password"]; got != secret {
				t.Errorf("the file's own properties no longer carry the password: %v.\n"+
					"A test that cannot open the archive cannot check anything - archive.go says so "+
					"beside that setting.", got)
			}
		})
	}
}

// A manifest carrying a credential is written for its owner rather than for
// everyone with an account.
//
// The ordinary mode is 0644 on purpose and against the usual advice: this tool
// exists to produce files somebody else's CI will read, and .golangci.yml turns
// gosec's permission rules off here for exactly that reason. A manifest holding
// a password is the one file that argument does not cover.
//
// Windows has no permission bits, so this is skipped there and says so. The
// other half of this pair runs everywhere.
func TestAManifestCarryingACredentialIsWrittenForItsOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no permission bits - Go maps only the owner write bit onto its " +
			"read only attribute - so there is nothing to read here. The pairing between this " +
			"package and the registry is asked by the guard below, which runs everywhere.")
	}

	cases := []struct {
		what  string
		props map[string]any
		want  os.FileMode
	}{
		{"a manifest with a password", map[string]any{"password": "hunter2"}, 0o600},
		{"a manifest without one", map[string]any{"entries": 2}, 0o644},
		// An empty value is how "no password" is said from a window, where a box
		// somebody never typed in arrives as an empty string.
		{"a password property that is empty", map[string]any{"password": ""}, 0o644},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "manifest.json")
			m := manifest.New("testing-files-generator", "0.0.0-test", "run_x", "tfg generate", 1, "linux", "amd64")
			m.Add(manifest.File{ID: "files", Path: "a.zip", Name: "a.zip", Bytes: 1024, Properties: c.props})
			if err := m.Save(path); err != nil {
				t.Fatalf("saving: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("reading the mode back: %v", err)
			}
			if got := info.Mode().Perm(); got != c.want {
				t.Errorf("%s came out %04o and should be %04o.\n"+
					"A manifest holding a password is readable by every account on the machine at "+
					"0644, and a fixture that nobody else's CI can read is a defect in the product.",
					c.what, got, c.want)
			}
		})
	}
}

// What the record calls a credential is what the registry declares as one.
//
// Two copies of one fact, compared - the same arrangement the licence notices
// and the registry already have, and for the same reason. internal/manifest
// records what a run produced and knows nothing about formats, so asking the
// registry from there would tie the record to it and would answer "no secrets"
// quietly in a process that registered none.
//
// This is the half that runs on every system, which matters because the mode
// cannot be read on Windows and a mutation has to be provable somewhere.
func TestTheRecordAndTheRegistryAgreeOnWhatIsACredential(t *testing.T) {
	declared := format.SecretProperties()
	known := manifest.SecretProperties()

	if len(declared) == 0 {
		t.Fatal("no registered format declares a secret property, so this guard compares two " +
			"empty lists and proves nothing. archive.password is one - if it has stopped being " +
			"declared, that is the finding.")
	}

	for _, name := range declared {
		if !slices.Contains(known, name) {
			t.Errorf("a format declares %q as a credential and internal/manifest does not know it.\n"+
				"A manifest carrying it would be written 0644, readable by every account on the "+
				"machine, and nothing would say so. Add it to secretProperties there.", name)
		}
	}
	for _, name := range known {
		if !slices.Contains(declared, name) {
			t.Errorf("internal/manifest treats %q as a credential and no format declares it.\n"+
				"An entry that has outlived its property will quietly cover the next setting "+
				"given that name. Delete it, or declare Secret on the property it means.", name)
		}
	}
}
