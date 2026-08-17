package guard

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// What this defends. A screen keeps looking the way somebody last looked at it.
// Every other window guard here asks a question it knew to ask - is the ring
// drawn, is the sentence present, can the button be pressed - and a screen can
// break in a way nobody thought to ask about. A stored picture is the only
// check that fails for a reason it was never told about.
//
// Why it can exist now, and why the comment at the top of window_test.go said
// it could not. That comment gave two reasons. The first was that fonts differ
// between the three systems in the CI matrix, which would make a pixel
// reference a machine for false alarms. Measured on 2026-08-17 and it does not
// hold between Windows and Linux: eleven screen states rendered on both, and
// the about screen - the one that is almost entirely text - came back identical
// to the byte. The full measurement, including what was NOT checked, is in
// tools/probes/pixel-parity.md.
//
// The second reason still stands and is the price of this guard: a toolkit
// update moves pixels. That makes this the same shape as stdlib-golden.json,
// which has to be looked at when Go moves. Regenerating is one command and the
// diff is the review.
//
// Two things are pinned rather than left to the machine, both found by
// measurement rather than by reading:
//
//   - The output directory. It comes from os.Getwd, so the picture depended on
//     where the test was started from - it differed between two runs on ONE
//     machine, not only between systems. Pinning it also keeps a home directory
//     out of a repository that is going to be public, and no other guard would
//     catch that: the private content guard reads text and this is pixels.
//   - Nothing that shows time. The running state was measured as different
//     between two consecutive runs on the same machine, because it photographs
//     a real run mid flight and a progress bar depends on the disk. It is out
//     of this set on purpose, and saying so here is the point - a set that
//     quietly skipped it would read as covering everything.
//
// macOS, measured on the owner's Mac Mini on 2026-08-17 rather than assumed.
// All eleven screens differ there, and the shape of the difference is what
// decided the design: a few thousand pixels spread over the whole window, never
// more than 2 of 255 in any channel. Same glyphs, same layout, same colours -
// the last bit of edge blending rounds the other way.
//
// So the tolerance is on how FAR a channel moved, not on how many pixels moved,
// and it applies only where that rounding was measured. The two worlds are
// eight times apart: narrowing one field by a single pixel - the smallest real
// change this window can make - moves a channel by 16.
//
// What is still NOT proven: whether 2 belongs to macOS or to that one machine
// and that one version of it. One Mac, measured once.
//
// To regenerate after a deliberate change:
//
//	TFG_WRITE_SCREEN_REFERENCE=1 go test ./internal/guard/ -run TestEveryScreenStillDrawsItsStoredPicture

// The size the reference images were measured at. Taller than the window opens,
// so the form is photographed whole rather than cut off at the fold.
const (
	referenceWidth  = 1100
	referenceHeight = 1300
)

// pinnedOutputDirectory replaces the working directory in the picture. Short,
// obviously artificial, and identical on every system - the three properties
// that matter. A real path would carry an account name into a public
// repository and would differ per machine.
//
// tools/probes/guirender pins the same string for its -compare mode.
const pinnedOutputDirectory = "/tfg/out"

// macOSBlendTolerance is how far one channel may move on macOS before this
// stops calling it rounding. Measured at 2 on every screen, so this is double
// that - and still four times below the 16 that the smallest real change makes.
const macOSBlendTolerance = 4

// pixelTolerance is deliberately different per system rather than uniform.
// Windows and Linux were measured at exactly zero, and taking a tolerance there
// would give away accuracy this guard actually has.
func pixelTolerance() int {
	if runtime.GOOS == "darwin" {
		return macOSBlendTolerance
	}
	return 0
}

// scene is one screen in one state, ready to be photographed.
type scene struct {
	tab    fyne.CanvasObject
	whole  fyne.CanvasObject
	canvas fyne.Canvas
}

