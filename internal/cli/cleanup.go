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
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

func cleanup(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	fs.SetOutput(errOut)
	yes := fs.Bool("yes", false, "actually remove the files. Without this nothing is deleted and the list is printed")
	force := fs.Bool("force", false, "remove files whose content has changed since they were written")
	withManifest := fs.Bool("with-manifest", false, "remove the manifest and the instructions beside it as well, once every file it lists is gone")
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
		// Not every refusal from the looking pass is a cancellation. It used to
		// be treated as one, so a manifest pointing outside the directory was
		// reported as "interrupted" under exit code 130 - an ending that says
		// somebody pressed Ctrl+C when nobody had.
		if errors.Is(inspectErr, context.Canceled) {
			fmt.Fprintln(errOut, "tfg: cleanup was interrupted while looking and removed nothing.")
			return ExitInterrupted
		}
		fmt.Fprintf(errOut, "tfg: %s Nothing was removed.\n", describeError(inspectErr))
		return classify(inspectErr)
	}

	if len(cands) == 0 {
		if *asJSON {
			return writeJSON(out, errOut, cleanupReport{Manifest: path, Directory: dir, Applied: *yes, Files: []cleanupEntry{}}, ExitOK)
		}
		fmt.Fprintf(errOut, "%s lists no files, so there was nothing to remove.\n", path)
		return ExitOK
	}

	run := cleanupRun{path: path, dir: dir, force: *force, asJSON: *asJSON, out: out, errOut: errOut}
	// Worked out before the preview as well as the real run, so the preview
	// names every file --yes would take. It named only the files the manifest
	// lists until 2026-09-25, and the real run then took the manifest and the
	// instructions beside it too.
	if *withManifest {
		run.record, run.notOurs = recordOf(path, m)
	}
	// The default run deletes nothing. A tool that removes files on the
	// strength of one argument is the wrong shape when the directory may hold
	// somebody's own work, and asking interactively is ruled out.
	if !*yes {
		return previewCleanup(cands, run)
	}
	return applyCleanup(ctx, cands, run)
}

// cleanupRun is what every part of one cleanup needs to know about it.
//
// One value rather than the nine arguments applyCleanup took until
// 2026-09-25, when the preview came to need the record as well.
type cleanupRun struct {
	path, dir     string
	force, asJSON bool
	// record is the manifest and the instructions beside it, when
	// --with-manifest asked for them to go too, and nothing otherwise.
	record []string
	// notOurs is instructions the manifest names that are not named after it.
	// They are said to stay and never removed - see recordOf.
	notOurs     string
	out, errOut io.Writer
}

// notOursReason is why instructions named after another manifest stay.
const notOursReason = "it is not named after this manifest, so it may belong to another run"

// recordOf is what a run wrote about itself: the manifest, and the
// instructions named after it when it names them. Load has already refused a
// manifest naming anything but a plain name there.
//
// Instructions named after some other manifest are not taken. They come back
// apart, to be reported as kept. A manifest is a file somebody can edit, and
// one naming the instructions of another run in the same directory had
// cleanup remove them - found by review on #140, measured on 2026-09-25. A
// manifest renamed after its run lands here too, and keeping its instructions
// is the cheaper mistake: a file left to remove by hand, rather than a file
// removed that was somebody else's. Refusing such a manifest outright would
// have stopped verify and cleanup on every renamed one (owner's choice).
func recordOf(path string, m *manifest.Manifest) (record []string, notOurs string) {
	record = []string{path}
	name := m.Run.Instructions
	if name == "" {
		return record, ""
	}
	full := filepath.Join(filepath.Dir(path), name)
	if name != manifest.InstructionsName(filepath.Base(path)) {
		return record, full
	}
	return append(record, full), ""
}

// previewCleanup lists what a run with --yes would remove, and removes nothing.
func previewCleanup(cands []audit.Candidate, run cleanupRun) int {
	force := run.force
	record := previewRecord(cands, run)
	if run.asJSON {
		report := cleanupReport{Manifest: run.path, Directory: run.dir, Applied: false, Record: record}
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
		return writeJSON(run.out, run.errOut, report, ExitOK)
	}

	fmt.Fprintf(run.out, "%s would be removed from %s:\n", core.Count(countRemovable(cands, force), "file", "files"), core.Shown(run.dir))
	for _, c := range cands {
		if c.Removable(force) {
			fmt.Fprintf(run.out, "  remove %s\n", core.Shown(c.Path))
			continue
		}
		fmt.Fprintf(run.out, "  keep   %s - %s\n", core.Shown(c.Path), skipNote(c, force))
	}
	sayRecord(run.out, record, "remove", "keep  ")
	fmt.Fprintf(run.errOut, "Nothing was removed. Run the same command with --yes to remove them.\n")
	return ExitOK
}

