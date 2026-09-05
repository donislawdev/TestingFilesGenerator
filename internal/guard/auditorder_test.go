package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Since 2026-09-05 verify and cleanup hash the files a manifest claims over as
// many goroutines as the machine has hardware threads, because that is where
// the time went once O117 took the path resolution out of the loop. Measured
// on the corpora this tool exists to produce: tfg verify on 6.1 GB went from
// 5.6-7.8 s to 0.68-0.76 s.
//
// The order of the answers is the property that had nothing watching it, and
// it is not decoration. cleanup PRINTS this list to a person and then deletes
// from it. A list assembled by appending whatever goroutine finished first
// still holds the same files, so every other cleanup guard here stays green -
// they ask what was removed, not in what order it was offered. What changes is
// that somebody reads one order and the tool acts in another, on the one
// command in this project that destroys data.
//
// Asked through the command line rather than by calling audit directly,
// because that is where the order is observable and because every other guard
// in this package drives the tool the way a person does.
func TestCleanupOffersTheFilesInTheOrderTheManifestListsThem(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")

	// Enough files that a shuffle cannot pass by luck. With sixteen workers and
	// a hundred and twenty files, a list built out of completion order comes
	// back wrong every time rather than one run in a hundred.
	const count = 120
	if code, _, errOut := run(t, "generate", "--format", "txt",
		"--count", "120", "--size", "4kb", "--out", out); code != cli.ExitOK {
		t.Fatalf("generate gave %d, expected %d: %s", code, cli.ExitOK, errOut)
	}
	mf := filepath.Join(out, "manifest.json")

	listed := manifestPathsInOrder(t, mf)
	// Asserted rather than assumed. A guard comparing two lists of nought
	// agrees with itself and proves nothing, and this suite has been bitten by
	// exactly that (O118).
	if len(listed) != count {
		t.Fatalf("the manifest lists %d files and the run asked for %d - this guard would be about the wrong thing",
			len(listed), count)
	}

	code, stdout, errOut := run(t, "cleanup", mf, "--json")
	if code != cli.ExitOK {
		t.Fatalf("the cleanup preview gave %d, expected %d: %s", code, cli.ExitOK, errOut)
	}

	var report struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("parsing the cleanup report: %v\n%s", err, stdout)
	}
	if len(report.Files) == 0 {
		t.Fatalf("the cleanup preview offered nothing, so there is no order to check:\n%s", stdout)
	}

	// A subsequence rather than an equal list, so this asks about ORDER and
	// nothing else. Which entries reach the offer is audit.Claimed's rule and
	// it has its own guards - repeating it here would mean two places to change
	// when it moves, and a red guard naming the wrong thing.
	at := 0
	for _, f := range report.Files {
		found := false
		for at < len(listed) {
			if listed[at] == f.Path {
				found, at = true, at+1
				break
			}
			at++
		}
		if !found {
			t.Fatalf("cleanup offered %q after the entries before it, and the manifest does not list it there.\n\n"+
				"The offer is built over several goroutines since 2026-09-05. In manifest order it is a list somebody\n"+
				"can read before the files go. In completion order it holds the same files and says them in an order\n"+
				"nobody was shown - on the one command here that deletes.\n\ncleanup offered:\n  %v\n\nthe manifest lists:\n  %v",
				f.Path, pathsOf(report.Files), listed)
		}
	}
}

// pathsOf is the offered list as plain strings, for a failure message that can
// be read next to the manifest.
func pathsOf(files []struct {
	Path string `json:"path"`
}) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// manifestPathsInOrder is every path a manifest names, in the order it names
// them - the order audit.Claimed walks and the order the offer has to keep.
func manifestPathsInOrder(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var m struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Path)
	}
	return out
}
