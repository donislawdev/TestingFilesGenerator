package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A run whose manifest this build cannot read back has to say so.
//
// Two ceilings were chosen independently and never held against each other.
// core.MaxFilesPerRun is a million files. manifest.MaxBytes is sixteen
// megabytes, and its comment works out that this "holds a run of roughly twenty
// thousand files".
//
// Measured on 2026-08-26: a run of 25 000 files wrote a manifest of 25 220 640
// B, and from that moment "tfg verify" and "tfg cleanup" both exited 5 on it
// with "is 25220640 B and the limit is 16777216 B". The files stayed on the
// disk and nothing in this toolset could remove them - the manifest is the only
// authority over what may be deleted, so a manifest that cannot be read is a
// set of files with no owner.
//
// The owner's decision on the same day was a note rather than a refusal: the
// run works, and refusing it would take away something this tool does today.
// What was missing was that nobody was told.
func TestARunSaysWhenItsManifestWillBeTooBigToReadBack(t *testing.T) {
	// Just over the ceiling, worked out from the estimate rather than from a
	// number written here - a guard carrying its own copy of a limit goes
	// stale the day somebody changes the real one and says nothing while it
	// does.
	over := int(manifest.MaxBytes/manifest.BytesPerEntry) + 1000
	if _, tooBig := manifest.TooLargeToReadBack(over, 0); !tooBig {
		t.Fatalf("%d entries was not judged too large, so this proved nothing", over)
	}

	said := runCLI(t, "generate", "--format", "txt", "--size", "200b",
		"--count", itoa(over), "--dry-run", "--out", t.TempDir())

	if !strings.Contains(said, "verify") || !strings.Contains(said, "cleanup") {
		t.Errorf("the note does not name the two commands that stop working:\n%s", said)
	}
	if !strings.Contains(said, "manifest") {
		t.Errorf("the note does not say what will be too big:\n%s", said)
	}
}

// And a run comfortably under it stays quiet.
//
// Without this the test above passes on a build that warns about every run,
// which teaches people to skip the line - the same reason the site probe skips
// pages carrying noindex rather than reporting noise.
func TestAnOrdinaryRunSaysNothingAboutTheManifestCeiling(t *testing.T) {
	said := runCLI(t, "generate", "--format", "txt", "--size", "200b",
		"--count", "100", "--dry-run", "--out", t.TempDir())

	if strings.Contains(said, "too big") || strings.Contains(said, "will not be able to read") {
		t.Errorf("a hundred files drew a warning about the manifest ceiling:\n%s", said)
	}
}

// The estimate has to stay near what a manifest really weighs.
//
// An estimate nobody checks is a number nobody measured, and this one decides
// whether the warning above appears at all. It is held to a wide band on
// purpose: the exact size moves with how long the names are and what the notes
// say, and a guard demanding the byte would go red for a reason that does not
// matter.
func TestTheManifestEstimateIsNearWhatAManifestWeighs(t *testing.T) {
	dir := t.TempDir()
	const files = 300

	runCLI(t, "generate", "--format", "txt", "--size", "200b",
		"--count", itoa(files), "--out", dir)

	info, err := os.Stat(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("the run wrote no manifest: %v", err)
	}
	real := info.Size()
	guess := manifest.EstimatedBytes(files, 0)

	// Within a quarter either way. Wider than the measurement, narrow enough
	// that the constant drifting by a factor would be caught.
	low, high := real*3/4, real*5/4
	if guess < low || guess > high {
		t.Errorf("the estimate for %d files is %d B and the manifest is %d B, which is outside a quarter either way",
			files, guess, real)
	}
}

// runCLI runs one command and gives back everything it said, on both channels.
//
// Both, because the line under test goes to the error channel - a report on
// stdout has to stay machine readable - and a guard reading only one of them
// would be measuring which channel it picked.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	cli.Run(context.Background(), args, &out, &errOut)
	return out.String() + errOut.String()
}
