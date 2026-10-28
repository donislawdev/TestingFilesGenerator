//go:build !linux && !darwin && !windows

package core

import "errors"

// MachineMemory has no way to ask a system this build does not know, and says
// so rather than guessing. Auto then works to MaxPlanBytes and the run says it
// did - see Memory.On.
func MachineMemory() (int64, error) {
	return 0, errors.ErrUnsupported
}
