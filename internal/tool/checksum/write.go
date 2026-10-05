package checksum

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// WriteID is the tool that writes the checksum file of a folder.
//
// A tool of its own rather than a way of running checksum, because a tool is
// one question with one set of things it works on (docs/NARZEDZIA-SUMY-
// 2026-09-29.md §15.1, the owner's decision of 2026-09-30): this one works on
// a folder, writes a file, and answers with what it wrote.
const WriteID = "checksum-write"

// InputFolder is the folder checksum-write lists.
const InputFolder = "folder"

// The notes of checksum-write.
const (
	noteWritten = "written"
	noteListed  = "listed"
	noteLinks   = "links"
	noteOthers  = "others"
	noteOurs    = "ours"
)

func init() {
	tool.Register(tool.Descriptor{
		ID:       WriteID,
		Question: "How will I know later that nothing in this folder changed?",
		Detail: "Writes the checksum of every file in a folder into one file beside them, SHA256SUMS for sha256. " +
			"Check it later with tfg tool checksum-check, or with sha256sum -c in that folder.",
		Inputs: []tool.Input{{
			Name: InputFolder, Kind: tool.Folder,
			Detail: "The folder to list. Every file under it is read, and the checksum file is written into it.",
		}},
		Settings: []format.Property{{
			Name: SettingAlgorithm, Kind: format.PropertyChoice, Choices: sumsAlgorithms, Default: "sha256",
			Detail: "Which checksum to write. sha256 is the one sha256sum and most release pages use.",
		}},
		Notes: []tool.Note{
			{ID: noteWritten, Says: "Written:"},
			{ID: noteListed, Says: "Files in it:"},
			{ID: noteLinks, Says: "Left out, because they are links and a link is not followed:"},
			{ID: noteOthers, Says: "Left out, because they are not files - a pipe, a device, a socket or a junction - and reading one may never end:"},
			{ID: noteOurs, Says: "Left out, because a run of this program left them half written:"},
		},
		Run: runWrite,
	})
}

// Written is what --json prints about a checksum file written.
type Written struct {
	Folder    string  `json:"folder"`
	File      string  `json:"file"`
	Algorithm string  `json:"algorithm"`
	Files     int     `json:"files"`
	LeftOut   LeftOut `json:"left_out"`
}

// LeftOut are the names under the folder that are not in the checksum file,
// by why.
type LeftOut struct {
	Links      []string `json:"links"`
	NotFiles   []string `json:"not_files"`
	Unfinished []string `json:"unfinished"`
}

// runWrite lists a folder, works out the checksum of every file in it and
// writes them down.
//
// The file is written last, after every checksum is known, and under a name
// of its own first (core.WriteNew) - so a run stopped anywhere leaves either
// no checksum file or a whole one, and never writes over one somebody has.
func runWrite(ctx context.Context, in tool.Request, progress tool.Progress) (tool.Result, error) {
	named := in.Inputs[InputFolder]
	algorithm := in.Values[SettingAlgorithm]
	if err := mustBeFolder(named); err != nil {
		return tool.Result{}, err
	}
	// The folder is looked up once, and the walk, the reading and the writing
	// all work in what that look found. Each followed the name on its own
	// until the review of #158, so a link pointed at another folder half way
	// had the files of one folder read and the checksum file land in the
	// other. Said as it was named, which is what the person typed - a folder
	// under /var on macOS is under /private/var once followed.
	dir := followed(named)
	target, shown := filepath.Join(dir, sumsName(algorithm)), filepath.Join(named, sumsName(algorithm))
	// Asked before a byte is read, so a folder of gigabytes is not read to be
	// refused at the end. Asked again, by the system, when the file is given
	// its name - somebody may write one in the meantime.
	if _, err := os.Lstat(target); err == nil {
		return tool.Result{}, &SumsExistError{Path: shown}
	}

	found, err := audit.Walk(ctx, dir)
	if err != nil {
		return tool.Result{}, err
	}
	plan := planWrite(found.Entries)
	// A directory that could not be listed means files nobody can see, and a
	// checksum file promises every file. Refused before anything is read - the
	// owner's decision of 2026-09-29 - naming every one.
	if len(found.Unreadable) > 0 {
		return tool.Result{}, &FolderUnreadableError{Folder: named, Problems: problemsOf(found.Unreadable)}
	}
	if len(plan.files) == 0 {
		return tool.Result{}, &NothingToListError{Folder: named, LeftOut: plan.leftOut.count()}
	}

	sums, err := hashAll(ctx, dir, plan.files, algorithm, progress)
	if err != nil {
		var unreadable *FolderUnreadableError
		if errors.As(err, &unreadable) {
			unreadable.Folder = named
		}
		return tool.Result{}, err
	}
	content := sumsContent(plan.files, sums)
	if err := roomFor(dir, shown, len(content)); err != nil {
		return tool.Result{}, err
	}
	if _, err := core.WriteNew(target, []byte(content), 0o644); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return tool.Result{}, &SumsExistError{Path: shown}
		}
		return tool.Result{}, err
	}
	return writeResult(named, shown, algorithm, plan), nil
}

// followed is a folder with the links on the way to it followed, or as it was
// named when they cannot be, the way the walk takes it. A junction on Windows
// is not followed (O265), and the walk then finds a name that is not a folder
// rather than the files behind it - measured on 2026-10-05, a refusal saying
// there is nothing to list (O267).
func followed(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}

