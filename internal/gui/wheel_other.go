//go:build !windows

package gui

// SystemWheelLines has nothing to give away from Windows, and says so. Linux
// desktops have no setting for it that a program can ask, and on macOS the
// toolkit's own wheel is kept (parts/wheel_darwin.go), so the window's default
// stands.
//
// A stub rather than a build tag at the call site, so run_cgo.go reads the
// same on every system - the arrangement of PreferDarkMenus.
func SystemWheelLines() (int, bool) { return 0, false }