// screenScene names a picture and says how to get the screen into that state.
//
// set runs before the window is laid out, after runs once the layout is
// settled. That split is not decoration: a popup attaches to the canvas that
// exists when it opens, so a menu opened before the final layout lands on a
// canvas nobody photographs. Measured in the probe on 2026-08-11 and the
// picture came back with no menu in it at all.
type screenScene struct {
	name  string
	tab   string
	set   func(t *testing.T, s scene)
	after func(t *testing.T, s scene)
}

func screenScenes() []screenScene {
	return []screenScene{
		{name: "about", tab: text.TabAbout},
		{name: "generate", tab: text.TabOneTarget},
		{name: "generate-empty", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, text.FieldCount, "0")
			pressNamed(t, s.tab, text.ButtonPreview)
		}},
		{name: "generate-refused", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, text.FieldSize, "1")
			pressNamed(t, s.tab, text.ButtonPreview)
		}},
		{name: "generate-refused-setting", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			chooseFormat(t, s.tab, "png")
			fillField(t, s.tab, "width", "99999")
			pressNamed(t, s.tab, text.ButtonPreview)
		}},
		{name: "generate-chosen", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			chooseFormat(t, s.tab, "png")
			s.canvas.Focus(chooserFor(t, s.tab))
		}},
		{name: "generate-focused", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			s.canvas.Focus(entryUnder(t, s.tab, text.FieldSize))
		}},
		{name: "generate-menu", tab: text.TabOneTarget, after: func(t *testing.T, s scene) {
			chooserFor(t, s.tab).Tapped(&fyne.PointEvent{})
		}},
		{name: "generate-hovered", tab: text.TabOneTarget, after: func(t *testing.T, s scene) {
			explanationBeside(t, s.tab, text.FieldSize).MouseIn(&desktop.MouseEvent{})
		}},
		{name: "preset", tab: text.TabPresets},
		{name: "preset-refused", tab: text.TabPresets, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, "limit", "512")
			pressNamed(t, s.tab, text.ButtonPreview)
		}},
	}
}

func TestEveryScreenStillDrawsItsStoredPicture(t *testing.T) {
	writing := os.Getenv("TFG_WRITE_SCREEN_REFERENCE") != ""
	for _, sc := range screenScenes() {
		t.Run(sc.name, func(t *testing.T) {
			got := renderScene(t, sc)
			reference := filepath.Join("testdata", "screens", sc.name+".png")
			if writing {
				writeImage(t, reference, got)
				t.Logf("wrote %s", reference)
				return
			}
			compareAgainstReference(t, sc.name, reference, got)
		})
	}
}

// renderScene builds the window, puts one screen into one state and returns
// what the canvas holds.
func renderScene(t *testing.T, sc screenScene) image.Image {
	t.Helper()

	app := test.NewApp()
	// The test driver ships a deliberately garish theme so colour changes show
	// up. That is the wrong theme for a picture meant to be what somebody sees,
	// and getting it wrong produces a reference that looks like a defect.
	app.Settings().SetTheme(parts.Theme())
	defer test.NewApp()

	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	tab := selectTab(t, host.content, sc.tab)

	// The window comes before the state rather than after it, so a state that
	// needs a canvas - focus, an open menu - has one to act on.
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	size := fyne.NewSize(referenceWidth, referenceHeight)
	w.Resize(size)

	s := scene{tab: tab, whole: host.content, canvas: w.Canvas()}
	if sc.set != nil {
		sc.set(t, s)
	}
	pinOutputDirectory(tab)

	// Laid out twice on purpose. A wrapping label reports its height for the
	// width it knows about, and on the first pass that is not yet the width it
	// ends up with, so it reserves a line it then does not use. Measured in the
	// probe on 2026-08-11: without the second pass the preset list stood 24, 24
	// and then 43 pixels apart.
	w.Resize(size)
	w.Resize(size)
	host.content.Refresh()

	if sc.after != nil {
		sc.after(t, s)
	}
	return w.Canvas().Capture()
}

