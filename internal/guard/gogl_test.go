package guard

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Where the copy of the OpenGL binding lives, relative to the repository root,
// and the module it replaces. Both are read back out of go.mod below, so this
// pair is the expectation and go.mod is the state.
const (
	bindingModule = "github.com/go-gl/gl"
	bindingCopy   = "third_party/go-gl-gl"
)

// The lines the copy takes out of the published module, exactly as the
// published files carry them. Two files carry the same directive, and the
// second one is the one the handover notes missed: procaddr.go names it and
// so does the generated package.go, on its own line of the cgo preamble.
const (
	bindingLDFLAGSInProcaddr = "#cgo !gles2,windows       LDFLAGS: -lopengl32\n"
	bindingLDFLAGSInPackage  = "// #cgo !gles2,windows       LDFLAGS: -lopengl32\n"
)

// The Windows branch of GlowGetProcAddress as published, and as the copy
// carries it. The published one calls wglGetProcAddress as an imported symbol,
// which is what puts opengl32.dll into the executable's import table. The copy
// looks the symbol up after loading the library by name, so nothing is
// imported at load time and a library already mapped under that name is the
// one used - see third_party/go-gl-gl/PATCH.md.
const bindingLookupAsPublished = `	static HMODULE ogl32dll = NULL;
	static void* GlowGetProcAddress(const char* name) {
		void* pf = wglGetProcAddress((LPCSTR) name);
		if (pf) {
			return pf;
		}
		if (ogl32dll == NULL) {
			ogl32dll = LoadLibraryA("opengl32.dll");
		}
		return GetProcAddress(ogl32dll, (LPCSTR) name);
	}
`

const bindingLookupAsPatched = `	// PATCHED - see PATCH.md at the root of this copy. wglGetProcAddress is
	// looked up at run time instead of being imported, so the executable
	// carries no load time import of opengl32.dll, and a library already
	// mapped under that name before the first call here is the one used.
	typedef PROC (WINAPI *wglGetProcAddressFn)(LPCSTR);
	static HMODULE ogl32dll = NULL;
	static wglGetProcAddressFn wglGetProcAddressPtr = NULL;
	static void* GlowGetProcAddress(const char* name) {
		if (ogl32dll == NULL) {
			ogl32dll = LoadLibraryA("opengl32.dll");
			if (ogl32dll == NULL) {
				return NULL;
			}
			wglGetProcAddressPtr = (wglGetProcAddressFn) GetProcAddress(ogl32dll, "wglGetProcAddress");
		}
		if (wglGetProcAddressPtr != NULL) {
			void* pf = (void*) wglGetProcAddressPtr((LPCSTR) name);
			if (pf) {
				return pf;
			}
		}
		return (void*) GetProcAddress(ogl32dll, (LPCSTR) name);
	}
`

// The OpenGL binding is the published module plus exactly the patch.
//
// go.mod replaces github.com/go-gl/gl with a directory in this repository,
// because the published module makes the window binary import opengl32.dll at
// load time and that import is what stops a software renderer from ever being
// the library the toolkit finds (docs/GUI-NO-OPENGL section 7, measured
// 2026-09-17). A replaced module is a module Dependabot no longer bumps and a
// module whose sum go.sum no longer carries, so two things that used to be
// held by the toolchain are held here instead: that the copy is the version
// the directive names, byte for byte, apart from the one change PATCH.md
// describes, and that the version it names still has the sum it had the day
// the copy was made.
//
// The published version is downloaded rather than read from the module cache,
// because a replaced module is not in the cache of a fresh checkout. That
// needs the toolchain and, on a clean machine, the network - so the absence of
// the toolchain is a skip, said out loud, and everything else is a failure.
func TestTheOpenGLBindingIsThePinnedModulePlusExactlyThePatch(t *testing.T) {
	root := repoRoot(t)
	version := bindingVersion(t, root)
	patchedVersion, patchedSum := bindingPatchNote(t, root)
	if patchedVersion != version {
		t.Fatalf("go.mod requires %s %s and %s/PATCH.md describes %s.\n"+
			"The note has to name the version the copy was taken from, or the guard below\n"+
			"compares the copy with a version nobody copied.", bindingModule, version, bindingCopy, patchedVersion)
	}

	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no Go toolchain here, so the published module cannot be fetched to compare with: %v", err)
	}
	published, sum := bindingDownload(t, root, version)
	if sum != patchedSum {
		t.Fatalf("%s@%s downloads with sum %s and %s/PATCH.md says %s.\n"+
			"go.sum stopped carrying this module's sum when it was replaced, so the note is\n"+
			"where the pin lives now - and a different sum means different bytes under the\n"+
			"same version, which is exactly what a pin exists to notice.",
			bindingModule, version, sum, bindingCopy, patchedSum)
	}

	copied := filepath.Join(root, filepath.FromSlash(bindingCopy))
	bindingCopyIsPublishedPlusPatch(t, copied, published)
	bindingCopyIsComplete(t, copied, published)
}

