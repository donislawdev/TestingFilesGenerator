package guard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// After the driver refused, the window is tried once more in a second
// process with the software renderer - and only where that renderer can be.
//
// Measured on 2026-09-17 on a Windows Server 2025 guest with no 3D
// acceleration: the toolkit could not open a window, a second process that
// loaded Mesa's llvmpipe before the toolkit started opened one, and the
// person used it (docs/GUI-NO-OPENGL section 7). This is the guard on the
// decision between those two, pressed without a window, a renderer or a
// second process: every piece that touches the world is a field of
// gui.SecondAttempt, and each case sets them to what a machine in that
// state would answer.
//
// Two things matter more than the rest and both are here. The second
// process never starts a third - the flag it was started with is what says
// so - because a machine where the renderer does not help either would
// otherwise start processes until something else stopped it. And the
// refusal the person reads says what became of the renderer, in words from
// the text package: which file is missing, that starting again failed, or
// that it was tried and did not help. On a system the renderer does not
// ship for it says nothing, by the owner's decision, because a sentence
// about a missing file would be about a file nobody promised.
func TestTheSecondAttemptIsTakenOnlyWhereTheRendererCanBe(t *testing.T) {
	missing := filepath.Join("C:", "somewhere", "opengl", "libgallium_wgl.dll")
	noDir := errors.New("no executable path")
	noStart := errors.New("access denied")
	loadFailed := errors.New("the renderer did not load")

	type attempt struct {
		name       string
		launch     gui.Launch
		goos       string
		dirErr     error
		missing    string
		startCode  int
		startErr   error
		loaded     error
		wantCode   int
		wantTried  bool   // Start was called
		wantSaid   bool   // the line before starting reached standard error
		wantAdd    string // what the refusal adds, when it was not tried
		wantWorked bool   // Try answered a code and no reason
	}
	cases := []attempt{
		{name: "windows, renderer beside, second process opened the window",
			launch: gui.Launch{Args: []string{"--catalogue"}}, goos: "windows", startCode: 0,
			wantTried: true, wantSaid: true, wantWorked: true},
		{name: "windows, renderer beside, second process refused too",
			launch: gui.Launch{}, goos: "windows", startCode: 1,
			wantCode: 1, wantTried: true, wantSaid: true, wantWorked: true},
		{name: "windows, renderer beside, second process could not be started",
			launch: gui.Launch{}, goos: "windows", startErr: noStart,
			wantTried: true, wantSaid: true, wantAdd: text.RendererStartFailed(noStart)},
		{name: "windows, renderer not beside",
			launch: gui.Launch{}, goos: "windows", missing: missing,
			wantAdd: text.RendererNotBeside(missing)},
		{name: "windows, the executable's directory unknown",
			launch: gui.Launch{}, goos: "windows", dirErr: noDir,
			wantAdd: text.RendererStartFailed(noDir)},
		{name: "linux, nothing ships",
			launch: gui.Launch{}, goos: "linux", wantAdd: ""},
		{name: "darwin, nothing ships",
			launch: gui.Launch{}, goos: "darwin", wantAdd: ""},
		{name: "already the second process, renderer loaded and did not help",
			launch: gui.Launch{SoftwareGL: true}, goos: "windows",
			wantAdd: text.RendererDidNotHelp()},
		{name: "already the second process, renderer did not load",
			launch: gui.Launch{SoftwareGL: true}, goos: "windows", loaded: loadFailed,
			wantAdd: text.RendererNotLoaded(loadFailed)},
		{name: "asked for by flag on a system nothing ships for, and refused by the driver",
			launch: gui.Launch{SoftwareGL: true}, goos: "linux", loaded: notShippedLoad(t),
			wantAdd: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var said bytes.Buffer
			var startedWith []string
			tried := false
			a := gui.SecondAttempt{
				Launch:        c.launch,
				GOOS:          c.goos,
				ExecutableDir: func() (string, error) { return "somewhere", c.dirErr },
				Beside:        func(string) string { return c.missing },
				Start: func(args []string) (int, error) {
					tried = true
					startedWith = args
					return c.startCode, c.startErr
				},
				ErrOut: &said,
			}
			code, why := a.Try()

			if tried != c.wantTried {
				t.Fatalf("a second process was started: %v, and in this state it has to be %v", tried, c.wantTried)
			}
			if c.wantTried {
				want := append(append([]string{}, c.launch.Args...), gui.SoftwareFlag)
				if strings.Join(startedWith, " ") != strings.Join(want, " ") {
					t.Errorf("the second process was started with %q and it has to get %q - "+
						"the same arguments plus the one flag, so it opens what the first was asked for.",
						startedWith, want)
				}
			}
			if said := said.String(); strings.Contains(said, text.StartingAgainWithSoftwareRenderer()) != c.wantSaid {
				t.Errorf("standard error got %q, and the line saying the program starts again has to be there: %v", said, c.wantSaid)
			}
			if c.wantWorked {
				if why != nil || code != c.wantCode {
					t.Errorf("Try answered code %d, reason %v - it has to answer the second process's code %d and no reason", code, why, c.wantCode)
				}
				return
			}
			if why == nil {
				t.Fatal("Try answered no reason, and in this state there is no second process to answer for")
			}
			if got := gui.RendererSentence(why, c.loaded); got != c.wantAdd {
				t.Errorf("the refusal adds %q about the renderer and it has to add %q", got, c.wantAdd)
			}
		})
	}
}

