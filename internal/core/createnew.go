package core

import (
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
// O_EXCL WAS NOT RELIABLE EVERYWHERE, measured on 2026-08-03 and again on
// 2026-08-25 (O47): on Windows the create reported "the file exists" about a
// file that was not there whenever any part of the path was a symbolic link or
// a junction. This function answered that with a second create, without
// O_EXCL, whenever os.Lstat found nothing - and that second create truncated
// whatever another process put under the name between the two calls, the
// first of the three windows in O252.
//
// It is gone since 2026-09-29, because the compiler moved underneath it.
// Measured that day on Go 1.27.0, the oldest compiler go.mod admits: O_EXCL
// through a junction creates the file and says nothing false. A symbolic link
// could not be measured on this machine, which grants no right to make one,
// and TestADirectoryReachedThroughALinkStillWorks asks exactly that question on
// the runners, which do.
//
// A refusal is still read with os.Lstat, and only to choose its words: a name
// that holds something - even a link pointing at nothing - gets the sentence
// about a name already in use, and any other failure is the create's own.
func CreateNew(path string, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err == nil {
		return f, nil
	}
	if _, lookErr := os.Lstat(path); lookErr == nil {
		return nil, &NameTakenError{Path: path, Err: err}
	}
	return nil, err
}

// OpenOwn opens for writing a file this tool made earlier and closed, and
// refuses when the name holds anything else by now.
//
// The question is asked of the opened file rather than of the name, and that
// order is the point. A name looked at and then opened can be swapped for a
// link in between, and the open follows the link wherever it points - so the
// look proves nothing. The open file is what the bytes would reach, so it is
// the one to ask.
//
// Here beside CreateNew rather than with the writers, because it is the other
// half of the same claim: CreateNew makes the file, and this is the only way
// back into it.
func OpenOwn(path string, own os.FileInfo) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	now, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !sameWrite(own, now) {
		_ = f.Close()
		return nil, &NotOursError{Path: path}
	}
	return f, nil
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
