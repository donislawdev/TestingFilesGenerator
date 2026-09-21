package gui

import "fyne.io/fyne/v2"

// WindowReturning tells the control holding the keyboard that the window is
// coming back to the front, so that the toolkit's next FocusGained is not
// mistaken for the keyboard arriving.
//
// Measured in the pinned toolkit on 2026-09-21: whenever the system gives
// the window the front, the driver calls FocusGained on whatever holds the
// keyboard as if the keyboard had just moved there
// (internal/driver/glfw/window.go, processFocused) - on the very first
// activation too, right after Open has put the keyboard on the first field
// quietly. So the first menu on the first screen opened marked, alone among
// every control in the window, which the owner reported as one menu wearing
// a different colour from the rest. The foreground hook runs just before
// that call, and this is what the real window registers there - see Run.
//
// Its own file, outside the cgo build, so that a guard can call it with a
// canvas of the test driver's and a focused control of ours. The line that
// registers it is behind cgo like the rest of the real window, and a guard
// reads that line out of the source instead - the same split the refusal
// seam has (OpenOrRefuse and TestTheWindowBinaryOpensThroughTheRefusalSeam).
//
// It asks for the method by shape rather than importing parts.Returnable,
// and that is a constraint of the build and not a style: the files of this
// package outside the cgo build must not reach the toolkit's widgets,
// because on darwin without cgo the toolkit's own internal/widget does not
// compile (ci.yml says so at the matrix, measured 2026-08-20) - and the
// guard that builds the window binary with cgo off runs on every system.
// The first version imported parts for the interface and turned the macOS
// job red on 2026-09-21 for exactly that.
//
// A control that cannot be told - a box to type in, whose focused look is
// the toolkit's own and is right to come back with the window - is left
// alone. Nothing holding the keyboard is left alone too.
func WindowReturning(c fyne.Canvas) {
	if c == nil {
		return
	}
	if returning, ok := c.Focused().(interface{ WindowReturning() }); ok {
		returning.WindowReturning()
	}
}
