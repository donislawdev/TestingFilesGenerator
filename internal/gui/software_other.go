//go:build !windows

package gui

import "runtime"

// LoadSoftwareRenderer answers that no renderer ships for this system.
//
// Linux and macOS have their own software OpenGL where a driver is missing
// - Mesa is part of the system on the one, and the other has never offered
// less than the toolkit needs - so nothing is shipped beside the binary
// there, and the flag says so rather than doing nothing in silence.
func LoadSoftwareRenderer(string) error {
	return notShipped{runtime.GOOS}
}
