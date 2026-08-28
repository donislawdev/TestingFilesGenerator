// Package cli parses flags, keeps data and log on separate channels and maps
// every ending onto an exit code.
//
// The command line is not an advanced mode. It is the interface CI drives, so
// an ending CI cannot tell apart is a defect, not a detail.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// Exit codes are a frozen contract. Changing what one means is a breaking
// change and needs a major version bump. See docs/CLI.md.
const (
	ExitOK      = 0
	ExitRuntime = 1
	ExitUsage   = 2
	ExitRecipe  = 3
	ExitFormat  = 4
	ExitIO      = 5
	ExitSpace   = 6
	ExitVerify  = 7
	ExitPartial = 8

	// Ctrl+C. Named here rather than written as a bare 130 at the point of
	// use, because it is in the same frozen table as the rest.
	ExitInterrupted = 130

	// Stopped by a signal rather than by a person - a CI timeout is the case
	// the table names. It is a separate ending from Ctrl+C on purpose: one
	// means somebody cancelled and the other means the job ran out of time,
	// and those call for different answers.
	ExitTerminated = 143
)

// ExitForSignal maps a signal onto the ending in the frozen table.
//
// It exists as its own function because the alternative is deciding this
// inside main, where nothing tests it. signal.NotifyContext does not say which
// signal arrived, which is how every stop ended up reported as Ctrl+C.
func ExitForSignal(s os.Signal) int {
	if s == syscall.SIGTERM {
		return ExitTerminated
	}
	return ExitInterrupted
}

// Run is the entry point of the command line.
//
// Data goes to out and everything else goes to errOut. A failed run puts
// nothing on out, so a consumer of a pipe never receives half an answer and
// has to guess whether that was all of it.
func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		usage(errOut)
		return ExitUsage
	}

	switch args[0] {
	case "generate":
		return generate(ctx, args[1:], out, errOut)
	case "validate":
		return validate(args[1:], out, errOut)
	case "verify":
		return verify(ctx, args[1:], out, errOut)
	case "cleanup":
		return cleanup(ctx, args[1:], out, errOut)
	case "recipe":
		return recipeCmd(args[1:], out, errOut)
	case "preset":
		return presetCmd(args[1:], out, errOut)
	case "formats":
		return formats(args[1:], out, errOut)
	case "--version", "version":
		fmt.Fprintln(out, version.Version)
		return ExitOK
	case "--license", "--licence", "license", "licence":
		// Both spellings. The tool writes British English and half the people
		// who reach for this will type the American one, and being right about
		// spelling at the cost of answering is not a trade worth making.
		//
		// An answer, so it goes to out and ends with zero, the same as version.
		fmt.Fprint(out, version.LicenceNotice)
		// And then what this particular binary actually carries. The notice
		// above points at a file, which is no help to somebody holding only
		// the binary - a download of the window is one file and the notices
		// are not in it. The list is read out of the build's own record, so
		// it describes the binary being asked rather than the source tree it
		// came from: the command line answers with two libraries, the window
		// with twenty-seven and the fonts they bring.
		printCarried(out, legal.CarriedHere())
		return ExitOK
	case "--help", "-h", "help":
		// Asking is not a mistake, so the answer goes where answers go and
		// "tfg --help | less" works.
		usage(out)
		return ExitOK
	default:
		fmt.Fprintf(errOut, "tfg: unknown command %q.\n\n", args[0])
		usage(errOut)
		return ExitUsage
	}
}

