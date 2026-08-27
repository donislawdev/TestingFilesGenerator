package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every fake host in this package comes from newFakeHost, so nothing a guard
// starts can outlive the guard that started it.
//
// O124, and it is the nastiest defect this package has had. The toolkit's test
// driver runs fyne.Do on the CALLING goroutine and says so in its own source -
// "Tests all run on a single (but potentially different per-test) thread" - so
// a worker still going when a test ends shapes text inside the next one. It
// panicked in the font shaper, in five consecutive runs, in four different
// guards, and none of the four touched a preview. The name Go prints in a
// panic is the test that happened to be running, which means the cost of
// finding it is paid by somebody with no reason to suspect their own guard.
//
// Three screen helpers already joined on cleanup and that was the fix. What
// was missing was anything making it true for a screen built by hand, and 57
// of the 61 hosts here were built by hand. The promise now lives in the
// constructor, and this is what keeps the sixty-second host - written by
// somebody who never read that comment - going through it.
//
// This reads the source, which is not how this package likes to work, and the
// note that recorded O124 predicted a guard for it would be "reading the source
// and brittle". Two things answer that. It is not brittle: it looks for one
// literal, and a literal is what somebody would actually write. And the
// alternative was measured and does not work - a goroutine count in TestMain
// sees nothing, because the worker ends within a fraction of a second and the
// damage happens BETWEEN tests, which is precisely where Go offers no hook.
func TestEveryFakeHostComesFromTheConstructorThatJoinsIt(t *testing.T) {
	// Assembled rather than written, and that is not cleverness for its own
	// sake: written whole, this line is itself a host built by hand as far as
	// the search below is concerned, and the guard reported itself on its first
	// run. The alternative - skipping this file - would leave the one file
	// nobody would think to check unchecked.
	literal := "&fake" + "Host{"

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var offenders []string
	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if !strings.Contains(line, literal) {
				continue
			}
			seen++
			// The constructor itself is the one place that writes it, and it
			// says so in a comment of its own. Recognised by what the line
			// does rather than by which file it is in, so moving it does not
			// quietly switch this guard off.
			if strings.HasPrefix(strings.TrimSpace(line), "h := "+literal+"}") {
				continue
			}
			offenders = append(offenders,
				strings.TrimSpace(line)+"  ("+e.Name()+":"+itoa(i+1)+")")
		}
	}

	// The count is asserted as well, because a guard that finds nothing to
	// check reads exactly like a guard that passes. If the literal ever stops
	// appearing at all, this has stopped measuring anything.
	if seen == 0 {
		t.Fatalf("no %s appears anywhere in this package, not even in the constructor - "+
			"this guard is looking for something that no longer exists", literal)
	}

	for _, o := range offenders {
		t.Errorf("a fake host is built by hand: %s\n"+
			"Use newFakeHost(t). It joins whatever the screen starts when the test ends, and "+
			"without that a worker still going shapes text inside the NEXT test - the panic "+
			"then names a guard that had nothing to do with it.", o)
	}
}
