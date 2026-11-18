//go:build windows

package guard

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
)

// What the window asks Windows for a notch of the wheel is what Windows is
// set to. The answer comes through SystemParametersInfoW, and the same setting
// is kept in the user's registry as text, which reg.exe reads without any of
// the window's code - two roads to one number, so a call asked the wrong
// question, or an answer read the wrong way, is told apart from the setting.
//
// "-1" in the registry is a whole screen at a time, which the window answers
// as -1 too.
func TestTheWheelLinesAreWhatWindowsIsSetTo(t *testing.T) {
	// The canary: the reading has to take reg.exe's line apart, or two
	// failures to read would agree with each other.
	if n, ok := wheelLinesIn("    WheelScrollLines    REG_SZ    7\r\n"); !ok || n != 7 {
		t.Fatalf("canary: a setting of 7 was read as %d (%v)", n, ok)
	}

	out, err := exec.Command("reg", "query", `HKCU\Control Panel\Desktop`, "/v", "WheelScrollLines").Output()
	if err != nil {
		t.Fatalf("reading the setting with reg.exe: %v", err)
	}
	set, ok := wheelLinesIn(string(out))
	if !ok {
		t.Fatalf("reg.exe answered without the setting:\n%s", out)
	}
	got, answered := gui.SystemWheelLines()
	if !answered {
		t.Fatal("Windows did not answer how many lines a notch scrolls")
	}
	if got != set {
		t.Errorf("Windows is set to %d line(s) a notch and the window asked it and got %d", set, got)
	}
}

// wheelLinesIn reads the number out of reg.exe's answer.
func wheelLinesIn(out string) (int, bool) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "WheelScrollLines" {
			n, err := strconv.Atoi(fields[2])
			return n, err == nil
		}
	}
	return 0, false
}
