package audit

import (
	"context"
	"io/fs"
	"path/filepath"
)

// EntryKind is what a name found under a directory is, told from the listing
// of the directory rather than by opening the name.
type EntryKind int

// The three kinds. A file is the only one anything here reads.
const (
	// Regular is an ordinary file.
	Regular EntryKind = iota + 1
	// Link is a symbolic link. Never followed: a link can lead out of the
	// directory, or back into it for ever.
	Link
	// Other is a pipe, a device, a socket, or a junction on Windows - which
	// the listing reports as irregular rather than as a link or a directory,
	// measured with go1.27.0 on 2026-09-30. Opening one for reading can wait
	// for ever, so nothing here does.
	Other
)

// Entry is one name found under a directory that is not a directory itself.
type Entry struct {
	// Path is relative to the directory walked and slash separated on every
	// system, the way a manifest and a checksum file both write a path.
	Path string
	Kind EntryKind

	found fs.DirEntry
}

// Info is what the system says about the entry, asked when somebody needs it
// rather than during the walk. Verify never does, and on Linux and macOS each
// answer is one more call to the system per file.
func (e Entry) Info() (fs.FileInfo, error) { return e.found.Info() }

// Found is what a walk of a directory came to.
type Found struct {
	Entries []Entry
	// Unreadable is every directory under the one walked that could not be
	// listed, in the order the walk met them, each error naming its path.
	// The walk goes on past them, so a caller refusing the whole directory can
	// name all of them at once rather than the first one on each try.
	Unreadable []error
}

// Walk lists every name under dir that is not a directory, with what kind of
// thing each one is.
//
// Recursive because the manifest carries a path rather than a bare name, and
// a run that groups its output into folders has to verify the same way. Shared
// by verify and the checksum tools, because "what is in this folder" is one
// question and a checksum file and a manifest have to answer it alike
// (docs/NARZEDZIA-SUMY-2026-09-29.md §3.3).
//
// It takes the context because this is the part with no upper bound: the loop
// over a manifest is as long as the manifest, and this is as long as whatever
// directory somebody pointed at. Until 2026-08-25 only the loop asked, so
// Ctrl+C during the walk of a large tree did nothing until the walk was over.
func Walk(ctx context.Context, dir string) (Found, error) {
	// The root is resolved first, because WalkDir does not follow links and a
	// directory that is itself one would be handed to the callback as a single
	// entry that is not a directory. Found on 2026-08-03 by the guard for
	// generating into a linked directory: verify reported "extra ." and called
	// the whole run a mismatch. People keep fixtures on redirected paths, so
	// this is an ordinary setup rather than a corner.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	var found Found
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			found.Unreadable = append(found.Unreadable, err)
			return pastIt(d)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		found.Entries = append(found.Entries, Entry{Path: filepath.ToSlash(rel), Kind: kindOf(d), found: d})
		return nil
	})
	return found, err
}

// pastIt is how the walk carries on after a directory it could not list: it
// leaves that directory and goes on with the rest. A root that could not be
// looked at has no entry, and then there is nothing to go on with.
func pastIt(d fs.DirEntry) error {
	if d != nil && d.IsDir() {
		return fs.SkipDir
	}
	return nil
}

// kindOf is what the listing says an entry is. Asked of the type bits the
// listing already has, so nothing is opened and nothing is followed.
func kindOf(d fs.DirEntry) EntryKind {
	switch {
	case d.Type().IsRegular():
		return Regular
	case d.Type()&fs.ModeSymlink != 0:
		return Link
	}
	return Other
}

// walk is Walk for verify, which needs only the paths and refuses on the first
// directory it could not list - the same error, in the same place, that it
// gave before the walk learned to go past one.
func walk(ctx context.Context, dir string) ([]string, error) {
	found, err := Walk(ctx, dir)
	if len(found.Unreadable) > 0 {
		return nil, found.Unreadable[0]
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(found.Entries))
	for _, e := range found.Entries {
		out = append(out, e.Path)
	}
	return out, nil
}
