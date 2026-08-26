package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/widget"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The generate screen, exercised the way somebody uses it: fields are filled
// in and buttons are pressed.
//
// Nothing here reaches inside the screen. Everything goes through the tree the
// window shows, so a guard that passes says the person in front of it can do
// the thing, which is what D1 counts.

// screen is a generate screen with a window that records rather than opens.
//
// It returns the generate tab rather than the window, and that distinction
// arrived with the tabs on 2026-08-11: the window now holds every screen at
// once, and both work screens have a field called "output directory", so a
// lookup across the whole thing finds whichever comes first and reads as
// though it worked.
func screen(t *testing.T) (*fakeHost, fyne.CanvasObject) {
	t.Helper()
	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	// Nothing this window started may outlive the test that started it. A
	// preview now answers from a worker, and under the test driver fyne.Do
	// runs on the calling goroutine - so a preview still in flight when a test
	// ends is a second goroutine shaping text inside the NEXT test. That is
	// not a theory: it panicked inside the font shaper, in a guard that had
	// nothing to do with previews.
	t.Cleanup(func() { join(host) })
	return host, tabNamed(t, host.content, text.TabOneTarget())
}

// heldScreen is the same screen with a hold on it, for a guard that has to read
// the middle of a run.
//
// Separate from screen rather than always on, because a hold parks the worker
// and every guard that does not look would then have to know to let it go. The
// two that need it say so by asking for this one. See holdDuringRun and O144.
func heldScreen(t *testing.T) (*fakeHost, fyne.CanvasObject, *holdDuringRun) {
	t.Helper()
	hold := newHold()
	host := &fakeHost{hold: hold}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	// Freed before joining, and in that order. A guard that failed before it
	// looked would otherwise leave the worker parked, and the join below would
	// hang the package instead of failing the one test.
	t.Cleanup(func() {
		hold.free()
		join(host)
	})
	return host, tabNamed(t, host.content, text.TabOneTarget()), hold
}

// press finds a button by its label and presses it.
func press(t *testing.T, o fyne.CanvasObject, name string) {
	t.Helper()
	b := buttonNamed(o, name)
	if b == nil {
		t.Fatalf("there is no %q button. The screen has: %v", name, buttonNames(o))
	}
	if b.Disabled() {
		t.Fatalf("the %q button is disabled", name)
	}
	b.OnTapped()
}

// fill puts a value into a labelled field.
func fill(t *testing.T, o fyne.CanvasObject, label, value string) {
	t.Helper()
	entryUnder(t, o, label).SetText(value)
}

// Every format the registry holds is one somebody can pick.
//
// Compared against the registry rather than against a list typed here, for the
// reason the parity guard gives: a list written out is a second place to keep in
// step, and the copy is the one that goes stale - so this would end up agreeing
// with the mistake it exists to catch. A fourteenth format joins this menu on
// the day it is registered.
func TestTheWindowOffersEveryFormatTheRegistryHas(t *testing.T) {
	_, content := screen(t)

	control := controlUnder(content, text.FieldFormat())
	picker, ok := control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the format field is %T rather than a list to choose from", control)
	}

	offered := append([]string{}, picker.Options...)
	sort.Strings(offered)
	want := format.IDs()

	if strings.Join(offered, ",") != strings.Join(want, ",") {
		t.Errorf("the window offers %v and the registry has %v", offered, want)
	}
	if picker.Selected == "" {
		t.Error("no format is chosen when the window opens, so the first press of Generate is a refusal about an empty value")
	}
	t.Logf("%d formats offered, all from the registry", len(offered))
}

