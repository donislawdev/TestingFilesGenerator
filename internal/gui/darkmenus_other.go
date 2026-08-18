//go:build !windows

package gui

// PreferDarkMenus does nothing away from Windows, and says so.
//
// The problem it solves is a Windows one: menus in the non client area are
// drawn by the system there, from a process wide setting the window's own theme
// does not reach. On macOS and Linux the desktop draws those from its own theme
// and there is nothing for an application to ask for.
//
// A stub rather than a build tag at the call site, so run_cgo.go reads the same
// on every system and nobody has to remember which systems it applies to.
func PreferDarkMenus() bool { return false }
