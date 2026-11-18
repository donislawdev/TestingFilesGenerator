//go:build !windows

package gui

import "fyne.io/fyne/v2"

// DarkTitleBar leaves the title bar to the system away from Windows and says
// so - the owner's decision of 2026-11-18 (O360). macOS draws it in the
// appearance the system is set to. On Linux it is drawn by the window manager
// or by the client's decorations, and on GNOME 50 the window goes through
// XWayland, whose frames ignored a request for the dark variant - measured in
// docs/TITLEBAR-DARK-2026-11-18.md, section 9.
func DarkTitleBar(fyne.Window) bool { return false }
