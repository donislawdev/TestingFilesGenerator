package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Two rules about the one thing the test driver cannot see: whether the
// canvas was TOLD that something changed.
//
// The driver paints when its canvas is dirty, and the canvas is dirty when an
// object it has painted before asks to be. That "before" is the whole trap.
// The driver files each object it paints under the value the tree holds -
// type and pointer together - so a refresh sent for any other value finds no
// canvas, marks nothing and says nothing. Measured on 2026-09-16 in a copy of
// the window with the driver logging every refresh (O217): the pointer
// arrived on the explanation button, the button's face changed, the
// explanation was put on its sheet, and the driver's log ended there. No
// repaint. The refresh had been asked for the Button embedded inside
// DetailButton, and the tree holds the DetailButton.
//
// The test driver answers CanvasForObject with the last window's canvas
// whatever the object, and that canvas ignores Refresh - so every guard that
// hovers, taps or reads the tree stayed green through this, and would stay
// green through it again. What can be asked in a test is the shape of the
// code, so that is what these two ask, mechanically and over both packages
// the window is built from.
//
// What neither can see: a piece or a box held in a local variable rather than
// in a field, a write hidden in a function that is not a method of the
// renderer, one element of a slice written and another named, and a box
// changed on a path that returns before the refresh at the end of the
// function. The copy of the window with the logging driver is the instrument
// for those, and it lives in tools, not here.

// canvasSource is one parsed production file of the window's code.
type canvasSource struct {
	rel  string
	fset *token.FileSet
	file *ast.File
}

// windowSources parses internal/gui/parts and internal/gui/window, production
// files only, and fails rather than returning nothing.
func windowSources(t *testing.T) []canvasSource {
	t.Helper()
	var out []canvasSource
	for _, pkg := range []string{"parts", "window"} {
		dir := filepath.Join(repoRoot(t), "internal", "gui", pkg)
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("listing %s: %v", dir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", file, err)
			}
			out = append(out, canvasSource{rel: "internal/gui/" + pkg + "/" + filepath.Base(file), fset: fset, file: parsed})
		}
	}
	return out
}

// declaredFields indexes every struct type declared in the sources: type name,
// then field name, then the field's type as it is written.
func declaredFields(sources []canvasSource) map[string]map[string]string {
	index := map[string]map[string]string{}
	for _, src := range sources {
		for _, decl := range src.file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				collectDeclaredFields(spec, index)
			}
		}
	}
	return index
}

func collectDeclaredFields(spec ast.Spec, index map[string]map[string]string) {
	ts, ok := spec.(*ast.TypeSpec)
	if !ok {
		return
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return
	}
	fields := map[string]string{}
	for _, field := range st.Fields.List {
		written := types.ExprString(field.Type)
		for _, name := range field.Names {
			fields[name.Name] = written
		}
	}
	index[ts.Name.Name] = fields
}

// methodBody is one method with the file it was read from, so a reader that
// follows a call into it can still say where a line is.
type methodBody struct {
	src canvasSource
	fn  *ast.FuncDecl
}

// methodsOf indexes the methods each receiver type answers, by name.
func methodsOf(sources []canvasSource) map[string]map[string]methodBody {
	index := map[string]map[string]methodBody{}
	for _, src := range sources {
		for _, decl := range src.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if _, typeName := receiverOf(fn); typeName != "" {
				if index[typeName] == nil {
					index[typeName] = map[string]methodBody{}
				}
				index[typeName][fn.Name.Name] = methodBody{src: src, fn: fn}
			}
		}
	}
	return index
}

// answers says whether a type has every one of these methods.
func answers(has map[string]methodBody, names ...string) bool {
	for _, name := range names {
		if _, ok := has[name]; !ok {
			return false
		}
	}
	return true
}

// receiverOf is the receiver's name and type of a method, both empty for a
// plain function. A pointer receiver reports the type it points at.
func receiverOf(fn *ast.FuncDecl) (ident, typeName string) {
	if fn.Recv == nil || len(fn.Recv.List) != 1 {
		return "", ""
	}
	recv := fn.Recv.List[0]
	if len(recv.Names) == 1 {
		ident = recv.Names[0].Name
	}
	return ident, strings.TrimPrefix(types.ExprString(recv.Type), "*")
}

