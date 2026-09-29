package core

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// renameNoReplace is renameat2 with RENAME_NOREPLACE: the rename happens only
// when nothing is at the destination, and EEXIST otherwise.
//
// Measured on 2026-09-29 on overlay, tmpfs, ext4, FAT (vfat) and exFAT through
// FUSE, all of them answering it correctly. A filesystem that does not know the
// flag answers EINVAL, and a kernel older than 3.15 has no such call at all.
// Both are "unsupported", which sends Publish to its fallbacks rather than
// failing the write.
func renameNoReplace(from, to string) error {
	err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE)
	if err == nil {
		return nil
	}
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("%w: %w", errors.ErrUnsupported, err)
	}
	return err
}

// linkUnsupported says whether a hard link failed because this filesystem has
// none. FAT and exFAT answer EPERM, measured on 2026-09-29.
func linkUnsupported(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) ||
		errors.Is(err, unix.EPERM) ||
		errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.ENOSYS)
}
