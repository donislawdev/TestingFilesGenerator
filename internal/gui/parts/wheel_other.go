//go:build !darwin

package parts

// ToolkitNotch is the step the toolkit hands over for one notch of a wheel on
// Windows and Linux: GLFW reports a notch as 1 and the toolkit multiplies it
// by scrollSpeed in fyne internal/driver/glfw/scroll_speed_default.go - 25 in
// v2.8.2. TestTheWheelStepIsTheToolkitsStep reads that file, so a toolkit
// that changes its step turns a guard red rather than every notch silently
// moving a different distance.
const ToolkitNotch = 25

// wheelDistance is the distance a wheel turned by dy should move an area
// showing view units: the notches dy stands for, each the system's distance.
// A touchpad's fraction of a notch stays a fraction.
func wheelDistance(dy, view float32) float32 {
	return dy / ToolkitNotch * notchDistance(view)
}
