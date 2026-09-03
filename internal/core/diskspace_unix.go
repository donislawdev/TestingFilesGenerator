//go:build !windows

package core

import (
	"path/filepath"
	"syscall"
)

// AvailableBytes reports the free space usable at path.
//
// This tool writes large amounts of data, so running out of disk is its most
// common failure. Finding out at file five thousand of ten thousand is far
// worse than refusing before the first byte.
func AvailableBytes(path string) (int64, error) {
	dir := existingAncestor(path)

	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	// Blocks available to an unprivileged user, which is the number that
	// matters to us rather than the total free on the device. The arithmetic
	// is next door because it is the half that can be wrong on its own, and a
	// check written in here would be one nothing could ever reach.
	return AvailableFrom(uint64(st.Bavail), uint64(st.Bsize)), nil
}

// existingAncestor walks up until it finds a directory that exists, because
// the output directory is often about to be created.
func existingAncestor(path string) string {
	dir, err := filepath.Abs(path)
	if err != nil {
		dir = path
	}
	for {
		if pathExists(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