// A field per declared setting, drawn from the declaration and nothing else.
//
// This is what declaring properties buys rather than only naming them. The
// registry carries a name, a kind, a range or a closed set, a default and a
// sentence, which is everything a field needs - so a format that gains a
// setting gains its field with no window code, and the wording under it is the
// one "tfg formats" prints rather than a second description of one format.
func TestTheWindowDrawsAFieldForEveryDeclaredProperty(t *testing.T) {
	_, content := screen(t)
	picker := controlUnder(content, text.FieldFormat()).(*parts.Chooser)

	checked := 0
	for _, d := range format.All() {
		picker.SetSelected(d.ID)

		for _, p := range d.Properties {
			control := controlUnder(content, text.SettingLabel(p.Name))
			if control == nil {
				t.Errorf("%s declares %q and the window draws no field for it", d.ID, p.Name)
				continue
			}
			if bad := wrongKindOfControl(p, control); bad != "" {
				t.Errorf("%s.%s is %s", d.ID, p.Name, bad)
			}
			// A closed set says what it takes with its menu rather than in
			// prose, since 2026-08-19 (O105). What it still has to say is what
			// it is FOR - the sentence spelling twenty format names out under
			// a menu offering the same twenty was two lines of duplication on
			// a screen that does not fit as it is.
			want := p.Allowed()
			if p.Kind == format.PropertyChoice {
				want = p.Detail
			}
			if shown := everythingSaid(content); want != "" && !strings.Contains(shown, want) {
				t.Errorf("the field for %s.%s does not say %q", d.ID, p.Name, want)
			}
			if p.Kind == format.PropertyChoice {
				if shown := everythingSaid(content); strings.Contains(shown, p.Allowed()) {
					t.Errorf("the field for %s.%s lists its values in prose (%q) as well as in the "+
						"menu above them", d.ID, p.Name, p.Allowed())
				}
			}
			checked++
		}

		// A setting of the format before it must not survive the change. Left
		// behind, it would be sent with the next run and refused as a key the
		// new format does not have.
		for _, other := range format.All() {
			if other.ID == d.ID {
				continue
			}
			for _, p := range other.Properties {
				if declares(d, p.Name) {
					continue
				}
				if controlUnder(content, text.SettingLabel(p.Name)) != nil {
					t.Errorf("choosing %s left the field %q of %s on screen", d.ID, p.Name, other.ID)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("no property was examined - this guard would pass without checking anything")
	}
	t.Logf("%d declared properties, each with a field the window drew from the declaration", checked)
}

func declares(d format.Descriptor, name string) bool {
	for _, p := range d.Properties {
		if p.Name == name {
			return true
		}
	}
	return false
}

// wrongKindOfControl says when a declaration got a control that cannot express
// it. A closed set drawn as a box to type in is how a value gets misspelled.
func wrongKindOfControl(p format.Property, control fyne.CanvasObject) string {
	// A registered control is sometimes a wrapper. Since 2026-08-20 a box for a
	// number is drawn at a width of its own, and the thing that decides the
	// width is what gets registered - so asking the registered object what type
	// it is answers about the wrapper. The same trap parts.inside exists for,
	// and it reported nine formats as drawing a container where a box belongs.
	control = controlItself(control)
	switch p.Kind {
	case format.PropertyChoice:
		if _, ok := control.(*parts.Chooser); !ok {
			return fmt.Sprintf("a %T rather than a list, so its closed set can be misspelled", control)
		}
	case format.PropertyBool:
		if _, ok := control.(*parts.Toggle); !ok {
			return fmt.Sprintf("a %T rather than a switch", control)
		}
	default:
		if _, ok := control.(*parts.Entry); !ok {
			return fmt.Sprintf("a %T rather than a box to type in", control)
		}
	}
	return ""
}

// controlItself is the widget under whatever was registered, found by looking
// for something that is not a plain container.
func controlItself(o fyne.CanvasObject) fyne.CanvasObject {
	box, is := o.(*fyne.Container)
	if !is {
		return o
	}
	for _, child := range box.Objects {
		if found := controlItself(child); found != nil {
			if _, stillABox := found.(*fyne.Container); !stillABox {
				return found
			}
		}
	}
	return o
}

// G1: the window has no arithmetic of its own, it asks the engine.
//
// Asserted on the sentence rather than on the refusal happening, and that is
// the whole point. A window with its own idea of what a size looks like would
// also refuse this - and would go on to accept or refuse something else the
// command line does not, which is D1 breaking in the way nobody sees. The only
// way to tell the two apart from outside is that the words are the engine's.
func TestTheWindowAsksTheEngineWhetherASizeIsGood(t *testing.T) {
	_, content := screen(t)

	for _, bad := range []string{"10 potatoes", "1e5", "-4", "1.5", ""} {
		fill(t, content, text.FieldSize(), bad)
		press(t, content, "Generate")

		_, want := core.ParseSize(bad)
		if want == nil {
			t.Fatalf("%q was supposed to be a size the engine refuses", bad)
		}
		// Compared without regard to case, since 2026-08-20, and that is a
		// narrow thing to give up rather than a loosening.
		//
		// The window names the box the way the screen names it - the label
		// above it rather than the key a recipe writes - so a refusal about the
		// size begins "Size" where the engine wrote "size". That is the ONE
		// word this window is allowed to change, in a message the engine still
		// wrote every other word of.
		//
		// What this guard is for survives whole. A window with its own idea of
		// what a size looks like would produce a different sentence rather than
		// a differently capitalised one, and would go on to accept or refuse
		// something the command line does not - which is the thing that cannot
		// be seen from outside any other way.
		if shown := errorShown(t, content); !strings.EqualFold(shown, want.Error()) {
			t.Errorf("for size %q the window says\n  %s\nand the engine says\n  %s", bad, shown, want)
		}
	}
}

// G9: the refusal arrives whole, with all four of its parts.
//
// A control that shows one line forces a message carrying one of the four, so
// this is a requirement on the layout rather than on the wording - which is why
// it is checked on what is on screen rather than on what was passed in.
func TestTheWindowShowsTheWholeRefusal(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "1")
	press(t, content, "Generate")
	// The answer comes back from a worker since 2026-08-26, so it is waited for.
	join(host)

	shown := errorShown(t, content)
	// The four parts of a refusal below a format's minimum: what cannot be
	// done, why, what is allowed, and what to do instead.
	for _, part := range []string{"cannot be smaller than", "Requested: 1 B"} {
		if !strings.Contains(shown, part) {
			t.Errorf("the refusal on screen is missing %q. It says:\n%s", part, shown)
		}
	}
	if len(shown) < 80 {
		t.Errorf("the refusal on screen is %d characters, which is too short to carry four parts:\n%s",
			len(shown), shown)
	}

	// And nothing was written on the way to being refused.
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("a refused run left %d thing(s) in the output directory", len(entries))
	}
}

// errorShown is what the error area is currently saying.
func errorShown(t *testing.T, o fyne.CanvasObject) string {
	t.Helper()
	var found string
	walk(o, func(obj fyne.CanvasObject) {
		label, ok := obj.(*widget.Label)
		if ok && label.Importance == widget.DangerImportance && label.Text != "" {
			found = label.Text
		}
	})
	if found == "" {
		t.Fatalf("nothing is shown in the error area. The screen says:\n%s", textIn(o))
	}
	return found
}

// G6: the cost comes before the writing, in that order on the screen.
//
// The one thing a window does better than a command line rather than merely as
// well. --dry-run has to be known about and remembered, and this is on the way
// to the button beside it.
func TestPreviewComesBeforeGenerate(t *testing.T) {
	_, content := screen(t)

	names := buttonNames(content)
	preview, generate := indexOf(names, "Preview"), indexOf(names, "Generate")
	switch {
	case preview < 0:
		t.Fatalf("there is no Preview button. The screen has: %v", names)
	case generate < 0:
		t.Fatalf("there is no Generate button. The screen has: %v", names)
	case preview > generate:
		t.Errorf("Generate comes before Preview on screen: %v", names)
	}
}

func indexOf(all []string, want string) int {
	for i, s := range all {
		if s == want {
			return i
		}
	}
	return -1
}

// Preview says what it would cost and writes nothing at all.
func TestPreviewSaysTheCostAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "4kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, "Preview")
	// A preview crosses to a worker now, so its answer arrives after the press
	// returns. Joined rather than polled: polling reads widgets from this
	// goroutine while the worker writes them, and under the test driver that is
	// two goroutines in the font shaper at once. It panics there, in whatever
	// test happens to be running.
	join(host)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the output directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Preview wrote %d thing(s) into the output directory", len(entries))
	}

	shown := textIn(content)
	for _, want := range []string{"3 files", "12.0 KB", "free"} {
		if !strings.Contains(shown, want) {
			t.Errorf("the preview does not say %q. The screen says:\n%s", want, shown)
		}
	}
}

