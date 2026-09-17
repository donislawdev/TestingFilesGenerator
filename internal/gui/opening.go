package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// OpenOrRefuse shows w and runs the toolkit's loop only if the toolkit gave
// it a window. When it did not, the window is tried again and, if that was
// not to be had either, refused: again is asked for a second attempt and
// answers with the exit code of the process that carried it, or with why
// there was none - and then refuse is called with that reason and the
// answer is 1, the exit code a build with no window in it already uses.
//
// Measured on 2026-09-16, on a Windows Server 2025 guest without 3D
// acceleration (O218): the graphics driver there offers no OpenGL, so the
// toolkit cannot create its window. It logs why to a standard error that a
// binary built for the windows subsystem does not have, and then Run waits
// forever - the failure path calls Quit before the loop is marked running,
// and Quit closes nothing then. A double click, nothing on screen, and
// tfg-gui.exe in the task list until the session ends.
//
// The second attempt is a second process, because the toolkit's state after
// a failed window is not something its public API can reset, and because
// the renderer that attempt loads has to be mapped before the toolkit asks
// the driver for anything - see software.go. Measured on the same guest on
// 2026-09-17: the second process opens the window.
//
// ShowAndRun is Show followed by Run and nothing else, so on a machine where
// the window opens this is the same two calls in the same order. The
// question between them is asked of the window's state, not of the
// toolkit's log - see Shown.
func OpenOrRefuse(w fyne.Window, run func(), again func() (int, error), refuse func(why error)) int {
	w.Show()
	if !Shown(w) {
		code, why := again()
		if why == nil {
			return code
		}
		refuse(why)
		return 1
	}
	run()
	return 0
}

// Shown reports whether the toolkit gave w a native window when it was shown.
//
// Asked of the driver's NativeWindow, which answers with a zero handle on
// every platform when the window it wraps was never created - each platform
// file checks for a missing view before filling the handle in. That is a
// public answer about state, and it does not depend on the wording of
// whatever the toolkit logged on the way.
//
// A window whose driver cannot answer is taken as shown: the test driver
// draws without a native window at all, so for it there is nothing to have
// failed. The same goes for a context this does not know how to read, which
// no desktop platform hands out.
func Shown(w fyne.Window) bool {
	native, ok := w.(driver.NativeWindow)
	if !ok {
		return true
	}
	handle, answered := uintptr(0), false
	native.RunNative(func(context any) {
		switch c := context.(type) {
		case driver.WindowsWindowContext:
			handle, answered = c.HWND, true
		case driver.MacWindowContext:
			handle, answered = c.NSWindow, true
		case driver.X11WindowContext:
			handle, answered = c.WindowHandle, true
		case driver.WaylandWindowContext:
			handle, answered = c.WaylandSurface, true
		}
	})
	if !answered {
		return true
	}
	return handle != 0
}

// causeMarker is the word the toolkit's logger puts in front of the error it
// was handed, on its own line after the line that says what failed. Read as
// an anchor and never shown: what comes after it is the toolkit's sentence
// about the driver, which is the one worth repeating to the person.
const causeMarker = "Cause:"

// CauseFrom picks the toolkit's own reason out of what it logged while the
// window was being opened, for the sentence that says the window could not
// be. The decision that it could not is taken elsewhere, from the window's
// state, so this only adds detail: the text after the cause marker, or
// nothing when no line carries one. A toolkit that changes how it logs costs
// the detail and not the refusal.
func CauseFrom(logged string) string {
	for _, line := range strings.Split(logged, "\n") {
		if _, after, found := strings.Cut(line, causeMarker); found {
			return strings.TrimSpace(after)
		}
	}
	return ""
}
