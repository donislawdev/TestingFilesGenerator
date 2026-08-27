package core

import "os"

// writingSuffix marks the half written copy while it is being filled.
//
// Beside the target rather than in the system temporary directory, because a
// rename across volumes is not one operation and the whole point of this is to
// have one.
const writingSuffix = ".tfg-writing"

// ReplaceFile puts new content in place of a file somebody else owns.
//
// This is a different operation from writing one of our own files, and the
// difference is the whole reason it exists separately. Everything else this
// tool writes goes into the output directory under a name it just claimed -
// nobody had it before and nobody else can be holding it. This lands on a file
// that already exists, belongs to a person, and lives in their repository.
//
// Three properties, and all three came from measuring rather than from care.
// The probe is tools/probes/atomic-replace and it was run on Windows and on
// Linux, because the two disagree about two of them.
//
//	IT KEEPS THE MODE. A rename moves the file it renames, mode and all, so
//	the mode that survives is the temporary file's. Measured on Linux: a
//	recipe somebody had made private at 0600 came back at 0644, readable by
//	everyone on the machine. That is the version this tool has shipped.
//
//	IT REFUSES A READ ONLY FILE, ON EVERY SYSTEM. A rename asks for permission
//	on the DIRECTORY, not on the file, so read only protects a recipe on
//	Windows and does not on Linux. Refusing everywhere is the same rule this
//	project applies to file names that only work on one system: a recipe that
//	behaves differently on a colleague's machine is worse than one refused on
//	all of them. Measured: the owner write bit is off in both places, so one
//	question answers it.
//
//	IT LEAVES NOTHING BEHIND. Whatever fails, the half written copy goes.
//
// What it does not do is protect against the file being changed underneath it
// between a read and this call. That belongs to the caller, which is the only
// one that knows when it read.
func ReplaceFile(path string, content []byte) error {
	mode, err := modeToKeep(path)
	if err != nil {
		return err
	}

	tmp := path + writingSuffix
	if err := writeWhole(tmp, content, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return &ReplaceError{Path: path, Err: err}
	}
	return nil
}

// modeToKeep is the mode the replacement has to come back with.
//
// A file that is not there yet is not a replacement at all, so it gets the
// ordinary mode for a new file. A file that is there and cannot be written to
// is refused before anything is created.
func modeToKeep(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return 0o644, nil
	case err != nil:
		return 0, err
	}

	mode := info.Mode().Perm()
	// The owner write bit, which is the one question that means the same thing
	// on both systems. Windows has no permission bits and an attribute
	// instead, and Go turns that attribute into exactly this bit.
	if mode&0o200 == 0 {
		return 0, &ReadOnlyError{Path: path, Mode: mode}
	}
	return mode, nil
}

// writeWhole fills the copy and makes sure it carries the mode it was given.
//
// The mode is set explicitly rather than left to the create call, for two
// reasons that both bite quietly: a create only applies its mode when the file
// is new, so a leftover copy from an interrupted run would keep whatever it
// had, and the process umask takes bits away from a create and not from a
// chmod.
func writeWhole(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return err
	}
	// Before the close, because ReplaceFile renames this copy over a file
	// somebody else wrote - a recipe of theirs, in a repository of theirs. A
	// rename that reaches the disk without the bytes leaves them holding an
	// empty file where their recipe was. One call per command, and the command
	// is "recipe fmt -w". Owner's call on 2026-08-25.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

// ReadOnlyError is refusing to write over a file marked read only.
type ReadOnlyError struct {
	Path string
	Mode os.FileMode
}

func (e *ReadOnlyError) Error() string {
	return "the file is read only, so it was left as it was and nothing was written. " +
		"A file has to be writable to be replaced. " +
		"Make it writable and try again, or write to a different path"
}

// ReplaceError is the rename failing, which is where the interesting failures
// land.
//
// The wording names the likely cause rather than repeating the system's, and
// that is measured rather than polite: on Windows a file that any other
// process has open cannot be renamed onto, and the sentence the system gives
// for it is "Access is denied" - which is true about the call and says nothing
// about what happened or what to do. A recipe is exactly the file somebody has
// open in their editor while they work on it.
type ReplaceError struct {
	Path string
	Err  error
}

func (e *ReplaceError) Error() string {
	return "the file could not be replaced and was left as it was, so nothing was written. " +
		"On Windows this is what happens when another program is holding it open. " +
		"Close it there and try again, or write to a different path"
}

func (e *ReplaceError) Unwrap() error { return e.Err }
