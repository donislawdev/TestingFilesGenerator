package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A refusal about one batch marks the boxes of THAT batch.
//
// This is the guard the addressing work was for, and it is the only one that can
// catch the thing that work was protecting against. Two batches means two boxes
// called Size and two called Group name, so a registry keyed on the setting alone
// would hold one of each and a refusal about the second batch would outline the
// first - pointing at a value that is perfectly fine while the wrong one sits
// unmarked below.
//
// Every other guard in this file would stay green through that. The refusal is
// produced, it is placed, a box is outlined, a message appears. Only asking WHICH
// box finds it.
//
// Both directions are checked, and the second is the one no picture could get at:
// the scene helpers find a box by its label, and with two batches that finds one
// of the two arbitrarily. Here the fields are addressed by position, which is the
// vocabulary the recipe package refuses in.
func TestEveryRefusalAboutABatchMarksTheBoxOfThatBatch(t *testing.T) {
	cases := []struct {
		name string
		// fill is the batch that gets a workable id and size, 1 based.
		fill int
		// wantMarked is the batch whose boxes have to carry the refusals.
		wantMarked int
	}{
		{name: "the second batch is the empty one", fill: 1, wantMarked: 2},
		{name: "the first batch is the empty one", fill: 2, wantMarked: 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			screen := window.NewRecipe(&fakeHost{})
			body := screen.Object()
			pressNamed(t, body, text.ButtonAddBatch())

			fields := screen.Fields()
			setBox(t, fields, recipe.TargetAddress(c.fill, recipe.KeyID), "filled")
			setBox(t, fields, recipe.TargetAddress(c.fill, recipe.KeySize), "1kb")

			pressNamed(t, body, text.ButtonPreview())

			// The two settings the empty batch cannot do without. Both have to
			// be marked, because marking one of several bad boxes is its own
			// defect and was reported on 2026-08-18.
			for _, setting := range []string{recipe.KeyID, recipe.KeySize} {
				want := recipe.TargetAddress(c.wantMarked, setting)
				if saying := saidBy(t, fields, want); saying == "" {
					t.Errorf("nothing is marked at %q, so the refusal about the empty batch\n"+
						"has no box and went to the foot of the form.\n%s",
						want, allSaid(fields))
				}

				quiet := recipe.TargetAddress(c.fill, setting)
				if saying := saidBy(t, fields, quiet); saying != "" {
					t.Errorf("%q is marked and that batch was filled in properly.\n"+
						"It says: %s\nA refusal landing on the wrong batch points at a value that\n"+
						"is fine and leaves the wrong one clean.", quiet, saying)
				}
			}
		})
	}
}

// The screen registers a box for every setting the recipe package can refuse.
//
// The other half of the pairing. The addresses agree in shape because both sides
// build them with recipe.TargetAddress, and they agree in vocabulary because both
// use the exported key names - but neither of those says the screen actually
// DRAWS a box for each of them. A setting the reader can refuse and the screen
// has no field for is a refusal that lands at the foot of the form for ever, and
// nothing else here would notice.
//
// The list is the settings this screen undertakes to offer, which is not every
// key a recipe has: the ones this build refuses outright have no box on purpose,
// and neither has the schema version.
func TestTheRecipeScreenDrawsABoxForEverySettingItOffers(t *testing.T) {
	screen := window.NewRecipe(&fakeHost{})
	fields := screen.Fields()

	perBatch := []string{
		recipe.KeyID, recipe.KeyFormat, recipe.KeyCount,
		recipe.KeySize, recipe.KeySizeRange, recipe.KeyBoundary,
		recipe.KeyName, recipe.KeyGroup,
		recipe.KeyExpected, recipe.KeyExpectedReason,
	}
	for _, setting := range perBatch {
		at := recipe.TargetAddress(1, setting)
		if findField(fields, at) == nil {
			t.Errorf("the screen has no box registered at %q, so a refusal naming that\n"+
				"setting would have nowhere to go.", at)
		}
	}

	for _, at := range []string{recipe.KeyOutputDir, recipe.KeyOutputManifest, recipe.KeySeed, recipe.KeyDefaultsLabel} {
		if findField(fields, at) == nil {
			t.Errorf("the screen has no box registered at %q", at)
		}
	}
}

