package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Asking what the licence is is a question, so it is answered on the channel
// answers go to and it ends with zero. The same rule that put the help text on
// standard output, reached by a different door.
//
// The middle of the notice is the reason the command exists. Somebody deciding
// whether to put a generator into a closed source product has to know whether
// its licence reaches the files it produces. It does not - the output of a
// program is not a derived work of it - but that is not obvious from the name
// GPL, and being wrong either way costs them a tool or a lawyer. README says
// so, and a person who ran the binary has not necessarily read README.
func TestAskingForTheLicenceIsAnsweredAndEndsWithZero(t *testing.T) {
	// Both spellings, and both as a command and as a flag. A person who types
	// the American spelling gets the licence, not a usage error.
	for _, arg := range []string{"license", "licence", "--license", "--licence"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, errOut := run(t, arg)
			if code != cli.ExitOK {
				t.Errorf("exit %d, expected %d - asking is not a mistake:\n%s", code, cli.ExitOK, errOut)
			}
			if errOut != "" {
				t.Errorf("the answer, or part of it, went to the complaint channel:\n%s", errOut)
			}
			if !strings.Contains(stdout, "General Public License") {
				t.Errorf("the notice does not name the licence:\n%s", stdout)
			}
			// A licence with nobody behind it is a licence nobody granted.
			if !strings.Contains(stdout, "Copyright (C)") || !strings.Contains(stdout, "DonislawDev") {
				t.Errorf("the notice does not say who holds the copyright:\n%s", stdout)
			}
			// Naming the licence is the easy half. Without this the notice
			// could shrink to one line and the command would stop being worth
			// running.
			if !strings.Contains(stdout, "generate are yours") {
				t.Errorf("the notice does not say the licence leaves generated files alone:\n%s", stdout)
			}
			if !strings.Contains(stdout, "THIRD-PARTY-NOTICES.md") {
				t.Errorf("the notice does not point at the notices for the code compiled in beside ours:\n%s", stdout)
			}
		})
	}
}

// And the file it points at has to be there. A notice naming a document that
// does not ship is worse than no notice, because it reads as an answer.
func TestTheFileTheLicenceNoticePointsAtExists(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"LICENSE", "THIRD-PARTY-NOTICES.md"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("the licence notice points at %s and it is not in the repository: %v", name, err)
			continue
		}
		if len(body) == 0 {
			t.Errorf("%s is there and empty, so the notice points at nothing", name)
		}
	}
}
