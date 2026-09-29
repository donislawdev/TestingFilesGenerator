// Part of package cli. See cli.go.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// toolEntry is what "tfg tool list --json" and "tfg tool show --json" return.
//
// The settings ride in the same propertyEntry a format, a preset and a damage
// use, because a tool setting IS a format.Property - a script that draws a
// field from one draws it from the others with no new code.
type toolEntry struct {
	ID       string           `json:"id"`
	Question string           `json:"question"`
	Detail   string           `json:"detail"`
	Inputs   []toolInputEntry `json:"inputs,omitempty"`
	Settings []propertyEntry  `json:"settings,omitempty"`
}

// toolInputEntry is one thing a tool works on.
type toolInputEntry struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// propertyEntriesFor is a list of declared settings as a script sees them.
//
// The fourth place that needs this, and the first written once for anybody -
// formats, presets and damages each build it inline (O261).
func propertyEntriesFor(declared []format.Property) []propertyEntry {
	out := make([]propertyEntry, 0, len(declared))
	for _, p := range declared {
		out = append(out, propertyEntry{
			Name: p.Name, Kind: string(p.Kind), Min: p.Min, Max: p.Max,
			Unit: p.Unit, Choices: p.Choices, Default: p.Default, Detail: p.Detail,
			Group: p.Group,
		})
	}
	return out
}

func toolEntryFor(d tool.Descriptor) toolEntry {
	inputs := make([]toolInputEntry, 0, len(d.Inputs))
	for _, in := range d.Inputs {
		inputs = append(inputs, toolInputEntry{Name: in.Name, Kind: string(in.Kind), Detail: in.Detail})
	}
	return toolEntry{
		ID: d.ID, Question: d.Question, Detail: d.Detail,
		Inputs: inputs, Settings: propertyEntriesFor(d.Settings),
	}
}

// toolCmd answers "tfg tool": what the tools are, what one takes, or the run
// of one.
//
// list and show are words of their own rather than "tfg tool" alone and "tfg
// tool <id>", which is how tfg damage answers - because here the id RUNS the
// tool, the way a verb does. tool.Register refuses a tool called list or show
// for the same reason.
func toolCmd(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "list":
			return toolList(args[1:], out, errOut)
		case "show":
			return toolShow(args[1:], out, errOut)
		}
		if !strings.HasPrefix(args[0], "-") {
			return toolRun(ctx, args[0], args[1:], out, errOut)
		}
	}
	if helpRequested(args) {
		toolUsage(out)
		return ExitOK
	}
	fmt.Fprintln(errOut, "tfg: tool takes one operation: list, show or the id of a tool. Example: tfg tool list")
	toolUsage(errOut)
	return ExitUsage
}

func toolUsage(w io.Writer) {
	fmt.Fprint(w, `tfg tool - small things to do with files you already have.

Usage:
  tfg tool list                    the tools this build has
  tfg tool show <id>               what one tool works on and takes
  tfg tool <id> <file> [flags]     run it

Every tool is also on the Tools tab of the window, with the same settings.
`)
}

func toolList(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("tool list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the list as JSON to standard output")
	fs.Usage = func() { toolUsage(errOut) }
	if helpRequested(args) {
		toolUsage(out)
		return ExitOK
	}
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(errOut, "tfg: tool list takes no names and %q came after it. Run \"tfg tool show %s\" for one tool.\n",
			fs.Arg(0), fs.Arg(0))
		return ExitUsage
	}
	all := tool.All()
	if *asJSON {
		entries := make([]toolEntry, 0, len(all))
		for _, d := range all {
			entries = append(entries, toolEntryFor(d))
		}
		return renderJSON(entries, out, errOut)
	}
	fmt.Fprintf(out, "%-12s %s\n", "TOOL", "QUESTION")
	for _, d := range all {
		fmt.Fprintf(out, "%-12s %s\n", d.ID, d.Question)
		fmt.Fprintf(out, "    %s\n", d.Detail)
	}
	fmt.Fprint(out, "\nRun \"tfg tool show <id>\" for what one tool takes.\n")
	return ExitOK
}

