//go:build windows

package core

import "unsafe"

// MachineMemory is how much memory this machine gives this program - see
// machinememory.go for what the number is used for.
//
// GlobalMemoryStatusEx answers two numbers that matter. ullTotalPhys is the
// memory in the machine. ullTotalPageFile is the commit limit "for the system
// or the current process, whichever is smaller" (Microsoft Learn,
// MEMORYSTATUSEX, read 2026-10-27), which is where a limit put on a job this
// process runs in shows up. The address space is the third, and on a 64 bit
// build it is never the smallest - it is taken anyway, because it costs one
// comparison and a 32 bit build would otherwise be told about memory it cannot
// reach.
func MachineMemory() (int64, error) {
	status := memoryStatusEx{}
	// The call refuses a structure whose length it was not told, so this is
	// set before it rather than being a field the system fills in.
	status.length = uint32(unsafe.Sizeof(status))
	if r, _, e := globalMemoryStatus.Call(uintptr(unsafe.Pointer(&status))); r == 0 {
		return 0, e
	}
	return smallest(status.totalPhys, status.totalPageFile, status.totalVirtual), nil
}

// memoryStatusEx is MEMORYSTATUSEX, field for field.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// In kernel32 beside GetDiskFreeSpaceExW, and looked up the same way - see the
// note on kernel32 in diskspace_windows.go for why a lazy lookup of this
// library is safe and of an ordinary one would not be.
var globalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
