//go:build windows

package gui

import (
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// Asking Windows to draw the title bar dark, as the window under it always is.
//
// Reported by the owner on 2026-11-18 (O360): on a Windows set to light apps
// the window is dark and its title bar white. The toolkit sets the same
// attribute itself, from the desktop's mode for apps - dark only when
// AppsUseLightTheme is 0 (fyne v2.8.2, internal/driver/glfw/window_windows.go,
// setDarkMode and isDark) - when it creates the window and after every change
// of settings. So this is asked once the window is shown and again after each
// change, KeepTitleBarDark.
//
// Measured on Windows Server 2025 with light apps, on 2026-11-18: before this,
// the attribute 0 and the caption (243, 243, 243), the same as Notepad's.
//
// DWMWA_USE_IMMERSIVE_DARK_MODE, a BOOL - four bytes. Microsoft documents it
// from Windows 11 build 22000, Windows 10 takes it from 20H1, and an older
// Windows answers an error and keeps its bar - where this program was before
// any of this existed.
const dwmUseImmersiveDarkMode = 20

// DarkTitleBar asks Windows to draw the title bar of w dark and says whether
// Windows now holds it so. A window with no native handle - not created yet,
// or one of the toolkit's test driver - is left alone and answers false.
func DarkTitleBar(w fyne.Window) bool {
	native, ok := w.(driver.NativeWindow)
	if !ok {
		return false
	}
	dark := false
	native.RunNative(func(context any) {
		if c, ok := context.(driver.WindowsWindowContext); ok {
			dark = DarkenTitleBar(c.HWND)
		}
	})
	return dark
}

// DarkenTitleBar sets the attribute on the window behind hwnd, then reads it
// back, and says whether Windows holds it set. The read is the answer rather
// than the set call's success, because what matters is what Windows keeps.
//
// Exported so a guard can ask it of a real window of its own: the title bar
// is drawn by Windows, outside anything a picture of the canvas can reach.
func DarkenTitleBar(hwnd uintptr) bool {
	on := int32(1)
	set := withDwm(hwnd, "DwmSetWindowAttribute", func(set *syscall.Proc) bool {
		r, _, _ := set.Call(hwnd, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on))
		return r == 0
	})
	return set && TitleBarHeldDark(hwnd)
}

// TitleBarHeldDark says whether Windows holds the title bar of the window
// behind hwnd dark. False when it cannot be asked, so a guard can demand both
// answers of it: false for a window nobody asked, true once asked.
func TitleBarHeldDark(hwnd uintptr) bool {
	held := int32(0)
	read := withDwm(hwnd, "DwmGetWindowAttribute", func(get *syscall.Proc) bool {
		r, _, _ := get.Call(hwnd, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&held)), unsafe.Sizeof(held))
		return r == 0
	})
	return read && held == 1
}

// withDwm finds call in dwmapi.dll, hands it to use, and gives the library
// back. False for a window of none, or when the library or the call is
// missing.
//
// dwmapi.dll by an absolute path under the system directory, for the reason
// darkmenus_windows.go gives for uxtheme.dll: it is not a KnownDLL either (37
// entries, measured 2026-11-18), so a name would be looked for beside the
// program first. Loaded for each call and released after it rather than kept:
// the toolkit holds the same library for as long as the window is open, so
// this moves a count up and down, and a library kept for the process would
// need a guard against two first calls at once.
func withDwm(hwnd uintptr, call string, use func(*syscall.Proc) bool) bool {
	if hwnd == 0 {
		return false
	}
	path, err := systemLibraryPath("dwmapi.dll")
	if err != nil {
		return false
	}
	lib, err := syscall.LoadDLL(path)
	if err != nil {
		return false
	}
	defer func() { _ = lib.Release() }()
	proc, err := lib.FindProc(call)
	if err != nil {
		return false
	}
	return use(proc)
}
