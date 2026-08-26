package guard

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The window, rendered without a screen.
//
// Measured on 2026-08-05 with Fyne v2.8.0: the toolkit's test driver builds a
// widget tree and renders it to an image with CGO_ENABLED=0, on a machine with
// no graphics environment and no C compiler. That is what lets the whole CI
// matrix look at a screen, and it is why internal/gui/window and
// internal/gui/parts never import the toolkit's app package - the one that
// does sits alone behind a build tag.
//
// What these guards do NOT do is compare pixels against a stored image. Until
// 2026-08-17 this comment said that could never be done, and half of that was
// wrong. It read: fonts differ between the three systems in the matrix, so a
// pixel golden would be a machine for producing false alarms.
//
// Measured on 2026-08-17 instead of assumed. Eleven screen states rendered on
// Windows and on Linux come back identical, and the about screen - the one that
// is almost entirely text - matches to the byte. What did differ turned out to
// be input rather than rasterising: a working directory shown in a field, and a
// progress bar photographed mid run. Both are pinned or excluded, and the
// comparison now lives in screenpixels_test.go. macOS is still unmeasured, so
// that guard skips there rather than pretending to cover it.
//
// The second half stands and is the price of it: a toolkit update moves pixels,
// so the stored images get regenerated and looked at when Fyne moves.
//
// The two questions these guards ask are still worth asking and stay cheaper:
// does it draw anything at all, and does it say the thing it exists to say.

// fakeHost is a window that records rather than opens.
//
// The screen takes an interface of three methods instead of fyne.Window for
// exactly this: a real window needs a C compiler and a graphics environment,
// and the two behaviours worth proving here - closing during a run, and moving
// between screens - are behaviours of the screen rather than of the toolkit.
type fakeHost struct {
	content   fyne.CanvasObject
	intercept func()
	// waitForWork waits for a preview or a run to finish, without stopping it.
	//
	// Separate from intercept, which is the CLOSE intercept and now cancels.
	// Until 2026-08-26 a preview could not be cancelled, so closing the window
	// was how these guards waited for an answer. Now closing really does
	// cancel, and a guard that closed the window to read a preview would be
	// cancelling the preview it wanted to read.
	waitForWork func()
	closed      int
	picked      string
	asked       int

	opened      string
	openedCount int

	canvas      fyne.Canvas
	folder      string
	folderCount int

	kept *keptInMemory
}

func (h *fakeHost) SetContent(o fyne.CanvasObject) {
	h.content = o
	// Onto the canvas as well, when a guard gave us one. The real host puts the
	// tree in the window, and anything asked about the canvas afterwards - what
	// has the keyboard, which shortcut is registered - is answered against a
	// canvas with nothing on it otherwise.
	if h.canvas != nil {
		h.canvas.SetContent(o)
	}
}
func (h *fakeHost) SetCloseIntercept(fn func()) { h.intercept = fn }

// SetWaitForWork is the optional interface the window offers rather than
// requires - a real window has no use for it and does not implement it.
func (h *fakeHost) SetWaitForWork(fn func()) { h.waitForWork = fn }
func (h *fakeHost) Close()                   { h.closed++ }

// picked is what the stand in answers when a screen asks where the files
// should go, and asked counts how often it was asked. A real picker needs a
// real window, and the behaviour worth proving is that the button reaches one
// and that the answer lands in the field.
// opened is the last address a screen asked to have opened, and openedCount
// how often. Recorded rather than followed: a stand in that really opened a
// browser would put a tab on somebody's screen for every guard that runs, and
// the behaviour worth proving is which address the button was heading for.
func (h *fakeHost) OpenLink(address string) {
	h.opened = address
	h.openedCount++
}

