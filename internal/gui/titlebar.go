package gui

import "fyne.io/fyne/v2"

// KeepTitleBarDark asks for the title bar of w dark again after every change
// of the settings s.
//
// The toolkit answers every change of settings by setting the title bar from
// the desktop's mode for apps (fyne v2.8.2, internal/driver/glfw/loop.go,
// applyThemeToWindow), so a dark bar asked for once turns white again the
// first time the desktop's mode or the scale changes. Through fyne.Do, which
// on the toolkit's loop queues the call behind the change being handled, so
// it lands after the toolkit's own answer whichever listener was added first
// (O360, docs/TITLEBAR-DARK-2026-11-18.md).
func KeepTitleBarDark(s fyne.Settings, w fyne.Window, darken func(fyne.Window) bool) {
	s.AddListener(func(fyne.Settings) { fyne.Do(func() { darken(w) }) })
}

// RunDark is run for a window that is kept dark: the title bar asked for once
// the window is shown and so has one, right before the loop starts, and again
// after every change of settings. Handed to OpenOrRefuse in place of a.Run.
func RunDark(a fyne.App, w fyne.Window) func() {
	KeepTitleBarDark(a.Settings(), w, DarkTitleBar)
	return func() {
		DarkTitleBar(w)
		a.Run()
	}
}
