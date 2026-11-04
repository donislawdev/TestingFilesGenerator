package guard

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"runtime"
	"testing"
	"weak"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// A field taken off the registry is let go, and so is a control without a
// setting that went with it.
//
// Cutting a slice keeps its array, and the array keeps every pointer past the
// cut: the boxes of the format chosen before, their closures and whatever those
// hold, alive for as long as the screen is (R19 of the stability review).
// Asked of the collector through weak pointers, because nothing outside the
// registry can see the array - a guard reading the length would pass on
// exactly this defect.
func TestAFieldTakenOffTheRegistryIsLetGo(t *testing.T) {
	fields := parts.NewFields()
	fields.Add("kept", "Kept", "", parts.Detail{}, parts.NewEntry())
	field, control := addedToBeCut(fields)
	fields.KeepFirst(1)
	runtime.GC()
	runtime.GC()

	if fields.Len() != 1 || fields.Lookup("kept") == nil {
		t.Fatalf("the cut left %d field(s) rather than the first one, so this proved nothing", fields.Len())
	}
	if field.Value() != nil {
		t.Error("a field cut off the registry is still held by it")
	}
	if control.Value() != nil {
		t.Error("a control without a setting cut off the registry is still held by it")
	}
	runtime.KeepAlive(fields)
}

// addedToBeCut puts a field and a control without a setting after the first
// field, and keeps nothing of either but a weak pointer, so the registry is the
// only thing left that can hold them. A function of its own so that no variable
// of the guard outlives the call.
func addedToBeCut(fields *parts.Fields) (weak.Pointer[parts.Field], weak.Pointer[parts.Chooser]) {
	fields.Add("gone", "Gone", "", parts.Detail{}, parts.NewEntry())
	menu := parts.NewChooser([]string{"one", "two"}, nil)
	fields.Named("Way", parts.Detail{}, menu)
	return weak.Make(fields.Lookup("gone")), weak.Make(menu)
}

// Nothing is deleted from a slice by gluing the halves on either side of it.
//
// append(x[:i], x[i+1:]...) moves the tail down one place and leaves the last
// place of the array holding what stood there, so a list of pointers keeps the
// last one alive until something is written over it - a batch taken off the
// recipe screen, boxes and all (R19 of the stability review). slices.Delete
// clears that place. Asked of the code rather than of a screen, because a
// widget that was ever drawn is held by the toolkit's own cache of renderers,
// and a guard asking whether the batch was let go would be red for a reason
// that has nothing to do with the list.
func TestNothingIsDeletedFromASliceByGluingItsHalves(t *testing.T) {
	root := repoRoot(t)
	read := 0
	var glued []string
	for _, dir := range []string{"internal", "cmd"} {
		walkGo(t, filepath.Join(root, dir), func(path string, file *ast.File) {
			read++
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && gluesItsHalves(call) {
					rel, _ := filepath.Rel(root, path)
					glued = append(glued, filepath.ToSlash(rel)+": "+types.ExprString(call))
				}
				return true
			})
		})
	}
	if read < 100 {
		t.Fatalf("read %d Go files, so the walk did not reach the code", read)
	}
	for _, g := range glued {
		t.Errorf("%s - use slices.Delete, which clears the place left at the end", g)
	}
}

// gluesItsHalves is append(x[:i], x[i+1:]...) with the same x on both sides,
// however x and i are spelled.
func gluesItsHalves(call *ast.CallExpr) bool {
	fun, ok := call.Fun.(*ast.Ident)
	if !ok || fun.Name != "append" || !call.Ellipsis.IsValid() || len(call.Args) != 2 {
		return false
	}
	head, ok1 := call.Args[0].(*ast.SliceExpr)
	tail, ok2 := call.Args[1].(*ast.SliceExpr)
	if !ok1 || !ok2 || head.Low != nil || head.High == nil || tail.Low == nil || tail.High != nil {
		return false
	}
	return types.ExprString(head.X) == types.ExprString(tail.X) &&
		types.ExprString(tail.Low) == types.ExprString(head.High)+" + 1"
}
