package guard

import (
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The number this tool prints as the minimum has to be a size it accepts.
//
// It was not. Measured on 2026-08-03 by asking for exactly what "tfg formats"
// advertised, with the tool's own defaults:
//
//	pdf   printed 3265   refused it, said 3286
//	wav   printed 44     refused it, said 98
//	zip   printed 156    refused it, said 4285
//
// The registry number is the structural floor of a format with no label, which
// is a real thing to know and not what somebody reads a column headed MINIMUM
// to mean. The label is on unless it is turned off, so for three formats out of
// twelve the advertised number was one the ordinary invocation would never
// take.
//
// This is the same class as the PNG dimensions closed earlier today - the tool
// advertising something it then refuses - and it is why the check below is on
// the outcome rather than on the declaration: ask for the number that is
// printed, and see whether it is taken.

// smallestPrinted reads the number "tfg formats <id>" puts in front of a
// person, so that this cannot drift from what is actually shown.
func smallestPrinted(t *testing.T, id string) int64 {
	t.Helper()
	code, stdout, errOut := run(t, "formats", id)
	if code != cli.ExitOK {
		t.Fatalf("formats %s gave %d:\n%s", id, code, errOut)
	}
	first := stdout
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	const marker = "minimum "
	i := strings.Index(first, marker)
	if i < 0 {
		t.Fatalf("formats %s does not print a minimum on its first line: %q", id, first)
	}
	rest := first[i+len(marker):]
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		t.Fatalf("the minimum is not followed by a unit: %q", first)
	}
	n, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil {
		t.Fatalf("the minimum is not a number: %q", first)
	}
	return n
}

func TestTheMinimumThisToolPrintsIsASizeItAccepts(t *testing.T) {
	descriptors := format.All()
	if len(descriptors) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	for _, d := range descriptors {
		t.Run(d.ID, func(t *testing.T) {
			dir := t.TempDir()
			printed := smallestPrinted(t, d.ID)

			// Exactly what was advertised, with nothing else said - so the
			// label is on and every property is at its default, which is what
			// somebody reading that column will type.
			code, _, errOut := run(t,
				"generate", "--format", d.ID, "--size", strconv.FormatInt(printed, 10),
				"--dry-run", "--out", dir)
			if code != cli.ExitOK {
				t.Errorf("the tool prints a minimum of %d B and refuses it:\n%s", printed, errOut)
			}
		})
	}
}

// And it is the smallest such size, not merely one that works. A number well
// above the real floor would satisfy the guard above while telling somebody
// they cannot ask for a file they can have.
func TestTheMinimumThisToolPrintsIsTheSmallestItAccepts(t *testing.T) {
	for _, d := range format.All() {
		t.Run(d.ID, func(t *testing.T) {
			printed := smallestPrinted(t, d.ID)
			if printed == 0 {
				// Nothing sits below zero, so there is nothing to refuse.
				return
			}
			dir := t.TempDir()
			code, _, _ := run(t,
				"generate", "--format", d.ID, "--size", strconv.FormatInt(printed-1, 10),
				"--dry-run", "--out", dir)
			if code == cli.ExitOK {
				t.Errorf("the tool prints a minimum of %d B and accepts %d B, so the number is not the smallest it takes",
					printed, printed-1)
			}
		})
	}
}
