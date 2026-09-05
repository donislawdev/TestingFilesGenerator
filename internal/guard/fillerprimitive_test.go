package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Bulk random bytes come from core, and no format keeps its own copy of the
// loop.
//
// Until 2026-09-06 thirteen packages each carried their own version of the same
// filler, in four different shapes, and the drift between them was not
// cosmetic. Three of them - zip, targz and wav - drew one byte per call to the
// generator and ran at 182 MB/s, which is slower than the disk they were
// feeding. Six drew eight bytes but through a temporary array. Four wrote the
// same eight bytes with a hand rolled shift loop. Measured over 64 MiB: the
// slow shape 182 MB/s, the temporary array 1468 MB/s, the shift loop 845 MB/s,
// and one store straight into the buffer 2499 MB/s.
//
// The performance report that started this named three of the thirteen. The
// other ten were found by grep, and two of the four shapes were nobody's
// finding at all. That is the argument for this guard rather than for a note in
// a document: the copies were not written by one person on one day, they
// accumulated one format at a time, and each one looked reasonable beside the
// format next to it.
//
// Uint64 is the bulk draw. Every one of the thirteen reached for it, so asking
// that no format calls it directly is asking the question the copies actually
// answer. UintN is here for the one legitimate caller, which picks a word out
// of a list rather than filling a buffer.
//
// What this does NOT see, stated because a guard that hides its edges is worse
// than none: a filler written with IntN, Uint32 or Float64 would walk straight
// past it. Two such loops already exist and are deliberately left alone - the
// salt and header fills in internal/format/archive, twelve and sixteen bytes
// once per archive, where the shape costs nothing and changing it would move
// bytes for no gain.
func TestBulkRandomBytesComeFromOnePlace(t *testing.T) {
	// Every call to UintN allowed to live outside core, and why. An entry that
	// stops naming real code is a failure below, not a comment nobody reads.
	allowed := map[string]string{
		"internal/format/opc/opc.go": "picks a word from a list, it does not fill a buffer",
	}

	seen := map[string]bool{}

	root := filepath.Join("..", "format")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(filepath.Join("internal", "format",
			strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")))

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			where := fset.Position(call.Pos())
			switch sel.Sel.Name {
			case "Uint64":
				t.Errorf("%s:%d calls Uint64 directly.\n"+
					"Bulk random bytes come from core.FillRandomBE or core.FillRandomLE, so that "+
					"every format draws them the same way and at the same speed. Thirteen packages "+
					"each had their own copy of this loop until 2026-09-06, in four shapes, the "+
					"slowest of them thirteen times slower than the fastest. Call the core helper "+
					"whose byte order this format already writes - and if the order is genuinely "+
					"new, add it there rather than here.", rel, where.Line)
			case "UintN":
				if _, ok := allowed[rel]; !ok {
					t.Errorf("%s:%d calls UintN directly.\n"+
						"If this fills a buffer, use core.FillRandomBE or core.FillRandomLE. If it "+
						"draws a single value for something else, add it to the allowed map above "+
						"with the reason.", rel, where.Line)
					return true
				}
				seen[rel] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the format packages: %v", err)
	}

	// An exception that outlived its code is an exception nobody granted.
	for rel, why := range allowed {
		if !seen[rel] {
			t.Errorf("%s is listed as allowed to call UintN (%s), but it does not call it.\n"+
				"Delete the entry. A standing exception for code that has gone will quietly "+
				"cover the next thing that lands in that file.", rel, why)
		}
	}
}
