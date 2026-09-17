package guard

import (
	"go/ast"
	"testing"
)

// An entry of the two registries in notelemetry_test.go that forgive a
// computed library load or a spawn names the OPERATION it forgives, not the
// kind: what is called, and what answered the call's first argument. This
// file is how a call is followed back to that answer, and the canary that
// presses it. Read by two guards - the telemetry scan, about a way out, and
// hardening_test.go, about the search order - which is why it is a file of
// its own rather than a corner of either.

// An approvedOperation is the one call an entry forgives, matched by what
// is called and by where its first argument was answered: the call as the
// file spells it, and the function that call's first argument is bound to
// in the enclosing function. Both, because each alone forgives too much -
// the approved call handed a path worked out somewhere else, or the
// approved path handed to another call of the same kind. The first version
// matched the kind and counted one, and an outside review of #109 pointed
// out that exec.Command("curl") in the registered file would then have been
// the one forgiven spawn. from is never empty: an argument this guard cannot
// follow is an argument it cannot vouch for.
type approvedOperation struct {
	call string // exec.Command, syscall.LoadDLL - as the file spells it
	from string // os.Executable, SoftwareFiles - what answers the argument
	why  string
}

// forgives reports whether the entry is the operation found: the same call,
// handed an argument answered by the same function.
func (op approvedOperation) forgives(f telemetryFinding) bool {
	return f.call == op.call && f.from == op.from
}

// boundTo is the call an argument was answered by: the argument itself when
// it is a call, or - when it is a name - the one statement of the enclosing
// function that assigns or ranges a call into that name. Nothing when the
// argument is anything else, when the name is bound more than once, or when
// it is bound to something that is not a call - a parameter, a literal, a
// field, an expression - because a guard that cannot follow an argument has
// to say so rather than guess. The spelling is the file's: os.Executable
// for a package function, SoftwareFiles for one of the file's own package.
func boundTo(fn *ast.FuncDecl, arg ast.Expr) string {
	if call, ok := arg.(*ast.CallExpr); ok {
		return spelledCall(call)
	}
	name, ok := arg.(*ast.Ident)
	if !ok || fn == nil || fn.Body == nil {
		return ""
	}
	var sources []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name.Name {
					sources = append(sources, callOrNothing(s.Rhs))
				}
			}
		case *ast.RangeStmt:
			for _, side := range []ast.Expr{s.Key, s.Value} {
				if id, ok := side.(*ast.Ident); ok && id.Name == name.Name {
					sources = append(sources, callOrNothing([]ast.Expr{s.X}))
				}
			}
		}
		return true
	})
	if len(sources) != 1 {
		return ""
	}
	return sources[0]
}

// callOrNothing is the call on the right hand side of a binding when the
// whole right hand side is one call, and nothing otherwise.
func callOrNothing(rhs []ast.Expr) string {
	if len(rhs) != 1 {
		return ""
	}
	call, ok := rhs[0].(*ast.CallExpr)
	if !ok {
		return ""
	}
	return spelledCall(call)
}

// spelledCall is the function a call names, as the file spells it: pkg.Name
// for a selector on a plain name, Name for a function of the file's own
// package, and nothing for a method on an expression.
func spelledCall(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := fn.X.(*ast.Ident); ok {
			return pkg.Name + "." + fn.Sel.Name
		}
	case *ast.Ident:
		return fn.Name
	}
	return ""
}

// firstArgumentSource is what a call's first argument is bound to, or
// nothing when the call has none.
func firstArgumentSource(call *ast.CallExpr, fn *ast.FuncDecl) string {
	if len(call.Args) == 0 {
		return ""
	}
	return boundTo(fn, call.Args[0])
}

// describeSource is the words for where an argument came from, for a
// message - a function's name, or that the guard could not follow it.
func describeSource(from string) string {
	if from == "" {
		return "an argument this guard cannot follow"
	}
	return "what " + from + " answers"
}

