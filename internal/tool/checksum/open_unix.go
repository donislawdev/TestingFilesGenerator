//go:build !windows

package checksum

import (
	"os"
	"syscall"
)

// openForLooking opens a path for reading without waiting on whatever it
// names.
//
// A pipe opened for reading waits until something writes to it, which may be
// never. O_NONBLOCK makes the open return at once, so what was opened can be
// asked what it is before a byte is read - see openRegular.
func openForLooking(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

// waitAgain takes the flag back off a descriptor that turned out to be a file.
//
// open(2) says the flag has no effect on a regular file, and in the same
// paragraph that a program should not depend on that staying so (man7.org,
// open(2), O_NONBLOCK, read 2026-09-30). A read that came back "try again"
// would end the checksum with an error nobody could act on.
func waitAgain(f *os.File) error {
	raw, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var set error
	if err := raw.Control(func(fd uintptr) { set = syscall.SetNonblock(int(fd), false) }); err != nil {
		return err
	}
	return set
}