// notShippedLoad is what loading the renderer answers on a system nothing
// ships for, built by the package that answers it, so this guard does not
// spell the reason itself - and the same on every system, because the case
// it feeds is about a Linux machine wherever the guard happens to run.
func notShippedLoad(t *testing.T) error {
	t.Helper()
	err := gui.NotShippedFor("linux")
	if err == nil {
		t.Fatal("NotShippedFor answered nil")
	}
	return err
}

// A window asked for the renderer says what became of the loading, and on
// a system nothing ships for it says that rather than naming the system as
// if it were an error - which is what the first version did: "could not be
// loaded: linux".
func TestAWindowAskedForTheRendererSaysWhatBecameOfIt(t *testing.T) {
	if got := gui.LoadingSentence(nil); got != text.DrawingWithSoftwareRenderer() {
		t.Errorf("a renderer that loaded is announced as %q", got)
	}
	if got := gui.LoadingSentence(gui.NotShippedFor("darwin")); got != text.RendererNotShipped() {
		t.Errorf("a system nothing ships for is announced as %q", got)
	}
	if strings.Contains(gui.LoadingSentence(gui.NotShippedFor("darwin")), "darwin") {
		t.Error("the sentence names the system as if it were the error")
	}
	failed := errors.New("the renderer did not load")
	if got := gui.LoadingSentence(failed); got != text.RendererNotLoaded(failed) {
		t.Errorf("a renderer that did not load is announced as %q", got)
	}
}

// The window's two flags are read off the launch line, and the arguments
// are kept whole for the second process.
func TestTheLaunchLineIsReadAndKeptWhole(t *testing.T) {
	args := []string{"--catalog", "--software-gl", "something-else"}
	launch := gui.ReadLaunch(args)
	if !launch.Catalogue || !launch.SoftwareGL {
		t.Errorf("%q read as catalogue %v, software %v - both flags are there", args, launch.Catalogue, launch.SoftwareGL)
	}
	if strings.Join(launch.Args, " ") != strings.Join(args, " ") {
		t.Errorf("the arguments were kept as %q and they have to be kept as given, %q", launch.Args, args)
	}
	if plain := gui.ReadLaunch(nil); plain.Catalogue || plain.SoftwareGL {
		t.Error("an empty launch line read as asking for something")
	}
}

// The renderer's files are named in the order the loader needs, under the
// directory the archive puts them in, and a load names the file it failed on.
//
// The order was measured on 2026-09-17 on a machine with a driver: the
// loader imports the renderer by name, and a dependency of a library loaded
// by path is searched for the ordinary way rather than beside that library,
// so loaded second the loader answers "module not found". Loaded first, the
// renderer is already mapped under its name when the loader asks.
func TestTheRendererIsLoadedRendererFirstFromBesideTheProgram(t *testing.T) {
	files := gui.SoftwareFiles("here")
	want := []string{
		filepath.Join("here", "opengl", "libgallium_wgl.dll"),
		filepath.Join("here", "opengl", "opengl32.dll"),
	}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("the renderer's files are %q and they have to be %q, in that order - the renderer first, "+
			"because the loader imports it by name and finds it only if it is already mapped.", files, want)
	}

	// The load itself, on the one system it is for. An empty directory
	// stands for a program with no renderer beside it: the load has to refuse
	// naming the FIRST file, and the driver has to have been named in the
	// environment before that, because the renderer reads it when it
	// initialises and a load that set it afterwards would be too late.
	if runtime.GOOS != "windows" {
		err := gui.LoadSoftwareRenderer(t.TempDir())
		if err == nil {
			t.Fatalf("on %s the load answered nil, and nothing ships for this system to have loaded", runtime.GOOS)
		}
		return
	}
	t.Setenv("GALLIUM_DRIVER", "")
	dir := t.TempDir()
	err := gui.LoadSoftwareRenderer(dir)
	if err == nil {
		t.Fatal("loading a renderer from an empty directory answered nil")
	}
	if !strings.Contains(err.Error(), want[0][len("here")+1:]) {
		t.Errorf("the load failed with %q and it has to name the renderer's file, the first one it tried", err)
	}
	if got := os.Getenv("GALLIUM_DRIVER"); got != "llvmpipe" {
		t.Errorf("after the load GALLIUM_DRIVER is %q and it has to be llvmpipe - named before the first file, "+
			"because without it the renderer picks a driver itself and the process dies in under three seconds, "+
			"twice out of twice on 2026-09-17.", got)
	}
}

// The About screen says the window is drawn in software when it is, and
// says nothing about it when it is not - a window drawn in software is
// slower, and the person should be able to read why in the place they go to
// read what this program is. Untouchable rule 6, both halves.
func TestTheAboutScreenSaysWhenTheWindowIsDrawnInSoftware(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	host := newFakeHost(t)
	if shown := textIn(window.About(host)); strings.Contains(shown, text.DrawingWithSoftwareRenderer()) {
		t.Error("the About screen says the window is drawn in software on a window that is not")
	}
	host = newFakeHost(t)
	host.software = true
	if shown := textIn(window.About(host)); !strings.Contains(shown, text.DrawingWithSoftwareRenderer()) {
		t.Errorf("the About screen of a window drawn in software does not say so.\nIt shows:\n%s", shown)
	}
}
