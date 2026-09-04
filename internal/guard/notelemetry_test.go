package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Untouchable rule 8 and D16: this tool opens no outgoing connection. Two
// guards already look at that from the import graph - network_test.go reads our
// own import lines, and clinetwork_test.go asks the compiler what the command
// line binary actually links. Both answer the same question: does anything here
// reach for Go's networking.
//
// This file asks the other question, which neither of them can: is there
// ANOTHER way out. Four of them exist and none needs an import of net.
//
//   - An endpoint written into the code. A string is not an import, so a URL
//     sitting in a constant is invisible to both guards beside this one.
//   - A library loaded by name. internal/gui reaches uxtheme.dll through
//     syscall.NewLazyDLL and calls into it by ordinal, which is a documented and
//     wanted thing - and it is also the exact shape that would load wininet.dll
//     instead. This is Go's version of ctypes.windll.
//   - A socket opened under the import graph. syscall is legitimately imported
//     in eight shipped files for disk space, signals and dark menus, so banning
//     it is not available. Naming the socket shaped calls is.
//   - Shelling out. curl needs no networking package at all, and the exec ban in
//     network_test.go stops at layer 2 - internal/cli is layer 4 and cmd/tfg is
//     layer 5, so neither was covered by anything.
//
// So this is not a proof that no traffic leaves the machine. That still needs a
// traffic monitor and the requirement stands in docs/PRODUCT.md section 9. It is
// a LOCK on the surface: nobody adds a way out by accident, and adding one on
// purpose means editing a registry here and writing down why.
//
// Two things about the scope, both deliberate.
//
// It reads the files that go into a shipped binary, asked of the COMPILER
// rather than assumed from the tree. internal/oracle shells out to python and
// internal/site renders the website, and neither is shipped - judging them by
// the rules of the thing they measure is how an allowlist starts collecting
// exceptions that mean nothing.
//
// And it reads every .go file in those packages, INCLUDING the ones this system
// does not build. build.ImportDir honours the current build context, so
// diskspace_unix.go is invisible on Windows and diskspace_windows.go is
// invisible on Linux. A guard that only reads its own platform would let an
// endpoint into the other one and go green on every run that mattered.
//
// What it cannot see, said plainly rather than left to be discovered:
//
//   - A URL assembled at runtime. "https://" + host defeats the literal scan,
//     and nothing here executes anything.
//   - A third party module. The registries cover our own code. What the toolkit
//     links is a separate sentence, already written in clinetwork_test.go: the
//     window carries net/http it never starts, accepted on purpose and proven
//     rather than removed.
//   - Data leaving without a socket, such as a file written into a synced
//     folder. Nothing here looks at that.

// shippedPackage is one of our packages that the compiler puts into a binary.
type shippedPackage struct {
	rel   string
	files []string
}

// shippedSource lists our packages that reach either binary, with every .go
// file in them regardless of the build tags on it.
func shippedSource(t *testing.T) []shippedPackage {
	t.Helper()
	root := repoRoot(t)

	seen := map[string]bool{}
	for _, target := range []string{"../../cmd/tfg", "../../cmd/tfg-gui"} {
		for _, p := range linkedWithCGO(t, target) {
			if rel, ok := ourPackage(p); ok {
				seen[rel] = true
			}
		}
	}

	var out []shippedPackage
	for rel := range seen {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		var files []string
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			files = append(files, filepath.Join(dir, name))
		}
		out = append(out, shippedPackage{rel: rel, files: files})
	}

	// Below this the set is not the real one, and a registry checked against a
	// short list would report every entry as stale.
	if len(out) < 30 {
		t.Fatalf("only %d shipped package(s) found, which is too few to be the real set", len(out))
	}
	return out
}

// ourPackage turns a full import path into a module relative one.
func ourPackage(p string) (string, bool) {
	if p == modulePath {
		return "", true
	}
	if !strings.HasPrefix(p, modulePath+"/") {
		return "", false
	}
	return strings.TrimPrefix(p, modulePath+"/"), true
}