// A registered file is forgiven the one operation its entry names - that
// call, handed what that function answers - and nothing beside it: a second
// of the same, the same call handed something else, another call handed
// the same thing. The canary above passes an unregistered name, so it could
// not see any of this: a registered file with two spawns was forgiven both,
// and then a registered file with any spawn was forgiven that one.
func TestARegisteredFileIsForgivenOneOperationAndNoMore(t *testing.T) {
	header := "package p\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"syscall\"\n)\n\n"
	approved := "func f() {\n\texe, _ := os.Executable()\n\t_ = exec.Command(exe)\n}\n"
	for _, c := range []struct {
		label string
		src   string
		rel   string
		kind  string
		left  int
	}{
		{"the approved spawn", header + approved, "internal/gui/again.go", "spawn", 0},
		{"the approved spawn and a second one", header + approved + "\nfunc g() { _ = exec.Command(\"curl\") }\n", "internal/gui/again.go", "spawn", 1},
		// Twice the approved one: the entry forgives one operation, and the
		// second copy is not forgiven for being identical - the mutation
		// runner found the shape missing here, because curl beside the
		// approved spawn is refused by the match alone.
		{"the approved spawn twice", header + approved + "\nfunc g() {\n\texe, _ := os.Executable()\n\t_ = exec.Command(exe)\n}\n", "internal/gui/again.go", "spawn", 1},
		{"the approved call handed another program", header + "func f() { _ = exec.Command(os.Args[0]) }\n", "internal/gui/again.go", "spawn", 1},
		{"another call handed the approved program", header + "func f() {\n\texe, _ := os.Executable()\n\t_, _ = os.StartProcess(exe, nil, nil)\n}\n", "internal/gui/again.go", "spawn", 1},
		{"the approved program bound twice on the way", header + "func f() {\n\texe, _ := os.Executable()\n\texe = os.Args[0]\n\t_ = exec.Command(exe)\n}\n", "internal/gui/again.go", "spawn", 1},
		{"the approved load", header + "func f(d string) {\n\tfor _, p := range SoftwareFiles(d) {\n\t\t_, _ = syscall.LoadDLL(p)\n\t}\n}\n", "internal/gui/software_windows.go", "library", 0},
		{"the approved load twice", header + "func f(d string) {\n\tfor _, p := range SoftwareFiles(d) {\n\t\t_, _ = syscall.LoadDLL(p)\n\t}\n}\n\nfunc g(d string) {\n\tfor _, p := range SoftwareFiles(d) {\n\t\t_, _ = syscall.LoadDLL(p)\n\t}\n}\n", "internal/gui/software_windows.go", "library", 1},
		{"the approved call handed a path from somewhere else", header + "func f(p string) { _, _ = syscall.LoadDLL(p) }\n", "internal/gui/software_windows.go", "library", 1},
		{"the approved call handed a path built in place", header + "func f(d string) { _, _ = syscall.LoadDLL(d + \"\\\\opengl32.dll\") }\n", "internal/gui/software_windows.go", "library", 1},
		{"another call handed the approved path", header + "func f(d string) {\n\tfor _, p := range SoftwareFiles(d) {\n\t\t_ = syscall.NewLazyDLL(p)\n\t}\n}\n", "internal/gui/software_windows.go", "library", 1},
		// The registry is per file on purpose: the approved operation in a
		// file that is not registered for it is what it would be anywhere
		// else, a finding.
		{"the approved spawn in a file not registered for it", header + approved, "internal/cli/cli.go", "spawn", 1},
		{"the approved load in a file not registered for it", header + "func f(d string) {\n\tfor _, p := range SoftwareFiles(d) {\n\t\t_, _ = syscall.LoadDLL(p)\n\t}\n}\n", "internal/gui/darkmenus_windows.go", "library", 1},
	} {
		if kinds := kindsIn(telemetryFindings(c.src, c.rel)); kinds[c.kind] != c.left {
			t.Errorf("%s in %s left %d %s finding(s), and %d had to be left", c.label, c.rel, kinds[c.kind], c.kind, c.left)
		}
	}
}