// Canvas is where a shortcut gets registered and where the keyboard starts.
//
// A guard that wants to PRESS a shortcut sets this to the canvas of the window
// it built, before Open, so the registration lands where the press will be
// delivered. Left unset it makes a windowless one, which is enough for every
// guard that only wants the screen tree.
func (h *fakeHost) Canvas() fyne.Canvas {
	if h.canvas == nil {
		h.canvas = test.NewCanvas()
	}
	return h.canvas
}

// OpenFolder records the directory a screen asked to have shown. Recorded
// rather than opened, for the same reason as OpenLink: a stand in that really
// opened one would put a file manager on somebody's screen for every guard.
func (h *fakeHost) OpenFolder(path string) {
	h.folder = path
	h.folderCount++
}

// Remembered is a store in memory, which is the whole reason the screens take
// one through the Host rather than reaching the toolkit's global preferences.
// A guard can say what the window kept without a single byte reaching a disk,
// and there is no file left behind on the machine that ran the suite.
func (h *fakeHost) Remembered() window.Remembered {
	if h.kept == nil {
		h.kept = &keptInMemory{}
	}
	return h.kept
}

type keptInMemory struct {
	dir      string
	size     fyne.Size
	dirWrite int
}

func (k *keptInMemory) Directory() string { return k.dir }
func (k *keptInMemory) RememberDirectory(d string) {
	k.dir = d
	k.dirWrite++
}
func (k *keptInMemory) Size() fyne.Size          { return k.size }
func (k *keptInMemory) RememberSize(s fyne.Size) { k.size = s }

func (h *fakeHost) ChooseDirectory(chosen func(string)) {
	h.asked++
	// Always, including the empty answer that means somebody changed their
	// mind. Staying silent here would make the caller's handling of that answer
	// unreachable from any test.
	chosen(h.picked)
}

// A tree that renders as one flat colour passes every structural check and
// shows nothing. That defect is not hypothetical here: SVG at exactly its
// minimum size rendered as a single colour and passed every other guard in
// this project, which is why its oracle counts colours rather than parsing.
// Same question, same method.
//
// The limit of the method, measured on 2026-08-05 rather than assumed: this
// counts whether anything was drawn, NOT how much. A screen with the licence
// notice and a screen with that notice emptied both render 188 distinct
// colours, because the number is the anti-aliasing palette of the font rather
// than a measure of content. A screen with no sections at all renders 1.
//
// So this guard answers one question and the ones below answer the other.
// Worth writing down, because a colour count reads like a content check and is
// not one.
func TestTheWindowActuallyDrawsSomething(t *testing.T) {
	w := test.NewWindow(window.FirstScreen(&fakeHost{}))
	defer w.Close()
	w.Resize(window.OpenSize)

	img := w.Canvas().Capture()
	bounds := img.Bounds()
	if bounds.Dx() < 100 || bounds.Dy() < 100 {
		t.Fatalf("the canvas came back %dx%d, which is too small to be the window", bounds.Dx(), bounds.Dy())
	}

	colours := distinctColours(img)
	if colours < 3 {
		t.Errorf("the window rendered %d distinct colour(s), so it drew a flat rectangle rather than a screen", colours)
	}
	t.Logf("rendered %dx%d with %d distinct colours, no screen and no C compiler",
		bounds.Dx(), bounds.Dy(), colours)
}

func distinctColours(img image.Image) int {
	seen := map[uint32]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			seen[r>>8<<16|g>>8<<8|bl>>8] = true
		}
	}
	return len(seen)
}

// The sentence the licence screen exists for.
//
// "tfg license" has carried it since 2026-08-04 and somebody using only the
// window had no way to read it - the drift D1 exists to stop, in the one place
// the parity guard cannot see, because a licence is not a capability of the
// engine. docs/GUI.md section 7 names it.
//
// Asserted on the text in the tree rather than on pixels, so it survives a
// font change and fails for the only reason worth failing for: the sentence
// went away.
func TestTheAboutScreenSaysWhoOwnsTheGeneratedFiles(t *testing.T) {
	shown := textIn(window.About(&fakeHost{}))

	for _, want := range []string{
		"General Public License",
		"generate are yours",
		"THIRD-PARTY-NOTICES.md",
		version.Version,
	} {
		if !strings.Contains(shown, want) {
			t.Errorf("the licence screen does not say %q. What it says:\n%s", want, shown)
		}
	}
}