// fieldChain reduces an expression of the shape recv.a.b to the field names
// a, b when it starts at the receiver, and returns nil for anything else.
func fieldChain(expr ast.Expr, recv string) []string {
	var chain []string
	for {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		chain = append([]string{sel.Sel.Name}, chain...)
		expr = sel.X
	}
	if id, ok := expr.(*ast.Ident); !ok || id.Name != recv || recv == "" || len(chain) == 0 {
		return nil
	}
	return chain
}

// pieceChain is fieldChain looking through an index: r.fills[i] and r.fills
// name the same field, because the slice is the piece a renderer holds and the
// index is which one it is drawing this time.
func pieceChain(expr ast.Expr, recv string) []string {
	var chain []string
	for {
		switch e := expr.(type) {
		case *ast.SelectorExpr:
			chain = append([]string{e.Sel.Name}, chain...)
			expr = e.X
			continue
		case *ast.IndexExpr:
			expr = e.X
			continue
		case *ast.ParenExpr:
			expr = e.X
			continue
		}
		break
	}
	if id, ok := expr.(*ast.Ident); !ok || id.Name != recv || recv == "" || len(chain) == 0 {
		return nil
	}
	return chain
}

// resolveChain follows field names down from a struct type and returns the
// type of the last one as written, or "" when the chain leaves what the
// sources declare.
func resolveChain(fields map[string]map[string]string, typeName string, chain []string) string {
	written := ""
	for _, name := range chain {
		declared, ok := fields[bareType(typeName)]
		if !ok {
			return ""
		}
		written, ok = declared[name]
		if !ok {
			return ""
		}
		typeName = written
	}
	return written
}

// bareType is a written type without the pointer and slice marks in front of
// it, so a field holding one piece and a field holding a row of them resolve
// to the same declared type.
func bareType(written string) string { return strings.TrimLeft(written, "[]*") }

// calledOn is the receiver expression and method name of a method call, or
// nil for anything else.
func calledOn(n ast.Node) (ast.Expr, string, *ast.CallExpr) {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return nil, "", nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, "", nil
	}
	return sel.X, sel.Sel.Name, call
}

// A renderer redraws the pieces it draws, and never the widget.
//
// The rule is mechanical because the failure is silent: canvas.Refresh handed
// the wrong value is not an error, it is nothing. So no code in parts or
// window calls canvas.Refresh at all, every renderer's Refresh names its pieces
// through redraw, and none of those pieces is a widget of this package - a
// widget of this package asked to refresh would ask the canvas about itself,
// which is the same wrong question one step removed.
//
// And EVERY piece the face changes is named, not merely one. Until 2026-09-16
// this counted the arguments of redraw and stopped, so a renderer setting the
// colour of four pieces and naming three passed - an outside review of the
// pull request pointed at the count. A piece is changed when a field of it is
// set, or when it is shown: Show on a rectangle, an image, a text or a
// container sets Hidden and stops (canvas/base.go, fyne v2.8.1, read in the
// pinned module), where Hide, Move and Resize repaint by themselves. The
// reading follows the renderer into its own methods, because two of the seven
// set their colours in a helper and a reader stopping at Refresh would have
// read nothing about them and stayed green for it.
func TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget(t *testing.T) {
	sources := windowSources(t)
	fields := declaredFields(sources)
	methods := methodsOf(sources)

	widgets := map[string]bool{}
	renderers := map[string]bool{}
	for typeName, has := range methods {
		if answers(has, "CreateRenderer") {
			widgets[typeName] = true
		}
		if answers(has, "Refresh", "Layout", "MinSize", "Objects", "Destroy") {
			renderers[typeName] = true
		}
	}
	if len(renderers) == 0 || len(widgets) == 0 {
		t.Fatalf("found %d renderers and %d widgets, so this guard read the wrong tree", len(renderers), len(widgets))
	}

	var offences []string
	checked, pieces := 0, 0
	for _, src := range sources {
		for _, decl := range src.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			offences = append(offences, canvasRefreshCalls(src, fn)...)
			recv, typeName := receiverOf(fn)
			if !renderers[typeName] || fn.Name.Name != "Refresh" {
				continue
			}
			checked++
			read := newRendererReading(recv, typeName, fields, widgets, methods)
			read.walk(src, fn)
			pieces += len(read.changed)
			offences = append(offences, read.verdict(src, fn)...)
		}
	}
	if checked == 0 || pieces == 0 {
		t.Fatalf("%d renderer Refresh methods read and %d changed pieces found, so this guard would pass against anything", checked, pieces)
	}
	t.Logf("%d renderer Refresh methods read in parts, %d pieces changed by them, %d widget types known", checked, pieces, len(widgets))
	sort.Strings(offences)
	for _, o := range offences {
		t.Error(o)
	}
}

