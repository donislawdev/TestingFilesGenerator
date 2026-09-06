package core

import (
	"errors"
	"io/fs"
	"os"
)

// CreateNew creates a file under a name nobody is holding, and refuses when
// something already is.
//
// Every file this tool writes goes through here, and there is one reason for
// that rather than a preference for tidiness. A create that is not exclusive
// follows whatever the name points at, so a name somebody else put there first
// decides where the bytes land. Measured on 2026-09-06 against the shipped
// binary, in a scratch directory outside the repository:
//
//	a link at <manifest>.tfg-writing   the manifest landed on the file the
//	                                   link pointed at, outside the output
//	                                   directory, and the run exited 0
//	a link at <recipe>.tfg-writing     "recipe fmt -w" wrote the recipe onto
//	                                   somebody else's file and left the
//	                                   recipe itself as a link, exit 0
//
// Neither name was checked anywhere, because the checks that exist are about
// the file a run produces and the manifest it records - and these two are the
// names those files are written under before they are renamed into place.
//
// THE CHECK CANNOT BE A LOOK BEFORE THE WRITE, and that is what makes this a
// create rather than a question. os.Stat follows a link, so a link pointing at
// nothing answers "there is nothing here" - and a hard link is not a link at
// all as far as any question goes: os.Lstat reports it as an ordinary file,
// because that is what it is. On Windows an ordinary user creates one without
// any privilege, which is measured rather than read. So the only answer that
// holds is the one the operating system settles while it creates the file.
//
// O_EXCL IS NOT RELIABLE EVERYWHERE, and that was measured too, on 2026-08-03
// and again on 2026-08-25. On Windows, Go asks for the reparse point rather
// than for what it points at when O_EXCL is set, and the create then reports
// "the file exists" about a file that is not there whenever any part of the
// path is a symbolic link or a junction. A directory reached through a link is
// an ordinary setup - a redirected workspace, a mounted scratch disk - and this
// tool supports it on purpose.
//
// So a refusal is believed only when something really is there, and the
// question that settles it is os.Lstat rather than os.Stat: a link pointing at
// nothing is a name being taken, whatever it points at. Where O_EXCL works this
// is exactly O_EXCL. Where it lies, this is what the tool did before it, and
// what is left is the window between the two calls - narrow, on that one
// platform, and smaller than the whole of the door it replaces.
func CreateNew(path string, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err == nil {
		return f, nil
	}

	_, lookErr := os.Lstat(path)
	if lookErr == nil {
		// Something is genuinely there. This is the refusal that matters, and
		// it is the one the escapes above went round.
		return nil, &NameTakenError{Path: path, Err: err}
	}
	if !errors.Is(lookErr, fs.ErrNotExist) {
		// A name we cannot ask about is not a name we may write over. Reported
		// as the create failed rather than as the look did, because the create
		// is what the caller asked for.
		return nil, err
	}

	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}

// NameTakenError is refusing to write under a name something else is holding.
//
// It carries the create's own error so that a caller can still ask
// errors.Is(err, fs.ErrExist) and give the refusal its own words - which the
// engine does, because "this run will not write over it" is a better sentence
// about a generated file than anything a general purpose helper could write.
type NameTakenError struct {
	Path string
	Err  error
}

func (e *NameTakenError) Error() string {
	return "the name " + e.Path + " is already in use, so nothing was written. " +
		"This tool writes under a temporary name and renames it into place, and it never writes over a name somebody else holds. " +
		"Remove what is at that name, or work in a directory nothing else is writing to"
}

func (e *NameTakenError) Unwrap() error { return e.Err }