// writePlan is a folder sorted into what goes in the checksum file and what is
// left out, and why.
type writePlan struct {
	files   []audit.Entry
	leftOut LeftOut
}

func (l LeftOut) count() int { return len(l.Links) + len(l.NotFiles) + len(l.Unfinished) }

// planWrite sorts the entries of a folder. Files are in byte order of their
// paths, so one folder gives one checksum file whichever system listed it -
// the walk lists each directory in order, and "a/x" came before "a.b" in it.
func planWrite(entries []audit.Entry) writePlan {
	var plan writePlan
	for _, e := range entries {
		switch {
		case e.Kind == audit.Link:
			plan.leftOut.Links = append(plan.leftOut.Links, e.Path)
		case e.Kind != audit.Regular:
			plan.leftOut.NotFiles = append(plan.leftOut.NotFiles, e.Path)
		case unfinished(path.Base(e.Path)):
			plan.leftOut.Unfinished = append(plan.leftOut.Unfinished, e.Path)
		default:
			plan.files = append(plan.files, e)
		}
	}
	sort.Slice(plan.files, func(i, j int) bool { return plan.files[i].Path < plan.files[j].Path })
	return plan
}

// unfinished is a name this program writes under while a file is not whole
// yet. One left in a folder is half a file, and its checksum would be the
// checksum of nothing anybody asked for.
func unfinished(name string) bool {
	return core.IsPartialName(name) || core.IsWritingName(name) || core.IsRunLockName(name)
}

// hashed is what one file of a folder came to.
type hashed struct {
	sum string
	err error
}

// hashAll works out the checksum of every file, several at once, and refuses
// the whole folder if any of them could not be read - naming all of them.
func hashAll(ctx context.Context, dir string, files []audit.Entry, algorithm string, progress tool.Progress) ([]string, error) {
	var total int64
	for _, f := range files {
		if info, err := f.Info(); err == nil {
			total += info.Size()
		}
	}
	tally := audit.NewTally(func(done int64) { progress(done, total) })
	chosen := map[string]bool{algorithm: true}
	answers, stopped := audit.InOrder(ctx, len(files), func(i int, scratch []byte) hashed {
		full := filepath.Join(dir, filepath.FromSlash(files[i].Path))
		sums, _, err := digest(ctx, full, chosen, scratch, func(read, _ int64) { tally.Add(read) })
		return hashed{sum: sums[algorithm], err: err}
	})
	if stopped != nil {
		return nil, stopped
	}
	sums := make([]string, len(answers))
	var problems []string
	for i, a := range answers {
		if a.err != nil {
			problems = append(problems, files[i].Path+" - "+a.err.Error())
		}
		sums[i] = a.sum
	}
	if len(problems) > 0 {
		return nil, &FolderUnreadableError{Folder: dir, Problems: problems}
	}
	return sums, nil
}

// problemsOf is each error as a line of a refusal.
func problemsOf(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Error())
	}
	return out
}

// sumsContent is the checksum file: one line a file, in the order given.
func sumsContent(files []audit.Entry, sums []string) string {
	var b strings.Builder
	for i, f := range files {
		b.WriteString(SumsLine(sums[i], f.Path))
	}
	return b.String()
}

// roomFor refuses a checksum file the disk has no room for, before a byte of
// it is written - the frozen table has a code of its own for a full disk.
// A disk whose free space cannot be read is not refused: the write that
// follows answers the question instead.
func roomFor(dir, target string, need int) error {
	have, err := core.AvailableBytes(dir)
	if err != nil || have >= int64(need) {
		return nil
	}
	return &NoRoomError{Path: target, Need: int64(need), Have: have}
}

// writeResult is what a checksum file written comes to, as notes and as data.
func writeResult(dir, target, algorithm string, plan writePlan) tool.Result {
	// Empty lists rather than null, so a script can count them without
	// asking first whether they are there.
	left := LeftOut{Links: orEmpty(plan.leftOut.Links), NotFiles: orEmpty(plan.leftOut.NotFiles), Unfinished: orEmpty(plan.leftOut.Unfinished)}
	data := &Written{Folder: dir, File: target, Algorithm: algorithm, Files: len(plan.files), LeftOut: left}
	notes := []tool.Noted{
		{ID: noteWritten, Items: []string{target}},
		{ID: noteListed, Items: []string{strconv.Itoa(len(plan.files))}},
	}
	notes = appendNote(notes, noteLinks, plan.leftOut.Links)
	notes = appendNote(notes, noteOthers, plan.leftOut.NotFiles)
	notes = appendNote(notes, noteOurs, plan.leftOut.Unfinished)
	return tool.Result{Notes: notes, Data: data}
}

// appendNote adds a note when it has something to say.
func appendNote(notes []tool.Noted, id string, items []string) []tool.Noted {
	if len(items) == 0 {
		return notes
	}
	return append(notes, tool.Noted{ID: id, Items: items})
}

// orEmpty is a list, never nil.
func orEmpty(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

// mustBeFolder refuses a path that is not a folder, in words about folders.
func mustBeFolder(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &NotAFolderError{Path: dir}
	}
	return nil
}
