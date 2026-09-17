package guard

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A window the toolkit could not create is a refusal, not a process with no
// window in it.
//
// Measured on 2026-09-16 on a Windows Server 2025 guest without 3D
// acceleration (O218): the toolkit's window creation fails with "WGL: The
// driver does not appear to support OpenGL", it logs that to a standard
// error the windows-subsystem binary does not have, and Run then waits
// forever on a channel nobody closes. No window, no message, no exit code -
// untouchable rule 6 broken three times at once.
//
// The decision is taken from the window's STATE: the driver's NativeWindow
// answers with a zero handle on every platform when the window was never
// created. That is what this pins, with a stand-in window whose handle is
// whatever the case says - the test driver has no OpenGL and cannot be made
// to fail the real way, so the seam is the only place a guard can reach.
//
// The real thing is checked the way the rest of run_cgo.go is, by a run of
// the binary: docs/GUI-NO-OPENGL-2026-09-16.md.

// plainWindow is a window that records being shown and cannot say whether
// it has a native one - the shape of the test driver.
type plainWindow struct {
	fyne.Window
	calls []string
}

func (w *plainWindow) Show() { w.calls = append(w.calls, "show") }

// nativeWindow is a window whose driver answers NativeWindow with whatever
// context the case gave it.
type nativeWindow struct {
	plainWindow
	context any
}

func (w *nativeWindow) RunNative(f func(any)) { f(w.context) }

func TestAWindowTheToolkitGaveNoWindowIsRefusedAndNeverRun(t *testing.T) {
	test.NewApp()
	t.Cleanup(func() { test.NewApp() })

	// Every platform context the driver hands out, empty and filled. The
	// filled ones use 1 rather than a real handle: the question is zero or
	// not, and a real handle is what the toolkit has when it has a window.
	refused := map[string]any{
		"windows": driver.WindowsWindowContext{},
		"mac":     driver.MacWindowContext{},
		"x11":     driver.X11WindowContext{},
		"wayland": driver.WaylandWindowContext{},
	}
	opened := map[string]any{
		"windows": driver.WindowsWindowContext{HWND: 1},
		"mac":     driver.MacWindowContext{NSWindow: 1},
		"x11":     driver.X11WindowContext{WindowHandle: 1},
		"wayland": driver.WaylandWindowContext{WaylandSurface: 1},
	}

	// Every refused window is tried again, and what the second attempt
	// answers is what the seam answers: the exit code of the process that
	// carried it, or - when there was none - the refusal, with the reason
	// the attempt gave, and 1.
	for name, context := range refused {
		w := &nativeWindow{plainWindow: plainWindow{Window: test.NewWindow(nil)}, context: context}
		code := openOrRefuse(t, w, &w.calls, noSecondAttempt)
		if got := strings.Join(w.calls, ","); got != "show,again,refuse" {
			t.Errorf("%s, no native window, no second attempt: the calls were %q and they have to be show,again,refuse.\n"+
				"Show first, because only after it can the toolkit have failed - then the second "+
				"attempt, then the refusal, and NOT the loop, which on such a machine never ends.", name, got)
		}
		if code != 1 {
			t.Errorf("%s, no native window: exit code %d, and a window that could not open exits 1 "+
				"like a build with no window in it.", name, code)
		}

		w = &nativeWindow{plainWindow: plainWindow{Window: test.NewWindow(nil)}, context: context}
		code = openOrRefuse(t, w, &w.calls, secondAttemptAnswered(7))
		if got := strings.Join(w.calls, ","); got != "show,again" {
			t.Errorf("%s, no native window, second attempt carried: the calls were %q and they have to be show,again - "+
				"no refusal, because the second process put the window on the screen, and no loop, "+
				"because that process ran it.", name, got)
		}
		if code != 7 {
			t.Errorf("%s, second attempt carried: exit code %d, and it has to be the second process's own, 7.", name, code)
		}
	}

	for name, context := range opened {
		w := &nativeWindow{plainWindow: plainWindow{Window: test.NewWindow(nil)}, context: context}
		code := openOrRefuse(t, w, &w.calls, secondAttemptAnswered(7))
		if got := strings.Join(w.calls, ","); got != "show,run" {
			t.Errorf("%s, native window present: the calls were %q and they have to be show,run - "+
				"the same two calls ShowAndRun makes, in the same order, and no second attempt.", name, got)
		}
		if code != 0 {
			t.Errorf("%s, native window present: exit code %d after a run that returned normally.", name, code)
		}
	}

	// A driver that cannot answer at all, and one that answers with a
	// context this code does not know: both are taken as a window that
	// opened, because nothing about them is known to have failed. The test
	// driver is the first kind, and every guard drawing a screen rests on it.
	plain := &plainWindow{Window: test.NewWindow(nil)}
	if code := openOrRefuse(t, plain, &plain.calls, secondAttemptAnswered(7)); code != 0 || strings.Join(plain.calls, ",") != "show,run" {
		t.Errorf("a window whose driver has no NativeWindow: code %d, calls %q - it has to run, "+
			"because the test driver is such a window and it draws every screen this package checks.",
			code, strings.Join(plain.calls, ","))
	}
	unknown := &nativeWindow{plainWindow: plainWindow{Window: test.NewWindow(nil)}, context: struct{}{}}
	if code := openOrRefuse(t, unknown, &unknown.calls, secondAttemptAnswered(7)); code != 0 || strings.Join(unknown.calls, ",") != "show,run" {
		t.Errorf("a window whose driver answers with an unknown context: code %d, calls %q - "+
			"cannot tell is not the same as failed.", code, strings.Join(unknown.calls, ","))
	}
}

