package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// Exactly one preset declares itself the one a surface opens on.
//
// Before 2026-09-22 both screens opened on the first id in order, which is
// alphabetical - so what somebody saw when they opened the Presets tab was
// decided by whoever wrote a preset next. It changed on the day
// empty-and-minimal arrived, and nine window guards went red at once, none of
// them reporting anything more useful than a field being nil. The catalogue in
// docs/PRESETS.md has fourteen entries, so that was going to keep happening.
//
// Two presets claiming it is a panic at registration rather than a failure
// here, the same way two presets of one id are. What this catches is the other
// end: nobody claiming it, which is silent - preset.Landing falls back to the
// first id, and the fallback exists so that a window still opens rather than so
// that anybody relies on it.
func TestExactlyOnePresetOpensTheWindow(t *testing.T) {
	all := preset.All()
	if len(all) == 0 {
		t.Fatal("no preset is registered, so this guard checked nothing")
	}

	claimed := make([]string, 0, 1)
	for _, p := range all {
		if p.Landing {
			claimed = append(claimed, p.ID)
		}
	}
	switch len(claimed) {
	case 1:
	case 0:
		t.Errorf("no preset says it is the one to open on, so the window falls back to %q - "+
			"the first id in alphabetical order, which is whichever preset gets written next",
			preset.Landing())
	default:
		t.Errorf("%v all say they are the one to open on, and only one can", claimed)
	}

	if got := preset.Landing(); len(claimed) == 1 && got != claimed[0] {
		t.Errorf("%s declares itself the one to open on and the window is told to open on %q",
			claimed[0], got)
	}
}

// Both screens that offer presets open on the declared one.
//
// Asked of the SCREENS rather than of preset.Landing, which is the whole value
// of it: the declaration being right says nothing about whether a screen went
// through it, and the two screens reached for the first id separately. A guard
// calling Landing directly would agree with Landing and prove nothing.
//
// What this one CANNOT tell apart today, and it was named by CodeRabbit on
// 2026-09-22 rather than noticed here: the declared preset is also the first id
// in alphabetical order, so a screen going back to picking by position would
// satisfy every assertion below. The mutation that "proved" this guard picked
// the LAST id, which is not a regression anybody would write.
//
// The pair to it is TestNoScreenChoosesAPresetByItsPlaceInTheList, which reads
// the source instead, because the two cannot be told apart by behaviour while
// one preset is both. The day a preset sorting before empty-and-minimal is
// written, this guard starts telling them apart on its own and the other one
// becomes the belt rather than the braces.
func TestBothScreensOpenOnTheDeclaredPreset(t *testing.T) {
	want := preset.Landing()
	if want == "" {
		t.Fatal("this build registers no preset, so neither screen has one to open on")
	}

	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	t.Cleanup(func() { join(host) })

	presets := selectTab(t, host.content, text.TabPresets())
	if got := chooserUnder(t, presets, text.FieldPreset()).Selected; got != want {
		t.Errorf("the presets screen opens on %q and %q is the declared one", got, want)
	}

	// The batch screen draws its preset menu only once the switch is on, so it
	// is turned on the way a press turns it on.
	batches := selectTab(t, host.content, text.TabRecipe())
	switchOn := checkNamed(batches, text.FieldBuildOnPreset())
	if switchOn == nil {
		t.Fatal("there is no switch to build on a preset on the batch screen")
	}
	switchOn.SetChecked(true)
	if got := basePresetOn(t, batches); got != want {
		t.Errorf("the batch screen opens on %q and %q is the declared one", got, want)
	}
}

// No screen picks a preset by where it sits in the list.
//
// The source rather than the behaviour, and that is a confession rather than a
// preference. TestBothScreensOpenOnTheDeclaredPreset above cannot separate "it
// asked preset.Landing" from "it took the first id", because empty-and-minimal
// is both - so a screen reverting to the line it had until 2026-09-22 would
// keep that guard green. Named by CodeRabbit, and it was right.
//
// The list a preset chooser is built from is preset.IDs(). So the rule is about
// THAT list: whatever a file binds it to may be offered whole and may not be
// indexed. The format menu beside it indexes its own list on purpose
// (generate.go picks the first format), which is why this asks where the list
// came from rather than looking for a shape.
func TestNoScreenChoosesAPresetByItsPlaceInTheList(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "gui", "window")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the window package: %v", err)
	}

	looked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		looked++
		checkNoPresetListIndexing(t, filepath.Join(dir, name))
	}
	if looked == 0 {
		t.Fatal("no source file was read, so this guard would pass against anything")
	}
}

// checkNoPresetListIndexing reports every place one file indexes the list of
// preset ids.
func checkNoPresetListIndexing(t *testing.T, path string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	lists := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		if !callsPresetIDs(assign.Rhs[0]) {
			return true
		}
		if name, ok := assign.Lhs[0].(*ast.Ident); ok {
			lists[name.Name] = true
		}
		return true
	})
	if len(lists) == 0 {
		return
	}

	ast.Inspect(file, func(n ast.Node) bool {
		index, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		name, ok := index.X.(*ast.Ident)
		if !ok || !lists[name.Name] {
			return true
		}
		t.Errorf("%s:%d takes a preset out of the list by its place. A preset says which one "+
			"a surface opens on - see preset.Landing - and the list is in alphabetical order, "+
			"so a position is whichever preset gets written next",
			filepath.Base(path), fset.Position(index.Pos()).Line)
		return true
	})
}

// callsPresetIDs reports whether an expression is the call preset.IDs().
func callsPresetIDs(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "IDs" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "preset"
}

// basePresetOn is the preset chosen in the batch screen's base section.
//
// Read through the tree, because the control registered under the recipe key is
// the menu inside its width wrapper.
func basePresetOn(t *testing.T, screen fyne.CanvasObject) string {
	t.Helper()
	control := controlUnder(screen, text.FieldBasePreset())
	menu, ok := control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the base preset field is %T rather than a list to choose from", control)
	}
	return menu.Selected
}
