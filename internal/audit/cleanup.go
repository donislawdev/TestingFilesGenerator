package audit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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
	boundary := core.NewBoundary(dir)
	claimed := Claimed(m)

	// Nothing is inspected and nothing is offered when one of these points
	// outside the directory. A list that points outside is not a list this tool
	// acts on, and the preview is where somebody decides - so it must not show
	// the entry as something it would remove.
	//
	// Resolved before anything is read and on one goroutine, for the reason
	// written out at claimedPaths and at parallel.go: a refusal that could
	// arrive from a worker would name whichever file lost the race.
	full, err := claimedPaths(boundary, claimed)
	if err != nil {
		return nil, err
	}

	// In order, because this list is what cleanup removes from and what it
	// printed to a person beforehand.
	return inOrder(ctx, len(claimed), func(i int, scratch []byte) Candidate {
		return look(claimed[i], full[i], scratch)
	})
}

// look is what one claimed file comes to for cleanup: whether it may be
// removed, and if not, why not in the words a person needs.
func look(f manifest.File, full string, scratch []byte) Candidate {
	info, err := os.Stat(full)
	if errors.Is(err, fs.ErrNotExist) {
		return Candidate{Path: f.Path, Disposition: Absent}
	}
	if err != nil {
		return Candidate{Path: f.Path, Disposition: Unreachable, Detail: err.Error()}
	}

	// Size before hash, so a file of obviously the wrong length does not cost
	// a read of however many gigabytes it is.
	if info.Size() != f.Bytes {
		return Candidate{Path: f.Path, Disposition: Changed,
			Detail: fmt.Sprintf("it is %d B and the manifest recorded %d B", info.Size(), f.Bytes)}
	}
	sum, err := hashFile(full, scratch)
	if err != nil {
		return Candidate{Path: f.Path, Disposition: Unreachable, Detail: err.Error()}
	}
	if sum != f.Hashes.SHA256 {
		return Candidate{Path: f.Path, Disposition: Changed,
			Detail: "its content is not the content this run wrote"}
	}
	return Candidate{Path: f.Path, Disposition: Ready}
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
	// This pass works the boundary out for itself rather than taking Inspect's,
	// which is the same rule as the one below: the two passes are separated by
	// however long somebody spends reading the preview, and nothing learned
	// before that pause is carried across it.
	//
	// Inside this pass the boundary is settled once. What that does and does not
	// cover is worth being exact about, because this is the one operation here
	// that destroys data. Every entry still gets its own walk below the
	// boundary, asked of the filesystem at the moment that entry is removed - so
	// a link planted under the directory mid-run is still caught. What is not
	// re-asked is the directory the person named. Swapping that for a link
	// halfway through was never caught: the old reading resolved both ends
	// through the new link and found them agreeing, so it reported "inside" too.
	// Redirections above the boundary are the caller's own, as the comment on
	// crossesUnresolvedLink says.
	boundary := core.NewBoundary(dir)
	stored := storedNames{dirs: map[string]map[string]string{}}

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
		full, err := resolved(boundary, manifest.File{Path: c.Path})
		if err != nil {
			return out, err
		}
		// Untouchable rule 7 in the one place a filesystem can bend it without
		// anybody noticing. os.Remove is given the name the manifest lists, and
		// on a filesystem that ignores case it will happily delete a file
		// stored under another spelling - measured on 2026-08-27, cleanup
		// removed REPORT_0001.TXT for a manifest listing report_0001.txt and
		// said "1 file removed", while verify on the same directory called that
		// file extra. Two commands, one directory, opposite answers, and the
		// destructive one was the one that assumed.
		//
		// So the name on the disk is read back and compared literally. Nothing
		// changes on a filesystem that keeps the spellings apart, because there
		// the file either is what the manifest says or was never found.
		if actual, known := stored.differing(full); known {
			out = append(out, Outcome{
				Path:    c.Path,
				Blocked: true,
				Reason: fmt.Sprintf("the directory holds this as %q. This filesystem treats the two spellings "+
					"as one file, so removing it here would delete a name the manifest does not list - "+
					"rename it back, or clean up against a manifest written for these names", actual),
			})
			continue
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

// storedNames answers what a directory really stored a name as, reading each
// directory once.
//
// Needed because os.Stat cannot answer it: on Windows and on a default APFS
// volume it finds a file under a spelling the directory does not hold, which is
// the whole point of those filesystems and the whole problem here.
type storedNames struct {
	dirs map[string]map[string]string
}

// differing reports the spelling on the disk when it is not the spelling asked
// for, and whether there is one at all.
//
// A file that is absent, or a directory that cannot be read, returns false:
// this exists to stop a deletion that would take the wrong name, not to invent
// a second way for cleanup to fail. Whatever is really wrong is then reported
// by os.Remove in its own words, which is where it belongs.
func (s storedNames) differing(full string) (string, bool) {
	dir, want := filepath.Split(full)
	names, ok := s.dirs[dir]
	if !ok {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", false
		}
		names = make(map[string]string, len(entries))
		for _, e := range entries {
			names[core.FoldName(e.Name())] = e.Name()
		}
		s.dirs[dir] = names
	}
	actual, found := names[core.FoldName(want)]
	if !found || actual == want {
		return "", false
	}
	return actual, true
}

func skipReason(c Candidate, force bool) string {
	switch c.Disposition {
	case Absent:
		return "it was already gone"
	case Changed:
		return "it has changed since it was written, so it may not be ours. Force the cleanup to remove it anyway"
	case Unreachable:
		if force {
			return "it could not be read, so there is no telling whether it is ours. Forcing the cleanup does not cover this"
		}
		return "it could not be read, so there is no telling whether it is ours"
	default:
		return "it was left alone"
	}
}
