// Part of package cli. See cli.go.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// verify compares a directory against a manifest an earlier run wrote.
func verify(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(errOut)
	against := fs.String("against", "", "directory to check. Defaults to the directory holding the manifest")
	asJSON := fs.Bool("json", false, "write the report to stdout as JSON")
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg verify - check that a directory still matches a manifest.

Reports files that are missing, files nobody asked for, and files whose
content has changed. The directory is always a local one.

Usage:
  tfg verify <manifest.json> [--against <dir>]

Flags:
`)
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(errOut)
	}
	fs.Usage = func() { usage(errOut) }
	if helpRequested(args) {
		usage(out)
		return ExitOK
	}
	leading, rest := splitLeadingPath(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	path, ok := onePath(leading, fs)
	if !ok {
		fmt.Fprintln(errOut, "tfg: verify takes one manifest file. Example: tfg verify out/manifest.json")
		return ExitUsage
	}
	if err := mustBeFile(path, "manifest.json", "verify"); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return ExitUsage
	}

	m, err := manifest.Load(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	// Defaulting to the directory the manifest sits in makes the common case
	// one argument. A manifest is written beside the files it describes.
	dir := *against
	if dir == "" {
		dir = filepath.Dir(path)
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		fmt.Fprintf(errOut, "tfg: cannot read the directory %s. Check the path and that you have permission to read it.\n", dir)
		return ExitIO
	}

	diffs, verifyErr := audit.Verify(ctx, dir, m, filepath.Base(path))
	claimed := len(audit.Claimed(m))

	// A cancelled run reports what it compared and never calls the rest sound.
	//
	// Anything else is a refusal rather than a cancellation, and saying
	// "interrupted" about it names an ending nobody caused. A manifest pointing
	// outside the directory used to arrive here as exit code 130.
	if verifyErr != nil {
		if !errors.Is(verifyErr, context.Canceled) {
			fmt.Fprintf(errOut, "tfg: %s\n", describeError(verifyErr))
			return classify(verifyErr)
		}
		fmt.Fprintf(errOut, "tfg: verify was interrupted after %s and did not check everything.\n", core.Count(len(diffs), "difference", "differences"))
		for _, d := range diffs {
			fmt.Fprintln(errOut, "  "+d.String())
		}
		return ExitInterrupted
	}

	return reportVerify(diffs, claimed, path, dir, *asJSON, out, errOut)
}

// mismatches is how many of these differences say the directory disagrees with
// this manifest.
//
// Not the same as how many differences there are, and the gap is the point. A
// file another run's manifest lists is a difference worth printing and it is
// not a disagreement with THIS one - the directory holds exactly what this
// manifest says it holds. Counting it as a mismatch made a shared directory
// permanently red, which is the same as having no check at all in the place
// output.manifest exists for.
//
// Every other kind still counts, the leftovers included. A leftover is ours and
// nothing is lost by it, and that is an argument for a different exit code that
// nobody has made or measured - so it keeps the one it has.
func mismatches(diffs []audit.Difference) int {
	n := 0
	for _, d := range diffs {
		if d.Kind != audit.AnotherRun {
			n++
		}
	}
	return n
}

// reportVerify says what the comparison found, as JSON or as prose.
func reportVerify(diffs []audit.Difference, claimed int, path, dir string, asJSON bool, out, errOut io.Writer) int {
	wrong := mismatches(diffs)
	if asJSON {
		report := verifyReport{
			Manifest:   path,
			Directory:  dir,
			Checked:    claimed,
			Matched:    wrong == 0,
			Difference: []verifyDifference{},
		}
		// Every difference is carried, including the ones that do not make the
		// directory a mismatch. A reader that wants only the disagreements
		// filters on kind, and a reader that wants to know what else is in the
		// directory has it. Dropping them would be the suppression this repair
		// is written to avoid.
		for _, d := range diffs {
			report.Difference = append(report.Difference, verifyDifference{
				Kind: string(d.Kind), Path: d.Path, Expected: d.Want, Found: d.Got,
			})
		}
		if wrong > 0 {
			// A failed run puts nothing on stdout, so the machine readable
			// report of a mismatch goes to stderr with the rest of the news.
			return writeJSON(errOut, errOut, report, ExitVerify)
		}
		return writeJSON(out, errOut, report, ExitOK)
	}

	if wrong > 0 {
		fmt.Fprintf(errOut, "tfg: %s does not match %s - %s:\n", dir, path, core.Count(wrong, "difference", "differences"))
		echoMismatches(diffs, errOut)
		echoOtherRuns(diffs, errOut)
		return ExitVerify
	}

	// A manifest describing nothing is not a match and not a mismatch. Saying
	// "everything is fine" about zero files invites somebody to trust a run
	// that never happened.
	if claimed == 0 {
		fmt.Fprintf(errOut, "%s claims no files, so there was nothing to check.\n", path)
		echoOtherRuns(diffs, errOut)
		return ExitOK
	}
	fmt.Fprintf(out, "%s matches %s: %s checked\n", dir, path, core.Count(claimed, "file", "files"))
	echoOtherRuns(diffs, errOut)
	return ExitOK
}

// echoMismatches lists the differences that are disagreements with THIS
// manifest.
//
// The neighbour's files are counted out of the heading, so they are listed out
// of the list too - they come back below, grouped, which is the only shape that
// survives a neighbour who wrote ten thousand files. A heading that says one
// number over a list of another is a report somebody stops reading.
func echoMismatches(diffs []audit.Difference, errOut io.Writer) {
	for _, d := range diffs {
		if d.Kind == audit.AnotherRun {
			continue
		}
		fmt.Fprintln(errOut, "  "+d.String())
	}
}

// otherRunExamples is how many file names one of these lines shows before it
// stops listing and starts counting.
//
// The same three the manifest's notes use, for the same measured reason: a run
// of 25 000 files once put 25 001 note lines on stderr, one per file, and the
// one line that mattered was buried under them. A neighbour can be that big.
const otherRunExamples = 3

// echoOtherRuns says what else is in the directory, and whose it is.
//
// One line per neighbouring record rather than one per file, which is what
// makes it safe to print at all. The number of records in a directory is small
// by construction - each one is a run that recorded itself - and the number of
// files is not.
//
// On the news channel rather than on stdout, beside the other things this tool
// says about a run that worked. The line before it is the answer somebody asked
// for and a script may be reading it.
func echoOtherRuns(diffs []audit.Difference, errOut io.Writer) {
	byRecord := groupedByRecord(diffs)

	names := make([]string, 0, len(byRecord))
	for name := range byRecord {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		files := byRecord[name]
		if len(files) == 0 {
			fmt.Fprintf(errOut, "note: %s is another run's record, and nothing else here belongs to it.\n", name)
			continue
		}
		fmt.Fprintf(errOut, "note: %s is another run's record. %s here %s to it: %s.\n",
			name, core.Count(len(files), "file", "files"), belongs(len(files)), someOf(files))
	}
}

// groupedByRecord collects the neighbours' files under the record that lists
// them.
//
// Its own function rather than a loop inside echoOtherRuns, which is the
// ceiling on nesting. A record gets an entry even when nothing else here is
// its, or a directory holding only somebody's manifest would say nothing about
// the one file in it that is not ours.
func groupedByRecord(diffs []audit.Difference) map[string][]string {
	byRecord := map[string][]string{}
	for _, d := range diffs {
		switch {
		case d.Kind != audit.AnotherRun:
		case d.Want != "":
			byRecord[d.Want] = append(byRecord[d.Want], d.Path)
		default:
			name := filepath.Base(d.Path)
			if _, seen := byRecord[name]; !seen {
				byRecord[name] = nil
			}
		}
	}
	return byRecord
}

// someOf names the first few and counts the rest.
func someOf(names []string) string {
	if len(names) <= otherRunExamples {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:otherRunExamples], ", ") + ", and " +
		core.Count(len(names)-otherRunExamples, "file", "files") + " not named here"
}

// belongs is the verb for that sentence. A number carries a noun in the right
// number on both surfaces, and it carries a verb too.
func belongs(n int) string {
	if n == 1 {
		return "belongs"
	}
	return "belong"
}

// cleanup removes the files a manifest lists, and nothing else.

type verifyReport struct {
	Manifest   string             `json:"manifest"`
	Directory  string             `json:"directory"`
	Checked    int                `json:"checked"`
	Matched    bool               `json:"matched"`
	Difference []verifyDifference `json:"differences"`
}

type verifyDifference struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Expected string `json:"expected,omitempty"`
	Found    string `json:"found,omitempty"`
}
