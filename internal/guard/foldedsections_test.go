package guard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The sections inside a batch, folded away since 2026-08-25.
//
// Two of them: what the chosen format declares, and the notes that describe the
// case rather than the files. Both arrive shut, which is the owner's decision of
// that day, and both are inside a batch that can be shut as well - so a box in
// one of them is two floors down from the screen.
//
// That second floor is the whole reason these guards exist. Folding was allowed
// onto this screen on one condition, recorded on 2026-08-18 and answered on
// 2026-08-25: a refusal names the box it is about, and a box nobody can see is a
// screen that refuses to run while marking nothing - which reads as a button
// that did nothing. Opening the batch and leaving the section shut would be that
// defect again, one floor lower.

// buriedFields is every registered setting of a screen whose control is inside
// something that is not shown, by the address a refusal names it by.
//
// Visible() answers for one object and not for its ancestors, so a child of a
// hidden container reports itself visible - truthfully and uselessly. See
// underSomethingHidden, which is what this is asked through.
func buriedFields(root fyne.CanvasObject, fields *parts.Fields) map[string]bool {
	buried := underSomethingHidden(root)
	out := map[string]bool{}
	for _, f := range fields.All() {
		if buried[f.Control] {
			out[f.Setting] = true
		}
	}
	return out
}

// TestARefusalOpensTheSectionTheBoxIsIn is the guard that lets a batch have
// folded sections inside it at all.
//
// TestARefusalOpensTheBatchItIsAbout holds the floor above. This one holds the
// floor below, and it is a separate guard rather than a case of that one because
// the code answering it is separate: the batch fold was found by matching the
// address against the batch it belongs to, and a section is not addressed at all
// - it is asked whether the control is anywhere inside it.
func TestARefusalOpensTheSectionTheBoxIsIn(t *testing.T) {
	// The setting is asked for by the address the engine refuses it by, and the
	// two screens address the same declaration differently: a batch carries its
	// position, one target on its own does not.
	t.Run("the batch screen", func(t *testing.T) {
		host := newFakeHost(t)
		screen := window.NewRecipe(host)
		body := screen.Object()
		fields := screen.Fields()
		width := recipe.TargetAddress(1, recipe.KeyProperties+".width")

		chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("bmp")
		setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "pictures")
		setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "40kb")
		setBox(t, fields, width, "not a number")

		refusalOpensTheBoxItIsAbout(t, host, body, fields, width)
	})

	t.Run("the single batch screen", func(t *testing.T) {
		host := newFakeHost(t)
		screen := window.NewGenerate(host)
		body := screen.Object()
		fields := screen.Fields()

		chooserIn(t, fields, "format").SetSelected("bmp")
		setBox(t, fields, "width", "not a number")

		refusalOpensTheBoxItIsAbout(t, host, body, fields, "width")
	})
}

func refusalOpensTheBoxItIsAbout(t *testing.T, host *fakeHost, body fyne.CanvasObject,
	fields *parts.Fields, setting string) {
	t.Helper()

	// Asserted rather than assumed. If a later change opens these sections by
	// default, this guard would press a button, find the box on the screen and
	// report that a refusal opened it - while proving nothing at all. That is
	// O118, and it has cost this package six guards already.
	// The value really is in the box. A guard whose typing missed would press
	// Preview on an answerable form, see no refusal and report a fold that never
	// had anything to open.
	if got := boxText(t, fields, setting); got == "" {
		t.Fatalf("nothing was typed into %q, so there is no refusal to cause", setting)
	}
	if !buriedFields(body, fields)[setting] {
		t.Fatalf("%q is on the screen before anything is pressed, so this guard cannot tell "+
			"whether a refusal opened the section it is in", setting)
	}

	pressNamed(t, body, text.ButtonPreview())
	join(host)

	if buriedFields(body, fields)[setting] {
		t.Errorf("the run was refused because of %q and the section holding it stayed shut, so the "+
			"screen refuses to run and marks a box nobody can see. Marked: %v", setting, fields.Marked())
	}
	// And the refusal really is about that box, or the screen opened a section
	// for some other reason and this guard is reading a coincidence.
	if said := fields.Lookup(setting).Saying(); said == "" {
		t.Errorf("the section holding %q was opened and the box says nothing, so the refusal "+
			"this guard thinks it caused was about something else. Marked: %v", setting, fields.Marked())
	}
}