// Adding and removing batches keeps what was typed and renumbers the rest.
//
// The screen registers every field again whenever the list changes, because an
// address carries the position of its batch. That is the correct thing to do and
// it is also the dangerous one: a rebuild that made fresh widgets would empty the
// form, and a rebuild that kept stale addresses would mark the wrong boxes
// afterwards. Both are silent.
//
// The case that matters is removing a batch from the MIDDLE, because that is when
// every address below the removal moves.
func TestAddingAndRemovingBatchesKeepsWhatWasTypedAndRenumbersTheRest(t *testing.T) {
	screen := window.NewRecipe(&fakeHost{})
	body := screen.Object()
	fields := screen.Fields()

	pressNamed(t, body, text.ButtonAddBatch())
	pressNamed(t, body, text.ButtonAddBatch())

	for i, name := range []string{"first", "second", "third"} {
		setBox(t, fields, recipe.TargetAddress(i+1, recipe.KeyID), name)
	}

	// Remove the middle one. Its Remove button is the second of the three.
	buttons := buttonsNamed(body, text.ButtonRemoveBatch())
	if len(buttons) != 3 {
		t.Fatalf("expected a Remove button per batch and found %d", len(buttons))
	}
	buttons[1].OnTapped()

	fields = screen.Fields()
	if got := boxText(t, fields, recipe.TargetAddress(1, recipe.KeyID)); got != "first" {
		t.Errorf("the first batch now says %q and it was typed as \"first\"", got)
	}
	// The third batch became the second, and its value has to have come with it.
	// If the widgets were rebuilt instead of reused this is empty, and if the
	// addresses went stale this still says "second".
	if got := boxText(t, fields, recipe.TargetAddress(2, recipe.KeyID)); got != "third" {
		t.Errorf("after removing the middle batch the second one says %q, and the batch\n"+
			"that moved up was typed as \"third\". Either the widgets were rebuilt and\n"+
			"lost what was in them, or the addresses did not move with them.", got)
	}
	if findField(fields, recipe.TargetAddress(3, recipe.KeyID)) != nil {
		t.Errorf("there is still a box registered for a third batch after one of three\n" +
			"was removed, so a refusal could be placed on a block nobody can see.")
	}
}

// The last batch cannot be removed, because a screen with no batches can produce
// nothing and would answer a press with a refusal about a document rather than
// about anything anybody did.
func TestTheLastBatchCannotBeRemoved(t *testing.T) {
	screen := window.NewRecipe(&fakeHost{})
	body := screen.Object()

	if buttons := buttonsNamed(body, text.ButtonRemoveBatch()); len(buttons) != 0 {
		t.Fatalf("a screen with one batch offers %d ways to remove it, and pressing one\n"+
			"would leave a form that can produce nothing", len(buttons))
	}

	pressNamed(t, body, text.ButtonAddBatch())
	buttons := buttonsNamed(body, text.ButtonRemoveBatch())
	if len(buttons) != 2 {
		t.Fatalf("two batches offer %d Remove buttons", len(buttons))
	}

	// Pressed twice, and the second press is the one that matters. Hiding the
	// button is not the defence - the button vanishes on the rebuild, so a guard
	// that only counted buttons stayed green when the refusal inside removeBatch
	// was taken out. Measured by mutation on 2026-08-18.
	//
	// A held button firing again is not a contrived case either: this reference
	// is exactly what a queued tap or a double press delivers, pointing at a
	// batch that has already gone.
	buttons[0].OnTapped()
	buttons[0].OnTapped()

	if buttons := buttonsNamed(body, text.ButtonRemoveBatch()); len(buttons) != 0 {
		t.Errorf("back to one batch and there are still %d Remove buttons", len(buttons))
	}
	if findField(screen.Fields(), recipe.TargetAddress(1, recipe.KeyID)) == nil {
		t.Errorf("the screen has no batch left at all, so there is nothing to fill in " +
			"and a press of Generate would answer with a refusal about a document " +
			"rather than about anything anybody did.")
	}
}

// findField is the field registered under one address, or nil.
func findField(fields *parts.Fields, at string) *parts.Field {
	for _, f := range fields.All() {
		if f.Setting == at {
			return f
		}
	}
	return nil
}

func setBox(t *testing.T, fields *parts.Fields, at, value string) {
	t.Helper()
	box := entryIn(t, fields, at)
	box.SetText(value)
}

func boxText(t *testing.T, fields *parts.Fields, at string) string {
	t.Helper()
	return entryIn(t, fields, at).Text
}

