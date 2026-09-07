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

// The fourth axis: the TYPE.
//
// The three ceilings already here measure files, functions and branching, and a
// type is none of them. That is not a gap in principle - it is a hole this tree
// can be shown to have. codeshape_test.go records three separate lowerings of
// longestFile, 503 to 457 to 433, and every one of them was earned the same way:
// code moved out of engine.go into a new file beside it. The number that is
// watched went down each time. Nothing asked whether the type a reader has to
// hold in their head went down with it, because nothing here can ask that.
//
// What this buys that a file ceiling cannot: a type is the unit of state.
// Twenty two fields is twenty two things any of its twenty eight methods may
// have changed, and splitting the FILE those methods live in does not divide
// that by anything.
//
// Today's numbers are healthy - the worst type here has 28 methods, not the
// ninety a long lived window class grows - so this ceiling buys nothing today
// and is bought for the reason deepestNesting was bought: it costs nothing
// while nothing grows, and by the time it would have been worth adding, the
// number it would have to be set to is already the problem.
//
// Down is routine. Up is the owner's decision, the same as every other ceiling
// in this package.
const (
	// Measured 2026-09-07. Both are internal/gui/window.runner, which is the
	// screen that drives a run - it holds the widgets, the progress state and
	// the cancel plumbing at once.
	mostMethods = 28
	mostFields  = 22

	// What counts as crowding, in the shape this package already uses
	// everywhere else: an ABSOLUTE number rather than a percentage of the
	// ceiling. That is a local convention and it is deliberate here, because a
	// band written as a fraction reshapes itself every time the ceiling moves,
	// and a band that moves under the thing it is watching says nothing.
	//
	// Set at roughly three quarters of the ceiling and then measured, which is
	// the order that matters - a threshold picked first and measured second is
	// a guess with a gate around it.
	crowdingMethods = 21
	crowdingFields  = 17

	// Caps measured on the tree of 2026-09-07 and then frozen. Like every count
	// in this package these only go down, and raising one to turn a run green is
	// the same act as raising a ceiling.
	crowdedMethodTypes = 3
	crowdedFieldTypes  = 4
)

// typeSize is one named type with the two counts this axis watches.
//
// METHODS are the declarations whose receiver is this type. A function inside
// one of them is that method's business and not another entry in this type's
// surface, which is why only top level declarations are read.
//
// FIELDS counts what a struct declares, embedded ones included, because an
// embedded type is state the reader still has to know about. A type that is not
// a struct has no fields and simply does not appear in that half.
type typeSize struct {
	name    string // internal/gui/window.runner
	where   string // internal/gui/window/run.go:112
	methods int
	fields  int
}