// Both surfaces read one constant, so they cannot come to say different
// things. The window and the command sit on the same layer and cannot import
// each other, which is exactly the shape in which a second copy gets written
// and nobody compares the two again.
func TestTheWindowAndTheCommandQuoteTheSameLicence(t *testing.T) {
	shown := textIn(window.About(&fakeHost{}))
	if !strings.Contains(shown, strings.TrimSpace(version.LicenceNotice)) {
		t.Error("the window does not show the licence notice verbatim, so it is a second copy now")
	}
}

// The screen moved out of the way on 2026-08-05 and staying reachable is the
// whole of what docs/GUI.md section 7 asks. A notice nobody is shown and nobody
// can get to is a notice that is not there, and the difference between those
// two states is one button that anybody could delete without noticing.
func TestTheLicenceIsStillReachableFromTheOpeningScreen(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)

	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	// A tab since 2026-08-11. It used to be a button in the row of actions at
	// the foot of the form, so reaching the licence meant scrolling past every
	// field, and the way back was another button somebody could delete without
	// noticing. A tab is its own way out, so that door is structural now.
	shown := textIn(selectTab(t, host.content, text.TabAbout()))
	if !strings.Contains(shown, "generate are yours") {
		t.Errorf("the About tab does not lead to the licence. What it shows:\n%s", shown)
	}

	if generate := selectTab(t, host.content, text.TabOneTarget()); buttonNamed(generate, "Generate") == nil {
		t.Error("there is no way from the licence back to the work")
	}
}

// The window opens on the work. Decided by the owner on 2026-08-05, against
// keeping the notice as a splash - a screen shown at every start is one nobody
// reads twice, and the reachability above is what the rule actually asks for.
func TestTheWindowOpensOnTheGenerateScreen(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)

	// Asked of the SELECTED tab rather than of the window, and that distinction
	// is the whole guard now: the tabs hold every screen at once, so looking
	// for a Generate button anywhere would find one even if the window opened
	// on the licence.
	tabs := tabsIn(host.content)
	if tabs == nil {
		t.Fatal("the window has no tabs")
	}
	if tabs.Selected() == nil || tabs.Selected().Text != text.TabOneTarget() {
		t.Errorf("the window does not open on the work. Its tabs are %v", tabNames(host.content))
	} else if buttonNamed(tabs.Selected().Content, "Generate") == nil {
		t.Error("the tab the window opens on has no Generate button")
	}
}

// walk visits every object of a tree, through both kinds of grouping this
// window uses. A walker that only knew about containers would stop at the
// scroll the generate screen sits in and report an empty screen.
func walk(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	visit(o)
	switch v := o.(type) {
	case *fyne.Container:
		for _, child := range v.Objects {
			walk(child, visit)
		}
	case *container.Scroll:
		walk(v.Content, visit)
	case *widget.Card:
		// A card is a widget rather than a container, so nothing below it is
		// reachable without this - and every field moved inside one on
		// 2026-08-11. The first symptom was a nil type assertion in a guard
		// that had been passing for weeks.
		walk(v.Content, visit)
	case *container.AppTabs:
		// Every tab, including the ones not on show. A guard that only saw the
		// selected one could not ask whether a screen it is not looking at
		// still holds what it should - and the close intercept has to reach a
		// run on the tab nobody is watching.
		for _, item := range v.Items {
			walk(item.Content, visit)
		}
	case *widget.PopUp:
		// A field's longer explanation opens in one of these.
		walk(v.Content, visit)
	default:
		walkUnknown(o, visit)
	}
}

