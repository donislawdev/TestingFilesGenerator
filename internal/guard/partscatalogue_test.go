package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/catalogue"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The catalogue of the window's parts, held to the package it draws.
//
// GUI rule 4 asks for a screen closed in both directions: nothing on it that
// the application does not have, nothing of the application missing from it.
// The first catalogue was a probe outside the repository and stopped
// compiling at step 5 of the rework, and nothing said so for two steps. These
// guards are why the second one lives in the tree: the surface of parts is
// read with go/ast, so a constructor or a type added tomorrow is on the list
// the day it lands, and the registry has to say where every one of them is
// drawn - or why it is not.

// What the registry has to account for: every exported type, every
// constructor (its name without New), and every exported function that
// returns something drawable. A function returning a number or a yes is not
// a component. Every name the registry uses has to exist - a catalogue that
// covers a part nobody has is the other way of being wrong.
func TestTheCatalogueCoversEveryPartAndNothingElse(t *testing.T) {
	surface, exported := partsSurface(t)
	if len(surface) < 30 {
		t.Fatalf("only %d names were found on the surface of parts, too few to be the real package", len(surface))
	}

	placed := map[string][]string{}
	place := func(name, where string) { placed[name] = append(placed[name], where) }
	for _, e := range catalogue.Entries() {
		place(e.Name, "the entry "+e.Name)
		for _, c := range e.Covers {
			place(c, "covered by "+e.Name)
		}
		if len(e.States) == 0 {
			t.Errorf("the entry %s has no states, so it draws nothing and is a name on a list", e.Name)
		}
	}
	for _, r := range catalogue.NotDrawn() {
		place(r.Name, "not drawn")
		if r.Why == "" {
			t.Errorf("%s is not drawn and no reason is written", r.Name)
		}
	}
	for _, r := range catalogue.LayoutOnly() {
		place(r.Name, "layout only")
		if r.Why == "" {
			t.Errorf("%s is layout only and no reason is written", r.Name)
		}
	}

	names := make([]string, 0, len(surface))
	for name := range surface {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		switch len(placed[name]) {
		case 1:
		case 0:
			t.Errorf("parts exports %s (%s) and the catalogue neither draws it nor says why not.\n"+
				"Add it as an entry, name it under the entry that draws it as a state, or put it in "+
				"NotDrawn or LayoutOnly with the reason.", name, surface[name])
		default:
			t.Errorf("%s is in the catalogue %d times: %s. One part, one place to look at it.",
				name, len(placed[name]), strings.Join(placed[name], ", "))
		}
	}
	for name, where := range placed {
		if !exported[name] {
			t.Errorf("the catalogue names %s (%s) and parts exports no such thing - a stale name is a "+
				"part that was renamed or removed and is still being vouched for", name, strings.Join(where, ", "))
		}
	}
	t.Logf("%d names on the surface of parts, every one placed once", len(surface))
}

// Every state of an entry draws differently from every other state of the
// same entry. A state that draws the same as another is not a state - it is
// a caption over the same picture, and a catalogue of those looks complete
// while showing nothing. Measured on the rendered tree rather than asserted
// from the flag the builder set, which is GUI rule 10.
func TestEveryCatalogueStateDrawsDifferently(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	defer test.NewApp()

	drawn := 0
	for _, e := range catalogue.Entries() {
		seen := map[string]string{}
		for _, s := range e.States {
			markup := renderedMarkup(t, s.Build())
			drawn++
			if other, dup := seen[markup]; dup {
				t.Errorf("%s: the state %q draws exactly as %q does, so one of them is a caption over "+
					"the other's picture", e.Name, s.Caption, other)
				continue
			}
			seen[markup] = s.Caption
		}
	}
	if drawn < 40 {
		t.Fatalf("only %d states were drawn, too few to be the real catalogue", drawn)
	}
	t.Logf("%d states drawn, every one different from its neighbours", drawn)
}

