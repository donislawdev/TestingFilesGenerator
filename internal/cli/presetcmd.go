// Part of package cli. See cli.go.
//
// The tfg preset command lives here and the machinery behind
// generate --preset lives in preset.go beside it. One file held both and
// grew past the file ceiling on 2026-09-09, and the split follows what the
// parts DO rather than where the line count happened to fall: everything
// here answers somebody who typed tfg preset, and everything there turns a
// named question into targets for a run.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

func presetCmd(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "list":
			return presetList(args[1:], out, errOut)
		case "show":
			return presetShow(ctx, args[1:], out, errOut)
		case "eject":
			return presetEject(args[1:], out, errOut)
		}
	}

	// Asking about the command itself, before any operation is named.
	if helpRequested(args) {
		presetUsage(out)
		return ExitOK
	}
	if len(args) == 0 {
		fmt.Fprintln(errOut, "tfg: preset takes one operation: list, show or eject. Example: tfg preset list")
	} else {
		fmt.Fprintf(errOut, "tfg: preset has no operation called %q. It takes list, show or eject.\n", args[0])
	}
	presetUsage(errOut)
	return ExitUsage
}

func presetUsage(w io.Writer) {
	fmt.Fprint(w, `tfg preset - build a set of files from a named test question.

Usage:
  tfg preset list                       what this build offers
  tfg preset show <id>                  what it takes and what it would produce
  tfg preset eject <id> > my.yaml       the recipe it stands for, to edit

A preset is a recipe with a name. Ejecting one gives back an ordinary recipe
file, so nothing here is a closed box.

Run "tfg generate --preset <id>" to produce the files.
`)
}

// presetFlagSet builds the flag set of an operation taking one preset id.
//
// The id has to be read before parsing, because the parameters of the preset
// are flags and there is no way to register them until it is known which they
// are.
// asJSON is filled in for the operations that have a machine readable form and
// nil for the one that does not - a recipe is already machine readable, and a
// second encoding of it would be a second thing to keep in step.
func presetFlagSet(name string, args []string, out, errOut io.Writer, usage func(io.Writer), asJSON *bool) (
	*preset.Expansion, int) {

	fs := flag.NewFlagSet("preset "+name, flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() { usage(errOut) }
	if helpRequested(args) {
		usage(out)
		return nil, ExitOK
	}
	// Registered before the parameters, so a preset declaring one called json
	// is caught by the collision check rather than by the flag package panicking.
	if asJSON != nil {
		fs.BoolVar(asJSON, "json", false, "write the answer as JSON to standard output")
	}

	id, rest := splitLeadingPath(args)
	if id == "" {
		fmt.Fprintf(errOut, "tfg: preset %s takes the id of one preset. Run \"tfg preset list\" to see them.\n", name)
		return nil, ExitUsage
	}
	p, err := preset.Get(id)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}
	if clash := clashingParameter(fs, p); clash != "" {
		fmt.Fprintf(errOut, "tfg: the preset %s declares a parameter called %q and that is already a flag of this command. This is a fault in the build rather than in what you typed, and there is nothing you can do about it from here.\n", p.ID, clash)
		return nil, ExitRuntime
	}
	registerPresetFlags(fs, p)

	if err := fs.Parse(rest); err != nil {
		return nil, ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "tfg: preset %s takes one preset id and %q came after it. Give the parameters as flags, for example --limit 10mb.\n", name, fs.Arg(0))
		return nil, ExitUsage
	}

	expanded, err := preset.Expand(id, givenPresetArgs(fs, p))
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}
	return expanded, ExitOK
}