// walkUnknown reaches into a widget this walk was never told about.
//
// It exists because the same defect had happened four times by 2026-08-12 and
// would keep happening. Every kind of grouping that is a widget rather than a
// container has to be named in the switch above - the scroll, the card, the
// tabs, the popup - and each was added only after a guard had gone QUIET rather
// than red. That is the shape of it: a walk that meets a type it does not know
// stops there and reports an empty tree, and an empty tree makes a guard pass
// while proving nothing. One of them had been green for weeks.
//
// Anything holding a single child in Fyne calls that field Content, and it is
// exported on every one of them - including the container the canvas wraps an
// overlay in, which lives in the toolkit's internal package and therefore
// cannot be named in a case at all. That last one is what turned this from a
// list to keep up to date into something that keeps itself.
//
// It is deliberately narrow. One exported field, one name, one type, no
// recursion into anything else - so a widget that holds children some other way
// is still missed, and the switch above is still where a case belongs when
// somebody notices.
func walkUnknown(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	value := reflect.ValueOf(o)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return
	}
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return
	}
	field := value.FieldByName("Content")
	if !field.IsValid() || !field.CanInterface() {
		return
	}
	if child, ok := field.Interface().(fyne.CanvasObject); ok && child != nil {
		walk(child, visit)
	}
}

// tabsIn is the tab strip of the window, which is where moving between screens
// lives since 2026-08-11.
func tabsIn(o fyne.CanvasObject) *container.AppTabs {
	var found *container.AppTabs
	walk(o, func(obj fyne.CanvasObject) {
		if tabs, ok := obj.(*container.AppTabs); ok && found == nil {
			found = tabs
		}
	})
	return found
}

// tabNamed is one screen of the window, whether or not it is the one on show.
//
// Guards ask for the screen they mean rather than for the window, because the
// tabs hold every screen at once: both work screens have a field called
// "output directory", so a lookup across the whole window finds whichever
// comes first and reads as though it worked.
func tabNamed(t *testing.T, o fyne.CanvasObject, name string) fyne.CanvasObject {
	t.Helper()
	tabs := tabsIn(o)
	if tabs == nil {
		t.Fatal("the window has no tabs")
	}
	for _, item := range tabs.Items {
		if item.Text == name {
			return item.Content
		}
	}
	t.Fatalf("there is no %q tab. The window has: %v", name, tabNames(o))
	return nil
}

// selectTab moves to a screen the way a person does, and returns it. Selecting
// is what carries the output directory across, so pressing is not the same as
// reading the tab's content.
func selectTab(t *testing.T, o fyne.CanvasObject, name string) fyne.CanvasObject {
	t.Helper()
	tabs := tabsIn(o)
	if tabs == nil {
		t.Fatal("the window has no tabs")
	}
	for _, item := range tabs.Items {
		if item.Text == name {
			tabs.Select(item)
			return item.Content
		}
	}
	t.Fatalf("there is no %q tab. The window has: %v", name, tabNames(o))
	return nil
}

func tabNames(o fyne.CanvasObject) []string {
	tabs := tabsIn(o)
	if tabs == nil {
		return nil
	}
	var out []string
	for _, item := range tabs.Items {
		out = append(out, item.Text)
	}
	return out
}

// textIn walks a widget tree and collects everything a person would read.
func textIn(o fyne.CanvasObject) string {
	var b strings.Builder
	walk(o, func(obj fyne.CanvasObject) {
		switch v := obj.(type) {
		case *widget.Label:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *widget.Button:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *parts.Entry:
			b.WriteString(v.Text)
			b.WriteString("\n")
		case *parts.Chooser:
			b.WriteString(v.Selected)
			b.WriteString("\n")
		}
	})
	return b.String()
}

func buttonNamed(o fyne.CanvasObject, name string) *widget.Button {
	var found *widget.Button
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok && b.Text == name {
			found = b
		}
	})
	return found
}