// The window writes the same files the command line does, and records them.
//
// The manifest is the part worth guarding rather than the bytes: the bytes have
// golden values already and they come from the same engine, while a surface that
// writes files and forgets the record leaves them with nothing able to remove
// them.
func TestGeneratingFromTheWindowWritesTheFilesAndTheManifest(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "2kb")
	fill(t, content, text.FieldCount(), "4")
	press(t, content, "Generate")

	waitForManifest(t, dir)
	join(host)

	names := namesIn(t, dir)
	if len(names) != 5 {
		t.Errorf("the run left %d file(s) and four files plus a manifest is five: %v", len(names), names)
	}
	if !strings.Contains(strings.Join(names, " "), "manifest.json") {
		t.Errorf("no manifest was written. What is there: %v", names)
	}
	if shown := everythingSaid(content); !strings.Contains(shown, "4 files written") {
		t.Errorf("the window does not say what it produced. It says:\n%s", shown)
	}
}

// G7: closing the window during a run is a cancellation and not a kill.
//
// The invariant that the output directory never holds a half written file rests
// on the signal handler in cmd/tfg, and closing a window is not a signal. So
// without this the run carries on with nobody watching it, or the process ends
// somewhere inside a file.
//
// Waiting is half of it and is the half that is easy to leave out. Cancelling
// and closing straight away ends the process while the engine is still winding
// down, so the manifest describing what did get written never lands - and that
// manifest is the only thing cleanup can work from.
func TestClosingTheWindowDuringARunStopsItAndWaitsForIt(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "64kb")
	fill(t, content, text.FieldCount(), "400")
	press(t, content, "Generate")
	waitUntilTheRunHasStarted(t, dir)

	if host.intercept == nil {
		t.Fatal("the window has no close intercept, so closing it does not reach the run at all")
	}
	host.intercept()

	if host.closed != 1 {
		t.Errorf("the window was closed %d time(s) and it should be closed exactly once", host.closed)
	}

	// The wait is what makes this assertable at all. If closing did not wait,
	// the engine would still be writing right now and this count would be a
	// number that changes while it is read.
	names := namesIn(t, dir)
	for _, name := range names {
		if strings.Contains(name, core.PartialMarker) {
			t.Errorf("a half written file was left behind: %s", name)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("no manifest after closing the window mid run: %v", err)
	}
	if info.Size() == 0 {
		t.Error("the manifest is the empty file the run claimed, so closing the window did not wait for it to be written")
	}
	t.Logf("closing the window stopped a 400 file run with %d file(s) on disk and a manifest", len(names)-1)
}

