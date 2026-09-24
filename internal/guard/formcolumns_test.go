package guard

import (
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The layout of 2026-09-23 and what it promises, asked of the running
// screens. Built round by round from the owner's verdicts on the real window.
// docs/GUI-PROTOTYP-2026-09-23.md has the rounds, the measurements and what
// was turned down.

// TestShortFieldsShareARowAndTakeOneColumnEach asks the first row of the
// single batch screen: the format, the size, how many and the damage stand in
// one row, one column each, the same width - and the output directory, a path
// nobody can predict the length of, takes more than one.
//
// The defect it keeps away is the one the owner reported twice: a box holding
// "1" as wide as half the form, and a form one field tall per row.
func TestShortFieldsShareARowAndTakeOneColumnEach(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	screen := tabContent(t, content, text.TabOneTarget())

	labels := []string{text.FieldFormat(), text.FieldSize(), text.FieldCount(), text.FieldDamage()}
	var first band
	for i, label := range labels {
		box, ok := objectBox(screen, controlUnder(screen, label))
		if !ok {
			t.Fatalf("the box under %q is not laid out", label)
		}
		if i == 0 {
			first = box
			continue
		}
		if off := box.Y - first.Y; off > 1 || off < -1 {
			t.Errorf("%q stands at y=%.1f and %q at y=%.1f - the short fields of the first row are not in one row",
				labels[0], first.Y, label, box.Y)
		}
		if off := box.Width - first.Width; off > 1 || off < -1 {
			t.Errorf("%q is %.1f px wide and %q %.1f px - one column each is one width", labels[0], first.Width, label, box.Width)
		}
		if box.X <= first.X {
			t.Errorf("%q starts at x=%.1f, not to the right of %q at x=%.1f", label, box.X, labels[0], first.X)
		}
	}
	path, ok := objectBox(screen, entryUnder(t, screen, text.FieldOutputDir()))
	if !ok {
		t.Fatal("the output directory is not laid out")
	}
	if path.Width < first.Width*3 {
		t.Errorf("the output directory is %.1f px wide, which is under three columns of %.1f - a path is the one "+
			"value whose length nobody can predict", path.Width, first.Width)
	}
}

// TestASizeBelowTheMinimumOffersTheSmallestSize presses the button the
// refusal carries and asks what reaches the box.
//
// Reported by the owner from the running window: the refusal named the
// smallest size a format can make as eight digits, and the only way to act on
// it was to copy them by hand. Pressed rather than read, because a button
// that says the right words and puts nothing in the box is the defect this is
// about - and the box is asked afterwards whether the run still refuses it.
func TestASizeBelowTheMinimumOffersTheSmallestSize(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewGenerate(host)
	content := screen.Object()
	fill(t, content, text.FieldSize(), "1")
	screen.PressPreview()
	join(host)

	field := screen.Fields().Lookup(core.SettingSize)
	if field == nil || field.Saying() == "" {
		t.Fatal("a size of 1 B was not refused under the size box, so there is nothing to offer a fix for")
	}
	offered := field.Offered()
	if offered == "" {
		t.Fatalf("the size box says %q and offers no button to put it right", field.Saying())
	}
	press(t, content, offered)
	join(host)

	typed := entryUnder(t, content, text.FieldSize()).Text
	bytes, err := core.ParseSize(typed)
	if err != nil {
		t.Fatalf("the button put %q in the size box, which is not a size: %v", typed, err)
	}
	if !strings.Contains(offered, core.ExactBytes(bytes)) {
		t.Errorf("the button said %q and put %q in the box - what it says and what it does differ", offered, typed)
	}
	if said := field.Saying(); said != "" {
		t.Errorf("the size the button put in the box is still refused: %q", said)
	}
}

// TestALimitBelowTheMinimumOffersNoButtonThatLeadsBackToItself is the same
// refusal under "Around a limit", where the box is not the size refused.
//
// The set is built one byte under the limit, on it and one over, so a limit
// of 74 for a PNG refuses the file of 73. The button used to put the minimum,
// 74, back in the box - the same 73, the same refusal, and a person pressing
// it saw nothing change. Measured on 2026-09-23 after an outside review of
// #126 (docs/REVIEW-126-2026-09-23.md). The smallest limit whose three files
// all exist is the engine's to answer and it cannot yet, so the promise is
// the modest one: this box gets the sentence and no button. The day the
// engine answers, this guard is the one to rewrite into "one press, and the
// limit is no longer refused".
func TestALimitBelowTheMinimumOffersNoButtonThatLeadsBackToItself(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	body := screen.Object()
	chooseFormat(t, body, "png")
	chooseSizeWay(t, body, text.SizeWayBoundary())
	fields := screen.Fields()
	at := recipe.TargetAddress(1, recipe.KeyBoundary)
	minimum := minimumOf(t, "png")
	setBox(t, fields, at, strconv.FormatInt(minimum, 10))
	pressNamed(t, body, text.ButtonPreview())
	join(host)

	field := fields.Lookup(at)
	if field == nil {
		t.Fatalf("the screen registers no box at %q", at)
	}
	if field.Saying() == "" {
		t.Fatalf("a limit of %d B builds a file of %d B for a format whose smallest is %d B and was not refused, "+
			"so there is nothing to offer a fix for.\n%s", minimum, minimum-1, minimum, allSaid(fields))
	}
	if offered := field.Offered(); offered != "" {
		t.Errorf("the limit box offers %q under a refusal about the file one byte below the limit.\n"+
			"Reason: the button puts the format's smallest size in the box, the box is the LIMIT, and\n"+
			"the file under it is refused again - measured, the button led back to itself.", offered)
	}
}

// minimumOf is the smallest file a format can make, asked of the registry.
func minimumOf(t *testing.T, id string) int64 {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatalf("the registry has no %s: %v", id, err)
	}
	return d.MinBytes
}