// pinOutputDirectory replaces whatever the working directory put in the field.
// Screens without such a field are left alone, which is how the about screen
// passes through here.
func pinOutputDirectory(tab fyne.CanvasObject) {
	control := controlUnder(tab, text.FieldOutputDir)
	if control == nil {
		return
	}
	walk(control, func(o fyne.CanvasObject) {
		if entry, ok := o.(*widget.Entry); ok && entry.Text != pinnedOutputDirectory {
			entry.SetText(pinnedOutputDirectory)
		}
	})
}

func fillField(t *testing.T, o fyne.CanvasObject, label, value string) {
	t.Helper()
	entryUnder(t, o, label).SetText(value)
}

// pressNamed calls the handler rather than tapping through the canvas, and that
// is deliberate here. Whether a button can really be reached is asked by
// TestEveryButtonAPersonCanSeeIsReallyPressable, which taps for real. This one
// is asking what the screen LOOKS like afterwards, so it wants the state and
// not the hit test - and a tap that missed would leave this guard comparing a
// screen in the wrong state against a picture, which reads as a rendering
// defect and is not one.
func pressNamed(t *testing.T, o fyne.CanvasObject, name string) {
	t.Helper()
	b := buttonNamed(o, name)
	if b == nil {
		t.Fatalf("there is no %q button on this screen", name)
	}
	if b.Disabled() {
		t.Fatalf("the %q button is disabled, so this state cannot be reached", name)
	}
	b.OnTapped()
}

func chooserFor(t *testing.T, o fyne.CanvasObject) *parts.Chooser {
	t.Helper()
	chooser, ok := controlUnder(o, text.FieldFormat).(*parts.Chooser)
	if !ok {
		t.Fatal("there is no format menu on this screen")
	}
	return chooser
}

func chooseFormat(t *testing.T, o fyne.CanvasObject, format string) {
	t.Helper()
	chooserFor(t, o).SetSelected(format)
}

// explanationBeside finds the question mark button that sits next to a field.
func explanationBeside(t *testing.T, o fyne.CanvasObject, label string) *parts.DetailButton {
	t.Helper()
	var found *parts.DetailButton
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) != 2 || found != nil {
			return
		}
		head, isLabel := row.Objects[0].(*widget.Label)
		if !isLabel || head.Text != label {
			return
		}
		if b, isButton := row.Objects[1].(*parts.DetailButton); isButton {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("there is no explanation button beside %q", label)
	}
	return found
}

func compareAgainstReference(t *testing.T, name, reference string, got image.Image) {
	t.Helper()

	stored, err := os.ReadFile(reference)
	if err != nil {
		t.Fatalf("there is no stored picture for the %s screen at %s.\n"+
			"Reason: this guard compares against images kept in the repository.\n"+
			"What to do: run it once with TFG_WRITE_SCREEN_REFERENCE=1 and look at what it wrote before committing it.\n"+
			"Underlying error: %v", name, reference, err)
	}
	want, err := png.Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("the stored picture %s could not be read as a PNG: %v", reference, err)
	}

	wantPix, gotPix := toNRGBA(want), toNRGBA(got)
	if wantPix.Rect != gotPix.Rect {
		t.Fatalf("the %s screen rendered %v and the stored picture is %v.\n"+
			"Reason: the window is being laid out at a different size than the reference was taken at.\n"+
			"What to do: this guard renders at %dx%d, so a deliberate change of that constant needs the pictures regenerated.",
			name, gotPix.Rect.Size(), wantPix.Rect.Size(), referenceWidth, referenceHeight)
	}
	if bytes.Equal(wantPix.Pix, gotPix.Pix) {
		return
	}

	differing, worst, box := pixelDifference(wantPix, gotPix)
	if worst <= pixelTolerance() {
		// Said out loud rather than passed in silence. If this number ever
		// starts climbing, the tolerance is covering something it was not
		// measured to cover, and nobody would see that from a green run.
		t.Logf("%d pixels differ by at most %d of 255, which is inside the %d allowed on %s - treated as edge blending",
			differing, worst, pixelTolerance(), runtime.GOOS)
		return
	}
	dir := saveEvidence(t, name, wantPix, gotPix)
	t.Errorf("the %s screen no longer draws its stored picture.\n"+
		"Reason: %d of %d pixels differ, by at most %d of 255 in one channel, inside %v.\n"+
		"What to look at: %s holds the stored picture, what was rendered now, and the two subtracted.\n"+
		"What to do: if the change was meant, regenerate with TFG_WRITE_SCREEN_REFERENCE=1 and commit the new pictures.",
		name, differing, wantPix.Rect.Dx()*wantPix.Rect.Dy(), worst, box, dir)
}

