//go:build windows

package core

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

// AvailableBytes reports the free space usable at path.
//
// This tool writes large amounts of data, so running out of disk is its most
// common failure. Finding out at file five thousand of ten thousand is far
// worse than refusing before the first byte.
func AvailableBytes(path string) (int64, error) {
	dir := existingAncestor(path)

	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}

	var freeForCaller, total, totalFree uint64

	// A quota can make the space available to this user smaller than the
	// space free on the volume, so the first value is the one that matters.
	r, _, e := getDiskFree.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&freeForCaller)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return 0, e
	}
	return int64(freeForCaller), nil
}

// The handle is looked up once rather than on every call. It was built inside
// AvailableBytes until 2026-08-25, so every check of free space repeated the
// lookup - a run asks once, but nothing says it always will.
//
// syscall rather than golang.org/x/sys/windows, which offers this call ready
// made and would take the unsafe.Pointer out of this file. That module is
// already in the graph as an indirect one, so promoting it would not add a
// download - but it would put it inside the command line binary, which does not
// link it today, and that is untouchable rule 11 rather than tidying.
var (
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	getDiskFree = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// existingAncestor walks up until it finds a directory that exists, because
// the output directory is often about to be created.
func existingAncestor(path string) string {
	dir, err := filepath.Abs(path)
	if err != nil {
		dir = path
	}
	for {
		if _, err := syscall.UTF16PtrFromString(dir); err != nil {
			return dir
		}
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
