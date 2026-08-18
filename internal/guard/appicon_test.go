package guard

import (
	"bytes"
	"encoding/binary"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/icon"
)

// What this defends. The desktop shows our picture for this program - in the
// taskbar, in the switcher and on the window - rather than the toolkit's own
// logo.
//
// Three things have to hold and each fails on its own, which is why this asks
// three questions instead of checking that a file exists. A file existing is
// the one thing that proves nothing: the picture can be blank, the package can
// be in the tree without being linked, and it can be linked without anybody
// handing it to the toolkit.
//
// The transparent corners are not a detail, they are a decision. The owner
// chose the picture with no plate behind it on 2026-08-13, and a plate is
// exactly the change that would arrive later as "while I was in there".
//
// What this does NOT check. That the picture looks like a chickpea, and that a
// running window shows it. The first is a judgement and it was made by the
// owner looking at tools/appicon.py --sheet. The second needs a window on a
// screen, so this reads the wiring instead - a window nobody opened cannot be
// asked what its icon is.
func TestTheWindowShipsTheIconWeDrew(t *testing.T) {
	decoded, err := png.Decode(bytes.NewReader(icon.PNG))
	if err != nil {
		t.Fatalf("the application icon is not a PNG the standard library can read: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != bounds.Dy() {
		t.Errorf("the icon is %dx%d and an application icon is square - every desktop scales it "+
			"into a square box and a rectangle arrives squashed", bounds.Dx(), bounds.Dy())
	}
	if bounds.Dx() < 128 {
		t.Errorf("the icon is %d px across, which is smaller than the 256 the toolkit asks for. "+
			"Scaling up is the one direction that cannot be done without showing", bounds.Dx())
	}

	opaque, tones := 0, map[uint32]bool{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := decoded.At(x, y).RGBA()
			if a < 0x8000 {
				continue
			}
			opaque++
			tones[r>>11<<22|g>>11<<11|b>>11] = true
		}
	}
	area := bounds.Dx() * bounds.Dy()
	// A shape, rather than an empty file or a filled square. Both of those are
	// what a placeholder looks like, and both would sail past "it decodes".
	if share := float64(opaque) / float64(area); share < 0.15 || share > 0.92 {
		t.Errorf("%.0f%% of the icon is solid, which is a blank file or a filled square rather "+
			"than a drawing of anything", share*100)
	}
	if len(tones) < 8 {
		t.Errorf("the icon uses only %d distinct tones, so it is a flat blob rather than the "+
			"drawing tools/appicon.py makes", len(tones))
	}

	// No plate. Owner's decision, and the corners are where a plate shows up
	// first.
	for _, corner := range []image.Point{
		{X: bounds.Min.X, Y: bounds.Min.Y},
		{X: bounds.Max.X - 1, Y: bounds.Min.Y},
		{X: bounds.Min.X, Y: bounds.Max.Y - 1},
		{X: bounds.Max.X - 1, Y: bounds.Max.Y - 1},
	} {
		if _, _, _, a := decoded.At(corner.X, corner.Y).RGBA(); a != 0 {
			t.Errorf("the corner at %d,%d is not transparent, so the icon has a plate behind it. "+
				"The owner chose it without one on 2026-08-13", corner.X, corner.Y)
		}
	}

	// Linked, not merely present. This is the class that was found on
	// 2026-08-05 when the window shipped without the format registrations and
	// every test importing the code was green - a package in the tree says
	// nothing about a package in the binary, and only the compiler knows.
	const pkg = "github.com/donislawdev/TestingFilesGenerator/internal/gui/icon"
	linked := false
	// Relative to this package, because that is where the test runs from.
	for _, dep := range linkedWithCGO(t, "../../cmd/tfg-gui") {
		if dep == pkg {
			linked = true
			break
		}
	}
	if !linked {
		t.Errorf("the window binary does not link %s, so it ships without the icon "+
			"whatever the file on disk says", pkg)
	}

	// And the other direction, which is the one that goes wrong by accident.
	// The command line has no window and no use for a picture, and the day
	// somebody moves this import out from behind the cgo tag it starts
	// carrying thirty kilobytes it never shows.
	for _, dep := range linkedWithCGO(t, "../../cmd/tfg") {
		if dep == pkg {
			t.Errorf("the command line binary links %s. It has no window to put an icon on, "+
				"so this is weight it carries and never uses", pkg)
		}
	}

	// Handed over. Linking the bytes and never giving them to the toolkit
	// leaves the desktop showing the toolkit's own logo, and nothing above
	// would notice.
	if !handsTheIconToTheToolkit(t) {
		t.Errorf("nothing in internal/gui/run_cgo.go passes icon.PNG to SetIcon, so the toolkit " +
			"keeps its own logo and the picture we ship is never shown")
	}
}