// errNoSecondAttempt is the reason a second attempt gives when there is
// none, and the refusal has to receive exactly it.
var errNoSecondAttempt = errors.New("no second attempt in this case")

// noSecondAttempt is a second attempt that was not to be had.
func noSecondAttempt() (int, error) { return 0, errNoSecondAttempt }

// secondAttemptAnswered is a second attempt carried by a process that
// exited with code.
func secondAttemptAnswered(code int) func() (int, error) {
	return func() (int, error) { return code, nil }
}

// openOrRefuse drives the seam with a window that records every call in
// calls, and holds the refusal to receiving the reason the second attempt
// gave. The window is passed as the seam sees it, so a driver's
// NativeWindow is found when the case has one.
func openOrRefuse(t *testing.T, w fyne.Window, calls *[]string, again func() (int, error)) int {
	t.Helper()
	return gui.OpenOrRefuse(w,
		func() { *calls = append(*calls, "run") },
		func() (int, error) {
			*calls = append(*calls, "again")
			return again()
		},
		func(why error) {
			*calls = append(*calls, "refuse")
			if !errors.Is(why, errNoSecondAttempt) {
				t.Errorf("the refusal was given %v and it has to be given the reason the second attempt gave, "+
					"because the sentence the person reads is built from it.", why)
			}
		})
}

// The three lines the toolkit wrote on the guest, verbatim from the
// measurement, with the module path the way a -trimpath build spells it.
const toolkitSaid = "2026/09/16 12:06:56 Fyne error:  window creation error\n" +
	"2026/09/16 12:06:56   Cause: APIUnavailable: WGL: The driver does not appear to support OpenGL\n" +
	"2026/09/16 12:06:56   At: fyne.io/fyne/v2@v2.8.1/internal/driver/glfw/driver.go:180\n"

func TestTheToolkitsOwnReasonReachesTheSentenceAndItsAbsenceDoesNotBreakIt(t *testing.T) {
	wantCause := "APIUnavailable: WGL: The driver does not appear to support OpenGL"
	if got := gui.CauseFrom(toolkitSaid); got != wantCause {
		t.Errorf("the cause read from what the toolkit logged is %q, wanted %q.\n"+
			"That line is the one thing on the guest that names the driver, and it is the "+
			"toolkit's sentence rather than ours, so it is quoted.", got, wantCause)
	}
	if got := gui.CauseFrom("2026/09/16 12:06:56 Fyne error:  window creation error\n"); got != "" {
		t.Errorf("a log with no cause line gave %q, and it has to give nothing - the refusal "+
			"does not depend on the detail.", got)
	}

	// No catalogue loaded: every sentence answers with its English then, which
	// is what a build whose catalogue failed to read answers too.
	with := text.WindowRefused(wantCause)
	without := text.WindowRefused("")
	for name, sentence := range map[string]string{"with a cause": with, "without one": without} {
		for _, must := range []string{"tfg --help", "OpenGL 2.1"} {
			if !strings.Contains(sentence, must) {
				t.Errorf("the refusal %s does not say %q: %q\n"+
					"Four parts, D6 - what did not happen, why, what works instead, what to do about it.", name, must, sentence)
			}
		}
		if strings.Contains(sentence, "{{") {
			t.Errorf("the refusal %s carries an unfilled template: %q", name, sentence)
		}
	}
	if !strings.Contains(with, wantCause) {
		t.Errorf("the refusal with a cause does not quote it: %q", with)
	}
	if strings.Contains(without, "said:") {
		t.Errorf("the refusal without a cause quotes an empty one: %q", without)
	}
}

