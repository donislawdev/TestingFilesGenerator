package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// O63: everything the window is told from a worker crosses through fyne.Do.
//
// Fyne 2.8 requires it and says so at every start - "this application has not
// been migrated to the fyne.Do threading model" - and adds that the next major
// release takes the safety net away. engine.Options.OnProgress is called from
// the goroutine doing the writing, which is exactly the shape the warning is
// about.
//
// This guard is static, and that is a measurement rather than a preference.
// Measured on 2026-08-05 against Fyne v2.8.0:
//
//	fyne.Do under the test driver runs the function on the CALLING goroutine
//	a widget setter called straight from a worker does not complain at all
//
// So a test that starts a run and watches the screen proves nothing about
// threading, and -race sees nothing either, because the race is inside the
// toolkit rather than in our memory. What is left is reading the code, and
// reading it by hand is the thing that stops happening in week three.
//
// The rule is deny by default. Inside a body that runs off the interface
// thread, a call that changes what is on screen has to sit inside a fyne.Do
// closure. Anything else - the engine, the disk, arithmetic - is free.

// interfaceCalls are the calls that change what is on screen.
//
// By name rather than by type, because these files are parsed rather than type
// checked. Being a name, it can be fooled by a method of ours that happens to
// share one - which costs a false positive and never a false negative, and that
// is the right way round for a rule about threading.
var interfaceCalls = map[string]bool{
	"SetText": true, "SetValue": true, "SetContent": true, "SetChecked": true,
	"SetSelected": true, "SetOptions": true, "SetPlaceHolder": true,
	"SetCloseIntercept": true, "Refresh": true, "Show": true, "Hide": true,
	"Enable": true, "Disable": true, "RemoveAll": true, "Resize": true,
	"Say": true, "Clear": true, "Close": true,
}

// interfaceFields are the fields of a widget that can be written to instead of
// being set through a method. Assigning to one is the same crossing by a
// quieter route, and a rule that only watched calls would wave it through.
var interfaceFields = map[string]bool{
	"Text": true, "Value": true, "Options": true, "Selected": true,
	"Checked": true, "Content": true, "Importance": true, "Wrapping": true,
}

// marshalling are the two ways onto the interface thread.
var marshalling = map[string]bool{"Do": true, "DoAndWait": true}

func TestTheWindowIsOnlyTouchedFromTheInterfaceThread(t *testing.T) {
	var found []string
	bodies := 0

	for _, p := range packages(t) {
		if !strings.HasPrefix(p.rel, "internal/gui") {
			continue
		}
		for _, path := range p.files {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			rel := relative(t, path)
			touches := interfaceTouchers(file)

			for _, body := range offThreadBodies(file) {
				bodies++
				found = append(found, crossings(fset, rel, body, touches)...)
			}
		}
	}

	// A rule nobody breaks because nobody does the thing any more is a rule
	// that has stopped being checked. Removing the goroutine would leave this
	// green while proving nothing, so the count is asserted too.
	if bodies == 0 {
		t.Fatal("no off thread body was examined - this guard would pass without checking anything. " +
			"If the window genuinely no longer works in the background, delete this guard and say so in docs/OBSERVATIONS.md")
	}

	if len(found) > 0 {
		sort.Strings(found)
		t.Errorf("%d place(s) in the window change the screen from a worker without going through fyne.Do:\n  %s\n\n"+
			"Fyne 2.8 warns about this at every start and the next major release stops tolerating it. "+
			"Neither -race nor the test driver can see it, which is why this is read rather than run - "+
			"measured 2026-08-05, see O63. Wrap the change in fyne.Do(func() { ... }).",
			len(found), strings.Join(found, "\n  "))
	}
	t.Logf("%d off thread body(s) examined, every screen change inside fyne.Do", bodies)
}

func relative(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

// interfaceTouchers is every function in a file that reaches the screen, whether
// it does so itself or through another one of ours.
//
// Worked out to a fixed point rather than one level deep, because one level is
// bypassed by writing a helper. A worker calling runFinished, which calls
// setRunning, which disables a button, is the same crossing three names further
// away.
func interfaceTouchers(file *ast.File) map[string]bool {
	direct := map[string]bool{}
	calls := map[string][]string{}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if touchesInterface(n) {
				direct[name] = true
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok {
					calls[name] = append(calls[name], id.Name)
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					calls[name] = append(calls[name], sel.Sel.Name)
				}
			}
			return true
		})
	}

	for changed := true; changed; {
		changed = false
		for name, called := range calls {
			if direct[name] {
				continue
			}
			for _, c := range called {
				if direct[c] {
					direct[name] = true
					changed = true
					break
				}
			}
		}
	}
	return direct
}

// touchesInterface reports whether one node changes what is on screen.
func touchesInterface(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.CallExpr:
		sel, ok := v.Fun.(*ast.SelectorExpr)
		return ok && interfaceCalls[sel.Sel.Name] && !isMarshalling(sel)
	case *ast.AssignStmt:
		for _, lhs := range v.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && interfaceFields[sel.Sel.Name] {
				return true
			}
		}
	}
	return false
}

// isMarshalling reports whether a call is fyne.Do or fyne.DoAndWait, which are
// the way across rather than a crossing.
func isMarshalling(sel *ast.SelectorExpr) bool {
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "fyne" && marshalling[sel.Sel.Name]
}

// offThreadBodies are the function bodies that do not run on the interface
// thread: the body of a goroutine, and the callback the engine reports progress
// through, which it calls from the goroutine doing the writing.
func offThreadBodies(file *ast.File) []*ast.BlockStmt {
	var out []*ast.BlockStmt
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.GoStmt:
			if lit, ok := v.Call.Fun.(*ast.FuncLit); ok {
				out = append(out, lit.Body)
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "OnProgress" || i >= len(v.Rhs) {
					continue
				}
				if lit, ok := v.Rhs[i].(*ast.FuncLit); ok {
					out = append(out, lit.Body)
				}
			}
		}
		return true
	})
	return out
}

// crossings walks one off thread body and reports every change to the screen
// that is not inside a fyne.Do closure.
func crossings(fset *token.FileSet, rel string, body *ast.BlockStmt, touches map[string]bool) []string {
	var out []string

	// The closures that are on the interface thread, so what is inside them is
	// allowed. Collected first, because a walk meets the inner nodes before it
	// can know which closure they sit in.
	safe := map[ast.Node]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !isMarshalling(sel) || len(call.Args) == 0 {
			return true
		}
		if lit, ok := call.Args[0].(*ast.FuncLit); ok {
			ast.Inspect(lit.Body, func(inner ast.Node) bool {
				safe[inner] = true
				return true
			})
		}
		return true
	})

	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil || safe[n] {
			return true
		}
		what := ""
		switch {
		case touchesInterface(n):
			what = "changes a widget"
		default:
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := calledName(call)
			if name == "" || !touches[name] {
				return true
			}
			what = "calls " + name + ", which changes a widget"
		}
		out = append(out, fmt.Sprintf("%s:%d %s", rel, fset.Position(n.Pos()).Line, what))
		return true
	})
	return out
}

func calledName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if isMarshalling(fn) {
			return ""
		}
		return fn.Sel.Name
	}
	return ""
}
