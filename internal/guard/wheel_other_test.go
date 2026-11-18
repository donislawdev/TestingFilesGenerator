//go:build !darwin

package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// driverNotch is what the toolkit's driver hands a scrolling area for one
// notch of the wheel on Windows and Linux.
const driverNotch = parts.ToolkitNotch

// notchMoves is how far one notch should move an area showing view units
// with the system set to lines: that many lines of text, a whole view for a
// negative number, nothing for nought.
func notchMoves(lines int, line, view float32) float32 {
	if lines < 0 {
		return view
	}
	return float32(lines) * line
}

// The window divides what the toolkit hands over by the toolkit's step to
// count the notches turned. That step is a constant inside the toolkit, not
// something it exports, so it is read from the toolkit's own source: a
// toolkit that changes it turns this red, where otherwise every notch would
// quietly move a different distance from the one the system is set to.
func TestTheWheelStepIsTheToolkitsStep(t *testing.T) {
	// The canary: the reading has to find a number that is not 25, or it
	// could be agreeing with the constant by never reading anything.
	if n, ok := scrollSpeedIn("const (\n\tscrollSpeed            = float32(31)\n)"); !ok || n != 31 {
		t.Fatalf("canary: a declared step of 31 was read as %v (%v)", n, ok)
	}

	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "fyne.io/fyne/v2").Output()
	if err != nil {
		t.Fatalf("finding the toolkit's source: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(out)), "internal", "driver", "glfw", "scroll_speed_default.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the toolkit's wheel step: %v", err)
	}
	n, ok := scrollSpeedIn(string(src))
	if !ok {
		t.Fatalf("%s no longer declares scrollSpeed, so the step the window divides by is unknown", path)
	}
	if n != parts.ToolkitNotch {
		t.Errorf("the toolkit's wheel step is %v and parts.ToolkitNotch says %v, so a notch moves %.2f times what the system is set to",
			n, parts.ToolkitNotch, n/parts.ToolkitNotch)
	}
}

var scrollSpeedDecl = regexp.MustCompile(`scrollSpeed\s*=\s*float32\(([0-9.]+)\)`)

// scrollSpeedIn reads the step out of the toolkit's declaration.
func scrollSpeedIn(src string) (float64, bool) {
	m := scrollSpeedDecl.FindStringSubmatch(src)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	return n, err == nil
}
