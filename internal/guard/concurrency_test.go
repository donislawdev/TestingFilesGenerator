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
	// The same shape one axis over, and the same reason: written once when the
	// package starts and read by planning, by the recipe reader, by both
	// surfaces and by the guards. It holds the second lock in the tree because
	// it is the second registry, not because anything here runs beside
	// anything else.
	"internal/damage/damage.go": "the damage registry is written at init and read by everything after",
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
	// The window's clock. time.AfterFunc calls back on a goroutine of its
	// own, which is how a delayed piece of work - the busy face, the wait for
	// quiet - waits without holding the window, and the callback hands its
	// work straight to the toolkit's thread. Declared 2026-09-24, when the scan
	// learned to see a timer's callback: it had been here since the busy face
	// got its delay, unlisted.
	"internal/gui/run_cgo.go": "the window's clock calls back on a timer's goroutine and hands the work to the toolkit's thread",
	// Giving memory back beside the window rather than on its thread: one of
	// ten FreeOSMemory calls measured on the window's thread took 2491 ms.
	// The goroutine touches nothing of ours. Added 2026-09-24, and THE OWNER
	// DECIDED IT - docs/GUI-MEMORY-2026-09-23.md section 4j.
	"internal/gui/window/tidy.go": "memory is given back beside the window, so the window never waits on it",
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
	// Writing the files IS the run. Measured 2026-09-06, after P7 stopped
	// planning from encoding the picture twice: planning 300 PNGs is 51 ms and
	// writing them is 2741 ms, so the write loop is 98% of it and the plan is
	// 2%. Over goroutines in one process, png 200 kB x240 goes 2.05x at two,
	// 3.03x at four, 4.19x at eight and 5.51x at sixteen, and zip 2 MB x80
	// reaches 3.87x at eight - measured with tools/probes/writeparallel, which
	// also answered the question that had to come first: a shared heap costs
	// nothing, 563.0 MB at one goroutine against 562.8 MB at sixteen.
	//
	// Added 2026-09-06 and THE OWNER DECIDED IT, with two things put to them
	// rather than assumed. Where the pool lives: here rather than shared with
	// internal/audit, because the two need opposite behaviour when a run is
	// stopped - audit a contiguous prefix, this every file that finished - and
	// a shared helper would carry a flag switching the one property each of
	// them rests on. And what a stopped run records: every finished file, hole
	// or no hole, because a finished file with no manifest entry is a file
	// untouchable rule 7 leaves nothing able to remove.
	//
	// Numbers, the instrument, and the two mistakes made getting them:
	// docs/PERFORMANCE-REVIEW-2026-09-05.md section 14.
	"internal/engine/parallel.go": "the planned files are written beside each other, and nothing else in the package does",
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
	var found, idle []string

	for _, p := range packages(t) {
		for _, path := range p.files {
			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			here := concurrencyIn(fset, file)
			if _, allowed := mayBeConcurrent[rel]; allowed {
				if len(here) == 0 {
					idle = append(idle, rel)
				}
				continue
			}
			for _, one := range here {
				found = append(found, rel+":"+one)
			}
		}
	}

	// The other direction, since 2026-09-24. A file declared here that runs
	// nothing beside anything is a declaration nobody can check - and it is
	// how a decision gets undone quietly: the window gives memory back on a
	// goroutine BECAUSE a call on its own thread took 2491 ms once, and taking
	// the go statement away would leave the file declared and every guard
	// green. A file the build leaves out on this machine is not asked. A file
	// that is gone is, because the walk above never reaches it and a deleted
	// file would otherwise keep its declaration and its place on the race
	// detector's list - an outside review of the pull request named it.
	idle = append(idle, declaredWithoutAFile(repoRoot(t), mayBeConcurrent)...)
	if len(idle) > 0 {
		sort.Strings(idle)
		t.Errorf("declared as concurrent and running nothing beside anything:\n  %s\n\n"+
			"Either the concurrency moved and the declaration should go with it, or it was taken out and\n"+
			"the reason it was put there - written beside the declaration - no longer holds.",
			strings.Join(idle, "\n  "))
	}

	if len(found) > 0 {
		sort.Strings(found)
		t.Errorf("concurrency turned up in %d place(s) outside the files that declare it:\n  %s\n\n"+
			"Adding it somewhere new is a decision, not a detail - a race changes nothing this suite can\n"+
			"otherwise see. Put the file in mayBeConcurrent with the reason, and say so to the owner.",
			len(found), strings.Join(found, "\n  "))
	}
}