// entryIn is the box somebody types into under one address.
//
// A walk rather than a cast, for the reason parts.boxesIn gives: a field's
// control is often a container, because a number is held to a width by one and
// the output directory carries a button beside it.
func entryIn(t *testing.T, fields *parts.Fields, at string) *parts.Entry {
	t.Helper()
	f := findField(fields, at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	if e := firstEntryIn(f.Control); e != nil {
		return e
	}
	t.Fatalf("the field at %q holds no box to type into", at)
	return nil
}

func firstEntryIn(o fyne.CanvasObject) *parts.Entry {
	switch it := o.(type) {
	case *parts.Entry:
		return it
	case *fyne.Container:
		for _, child := range it.Objects {
			if e := firstEntryIn(child); e != nil {
				return e
			}
		}
	}
	return nil
}

func saidBy(t *testing.T, fields *parts.Fields, at string) string {
	t.Helper()
	f := findField(fields, at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	return f.Saying()
}

// allSaid is every mark on the screen, for a failure to be readable without a
// debugger.
func allSaid(fields *parts.Fields) string {
	var b strings.Builder
	b.WriteString("What is marked on the screen:")
	found := false
	for _, f := range fields.All() {
		if saying := f.Saying(); saying != "" {
			found = true
			b.WriteString("\n  " + f.Setting + ": " + strings.SplitN(saying, "\n", 2)[0])
		}
	}
	if !found {
		b.WriteString(" nothing at all")
	}
	return b.String()
}

// buttonsNamed is every button carrying one label, in the order they are drawn.
// Several rather than one, because a repeating block has a button per repetition
// and which one was pressed is the whole question.
func buttonsNamed(o fyne.CanvasObject, name string) []*widget.Button {
	var out []*widget.Button
	// walk from window_test.go, which knows to step inside a Card - every field
	// on these screens sits in one, and nothing below it is reachable otherwise.
	walk(o, func(child fyne.CanvasObject) {
		if b, ok := child.(*widget.Button); ok && b.Text == name {
			out = append(out, b)
		}
	})
	return out
}

// What is typed on the recipe screen is what gets written.
//
// The parity guard's bar is deliberately narrow: a capability counts as reachable
// from the window when there is a control on the screen AND a guard presses it
// and finds the value on the other side. Drawing a box that is dropped on the way
// to the engine looks exactly like drawing one that works, so this runs the
// screen for real and reads the manifest off the disk.
//
// It is what lets ten keys move out of notYetReachable in parity_test.go, and
// each of the ten is exercised here on purpose rather than incidentally: several
// batches at once, a class, a declared expectation, a boundary set, a range, an
// archive with files inside, and a manifest under a name somebody chose.
func TestWhatIsTypedOnTheRecipeScreenIsWhatGetsWritten(t *testing.T) {
	dir := t.TempDir()
	host := &fakeHost{}
	screen := window.NewRecipe(host)
	body := screen.Object()

	// Four batches, because the point of this screen is more than one and
	// because the four ways of stating a size need somewhere to live.
	for i := 0; i < 3; i++ {
		pressNamed(t, body, text.ButtonAddBatch())
	}
	pressNamed(t, body, text.ButtonAddContents())

	fields := screen.Fields()
	set := func(position int, setting, value string) {
		setBox(t, fields, recipe.TargetAddress(position, setting), value)
	}
	choose := func(position int, setting, value string) {
		chooserIn(t, fields, recipe.TargetAddress(position, setting)).SetSelected(value)
	}

	// One exact size, twice over, with a class and a declared expectation.
	set(1, recipe.KeyID, "docs")
	choose(1, recipe.KeyFormat, "txt")
	set(1, recipe.KeySize, "1kb")
	set(1, recipe.KeyCount, "2")
	set(1, recipe.KeyGroup, "smoke")
	choose(1, recipe.KeyExpected, "accept")

	// A boundary set: three files, one byte either side of a limit. The way of
	// stating a size is chosen first, since 2026-08-25 - the three ways are a
	// switch with one box under it, and a value typed into a box that is not
	// showing is not sent.
	set(2, recipe.KeyID, "edge")
	choose(2, recipe.KeyFormat, "txt")
	chooseSizeWayIn(t, body, 2, text.SizeWayBoundary())
	set(2, recipe.KeyBoundary, "4kb")

	// A range, so the two files come out different sizes.
	set(3, recipe.KeyID, "vary")
	choose(3, recipe.KeyFormat, "txt")
	chooseSizeWayIn(t, body, 3, text.SizeWayRange())
	set(3, recipe.KeySizeRange, "1kb-4kb")
	set(3, recipe.KeyCount, "2")

	// An archive whose size follows from what it holds.
	set(4, recipe.KeyID, "arch")
	choose(4, recipe.KeyFormat, "zip")
	chooserIn(t, fields, recipe.ContentAddress(4, 1, recipe.KeyFormat)).SetSelected("txt")
	setBox(t, fields, recipe.ContentAddress(4, 1, recipe.KeyCount), "2")
	setBox(t, fields, recipe.ContentAddress(4, 1, recipe.KeySize), "1kb")

	setBox(t, fields, recipe.KeyOutputDir, dir)
	setBox(t, fields, recipe.KeyOutputManifest, "run.json")
	setBox(t, fields, recipe.KeySeed, "4242")
	// On by default, so turning it off is the change worth making - a switch
	// that agrees with the default proves nothing about whether it was read.
	toggleIn(t, fields, recipe.KeyDefaultsLabel).SetChecked(false)

	pressNamed(t, body, "Generate")
	waitForNamedManifest(t, dir, "run.json")
	screen.Stop()

	raw, err := os.ReadFile(filepath.Join(dir, "run.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	recorded := string(raw)

	for field, value := range map[string]string{
		"the class of the first batch": `"group": "smoke"`,
		"the declared expectation":     `"outcome": "accept"`,
		"the label switch":             `"label_embedded": false`,
		"the seed":                     `"seed": 4242`,
		"the archive batch":            `"format": "zip"`,
	} {
		if !strings.Contains(recorded, value) {
			t.Errorf("%s did not reach the manifest - looked for %s", field, value)
		}
	}
	// A hash of the document the screen composed. Without it a run from this
	// screen could not be told apart from one somebody wrote by hand.
	if !strings.Contains(recorded, `"recipe_hash"`) {
		t.Errorf("the manifest carries no recipe_hash, so the document this screen\n" +
			"composed left no trace of itself")
	}

	var got struct {
		Files []struct {
			Name  string `json:"name"`
			Bytes int64  `json:"bytes"`
			Group string `json:"group"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("the manifest is not readable as JSON: %v", err)
	}

	// Grouped by the name, because the id on a manifest entry is the file's own
	// - f_0001 and up, counted across the whole run - and nothing on an entry
	// says which batch produced it. The name carries the batch because the
	// default template is built from its id. Written down here rather than
	// worked around silently: see O97.
	perBatch := map[string]int{}
	sizes := map[string][]int64{}
	for _, f := range got.Files {
		for _, id := range []string{"docs", "edge", "vary", "arch"} {
			if strings.HasPrefix(f.Name, id+"_") {
				perBatch[id]++
				sizes[id] = append(sizes[id], f.Bytes)
			}
		}
	}

	// Every batch reached the run, which is recipe:targets - the list rather
	// than the one target the other screens produce.
	for id, want := range map[string]int{"docs": 2, "edge": 3, "vary": 2, "arch": 1} {
		if perBatch[id] != want {
			t.Errorf("batch %q produced %d files and %d were asked for.\nAll of them: %v",
				id, perBatch[id], want, perBatch)
		}
	}

	// The boundary set is three consecutive sizes around the limit, which is
	// what makes it a boundary set rather than three files.
	if len(sizes["edge"]) == 3 {
		limit := int64(4 * 1024)
		want := []int64{limit - 1, limit, limit + 1}
		for i, size := range sortedSizes(sizes["edge"]) {
			if size != want[i] {
				t.Errorf("the boundary set is %v and %v was asked for around %d B",
					sortedSizes(sizes["edge"]), want, limit)
				break
			}
		}
	}

	// A range gives a size per file rather than one repeated, and both ends are
	// inside what was asked for.
	if len(sizes["vary"]) == 2 {
		for _, size := range sizes["vary"] {
			if size < 1024 || size > 4*1024 {
				t.Errorf("a file of the range batch is %d B, outside 1kb-4kb", size)
			}
		}
	}
}

// sortedSizes is a copy in order, so a failure reads the same way twice.
func sortedSizes(in []int64) []int64 {
	out := append([]int64(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func chooserIn(t *testing.T, fields *parts.Fields, at string) *parts.Chooser {
	t.Helper()
	f := findField(fields, at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	c, ok := f.Control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the field at %q is not a list to choose from", at)
	}
	return c
}

func toggleIn(t *testing.T, fields *parts.Fields, at string) *parts.Toggle {
	t.Helper()
	f := findField(fields, at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	c, ok := f.Control.(*parts.Toggle)
	if !ok {
		t.Fatalf("the field at %q is not a switch", at)
	}
	return c
}

// waitForNamedManifest is waitForManifest for a run that was told what to call
// its record, which is the whole point of the manifest name field.
func waitForNamedManifest(t *testing.T, dir, name string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.Size() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("the run never wrote a manifest called %q. The directory holds: %v", name, namesIn(t, dir))
}