// TestARefusalAboutWhatIsInTheDirectoryStandsUnderItAndOpensIt runs a
// preview into a directory holding a manifest already.
//
// Three things the owner's report from the running window came to: the
// refusal stands under the output directory box where it can be read whole,
// it carries the way to that directory, and the line at the foot no longer
// says it is still working out the cost of a run that was refused.
func TestARefusalAboutWhatIsInTheDirectoryStandsUnderItAndOpensIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, engine.DefaultManifestName), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := newFakeHost(t)
	screen := window.NewGenerate(host)
	content := screen.Object()
	fill(t, content, text.FieldSize(), "1kb")
	entryUnder(t, content, text.FieldOutputDir()).SetText(dir)
	screen.PressPreview()
	join(host)

	field := screen.Fields().Lookup(engine.SettingOutDir)
	if field == nil {
		t.Fatal("the screen registers no output directory field, so there is nothing to place the refusal under")
	}
	if !strings.Contains(field.Saying(), engine.DefaultManifestName) {
		t.Fatalf("a manifest already in the directory was not refused under the output directory box - it says %q",
			field.Saying())
	}
	if got := field.Offered(); got != text.ButtonOpenFolder() {
		t.Fatalf("the refusal about the directory offers %q, not the way to the directory", got)
	}
	// The one on the screen: the bar keeps a button of the same name, hidden
	// until a run has written something, and pressing that one is not the
	// question.
	shown := visibleButtonNamed(content, text.ButtonOpenFolder())
	if shown == nil {
		t.Fatalf("there is no %q button on the screen", text.ButtonOpenFolder())
	}
	shown.OnTapped()
	if host.folderCount != 1 || host.folder != dir {
		t.Errorf("the button opened %q %d time(s), and the directory the refusal is about is %q",
			host.folder, host.folderCount, dir)
	}
	if said := shownText(content); strings.Contains(said, text.WorkingOutTheCost()) {
		t.Errorf("the preview was refused and the screen still says %q", text.WorkingOutTheCost())
	}
}

// TestARefusalStandsUnderItsRowAcrossIt asks where the sentence goes: under
// the row of the field it is about, starting on the row's left edge, and
// reaching past the field's own column - measured in the real window, the same
// sentence broke into three lines inside a 185 px column and pushed the form
// down.
//
// The row's edge rather than the field's since 2026-09-24. Started on its own
// field's edge, a refusal about the second field of a row stood a column in
// from one about the first, and two of them read as a staircase (review
// UI-011). The size box is the second field of its row, so this asks the
// case that moved: the sentence about it starts under Format, not under Size.
func TestARefusalStandsUnderItsRowAcrossIt(t *testing.T) {
	ourTheme(t)
	generate := window.NewGenerate(newFakeHost(t))
	screen := generate.Object()
	w := test.NewTempWindow(t, screen)
	w.Resize(fyne.NewSize(window.LargestOpening.Width, window.LargestOpening.Height))
	fields := generate.Fields()
	fields.Mark(core.SettingSize, errorString("a size of 3 B is below the smallest avif, which is 311 B"))
	screen.Refresh()
	w.Resize(fyne.NewSize(window.LargestOpening.Width, window.LargestOpening.Height+1))

	box, ok := objectBox(screen, entryUnder(t, screen, text.FieldSize()))
	if !ok {
		t.Fatal("the size box is not laid out")
	}
	said, ok := labelBox(screen, fields.Lookup(core.SettingSize).Saying())
	if !ok {
		t.Fatal("the refusal under the size box is not on the screen")
	}
	if said.Y < box.Y+box.Height {
		t.Errorf("the refusal starts at y=%.1f, above the bottom of its box at y=%.1f", said.Y, box.Y+box.Height)
	}
	first, ok := objectBox(screen, chooserUnder(t, screen, text.FieldFormat()))
	if !ok {
		t.Fatal("the format box, the first of the size box's row, is not laid out")
	}
	if first.X >= box.X {
		t.Fatalf("the format box at x=%.1f is not to the left of the size box at x=%.1f, so this guard is not asking about a field that moved", first.X, box.X)
	}
	if off := said.X - first.X; off > 1 || off < -1 {
		t.Errorf("the refusal starts at x=%.1f and its row at x=%.1f - every refusal of a row starts on the row's edge, or two of them stand as a staircase",
			said.X, first.X)
	}
	if said.Width <= box.Width {
		t.Errorf("the refusal is %.1f px wide and its box %.1f - it is laid inside the field's column, not across the row",
			said.Width, box.Width)
	}
}