// previewRecord is what --yes would do with the manifest and the instructions
// beside it, decided the way applyCleanup decides it: the record goes only
// when no file it lists would still be on the disk. A file already gone does
// not hold it back, and neither do instructions already gone - removeRecord
// passes over those, so the preview says so rather than promising to remove
// them.
//
// A forecast, as the rest of the preview is. A file changed or removed between
// this and --yes is found by the real run and said there.
func previewRecord(cands []audit.Candidate, run cleanupRun) []cleanupEntry {
	if len(run.record) == 0 {
		return nil
	}
	staying := 0
	for _, c := range cands {
		if !c.Removable(run.force) && c.Disposition != audit.Absent {
			staying++
		}
	}
	entries := make([]cleanupEntry, 0, len(run.record))
	for i, p := range run.record {
		e := cleanupEntry{Path: p, Action: "would-remove"}
		switch {
		case staying > 0:
			e.Action, e.Reason = "would-keep", keptBecause(i, staying)
		case i > 0 && isGone(p):
			e.Action, e.Reason = "would-keep", "it is already gone"
		}
		entries = append(entries, e)
	}
	if run.notOurs != "" {
		entries = append(entries, cleanupEntry{Path: run.notOurs, Action: "would-keep", Reason: notOursReason})
	}
	return entries
}

// sayRecord puts the manifest and the instructions under the list of files, in
// the list's own words - gone for a file that goes or went, kept for one that
// stays, each as wide as the other so the names line up.
func sayRecord(w io.Writer, record []cleanupEntry, gone, kept string) {
	if len(record) == 0 {
		return
	}
	fmt.Fprintln(w, "After them, because of --with-manifest:")
	for _, e := range record {
		if e.Action == "removed" || e.Action == "would-remove" {
			fmt.Fprintf(w, "  %s %s\n", gone, core.Shown(e.Path))
			continue
		}
		fmt.Fprintf(w, "  %s %s - %s\n", kept, core.Shown(e.Path), e.Reason)
	}
}

// isGone says a name holds nothing at all - not a link, not a file.
func isGone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

// applyCleanup removes the files and says exactly what happened to each.
func applyCleanup(ctx context.Context, cands []audit.Candidate, run cleanupRun) int {
	force, asJSON, out, errOut := run.force, run.asJSON, run.out, run.errOut
	outcomes, removeErr := audit.Remove(ctx, run.dir, cands, force)

	// A file that was already gone is not a leftover. Counting it as one would
	// make the second run of cleanup fail, and the second run is the one a
	// script makes.
	report := cleanupReport{Manifest: run.path, Directory: run.dir, Applied: true}
	// Two counts rather than one, because they answer different questions and
	// conflating them is what this fixed. blocked is what a file still being
	// there MEANS - it decides the exit code and whether the manifest may go.
	// A file already gone blocks nothing. The report's own count is below.
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
			fmt.Fprintf(errOut, "kept %s - %s\n", core.Shown(o.Path), core.Shown(o.Reason))
		}
	}
	// Kept counts every entry that is not removed, which is what the entries
	// themselves say: each one carries "removed" or "kept" and nothing else.
	//
	// It used to be the number BLOCKED, which is a smaller set - a file already
	// gone is kept and is not blocked. Measured on 2026-08-26 on a run of four
	// files with one deleted by hand: removed 3, kept 0, four entries. A script
	// adding the two got three and had no way to know which entry it had lost.
	// The blocked count has no reader of its own, so it is gone rather than
	// moved to a third field somebody would have to be told about.
	report.Removed, report.Kept = removed, len(outcomes)-removed

	// There is no undo, so an interrupted run has to say exactly how far it
	// got. "Some of them" is not an answer somebody can act on.
	if removeErr != nil {
		fmt.Fprintf(errOut, "tfg: cleanup was interrupted. %s removed, %s left - what is gone is gone.\n",
			core.Count(removed, "file", "files"), core.Count(len(cands)-removed, "file", "files"))
		return ExitInterrupted
	}

	// A record that could not be removed ends the run here, with the report on
	// stderr like any failed run's. Before 2026-09-25 this ended without one.
	if code := settleRecord(run, blocked, &report); code != ExitOK {
		if asJSON {
			return writeJSON(errOut, errOut, report, code)
		}
		return code
	}

	if asJSON {
		// A run that left something behind ends non zero, and a failed run puts
		// nothing on stdout - so its report goes to stderr with the rest of the
		// news, the same as verify.
		if blocked > 0 {
			return writeJSON(errOut, errOut, report, ExitIO)
		}
		return writeJSON(out, errOut, report, ExitOK)
	}

	fmt.Fprintf(out, "%s removed from %s\n", core.Count(removed, "file", "files"), core.Shown(run.dir))
	// A record held back by a file left behind was said on stderr, with why.
	if blocked == 0 {
		sayRecord(out, report.Record, "removed", "kept   ")
	}

	// A file left behind is not a silent outcome. It was reported above, and
	// the exit code has to carry it too or a script never learns.
	if blocked > 0 {
		return ExitIO
	}
	return ExitOK
}

