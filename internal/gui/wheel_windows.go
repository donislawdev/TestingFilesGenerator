//go:build windows

package gui

import (
	"syscall"
	"unsafe"
)

// What SystemParametersInfoW is asked, and the answer that means a whole
// screen at a time rather than a number of lines (WHEEL_PAGESCROLL, UINT_MAX).
const (
	spiGetWheelScrollLines = 0x0068
	wheelPageScroll        = 0xFFFFFFFF
)

// systemParametersInfo is user32.dll by name, allowed for the reason
// dialog_windows.go gives: it is a KnownDLL, so the loader hands back the
// module already mapped without consulting any search order. Lazy, so nothing
// is loaded until the wheel is first turned.
var systemParametersInfo = syscall.NewLazyDLL("user32.dll").NewProc("SystemParametersInfoW")

// SystemWheelLines is how many lines Windows is set to scroll for one notch
// of the mouse wheel - Settings, Bluetooth and devices, Mouse, "Lines to
// scroll at a time" - and -1 when it is set to a whole screen at a time.
// false when the system did not answer, and the window's default stands.
//
// Asked at every notch rather than once, so a change in Settings reaches a
// window that is already open, as it reaches every other program. One call
// into a library already loaded, a few microseconds, against a notch that
// costs a frame to draw.
func SystemWheelLines() (int, bool) {
	var lines uint32
	ok, _, _ := systemParametersInfo.Call(spiGetWheelScrollLines, 0, uintptr(unsafe.Pointer(&lines)), 0)
	switch {
	case ok == 0:
		return 0, false
	case lines == wheelPageScroll:
		return -1, true
	}
	return int(lines), true
}
