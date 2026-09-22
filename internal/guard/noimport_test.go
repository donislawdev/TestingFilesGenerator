package guard

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The window binary does not import opengl32.dll at load time.
//
// An import in the executable's table is mapped by the loader before a line
// of our code runs, and Windows hands a library already mapped under a name
// to every later request for that name. So with the import in place, a
// software renderer loaded from a path afterwards can never be the opengl32
// the toolkit finds when it creates its context - measured on 2026-09-17
// with the process's own module list, docs/GUI-NO-OPENGL section 7. The copy
// of the OpenGL binding under third_party exists to take that import out, and
// this is the guard that asks whether it is out.
//
// Asked of a built binary rather than of any build flag, and the reason was
// measured before this guard was written: the windowing library links
// -lopengl32 too, on the same line as -lgdi32, and the binary still carries no
// import from it, because an import library on the link line contributes
// nothing unless some code refers to one of its symbols. A guard reading
// linker flags would have refused a binary that was already right. What
// decides is whether anything refers to a symbol, and only the linker knows.
//
// The table is read through debug/pe's ImportedSymbols, because its
// ImportedLibraries answers with an empty list for every binary - measured
// on the same day. Each symbol names the library it comes from, so the set
// of libraries is derived from the symbols.
//
// A Windows binary with cgo needs a C compiler, and the build here says so
// when it has none rather than passing. It is a skip and not a failure on
// the runners that are not Windows, and on a Windows machine without gcc,
// because a guard that cannot be run is not a guard that passed - the one
// that can run is a Windows job of CI, which has the compiler.
//
// Which Windows job is the point of the variable below. Measured on
// 2026-09-17 (docs/GUI-SOFTWARE-RENDERER-2026-09-17.md section 6.1): the test
// matrix runs with CGO_ENABLED=0, so nothing else in that job compiles the
// OpenGL binding or GLFW, and the build here was the one cold cgo build of
// the run - 822 s for the Windows test step against 434 s with a warm cache,
// with four minutes left under the step's timeout. Every change to go.sum
// makes the cache cold again. So CI runs this guard in a job of its own, with
// a cache of its own, and asks the matrix to skip it. The skip is not a
// preference and is never taken by itself: it is taken only when that job
// exists to run the guard instead, and a guard in internal/guard holds the
// workflow to that - the job names this test, the matrix sets this variable,
// and the job does not.
const importTableJobVariable = "TFG_IMPORT_TABLE_JOB"

func TestTheWindowBinaryDoesNotImportOpenGLAtLoadTime(t *testing.T) {
	if os.Getenv(importTableJobVariable) != "" {
		t.Skipf("skipped here on purpose: %s is set, so the import table is read by the CI job that builds the window with cgo and a warm cache - see ci.yml", importTableJobVariable)
	}
	if runtime.GOOS != "windows" {
		t.Skipf("the import table being read is a Windows one, and a Windows binary with cgo cannot be built on %s", runtime.GOOS)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no Go toolchain here, so no binary can be built to read an import table from: %v", err)
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skipf("no C compiler here, so the window binary cannot be linked to read its import table: %v", err)
	}

	built := filepath.Join(t.TempDir(), "tfg-gui.exe")
	// The shipped build: the tags, the linker flags the repository declares,
	// and cgo on - because the import table belongs to the binary people
	// download, not to some other build of it.
	build := exec.Command("go", "build", "-tags", buildTags(), "-ldflags="+windowLinkerFlags(t), "-o", built, "./cmd/tfg-gui")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=1")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building ./cmd/tfg-gui with cgo: %v\n%s", err, out)
	}

	libraries := importedLibraries(t, built)
	// The canary. The toolkit's windowing library asks GDI for its pixel
	// format, so a table with no GDI32 in it is a table this guard did not
	// read, whatever it says about OpenGL.
	if !libraries["gdi32.dll"] {
		t.Fatalf("the window binary imports no gdi32.dll, so this guard read the wrong table or an empty one.\n"+
			"Imports found: %s", strings.Join(sortedKeys(libraries), ", "))
	}
	if libraries["opengl32.dll"] {
		t.Errorf("the window binary imports opengl32.dll at load time.\n"+
			"Reason: the loader maps the system's opengl32.dll before any code of ours runs, so a\n"+
			"software renderer loaded by path afterwards is never the one the toolkit finds - the\n"+
			"fallback for a machine with no OpenGL driver cannot work (O218).\n"+
			"What to do: the copy of the OpenGL binding under third_party/go-gl-gl takes that import\n"+
			"out - see its PATCH.md and go.mod's replace directive. Something has put it back:\n"+
			"a symbol of opengl32.dll is referred to directly by some linked C code.\n"+
			"Imports found: %s", strings.Join(sortedKeys(libraries), ", "))
	}
}

// importedLibraries is the set of libraries a Windows binary imports at load
// time, lower cased, derived from the symbols its import table names.
func importedLibraries(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := pe.Open(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	defer f.Close()
	symbols, err := f.ImportedSymbols()
	if err != nil {
		t.Fatalf("reading the import table of %s: %v", path, err)
	}
	libraries := map[string]bool{}
	for _, symbol := range symbols {
		// "ChoosePixelFormat:GDI32.dll" - the library after the last colon.
		if i := strings.LastIndex(symbol, ":"); i >= 0 {
			libraries[strings.ToLower(symbol[i+1:])] = true
		}
	}
	if len(libraries) == 0 {
		t.Fatalf("%s names no imported library at all, so nothing here was read", path)
	}
	return libraries
}
