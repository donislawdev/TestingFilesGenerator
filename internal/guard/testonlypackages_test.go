package guard

import (
	"strings"
	"testing"
)

// Two packages here exist for the tests and must not reach either binary.
//
// internal/oracle shells out to python and finds its script through
// runtime.Caller, so it reads a path from the source tree of the machine that
// COMPILED it. In a released binary that path does not exist, which means the
// package cannot work there by construction - and a package that cannot work
// in the thing shipped has no business being linked into it. internal/site
// renders the website from the registry and is asked about only by a guard.
//
// Review item N8 proposed a build tag or a move. Measured first, on
// 2026-08-27: neither package is linked into either binary today, so the tag
// would defend something already true. What was missing is anything that keeps
// it true - an ordinary looking import from cli or window would pull the whole
// of oracle, python-finding and all, into what people download. This is that.
//
// It asks the compiler rather than reading imports, for the same reason the
// registration guard beside it does: what ends up in the binary is decided by
// build constraints, and a test that imports the code sees its own build
// instead. That guard's helper is reused here, C support and all, since with
// CGO off the window binary lists as the stub that has no window.
func TestThePackagesOnlyTestsUseStayOutOfBothBinaries(t *testing.T) {
	testOnly := []string{
		"github.com/donislawdev/TestingFilesGenerator/internal/oracle",
		"github.com/donislawdev/TestingFilesGenerator/internal/site",
	}

	for _, target := range []string{"../../cmd/tfg", "../../cmd/tfg-gui"} {
		linked := linkedWithCGO(t, target)

		for _, unwanted := range testOnly {
			for _, p := range linked {
				if p != unwanted && !strings.HasPrefix(p, unwanted+"/") {
					continue
				}
				t.Errorf("%s links %s, which exists for the tests.\n"+
					"oracle finds its python script through runtime.Caller, so it reads a path "+
					"from the machine that compiled the binary and cannot work in a released one. "+
					"Whatever now imports it has to stop, or the package has to move out of internal.",
					target, p)
			}
		}
	}
}
