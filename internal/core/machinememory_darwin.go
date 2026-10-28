package core

import (
	"encoding/binary"
	"fmt"
	"syscall"
)

// MachineMemory is how much memory this machine gives this program - see
// machinememory.go for what the number is used for.
//
// hw.memsize is the memory fitted, as eight bytes in the machine's own order.
// macOS has no control groups, and a process limit set with ulimit is not
// read: it is rare, and a Go program under a small one fails long before it
// gets here.
//
// syscall.Sysctl hands the answer back as a string and drops its last byte when
// that byte is nought (syscall_bsd.go in Go 1.27.3, read 2026-10-27), which is
// right for the text answers it was written for and wrong for this one: the
// highest byte of the size of any real machine is nought, so what arrives is
// seven bytes. Put back before it is read, or the read is of a short buffer.
func MachineMemory() (int64, error) {
	answer, err := syscall.Sysctl("hw.memsize")
	if err != nil {
		return 0, err
	}
	raw := []byte(answer)
	// hw.memsize is a uint64_t (sys/sysctl.h of the macOS 26 SDK, read
	// 2026-10-28), so a longer answer can only come from a change in
	// syscall.Sysctl or in the kernel's type - a fault in what this program
	// assumes rather than something a person did. A defect
	// rather than a sentence: a sentence in this file alone would be in the
	// window's catalogue on macOS and missing from it everywhere else.
	if len(raw) > 8 {
		return 0, Defect(fmt.Errorf("hw.memsize came back as %d bytes rather than eight", len(raw)))
	}
	for len(raw) < 8 {
		raw = append(raw, 0)
	}
	return smallest(binary.NativeEndian.Uint64(raw)), nil
}