// TestAMenuShowsItsArrowInTheAccent asks the renderer of a menu what colour
// its arrow is drawn in. The owner chose the accent from three variants shown
// side by side, after a menu and a box to type in measured the same to the
// pixel. Disabled, the menu keeps the toolkit's disabled arrow, so a menu
// frozen for a run does not look live.
func TestAMenuShowsItsArrowInTheAccent(t *testing.T) {
	test.NewTempApp(t)
	menu := parts.NewChooser([]string{"avif", "png"}, nil)
	if got := arrowColourOf(t, menu); got != theme.ColorNamePrimary {
		t.Errorf("a menu draws its arrow in %q, not in the accent", got)
	}
	menu.Disable()
	if got := arrowColourOf(t, menu); got == theme.ColorNamePrimary {
		t.Error("a disabled menu still draws its arrow in the accent, so it looks live")
	}
}

// arrowColourOf is the colour name the arrow of a menu is drawn in, or
// nothing when the arrow is not a coloured resource of ours.
func arrowColourOf(t *testing.T, menu *parts.Chooser) fyne.ThemeColorName {
	t.Helper()
	renderer := test.TempWidgetRenderer(t, menu)
	renderer.Refresh()
	for _, o := range renderer.Objects() {
		icon, ok := o.(*widget.Icon)
		if !ok {
			continue
		}
		if themed, ok := icon.Resource.(*theme.ThemedResource); ok {
			return themed.ColorName
		}
		return ""
	}
	t.Fatal("the menu draws no arrow")
	return ""
}

// TestAFoldedSectionOpensForARefusalAboutItsField folds File configuration
// away, refuses the size inside it and asks whether it opened. Every section
// of a work screen folds since 2026-09-23, and a box marked where nobody can
// see it reads as a button that did nothing.
func TestAFoldedSectionOpensForARefusalAboutItsField(t *testing.T) {
	host := newFakeHost(t)
	screen := window.NewGenerate(host)
	content := screen.Object()
	head := foldTitled(t, content, "", text.SectionConfiguration())
	head.Tapped(nil)
	if head.Open() {
		t.Fatal("pressing the head of File configuration did not fold it, so this guard asks nothing")
	}
	fill(t, content, text.FieldSize(), "1")
	screen.PressPreview()
	join(host)
	if !head.Open() {
		t.Error("the size inside a folded File configuration was refused and the section stayed shut")
	}
}

// TestAGroupOfSettingsIsFramedWithTheOneNeutralRail asks a group of settings
// for the rail down its left edge: drawn, and in the neutral colour of a
// field's name - the one colour every rail has had since 2026-09-24. Until
// then the rail carried the kind of the group, the primary colour for a
// format's settings among them, and that blue read as "this one is chosen"
// beside the main button and the keyboard's mark (review UI-009, the owner
// chose grey for all). So the primary colour is asked for as well, and must be
// absent.
func TestAGroupOfSettingsIsFramedWithTheOneNeutralRail(t *testing.T) {
	group := parts.NewInnerFolding("Settings", parts.Prose("inside")).Object()
	if !drawsFill(group, parts.PaletteColour(parts.ColorNameLabel, theme.VariantDark)) {
		t.Error("a group of settings draws no rail in the colour of a field's name")
	}
	if drawsFill(group, parts.PaletteColour(theme.ColorNamePrimary, theme.VariantDark)) {
		t.Error("a group of settings draws something in the primary colour, the colour of the main button and the keyboard's mark")
	}
}

// drawsFill is whether anything under o is a rectangle filled with c.
func drawsFill(o fyne.CanvasObject, c color.Color) bool {
	found := false
	walk(o, func(obj fyne.CanvasObject) {
		if r, ok := obj.(*canvas.Rectangle); ok && r.FillColor == c {
			found = true
		}
	})
	return found
}

// visibleButtonNamed is the button with these words that is shown, where a
// screen keeps two of one name and only one of them on the screen.
func visibleButtonNamed(o fyne.CanvasObject, name string) *parts.Button {
	var found *parts.Button
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*parts.Button); ok && b.Text == name && b.Visible() {
			found = b
		}
	})
	return found
}

// errorString is a refusal with nothing but words, for a guard marking a box.
type errorString string

func (e errorString) Error() string { return string(e) }
