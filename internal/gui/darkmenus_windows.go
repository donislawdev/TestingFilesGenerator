//go:build windows

package gui

import "syscall"

// Asking Windows to draw this process's own menus dark.
//
// The window is dark and the menu Windows draws over it was white. Reported
// from use on 2026-08-18, with a picture: right click the title bar and the
// Restore, Move, Size, Minimise, Maximise, Close menu comes up in the light
// theme against a dark application.
//
// That menu is drawn by Windows and not by us, and the attribute that darkens a
// title bar does not reach it. Fyne already sets DWMWA_USE_IMMERSIVE_DARK_MODE
// on the window - measured in the toolkit source at v2.8.0 - and that covers the
// caption and its buttons and nothing else. Menus, standard dialogs and
// scrollbars follow a separate, process wide setting.
//
// That setting is SetPreferredAppMode in uxtheme.dll, and three things about it
// have to be said plainly rather than discovered later by somebody else:
//
//   - It is UNDOCUMENTED. Microsoft has never published it, and what is known
//     about it comes from reverse engineering.
//   - It is not exported by name, so it can only be reached by ORDINAL, which
//     is 135. Checked against a written source on 2026-08-18 rather than
//     remembered.
//   - The residual risk is real and cannot be engineered away here: if a future
//     Windows puts a different function at that ordinal, this calls that
//     function instead. What can be done is done - the call is skipped entirely
//     unless the library loads and the ordinal resolves - but a wrong function
//     behind a right ordinal would not be caught by that.
//
// The alternative was to leave the menu light, and it was a real option: this is
// a cosmetic mismatch on a control most people never open. It was chosen against
// because the program is dark by the owner's decision whatever the desktop is
// set to, and a white menu is the one place that decision visibly breaks.
//
// ForceDark rather than AllowDark, and the difference matters here. AllowDark
// follows the desktop, which is what Fyne already does for the title bar.
// ForceDark is dark regardless, which is what this program is.
const (
	uxthemeSetPreferredAppMode = 135
	preferredAppModeForceDark  = 2
)

// PreferDarkMenus asks for dark menus and says whether the ask reached Windows.
//
// Exported so a guard can ask the question on a real system instead of reading
// the code and believing it. There is no other way to check this one: what it
// changes is drawn by Windows, outside anything this program can photograph.
//
// It returns rather than logs. False is not a failure worth
// telling anybody about at runtime: the menu stays the colour it already was,
// which is exactly where this program was before any of this existed.
func PreferDarkMenus() bool {
	uxtheme := syscall.NewLazyDLL("uxtheme.dll")
	if err := uxtheme.Load(); err != nil {
		return false
	}
	// Deliberately not freed. The setting is process wide and outlives this
	// call, uxtheme is already loaded by any process that draws a window, and
	// unloading a library the toolkit is using to draw would be a far worse
	// thing to get wrong than a handle held for the life of the program.
	//
	// Looked up through kernel32 rather than through a helper that takes an
	// ordinal, because the standard library has no such helper and the one that
	// does is in a module this project does not depend on directly. Adding a
	// dependency to reach one function would be the expensive way round.
	//
	// Passing the ordinal where a name goes is how Win32 has always taken
	// ordinals: GetProcAddress reads its second argument as a number rather
	// than a pointer when the high word is zero, which is what MAKEINTRESOURCE
	// builds. 135 fits in the low word.
	getProcAddress := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcAddress")
	addr, _, _ := getProcAddress.Call(uxtheme.Handle(), uintptr(uxthemeSetPreferredAppMode))
	if addr == 0 {
		return false
	}
	syscall.SyscallN(addr, uintptr(preferredAppModeForceDark))
	return true
}
