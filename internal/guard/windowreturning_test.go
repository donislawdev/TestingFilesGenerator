package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The foreground hook tells the control holding the keyboard that the window
// is coming back, and leaves a control that cannot be told alone.
//
// The other half of TestTheWindowComingBackDrawsOnlyTheMarkThatWasThere,
// which drives a control the way the driver does and takes the hook on
// trust. An outside review of the pull request asked for the hook itself
// to be under a guard, and it was right: the hook lived inside Run, behind
// cgo, where no guard reaches, so a registration taken out or a type guard
// widened would have gone unnoticed until somebody opened the window and
// saw the first menu blue again. gui.WindowReturning is the hook's body in
// a file of its own, and this asks it two things with a canvas of the test
// driver's: a focused menu told the window is returning draws no mark on
// the FocusGained that follows, and a focused box to type in - which is not
// Returnable, and whose focused look is the toolkit's own - is left alone
// rather than made to panic. Nothing focused is the third case.
func TestTheForegroundHookTellsTheFocusedControlTheWindowIsBack(t *testing.T) {
	c, content := screenOnACanvas(t)

	menu := chooserUnder(t, content, text.FieldFormat())
	parts.FocusQuietly(c, menu)
	if c.Focused() != fyne.Focusable(menu) || menu.Marked() {
		t.Fatal("the menu does not hold the keyboard quietly, so this guard is not starting from the state it is about")
	}
	menu.FocusLost()
	gui.WindowReturning(c)
	menu.FocusGained()
	if menu.Marked() {
		t.Error("the hook ran and the focused menu still drew the keyboard mark on the window's return - the hook told it nothing")
	}

	// A box to type in cannot be told, and is not to be broken by the
	// telling. Its own FocusGained is the toolkit's and draws the box's
	// focused edge, which is right: the caret comes back with the window.
	box := entryUnder(t, content, text.FieldSize())
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	c.Focus(box)
	if c.Focused() != fyne.Focusable(box) {
		t.Fatal("the size box did not take the keyboard, so the second half of this guard cannot start")
	}
	gui.WindowReturning(c)

	// And nothing at all holding the keyboard.
	c.Unfocus()
	gui.WindowReturning(c)
	gui.WindowReturning(nil)
}

// And the real window registers exactly that, in the toolkit's foreground
// hook, once - read out of the source, because the registration is behind
// cgo where no guard runs. A registration taken out, or a hook registered
// with some other body, is a window whose first menu opens blue again.
func TestTheWindowBinaryRegistersTheForegroundHook(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, "internal", "gui", "run_cgo.go"), nil, 0)
	if err != nil {
		t.Fatalf("parsing run_cgo.go: %v", err)
	}
	registered, telling := 0, 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if fun, ok := call.Fun.(*ast.SelectorExpr); ok && fun.Sel.Name == "SetOnEnteredForeground" {
			registered++
			// The body of what is registered has to call WindowReturning,
			// so the hook and the guarded function are one thing.
			for _, arg := range call.Args {
				ast.Inspect(arg, func(inner ast.Node) bool {
					if c, ok := inner.(*ast.CallExpr); ok {
						if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "WindowReturning" {
							telling++
						}
					}
					return true
				})
			}
		}
		return true
	})
	if registered != 1 {
		t.Errorf("run_cgo.go registers the foreground hook %d time(s), and the window binary registers it exactly once", registered)
	}
	if telling != 1 {
		t.Errorf("the registered foreground hook calls WindowReturning %d time(s), and it is the one thing the hook is for", telling)
	}
}
