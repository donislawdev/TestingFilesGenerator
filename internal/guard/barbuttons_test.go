package guard

import (
	"fmt"
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// barButtonsChild runs the bar guard in a process of its own, because the
// language a window speaks is the whole process's.
const barButtonsChild = "TFG_BAR_BUTTONS_CHILD"

// TestTheBarsButtonsNeverStandOverOneAnother asks the bar at the foot of each
// work screen after a run has finished, in every language of the build and in
// the pseudo language that makes every word longer, in the window as it opens and in the
// narrowest one it allows: no button of the bar may stand over another, and
// none may reach past the window's edge.
//
// Measured on 2026-11-26 with guirender after the review of #264
// (docs/REVIEW-264-2026-11-25.md, remark 6): Copy command, at the bar's right
// end, stood over the offers of a finished run - over Open manifest of the
// batch screen entirely, in English - at every width, because the bar is never
// wider than the readable column. No guard looked at the bar after a run.
func TestTheBarsButtonsNeverStandOverOneAnother(t *testing.T) {
	if os.Getenv(barButtonsChild) != "1" {
		t.Run("canary: a row that cannot wrap is caught", canaryBarThatCannotWrap)
		runChild(t, "TestTheBarsButtonsNeverStandOverOneAnother", barButtonsChild)
		return
	}
	// Every language the build carries, read from the build, and the pseudo
	// one, whose longer words stand for the language nobody has written yet.
	var languages []string
	for _, l := range text.Languages() {
		languages = append(languages, l.Tag)
	}
	for _, language := range append(languages, text.Pseudo) {
		if err := text.LoadBuiltIn(language); err != nil {
			t.Fatalf("the window would not speak %s: %v", language, err)
		}
		for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
			t.Run(language+"/"+tab, func(t *testing.T) { barAfterARun(t, tab) })
		}
	}
}

// barAfterARun finishes a small run on a screen and asks the bar about it, at
// the size the window opens at and at the narrowest.
func barAfterARun(t *testing.T, tab string) {
	content, w, host := screenInAWindowWithHost(t, tab)
	if tab != text.TabPresets() {
		chooser, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
		if !ok {
			t.Fatalf("the %s screen has no format menu, so this guard read the wrong tree", tab)
		}
		chooser.SetSelected("txt")
		fill(t, content, text.FieldSize(), "1kb")
		fill(t, content, text.FieldCount(), "3")
	}
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	press(t, content, text.ButtonGenerate())
	join(host)
	settle(content, w)

	// The state this guard is about, asserted rather than assumed: a run that
	// failed offers nothing, and a bar with nothing offered has nothing to
	// stand over.
	for _, offer := range []string{text.ButtonOpenFolder(), text.ButtonOpenManifest()} {
		if b := buttonNamed(content, offer); b == nil || !b.Visible() {
			t.Fatalf("after the run the %s screen offers no %q - the run did not finish. The screen says: %s",
				tab, offer, textIn(content))
		}
	}
	noButtonOverAnother(t, content, w, "the window as it opens")

	width := w.Content().MinSize().Width
	if w.Padded() {
		width += 2 * theme.Padding()
	}
	narrowest := fyne.NewSize(width, window.LargestOpening.Height)
	opening := w.Canvas().Size().Width
	settleAt(content, w, narrowest)
	// settle lays the window out at the size it opens at, which is the size
	// asked about above - so the narrow state is asserted, not assumed. Until
	// the review of this guard (#266) it went through settle and asked the
	// opening width twice.
	if now := w.Canvas().Size().Width; now >= opening {
		t.Fatalf("the window was to be narrowed to %.0f px and is %.0f px wide, as wide as it opened, so the "+
			"narrowest window is not what is asked about", narrowest.Width, now)
	}
	noButtonOverAnother(t, content, w, "the narrowest window")
}

// settleAt is settle at a size of the guard's choosing: laid out again at
// another size and back, so every container asks its children again.
func settleAt(content fyne.CanvasObject, w fyne.Window, size fyne.Size) {
	content.Refresh()
	w.Resize(fyne.NewSize(size.Width, size.Height-1))
	content.Refresh()
	w.Resize(size)
}

// TestAnOfferWiderThanTheRunButtonsStaysInsideTheBar lays out a bar whose one
// offer is wider than the button that runs something, in the narrowest window
// the bar allows, and asks that the offer ends inside it. The row's minimum was
// the run buttons alone until the review of #266, so a window that narrow put
// the offer on a line of its own and let it run past the edge.
func TestAnOfferWiderThanTheRunButtonsStaysInsideTheBar(t *testing.T) {
	newTestApp()
	t.Cleanup(func() { newTestApp() })
	offer := parts.NewButton(parts.Secondary, "Open the instructions this run wrote beside its files", func() {}).InTheBar()
	bar := parts.ActionBar(nil, parts.ActionRow(
		[]fyne.CanvasObject{parts.NewButton(parts.Primary, "Go", func() {}).InTheBar()}, offer))
	w := test.NewWindow(bar)
	t.Cleanup(w.Close)
	narrowest := bar.MinSize()
	settleAt(bar, w, narrowest)
	settleAt(bar, w, narrowest)
	drv := fyne.CurrentApp().Driver()
	ends := drv.AbsolutePositionForObject(offer).X + offer.Size().Width
	if width := w.Canvas().Size().Width; ends > width+0.5 {
		t.Errorf("in a window %.0f px wide, the narrowest the bar allows, the offer ends at x=%.1f - past the edge",
			width, ends)
	}
}

