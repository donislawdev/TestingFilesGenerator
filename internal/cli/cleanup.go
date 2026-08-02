// Part of package cli. See cli.go.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

func cleanup(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(errOut)
	yes := fs.Bool("yes", false, "actually remove the files. Without this nothing is deleted and the list is printed")
	force := fs.Bool("force", false, "remove files whose content has changed since they were written")
	withManifest := fs.Bool("with-manifest", false, "remove the manifest as well, once every file it lists is gone")
	asJSON := fs.Bool("json", false, "write the report as JSON instead of prose")
	against := fs.String("against", "", "directory to clean. Defaults to the directory holding the manifest")
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg cleanup - remove the files a manifest lists.

Removes what the manifest lists and nothing else. Without --yes it deletes
nothing and prints what it would remove. A file whose content has changed
since it was written is left alone and reported, because it may not be ours.

Usage:
  tfg cleanup <manifest.json> [--yes]

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
		fmt.Fprintln(errOut, "tfg: cleanup takes one manifest file. Example: tfg cleanup out/manifest.json")
		return ExitUsage
	}
	if err := mustBeFile(path, "manifest.json", "cleanup"); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return ExitUsage
	}

	m, err := manifest.Load(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	dir := *against
	if dir == "" {
		dir = filepath.Dir(path)
	}

	cands, inspectErr := audit.Inspect(ctx, dir, m)
	if inspectErr != nil {
		fmt.Fprintln(errOut, "tfg: cleanup was interrupted while looking and removed nothing.")
		return ExitInterrupted
	}

	if len(cands) == 0 {
		if *asJSON {
			writeJSON(out, cleanupReport{Manifest: path, Directory: dir, Applied: *yes, Files: []cleanupEntry{}})
			return ExitOK
		}
		fmt.Fprintf(errOut, "%s lists no files, so there was nothing to remove.\n", path)
		return ExitOK
	}

	// The default run deletes nothing. A tool that removes files on the
	// strength of one argument is the wrong shape when the directory may hold
	// somebody's own work, and asking interactively is ruled out.
	if !*yes {
		return previewCleanup(cands, path, dir, *force, *asJSON, out, errOut)
	}
	return applyCleanup(ctx, cands, path, dir, *force, *withManifest, *asJSON, out, errOut)
}

// previewCleanup lists what a run with --yes would remove, and removes nothing.
func previewCleanup(cands []audit.Candidate, path, dir string, force, asJSON bool, out, errOut io.Writer) int {
	if asJSON {
		report := cleanupReport{Manifest: path, Directory: dir, Applied: false}
		for _, c := range cands {
			e := cleanupEntry{Path: c.Path, State: string(c.Disposition)}
			if c.Removable(force) {
				e.Action = "would-remove"
			} else {
				e.Action, e.Reason = "would-keep", skipNote(c, force)
			}
			report.Files = append(report.Files, e)
			report.WouldRemove += boolToInt(c.Removable(force))
		}
		writeJSON(out, report)
		return ExitOK
	}

	fmt.Fprintf(out, "%d file(s) would be removed from %s:\n", countRemovable(cands, force), dir)
	for _, c := range cands {
		if c.Removable(force) {
			fmt.Fprintf(out, "  remove %s\n", c.Path)
			continue
		}
		fmt.Fprintf(out, "  keep   %s - %s\n", c.Path, skipNote(c, force))
	}
	fmt.Fprintf(errOut, "Nothing was removed. Run the same command with --yes to remove them.\n")
	return ExitOK
}

// applyCleanup removes the files and says exactly what happened to each.
func applyCleanup(ctx context.Context, cands []audit.Candidate, path, dir string, force, withManifest, asJSON bool, out, errOut io.Writer) int {
	outcomes, removeErr := audit.Remove(ctx, dir, cands, force)

	// A file that was already gone is not a leftover. Counting it as one would
	// make the second run of cleanup fail, and the second run is the one a
	// script makes.
	report := cleanupReport{Manifest: path, Directory: dir, Applied: true}
	removed, blocked := 0, 0
	for _, o := range outcomes {
		if o.Removed {
			removed++
			report.Files = append(report.Files, cleanupEntry{Path: o.Path, Action: "removed"})
			continue
		}
		if o.Blocked {
			blocked++
		}
		report.Files = append(report.Files, cleanupEntry{Path: o.Path, Action: "kept", Reason: o.Reason})
		if !asJSON {
			fmt.Fprintf(errOut, "kept %s - %s\n", o.Path, o.Reason)
		}
	}
	report.Removed, report.Kept = removed, blocked

	// There is no undo, so an interrupted run has to say exactly how far it
	// got. "Some of them" is not an answer somebody can act on.
	if removeErr != nil {
		fmt.Fprintf(errOut, "tfg: cleanup was interrupted. %d file(s) were removed and %d were not - what is gone is gone.\n",
			removed, len(cands)-removed)
		return ExitInterrupted
	}

	if withManifest {
		if blocked > 0 {
			fmt.Fprintf(errOut, "tfg: the manifest was kept because %d file(s) it lists are still there. It is the only record of them.\n", blocked)
		} else if err := os.Remove(path); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot remove the manifest %s: %s\n", path, describeError(err))
			return ExitIO
		}
	}

	if asJSON {
		// A run that left something behind ends non zero, and a failed run puts
		// nothing on stdout - so its report goes to stderr with the rest of the
		// news, the same as verify.
		if blocked > 0 {
			writeJSON(errOut, report)
			return ExitIO
		}
		writeJSON(out, report)
		return ExitOK
	}

	fmt.Fprintf(out, "%d file(s) removed from %s\n", removed, dir)

	// A file left behind is not a silent outcome. It was reported above, and
	// the exit code has to carry it too or a script never learns.
	if blocked > 0 {
		return ExitIO
	}
	return ExitOK
}

func countRemovable(cands []audit.Candidate, force bool) int {
	n := 0
	for _, c := range cands {
		if c.Removable(force) {
			n++
		}
	}
	return n
}

func skipNote(c audit.Candidate, force bool) string {
	switch c.Disposition {
	case audit.Absent:
		return "it is already gone"
	case audit.Changed:
		if !force {
			return "it has changed since it was written, so it may not be ours. Pass --force to remove it anyway"
		}
	case audit.Unreachable:
		return "it could not be read, so there is no telling whether it is ours"
	}
	return string(c.Disposition)
}

// cleanupReport is what --json puts out. Applied says whether anything was
// actually deleted, so a preview and a real run cannot be mistaken for each
// other by a script reading the file list.
type cleanupReport struct {
	Manifest    string         `json:"manifest"`
	Directory   string         `json:"directory"`
	Applied     bool           `json:"applied"`
	WouldRemove int            `json:"would_remove,omitempty"`
	Removed     int            `json:"removed"`
	Kept        int            `json:"kept"`
	Files       []cleanupEntry `json:"files"`
}

type cleanupEntry struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	// State is what was found there - ready, absent, changed or unreachable.
	// Only on a preview, where nothing has happened to it yet.
	State  string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
