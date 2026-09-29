package core

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// renameNoReplace is renamex_np with RENAME_EXCL: the rename happens only when
// nothing is at the destination, and EEXIST otherwise.
//
// NOT MEASURED on a Mac - there is none to measure on (O153). The flag is
// documented for APFS and HFS+, and a filesystem that does not know it answers
// ENOTSUP, which sends Publish to its fallbacks rather than failing the write.
func renameNoReplace(from, to string) error {
	err := unix.RenamexNp(from, to, unix.RENAME_EXCL)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) {
		return fmt.Errorf("%w: %w", errors.ErrUnsupported, err)
	}
	return err
}

// linkUnsupported says whether a hard link failed because this filesystem has
// none. NOT MEASURED on a Mac.
func linkUnsupported(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) ||
		errors.Is(err, unix.EPERM) ||
		errors.Is(err, unix.ENOTSUP) ||
		errors.Is(err, unix.EOPNOTSUPP)
}
