package guard

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Asking for help is not a mistake.
//
// This guard exists because of what its absence hid. Every subcommand answered
// --help with code 2, which is the ending that means the caller typed
// something wrong, and put the text on the channel meant for complaints. The
// top level help said "Run tfg <command> --help for the flags of one command",
// so the tool instructed people to run something it then reported as an error.
//
// Nothing caught it, and the reason is worth more than the defect.
// TestEveryEndingHasACodeFromTheTable asks whether an ending carries a code
// from the frozen table. Code 2 is in the table, so it passed. The guard
// watched whether an ending is legal, never whether it is right - the same
// shape as a regression surface row claiming it was proven by mutation with no
// mutation behind it. Measured 2026-08-03: the word help appeared in no guard
// in this package.
//
// See docs/UX.md, rule UX2 and section 3.3.

// commandsTakingHelp is every verb a person can type, including the empty one.
// Adding a command without adding it here leaves that command unwatched, which
// is exactly how the defect above survived.
var commandsTakingHelp = [][]string{
	{"generate"},
	{"validate"},
	{"verify"},
	{"cleanup"},
	{"formats"},
	{"recipe", "fmt"},
}

func TestAskingForHelpIsNotAMistake(t *testing.T) {
	for _, cmd := range commandsTakingHelp {
		for _, flag := range []string{"--help", "-h"} {
			args := append(append([]string{}, cmd...), flag)
			name := strings.Join(args, " ")

			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), args, &out, &errOut)

			if code != cli.ExitOK {
				t.Errorf("%q ended with %d, expected %d - asking for help is not a mistake",
					name, code, cli.ExitOK)
			}
			if out.Len() == 0 {
				t.Errorf("%q put nothing on stdout - the answer has to arrive where "+
					"a person can pipe it into less", name)
			}
			if errOut.Len() != 0 {
				t.Errorf("%q also wrote %d bytes to stderr, so the answer is split "+
					"across two channels: %q", name, errOut.Len(), errOut.String())
			}
		}
	}
}

// The top level behaves the same way, and bare "tfg" deliberately does not.
//
// A person who types tfg on its own gets the help, because there is nothing
// else useful to show. It still ends with the usage code, because in a script
// a bare invocation is a mistake and has to fail the build. clig.dev would
// have this end with 0. We differ on purpose - docs/CLI.md section 1 says the
// command line is the interface CI drives, and that decides the tie.
func TestTheTopLevelTellsAskingApartFromTyping(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		var out, errOut bytes.Buffer
		if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
			t.Errorf("tfg %v ended with %d, expected %d", args, code, cli.ExitOK)
		}
		if out.Len() == 0 || errOut.Len() != 0 {
			t.Errorf("tfg %v put %d bytes on stdout and %d on stderr, expected all of it on stdout",
				args, out.Len(), errOut.Len())
		}
	}

	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), nil, &out, &errOut)
	if code != cli.ExitUsage {
		t.Errorf("bare tfg ended with %d, expected %d - in a script it is a mistake", code, cli.ExitUsage)
	}
	if out.Len() != 0 {
		t.Errorf("bare tfg wrote %d bytes to stdout, and a failed run writes nothing there", out.Len())
	}
	if errOut.Len() == 0 {
		t.Error("bare tfg said nothing at all, so the person has no idea what to type next")
	}
}

// A mistyped command is a mistake and keeps the usage code, so the two cases
// stay told apart. Without this, moving help onto stdout could be "fixed" by
// moving everything onto stdout.
func TestAMistypedCommandIsStillAMistake(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"lst"}, &out, &errOut)

	if code != cli.ExitUsage {
		t.Errorf("a mistyped command ended with %d, expected %d", code, cli.ExitUsage)
	}
	if out.Len() != 0 {
		t.Errorf("a mistyped command wrote %d bytes to stdout, and a failed run writes nothing there", out.Len())
	}
	// The first line only. The usage text printed underneath lists the
	// commands, so it carries the word whatever the message says - asserting
	// on the whole buffer passed even when the message called it an option.
	// Found by the mutation runner, which is what it is for.
	first, _, _ := strings.Cut(errOut.String(), "\n")
	if !strings.Contains(first, "command") {
		t.Errorf("a mistyped command was not called a command: %q", first)
	}
}