// checkNamed is a switch found by the words on it, which is where a switch
// carries its name - a heading above one leaves a bare square to click.
//
// It looks for parts.Switch rather than widget.Check. The window's switches
// report when the keyboard reaches them, which the toolkit's do not, and a
// type that embeds another is not that other type - so this asks for the one
// the window actually builds instead of matching both and pretending they are
// interchangeable.
func checkNamed(o fyne.CanvasObject, name string) *parts.Toggle {
	var found *parts.Toggle
	walk(o, func(obj fyne.CanvasObject) {
		if c, ok := obj.(*parts.Toggle); ok && c.Text == name {
			found = c
		}
	})
	return found
}

func buttonNames(o fyne.CanvasObject) []string {
	var out []string
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok {
			out = append(out, b.Text)
		}
	})
	return out
}

// controlUnder finds the control of a labelled field.
//
// A field is a heading, a control and a sentence, so the control is the object
// after the heading. Found by the label somebody reads rather than by position
// in the screen, so adding a field above does not silently point a guard at a
// different box.
func controlUnder(o fyne.CanvasObject, label string) fyne.CanvasObject {
	var found fyne.CanvasObject
	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || len(box.Objects) < 2 {
			return
		}
		// The heading row of a field with a longer explanation is itself a
		// label followed by something, so it matches this shape and would
		// answer with its own button. Walk visits a field before the row
		// inside it and the last match wins, so without this every field
		// carrying an explanation reports the button as its control.
		if isHeadingExtra(box.Objects[1]) {
			return
		}
		if head := headingOf(box.Objects[0]); head != nil && head.Text == label {
			found = unringed(box.Objects[1])
		}
	})
	return found
}

// unringed is the control inside the edge that marks it, or the object itself.
//
// Every control that can be refused stands under a line it draws when the run
// says something about it - parts.WithRing - so what sits under a field's name
// is a stack of the control and a rectangle. Unwrapped here, in the one place
// every guard finds a control through, so a mark added to the window does not
// mean an edit in a dozen guards that only wanted the box.
//
// Recognised by shape rather than by a type of ours, and that is deliberate:
// the wrapper has to stay a plain container. Fyne's own painter switches on
// *fyne.Container, so a named type embedding one would be drawn as an empty
// object with its children invisible - which is the same lesson the walk above
// carries, seen from the other side.
func unringed(o fyne.CanvasObject) fyne.CanvasObject {
	// A theme wrapper first, because it says nothing about what the control IS.
	// The label switch went inside one on 2026-08-18 to buy the room between
	// its square and its words, and every lookup that reads a control by type
	// stopped finding it - the explanation guard said the switch had no button
	// beside it, which was untrue and read exactly like a defect.
	if over, wrapped := o.(*container.ThemeOverride); wrapped {
		return unringed(over.Content)
	}
	box, ok := o.(*fyne.Container)
	if !ok {
		return o
	}
	// A width wrapper next, and it is one named shape rather than "a container
	// holding one thing": parts.Sized holds a control to what its value needs,
	// and WithRing puts the edge INSIDE that wrapper so the line goes round the
	// box and not round the empty half of the column. A menu got one of these on
	// 2026-08-25 and every lookup that reads a control by type stopped finding
	// it - the render probe said there was no Format menu to open, which was
	// untrue and read exactly like a defect.
	if len(box.Objects) == 1 {
		if nested, is := box.Objects[0].(*fyne.Container); is {
			box = nested
		}
	}
	if len(box.Objects) != 2 {
		return o
	}
	if _, isEdge := box.Objects[1].(*canvas.Rectangle); !isEdge {
		return o
	}
	// One layer, not all of them. Peeling until nothing peels reached INSIDE a
	// control - the preset run stopped writing a manifest, because a field
	// lookup came back with a piece of a box rather than the box.
	return box.Objects[0]
}

