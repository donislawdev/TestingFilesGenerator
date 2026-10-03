package checksum

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// CheckID is the tool that checks the files a checksum file lists.
const CheckID = "checksum-check"

// InputChecksumFile is the checksum file checksum-check reads.
const InputChecksumFile = "checksum_file"

// The notes of checksum-check. The first is a count, the next six are what
// makes a check fail, and the last four are named without failing it.
const (
	noteChecked      = "checked"
	noteMismatched   = "mismatched"
	noteMissing      = "missing"
	noteNotAFile     = "not_a_file"
	noteOutside      = "outside"
	noteUnreadable   = "unreadable"
	noteChanged      = "changed"
	noteItself       = "itself"
	noteTwice        = "twice"
	noteNotChecksums = "not_checksums"
	noteUnknown      = "unknown"
)

func init() {
	tool.Register(tool.Descriptor{
		ID:       CheckID,
		Question: "Is every file still what its checksum file says?",
		Detail: "Reads a checksum file - SHA256SUMS, MD5SUMS or the tagged kind shasum writes - and checks every file it lists " +
			"in the folder the checksum file is in. Files it does not list are not looked at, the same as sha256sum -c.",
		Inputs: []tool.Input{{
			Name: InputChecksumFile, Kind: tool.File,
			Detail: "The checksum file. The paths in it are read from the folder it is in, and nothing is written.",
		}},
		Notes: []tool.Note{
			{ID: noteChecked, Says: "Files checked:"},
			{ID: noteMismatched, Says: "Not what the checksum file says:"},
			{ID: noteMissing, Says: "Listed and not there:"},
			{ID: noteNotAFile, Says: "Listed and not a file - a folder, a pipe, a device or a socket - so not read:"},
			{ID: noteOutside, Says: "Listed with a path that leaves the folder of the checksum file, so not read:"},
			{ID: noteUnreadable, Says: "Listed and could not be read:"},
			{ID: noteChanged, Says: "Changed while it was being read, so no checksum is given:"},
			{ID: noteItself, Says: "The checksum file itself, which cannot hold its own checksum - not checked:"},
			{ID: noteTwice, Says: "Listed more than once - every line was checked:"},
			{ID: noteNotChecksums, Says: "Lines that are not checksums, not checked:"},
			{ID: noteUnknown, Says: "Lines of an algorithm this tool does not work out, not checked:"},
		},
		Run: runCheck,
	})
}

// failing are the notes that make a check fail.
var failing = []string{noteMismatched, noteMissing, noteNotAFile, noteOutside, noteUnreadable, noteChanged}

// Checked is what --json prints about a checksum file checked.
type Checked struct {
	File     string    `json:"file"`
	Checked  int       `json:"checked"`
	Matched  int       `json:"matched"`
	Problems []Problem `json:"problems"`
	// NotChecked are the lines passed over without failing the check.
	NotChecked NotChecked `json:"not_checked"`
}