// bindingCopyIsPublishedPlusPatch holds every file of the copy to the
// published file at the same path, transformed by the patch where the patch
// applies and untouched everywhere else.
func bindingCopyIsPublishedPlusPatch(t *testing.T, copied, published string) {
	t.Helper()
	compared := 0
	err := filepath.WalkDir(copied, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(copied, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "PATCH.md" {
			return nil
		}
		if !bindingCarries(rel) {
			t.Errorf("%s/%s is not go.mod, LICENSE, PATCH.md or a file of the two packages carried.\n"+
				"The copy holds what the window imports and nothing else - see PATCH.md.", bindingCopy, rel)
			return nil
		}
		theirs, err := os.ReadFile(filepath.Join(published, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s/%s has no counterpart in the published module: %v", bindingCopy, rel, err)
			return nil
		}
		ours, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		want := bindingPatched(t, rel, string(theirs))
		compared++
		if string(ours) != want {
			t.Errorf("%s/%s differs from the published file plus the patch, first at line %d.\n"+
				"The copy is allowed exactly the change PATCH.md describes and this is not it.",
				bindingCopy, rel, firstDifferingLine(want, string(ours)))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", bindingCopy, err)
	}
	if compared == 0 {
		t.Fatalf("no file of %s was compared, so this guard checked nothing", bindingCopy)
	}
}

// bindingCopyIsComplete holds the copy to carrying every file of each package
// it carries. A package missing one file is not a package the compiler will
// take, so this is about the next file upstream adds rather than about today.
func bindingCopyIsComplete(t *testing.T, copied, published string) {
	t.Helper()
	for _, pkg := range []string{"v2.1/gl", "v3.1/gles2"} {
		dir := filepath.Join(published, filepath.FromSlash(pkg))
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(published, path)
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(copied, rel)); err != nil {
				t.Errorf("the published %s carries %s and the copy does not.\n"+
					"A package is carried whole or not at all.", pkg, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking the published %s: %v", pkg, err)
		}
	}
}

// bindingCarries says whether a path inside the copy is one the copy is meant
// to hold: the module's own two files and the two packages the window imports.
func bindingCarries(rel string) bool {
	return rel == "go.mod" || rel == "LICENSE" ||
		strings.HasPrefix(rel, "v2.1/gl/") || strings.HasPrefix(rel, "v3.1/gles2/")
}

// bindingPatched applies the patch to a published file, and asserts that the
// text it changes is there to be changed. A transformation that finds nothing
// to transform would hold the copy to an unpatched file, which is the trap of
// a guard that no longer reaches the state it exists to check.
func bindingPatched(t *testing.T, rel, published string) string {
	t.Helper()
	switch rel {
	case "v2.1/gl/package.go":
		return bindingWithout(t, rel, published, bindingLDFLAGSInPackage)
	case "v2.1/gl/procaddr.go":
		text := bindingWithout(t, rel, published, bindingLDFLAGSInProcaddr)
		if strings.Count(text, bindingLookupAsPublished) != 1 {
			t.Fatalf("the published %s does not carry the Windows lookup this guard patches, exactly once.\n"+
				"Upstream changed the function. Read it, redo the patch in the copy, and update the\n"+
				"published form above - the guard has to describe the same change the copy makes.", rel)
		}
		return strings.Replace(text, bindingLookupAsPublished, bindingLookupAsPatched, 1)
	default:
		return published
	}
}

// bindingWithout removes one line that has to be there exactly once.
func bindingWithout(t *testing.T, rel, text, line string) string {
	t.Helper()
	if strings.Count(text, line) != 1 {
		t.Fatalf("the published %s does not carry %q exactly once, so there is nothing for the copy to have taken out.\n"+
			"Upstream changed how it links. Read it before touching the copy.", rel, strings.TrimSpace(line))
	}
	return strings.Replace(text, line, "", 1)
}

// bindingVersion reads the version go.mod requires for the binding, and
// asserts that go.mod replaces it with the copy - the directive is the state
// this whole file describes.
func bindingVersion(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	replaced := regexp.MustCompile(`(?m)^replace ` + regexp.QuoteMeta(bindingModule) + ` => (\S+)$`).FindStringSubmatch(string(raw))
	if replaced == nil {
		t.Fatalf("go.mod does not replace %s, so the window binary imports opengl32.dll at load time again", bindingModule)
	}
	if replaced[1] != "./"+bindingCopy {
		t.Fatalf("go.mod replaces %s with %s and this guard reads %s", bindingModule, replaced[1], "./"+bindingCopy)
	}
	required := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(bindingModule) + ` (v\S+)`).FindStringSubmatch(string(raw))
	if required == nil {
		t.Fatalf("go.mod does not require %s at any version", bindingModule)
	}
	return required[1]
}

// bindingPatchNote reads the version and the module sum PATCH.md pins.
func bindingPatchNote(t *testing.T, root string) (version, sum string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(bindingCopy), "PATCH.md"))
	if err != nil {
		t.Fatalf("the copy carries no PATCH.md: %v", err)
	}
	v := regexp.MustCompile("`(v0\\.0\\.0-[0-9]{14}-[0-9a-f]{12})`").FindStringSubmatch(string(raw))
	s := regexp.MustCompile("`(h1:[A-Za-z0-9+/]+=*)`").FindStringSubmatch(string(raw))
	if v == nil || s == nil {
		t.Fatalf("%s/PATCH.md has to name the version and the module sum it was taken from, in backticks", bindingCopy)
	}
	return v[1], s[1]
}