// headingOf is the name of a field, whether or not it has a button beside it.
//
// A heading stopped being a bare label on 2026-08-12: a field with a longer
// explanation carries the button that opens it on the same line, so the first
// thing in a field is a row rather than the words. Every lookup here reads the
// first object of a field, so without this each one silently found nothing -
// and "silently" is right, because the failure arrives as a nil control at the
// point of use rather than as a missing field.
//
// The same shape as the walk above and the same lesson: a tree gains a kind of
// grouping and everything that reads the tree has to be told.
// isDetailButton says whether an object is the control that shows a field's
// longer explanation.
//
// Asked by type rather than by shape. It was "a button with no words" for half
// a day on 2026-08-12, which worked and was a rule with a shelf life: the first
// second icon button anywhere on either screen would have been read as one of
// these, and the failure would have been a guard quietly finding the wrong
// control rather than an error.
func isDetailButton(o fyne.CanvasObject) bool {
	_, ok := o.(*parts.DetailButton)
	return ok
}

// isHeadingExtra says whether an object is one of the things that share a
// field's heading line with its name.
//
// Two of them since 2026-08-24, when the star marking a field that has to be
// filled in joined the button holding the longer explanation. Both are asked
// about together everywhere, because every one of these walks is looking for
// the same thing - a label with the field's CONTROL after it - and a heading
// row is exactly what has to be stepped over to find one.
//
// One helper rather than the same two type assertions in four files. The
// alternative is what this project has now been bitten by three times: a walk
// that recognises a row by what sits at a fixed position in it, and a fourth
// thing added to that row later.
func isHeadingExtra(o fyne.CanvasObject) bool {
	if isDetailButton(o) {
		return true
	}
	_, star := o.(*parts.RequiredMark)
	return star
}

// everythingSaid is every word a screen offers, visible or behind a button.
//
// The line under a field moved behind its button on 2026-08-24, so a guard
// asking "does this screen say what the setting takes" and reading only the
// rendered text would now answer no about a screen that says it perfectly well.
// The question those guards ask is whether the words are THERE and reachable,
// not which of the two places they sit in - so this answers that question and
// the placement is asked about by the guards built for it.
func everythingSaid(o fyne.CanvasObject) string {
	said := textIn(o)
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*parts.DetailButton); ok {
			said += "\n" + b.Explanation()
		}
	})
	return said
}

// detailButtonIn is the explanation button somewhere in a heading row, or nil.
//
// A search rather than row.Objects[1], which is what three separate guards and
// one probe did until 2026-08-24. They were all correct while a heading held at
// most two things, and all four broke the moment it could hold three - so the
// position is not read anywhere any more.
func detailButtonIn(row *fyne.Container) *parts.DetailButton {
	for _, o := range row.Objects {
		if button, ok := o.(*parts.DetailButton); ok {
			return button
		}
	}
	return nil
}

func headingOf(o fyne.CanvasObject) *widget.Label {
	if label, ok := o.(*widget.Label); ok {
		return label
	}
	row, ok := o.(*fyne.Container)
	if !ok || len(row.Objects) == 0 {
		return nil
	}
	label, _ := row.Objects[0].(*widget.Label)
	return label
}

// entryUnder is the box somebody types into for a labelled field.
//
// It looks inside a grouping as well as at the control itself, because a field
// can be a box with something beside it - the output directory is a box and a
// button to browse with - and a guard should not have to know which fields are
// which shape.
func entryUnder(t *testing.T, o fyne.CanvasObject, label string) *parts.Entry {
	t.Helper()
	control := controlUnder(o, label)
	if entry, ok := control.(*parts.Entry); ok {
		return entry
	}
	var found *parts.Entry
	walk(control, func(obj fyne.CanvasObject) {
		if entry, ok := obj.(*parts.Entry); ok && found == nil {
			found = entry
		}
	})
	if found == nil {
		t.Fatalf("the field %q is %T and holds no box to type in", label, control)
	}
	return found
}