// settleRecord removes the manifest and the instructions beside it, when
// --with-manifest asked for them and no file they list is still on the disk,
// and puts what happened to each into the report. The exit code is about the
// record alone. A file left behind is the caller's to count.
func settleRecord(run cleanupRun, blocked int, report *cleanupReport) int {
	if len(run.record) == 0 {
		return ExitOK
	}
	code := ExitOK
	if blocked > 0 {
		for i, p := range run.record {
			report.Record = append(report.Record, cleanupEntry{Path: p, Action: "kept", Reason: keptBecause(i, blocked)})
		}
		also := ""
		if len(run.record) > 1 {
			also = ", and the instructions with it"
		}
		fmt.Fprintf(run.errOut, "tfg: the manifest was kept%s. It is the only record of %s still on disk.\n", also, core.Count(blocked, "file", "files"))
	} else {
		report.Record, code = removeRecord(run.record, run.errOut)
	}
	if run.notOurs != "" {
		report.Record = append(report.Record, cleanupEntry{Path: run.notOurs, Action: "kept", Reason: notOursReason})
	}
	return code
}

// keptBecause is why the file of the record at position i stays while n files
// the manifest lists stay on the disk. The same words in the preview and in
// the real run, so the two cannot come to disagree about the reason.
func keptBecause(i, n int) string {
	if i > 0 {
		return "it stays with the manifest"
	}
	return fmt.Sprintf("it is the only record of %s still on disk", core.Count(n, "file", "files"))
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
	case audit.Ready:
		// Named rather than left out, so a state added later reddens this
		// instead of falling through the sentence below in silence. A file
		// that is ready is not skipped, so it has no note.
	}
	// Unreachable today, and it stays a sentence rather than the bare state for
	// the day it is not. Removable covers Ready and covers Changed with
	// --force, so the only states arriving here are the three above - which is
	// why the bare word this used to return could never be seen. A state added
	// later would otherwise put a single word into a field that is a sentence
	// everywhere else, and scripts read this field.
	return "it was kept, and this build has no sentence for that state - which is a defect in the tool rather than in the file"
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
	// Record is the manifest and the instructions beside it, only when
	// --with-manifest was given. Apart from files, so that removed, kept and
	// would_remove go on counting what the manifest lists and nothing else.
	Record []cleanupEntry `json:"record,omitempty"`
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

// removeRecord takes away what a run wrote about itself, the instructions
// before the manifest that names them - so a failure part way leaves the
// manifest standing beside what it names rather than naming something gone.
// Instructions already gone are not a failure: somebody deleting a page of
// prose is no reason to keep a manifest they asked to have removed.
//
// What happened to each file comes back in the order of record, for the
// report, with the file that failed and every one not yet tried kept.
func removeRecord(record []string, errOut io.Writer) ([]cleanupEntry, int) {
	entries := make([]cleanupEntry, len(record))
	for i, p := range record {
		entries[i] = cleanupEntry{Path: p, Action: "removed"}
	}
	for i := len(record) - 1; i >= 0; i-- {
		err := os.Remove(record[i])
		if err == nil {
			continue
		}
		if i > 0 && errors.Is(err, os.ErrNotExist) {
			entries[i].Action, entries[i].Reason = "kept", "it is already gone"
			continue
		}
		what := "the manifest"
		if i > 0 {
			what = "the instructions"
		}
		fmt.Fprintf(errOut, "tfg: cannot remove %s %s: %s\n", what, core.Shown(record[i]), describeError(err))
		for j := 0; j < i; j++ {
			entries[j].Action, entries[j].Reason = "kept", "the instructions it names could not be removed"
		}
		entries[i].Action, entries[i].Reason = "kept", describeError(err)
		return entries, ExitIO
	}
	return entries, ExitOK
}
