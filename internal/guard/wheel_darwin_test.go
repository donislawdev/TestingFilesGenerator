//go:build darwin

package guard

// driverNotch is a delta the toolkit's driver might hand over on macOS. There
// the system's own deltas arrive and the window keeps them (parts/wheel_darwin.go),
// so any delta will do.
const driverNotch = 10

// notchMoves is the delta itself on macOS, whatever the lines - the window
// leaves the wheel as the system gives it there.
func notchMoves(_ int, _, _ float32) float32 { return driverNotch }
