package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A listing command refuses a name it cannot answer about, rather than
// answering about a different one.
//
// Measured on 2026-09-09, on the built binary: "tfg formats png svg" printed
// the card for png, said nothing at all about svg and ended with zero, and
// "tfg preset list size-boundaries" printed the whole list and ended with zero
// though that operation takes no name. Both are silence about an argument
// somebody typed - untouchable rule 6 read from the other end - and both are
// worse in a script than a refusal, because a confident answer about the wrong
// thing is indistinguishable from a right one. O196.
//
// The refusal has to NAME the word it could not use. An exit code alone is a
// weak thing to assert on: a command that refused everything with the same code
// would satisfy it, and so would one that refused for an unrelated reason.
//
// Two commands answered this correctly already and are here as the other half
// of the comparison, because the defect is not that a check is missing
// somewhere - it is that four commands answered one question two ways.
func TestAListingCommandRefusesANameItCannotAnswerAbout(t *testing.T) {
	refusals := []struct {
		what  string
		args  []string
		extra string
	}{
		{"formats asked about two", []string{"formats", "png", "svg"}, "svg"},
		{"damage asked about two", []string{"damage", "zero-head", "other"}, "other"},
		{"preset list given a name", []string{"preset", "list", "size-boundaries"}, "size-boundaries"},
		{"preset show given two", []string{"preset", "show", "size-boundaries", "extra"}, "extra"},
		{"verify given two", []string{"verify", "a.json", "b.json"}, ""},
	}

	for _, c := range refusals {
		t.Run(c.what, func(t *testing.T) {
			code, stdout, errOut := run(t, c.args...)
			if code != cli.ExitUsage {
				t.Errorf("ended with %d, expected %d - a name it cannot use is a mistake "+
					"in what was typed\nstdout: %s\nstderr: %s", code, cli.ExitUsage, stdout, errOut)
			}
			if stdout != "" {
				t.Errorf("a refused run wrote to stdout, so a pipe receives half an answer: %q", stdout)
			}
			// Named rather than merely refused. Without this the guard passes
			// for a command that says "usage" and leaves the reader to work out
			// which of the words they typed was the problem.
			if c.extra != "" && !strings.Contains(errOut, c.extra) {
				t.Errorf("the refusal does not name %q, the word it could not use: %s", c.extra, errOut)
			}
		})
	}

	// The control, and it is not decoration: every assertion above is satisfied
	// by a build that refuses these commands outright. This half says the
	// refusal is about the extra name and nothing else.
	answers := [][]string{
		{"formats"},
		{"formats", "png"},
		{"damage"},
		{"damage", "zero-head"},
		{"preset", "list"},
	}
	for _, args := range answers {
		t.Run("still answers "+strings.Join(args, " "), func(t *testing.T) {
			code, stdout, errOut := run(t, args...)
			if code != cli.ExitOK {
				t.Errorf("ended with %d, expected %d: %s", code, cli.ExitOK, errOut)
			}
			if stdout == "" {
				t.Error("answered with nothing on stdout")
			}
		})
	}
}
