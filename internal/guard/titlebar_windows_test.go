//go:build windows

package guard

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
)

// hiddenWindow is a top-level window of the system's own STATIC class that is
// never shown: no WS_VISIBLE, so nothing appears on the screen and nothing
// takes the keyboard while the guard asks Windows about its title bar.
func hiddenWindow(t *testing.T) uintptr {
	t.Helper()
	// user32.dll is a KnownDLL, so its name is safe to load by.
	user32 := syscall.NewLazyDLL("user32.dll")
	create := user32.NewProc("CreateWindowExW")
	destroy := user32.NewProc("DestroyWindow")
	class, err := syscall.UTF16PtrFromString("STATIC")
	if err != nil {
		t.Fatal(err)
	}
	const overlappedWindow = 0x00CF0000 // WS_OVERLAPPEDWINDOW, without WS_VISIBLE
	hwnd, _, callErr := create.Call(0, uintptr(unsafe.Pointer(class)), 0, overlappedWindow, 0, 0, 200, 100, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("CreateWindowExW made no window: %v", callErr)
	}
	t.Cleanup(func() { _, _, _ = destroy.Call(hwnd) })
	return hwnd
}

// Windows holds the title bar of a real window dark once the program asks.
//
// What is checked is what Windows keeps, read back with
// DwmGetWindowAttribute, and the reading is first made to say no: a window
// nobody asked about has to read as not dark, so a reading that always says
// yes cannot pass. A window of none is refused rather than crashed on. What the
// bar then looks like on a desktop set to light apps needs a window on a
// screen: measured on Windows Server 2025 on 2026-11-18, the caption went from
// (243, 243, 243) to dark (docs/TITLEBAR-DARK-2026-11-18.md).
func TestWindowsHoldsTheTitleBarDarkOnceAsked(t *testing.T) {
	hwnd := hiddenWindow(t)
	if gui.TitleBarHeldDark(hwnd) {
		t.Fatal("a window nobody asked about reads as having a dark title bar, " +
			"so the reading below would prove nothing")
	}
	if !gui.DarkenTitleBar(hwnd) {
		t.Error("Windows did not hold the title bar of a real window dark after the program asked.\n" +
			"Either dwmapi.dll would not load from the system directory, a call is missing, or this " +
			"Windows does not know DWMWA_USE_IMMERSIVE_DARK_MODE (20). The window still works, with a " +
			"title bar that follows the desktop's mode for apps.")
	}
	if !gui.TitleBarHeldDark(hwnd) {
		t.Error("asked and answered dark, but a second reading says the title bar is not dark")
	}
	if gui.DarkenTitleBar(0) {
		t.Error("no window at all answered that its title bar is dark - the call is not reaching Windows")
	}
}