// Every field on the screen reaches the run, and the manifest proves it.
//
// Drawing a field is not offering a capability. A box that is typed into and
// then dropped on the way to the engine looks exactly like one that works, and
// it is the shape D1 breaks in - the parity list would say the window can set a
// seed while the window sets nothing. So each value here is deliberately not
// the default, and each one is looked for on the other side.
func TestWhatIsTypedOnTheScreenIsWhatGetsWritten(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	picker := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
	picker.SetSelected("png")

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "64kb")
	fill(t, content, text.FieldCount(), "2")
	fill(t, content, text.FieldTargetID(), "shot")
	fill(t, content, text.FieldNameTemplate(), "shot_{index:04}.png")
	fill(t, content, text.FieldSeed(), "4242")
	fill(t, content, text.SettingLabel("width"), "64")
	fill(t, content, text.SettingLabel("height"), "64")

	// The label is on by default, so turning it off is the change worth making.
	//
	// Found by the words on the switch rather than by a heading above it. A
	// switch carries its own name since 2026-08-11 - given a heading it arrived
	// as a bare square with nothing to read on the part you click, which is O72.
	label := checkNamed(content, text.FieldLabel())
	if label == nil {
		t.Fatal("there is no self describing label switch on the screen")
	}
	label.SetChecked(false)

	press(t, content, "Generate")
	waitForManifest(t, dir)
	join(host)

	names := namesIn(t, dir)
	want := []string{"manifest.json", "shot_0001.png", "shot_0002.png"}
	if strings.Join(names, " ") != strings.Join(want, " ") {
		t.Errorf("the name template did not reach the run.\n  on disk: %v\n  wanted:  %v", names, want)
	}

	// Read as bytes, because that is what somebody's script reads.
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	recorded := string(raw)

	for field, value := range map[string]string{
		"format":          `"format": "png"`,
		"seed":            `"seed": 4242`,
		"width property":  `"width": 64`,
		"height property": `"height": 64`,
		"label switch":    `"label_embedded": false`,
	} {
		if !strings.Contains(recorded, value) {
			t.Errorf("the %s did not reach the manifest - looked for %s", field, value)
		}
	}

	// Each file exactly the size that was asked for, which is the promise the
	// whole tool stands on and is worth asserting once from this surface too.
	for _, name := range []string{"shot_0001.png", "shot_0002.png"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Size() != 64*1024 {
			t.Errorf("%s is %d B and 64kb was asked for", name, info.Size())
		}
	}
}