// renderedMarkup is one object laid out in a window of the reference width
// and drawn to words, so two states can be compared without a picture.
func renderedMarkup(t *testing.T, o fyne.CanvasObject) string {
	t.Helper()
	w := test.NewWindow(o)
	defer w.Close()
	w.Resize(fyne.NewSize(referenceWidth, o.MinSize().Height+referenceHeight/10))
	return test.RenderToMarkup(w.Canvas())
}

// The screen shows the whole registry: every entry by name, every state by
// its caption, and every name the catalogue declines to draw, with its reason
// - so that a person reading the screen and a guard reading the registry are
// reading the same thing.
func TestTheCatalogueScreenShowsTheWholeRegistry(t *testing.T) {
	shown := textIn(catalogue.Page())
	for _, e := range catalogue.Entries() {
		if !strings.Contains(shown, e.Name) {
			t.Errorf("the catalogue screen never names the entry %s", e.Name)
		}
		for _, s := range e.States {
			if !strings.Contains(shown, s.Caption) {
				t.Errorf("the catalogue screen never shows the state %q of %s", s.Caption, e.Name)
			}
		}
	}
	for _, r := range append(catalogue.NotDrawn(), catalogue.LayoutOnly()...) {
		if !strings.Contains(shown, r.Name) || !strings.Contains(shown, r.Why) {
			t.Errorf("the catalogue screen does not say that %s is left out, or not why", r.Name)
		}
	}
}

// partsSurface reads internal/gui/parts and returns the names the catalogue
// has to account for, each with what it is, and every exported name at all,
// which is what a registry name has to be one of.
func partsSurface(t *testing.T) (surface map[string]string, exported map[string]bool) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "internal", "gui", "parts")
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("listing %s: %v", dir, err)
	}
	surface = map[string]string{}
	exported = map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			collectSurface(decl, surface, exported)
		}
	}
	return surface, exported
}

// collectSurface files one declaration. A method is not a component and is
// left alone, which is also what keeps the receiver types out of "exported":
// they arrive through their own type declarations.
func collectSurface(decl ast.Decl, surface map[string]string, exported map[string]bool) {
	switch d := decl.(type) {
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
				surface[ts.Name.Name] = "an exported type"
				exported[ts.Name.Name] = true
			}
		}
	case *ast.FuncDecl:
		if d.Recv != nil || !d.Name.IsExported() {
			return
		}
		name := d.Name.Name
		if strings.HasPrefix(name, "New") && len(name) > 3 {
			exported[name[3:]] = true
			surface[name[3:]] = "the constructor " + name
			return
		}
		exported[name] = true
		if returnsDrawable(d.Type.Results) {
			surface[name] = "a function returning something drawable"
		}
	}
}

// returnsDrawable says whether a result list carries a thing a screen can
// draw: a canvas object, a container, or a resource - alone, in a slice, or
// beside other results.
func returnsDrawable(results *ast.FieldList) bool {
	if results == nil {
		return false
	}
	for _, field := range results.List {
		var b strings.Builder
		ast.Fprint(&b, nil, field.Type, nil)
		text := b.String()
		if strings.Contains(text, "CanvasObject") || strings.Contains(text, "Container") ||
			strings.Contains(text, "Resource") {
			return true
		}
	}
	return false
}

// The catalogue is one of the stored scenes, drawn whole. It is the scene
// where a change of a token shows on every control at once, so a catalogue
// that dropped off that list would take the before-and-after of every later
// step with it - quietly, because the other scenes would still pass.
func TestTheCatalogueIsAStoredScene(t *testing.T) {
	var found *screenScene
	for i := range screenScenes() {
		if screenScenes()[i].name == "catalogue" {
			found = &screenScenes()[i]
		}
	}
	if found == nil {
		t.Fatal("no scene called catalogue is on the list the pixel guard draws")
	}
	if found.page == nil {
		t.Fatal("the catalogue scene is not built as a page, so it would be drawn one screenful tall")
	}
	for _, ext := range []string{".png", ".xml"} {
		if _, err := os.Stat(filepath.Join("testdata", "screens", "catalogue"+ext)); err != nil {
			t.Errorf("no stored %s of the catalogue: %v - regenerate with TFG_WRITE_SCREEN_REFERENCE=1", ext, err)
		}
	}
}
