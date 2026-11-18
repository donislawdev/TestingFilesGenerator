//go:build darwin

package parts

// wheelDistance leaves the wheel as the toolkit hands it over on macOS. There
// the system's own deltas arrive already scaled and accelerated, the way every
// other program on the machine gets them, and the toolkit multiplies them by
// its scrollSpeed of 10 (fyne internal/driver/glfw/scroll_speed_darwin.go,
// v2.8.2) - so a Mac already scrolls this window as it scrolls anything else.
func wheelDistance(dy, _ float32) float32 { return dy }