// canvasRefreshCalls names every canvas.Refresh call in a function.
func canvasRefreshCalls(src canvasSource, fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn, func(n ast.Node) bool {
		on, method, call := calledOn(n)
		if call == nil || method != "Refresh" {
			return true
		}
		if pkg, ok := on.(*ast.Ident); ok && pkg.Name == "canvas" {
			out = append(out, fmt.Sprintf("%s:%d calls canvas.Refresh - a refresh asked for a value the tree does not hold "+
				"reaches no canvas and says nothing, so a renderer names its pieces through redraw instead",
				src.rel, src.fset.Position(call.Pos()).Line))
		}
		return true
	})
	return out
}

// rendererReading is what one renderer's Refresh does to the pieces it holds,
// read through every method of the renderer that Refresh calls.
type rendererReading struct {
	recv, typeName string
	fields         map[string]map[string]string
	widgets        map[string]bool
	methods        map[string]map[string]methodBody

	// changed is each piece a field was set on or that was shown, with where
	// that first happened, and named is each piece handed to redraw.
	changed  map[string]string
	named    map[string]bool
	offences []string
	visited  map[string]bool
}

func newRendererReading(recv, typeName string, fields map[string]map[string]string,
	widgets map[string]bool, methods map[string]map[string]methodBody) *rendererReading {
	return &rendererReading{recv: recv, typeName: typeName, fields: fields, widgets: widgets, methods: methods,
		changed: map[string]string{}, named: map[string]bool{}, visited: map[string]bool{}}
}

// piece is the field of the renderer an expression names, through any index,
// and whether that field holds a widget of this package - which is not a piece
// but the thing wearing the face.
func (rd *rendererReading) piece(expr ast.Expr) (key string, widget bool) {
	chain := pieceChain(expr, rd.recv)
	if chain == nil {
		return "", false
	}
	written := resolveChain(rd.fields, rd.typeName, chain)
	return strings.Join(chain, "."), rd.widgets[bareType(written)]
}

// walk reads one method: every field set on a piece, every piece shown, every
// piece named through redraw, and every method of this renderer called along
// the way, once each.
func (rd *rendererReading) walk(src canvasSource, fn *ast.FuncDecl) {
	if rd.visited[fn.Name.Name] {
		return
	}
	rd.visited[fn.Name.Name] = true
	where := func(n ast.Node) string { return fmt.Sprintf("%s:%d", src.rel, src.fset.Position(n.Pos()).Line) }
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					rd.change(sel.X, where(lhs))
				}
			}
		case *ast.CallExpr:
			rd.call(node, where(node))
		}
		return true
	})
}

// change records a piece whose face is now different from what the canvas
// last painted.
func (rd *rendererReading) change(expr ast.Expr, at string) {
	key, widget := rd.piece(expr)
	if key == "" || widget {
		return
	}
	if _, seen := rd.changed[key]; !seen {
		rd.changed[key] = at
	}
}

// call reads one call: redraw and what it was handed, a method of the renderer
// to follow, Show on a piece, or Refresh on a widget of this package.
func (rd *rendererReading) call(call *ast.CallExpr, at string) {
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "redraw" {
		for _, arg := range call.Args {
			key, widget := rd.piece(arg)
			if widget {
				rd.offences = append(rd.offences, at+" hands redraw a widget of this package, which asks the canvas about itself one step removed")
			}
			if key != "" {
				rd.named[key] = true
			}
		}
		return
	}
	on, method, _ := calledOn(call)
	if on == nil {
		return
	}
	if id, ok := on.(*ast.Ident); ok && id.Name == rd.recv {
		if helper, ok := rd.methods[rd.typeName][method]; ok {
			rd.walk(helper.src, helper.fn)
		}
		return
	}
	key, widget := rd.piece(on)
	switch {
	case method == "Refresh" && widget:
		rd.offences = append(rd.offences, at+" refreshes a widget of this package from inside a renderer, which is the wrong value under embedding")
	case method == "Show" && key != "":
		rd.change(on, at)
	}
}