// TestNothingInTheManifestNotesChangesAByte holds the line the section was drawn
// on.
//
// The owner's boundary of 2026-08-25: those settings describe the case and not
// the files, they are carried into the manifest, and not one of them changes a
// byte of what is written. That is what makes them safe to put away by default,
// and it is also the only rule the section has - the seed reads as if it
// belonged there and does not, because it changes every byte.
//
// Asked of the engine rather than of a list. Which settings are in the section
// is read off the screen, by opening it and seeing what appears, so a setting
// moved into it tomorrow is covered without a line here. Then the same recipe is
// run twice, once with those settings answered and once without, and the files
// are compared byte for byte.
func TestNothingInTheManifestNotesChangesAByte(t *testing.T) {
	bare := t.TempDir()
	noted := t.TempDir()

	inNotes, plain := runWithNotes(t, bare, nil)
	if len(inNotes) == 0 {
		t.Fatal("opening the manifest notes put no settings on the screen, so this guard checked nothing")
	}
	_, manifest := runWithNotes(t, noted, inNotes)

	if len(plain.files) == 0 {
		t.Fatal("the run wrote no files, so there are no bytes to compare")
	}
	for name, sum := range plain.files {
		switch other, there := manifest.files[name]; {
		case !there:
			t.Errorf("%q was written when the manifest notes were empty and not when they were "+
				"filled in, so a note changed what the run produced", name)
		case other != sum:
			t.Errorf("%q comes out differently once the manifest notes are filled in, so something "+
				"in that section changes the files rather than only describing them. The section "+
				"holds %v", name, inNotes)
		}
	}
	// And the values are not merely harmless - they arrive where the section is
	// named for. A note nothing carries would be a box with no effect anywhere,
	// which is a worse thing to hide behind a fold than a setting that works.
	for _, want := range []string{"boundary-cases", "reject"} {
		if !strings.Contains(manifest.raw, want) {
			t.Errorf("%q was typed into the manifest notes and the manifest does not carry it", want)
		}
	}
}

type writtenRun struct {
	files map[string]string
	raw   string
}

// runWithNotes runs one batch and returns what came out. Given no settings it
// answers with the settings the notes section turns out to hold, found by
// opening it and seeing which registered boxes come onto the screen.
func runWithNotes(t *testing.T, dir string, notes []string) ([]string, writtenRun) {
	t.Helper()
	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	body := screen.Object()
	fields := screen.Fields()

	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("txt")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "notes")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "1kb")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyCount), "2")
	setBox(t, fields, recipe.KeyOutputDir, dir)
	setBox(t, fields, recipe.KeySeed, "77")

	shut := buriedFields(body, fields)
	openFold(t, body, text.BatchHeading(1), text.SectionManifestNotes())
	open := buriedFields(body, fields)

	var held []string
	for _, f := range fields.All() {
		if shut[f.Setting] && !open[f.Setting] {
			held = append(held, f.Setting)
		}
	}
	sort.Strings(held)

	// Filled in all at once rather than one at a time, because a reason for an
	// expectation is refused without the expectation it is about - so asking
	// about each on its own would be asking about a run that does not happen.
	for _, setting := range notes {
		switch control := fields.Lookup(setting).Control.(type) {
		case *parts.Chooser:
			control.SetSelected(legalChoice(t, setting, control))
		default:
			setBox(t, fields, setting, "boundary-cases")
		}
	}

	pressNamed(t, body, text.ButtonGenerate())
	waitForManifest(t, host, dir)
	screen.Stop()

	return held, readRun(t, dir)
}

