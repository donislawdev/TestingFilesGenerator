package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
	// Hashing the files a manifest claims is the work verify and cleanup are
	// made of, and it is embarrassingly parallel. Added 2026-09-05 and the
	// owner decided it: O116 turned the same idea down on 2026-08-20 on a
	// measurement of 3000 files of 1 kB, where the whole of verify is about a
	// second. Measured again on the corpora this tool exists to produce -
	// 6.1 GB in files of 64 MB - tfg verify goes from 4.28-4.30 s to
	// 0.54-0.56 s, the hashing itself is 9.33x at sixteen workers, and 1.58x
	// even when the corpus is larger than memory and the disk is the limit.
	// Numbers and the two instrument mistakes made getting them:
	// docs/PERFORMANCE-REVIEW-2026-09-05.md.
	//
	// Kept to one file on purpose. Everything that could refuse a whole pass
	// is settled before the goroutines start, so a worker answers about one
	// file and cannot fail - which is what makes the order of the answers, and
	// the file a refusal names, the same on every run.
	"internal/audit/parallel.go": "hashing the claimed files runs beside itself, and nothing else in the package does",
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

// TestTheRaceDetectorIsRunForEveryFileThatDeclaresConcurrency ties the map
// above to the list in .github/workflows/ci.yml that decides whether the race
// detector job runs at all.
//
// Why there are two lists. The detector does not run on every push - it was
// measured at 10m31s on 2026-08-20 and given its own job with its own trigger.
// That trigger is a literal list of file names inside the workflow, and it is a
// second copy of the map above.
//
// The workflow used to claim the two could not drift, on the reasoning that a
// file growing a goroutine reddens the map guard before it gets that far. That
// is true only while the file is MISSING from the map. Adding it - which is
// exactly what the map guard's own message tells somebody to do - turns that
// guard green and leaves this question to nobody. So the list could fall behind
// precisely when it mattered: concurrency living in a file the detector is
// never run for, with the job reporting "skipped" and looking like a decision.
//
// Found on 2026-09-05 while adding internal/audit/parallel.go, by walking into
// it. This is the mechanism rather than the warning.
//
// A file watched but not declared is fine and is not reported. go.mod is
// exactly that: a toolchain or dependency change can alter what the detector
// sees without one of our own lines moving.
func TestTheRaceDetectorIsRunForEveryFileThatDeclaresConcurrency(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	// Asserted rather than assumed. A guard that quietly finds nothing to
	// compare is green about a question it never asked - and this whole test
	// exists because a claim about drift went unchecked.
	const marker = "watched='"
	start := strings.Index(string(body), marker)
	if start < 0 {
		t.Fatalf("%s no longer sets %s, so nothing states which files the race detector runs for",
			path, strings.TrimSuffix(marker, "='"))
	}
	rest := string(body)[start+len(marker):]
	end := strings.Index(rest, "'")
	if end < 0 {
		t.Fatalf("the %s list in %s is never closed", strings.TrimSuffix(marker, "='"), path)
	}
	watched := strings.Fields(rest[:end])
	if len(watched) == 0 {
		t.Fatalf("the race detector trigger in %s watches nothing at all", path)
	}

	listed := make(map[string]bool, len(watched))
	for _, f := range watched {
		listed[f] = true
	}

	var missing []string
	for file := range mayBeConcurrent {
		if !listed[file] {
			missing = append(missing, file)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("%d file(s) declare concurrency and the race detector is not run for them:\n  %s\n\n"+
			"The detector is the only thing in this project that sees a data race. A file that may run\n"+
			"beside itself and is not on the trigger list gets a job that reports \"skipped\", which reads\n"+
			"like a decision rather than a gap. Add it to the watched list in %s.",
			len(missing), strings.Join(missing, "\n  "), path)
	}
}
