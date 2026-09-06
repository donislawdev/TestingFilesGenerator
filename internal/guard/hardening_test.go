package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// A library is loaded lazily only when the system already has it mapped.
//
// syscall.NewLazyDLL goes through the standard search order, which has
// historically included the directory the program was started from. An outside
// review on 2026-09-05 asked for syscall.NewLazySystemDLL instead. Two things
// were measured on 2026-09-06 and they point the same way:
//
//   - THAT FUNCTION DOES NOT EXIST. syscall offers LoadDLL, LoadLibrary and
//     NewLazyDLL and no System variant, no LoadLibraryEx, and no
//     LOAD_LIBRARY_SEARCH_ constants. It lives in golang.org/x/sys/windows,
//     which untouchable rule 11 keeps out of the command line binary - the
//     comment beside the call has said so since 2026-08-25.
//   - THE ONE CALL WE MAKE NEVER REACHES THE SEARCH ORDER. kernel32.dll is a
//     KnownDLL - measured in the registry, 37 entries, one of them
//     "*kernel32 = kernel32.dll" - so it is already mapped and the loader hands
//     back what is there.
//
// So the finding is right about the NEXT call rather than about this one, and
// that is what this guard holds: this form is allowed for a KnownDLL and for
// nothing else. A second NewProc pointed at an ordinary library fails here with
// a sentence instead of inheriting the shape of the line above it.
//
// Read from the source rather than by calling anything, because the file is
// built only on Windows and this guard has to mean the same thing on the
// runners that are not.
func TestALibraryIsOnlyLoadedLazilyWhenTheSystemAlreadyHasIt(t *testing.T) {
	// The names this form may be used for, and why. Windows keeps these mapped
	// from boot, so the search order is never consulted for them. Measured
	// rather than remembered: the KnownDLLs registry key on 2026-09-06.
	knownDLLs := map[string]string{
		"kernel32.dll": "a KnownDLL, always already mapped - free space asks it for GetDiskFreeSpaceExW",
	}

	// Where a load may name something this cannot read, and why. A path worked
	// out at run time is the SAFE form when it is absolute and comes from the
	// system rather than from the search order, and it is the dangerous one
	// when it comes from anywhere else - a list of names cannot tell those
	// apart, so the file is named instead.
	byPath := map[string]string{
		"internal/gui/darkmenus_windows.go": "builds an absolute path from the system directory, because uxtheme.dll is not a KnownDLL",
	}

	root := repoRoot(t)
	used := map[string]bool{}
	viaPath := map[string]bool{}

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			if rel == "internal/guard" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "syscall" {
				return true
			}
			switch sel.Sel.Name {
			case "NewLazyDLL", "LoadDLL", "LoadLibrary":
			default:
				return true
			}
			name := loadedName(call)
			where := fset.Position(call.Pos()).Line
			if name == "" {
				if _, granted := byPath[rel]; !granted {
					t.Errorf("%s:%d loads a library under a name this guard cannot read.\n"+
						"A load whose argument is worked out at run time is safe when the path is "+
						"absolute and comes from the system, and unsafe when it comes from anywhere "+
						"else - and nothing here can tell those apart. Add this file to the list "+
						"above with the reason, or name a KnownDLL.", rel, where)
					return true
				}
				viaPath[rel] = true
				return true
			}
			if _, known := knownDLLs[name]; !known {
				t.Errorf("%s:%d loads %q with syscall.%s.\n"+
					"That goes through the standard search order, which has included the directory "+
					"the program was started from, and only a KnownDLL is immune because it is "+
					"already mapped. syscall has no System variant of this call - measured 2026-09-06 - "+
					"so a library that is not on the list above needs a deliberate answer rather than "+
					"this form: an absolute path, or golang.org/x/sys/windows with the owner's yes "+
					"under untouchable rule 11.", rel, where, name, sel.Sel.Name)
				return true
			}
			used[name] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal: %v", err)
	}

	if len(used) == 0 {
		t.Fatal("no library lookup was found at all, so this guard checked nothing. " +
			"internal/core/diskspace_windows.go makes one - if it has gone, take this with it.")
	}
	// An allowance that has outlived its call is an allowance nobody granted.
	for name, why := range knownDLLs {
		if !used[name] {
			t.Errorf("%s is allowed to be loaded this way (%s) and nothing loads it.\n"+
				"Delete the entry rather than leaving it to cover the next arrival.", name, why)
		}
	}
	for rel, why := range byPath {
		if !viaPath[rel] {
			t.Errorf("%s is allowed to load a library by a path it works out (%s) and it loads "+
				"none.\nDelete the entry rather than leaving it to cover whatever lands in that "+
				"file next.", rel, why)
		}
	}
}