// verdict is every offence read, plus one for each piece changed and never
// named, and one for a Refresh that names nothing at all.
func (rd *rendererReading) verdict(src canvasSource, fn *ast.FuncDecl) []string {
	out := rd.offences
	for key, at := range rd.changed {
		if !rd.named[key] {
			out = append(out, fmt.Sprintf("%s %s.Refresh changes %s.%s and never names it through redraw, so that piece keeps the face the canvas last painted",
				at, rd.typeName, rd.recv, key))
		}
	}
	if len(rd.named) == 0 {
		out = append(out, fmt.Sprintf("%s:%d %s.Refresh names no piece through redraw, so nothing tells the canvas the face changed",
			src.rel, src.fset.Position(fn.Pos()).Line, rd.typeName))
	}
	return out
}

// A box that gains or loses a piece says so, in the same function.
//
// Container.Add and Remove lay the box out and stop there. A piece just added
// is not known to the canvas until it has been painted once, so its own
// refresh reaches nothing, and a piece just removed is out of the tree - the
// box is the only thing that can ask for the repaint. Every box in
// internal/gui/window already did (five of six sites, measured 2026-09-16),
// and the sixth was the explanation sheet, which got away with it on the
// pointer's path because the button's face asked in the same tick and never
// on the keyboard's, where no face changes (O217).
func TestABoxThatGainsOrLosesAPieceSaysSo(t *testing.T) {
	sources := windowSources(t)
	fields := declaredFields(sources)

	var offences []string
	sites := 0
	for _, src := range sources {
		for _, decl := range src.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			recv, typeName := receiverOf(fn)
			if recv == "" {
				continue
			}
			changed, refreshed := boxesTouched(fn, recv, typeName, fields)
			sites += len(changed)
			for box, last := range changed {
				line := src.fset.Position(last).Line
				switch {
				case !refreshed[box].IsValid():
					offences = append(offences, fmt.Sprintf("%s:%d %s changes %s and never refreshes it - the piece it added waits for a repaint from anywhere, and the piece it removed stays drawn until one",
						src.rel, line, fn.Name.Name, box))
				case refreshed[box] < last:
					offences = append(offences, fmt.Sprintf("%s:%d %s changes %s after the last word to the canvas about it (line %d) - a refresh before the change repaints the old contents",
						src.rel, line, fn.Name.Name, box, src.fset.Position(refreshed[box]).Line))
				}
			}
		}
	}
	if sites == 0 {
		t.Fatal("no Add, Remove or RemoveAll on a container field was read, so this guard would pass against anything")
	}
	t.Logf("%d container changes read across parts and window", sites)
	sort.Strings(offences)
	for _, o := range offences {
		t.Error(o)
	}
}

// boxesTouched reads one method and returns the container fields it adds to
// or removes from, each with where the LAST change is, and the ones it
// refreshes, each with where the LAST refresh is.
//
// The last of each rather than any: a refresh standing before the change
// repaints what the box held before it, and the change after it is as silent
// as no refresh at all. Until 2026-09-16 any refresh anywhere in the method
// counted, which an outside review of the pull request pointed at. What this
// still reads as one method is the source order, so a change on a path that
// returns before the refresh at the foot of the method passes - two such
// paths exist, both on a value the registry cannot hand the menu.
func boxesTouched(fn *ast.FuncDecl, recv, typeName string, fields map[string]map[string]string) (changed, refreshed map[string]token.Pos) {
	changed = map[string]token.Pos{}
	refreshed = map[string]token.Pos{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		on, method, call := calledOn(n)
		if call == nil {
			return true
		}
		chain := fieldChain(on, recv)
		if chain == nil || resolveChain(fields, typeName, chain) != "*fyne.Container" {
			return true
		}
		box := recv + "." + strings.Join(chain, ".")
		switch method {
		case "Add", "Remove", "RemoveAll":
			changed[box] = max(changed[box], call.Pos())
		case "Refresh":
			refreshed[box] = max(refreshed[box], call.Pos())
		}
		return true
	})
	return changed, refreshed
}