// --------------------------------------------------------------------------
// The registries. Each entry carries the reason it exists, and each is exact in
// BOTH directions: an unlisted use fails, and a listed one naming code that is
// gone fails too. Permission that outlives the code it was granted for is how
// an allowlist rots into a rubber stamp.
// --------------------------------------------------------------------------

// allowedURLs are the addresses that may appear as literals in shipped code.
//
// Every one of them but the last two is an XML namespace, which is an
// identifier and not an address: it names a vocabulary, it is written INTO the
// files this tool generates, and nothing ever fetches it. They are here anyway,
// because a scan that guessed which URLs are "only identifiers" would be
// guessing at exactly the shape somebody would use to hide one.
var allowedURLs = map[string]string{
	"internal/format/docx/docx.go|http://schemas.openxmlformats.org/officeDocument/2006/relationships": "OPC namespace, written into the document",
	"internal/format/docx/docx.go|http://schemas.openxmlformats.org/wordprocessingml/2006/main":        "WordprocessingML namespace",
	"internal/format/opc/opc.go|http://schemas.openxmlformats.org/package/2006/content-types":          "OPC content types namespace",
	"internal/format/opc/opc.go|http://schemas.openxmlformats.org/package/2006/relationships":          "OPC relationships namespace",
	"internal/format/pptx/pptx.go|http://schemas.openxmlformats.org/drawingml/2006/main":               "DrawingML namespace",
	"internal/format/pptx/pptx.go|http://schemas.openxmlformats.org/officeDocument/2006/relationships": "OPC namespace, written into the document",
	"internal/format/pptx/pptx.go|http://schemas.openxmlformats.org/presentationml/2006/main":          "PresentationML namespace",
	"internal/format/xlsx/xlsx.go|http://schemas.openxmlformats.org/officeDocument/2006/relationships": "OPC namespace, written into the document",
	"internal/format/xlsx/xlsx.go|http://schemas.openxmlformats.org/spreadsheetml/2006/main":           "SpreadsheetML namespace",
	"internal/format/svgfile/svg.go|http://www.w3.org/2000/svg":                                        "SVG namespace, written into the image",
	"internal/legal/spdx.go|https://github.com/donislawdev/TestingFilesGenerator":                      "this project, named in the bill of materials it writes",
	"internal/legal/modules.go|https://github.com/nicksnyder":                                          "part of a third party copyright line, copied into the notices we ship",
	"internal/version/version.go|https://www.gnu.org/licenses/gpl-3.0.html":                            "where the GPL text is, named by the licence notice this tool prints",
	"internal/gui/text/screens.go|https://donislawdev.com/support/": "the support page, handed to the system browser on a click - " +
		"untouchable rule 8 permits this and permits nothing else like it",
}

// allowedLibraries are the libraries shipped code may load by name.
//
// Both are local to the machine and neither can carry anything off it. The
// entry that matters is the one that is NOT here: wininet, winhttp and ws2_32
// are loaded the same way and would reach a network without one line of Go
// networking.
var allowedLibraries = map[string]string{
	"kernel32.dll": "free disk space, and GetProcAddress for the call below",
	"uxtheme.dll":  "the dark window menu, called by ordinal - see darkmenus_windows.go",
}

// socketCalls are the low level calls that open or use a socket. syscall itself
// cannot be banned - it carries disk space, signals and the dark menu - so the
// ban is on the calls rather than on the package.
var socketCalls = map[string]bool{
	"Socket": true, "Connect": true, "Bind": true, "Listen": true,
	"Accept": true, "Accept4": true, "Sendto": true, "Recvfrom": true,
	"SendmsgN": true, "Recvmsg": true, "Send": true, "Recv": true,
	"WSASocket": true, "WSAStartup": true, "WSAConnect": true,
	"WSASendTo": true, "WSARecvFrom": true, "WSASend": true, "WSARecv": true,
	"GetAddrInfoW": true, "DnsQuery": true,
}