// declaredWithoutAFile is every declared path with no file under root, each
// with what the system said about it.
func declaredWithoutAFile(root string, declared map[string]string) []string {
	var gone []string
	for rel := range declared {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			gone = append(gone, rel+" ("+err.Error()+")")
		}
	}
	return gone
}

// A declaration whose file is gone is reported, and one whose file is there
// is not - asked of a folder made here, because every file the tree declares
// exists and a check that stopped looking would stay green on it.
func TestADeclarationWhoseFileIsGoneIsReported(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "here.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := declaredWithoutAFile(root, map[string]string{"a/here.go": "", "a/gone.go": ""})
	if len(got) != 1 || !strings.HasPrefix(got[0], "a/gone.go ") {
		t.Errorf("declared a/here.go, which exists, and a/gone.go, which does not - reported %q, expected a/gone.go alone", got)
	}
}

// concurrencyIn is every place in one file that runs something beside the
// code around it, as "line what".
//
// A timer's callback since 2026-09-24. time.AfterFunc runs the function it is
// handed on a goroutine of its own, and this scan saw only the go statement -
// so the window's clock, which has handed every delayed piece of work to such
// a goroutine since the busy face got a delay, stood outside the list without
// a word. Found while the window was taught to give memory back
// (docs/GUI-MEMORY-2026-09-23.md section 4j), decided by the owner that day.
func concurrencyIn(fset *token.FileSet, file *ast.File) []string {
	var found []string
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
			what = concurrentName(node)
		}
		if what == "" {
			return true
		}
		found = append(found, fmt.Sprintf("%d %s", fset.Position(n.Pos()).Line, what))
		return true
	})
	return found
}

// concurrentName is what a package-qualified name says about running beside
// something, or nothing: a lock or an atomic, or a timer that calls back on
// a goroutine of its own.
func concurrentName(node *ast.SelectorExpr) string {
	id, ok := node.X.(*ast.Ident)
	if !ok {
		return ""
	}
	switch {
	case id.Name == "sync" || id.Name == "atomic":
		return "uses " + id.Name + "." + node.Sel.Name
	case id.Name == "time" && node.Sel.Name == "AfterFunc":
		return "hands a callback to a timer's goroutine (time.AfterFunc)"
	}
	return ""
}

// The scan sees each kind of concurrency it names, asked of source written
// here - because the tree may hold none of a kind outside the declared files,
// and a scan that stopped seeing it would then stay green.
func TestTheConcurrencyScanSeesEachKindItLooksFor(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"a goroutine", `go f()`},
		{"a channel", `var c chan int; _ = c`},
		{"a lock", `var m sync.Mutex; m.Lock()`},
		{"a timer's callback", `time.AfterFunc(0, f)`},
	} {
		src := "package x\nfunc f() {}\nfunc g() {\n" + c.body + "\n}\n"
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(concurrencyIn(fset, file)) == 0 {
			t.Errorf("the scan does not see %s: %s", c.name, c.body)
		}
	}
	// And a name from package time that runs nothing beside anything is not
	// reported, so the timer rule is about the callback and not the package.
	src := "package x\nfunc g() {\n_ = time.Now()\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := concurrencyIn(fset, file); len(got) != 0 {
		t.Errorf("the scan calls a plain duration concurrency: %v", got)
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