// helpRequested reports whether these arguments explicitly ask for help.
//
// The question has to be settled before the flag package parses. By the time
// it hands back ErrHelp it has already printed the text to the stream the set
// was pointed at, and it reports the parse as a failure - so an answer arrives
// on the channel meant for complaints, with the ending that means the caller
// typed something wrong.
//
// Scanning stops at "--", after which nothing is a flag any more. A value that
// happens to read as "--help" is therefore taken as a request for help, which
// is the harmless way to be wrong about it.
func helpRequested(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

func usage(w io.Writer) {
	fmt.Fprint(w, `tfg - generate test files and know how the system under test should react.

Commands:
  generate    produce files, from a recipe or from flags
  validate    check a recipe and write nothing
  verify      check a directory against a manifest
  cleanup     remove the files a manifest lists
  recipe fmt  print a recipe in its settled shape
  preset      build a set of files from a named test question
  formats     list the formats this build supports
  version     print the tool version
  license     print the licence and what it means for generated files

Run "tfg <command> --help" for the flags of one command.
`)
}

// The licence notice this command prints lives in internal/version, on the
// bottom layer, because the window shows the same text and the two surfaces
// cannot import each other. See version.LicenceNotice.

// defaultManifestName mirrors the engine, which has to know the name to keep
// a run from writing over an earlier one's record.
const defaultManifestName = engine.DefaultManifestName

// splitLeadingPath takes the file a command works on off the front of the
// arguments, leaving the rest for flag parsing.
//
// It has to be first. A path recognised anywhere in the list could not be told
// apart from the value of a flag, so "--seed 5" would turn 5 into a file name.
//
// Every command that takes a file uses this, so "tfg verify m.json --against
// x" and "tfg generate r.yaml --seed 9" read the same way round. The flag
// package on its own stops at the first non flag argument, which turns the
// documented form of a command into a usage error.
func splitLeadingPath(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

// onePath resolves the single file a command works on, accepting it either
// before or after the flags.
//
// Both forms are in circulation. docs/CLI.md writes "tfg verify manifest.json
// --against dir" and a person who has just typed "tfg recipe fmt --check f"
// expects the other order to work too. Turning one of them into a usage error
// is a papercut with nothing on the other side of it.
//
// Anything else - no path, or two - is a real mistake and says so.
func onePath(leading string, fs *flag.FlagSet) (string, bool) {
	switch {
	case leading != "" && fs.NArg() == 0:
		return leading, true
	case leading == "" && fs.NArg() == 1:
		return fs.Arg(0), true
	}
	return "", false
}

// flagsGiven is the set of flags the user actually wrote.
//
// This is the whole of the precedence rule. Reading the values back cannot
// tell "not given" from "given the same value as the default", and that
// difference decides whether the recipe or the flag wins.
func flagsGiven(fs *flag.FlagSet) map[string]bool {
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	return given
}

// describingFlagsGiven lists flags that describe one target, which is
// meaningless next to a recipe that may hold many.
func describingFlagsGiven(given map[string]bool) []string {
	var bad []string
	for _, name := range []string{"format", "size", "size-range", "boundary", "count", "name", "id", "set", "expected", "expected-reason"} {
		if given[name] {
			bad = append(bad, "--"+name)
		}
	}
	return bad
}

// writeJSON renders a machine readable report and says what the command should
// end with.
//
// The exit code comes in and goes out again, because a report that could not be
// written whole is only news when there was no other news. A command that
// already failed has a better answer than "the pipe broke", and replacing it
// would take away the reason it failed.
//
// The error was dropped here until 2026-08-25, so "tfg verify --json | head"
// ended with code 0 and half a document. Every machine report of this tool goes
// through this function, which is why it was worth one place rather than
// thirteen - and formats.go had been checking it all along, so the two ways out
// of the same tool behaved differently.
func writeJSON(w, errOut io.Writer, v any, code int) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(errOut, "tfg: the report could not be written whole: %s\n", describeError(err))
		if code == ExitOK {
			return ExitIO
		}
	}
	return code
}

// propertyFlag collects repeated --set key=value pairs.
type propertyFlag map[string]string

func (p propertyFlag) String() string { return "" }

func (p propertyFlag) Set(v string) error {
	key, value, found := strings.Cut(v, "=")
	if !found || key == "" {
		return fmt.Errorf("expected key=value, got %q", v)
	}
	if _, exists := p[key]; exists {
		// Setting the same property twice is a mistake worth naming. One of
		// the two values would be lost, and nobody would know which.
		return fmt.Errorf("%s is set more than once", key)
	}
	p[key] = value
	return nil
}

// args2 rebuilds the command as it would have to be typed to run again.
//
// It goes into the manifest, where its whole job is to be re-runnable, and it
// was assembled by joining the arguments with spaces. An argument holding a
// space then arrived as two - "--name my file.txt" reads as a name of "my" and
// a stray word - so the recorded command produced a different run, or none.
//
// Quoted only where it is needed, so the common line stays readable. Single
// quotes are avoided because the shells this tool is aimed at disagree about
// them, and double quotes with escaping work in all of them.
func args2(args []string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, "generate")
	for _, a := range args {
		out = append(out, quoteArg(a))
	}
	return out
}

func quoteArg(a string) string {
	if a != "" && !strings.ContainsAny(a, " \t\"\\") {
		return a
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(a) + `"`
}
