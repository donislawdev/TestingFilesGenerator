package core

import (
	"os"
	"syscall"
)

// MachineMemory is how much memory this machine gives this program - see
// machinememory.go for what the number is used for.
//
// The memory fitted comes from sysinfo, the limit of a container from its
// control group (CgroupMemoryLimit), and the answer is the smaller. A limit
// that cannot be read is an error rather than the memory fitted: in a
// container that is the number the process would be ended for passing, and a
// run told the size of the host is the run R02 was about.
func MachineMemory() (int64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, err
	}
	// The unit is the size of one of the numbers sysinfo counts in. Kernels
	// before 2.3.23 did not set it and counted in bytes.
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	fitted := uint64(info.Totalram) * unit

	limit, limited, err := CgroupMemoryLimit(os.ReadFile)
	if err != nil {
		return 0, err
	}
	if limited {
		return smallest(fitted, uint64(limit)), nil
	}
	return smallest(fitted), nil
}
