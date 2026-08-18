package guard

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
			chooseWithThePointer(t, s.canvas, s.tab, "png")
		}},
		// The same menu reached the other way. Since 2026-08-18 the two look
		// different on purpose - the pointer moves the keyboard without saying
		// so and the keyboard says so - and a picture of only one of them would
		// leave the rule half looked at.
		{name: "generate-chosen-by-key", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			chooseFormat(t, s.tab, "png")
			s.canvas.Focus(chooserFor(t, s.tab))
		}},
		// The switch with the keyboard in it. The disc behind the square is the
		// toolkit's own mark and it is what the owner called ugly on 2026-08-18
		// - it is still here, and this is the state it is still here in. What
		// changed is that a press no longer produces it, which is what
		// generate-unchecked shows.
		{name: "generate-switch-by-key", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			box := checkNamed(s.tab, text.FieldLabel)
			if box == nil {
				t.Fatalf("there is no switch labelled %q on this screen", text.FieldLabel)
			}
			s.canvas.Focus(box)
		}},
		// A refusal nobody asked for. Typing a value the run cannot use marks
		// the box straight away since 2026-08-18, with no button pressed - the
		// state this window had no picture of because it had no such state.
		{name: "generate-typed", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, text.FieldSize, "abc")
		}},
		// Two bad boxes and two marks. It marked one however many were wrong
		// until 2026-08-18, so this picture is the whole of what was reported.
		{name: "generate-refused-both", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, text.FieldSize, "abc")
			fillField(t, s.tab, text.FieldCount, "many")
			pressNamed(t, s.tab, text.ButtonPreview)
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
		{name: "generate-unchecked", tab: text.TabOneTarget, set: func(t *testing.T, s scene) {
			flipSwitch(t, s.canvas, s.tab, text.FieldLabel)
		}},
		{name: "preset", tab: text.TabPresets},
		{name: "preset-refused", tab: text.TabPresets, set: func(t *testing.T, s scene) {
			fillField(t, s.tab, "limit", "512")
			pressNamed(t, s.tab, text.ButtonPreview)
		}},
		// Both lists on this screen, neither of which had ever been opened by
		// anything in this project. The probe asked every screen for the field
		// called Format, which only the other screen has, so it stopped with
		// "there is no format menu to open" and the state read as one this
		// screen does not have. It has two. docs/UX.md section 7.0 gate 1
		// counts a state nothing can reach as a finding rather than as an
		// absence, and this is what that rule was written for.
		{name: "preset-menu", tab: text.TabPresets, after: func(t *testing.T, s scene) {
			menuUnder(t, s.tab, text.FieldPreset).Tapped(&fyne.PointEvent{})
		}},
		// The list a preset DECLARES, drawn by the same machinery from the same
		// kind of declaration as a format's own settings, and landing somewhere
		// else on the form.
		{name: "preset-menu-setting", tab: text.TabPresets, after: func(t *testing.T, s scene) {
			menuUnder(t, s.tab, "format").Tapped(&fyne.PointEvent{})
		}},
	}
}

func TestEveryScreenStillDrawsItsStoredPicture(t *testing.T) {
	writing := os.Getenv("TFG_WRITE_SCREEN_REFERENCE") != ""
	for _, sc := range screenScenes() {
		t.Run(sc.name, func(t *testing.T) {
			got, markup := renderScene(t, sc)
			picture := filepath.Join("testdata", "screens", sc.name+".png")
			tree := filepath.Join("testdata", "screens", sc.name+".xml")
			if writing {
				writeImage(t, picture, got)
				writeText(t, tree, markup)
				t.Logf("wrote %s and %s", picture, tree)
				return
			}
			// The tree first, and that order is the whole reason it is kept.
			// A picture says THAT something changed and a person has to look to
			// find out what. The tree says it in words, in a diff, and it holds
			// sizes, positions, colours and every string on the screen - so a
			// failure that starts here needs no image viewer at all.
			//
			// It also splits the two kinds of failure apart. Tree changed means
			// the screen was built differently. Tree the same and pixels
			// different means the same screen was drawn differently, which is
			// what macOS does and what a toolkit update would do.
			compareMarkup(t, sc.name, tree, markup)
			compareAgainstReference(t, sc.name, picture, got)
		})
	}
}