// barButtonNames are the buttons that stand in the bar at the foot: the rail,
// the buttons that run something, what a finished run offers and Copy command.
func barButtonNames() []string {
	return []string{text.ButtonDonate(), text.ButtonGitHub(), text.ButtonAddBatch(),
		text.ButtonPreview(), text.ButtonGenerate(), text.ButtonCancel(),
		text.ButtonOpenFolder(), text.ButtonOpenManifest(), text.ButtonOpenInstructions(),
		text.ButtonCopyCommand()}
}

// noButtonOverAnother reports every pair of the bar's shown buttons that share
// a pixel, and every one past the window's edge.
func noButtonOverAnother(t *testing.T, content fyne.CanvasObject, w fyne.Window, where string) {
	t.Helper()
	for _, problem := range barOverlaps(content, w.Canvas().Size().Width) {
		t.Errorf("in %s, %s", where, problem)
	}
}

// barOverlaps is what noButtonOverAnother reports, as sentences, so the canary
// can ask it the same question.
func barOverlaps(content fyne.CanvasObject, windowWidth float32) []string {
	drv := fyne.CurrentApp().Driver()
	type placed struct {
		name string
		pos  fyne.Position
		size fyne.Size
	}
	var shown []placed
	for _, name := range barButtonNames() {
		b := buttonNamed(content, name)
		if b == nil || !b.Visible() {
			continue
		}
		shown = append(shown, placed{name, drv.AbsolutePositionForObject(b), b.Size()})
	}
	var problems []string
	for i, a := range shown {
		if a.pos.X < 0 || a.pos.X+a.size.Width > windowWidth+0.5 {
			problems = append(problems, fmt.Sprintf("%q runs from x=%.1f to x=%.1f, past the window's edge at %.0f",
				a.name, a.pos.X, a.pos.X+a.size.Width, windowWidth))
		}
		for _, b := range shown[i+1:] {
			apart := a.pos.X+a.size.Width <= b.pos.X+0.5 || b.pos.X+b.size.Width <= a.pos.X+0.5 ||
				a.pos.Y+a.size.Height <= b.pos.Y+0.5 || b.pos.Y+b.size.Height <= a.pos.Y+0.5
			if !apart {
				problems = append(problems, fmt.Sprintf("%q (x %.1f-%.1f, y %.1f) and %q (x %.1f-%.1f, y %.1f) stand over one another",
					a.name, a.pos.X, a.pos.X+a.size.Width, a.pos.Y, b.name, b.pos.X, b.pos.X+b.size.Width, b.pos.Y))
			}
		}
	}
	return problems
}

// canaryBarThatCannotWrap builds the bar the way the work screens built it
// before 2026-11-26 - the offers in a row that has one line - in a window as
// narrow as the readable column leaves it, and asks that the check above finds
// the buttons over one another. A check that passes this bar would pass the
// bug.
func canaryBarThatCannotWrap(t *testing.T) {
	newTestApp()
	t.Cleanup(func() { newTestApp() })
	bar := parts.ActionBarEnds(
		container.NewHBox(parts.NewButton(parts.Quiet, text.ButtonDonate(), func() {}).InTheBar(),
			parts.NewButton(parts.Quiet, text.ButtonGitHub(), func() {}).InTheBar()),
		parts.NewButton(parts.Secondary, text.ButtonCopyCommand(), func() {}).InTheBar(),
		parts.ButtonRow(
			parts.NewButton(parts.Secondary, text.ButtonPreview(), func() {}).InTheBar(),
			parts.NewButton(parts.Primary, text.ButtonGenerate(), func() {}).InTheBar(),
			parts.NewButton(parts.Secondary, text.ButtonOpenFolder(), func() {}).InTheBar(),
			parts.NewButton(parts.Secondary, text.ButtonOpenManifest(), func() {}).InTheBar(),
			parts.NewButton(parts.Secondary, text.ButtonOpenInstructions(), func() {}).InTheBar()))
	w := test.NewWindow(bar)
	t.Cleanup(w.Close)
	size := fyne.NewSize(parts.ColumnWidth, 200)
	w.Resize(size)
	bar.Refresh()
	w.Resize(size)
	if len(barOverlaps(bar, size.Width)) == 0 {
		t.Fatal("five buttons, a rail and Copy command on one line of a column as wide as the window allows, " +
			"and the check found nothing over anything - it would pass the bar of #264 as well")
	}
}
