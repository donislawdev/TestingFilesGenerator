//go:build !windows

package gui

import "fyne.io/fyne/v2"

// DarkTitleBar leaves the title bar to the system away from Windows and says
// so. macOS draws it in the appearance the system is set to, and on Linux it
// is drawn by the window manager or by the client's decorations. Whether this
// program can ask either of them for dark is measured in
// docs/TITLEBAR-DARK-2026-11-18.md, sections 4 and 5.
func DarkTitleBar(fyne.Window) bool { return false }
