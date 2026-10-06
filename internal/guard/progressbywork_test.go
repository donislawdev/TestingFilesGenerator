package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// The bar's percentage and the time it says is left come from the work, on
// both surfaces (docs/WEBM-WYDAJNOSC-2026-10-06.md).
//
// Two halves, because the property lives in two places. engine.Progress
// works the two numbers out, and that is held here by what it answers for a
// report whose bytes and work disagree - five percent of the bytes, half of
// the work, the shape a film has while its pictures are coded. And the window
// and the command line must take both from there rather than work out their
// own: read from their source, every use they make of a report's bytes or work
// is an argument to HumanBytes, the size the line shows. A surface going back
// to a percentage of the bytes would use BytesDone somewhere else, and both of
// its own bars are out of a guard's reach - the window's moves on a goroutine
// the guards must not read beside, and the command line draws only on a
// terminal.
func TestTheBarAndTheTimeLeftComeFromTheWorkOnBothSurfaces(t *testing.T) {
	p := engine.Progress{FilesTotal: 1, BytesDone: 5, BytesTotal: 100, WorkDone: 50, WorkTotal: 100}
	if got := p.Percent(); got != 50 {
		t.Errorf("half the work and a twentieth of the bytes is %d%%, and the work says 50", got)
	}
	if left, ok := p.Left(10 * time.Second); !ok || left != 10*time.Second {
		t.Errorf("half the work in ten seconds leaves %v (%v), and the work says ten seconds", left, ok)
	}
	for name, quiet := range map[string]struct {
		p       engine.Progress
		elapsed time.Duration
	}{
		"the first second":  {p, 500 * time.Millisecond},
		"no work done yet":  {engine.Progress{BytesTotal: 100, WorkTotal: 100}, time.Minute},
		"all the work done": {engine.Progress{BytesDone: 100, BytesTotal: 100, WorkDone: 100, WorkTotal: 100}, time.Minute},
	} {
		if left, ok := quiet.p.Left(quiet.elapsed); ok {
			t.Errorf("%s: the estimate said %v where it has nothing to go on", name, left)
		}
	}

	shown, percents, lefts := 0, 0, 0
	for _, dir := range []string{"internal/cli", "internal/gui"} {
		walkGo(t, filepath.Join(repoRoot(t), dir), func(path string, file *ast.File) {
			counted := map[ast.Node]bool{}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch calledName(call) {
				case "HumanBytes":
					for _, arg := range call.Args {
						counted[arg] = true
					}
				case "Percent":
					if len(call.Args) == 0 {
						percents++
					}
				case "Left":
					lefts++
				}
				return true
			})
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "BytesDone", "BytesTotal", "WorkDone", "WorkTotal":
					if counted[sel] {
						shown++
						return true
					}
					rel, _ := filepath.Rel(repoRoot(t), path)
					t.Errorf("%s reads a report's %s other than to show it as a size - a surface working out its own percentage or estimate rather than asking engine.Progress", filepath.ToSlash(rel), sel.Sel.Name)
				}
				return true
			})
		})
	}
	// Asserted, not assumed: both lines show their bytes, all three places
	// that draw a percentage ask Progress for it, and both estimates come
	// from Left. A scan that found none of it would pass about nothing.
	if shown < 4 || percents < 3 || lefts < 2 {
		t.Fatalf("the surfaces show %d sizes from a report, ask %d times for a percentage and %d times for the time left - the scan did not reach the bars it is about", shown, percents, lefts)
	}
}

// walkGo parses every Go file under dir that is not a test.
func walkGo(t *testing.T, dir string, visit func(path string, file *ast.File)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		visit(path, file)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A panic while a helper codes a picture comes out on the goroutine that asked
// for the picture, and leaves no helper running.
//
// The engine turns a generator's panic into the run's error only on the
// goroutine that writes the file (writeWithoutCrashing), and a panic on any
// other goroutine ends the process - the window with it. So a helper keeps its
// panic and the writer raises it when it reaches that picture. gav1d cannot be
// made to panic from outside, so the pictures here are coded by a stand-in
// that panics the first time a helper calls it, through video.CodeBeside,
// which codes them exactly as a film's are. Under a helper that did not keep
// its panic this test does not fail - the test binary dies, which is red too.
func TestAPanicOnAHelperComesOutOnTheGoroutineThatAskedForIt(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))
	var fromHelper atomic.Bool
	helped := make(chan struct{})
	got := func() (v any) {
		defer func() { v = recover() }()
		_, _ = video.CodeBeside(40, func(change int64, beside bool) (int, error) {
			if beside && fromHelper.CompareAndSwap(false, true) {
				close(helped)
				panic("a helper's picture")
			}
			if !beside {
				// The writer's own picture waits for a helper to have
				// started, or the writer, with nothing to code, takes all
				// forty before any helper is scheduled.
				select {
				case <-helped:
				case <-time.After(5 * time.Second):
				}
			}
			return 1, nil
		})
		return nil
	}()
	if !fromHelper.Load() {
		t.Fatal("no helper coded a picture, so no helper could have panicked")
	}
	if got != "a helper's picture" {
		t.Errorf("the goroutine that asked for the pictures got %v, and a helper panicked with %q", got, "a helper's picture")
	}
	if running, _ := video.Helpers(); running != 0 {
		t.Errorf("%d helpers are still running after the panic came out", running)
	}
}