// lowLevelPackages are the ones whose calls socketCalls is read against. The
// local name a file gives them is resolved from its own imports, so an alias
// changes nothing - an import statement names its module whatever it calls it.
var lowLevelPackages = map[string]bool{
	"syscall":                   true,
	"golang.org/x/sys/windows":  true,
	"golang.org/x/sys/unix":     true,
	"golang.org/x/sys/execabs":  true,
	"os/exec":                   true,
	"golang.org/x/net/internal": true,
}

// spawnCalls start another program. curl needs no networking package, so a
// guard on imports alone leaves this open.
var spawnCalls = map[string]bool{
	"Command": true, "CommandContext": true, "StartProcess": true,
	"Run": true, "Start": true, "Output": true, "CombinedOutput": true,
}

// --------------------------------------------------------------------------
// The scan. One function producing typed findings rather than several loops,
// so the canary at the bottom can point it at code it must reject.
// --------------------------------------------------------------------------

// telemetryFinding is one thing worth refusing, named by kind so the canary can
// require the right one rather than any one at all.
type telemetryFinding struct {
	kind   string // "url", "library", "socket", "spawn"
	detail string
}

// telemetryFindings reads one file's source and reports every way out it holds.
func telemetryFindings(src, rel string) []telemetryFinding {
	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return []telemetryFinding{{kind: "unparsed", detail: err.Error()}}
	}

	local := lowLevelLocalNames(tree)
	var found []telemetryFinding
	ast.Inspect(tree, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			found = append(found, urlFinding(node, rel)...)
		case *ast.CallExpr:
			found = append(found, callFindings(node, local)...)
		}
		return true
	})
	return found
}

// lowLevelLocalNames maps the name a file uses for a low level package back to
// the package itself. This is the half that makes the scan alias proof.
func lowLevelLocalNames(tree *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range tree.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !lowLevelPackages[path] {
			continue
		}
		if imp.Name != nil {
			names[imp.Name.Name] = true
			continue
		}
		names[path[strings.LastIndex(path, "/")+1:]] = true
	}
	return names
}

// urlSchemes are the ones worth finding. Each is a way of naming somewhere
// else, which is the whole of what this looks for.
var urlSchemes = []string{"https://", "http://", "ftp://", "ws://", "wss://"}

// urlsIn pulls every address out of one string, wherever in it they sit.
//
// It reads ANYWHERE in the literal, and the first version of this file did
// not - it asked whether the literal STARTED with a scheme. That was written,
// reviewed, canaried with thirteen cases and shipped past all of them, because
// every case in the canary made the URL the whole string. What found it was the
// staleness half: three registered namespaces reported as gone, when they were
// sitting in plain sight inside `<svg xmlns="http://www.w3.org/2000/svg" ...>`.
//
// So the hole was real and the shape of it is worth keeping: an endpoint
// written as "POST https://telemetry.example/v1" walked straight through a
// guard whose whole job was that string. The canary now carries the embedded
// shapes too.
func urlsIn(text string) []string {
	var found []string
	for i := 0; i < len(text); i++ {
		rest := text[i:]
		for _, scheme := range urlSchemes {
			if !strings.HasPrefix(rest, scheme) {
				continue
			}
			end := strings.IndexAny(rest, "\"'`<>, \t\n\\")
			if end < 0 {
				end = len(rest)
			}
			found = append(found, rest[:end])
			i += end - 1
			break
		}
	}
	return found
}

