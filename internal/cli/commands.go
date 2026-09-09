// Part of package cli. See cli.go.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// command is one verb of the command line: what the dispatch matches, what the
// help prints about it, and what runs.
//
// One declaration rather than two lists, since 2026-09-09. The help used to be
// a raw string beside a switch, and nothing compared them - measured that day,
// the dispatch offered eight verbs taking arguments while the help printed
// ten, and the four differences were all legitimate, so no naive comparison
// could have told a real gap from those. tfg damage had already reached the
// program and the website while the site's own list was a hand copy of this
// help. O201.
//
// The parts are separate because they are genuinely different questions:
//
//   - Verb is the one word the dispatch matches and the help prints.
//   - Aliases reach the same place and are NOT printed. Four spellings of
//     licence exist because half the people who want it type the American one,
//     and being right about spelling at the cost of answering is not a trade
//     worth making. Printing all four would make the help a spelling lesson.
//   - Shown replaces Verb in the help where the two differ. Only recipe does:
//     the dispatch branches on "recipe", and "recipe fmt" is what a person
//     types, because recipe on its own does nothing.
//   - Summary empty means reachable but not listed, which is help itself. A
//     reader looking at the help does not need to be told it exists.
type command struct {
	Verb    string
	Aliases []string
	Shown   string
	Summary string
	Run     func(ctx context.Context, args []string, out, errOut io.Writer) int
}

// name is what the help prints for this command.
func (c command) name() string {
	if c.Shown != "" {
		return c.Shown
	}
	return c.Verb
}

// matches says whether this is the command somebody asked for.
func (c command) matches(word string) bool {
	if word == c.Verb {
		return true
	}
	for _, alias := range c.Aliases {
		if word == alias {
			return true
		}
	}
	return false
}

// commands is every verb, in the order the help lists them.
//
// A function rather than a package variable, because help is in here and its
// body renders this list - which as a variable would be an initialisation
// cycle. Rebuilding eleven entries per invocation costs nothing next to the
// process start that precedes it.
//
// Three of these take no context and are wrapped rather than changed. The
// wrapper is where that shows, instead of five signatures being widened for a
// table.
func commands() []command {
	return []command{
		{Verb: "generate", Summary: "produce files, from a recipe or from flags", Run: generate},
		{Verb: "validate", Summary: "check a recipe and write nothing", Run: validate},
		{Verb: "verify", Summary: "check a directory against a manifest", Run: verify},
		{Verb: "cleanup", Summary: "remove the files a manifest lists", Run: cleanup},
		{
			Verb: "recipe", Shown: "recipe fmt",
			Summary: "print a recipe in its settled shape",
			Run: func(_ context.Context, args []string, out, errOut io.Writer) int {
				return recipeCmd(args, out, errOut)
			},
		},
		{Verb: "preset", Summary: "build a set of files from a named test question", Run: presetCmd},
		{
			Verb: "formats", Summary: "list the formats this build supports",
			Run: func(_ context.Context, args []string, out, errOut io.Writer) int {
				return formats(args, out, errOut)
			},
		},
		{
			Verb: "damage", Summary: "list the ways this build can break a file on purpose",
			Run: func(_ context.Context, args []string, out, errOut io.Writer) int {
				return damageCmd(args, out, errOut)
			},
		},
		{
			Verb: "version", Aliases: []string{"--version"},
			Summary: "print the tool version",
			Run: func(_ context.Context, _ []string, out, _ io.Writer) int {
				fmt.Fprintln(out, version.Version)
				return ExitOK
			},
		},
		{
			Verb:    "license",
			Aliases: []string{"--license", "--licence", "licence"},
			Summary: "print the licence and what it means for generated files",
			Run:     licenceCmd,
		},
		{
			// Listed nowhere, reachable three ways. Somebody reading the help
			// has already found it.
			Verb: "help", Aliases: []string{"--help", "-h"},
			Run: func(_ context.Context, _ []string, out, _ io.Writer) int {
				// Asking is not a mistake, so the answer goes where answers go
				// and "tfg --help | less" works.
				usage(out)
				return ExitOK
			},
		},
	}
}

// Command is what one verb is called and what the help says about it.
//
// Exported without the function that runs it, because what a caller outside
// this package can want is the FACTS - a guard checking that the website
// repeats them, or that every verb is watched. Handing out the behaviour as
// well would let something outside the command line invoke half of it.
type Command struct {
	// Name is how the help prints it, which is "recipe fmt" where the verb is
	// "recipe". Verb is the word the dispatch matches.
	Name string
	Verb string
	// Summary is empty for a command the help does not list.
	Summary string
}

// Commands is every verb this build answers to, in the order the help lists
// them.
func Commands() []Command {
	all := commands()
	out := make([]Command, 0, len(all))
	for _, c := range all {
		out = append(out, Command{Name: c.name(), Verb: c.Verb, Summary: c.Summary})
	}
	return out
}

// licenceCmd prints the licence and then what this particular binary carries.
//
// The notice points at a file, which is no help to somebody holding only the
// binary - a download of the window is one file and the notices are not in it.
// The list is read out of the build's own record, so it describes the binary
// being asked rather than the source tree it came from: the command line
// answers with two libraries, the window with twenty-seven and the fonts they
// bring.
//
// An answer, so it goes to out and ends with zero, the same as version.
func licenceCmd(_ context.Context, _ []string, out, _ io.Writer) int {
	fmt.Fprint(out, version.LicenceNotice)
	printCarried(out, legal.CarriedHere())
	return ExitOK
}

// usage prints what this tool is and what it can be asked to do.
//
// The command block is rendered from the declaration above rather than written
// out here, so a verb added tomorrow appears without anybody remembering to
// add it. The column is worked out from the longest name for the same reason:
// a number here would be a second copy of "the longest is recipe fmt" and
// would go stale the day something longer arrives.
func usage(w io.Writer) {
	fmt.Fprint(w, "tfg - generate test files and know how the system under test should react.\n\nCommands:\n")
	widest := 0
	for _, c := range commands() {
		if c.Summary != "" && len(c.name()) > widest {
			widest = len(c.name())
		}
	}
	for _, c := range commands() {
		if c.Summary == "" {
			continue
		}
		pad := strings.Repeat(" ", widest-len(c.name())+2)
		fmt.Fprintf(w, "  %s%s%s\n", c.name(), pad, c.Summary)
	}
	fmt.Fprint(w, "\nRun \"tfg <command> --help\" for the flags of one command.\n")
}
