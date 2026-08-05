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

// Concurrency is the one defect class in this tree that no other guard here can
// see. A data race does not change the size of a file, does not change its
// bytes on a run that happens to interleave the safe way, and does not fail a
// determinism check that got lucky twice. It shows up as a file that is wrong
// once a month on somebody else's machine.
//
// Measured on 2026-08-02: the whole tree starts exactly one goroutine and holds
// exactly one lock. That is a surface small enough to name, so it is named -
// and anything that widens it has to be a decision rather than a habit.
//
// This is not a ban. It is a gate: adding concurrency somewhere new means
// adding the file here, which means somebody looked at it.
var mayBeConcurrent = map[string]string{
	// The registry is read by every generator and written once at startup, so
	// it carries the one lock in the tree.
	"internal/format/registry.go": "the format registry is written at init and read by everything after",
	// Signals arrive on a channel by definition, and the handler has to run
	// beside the work it interrupts.
	"cmd/tfg/main.go": "the interrupt handler has to run beside the work it stops",
	// A window that waits for engine.Run is a window the desktop reports as not
	// responding, so the run happens beside it. The channel is the other half:
	// closing the window has to wait for that goroutine to wind down, because
	// cancelling and exiting at once would end the process inside a file - the
	// invariant G7 exists to hold. Added 2026-08-05 with the first generate
	// window, and the owner was told.
	"internal/gui/window/run.go": "the run happens beside the window, and closing the window waits for it",
}

// Waiting on cancellation is not the same thing as running in parallel. Every
// long loop in this tree checks ctx.Done(), and calling that concurrency would
// make the rule meaningless on the day it was written.
func isCancellation(n ast.Node) bool {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Done"
}

func TestConcurrencyStaysWhereItWasPutOnPurpose(t *testing.T) {
	var found []string

	for _, p := range packages(t) {
		for _, path := range p.files {
			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)
			if _, allowed := mayBeConcurrent[rel]; allowed {
				continue
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}

			ast.Inspect(file, func(n ast.Node) bool {
				if n == nil {
					return false
				}
				what := ""
				switch node := n.(type) {
				case *ast.GoStmt:
					what = "starts a goroutine"
				case *ast.ChanType:
					what = "declares a channel"
				case *ast.SendStmt:
					what = "sends on a channel"
				case *ast.SelectStmt:
					// A select waiting only on cancellation is how every long
					// loop here notices Ctrl+C.
					if onlyCancellation(node) {
						return true
					}
					what = "selects over channels"
				case *ast.SelectorExpr:
					if id, ok := node.X.(*ast.Ident); ok && (id.Name == "sync" || id.Name == "atomic") {
						what = "uses " + id.Name + "." + node.Sel.Name
					}
				}
				if what == "" {
					return true
				}
				found = append(found, fmt.Sprintf("%s:%d %s",
					rel, fset.Position(n.Pos()).Line, what))
				return true
			})
		}
	}

	if len(found) > 0 {
		sort.Strings(found)
		t.Errorf("concurrency turned up in %d place(s) outside the files that declare it:\n  %s\n\n"+
			"Adding it somewhere new is a decision, not a detail - a race changes nothing this suite can\n"+
			"otherwise see. Put the file in mayBeConcurrent with the reason, and say so to the owner.",
			len(found), strings.Join(found, "\n  "))
	}
}

// onlyCancellation reports whether every case of a select is either a receive
// from something named Done or the default branch.
func onlyCancellation(s *ast.SelectStmt) bool {
	for _, stmt := range s.Body.List {
		clause, ok := stmt.(*ast.CommClause)
		if !ok {
			return false
		}
		if clause.Comm == nil {
			continue // default:
		}
		expr, ok := clause.Comm.(*ast.ExprStmt)
		if !ok {
			return false
		}
		unary, ok := expr.X.(*ast.UnaryExpr)
		if !ok || unary.Op != token.ARROW {
			return false
		}
		call, ok := unary.X.(*ast.CallExpr)
		if !ok || !isCancellation(call.Fun) {
			return false
		}
	}
	return true
}