// urlFinding reports an address in a string literal that is not registered for
// this file.
func urlFinding(lit *ast.BasicLit, rel string) []telemetryFinding {
	if lit.Kind != token.STRING {
		return nil
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	var found []telemetryFinding
	for _, url := range urlsIn(text) {
		if _, ok := allowedURLs[rel+"|"+url]; ok {
			continue
		}
		found = append(found, telemetryFinding{kind: "url", detail: rel + " holds " + url})
	}
	return found
}

// callFindings reports a call that loads a library, opens a socket or starts a
// program.
func callFindings(call *ast.CallExpr, local map[string]bool) []telemetryFinding {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	name := sel.Sel.Name

	if name == "NewLazyDLL" || name == "NewLazySystemDLL" || name == "LoadDLL" || name == "LoadLibrary" {
		return libraryFinding(call)
	}

	// A call on a name the file resolved to a low level package. The receiver
	// has to be that name, so an ordinary method called Send on our own type is
	// not mistaken for a socket.
	ident, ok := sel.X.(*ast.Ident)
	if !ok || !local[ident.Name] {
		return nil
	}
	if socketCalls[name] {
		return []telemetryFinding{{kind: "socket", detail: ident.Name + "." + name}}
	}
	if spawnCalls[name] {
		return []telemetryFinding{{kind: "spawn", detail: ident.Name + "." + name}}
	}
	return nil
}

// libraryFinding reads the library a load call names, and reports it when the
// name is computed rather than written down - a library nobody can read here is
// a library this guard cannot vouch for.
func libraryFinding(call *ast.CallExpr) []telemetryFinding {
	if len(call.Args) == 0 {
		return nil
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return []telemetryFinding{{kind: "library", detail: "a library named by something other than a literal"}}
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	if _, ok := allowedLibraries[strings.ToLower(name)]; ok {
		return nil
	}
	return []telemetryFinding{{kind: "library", detail: name}}
}

// --------------------------------------------------------------------------
// The guards
// --------------------------------------------------------------------------

// Nothing in a shipped binary holds an address, loads an unnamed library, opens
// a socket or starts a program, unless it is registered above with its reason.
func TestNothingShippedHoldsAnUndeclaredWayOut(t *testing.T) {
	scanned := 0
	for _, p := range shippedSource(t) {
		for _, file := range p.files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			scanned++
			rel := p.rel + "/" + filepath.Base(file)
			for _, f := range telemetryFindings(string(raw), rel) {
				t.Errorf("%s (%s).\n"+
					"Reason: untouchable rule 8 says this tool opens no outgoing connection, and this is a way\n"+
					"out that no import of net would show. If it is wanted, add it to the registry in\n"+
					"notelemetry_test.go with the reason - if it is not, this is the guard doing its job.",
					f.detail, f.kind)
			}
		}
	}
	if scanned < 60 {
		t.Fatalf("only %d shipped file(s) were read, which is too few to be the real tree", scanned)
	}
	t.Logf("%d shipped file(s) read, across every build tag rather than this system's", scanned)
}

// Every entry in the two registries names code that is still there.
//
// This is the direction an allowlist rots in. A permission nobody removed reads
// as a decision somebody made, and the next person adding a line beside it has
// a longer list to argue from.
func TestEveryTelemetryExceptionStillNamesLiveCode(t *testing.T) {
	urls, libraries := map[string]bool{}, map[string]bool{}
	for _, p := range shippedSource(t) {
		for _, file := range p.files {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("reading %s: %v", file, err)
			}
			rel := p.rel + "/" + filepath.Base(file)
			collectUses(string(raw), rel, urls, libraries)
		}
	}

	for key := range allowedURLs {
		if !urls[key] {
			t.Errorf("the registry allows %q and no shipped file holds it any more.\n"+
				"Reason: an exception that outlives its code is permission nobody granted. Delete the line.", key)
		}
	}
	for name := range allowedLibraries {
		if !libraries[name] {
			t.Errorf("the registry allows loading %q and no shipped file loads it any more.\n"+
				"Reason: the same - a stale permission is the half of an allowlist nobody rereads.", name)
		}
	}
}

// collectUses records which registered entries a file actually uses. It reads
// the same shapes the scan does, so the two halves cannot disagree about what
// counts as a use.
func collectUses(src, rel string, urls, libraries map[string]bool) {
	fset := token.NewFileSet()
	tree, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return
	}
	ast.Inspect(tree, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind != token.STRING {
				return true
			}
			text, err := strconv.Unquote(node.Value)
			if err != nil {
				return true
			}
			// The same extraction the scan uses, called rather than repeated,
			// so the two halves cannot disagree about what an address is.
			for _, url := range urlsIn(text) {
				urls[rel+"|"+url] = true
			}
		case *ast.CallExpr:
			if name, ok := loadedLibrary(node); ok {
				libraries[strings.ToLower(name)] = true
			}
		}
		return true
	})
}