// addressedFirst is the name of the variable whose address is the first
// argument of call, as in io.MultiWriter(&said, ...), or nothing.
func addressedFirst(call *ast.CallExpr) string {
	if len(call.Args) == 0 {
		return ""
	}
	first, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || first.Op != token.AND {
		return ""
	}
	if buffer, ok := first.X.(*ast.Ident); ok {
		return buffer.Name
	}
	return ""
}

// stringOf is the name of x in a call shaped f(x.String()), or nothing.
func stringOf(call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return ""
	}
	inner, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "String" {
		return ""
	}
	if buffer, ok := sel.X.(*ast.Ident); ok {
		return buffer.Name
	}
	return ""
}

// The window binary goes through the seam above and never through
// ShowAndRun, and the copy of the toolkit's log is written before the
// toolkit's own stream is - a multi-writer stops at the first writer that
// fails, and the stream a windows-subsystem binary does not have is exactly
// such a writer. Read from the source, because run_cgo.go is behind cgo and
// the test driver cannot fail the way the guest did.
//
// Since 2026-09-17 it also asks that the real window takes the second
// attempt the seam offers - the one built by SecondAttemptFor, which starts
// this program again with the software renderer - and that a window asked
// for the renderer loads it, through LoadSoftwareRenderer. Both calls are
// behind cgo too, so presence in the source is what can be asked. What
// they do when reached was measured by running the binary, on a machine
// with a driver and on a guest without one.
func TestTheWindowBinaryOpensThroughTheRefusalSeam(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	seamCalls, showAndRun := 0, []string{}
	secondAttempt, loadsRenderer := 0, 0
	// The buffer the cause is read from, and the buffer the copy of the log
	// goes to. They have to be the same variable - named by the code, not
	// by this guard, because "the first writer is an address" passed for
	// the address of any buffer at all (an outside review of the pull
	// request named it), and a copy into some other buffer leaves the one
	// CauseFrom reads empty.
	readFrom, copiedTo := "", ""

	err := filepath.WalkDir(filepath.Join(root, "internal", "gui"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				switch fun.Sel.Name {
				case "ShowAndRun":
					showAndRun = append(showAndRun, rel)
				case "MultiWriter":
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "io" && strings.HasSuffix(rel, "run_cgo.go") {
						copiedTo = addressedFirst(call)
					}
				}
			case *ast.Ident:
				if !strings.HasSuffix(rel, "run_cgo.go") {
					return true
				}
				switch fun.Name {
				case "OpenOrRefuse":
					seamCalls++
				case "CauseFrom":
					readFrom = stringOf(call)
				case "SecondAttemptFor":
					secondAttempt++
				case "LoadSoftwareRenderer":
					loadsRenderer++
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/gui: %v", err)
	}

	if len(showAndRun) != 0 {
		t.Errorf("ShowAndRun is called in %v.\n"+
			"It is Show then Run with no question between them, and on a machine where the "+
			"toolkit cannot create a window it runs a loop nothing ends (O218). Open through "+
			"gui.OpenOrRefuse instead.", showAndRun)
	}
	if seamCalls != 1 {
		t.Errorf("run_cgo.go calls OpenOrRefuse %d time(s), and the window binary opens through "+
			"it exactly once - that is the seam the guard above reaches.", seamCalls)
	}
	if secondAttempt != 1 {
		t.Errorf("run_cgo.go builds the second attempt with SecondAttemptFor %d time(s), and the "+
			"real window takes exactly the one the guard above presses: without it, a guest with "+
			"no driver gets the refusal and never the renderer.", secondAttempt)
	}
	if loadsRenderer != 1 {
		t.Errorf("run_cgo.go calls LoadSoftwareRenderer %d time(s), and a window started with "+
			"--software-gl loads the renderer exactly once, before the toolkit asks the driver "+
			"for anything.", loadsRenderer)
	}
	if readFrom == "" {
		t.Fatalf("run_cgo.go does not read the cause with CauseFrom(<buffer>.String()), so this " +
			"guard cannot tell which buffer the copy of the toolkit's log has to reach.")
	}
	if copiedTo != readFrom {
		t.Errorf("run_cgo.go hands io.MultiWriter %q first, and CauseFrom reads %q.\n"+
			"io.MultiWriter stops at the first writer that fails. The toolkit's own stream is "+
			"standard error, which a windows-subsystem binary does not have, so the copy has to "+
			"come first - and it has to be the buffer the cause is read from, or that buffer stays "+
			"empty and the refusal says nothing about the driver.", copiedTo, readFrom)
	}
}