// Cancel stops the run, and it is the same stopping that closing the window
// does rather than a second way of doing it.
//
// Nothing pressed this until the coverage of onCancel came back at nought,
// which is worth writing down: the button was on the screen, it was in the
// brief, and every other guard here was green. A control nobody presses is a
// control nobody has checked.
//
// Deterministic without polling, because stopping waits. By the time the press
// returns the engine has wound down, so what is on the disk is not a number
// that changes while it is read.
func TestCancelStopsTheRun(t *testing.T) {
	dir := t.TempDir()
	_, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "64kb")
	fill(t, content, text.FieldCount(), "400")
	press(t, content, "Generate")
	waitUntilTheRunHasStarted(t, dir)

	cancel := buttonNamed(content, "Cancel")
	if cancel == nil {
		t.Fatalf("there is no Cancel button. The screen has: %v", buttonNames(content))
	}
	cancel.OnTapped()

	info, err := os.Stat(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("no manifest after cancelling: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("the manifest is the empty file the run claimed, so cancelling did not wait for the run to wind down")
	}

	// Stopped rather than finished. A cancel that let all four hundred through
	// would leave a full set and a manifest that says the run completed.
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	if !strings.Contains(string(raw), `"complete": false`) {
		t.Error("the manifest says the run completed, so Cancel did not reach it")
	}
	if names := namesIn(t, dir); len(names) >= 401 {
		t.Errorf("Cancel left %d file(s), which is the whole set", len(names)-1)
	}
	if shown := everythingSaid(content); !strings.Contains(shown, "Stopped after") {
		t.Errorf("the window does not say the run was stopped. It says:\n%s", shown)
	}
}

// waitForManifest waits for the record rather than for the widgets.
//
// Watching the disk rather than the screen is deliberate. Under the toolkit's
// test driver fyne.Do runs on the calling goroutine, so reading a widget while
// the run is still going is a data race in the test even though production has
// none - measured on 2026-08-05. The manifest is written by the worker before
// it crosses back, and it is claimed as an empty file at the start of the run,
// so the thing to wait for is a manifest with something in it.
func waitForManifest(t *testing.T, dir string) {
	t.Helper()
	const budget = 10 * time.Second
	started := time.Now()
	deadline := started.Add(budget)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	// What was waited for and what was there instead, because the sentence on
	// its own is not a measurement. O126: this budget has run out three times on
	// 2026-08-25, in three different guards, and only ever inside a full run of
	// the package - each of the three passes on its own in under a third of a
	// second. Nobody knows why, and the first thing anybody will want is the
	// number and the state of the directory rather than another repetition.
	t.Fatalf("the run never wrote a manifest. Waited %s of a %s budget, and %s holds %v",
		time.Since(started).Round(time.Millisecond), budget, dir, whatIsIn(dir))
}

