package gui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The software renderer that ships beside the window binary on Windows.
//
// Two files of Mesa, in a directory of their own beside the executable: the
// renderer, which carries llvmpipe and LLVM, and the thin loader that Windows
// programs ask for OpenGL by name. Measured on 2026-09-17 on a Windows Server
// 2025 guest with no 3D acceleration, where the toolkit could not open a
// window at all: with these two loaded before the toolkit starts, the window
// opens, and the process's own module list shows nothing else answering to
// that name (docs/GUI-NO-OPENGL section 7).
//
// The order is not a preference. The loader imports the renderer BY NAME, and
// a dependency of a library loaded by path is searched for the ordinary way
// rather than in that library's own directory - so loaded second, the loader
// fails with "module not found" and nothing here has a renderer. Loaded first,
// the renderer is already mapped under its name when the loader asks.
//
// The driver is named out loud rather than left to the renderer, which
// otherwise picks one for itself: measured on a machine with a graphics card,
// the window came up and the process died within three seconds, twice out of
// twice, with no word on standard error. With the variable set it draws.
//
// In a directory of their own rather than beside the executable, because a
// file called opengl32.dll beside a program is the classic shape of a
// library hijack, and the only reason it works here is that the program asks
// for these two by absolute path - see software_windows.go.
const (
	softwareDir      = "opengl"
	softwareRenderer = "libgallium_wgl.dll"
	softwareLoader   = "opengl32.dll"
	softwareDriver   = "GALLIUM_DRIVER"
	softwareLLVMPipe = "llvmpipe"
)

// SoftwareFiles names the renderer's files beside the executable, in the
// order they have to be loaded. Every caller takes the order from here.
func SoftwareFiles(exeDir string) []string {
	return []string{
		filepath.Join(exeDir, softwareDir, softwareRenderer),
		filepath.Join(exeDir, softwareDir, softwareLoader),
	}
}

// softwareBeside reports whether every file of the renderer is beside the
// executable, and names the first one that is not.
func softwareBeside(exeDir string) (missing string) {
	for _, path := range SoftwareFiles(exeDir) {
		if _, err := os.Stat(path); err != nil {
			return path
		}
	}
	return ""
}

// SecondAttempt is the second attempt at a window, taken when the toolkit
// gave none: this program started again, with the same arguments plus the
// flag, so that the second process can load the software renderer before
// the toolkit asks the driver for anything.
//
// The pieces that touch the world are fields, so that the decision can be
// pressed by a guard on any system without a window, a renderer or a second
// process - which is also why the system's name is a field rather than read.
type SecondAttempt struct {
	Launch Launch
	GOOS   string
	// ExecutableDir is where the executable lives, or the error that stood
	// in the way of knowing.
	ExecutableDir func() (string, error)
	// Beside names the first file of the renderer that is not beside the
	// executable, or nothing when they all are.
	Beside func(exeDir string) (missing string)
	// Start starts this program again with args and answers its exit code.
	Start  func(args []string) (code int, err error)
	ErrOut io.Writer
}

// The reasons the fallback was not taken. Each becomes a sentence of the
// refusal through fallbackSentence, apart from the one that becomes none:
// on a system the renderer does not ship for, there is nothing missing to
// name. Their Error strings carry the fact and no words, because the words
// a person reads come from the text package and nowhere else.
type (
	// alreadySoftware: this IS the second process, and it does not start a
	// third. The refusal it shows says the renderer was tried.
	alreadySoftware struct{}
	// notShipped: no renderer ships for this system.
	notShipped struct{ goos string }
	// notBeside: the renderer's file is not where the archive put it.
	notBeside struct{ path string }
	// startFailed: the second process could not be started at all.
	startFailed struct{ err error }
)

func (alreadySoftware) Error() string { return SoftwareFlag }
func (e notShipped) Error() string    { return e.goos }
func (e notBeside) Error() string     { return e.path }
func (e startFailed) Error() string   { return e.err.Error() }
func (e startFailed) Unwrap() error   { return e.err }

// SecondAttemptFor builds the attempt the real window takes: this
// executable, this system, a real second process.
func SecondAttemptFor(launch Launch, errOut io.Writer) SecondAttempt {
	return SecondAttempt{
		Launch:        launch,
		GOOS:          runtime.GOOS,
		ExecutableDir: executableDir,
		Beside:        softwareBeside,
		Start:         startAgain,
		ErrOut:        errOut,
	}
}

// Try tries the window a second time, in a second process, and answers
// that process's exit code. When it does not try, it answers why, and the
// caller's refusal carries the reason - see RendererSentence.
//
// The second process never starts a third: a process started with the flag
// is the attempt, and if the toolkit refuses there too, the answer is the
// refusal, not a chain of processes each waiting for the next.
func (a SecondAttempt) Try() (int, error) {
	if a.Launch.SoftwareGL {
		return 0, alreadySoftware{}
	}
	if a.GOOS != "windows" {
		return 0, notShipped{a.GOOS}
	}
	dir, err := a.ExecutableDir()
	if err != nil {
		return 0, startFailed{err}
	}
	if missing := a.Beside(dir); missing != "" {
		return 0, notBeside{missing}
	}
	// Said out loud before the second process starts, on the stream whatever
	// started this one is reading: the driver refused, and this is what is
	// being done about it. Untouchable rule 6.
	fmt.Fprintln(a.ErrOut, text.StartingAgainWithSoftwareRenderer())
	code, err := a.Start(append(append([]string{}, a.Launch.Args...), SoftwareFlag))
	if err != nil {
		return 0, startFailed{err}
	}
	return code, nil
}

// executableDir is the directory the running program was started from.
func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// RendererSentence is what the refusal adds about the software renderer:
// why it was not tried, or that it was and did not help. Nothing on a
// system the renderer does not ship for, where a sentence about a missing
// file would be about a file nobody promised (owner's decision, 2026-09-17).
//
// why is what Try answered. loaded is what this process found when it
// loaded the renderer: nil when it loaded, the error when it did not, and
// nil as well in a first process that never tried.
func RendererSentence(why error, loaded error) string {
	var beside notBeside
	var failed startFailed
	var shipsNot notShipped
	switch {
	case errors.As(why, &beside):
		return text.RendererNotBeside(beside.path)
	case errors.As(why, &failed):
		return text.RendererStartFailed(failed.err)
	case errors.Is(why, alreadySoftware{}) && errors.As(loaded, &shipsNot):
		// Asked for by flag on a system nothing ships for, and refused by
		// the driver: the same silence as without the flag, because the
		// sentence would be about a file nobody promised there.
		return ""
	case errors.Is(why, alreadySoftware{}) && loaded != nil:
		return text.RendererNotLoaded(loaded)
	case errors.Is(why, alreadySoftware{}):
		return text.RendererDidNotHelp()
	default:
		return ""
	}
}

// LoadingSentence is what a window asked for the renderer says on standard
// error about the loading: that it draws with the renderer, that no
// renderer ships for this system, or what stood in the way. Nothing when
// loaded is nil and the flag was not given, which is every ordinary start.
func LoadingSentence(loaded error) string {
	var shipsNot notShipped
	switch {
	case errors.As(loaded, &shipsNot):
		return text.RendererNotShipped()
	case loaded != nil:
		return text.RendererNotLoaded(loaded)
	default:
		return text.DrawingWithSoftwareRenderer()
	}
}

// NotShippedFor is the reason loading the renderer answers on a system
// nothing ships for, for a guard on another system to hand to the sentences
// above - the reason is a type of this package and the guard cannot spell it.
func NotShippedFor(goos string) error { return notShipped{goos} }