// legalChoice is a value a closed list really offers. The expectation and its
// reason are picked so they go together, because the engine refuses a reason
// that does not belong to the outcome beside it.
func legalChoice(t *testing.T, setting string, control *parts.Chooser) string {
	t.Helper()
	want := "reject"
	if strings.HasSuffix(setting, recipe.KeyExpectedReason) {
		want = "size_limit"
	}
	for _, option := range control.Options {
		if option == want {
			return option
		}
	}
	t.Fatalf("%q does not offer %q, and this guard needs an outcome and a reason that go together",
		setting, want)
	return ""
}

func readRun(t *testing.T, dir string) writtenRun {
	t.Helper()
	out := writtenRun{files: map[string]string{}}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		if strings.HasSuffix(entry.Name(), ".json") {
			out.raw = string(body)
			continue
		}
		out.files[entry.Name()] = string(body)
	}
	return out
}

// TestASectionPutAwayStillSaysWhatIsInIt keeps a folded section from swallowing
// a value.
//
// These sections arrive shut, so this is not the same question as for a batch: a
// batch is folded by somebody who has just looked at it, and a section is folded
// before anybody has seen it. A value typed into one and then hidden without a
// word would be a setting somebody cannot see and did not remove, which is worse
// than the scrolling folding was meant to save.
func TestASectionPutAwayStillSaysWhatIsInIt(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	body := screen.Object()
	fields := screen.Fields()

	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("bmp")

	for _, section := range []struct {
		title   string
		setting string
		typed   string
	}{
		{text.SettingsFor("bmp"), recipe.KeyProperties + ".width", "640"},
		{text.SectionManifestNotes(), recipe.KeyGroup, "boundary-cases"},
	} {
		at := recipe.TargetAddress(1, section.setting)
		openFold(t, body, text.BatchHeading(1), section.title)
		setBox(t, fields, at, section.typed)
		// Shut again, which is what a person does after looking - and the line
		// has to be worked out at that moment rather than when the panel was
		// built, because at build time nobody had typed anything.
		foldTitled(t, body, text.BatchHeading(1), section.title).OnTapped()

		if !buriedFields(body, fields)[at] {
			t.Fatalf("%q did not shut, so this guard is asking about an open section", section.title)
		}
		if said := shownText(body); !strings.Contains(said, section.typed) {
			t.Errorf("%q was typed into %q, the section was put away, and nothing on the screen "+
				"says the value is there:\n%s", section.typed, section.title, said)
		}
	}
}

// TestTheSectionsInsideABatchArriveFolded records the owner's decision of
// 2026-08-25 as something a build can be wrong about.
//
// The number under it is 248 px of form per screen, measured at bmp (O98), for
// settings a format works out on its own when nobody states them - and three
// notes that change nothing in the files.
func TestTheSectionsInsideABatchArriveFolded(t *testing.T) {
	for _, screen := range []struct {
		name     string
		open     func(*testing.T) (fyne.CanvasObject, *parts.Fields)
		sections []string
	}{
		{
			name: "the single batch screen",
			open: func(t *testing.T) (fyne.CanvasObject, *parts.Fields) {
				s := window.NewGenerate(newFakeHost(t))
				return s.Object(), s.Fields()
			},
			sections: []string{text.SettingsFor("bmp")},
		},
		{
			name: "the batch screen",
			open: func(t *testing.T) (fyne.CanvasObject, *parts.Fields) {
				s := window.NewRecipe(newFakeHost(t))
				return s.Object(), s.Fields()
			},
			sections: []string{text.SettingsFor("bmp"), text.SectionManifestNotes()},
		},
	} {
		t.Run(screen.name, func(t *testing.T) {
			body, fields := screen.open(t)
			for _, title := range screen.sections {
				// openFold refuses a section that is already open, which is the
				// assertion this test is made of.
				openFold(t, body, "", title)
			}
			// And the batch itself is NOT folded, or a screen that opens with
			// everything away is a screen with nothing on it.
			if len(fields.All()) == 0 {
				t.Fatal("this screen registered no fields at all")
			}
			if buriedFields(body, fields)[recipe.TargetAddress(1, recipe.KeyID)] &&
				buriedFields(body, fields)["id"] {
				t.Error("the batch itself arrives folded, so the screen opens with nothing to fill in")
			}
		})
	}
}