// loadedLibrary reads the literal name out of a library load call.
func loadedLibrary(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	switch sel.Sel.Name {
	case "NewLazyDLL", "NewLazySystemDLL", "LoadDLL", "LoadLibrary":
	default:
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(lit.Value)
	return name, err == nil
}

// The command line binary carries no way to start another program.
//
// Asked of the compiler, like the network question beside it in
// clinetwork_test.go, and for the same reason: what ends up in a binary is
// decided by build constraints, and a test reading import lines sees its own
// build instead.
//
// MEASURED 2026-09-04: cmd/tfg links os/exec zero times and cmd/tfg-gui links
// it once, through the toolkit that opens the support page on a click. So this
// holds the command line at what it already is, and does not pretend the window
// is the same sentence - that difference is real and untouchable rule 8 names
// it, because handing an address to the system browser is not the program
// connecting to anything.
func TestTheCommandLineBinaryCannotStartAProcess(t *testing.T) {
	var found []string
	for _, p := range linkedWithCGO(t, "../../cmd/tfg") {
		if p == "os/exec" || p == "golang.org/x/sys/execabs" {
			found = append(found, p)
		}
	}
	if len(found) > 0 {
		t.Errorf("the command line binary links %s.\n"+
			"Reason: a program this tool starts needs no networking package of its own, so shelling out\n"+
			"is the way around every other guard here - curl is one line and one dependency away.\n"+
			"What to do: whatever needs to spawn something has to stop, or this promise has to change\n"+
			"and untouchable rule 8 has to change with it.",
			strings.Join(found, ", "))
	}
}

// --------------------------------------------------------------------------
// The canary. A guard that has only ever seen clean code has been shown to run,
// not to look.
//
// Every alias shape is here separately rather than as one representative case,
// because that is where the sister project measured its own scan reading one
// spelling out of three. In Go an import statement always carries the real path,
// so the resolving is easier - but "easier" is not "done", and the cases below
// are what says so.
// --------------------------------------------------------------------------

var badTelemetryCode = []struct {
	label string
	src   string
	kind  string
}{
	{"an endpoint written into the code",
		"package p\n\nconst Endpoint = \"https://telemetry.example/v1\"\n", "url"},
	{"the same one as a websocket",
		"package p\n\nconst Endpoint = \"wss://telemetry.example/v1\"\n", "url"},
	// The three below are the shape that walked through the first version of
	// this file. They are here one by one rather than as one case, because the
	// reason the first version missed them was that every case in its canary
	// put the address on its own.
	{"an endpoint sitting INSIDE a longer string",
		"package p\n\nconst Line = \"POST https://telemetry.example/v1 HTTP/1.1\"\n", "url"},
	{"an endpoint inside a raw string, the way a namespace is written",
		"package p\n\nvar body = `<ping to=\"https://telemetry.example/v1\"/>`\n", "url"},
	{"an endpoint in a format string, which is how one would be built",
		"package p\n\nconst Format = \"https://telemetry.example/v1?id=%s\"\n", "url"},
	{"a network library loaded by name",
		"package p\n\nimport \"syscall\"\n\nvar d = syscall.NewLazyDLL(\"wininet.dll\")\n", "library"},
	{"the same one under an import alias",
		"package p\n\nimport sc \"syscall\"\n\nvar d = sc.NewLazyDLL(\"winhttp.dll\")\n", "library"},
	{"a library whose name is computed, so nobody here can read it",
		"package p\n\nimport \"syscall\"\n\nfunc f(n string) { _ = syscall.NewLazyDLL(n) }\n", "library"},
	{"a socket opened straight from syscall",
		"package p\n\nimport \"syscall\"\n\nfunc f() { _, _ = syscall.Socket(2, 1, 0) }\n", "socket"},
	{"the same call under an alias",
		"package p\n\nimport sys \"syscall\"\n\nfunc f() { _, _ = sys.Socket(2, 1, 0) }\n", "socket"},
	{"a connect through x/sys/windows",
		"package p\n\nimport \"golang.org/x/sys/windows\"\n\nfunc f() { _ = windows.Connect(0, nil, 0) }\n", "socket"},
	{"the same through an aliased x/sys/windows",
		"package p\n\nimport w \"golang.org/x/sys/windows\"\n\nfunc f() { _ = w.WSAConnect(0) }\n", "socket"},
	{"a name resolved before anything is sent",
		"package p\n\nimport \"syscall\"\n\nfunc f() { _ = syscall.GetAddrInfoW(nil, nil, nil, nil) }\n", "socket"},
	{"shelling out to curl",
		"package p\n\nimport \"os/exec\"\n\nfunc f() { _ = exec.Command(\"curl\", \"https://x.example\") }\n", "spawn"},
	{"shelling out under an alias",
		"package p\n\nimport sh \"os/exec\"\n\nfunc f() { _ = sh.Command(\"curl\") }\n", "spawn"},
	{"starting a process the plainest way",
		"package p\n\nimport \"syscall\"\n\nfunc f() { _, _ = syscall.StartProcess(\"curl\", nil, nil) }\n", "spawn"},
}

func TestTheTelemetryScannerRejectsCodeItMustReject(t *testing.T) {
	var missed []string
	for _, c := range badTelemetryCode {
		kinds := map[string]bool{}
		for _, f := range telemetryFindings(c.src, "internal/format/txt/made-up.go") {
			kinds[f.kind] = true
		}
		if !kinds[c.kind] {
			missed = append(missed, c.label+" - wanted "+c.kind+", saw "+strings.Join(sortedKeys(kinds), ","))
		}
	}
	if len(missed) > 0 {
		t.Errorf("the scanner missed %d shape(s) it exists to catch:\n  %s\n"+
			"Reason: a guard that has only ever read clean code has been shown to run, not to look.",
			len(missed), strings.Join(missed, "\n  "))
	}
}

// And it does not reach that by flagging everything, or the case above proves
// nothing at all.
func TestTheTelemetryScannerLeavesOrdinaryCodeAlone(t *testing.T) {
	// It carries ordinary strings on purpose. A scanner reading every literal
	// as an address would satisfy the canary above and refuse the whole tree,
	// and with nothing but comments here that mistake would go unnoticed.
	clean := "package p\n\n" +
		"import (\n\t\"os\"\n\t\"syscall\"\n)\n\n" +
		"// Read a file. Not https://example.invalid - that is prose, in a comment.\n" +
		"const Name = \"report.txt\"\n\n" +
		"func Read(path string) ([]byte, error) {\n" +
		"\tvar st syscall.Stat_t\n" +
		"\t_ = st\n" +
		"\tif path == \"\" {\n\t\tpath = Name\n\t}\n" +
		"\treturn os.ReadFile(path)\n" +
		"}\n"
	if found := telemetryFindings(clean, "internal/format/txt/made-up.go"); len(found) > 0 {
		t.Errorf("ordinary code produced %d finding(s): %v.\n"+
			"Reason: a scanner that refuses everything satisfies the canary and guards nothing.",
			len(found), found)
	}
}

// And a registered address in the file it is registered for stays allowed, so
// the registry is doing work rather than being carried.
func TestARegisteredAddressInItsOwnFileIsAllowed(t *testing.T) {
	src := "package text\n\nconst SupportURL = \"https://donislawdev.com/support/\"\n"
	if found := telemetryFindings(src, "internal/gui/text/screens.go"); len(found) > 0 {
		t.Errorf("the support page was refused in the file that is registered for it: %v", found)
	}
	if found := telemetryFindings(src, "internal/cli/cli.go"); len(found) == 0 {
		t.Error("the same address was allowed in a file it is NOT registered for.\n" +
			"Reason: the registry is per file on purpose. An address permitted everywhere once it is\n" +
			"permitted anywhere would let the next one in beside it.")
	}
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return []string{"nothing"}
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