// whatIsIn lists a directory for a failure message, saying why it cannot rather
// than hiding the reason.
func whatIsIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{"unreadable: " + err.Error()}
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			names = append(names, fmt.Sprintf("%s (%d B)", e.Name(), info.Size()))
			continue
		}
		names = append(names, e.Name())
	}
	return names
}

// join brings the worker to an end before anything reads a widget, through the
// same close path a person uses. After it returns the goroutine has finished,
// so what it wrote is visible here and is not being written any more.
// join waits for whatever the window has in flight.
//
// It used to CLOSE the window, because until 2026-08-26 that was the only
// handle there was and closing could not cancel anything. Now it can, and a
// guard that closed the window to read a preview would be cancelling the
// preview - measured that day, three guards went red for exactly that.
//
// The two guards that are really about closing call host.intercept directly,
// which is what they always meant.
func join(host *fakeHost) {
	if host.waitForWork != nil {
		host.waitForWork()
	}
}

func namesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// What this defends. A run can be stopped from the moment it starts, and not a
// moment after.
//
// This reads the source, which is not how this package likes to work. The
// reason is that there is nothing else to ask. The defect is a window between
// two statements: the worker goroutine is started, files begin to be written,
// and the handle that cancels and waits for it is assigned on the next line. A
// window closed inside that gap finds nothing to call, waits for nothing, and
// ends the process somewhere inside a file with no manifest - G7, and the one
// thing the output directory is promised never to hold.
//
// It cannot be reached by running anything. The gap is a few instructions wide
// and no test can be made to land in it reliably, so a guard that tried would
// be green for the same reason a broken one is. The ordering IS the fix, so the
// ordering is what gets asserted.
func TestARunIsStoppableFromTheMomentItStarts(t *testing.T) {
	body := readFile(t, "../gui/window/run.go")

	const signature = "func (r *runner) startRun("
	from := strings.Index(body, signature)
	if from < 0 {
		t.Fatalf("there is no %s in run.go, so this guard is looking at something that has been renamed", signature)
	}
	fn := body[from:]
	if end := strings.Index(fn, "\n}\n"); end > 0 {
		fn = fn[:end]
	}

	handle := strings.Index(fn, "r.stop = func()")
	worker := strings.Index(fn, "go func()")
	switch {
	case handle < 0:
		t.Fatal("startRun no longer assigns r.stop, so nothing would stop a run at all")
	case worker < 0:
		t.Fatal("startRun no longer starts a goroutine, so this guard is reading the wrong function")
	case handle > worker:
		t.Error("startRun starts the worker before it assigns r.stop.\n" +
			"Reason: between those two statements files are being written and Stop has nothing to call, so closing the window there ends the process inside a file with no manifest.\n" +
			"What to do: assign r.stop first. ctx, cancel and done all exist by then, so nothing is gained by waiting.")
	}
}

// waitUntilTheRunHasStarted blocks until the run has put something on the disk.
//
// A guard about interrupting a run has to interrupt a run, and since 2026-08-26
// pressing Generate no longer means one is going: planning moved off the
// interface thread, so for a moment after the press the worker is still working
// out what to write. Cancelling in that moment is answered correctly and
// uselessly - nothing has been written, so there is nothing to record and no
// manifest, which is the rule Result.Started states.
//
// The manifest claims its name before the first file, so anything at all
// appearing here means the writing has begun.
func waitUntilTheRunHasStarted(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("nothing was written within twenty seconds, so there was never a run to interrupt")
}