// measureTypes reads every named type of the shipped tree.
//
// Scope is packages(t) and its files field, which is production code only and
// excludes _test.go - the same tree longestFile and longestFunction measure. A
// guard measuring a different tree from its neighbours would be a fourth axis
// answering about a fourth codebase.
//
// 🔴 What build constraints do to this number, measured 2026-09-07 rather than
// assumed, because a pinned count that moves with the environment is the shape
// this project has in its own table of environmental noise. build.ImportDir
// applies the constraints of the machine it runs on, so a type behind a tag is
// invisible on a build that does not carry the tag. Two files declare types
// that way: internal/gui/run_cgo.go behind cgo, holding desktop with 5 methods
// and stored with 4, and internal/format/avif/requiretag.go behind !noasm,
// holding a struct with no fields at all. Every one of them is far below both
// ceilings and below both bands, so CGO_ENABLED and the build tags cannot move
// any number in this file. A large type added behind a tag would break that,
// and it would show up as a pinning failure on one platform only.
func measureTypes(t *testing.T) []typeSize {
	t.Helper()
	root := repoRoot(t)

	methods := map[string]int{}
	fields := map[string]int{}
	where := map[string]string{}

	for _, p := range packages(t) {
		for _, path := range p.files {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			for _, decl := range file.Decls {
				switch node := decl.(type) {
				case *ast.FuncDecl:
					if node.Recv == nil || len(node.Recv.List) == 0 {
						continue
					}
					owner := receiver(node.Recv.List[0].Type)
					if owner == "" {
						continue
					}
					methods[qualify(p.rel, owner)]++
				case *ast.GenDecl:
					for _, spec := range node.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						name := qualify(p.rel, ts.Name.Name)
						where[name] = fmt.Sprintf("%s:%d", rel, fset.Position(ts.Pos()).Line)
						st, ok := ts.Type.(*ast.StructType)
						if !ok || st.Fields == nil {
							continue
						}
						fields[name] = structFields(st)
					}
				}
			}
		}
	}

	names := map[string]bool{}
	for name := range methods {
		names[name] = true
	}
	for name := range fields {
		names[name] = true
	}

	out := make([]typeSize, 0, len(names))
	for name := range names {
		out = append(out, typeSize{
			name:    name,
			where:   where[name],
			methods: methods[name],
			fields:  fields[name],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// qualify names a type the way a person would have to say it out loud.
//
// Keyed by package AND name rather than by name alone, which is not tidiness:
// this tree has a type called generator in every format package, and a map
// keyed by the bare name adds twenty of them together. The first measurement
// taken for this file did exactly that and reported a type with 48 methods that
// does not exist.
func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// receiver reports the type a method is declared on, through a pointer and
// through the type parameters of a generic type.
func receiver(e ast.Expr) string {
	switch node := e.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.StarExpr:
		return receiver(node.X)
	case *ast.IndexExpr:
		return receiver(node.X)
	case *ast.IndexListExpr:
		return receiver(node.X)
	}
	return ""
}

// structFields counts what a struct declares. An embedded type carries no name
// of its own and counts as one.
func structFields(st *ast.StructType) int {
	n := 0
	for _, f := range st.Fields.List {
		if len(f.Names) == 0 {
			n++
			continue
		}
		n += len(f.Names)
	}
	return n
}

func TestNoTypeHasGrownPastWhatAPersonCanHoldInTheirHead(t *testing.T) {
	sizes := measureTypes(t)

	// The canary every guard in this package carries. A walk that reads nothing
	// satisfies every ceiling ever set and looks exactly like a walk that works.
	if len(sizes) < 100 {
		t.Fatalf("the type scan found %d types, which is too few to be this tree - it read nothing", len(sizes))
	}

	var over []string
	for _, ts := range sizes {
		if ts.methods > mostMethods {
			over = append(over, fmt.Sprintf(
				"%s has %d methods and the ceiling is %d - move behaviour out, do not raise the number (%s)",
				ts.name, ts.methods, mostMethods, ts.where))
		}
		if ts.fields > mostFields {
			over = append(over, fmt.Sprintf(
				"%s holds %d fields and the ceiling is %d - move state out, do not raise the number (%s)",
				ts.name, ts.fields, mostFields, ts.where))
		}
	}

	// Every one at once rather than the first, for the reason RC7 gives about
	// recipes: being sent back four times for four answers is its own defect.
	if len(over) > 0 {
		sort.Strings(over)
		t.Errorf("%d type(s) have grown past the ceiling:\n  %s",
			len(over), strings.Join(over, "\n  "))
	}
}

// typeCrowd names the types already inside each band.
//
// One copy of the question, called by the count and by the pinning test below,
// because two copies of "what counts as crowded" is two places for it to drift.
// The predicate itself is crowding(), shared with the file and function bands -
// a band that stopped being a number of things and went back to a share of the
// ceiling would reshape itself under every axis at once, and one mutation
// already watches exactly that.
func typeCrowd(sizes []typeSize) (byMethods, byFields []string) {
	for _, ts := range sizes {
		if crowding(ts.methods, crowdingMethods) {
			byMethods = append(byMethods, fmt.Sprintf("%s (%d)", ts.name, ts.methods))
		}
		if crowding(ts.fields, crowdingFields) {
			byFields = append(byFields, fmt.Sprintf("%s (%d)", ts.name, ts.fields))
		}
	}
	sort.Strings(byMethods)
	sort.Strings(byFields)
	return byMethods, byFields
}

func TestNoSecondTypeIsCreepingUpOnTheTypeCeilings(t *testing.T) {
	// The second knob, for the reason crowding_test.go gives at length: a
	// ceiling on the worst single type sees one thing growing to a record and is
	// blind to two of them climbing together, neither a record.
	byMethods, byFields := typeCrowd(measureTypes(t))

	if len(byMethods) > crowdedMethodTypes {
		t.Errorf("%d type(s) reach %d methods and at most %d may - move behaviour out of one before adding another:\n  %s",
			len(byMethods), crowdingMethods, crowdedMethodTypes, strings.Join(byMethods, "\n  "))
	}
	if len(byFields) > crowdedFieldTypes {
		t.Errorf("%d type(s) reach %d fields and at most %d may - move state out of one before adding another:\n  %s",
			len(byFields), crowdingFields, crowdedFieldTypes, strings.Join(byFields, "\n  "))
	}
}

func TestTheTypeCeilingsAreTodaysMeasurementAndNotALooserNumber(t *testing.T) {
	// The other half of both ceilings, and the reason ratchet_test.go exists at
	// all: a number parked above the truth grants headroom nobody decided to
	// grant, and the next arrival slips in under it in silence. So shrinking a
	// type comes with a two character chore - bring its number down with it.
	sizes := measureTypes(t)

	worstMethods, worstFields := 0, 0
	methodHolder, fieldHolder := "", ""
	for _, ts := range sizes {
		if ts.methods > worstMethods {
			worstMethods, methodHolder = ts.methods, ts.name
		}
		if ts.fields > worstFields {
			worstFields, fieldHolder = ts.fields, ts.name
		}
	}
	inMethodBand, inFieldBand := typeCrowd(sizes)
	crowdedByMethods, crowdedByFields := len(inMethodBand), len(inFieldBand)

	if worstMethods != mostMethods {
		t.Errorf("mostMethods is %d and the widest type is %s at %d - move the ceiling to %d.",
			mostMethods, methodHolder, worstMethods, worstMethods)
	}
	if worstFields != mostFields {
		t.Errorf("mostFields is %d and the widest type is %s at %d - move the ceiling to %d.",
			mostFields, fieldHolder, worstFields, worstFields)
	}
	if crowdedByMethods != crowdedMethodTypes {
		t.Errorf("crowdedMethodTypes is %d and %d type(s) reach %d methods - lower it to %d.",
			crowdedMethodTypes, crowdedByMethods, crowdingMethods, crowdedByMethods)
	}
	if crowdedByFields != crowdedFieldTypes {
		t.Errorf("crowdedFieldTypes is %d and %d type(s) reach %d fields - lower it to %d.",
			crowdedFieldTypes, crowdedByFields, crowdingFields, crowdedByFields)
	}

	t.Logf("%d types, worst %d methods (%s) and %d fields (%s), crowding %d by methods and %d by fields",
		len(sizes), worstMethods, methodHolder, worstFields, fieldHolder,
		crowdedByMethods, crowdedByFields)
}
