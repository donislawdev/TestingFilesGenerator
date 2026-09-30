//go:build windows

package checksum

import "os"

// openForLooking is a plain open on Windows. syscall.Open there maps the
// flags it knows and O_NONBLOCK is not one of them (syscall_windows.go, Open,
// read 2026-09-30), so asking for it would change nothing. What was opened is
// still asked what it is before a byte is read, the same as everywhere else -
// see openRegular.
func openForLooking(path string) (*os.File, error) {
	return os.Open(path)
}

// waitAgain has nothing to take back, since nothing was set.
func waitAgain(*os.File) error { return nil }