// pixelDifference counts the pixels that differ, the largest distance any one
// channel moved, and the rectangle holding all of it.
//
// Three numbers rather than one, because they answer different questions and
// the difference between them was measured. On macOS on 2026-08-17 every screen
// differed - a few thousand pixels each, spread over the whole rectangle, but
// never by more than 2 of 255. That is rounding in the edge blending of the
// same glyphs, not a different layout. A moved control or a changed colour
// shows up as a large channel distance in a small rectangle, which is the
// opposite shape.
func pixelDifference(want, got *image.NRGBA) (int, int, image.Rectangle) {
	count, worst := 0, 0
	minX, minY := want.Rect.Max.X, want.Rect.Max.Y
	maxX, maxY := want.Rect.Min.X, want.Rect.Min.Y
	for y := want.Rect.Min.Y; y < want.Rect.Max.Y; y++ {
		for x := want.Rect.Min.X; x < want.Rect.Max.X; x++ {
			i := want.PixOffset(x, y)
			if bytes.Equal(want.Pix[i:i+4], got.Pix[i:i+4]) {
				continue
			}
			count++
			for c := 0; c < 4; c++ {
				if d := int(want.Pix[i+c]) - int(got.Pix[i+c]); d > worst {
					worst = d
				} else if -d > worst {
					worst = -d
				}
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x >= maxX {
				maxX = x + 1
			}
			if y >= maxY {
				maxY = y + 1
			}
		}
	}
	if count == 0 {
		return 0, 0, image.Rectangle{}
	}
	return count, worst, image.Rect(minX, minY, maxX, maxY)
}

// saveEvidence writes the three images somebody needs to judge the failure.
// Outside the repository, because a failing run must not leave files in a tree
// this project already fills with generated output.
func saveEvidence(t *testing.T, name string, want, got *image.NRGBA) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tfg-screens")
	if err != nil {
		t.Logf("could not write the images to look at: %v", err)
		return "nowhere - the temporary directory could not be made"
	}
	writeImage(t, filepath.Join(dir, name+"-stored.png"), want)
	writeImage(t, filepath.Join(dir, name+"-now.png"), got)
	writeImage(t, filepath.Join(dir, name+"-difference.png"), subtract(want, got))
	return dir
}

// subtract paints the pixels that differ and leaves the rest black, so the
// change is visible without hunting for it.
func subtract(want, got *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(want.Rect)
	for i := 0; i < len(out.Pix); i += 4 {
		for c := 0; c < 3; c++ {
			d := int(want.Pix[i+c]) - int(got.Pix[i+c])
			if d < 0 {
				d = -d
			}
			out.Pix[i+c] = uint8(d)
		}
		out.Pix[i+3] = 0xFF
	}
	return out
}

func toNRGBA(img image.Image) *image.NRGBA {
	if already, ok := img.(*image.NRGBA); ok {
		return already
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Rect, img, img.Bounds().Min, draw.Src)
	return out
}

func writeImage(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%v", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("%v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("%v", err)
	}
}
