package guard

import (
	"os/exec"
	"strings"
	"testing"
)

// D16 and untouchable rule 8: zero outgoing connections. Until 2026-08-05 the
// evidence for that was a scan of our own import statements, and on that day
// it stopped being enough.
//
// The graphics toolkit imports net/http. Measured before it was added: three
// call sites reach it - a repository for http:// URIs that nothing registers,
// a public helper for loading a resource from a URL that nothing calls, and a
// driver that downloads a DLL from dl.google.com, which is not linked into a
// desktop build at all. So the window binary carries HTTP machinery it never
// starts.
//
// The owner's decision was to accept that and prove it rather than to change
// toolkit. This is the proof, and it is deliberately stronger than the scan
// beside it: that one reads the import lines of our own packages, and this one
// asks the compiler what actually goes into the binary. An import arriving
// four modules deep is invisible to the first and caught by the second.
//
// What it covers is the command line, where the promise is absolute. The
// window is a separate binary and a separate sentence - see the guard on our
// own source below.

// networkPackages are the ones whose presence means the binary can open a
// socket. Named rather than pattern matched on "net", because net/url parses
// text and reaches nothing.
func isNetworkPackage(p string) bool {
	switch p {
	case "net", "net/http", "net/smtp", "net/rpc", "crypto/tls":
		return true
	}
	switch {
	case strings.HasPrefix(p, "net/http/"),
		strings.HasPrefix(p, "golang.org/x/net/"),
		p == "golang.org/x/net":
		return true
	}
	return false
}

func linkedPackages(t *testing.T, target string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", target).Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			pkgs = append(pkgs, p)
		}
	}
	if len(pkgs) < 20 {
		t.Fatalf("go list returned %d packages, which is too few to be the real dependency set", len(pkgs))
	}
	return pkgs
}

// The command line binary contains no machinery for talking to a network.
//
// This is the sentence a person auditing the tool can check for themselves,
// and it is now true of the artefact rather than of the source that produced
// it.
func TestTheCommandLineBinaryCannotReachTheNetwork(t *testing.T) {
	var found []string
	for _, p := range linkedPackages(t, "../../cmd/tfg") {
		if isNetworkPackage(p) {
			found = append(found, p)
		}
	}
	if len(found) > 0 {
		t.Errorf("the command line binary links %s.\n"+
			"D16 and untouchable rule 8 say this tool makes no network calls, and a binary "+
			"carrying a socket cannot demonstrate that to anybody auditing it. If a dependency "+
			"brought this in, the dependency is the thing to reconsider.",
			strings.Join(found, ", "))
	}
}

// And our own window code does not reach for a network either.
//
// The toolkit's own imports are outside our control and that is written down.
// Ours are not, so this asks the narrower question the scan beside it cannot:
// no file we wrote under internal/gui imports a network package, whatever the
// toolkit under it does.
func TestOurOwnWindowCodeDoesNotReachTheNetwork(t *testing.T) {
	pkgs := packages(t)
	checked := 0
	for _, p := range pkgs {
		if !strings.HasPrefix(p.rel, "internal/gui") && p.rel != "cmd/tfg-gui" {
			continue
		}
		checked++
		// rawImports rather than p.imports. The latter keeps only imports
		// inside this module and strips the rest, so "net/http" could never
		// appear in it - a guard reading that list would pass without ever
		// reaching the thing it claims to watch, which is the trap this
		// project has written down and hit before.
		for _, imp := range rawImports(t, p) {
			if isNetworkPackage(imp) {
				t.Errorf("%s imports %q - the toolkit may carry its own, ours does not", p.rel, imp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no window package was examined, so this guard would pass on an empty tree")
	}
	t.Logf("%d window package(s) carry no network import of their own", checked)
}
