//go:build windows

package gui

import (
	"syscall"
	"unsafe"
)

// The flavour of system dialog: an OK button under an error icon, brought to
// the front, because it is the only thing this process has on screen.
const (
	messageBoxIconError     = 0x00000010
	messageBoxSetForeground = 0x00010000
)

// sayInADialog puts title and body in a system dialog owned by no window,
// for the one moment this program has something to say and no window to say
// it in. A binary built for the windows subsystem has no console either, so
// without this the sentence goes nowhere a person looks.
//
// user32.dll by name, and that is allowed where uxtheme was not: it is a
// KnownDLL - measured in the registry on 2026-09-16, thirty seven entries and
// it is among them - so it is already mapped and the loader hands back the
// module that is there without consulting the search order. The guard over
// this form lists it with the same measurement.
//
// The dialog needs a desktop and answers nought without one. That answer is
// not read: the sentence has already gone to standard error and the exit code
// says the rest, so a session without a desktop loses the picture and nothing
// else.
func sayInADialog(title, body string) {
	messageBox := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	caption, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	text, err := syscall.UTF16PtrFromString(body)
	if err != nil {
		return
	}
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(caption)),
		messageBoxIconError|messageBoxSetForeground)
}