// bindingDownload fetches the published module and answers with where it was
// put and the sum the toolchain computed for it.
func bindingDownload(t *testing.T, root, version string) (dir, sum string) {
	t.Helper()
	cmd := exec.Command("go", "mod", "download", "-json", bindingModule+"@"+version)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go mod download %s@%s: %v\n%s", bindingModule, version, err, out)
	}
	var answer struct {
		Dir string
		Sum string
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		t.Fatalf("reading what go mod download said: %v\n%s", err, out)
	}
	if answer.Dir == "" || answer.Sum == "" {
		t.Fatalf("go mod download named no directory or no sum:\n%s", out)
	}
	return answer.Dir, answer.Sum
}

// firstDifferingLine is the one based number of the first line on which two
// texts disagree, for a message that says where to look.
func firstDifferingLine(a, b string) int {
	as, bs := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range as {
		if i >= len(bs) || as[i] != bs[i] {
			return i + 1
		}
	}
	return len(as) + 1
}

// A nested module is not a package of this one.
//
// packages() walks the repository and skips a directory that carries its own
// go.mod, the way "./..." does. This asks that the skip reaches the one nested
// module there is: the copy of the binding is there, holding a 2.3 MB
// generated file with C in its comments, and no guard reading the shape of
// this module's code sees it. A walk that stopped skipping would hand that
// file to fourteen guards at once, and they would all be right to refuse it.
func TestANestedModuleIsNotAPackageOfThisModule(t *testing.T) {
	root := repoRoot(t)
	generated := filepath.Join(root, filepath.FromSlash(bindingCopy), "v2.1", "gl", "package.go")
	if _, err := os.Stat(generated); err != nil {
		t.Fatalf("the nested module this guard expects to be skipped is not there: %v", err)
	}
	for _, p := range packages(t) {
		if strings.HasPrefix(p.rel, "third_party/") {
			t.Errorf("packages() lists %s, which is inside another module.\n"+
				"A directory with its own go.mod is outside \"./...\" and has to be outside this walk.", p.rel)
		}
	}
	// And the walk skipped exactly that module. A go.mod dropped into a
	// first party directory would take it out of every guard that reads
	// packages(), and nothing but this line would say so.
	if got := strings.Join(nestedModules, ","); got != bindingCopy {
		t.Errorf("the walk skipped %q as nested modules, and the only one there is %s.\n"+
			"A directory that grew a go.mod of its own has left every guard that reads packages().", got, bindingCopy)
	}
}
