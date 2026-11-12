package guard

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// newTestApp is test.NewApp for a guard, with the toolkit's clock read before
// anything is drawn. Every guard that needs an application takes it here.
//
// Fyne stamps how long a widget's renderer lives from a sample of the clock it
// takes only when something is painted, and a renderer lives a minute
// (internal/cache in Fyne 2.8.2: setAlive, Clean, ValidDuration). After a
// guard of more than a minute that paints nothing - a film under the race
// detector took 102 s - every renderer the next guard makes is born expired.
// Its first Capture destroys them, paints renderers made again with nothing
// laid out, and the picture comes out blank (docs/REVIEW-236-2026-11-12.md).
// Capturing an empty canvas takes a fresh sample.
func newTestApp() fyne.App {
	app := test.NewApp()
	readFyneClock()
	return app
}

// newTempTestApp is test.NewTempApp with the clock read, as newTestApp is.
func newTempTestApp(t testing.TB) fyne.App {
	app := test.NewTempApp(t)
	readFyneClock()
	return app
}

// readFyneClock has Fyne sample its clock, the only way a test can: a capture
// calls the cache's clean-up, which takes the sample first.
func readFyneClock() {
	test.NewCanvas().Capture()
}

// TestAGuardThatDrawsAfterALongPauseStillDraws holds newTestApp to what it is
// for, in a process of its own with a renderer's life cut to FYNE_CACHE=300ms
// and a pause longer than that before anything is drawn - the state a long
// guard leaves the next one in. A box and a button drawn after test.NewApp
// alone have to come out blank, or the pause no longer reaches the state this
// is about. Drawn after newTestApp, they have to be there.
func TestAGuardThatDrawsAfterALongPauseStillDraws(t *testing.T) {
	const name = "TestAGuardThatDrawsAfterALongPauseStillDraws"
	if how := os.Getenv(pauseChild); how != "" {
		drawAfterAPause(t, how)
		return
	}
	for how, want := range map[string]string{"plain": "box drawn=false", "fresh": "box drawn=true"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		run := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+name+"$", "-test.v", "-test.count=1")
		run.Env = append(os.Environ(), pauseChild+"="+how, "FYNE_CACHE=300ms")
		out, err := run.CombinedOutput()
		cancel()
		if err != nil || !strings.Contains(string(out), "--- PASS: "+name) {
			t.Fatalf("the %s child did not pass (%v):\n%s", how, err, out)
		}
		if !strings.Contains(string(out), want) {
			t.Errorf("drawn after a pause past a renderer's life with the %s application, the picture is not %s:\n%s", how, want, out)
		}
	}
}

const pauseChild = "TFG_FYNE_PAUSE_CHILD"

// drawAfterAPause is the child: nothing drawn since the process started, a
// pause past a renderer's life, then the box and button of
// TestAButtonBesideABoxIsDrawnAsTallAsTheBox, captured once.
func drawAfterAPause(t *testing.T, how string) {
	t.Helper()
	time.Sleep(400 * time.Millisecond)
	var app fyne.App
	if how == "fresh" {
		app = newTestApp()
	} else {
		app = test.NewApp()
	}
	app.Settings().SetTheme(parts.Theme())
	e := parts.NewEntry()
	e.SetText("/tfg/out")
	boxed, _ := parts.WithRing(e)
	b := parts.NewButton(parts.Secondary, "Choose...", func() {})
	w := test.NewTempWindow(t, container.NewWithoutLayout(boxed, b))
	height := boxed.MinSize().Height
	boxed.Resize(fyne.NewSize(140, height))
	boxed.Move(fyne.NewPos(10, 10))
	b.Resize(fyne.NewSize(100, height))
	b.Move(fyne.NewPos(170, 10))
	w.Resize(fyne.NewSize(300, 60))
	picture := w.Canvas().Capture()
	// The box apart from the button: the button is drawn either way, so a
	// picture with anything in it says nothing (measured on 2026-11-12, a
	// renderer born expired: the box 0 rows, the button 29).
	page := parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)
	box := false
	for y := 0; y < picture.Bounds().Dy(); y++ {
		box = box || !sameColour(picture.At(60, y), page)
	}
	t.Logf("box drawn=%v", box)
}

// TestEveryGuardTakesItsApplicationWithTheClockRead holds every guard to
// newTestApp: a guard that calls test.NewApp itself draws blank whenever it
// runs after a long one, and only under the race detector, where guards are
// slow enough.
func TestEveryGuardTakesItsApplicationWithTheClockRead(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) < 100 {
		t.Fatalf("found %d guard files (%v), so this read nothing", len(files), err)
	}
	for _, f := range files {
		if f == "fyneclock_test.go" {
			continue
		}
		for _, call := range rawAppCalls(readFile(t, f)) {
			t.Errorf("%s calls %s - take the application from newTestApp or newTempTestApp", f, call)
		}
	}
	if got := rawAppCalls("func f() {\n\tapp := test.NewApp()\n\tdefer test.NewTempApp(t)\n}\n"); len(got) != 2 {
		t.Errorf("canary: a source with both calls was read as %v", got)
	}
}

var rawAppCall = regexp.MustCompile(`\btest\.(NewApp|NewTempApp)\(`)

// rawAppCalls is every call in src that makes an application without reading
// the clock.
func rawAppCalls(src string) []string {
	return rawAppCall.FindAllString(src, -1)
}
