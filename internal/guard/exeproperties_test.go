package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// Both binaries say who made them and which version they are.
//
// Windows reads this out of a resource compiled into the exe, and without one
// the Details tab is blank: no description, no version, no author. A build agent
// running a tool nobody can identify is the case this is for - somebody finds
// tfg.exe six months later and has to work out what it is and where it came
// from.
//
// The version is the interesting half. It lives in internal/version, which is
// the one place it exists and the only place the owner sets it, and the resource
// script carries a COPY of it - because a resource script is plain text compiled
// by hand and cannot ask a Go constant. A copy drifts, and the day it drifts the
// program reports one version in its manifest and another in its file
// properties, which is the sort of thing nobody notices until they are trying to
// reproduce a bug from a manifest.
//
// So this reads the compiled resource, not the script. What ships is what is
// asked, which is also what catches a .syso that somebody forgot to regenerate
// after editing the script - the failure that a test reading the .rc would miss
// entirely.
func TestBothBinariesCarryTheirVersionAndAuthor(t *testing.T) {
	for _, binary := range []struct {
		name string
		syso string
	}{
		{name: "tfg.exe", syso: filepath.Join("..", "..", "cmd", "tfg", "rsrc_windows_amd64.syso")},
		{name: "tfg-gui.exe", syso: filepath.Join("..", "..", "cmd", "tfg-gui", "rsrc_windows_amd64.syso")},
	} {
		t.Run(binary.name, func(t *testing.T) {
			raw, err := os.ReadFile(binary.syso)
			if err != nil {
				t.Fatalf("no compiled resource for %s: %v\n"+
					"Build it with windres - the command is in the .rc file beside it.", binary.name, err)
			}

			// A resource holds its strings as UTF-16, so an ordinary search for
			// ASCII finds nothing and would make this guard pass on an empty
			// resource. That is the trap this project has hit before: a check
			// that cannot fail reads exactly like one that never does.
			for what, want := range map[string]string{
				"the version, which has to match internal/version": version.Version,
				"the author":        "DonislawDev",
				"the product name":  "Testing Files Generator",
				"its own file name": binary.name,
			} {
				if !strings.Contains(string(raw), asUTF16(want)) {
					t.Errorf("the resource compiled into %s does not carry %s (%q).\n"+
						"If the script beside it says otherwise, the .syso was not rebuilt after\n"+
						"the script changed - regenerate it with the windres command in the .rc.",
						binary.name, what, want)
				}
			}
		})
	}
}

// asUTF16 is how a Windows resource stores a string, so a search for one finds
// it. Little endian, which is what every system this ships to uses.
func asUTF16(s string) string {
	var b strings.Builder
	for _, unit := range utf16.Encode([]rune(s)) {
		b.WriteByte(byte(unit))
		b.WriteByte(byte(unit >> 8))
	}
	return b.String()
}