func presetList(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("preset list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the list as JSON to standard output")
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg preset list - the test questions this build can answer.

Usage:
  tfg preset list
  tfg preset list --json

Flags:
`)
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(errOut)
	}
	fs.Usage = func() { usage(errOut) }
	if helpRequested(args) {
		usage(out)
		return ExitOK
	}
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	// This operation takes no name, and until 2026-09-09 anything written
	// after it was dropped without a word: tfg preset list size-boundaries
	// printed the whole list and ended with zero, so somebody reaching for
	// show got list and no sign of it. Found while measuring the same
	// silence in tfg formats, O196.
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(errOut, "tfg: preset list takes no name and %q came after it. "+
			"Run \"tfg preset list\" for all of them, or \"tfg preset show %s\" for one.\n",
			extra[0], extra[0])
		return ExitUsage
	}

	all := preset.All()
	if *asJSON {
		list := make([]presetEntry, 0, len(all))
		for _, p := range all {
			list = append(list, presetEntryFor(p))
		}
		return renderJSON(list, out, errOut)
	}

	// An empty build says so rather than printing a heading over nothing.
	if len(all) == 0 {
		fmt.Fprint(out, "This build registers no presets.\n")
		return ExitOK
	}
	fmt.Fprintf(out, "%-18s %s\n", "PRESET", "QUESTION IT ANSWERS")
	for _, p := range all {
		fmt.Fprintf(out, "%-18s %s\n", p.ID, p.Question)
	}
	fmt.Fprint(out, "\nRun \"tfg preset show <id>\" for what one takes and what it would produce.\n")
	return ExitOK
}

func presetShow(ctx context.Context, args []string, out, errOut io.Writer) int {
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg preset show - what a preset takes and what it would produce.

The budget is counted from the plan, at the parameters you gave, so it is the
number of files and bytes this run would really write.

Usage:
  tfg preset show size-boundaries
  tfg preset show size-boundaries --limit 20mb --format png
  tfg preset show size-boundaries --json
`)
	}
	var asJSON bool
	expanded, code := presetFlagSet("show", args, out, errOut, usage, &asJSON)
	if expanded == nil {
		return code
	}

	b, err := budgetOf(ctx, expanded)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	if asJSON {
		entry := presetEntryFor(expanded.Preset)
		entry.Budget = &b
		entry.Defaulted = expanded.Defaulted
		entry.Notes = expanded.Notes()
		return renderJSON(entry, out, errOut)
	}
	describePreset(expanded, b, out)
	return ExitOK
}

func describePreset(e *preset.Expansion, b budget, out io.Writer) {
	p := e.Preset
	fmt.Fprintf(out, "%s - %s\n%s\n", p.ID, p.Title, p.Question)

	if len(p.Parameters) > 0 {
		fmt.Fprint(out, "\nparameters:\n")
		for _, param := range p.Parameters {
			fmt.Fprintf(out, "  --%-12s %s\n", param.Name, param.Allowed())
			if param.Detail != "" {
				fmt.Fprintf(out, "  %-14s %s\n", "", param.Detail)
			}
		}
	}
	for _, name := range p.Reads {
		fmt.Fprintf(out, "  --%-12s the global flag, this preset gives it a default\n", name)
	}

	fmt.Fprintf(out, "\nbudget at these values:\n  %s, %s, %s total, format %s\n",
		core.Count(b.Targets, "target", "targets"), core.Count(b.Files, "file", "files"),
		core.ExactBytes(b.Bytes), strings.Join(b.Formats, ", "))
	for _, note := range e.Notes() {
		fmt.Fprintf(out, "\nnote: %s\n", note)
	}

	if len(p.Catches) > 0 {
		fmt.Fprint(out, "\nwhat it typically catches:\n")
		for _, c := range p.Catches {
			fmt.Fprintf(out, "  - %s\n", c)
		}
	}
	fmt.Fprintf(out, "\nRun \"tfg preset eject %s\" for the recipe, or \"tfg generate --preset %s\" to produce the files.\n",
		p.ID, p.ID)
}

func presetEject(args []string, out, errOut io.Writer) int {
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg preset eject - the recipe a preset stands for.

Prints an ordinary recipe file. Edit it, commit it, run it with tfg generate -
from here on it is yours and nothing about it is special.

The recipe goes to standard output and everything else to standard error, so
"tfg preset eject size-boundaries > my.yaml" gives a clean file.

Usage:
  tfg preset eject size-boundaries > my.yaml
  tfg preset eject size-boundaries --limit 20mb --format png > my.yaml
`)
	}
	expanded, code := presetFlagSet("eject", args, out, errOut, usage, nil)
	if expanded == nil {
		return code
	}

	// The note goes to the error channel. The recipe is the data here, and a
	// sentence about a number we chose has no business inside a file somebody
	// is about to commit.
	for _, note := range expanded.Notes() {
		fmt.Fprintf(errOut, "note: %s\n", note)
	}
	if _, err := out.Write(expanded.Source); err != nil {
		fmt.Fprintf(errOut, "tfg: cannot write the recipe: %s\n", describeError(err))
		return ExitIO
	}
	return ExitOK
}