// loadedName is the library a lazy load asks for, or the empty string when the
// argument is not a plain literal.
//
// Anything that is not a literal is reported as unnamed and refused by the
// caller, which is the answer that cannot be wrong: a name worked out at run
// time is exactly the case a list of allowed names cannot judge.
func loadedName(call *ast.CallExpr) string {
	if len(call.Args) != 1 {
		return ""
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	return strings.Trim(lit.Value, `"`)
}

// The text catalogue is loaded from exactly one place.
//
// internal/gui/text keeps the localiser in a package variable with no lock:
// Load writes it, and say, sayf and sayN read it on every string the window
// draws. That is safe because it happens once, in run, before the first screen
// exists - and that is a constraint rather than a property of the code.
//
// A second caller makes it a write racing every read on the interface thread. A
// language switch is the obvious one and LoadBuiltIn's own comment names it as
// its own piece of work. The race detector would not necessarily say so, because
// fyne.Do runs on the calling goroutine under the test driver, which is exactly
// the condition that hides threading defects in this tree.
//
// So the constraint is held here rather than defended with an atomic pointer
// nothing in this build could redden - a shape this project removes rather than
// keeps. Whoever adds the language switch meets this guard and starts from the
// sentence on the variable. Raised by an outside review on 2026-09-05.
func TestTheTextCatalogueIsLoadedFromOnePlaceOnly(t *testing.T) {
	root := repoRoot(t)

	// Where the one call is allowed to be, and why. An entry naming a file that
	// no longer calls it is a failure below rather than a comment nobody reads.
	allowed := map[string]string{
		"internal/gui/run_cgo.go": "before the first screen is built, which is what makes a plain variable enough",
	}

	found := map[string]int{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if d.IsDir() {
			switch rel {
			case "internal/guard", "internal/gui/text":
				// The package itself, where Load and LoadBuiltIn are declared
				// and where LoadBuiltIn calls Load.
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "text" {
				return true
			}
			if sel.Sel.Name == "Load" || sel.Sel.Name == "LoadBuiltIn" {
				found[rel]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal: %v", err)
	}

	total := 0
	for rel, n := range found {
		total += n
		if _, granted := allowed[rel]; !granted {
			t.Errorf("%s loads the text catalogue, and only one place may.\n"+
				"The localiser is a package variable with no lock, written by Load and read by "+
				"every string the window draws. A second caller is a write racing those reads on "+
				"the interface thread, and the race detector will not necessarily say so. If this "+
				"is the language switch, that work starts by making the localiser safe to replace - "+
				"see the comment on it.", rel)
		}
	}
	if total != 1 {
		t.Errorf("the text catalogue is loaded %d times and it has to be loaded exactly once: %v.\n"+
			"Once is what makes a variable with no lock enough.", total, found)
	}
	for rel, why := range allowed {
		if found[rel] == 0 {
			t.Errorf("%s is listed as the one place that loads the text catalogue (%s) and it does "+
				"not load it.\nDelete the entry, or move it to wherever the call went - a standing "+
				"exception for code that has gone quietly covers the next thing that lands there.",
				rel, why)
		}
	}
}
