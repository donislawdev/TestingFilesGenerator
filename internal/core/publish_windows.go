package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// Two answers Windows gives for "this filesystem cannot do that", by number,
// because the syscall package names neither.
const (
	errInvalidFunction = syscall.Errno(1)  // ERROR_INVALID_FUNCTION
	errNotSupported    = syscall.Errno(50) // ERROR_NOT_SUPPORTED
)

// renameNoReplace is MoveFileW, which moves a file only when nothing is at the
// destination and answers ERROR_ALREADY_EXISTS otherwise - which Go reads as
// fs.ErrExist. os.Rename calls MoveFileEx with MOVEFILE_REPLACE_EXISTING, and
// that flag is the whole difference.
//
// From syscall rather than golang.org/x/sys/windows, and that is measured
// rather than taste: x/sys/windows imports net, so the command line binary
// would have linked a network stack for one call. The guard holding the command
// line away from the network went red on 2026-09-29 the moment it was tried
// (D16, untouchable rule 8).
//
// Measured on 2026-09-29 on NTFS, through a junction and on an exFAT pendrive,
// where a hard link answers "Incorrect function" and this does not.
func renameNoReplace(from, to string) error {
	f, err := syscall.UTF16PtrFromString(extendedLength(from))
	if err != nil {
		return err
	}
	t, err := syscall.UTF16PtrFromString(extendedLength(to))
	if err != nil {
		return err
	}
	err = syscall.MoveFile(f, t)
	if err == nil {
		return nil
	}
	if errors.Is(err, errInvalidFunction) || errors.Is(err, errNotSupported) {
		return fmt.Errorf("%w: %w", errors.ErrUnsupported, err)
	}
	return err
}

// linkUnsupported says whether a hard link failed because this filesystem has
// none. exFAT answers ERROR_INVALID_FUNCTION, measured on 2026-09-29.
func linkUnsupported(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) ||
		errors.Is(err, errInvalidFunction) ||
		errors.Is(err, errNotSupported)
}

// extendedLength writes a long path the way the system takes it past 260
// characters.
//
// Every call in package os does this for us, and this is the one call that
// does not go through os. Without it a name at the length limit (O239) - which
// the tool writes on purpose, and whose temporary name is longer still - would
// be refused here while os.Rename took it. The same threshold Go uses, 248,
// because below it a directory plus an 8.3 name still fits.
func extendedLength(path string) string {
	if len(path) < 248 || strings.HasPrefix(path, `\\?\`) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(abs, `\\`) {
		return `\\?\UNC\` + abs[2:]
	}
	return `\\?\` + abs
}
