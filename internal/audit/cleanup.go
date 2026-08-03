package audit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Cleanup removes a finished run. It removes what the manifest lists and
// nothing else, ever.
//
// This is the one command in the tool that destroys data, and it runs in
// directories that belong to somebody else. A person generates fixtures into a
// folder where they already keep something of their own, and the tool has no
// way of telling the two apart except the list it was handed. So the list is
// the whole authority: a file not in it is not touched, not counted, and not
// looked at.

// Disposition is what cleanup found for one file the manifest lists.
type Disposition string

const (
	// Ready is present, and its content is the content the manifest recorded.
	Ready Disposition = "ready"
	// Absent is already gone. Not an error - cleanup run twice has to be a
	// quiet no-op the second time, or people stop putting it in scripts.
	Absent Disposition = "absent"
	// Changed is present with content the manifest does not describe. Somebody
	// edited it or wrote over it, and whose work that is we cannot know.
	Changed Disposition = "changed"
	// Unreachable is present and could not be read, so we cannot tell whether
	// it is ours. Refusing is the only answer that is not a guess.
	Unreachable Disposition = "unreachable"
)

// Candidate is one file cleanup would remove, and what it found there.
type Candidate struct {
	Path        string
	Disposition Disposition
	Detail      string
}

// Removable says whether this candidate is one cleanup will delete.
//
// force covers Changed and nothing else. A file we could not read stays out of
// reach even with force, because force is a statement about whose work the
// file is, not about whether the disk is answering.
func (c Candidate) Removable(force bool) bool {
	return c.Disposition == Ready || (force && c.Disposition == Changed)
}

// Inspect works out what cleanup would do, touching nothing.
//
// It is a separate pass because the default run of cleanup deletes nothing and
// prints this list. A person gets to read what is about to disappear before it
// does, and CLI.md section 9 rules out asking them interactively.
func Inspect(ctx context.Context, dir string, m *manifest.Manifest) ([]Candidate, error) {
	var out []Candidate
	for _, f := range Claimed(m) {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		full, err := resolved(dir, f)
		if err != nil {
			// Nothing is inspected and nothing is offered. A list that points
			// outside the directory is not a list this tool acts on, and the
			// preview is where somebody decides - so it must not show the entry
			// as something it would remove.
			return nil, err
		}

		info, err := os.Stat(full)
		if errors.Is(err, fs.ErrNotExist) {
			out = append(out, Candidate{Path: f.Path, Disposition: Absent})
			continue
		}
		if err != nil {
			out = append(out, Candidate{Path: f.Path, Disposition: Unreachable, Detail: err.Error()})
			continue
		}

		// Size before hash, so a file of obviously the wrong length does not
		// cost a read of however many gigabytes it is.
		if info.Size() != f.Bytes {
			out = append(out, Candidate{Path: f.Path, Disposition: Changed,
				Detail: fmt.Sprintf("it is %d B and the manifest recorded %d B", info.Size(), f.Bytes)})
			continue
		}
		sum, err := hashFile(full)
		if err != nil {
			out = append(out, Candidate{Path: f.Path, Disposition: Unreachable, Detail: err.Error()})
			continue
		}
		if sum != f.Hashes.SHA256 {
			out = append(out, Candidate{Path: f.Path, Disposition: Changed,
				Detail: "its content is not the content this run wrote"})
			continue
		}
		out = append(out, Candidate{Path: f.Path, Disposition: Ready})
	}
	return out, ctx.Err()
}

// Outcome is what happened to one file.
type Outcome struct {
	Path    string
	Removed bool
	// Blocked is a file that is still on the disk and was not removed.
	//
	// It is not the same as "not removed". A file that was already gone is the
	// state the caller asked for, and counting it as a leftover would make
	// cleanup fail the second time it runs - which is exactly the run a script
	// makes. Only something still sitting there is a leftover.
	Blocked bool
	// Reason is filled in when the file was left alone, in the words a person
	// needs to decide what to do about it.
	Reason string
}

// Remove deletes the candidates that may be deleted.
//
// A file that cannot be removed does not stop the run. The rest of the list is
// still somebody's disk space, and stopping halfway would leave a directory in
// a state neither the manifest nor this report describes.
//
// Cancelling stops before the next file rather than in the middle of one. What
// was removed stays removed, and the caller reports both halves - there is no
// undo, so the report is the only record.
func Remove(ctx context.Context, dir string, cands []Candidate, force bool) ([]Outcome, error) {
	var out []Outcome
	for _, c := range cands {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if !c.Removable(force) {
			out = append(out, Outcome{
				Path:    c.Path,
				Blocked: c.Disposition != Absent,
				Reason:  skipReason(c, force),
			})
			continue
		}
		// Asked again rather than carried over from Inspect. The two passes are
		// separated by however long a person spends reading the preview, and
		// this is the one operation in the tool that destroys data - so the
		// question is put to the filesystem in the state it is in now.
		full, err := resolved(dir, manifest.File{Path: c.Path})
		if err != nil {
			return out, err
		}
		if err := os.Remove(full); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// It went away between the two passes. That is the state the
				// caller asked for, so it is not a failure.
				out = append(out, Outcome{Path: c.Path, Reason: "it was already gone"})
				continue
			}
			out = append(out, Outcome{Path: c.Path, Blocked: true, Reason: err.Error()})
			continue
		}
		out = append(out, Outcome{Path: c.Path, Removed: true})
	}
	return out, ctx.Err()
}

func skipReason(c Candidate, force bool) string {
	switch c.Disposition {
	case Absent:
		return "it was already gone"
	case Changed:
		return "it has changed since it was written, so it may not be ours. Pass --force to remove it anyway"
	case Unreachable:
		if force {
			return "it could not be read, so there is no telling whether it is ours. --force does not cover this"
		}
		return "it could not be read, so there is no telling whether it is ours"
	default:
		return "it was left alone"
	}
}