func toolShow(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("tool show", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the description as JSON to standard output")
	fs.Usage = func() { toolUsage(errOut) }
	if helpRequested(args) {
		toolUsage(out)
		return ExitOK
	}
	leading, rest := splitLeadingPath(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	wanted, ok := atMostOneName(leading, fs, errOut)
	if !ok {
		return ExitUsage
	}
	if wanted == "" {
		fmt.Fprintln(errOut, "tfg: tool show takes the id of one tool. Run \"tfg tool list\" to see them.")
		return ExitUsage
	}
	d, err := tool.Get(wanted)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}
	if *asJSON {
		return renderJSON(toolEntryFor(d), out, errOut)
	}
	describeOneTool(d, out)
	return ExitOK
}

// describeOneTool prints everything one tool declares, in the order somebody
// about to run it needs: what it answers, what to type, what each part means.
func describeOneTool(d tool.Descriptor, out io.Writer) {
	fmt.Fprintf(out, "%s - %s\n", d.ID, d.Question)
	fmt.Fprintf(out, "  %s\n\n", d.Detail)
	fmt.Fprintf(out, "Usage:\n  %s\n", toolExample(d))
	for _, in := range d.Inputs {
		fmt.Fprintf(out, "  %-12s %s\n", "<"+in.Name+">", in.Detail)
	}
	if len(d.Settings) == 0 {
		return
	}
	fmt.Fprint(out, "\nSettings:\n")
	for _, p := range d.Settings {
		fmt.Fprintf(out, "  %-12s %s\n", "--"+p.Name, p.Allowed())
		if p.Detail != "" {
			fmt.Fprintf(out, "  %-12s %s\n", "", p.Detail)
		}
	}
}

// toolExample is the command that runs a tool, built from what it declares
// rather than written out, so it cannot name an input that is gone.
func toolExample(d tool.Descriptor) string {
	parts := []string{"tfg tool", d.ID}
	for _, in := range d.Inputs {
		parts = append(parts, "<"+in.Name+">")
	}
	if len(d.Settings) > 0 {
		parts = append(parts, "[settings]")
	}
	return strings.Join(parts, " ") + " [--json]"
}

// toolRun runs one tool with what the command line gave it.
func toolRun(ctx context.Context, id string, args []string, out, errOut io.Writer) int {
	d, err := tool.Get(id)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}
	fs := flag.NewFlagSet("tool "+id, flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the result as JSON")
	fs.Usage = func() { describeOneTool(d, errOut) }
	if helpRequested(args) {
		describeOneTool(d, out)
		return ExitOK
	}
	for _, p := range d.Settings {
		if fs.Lookup(p.Name) != nil {
			fmt.Fprintf(errOut, "tfg: the tool %s declares a setting called %q and that is already a flag of this command. "+
				"This is a fault in the build rather than in what you typed.\n", d.ID, p.Name)
			return ExitRuntime
		}
		fs.String(p.Name, "", parameterUsage(p))
	}
	given, err := parseAround(fs, args)
	if err != nil {
		return ExitUsage
	}
	if len(given) > len(d.Inputs) {
		fmt.Fprintf(errOut, "tfg: %s works on %s and was given %d. Run it once for each, for example: %s\n",
			d.ID, inputCount(d), len(given), toolExample(d))
		return ExitUsage
	}

	req := tool.Request{Inputs: map[string]string{}, Values: map[string]string{}}
	for i, path := range given {
		req.Inputs[d.Inputs[i].Name] = path
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name != "json" {
			req.Values[f.Name] = f.Value.String()
		}
	})

	result, err := d.Start(ctx, req, nil)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}
	return renderToolResult(d, result, *asJSON, out, errOut)
}

// inputCount is how many things a tool works on, in words.
func inputCount(d tool.Descriptor) string {
	if len(d.Inputs) == 1 {
		return "one " + string(d.Inputs[0].Kind)
	}
	return fmt.Sprintf("%d inputs", len(d.Inputs))
}

// parseAround reads flags wherever they stand among the paths.
//
// The flag package stops at the first word that is not a flag, so "tfg tool
// checksum file --expected X" would leave the flag unread and take it for a
// second file. Both orders are in circulation - onePath says the same about
// the other commands - so this parses again after each run of paths. A "--"
// ends the flags for good, which is how a file whose name starts with a dash
// is reached.
func parseAround(fs *flag.FlagSet, args []string) ([]string, error) {
	var paths []string
	for {
		for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			paths = append(paths, args[0])
			args = args[1:]
		}
		if len(args) == 0 {
			return paths, nil
		}
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		used := len(args) - fs.NArg()
		if used > 0 && args[used-1] == "--" {
			return append(paths, fs.Args()...), nil
		}
		args = fs.Args()
	}
}

// renderToolResult prints what a tool found.
//
// A mismatch is a failure with its own code, and a failed run writes nothing
// to standard output - the same as verify - so the whole answer goes to the
// error channel, JSON included.
func renderToolResult(d tool.Descriptor, r tool.Result, asJSON bool, out, errOut io.Writer) int {
	failed := r.Verdict.Outcome == tool.Mismatch
	w := out
	if failed {
		w = errOut
	}
	if asJSON {
		if code := renderJSON(r.Data, w, errOut); code != ExitOK {
			return code
		}
	} else {
		printToolTable(d.Columns, r.Rows, w)
		if said := r.Verdict.Said(); said != "" {
			fmt.Fprintf(w, "\n%s\n", said)
		}
	}
	if failed {
		return ExitVerify
	}
	return ExitOK
}

// printToolTable prints a result as columns under their headings, each as wide
// as its longest cell.
func printToolTable(columns []string, rows [][]string, w io.Writer) {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	line := func(cells []string) {
		padded := make([]string, len(cells))
		for i, cell := range cells {
			if i == len(cells)-1 {
				padded[i] = cell
				continue
			}
			padded[i] = cell + strings.Repeat(" ", widths[i]-len(cell))
		}
		fmt.Fprintln(w, strings.Join(padded, "  "))
	}
	headings := make([]string, len(columns))
	for i, c := range columns {
		headings[i] = strings.ToUpper(c)
	}
	line(headings)
	for _, row := range rows {
		line(row)
	}
}
