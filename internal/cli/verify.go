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

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

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
		fmt.Fprintf(errOut, "tfg: verify was interrupted after %d difference(s) and did not check everything.\n", len(diffs))
		for _, d := range diffs {
			fmt.Fprintln(errOut, "  "+d.String())
		}
		return ExitInterrupted
	}

	return reportVerify(diffs, claimed, path, dir, *asJSON, out, errOut)
}

// reportVerify says what the comparison found, as JSON or as prose.
func reportVerify(diffs []audit.Difference, claimed int, path, dir string, asJSON bool, out, errOut io.Writer) int {
	if asJSON {
		report := verifyReport{
			Manifest:   path,
			Directory:  dir,
			Checked:    claimed,
			Matched:    len(diffs) == 0,
			Difference: []verifyDifference{},
		}
		for _, d := range diffs {
			report.Difference = append(report.Difference, verifyDifference{
				Kind: string(d.Kind), Path: d.Path, Expected: d.Want, Found: d.Got,
			})
		}
		if len(diffs) > 0 {
			// A failed run puts nothing on stdout, so the machine readable
			// report of a mismatch goes to stderr with the rest of the news.
			writeJSON(errOut, report)
			return ExitVerify
		}
		writeJSON(out, report)
		return ExitOK
	}

	if len(diffs) > 0 {
		fmt.Fprintf(errOut, "tfg: %s does not match %s - %d difference(s):\n", dir, path, len(diffs))
		for _, d := range diffs {
			fmt.Fprintln(errOut, "  "+d.String())
		}
		return ExitVerify
	}

	// A manifest describing nothing is not a match and not a mismatch. Saying
	// "everything is fine" about zero files invites somebody to trust a run
	// that never happened.
	if claimed == 0 {
		fmt.Fprintf(errOut, "%s claims no files, so there was nothing to check.\n", path)
		return ExitOK
	}
	fmt.Fprintf(out, "%s matches %s: %d file(s) checked\n", dir, path, claimed)
	return ExitOK
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