// What this defends. The picture on the TASKBAR button and in File Explorer,
// which on Windows is a different icon from the one the toolkit draws on the
// window - and finding that out cost a real report from a real taskbar.
//
// On 2026-08-13 app.SetIcon was in place and working: the window wore our
// chickpea. The taskbar wore the placeholder Windows draws for a binary with no
// icon resource, because a resource compiled INTO the exe is where that one
// comes from. Every guard was green, because every guard was asking about the
// toolkit.
//
// So there are three committed artefacts for one drawing - the PNG the toolkit
// gets, the ICO Windows wants, and the object file the linker pulls in - and
// three copies of one thing is a staleness problem waiting to happen. Somebody
// redraws the icon, regenerates the PNG, and the taskbar keeps the old picture
// for a year.
//
// The chain is checkable by bytes, which is what makes this worth writing.
// Measured on 2026-08-13: the PNG appears VERBATIM inside the ICO, because the
// 256 px entry of a Pillow written ICO is that PNG, and every image payload in
// the ICO appears verbatim inside the .syso. So one drawing can be followed all
// the way to the linker without decoding anything.
//
// What this does NOT check. That Windows draws it. That needs Windows, a
// taskbar and eyes - tools/probes/exe-icon.ps1 is the instrument, and it
// reported 7 of 7 for the window binary and 0 of 7 for the command line one.
func TestTheTaskbarIconIsTheSameDrawingAsTheWindowIcon(t *testing.T) {
	root := repoRoot(t)
	read := func(parts ...string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
		if err != nil {
			t.Fatalf("reading %s: %v", filepath.Join(parts...), err)
		}
		return body
	}

	drawing := read("internal", "gui", "icon", "chickpea.png")
	ico := read("internal", "gui", "icon", "chickpea.ico")

	// Both binaries, since 2026-08-19. The command line one had no icon at all
	// and Explorer drew it with the blank placeholder, beside a window binary
	// that had one - reported by the owner, and already measured by the probe
	// this guard's comment quotes: 7 of 7 for the window, 0 of 7 for this one.
	//
	// One drawing reached from two resource scripts rather than an .ico copied
	// beside each. Two copies is how the two binaries come to wear different
	// pictures of the same bean, which is the thing nobody checks.
	compiled := map[string][]byte{
		"tfg-gui.exe": read("cmd", "tfg-gui", "rsrc_windows_amd64.syso"),
		"tfg.exe":     read("cmd", "tfg", "rsrc_windows_amd64.syso"),
	}

	// The ICO is a directory of images. Windows picks a size per place it draws
	// one, so the small ones are not decoration - 16 is the taskbar and the
	// title bar, and a missing 16 is scaled down from 256 by somebody else.
	if len(ico) < 22 || ico[0] != 0 || ico[1] != 0 || ico[2] != 1 {
		t.Fatalf("chickpea.ico does not start with an icon directory header")
	}
	images := int(binary.LittleEndian.Uint16(ico[4:6]))
	present := map[int]bool{}
	payloads := make([][]byte, 0, images)
	for i := 0; i < images; i++ {
		entry := 6 + i*16
		if entry+16 > len(ico) {
			t.Fatalf("chickpea.ico claims %d images and is too short for them", images)
		}
		width := int(ico[entry])
		if width == 0 {
			width = 256
		}
		length := int(binary.LittleEndian.Uint32(ico[entry+8 : entry+12]))
		offset := int(binary.LittleEndian.Uint32(ico[entry+12 : entry+16]))
		if offset+length > len(ico) {
			t.Fatalf("the %d px image in chickpea.ico points past the end of the file", width)
		}
		present[width] = true
		payloads = append(payloads, ico[offset:offset+length])
	}
	for _, want := range []int{16, 32, 48, 256} {
		if !present[want] {
			t.Errorf("chickpea.ico carries no %d px image, so Windows scales that size itself "+
				"from whichever one it picks", want)
		}
	}

	// One drawing, not three. This is the assertion that catches a redraw that
	// stopped halfway.
	if !bytes.Contains(ico, drawing) {
		t.Errorf("chickpea.ico does not contain chickpea.png, so the taskbar icon and the window " +
			"icon are different pictures. Regenerate both - see internal/gui/icon/chickpea.rc")
	}

	for binary, resource := range compiled {
		for i, payload := range payloads {
			if !bytes.Contains(resource, payload) {
				t.Errorf("image %d of chickpea.ico is not inside the resource compiled into %s,\n"+
					"so that resource is older than the icon. Rerun windres - the command is in\n"+
					"internal/gui/icon/chickpea.rc for the window and cmd/tfg/tfg.rc for the\n"+
					"command line.", i+1, binary)
			}
		}
	}
}

// handsTheIconToTheToolkit looks for the one call that matters, through the
// parser rather than through the file text - so that a mention of SetIcon in a
// comment explaining the call cannot stand in for the call.
func handsTheIconToTheToolkit(t *testing.T) bool {
	t.Helper()
	path := filepath.Join(repoRoot(t), "internal", "gui", "run_cgo.go")

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	found := false
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "SetIcon" {
			return true
		}
		for _, arg := range call.Args {
			var written bytes.Buffer
			if err := printer.Fprint(&written, fset, arg); err != nil {
				continue
			}
			if strings.Contains(written.String(), "icon.PNG") {
				found = true
			}
		}
		return true
	})
	return found
}