// compareMarkup holds the widget tree against the stored one.
//
// Exact, on every system, with no tolerance anywhere - measured on 2026-08-18
// across Windows, Linux and macOS. The markup carries no rasterising, so the
// rounding that forces a tolerance on the pixels cannot reach it.
func compareMarkup(t *testing.T, name, reference, got string) {
	t.Helper()

	stored, err := os.ReadFile(reference)
	if err != nil {
		t.Fatalf("there is no stored tree for the %s screen at %s.\n"+
			"Reason: this guard compares the widget tree as well as the picture.\n"+
			"What to do: run once with TFG_WRITE_SCREEN_REFERENCE=1 and read what it wrote before committing it.\n"+
			"Underlying error: %v", name, reference, err)
	}
	want := strings.ReplaceAll(string(stored), "\r\n", "\n")
	if want == got {
		return
	}

	line, wantLine, gotLine := firstDifference(want, got)
	t.Errorf("the %s screen is no longer built the way the stored tree says.\n"+
		"Reason: line %d differs.\n"+
		"  stored: %s\n"+
		"  now:    %s\n"+
		"What to do: the whole tree is in %s, so git diff shows every change in words. If it was meant, regenerate with TFG_WRITE_SCREEN_REFERENCE=1.",
		name, line, strings.TrimSpace(wantLine), strings.TrimSpace(gotLine), reference)
}

// firstDifference names the line somebody should look at. A diff of 351 lines
// with one changed attribute is unreadable in a test log, and the line number
// is what makes the stored file worth opening.
func firstDifference(want, got string) (int, string, string) {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		w, g := "", ""
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return i + 1, w, g
		}
	}
	return 0, "", ""
}

// renderScene builds the window, puts one screen into one state and returns
// both what the canvas holds and how it was built.
func renderScene(t *testing.T, sc screenScene) (image.Image, string) {
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
	// Both taken from the same canvas in the same state, so they can never
	// describe two different moments.
	return w.Canvas().Capture(), test.RenderToMarkup(w.Canvas())
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

// flipSwitch turns a switch off the way a person does, through the canvas.
//
// Tapping rather than calling SetChecked, and the difference is visible: a tap
// also takes focus - widget.Check.Tapped calls focusIfNotMobile - so a switch
// set in code draws no focus ring and a switch pressed by somebody does. The
// picture would differ from the screen, which is the one thing this guard must
// never do.
//
// The point is inside MinSize rather than the middle of the widget, because
// Check.Tapped ignores anything past its MinSize width and our switches are
// stretched to the width of the column. tools/probes/guirender presses the same
// way for the same reason.
func flipSwitch(t *testing.T, c fyne.Canvas, o fyne.CanvasObject, label string) {
	t.Helper()
	box := checkNamed(o, label)
	if box == nil {
		t.Fatalf("there is no switch labelled %q on this screen", label)
	}
	before := box.Checked

	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(box)
	active := box.MinSize()
	test.TapCanvas(c, at.Add(fyne.NewPos(active.Width/2, box.Size().Height/2)))

	if box.Checked == before {
		t.Fatalf("a press on the switch %q did not change it, so this state was never built.\n"+
			"Reason: it is %gx%g at %v and the press reached something else.\n"+
			"What to do: check whether the switch moved behind another control or off the laid out area.",
			label, box.Size().Width, box.Size().Height, at)
	}
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

// menuUnder is the list under any labelled field, on any screen.
//
// chooserFor asks for the format menu by name and only one screen has one. This
// is what the preset screen's own two lists needed, and not having it is why
// neither had ever been photographed.
func menuUnder(t *testing.T, o fyne.CanvasObject, label string) *parts.Chooser {
	t.Helper()
	chooser, ok := controlUnder(o, label).(*parts.Chooser)
	if !ok {
		t.Fatalf("there is no menu under %q on this screen", label)
	}
	return chooser
}

// chooseWithThePointer picks a value the way somebody with a mouse does.
//
// The press is what matters and not the value. Since 2026-08-18 a press moves
// the keyboard into the control WITHOUT drawing the mark that says so, and
// calling SetSelected on its own never presses anything - so a scene built that
// way photographs the keyboard path and says nothing about the one that was
// reported twice from the screen.
//
// The list is taken away afterwards because that is what happens: the item that
// sets the value is inside the popup, and pressing it closes the popup behind
// itself.
func chooseWithThePointer(t *testing.T, c fyne.Canvas, o fyne.CanvasObject, format string) {
	t.Helper()
	chooser := chooserFor(t, o)
	chooser.Tapped(&fyne.PointEvent{})
	chooser.SetSelected(format)
	if top := c.Overlays().Top(); top != nil {
		c.Overlays().Remove(top)
	}
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

// writeText stores the widget tree with newlines the way the toolkit produced
// them, so a checkout on another system does not read as a change on every
// line. The comparison normalises the same way.
func writeText(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("%v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("%v", err)
	}
}