// Problem is one listed file that is not what the checksum file says.
type Problem struct {
	Path string `json:"path"`
	// Kind is one of the failing notes: mismatched, missing, not_a_file,
	// outside, unreadable, changed.
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	Expected string `json:"expected,omitempty"`
	Got      string `json:"got,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// NotChecked are the lines of a checksum file named without failing a check.
type NotChecked struct {
	Itself       []string `json:"itself"`
	Twice        []string `json:"twice"`
	NotChecksums []int    `json:"not_checksums"`
	Unknown      []string `json:"unknown"`
}

// target is one listed file on its way to being read: where it is on the
// disk, or why it is not read at all.
type target struct {
	entry Listed
	full  string
	// refused is the note a path gets without being read - outside, or the
	// checksum file itself - and why.
	refused, why string
}

// runCheck reads a checksum file and checks every file it lists.
func runCheck(ctx context.Context, in tool.Request, progress tool.Progress) (tool.Result, error) {
	sumsPath := in.Inputs[InputChecksumFile]
	parsed, self, err := readSums(sumsPath)
	if err != nil {
		var notAFile *NotAFileError
		if errors.As(err, &notAFile) {
			notAFile.Input = InputChecksumFile
		}
		return tool.Result{}, err
	}
	dir := filepath.Dir(sumsPath)
	targets := placeAll(core.NewBoundary(dir), sumsPath, parsed.Listed)

	var total int64
	for _, t := range targets {
		if info, statErr := os.Stat(t.full); t.refused == "" && statErr == nil {
			total += info.Size()
		}
	}
	tally := audit.NewTally(func(done int64) { progress(done, total) })
	answers, stopped := audit.InOrder(ctx, len(targets), func(i int, scratch []byte) Problem {
		return checkOne(ctx, targets[i], self, scratch, tally)
	})
	if stopped != nil {
		return tool.Result{}, stopped
	}
	return checkResult(sumsPath, parsed, targets, answers), nil
}

// placeAll is where each listed file is on the disk, settled in order on one
// goroutine before anything is read - the same rule verify keeps, for the
// same reason (audit.InOrder): the workers are never handed a refusal.
func placeAll(b core.Boundary, sumsPath string, all []Listed) []target {
	out := make([]target, 0, len(all))
	for _, e := range all {
		t := target{entry: e}
		if problem := core.ContainmentProblem(e.Path); problem != "" {
			t.refused, t.why = noteOutside, problem
			out = append(out, t)
			continue
		}
		t.full = filepath.Join(b.Dir(), filepath.FromSlash(e.Path))
		switch {
		case b.Escapes(t.full):
			// Said as what is known. A junction on Windows is refused here
			// even when it points inside, because the system gives no way to
			// follow one (measured on 2026-09-30, O265), so "leads out" would
			// claim more than was found.
			t.refused, t.why = noteOutside, "a link or a junction on the way leads out of the folder, or cannot be followed to tell"
		case filepath.Clean(t.full) == filepath.Clean(sumsPath):
			t.refused = noteItself
		}
		out = append(out, t)
	}
	return out
}

// checkOne reads one listed file and says what it came to. The zero Kind is a
// file that is what the checksum file says.
func checkOne(ctx context.Context, t target, self os.FileInfo, scratch []byte, tally *audit.Tally) Problem {
	p := Problem{Path: t.entry.Path, Line: t.entry.Line, Expected: t.entry.Sum}
	if t.refused != "" {
		p.Kind, p.Detail = t.refused, t.why
		return p
	}
	sums, opened, err := digest(ctx, t.full, map[string]bool{t.entry.Algorithm: true}, scratch, func(read, _ int64) { tally.Add(read) })
	var notAFile *NotAFileError
	var moved *ChangedError
	switch {
	case err == nil && os.SameFile(opened, self):
		// Reached through a link or another spelling, so the path above did
		// not see it. Asked of the two open files, which on Windows is the
		// only way the identity is there to compare.
		p.Kind = noteItself
	case err == nil && sums[t.entry.Algorithm] != t.entry.Sum:
		p.Kind, p.Got = noteMismatched, sums[t.entry.Algorithm]
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		p.Kind = noteMissing
	case errors.As(err, &notAFile):
		p.Kind = noteNotAFile
	case errors.As(err, &moved):
		p.Kind = noteChanged
	default:
		p.Kind, p.Detail = noteUnreadable, err.Error()
	}
	return p
}

// checkResult is what the answers come to: a note for each kind of problem,
// in the order of the lines, and the verdict about the checksum file.
func checkResult(sumsPath string, parsed Sums, targets []target, answers []Problem) tool.Result {
	data := &Checked{File: sumsPath, Checked: len(answers), Problems: []Problem{}, NotChecked: notChecked(parsed, targets)}
	byKind := map[string][]string{}
	for _, a := range answers {
		switch {
		case a.Kind == "":
			data.Matched++
		case a.Kind == noteItself:
			data.NotChecked.Itself = append(data.NotChecked.Itself, a.Path)
		default:
			data.Problems = append(data.Problems, a)
			byKind[a.Kind] = append(byKind[a.Kind], shownProblem(a))
		}
	}
	notes := []tool.Noted{{ID: noteChecked, Items: []string{strconv.Itoa(len(answers))}}}
	for _, kind := range failing {
		notes = appendNote(notes, kind, byKind[kind])
	}
	notes = appendNote(notes, noteItself, data.NotChecked.Itself)
	notes = appendNote(notes, noteTwice, data.NotChecked.Twice)
	notes = appendNote(notes, noteNotChecksums, numbers(data.NotChecked.NotChecksums))
	notes = appendNote(notes, noteUnknown, data.NotChecked.Unknown)

	verdict := tool.Verdict{Outcome: tool.Match, About: filepath.Base(sumsPath), Listed: true}
	if len(data.Problems) > 0 {
		verdict.Outcome = tool.Mismatch
	}
	return tool.Result{Verdict: verdict, Notes: notes, Data: data}
}

// shownProblem is a problem as an item of its note: the path, and the reason
// when there is one to give.
func shownProblem(p Problem) string {
	if p.Detail == "" {
		return p.Path
	}
	return p.Path + " - " + p.Detail
}

// notChecked are the lines named without failing the check, the checksum
// file itself aside - that one is only known once its line is read.
func notChecked(parsed Sums, targets []target) NotChecked {
	out := NotChecked{Itself: []string{}, Twice: []string{}, NotChecksums: parsed.NotSums, Unknown: parsed.Unknown}
	if out.NotChecksums == nil {
		out.NotChecksums = []int{}
	}
	if out.Unknown == nil {
		out.Unknown = []string{}
	}
	seen := map[string]int{}
	for _, t := range targets {
		seen[path.Clean(t.entry.Path)]++
	}
	for _, t := range targets {
		key := path.Clean(t.entry.Path)
		if seen[key] > 1 {
			out.Twice = append(out.Twice, t.entry.Path)
			seen[key] = 0
		}
	}
	return out
}

// numbers is a list of line numbers as items of a note.
func numbers(lines []int) []string {
	out := make([]string, 0, len(lines))
	for _, n := range lines {
		out = append(out, strconv.Itoa(n))
	}
	return out
}
