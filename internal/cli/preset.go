// Part of package cli. See cli.go.
package cli

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// expandedPreset is one preset settled on its parameters and turned into the
// recipe it stands for.
//
// The source is what a run consumes and what eject prints, because it is the
// same bytes - PR5 in docs/PRESETS.md. Every command here goes through this one
// function, so none of them can be the one that expands differently.
type expandedPreset struct {
	Preset  preset.Preset
	Settled preset.Args
	// Defaulted names the parameters nobody gave, whose declared default stood
	// in. Sorted, so two runs of one preset produce identical records.
	Defaulted []string
	Source    []byte
}

func expandPreset(id string, given preset.Args) (*expandedPreset, error) {
	p, err := preset.Get(id)
	if err != nil {
		return nil, err
	}
	settled, err := p.Settle(given)
	if err != nil {
		return nil, err
	}

	var defaulted []string
	for name := range p.Defaults() {
		if given[name] == "" {
			defaulted = append(defaulted, name)
		}
	}
	sort.Strings(defaulted)

	src, err := p.Expand(settled)
	if err != nil {
		return nil, err
	}
	return &expandedPreset{Preset: p, Settled: settled, Defaulted: defaulted, Source: src}, nil
}

// record is what goes into the manifest.
func (e *expandedPreset) record() *manifest.Preset {
	return &manifest.Preset{
		ID:         e.Preset.ID,
		Parameters: map[string]string(e.Settled),
		Defaulted:  e.Defaulted,
	}
}

// notes is what to say out loud about a value nobody gave us.
//
// Some defaults describe our own file and some describe somebody else's system.
// A set built around a limit we invented carries expectations that read exactly
// like a set built around the real one, so the run says which number it made up.
func (e *expandedPreset) notes() []string {
	var out []string
	for _, name := range e.Defaulted {
		if said := e.Preset.SaidWhenDefaulted[name]; said != "" {
			out = append(out, said)
		}
	}
	return out
}

// budget is what a preset would produce, counted by the planner.
//
// Not a declared number beside the code. The one that used to sit in
// docs/PRESETS.md was wrong by a factor of three and a half for three days,
// because the distances of a boundary set cancel in pairs and nobody had
// multiplied it out. PR3 gets a mechanism rather than a promise.
type budget struct {
	Targets int      `json:"targets"`
	Files   int      `json:"files"`
	Bytes   int64    `json:"total_bytes"`
	Formats []string `json:"formats"`
}

func budgetOf(e *expandedPreset) (budget, error) {
	rec, err := recipe.Parse(e.Source, e.Preset.ID)
	if err != nil {
		return budget{}, err
	}
	targets := make([]engine.Target, 0, len(rec.Targets))
	seen := map[string]bool{}
	for _, t := range rec.Targets {
		targets = append(targets, engineTarget(t, t.Label))
		seen[t.Format] = true
	}
	planned, err := engine.Plan(targets, engine.Options{OutDir: rec.Output.Dir, Seed: rec.Seed})
	if err != nil {
		return budget{}, err
	}

	// Read back from the expansion rather than from the parameters, because a
	// preset may settle a format nobody named and the budget is for the files
	// it would really write.
	formats := make([]string, 0, len(seen))
	for id := range seen {
		formats = append(formats, id)
	}
	sort.Strings(formats)

	return budget{
		Targets: len(rec.Targets), Files: len(planned),
		Bytes: engine.TotalBytes(planned), Formats: formats,
	}, nil
}

// presetEntry is one preset as a script sees it. Everything the declaration
// carries, the same as a format entry, so a window has something to build from.
type presetEntry struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Question   string          `json:"question"`
	Parameters []propertyEntry `json:"parameters,omitempty"`
	// Reads are global flags this preset gives a default to instead of
	// declaring a parameter of its own.
	Reads    []string `json:"reads,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Catches  []string `json:"catches,omitempty"`
	// Budget is present in "preset show", where the parameters are settled.
	Budget *budget `json:"budget,omitempty"`
	// Defaulted and Notes say which numbers are ours rather than the caller's.
	Defaulted []string `json:"defaulted,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

func presetEntryFor(p preset.Preset) presetEntry {
	params := make([]propertyEntry, 0, len(p.Parameters))
	for _, param := range p.Parameters {
		params = append(params, propertyEntry{
			Name: param.Name, Kind: string(param.Kind),
			Min: param.Min, Max: param.Max, Unit: param.Unit,
			Choices: param.Choices, Default: param.Default, Detail: param.Detail,
		})
	}
	return presetEntry{
		ID: p.ID, Title: p.Title, Question: p.Question,
		Parameters: params, Reads: p.Reads,
		Requires: p.Requires, Catches: p.Catches,
	}
}

// registerPresetFlags puts a preset's parameters on the flag set.
//
// They cannot be registered before the preset is known, which is why every
// command taking one reads the id out of the arguments before it parses.
// docs/CLI.md section 6 puts them in one namespace with the global flags on
// purpose: --limit reads as a limit whoever declared it.
//
// Registered with an empty default rather than the declared one. Telling "not
// given" from "given the value of the default" is the whole of the precedence
// rule, and Settle is what fills the defaults in afterwards.
func registerPresetFlags(fs *flag.FlagSet, p preset.Preset) {
	for _, param := range p.Parameters {
		fs.String(param.Name, "", parameterUsage(param))
	}
	for _, name := range p.Reads {
		// generate already has the global flag and its own wording for it.
		if fs.Lookup(name) != nil {
			continue
		}
		fs.String(name, "", "the global --"+name+" flag, which this preset gives a default for")
	}
}

func parameterUsage(param format.Property) string {
	usage := allowedText(param)
	if param.Detail != "" {
		usage += ". " + param.Detail
	}
	return usage
}

// clashingParameter reports a parameter colliding with a flag already there.
//
// docs/CLI.md section 6 calls this a mistake in the definition of the preset,
// caught at startup rather than in the middle of a run. It has to be caught,
// not merely documented: the flag package answers a name registered twice with
// a panic, and a panic reaches somebody as a stack trace under the exit code
// that means they mistyped something. A guard refuses to let one ship, and this
// is what happens if one ever does.
func clashingParameter(fs *flag.FlagSet, p preset.Preset) string {
	for _, param := range p.Parameters {
		if fs.Lookup(param.Name) != nil {
			return param.Name
		}
	}
	return ""
}

// givenPresetArgs is the parameters the caller actually wrote.
//
// Read from the flag set rather than from the values, for the same reason
// flagsGiven exists: a parameter given the same text as its default is not the
// same as one left out, and only one of the two gets said out loud.
func givenPresetArgs(fs *flag.FlagSet, p preset.Preset) preset.Args {
	names := map[string]bool{}
	for _, param := range p.Parameters {
		names[param.Name] = true
	}
	for _, r := range p.Reads {
		names[r] = true
	}

	given := preset.Args{}
	fs.Visit(func(f *flag.Flag) {
		if names[f.Name] && f.Value.String() != "" {
			given[f.Name] = f.Value.String()
		}
	})
	return given
}

// presetNamed finds the preset in the arguments before they are parsed.
//
// Only to decide which flags exist. What actually runs is the parsed value, so
// a wrong guess here costs nothing: flags nobody used were registered, and any
// parameter given without its preset is still refused as undefined.
//
// Scanning stops at "--", after which nothing is a flag any more.
func presetNamed(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		name, value, hasValue := strings.Cut(a, "=")
		if name != "--preset" && name != "-preset" {
			continue
		}
		if hasValue {
			return value
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// parameterWithoutItsPreset names a preset parameter typed on its own.
//
// Only consulted after parsing has already failed, so it cannot mistake a value
// for a flag - a parse that succeeded means every name was defined. Names the
// command does define are skipped for the same reason: with --preset given,
// --limit is defined, and the failure was about something else entirely.
func parameterWithoutItsPreset(fs *flag.FlagSet, args []string) (name, owner string) {
	for _, a := range args {
		if a == "--" {
			return "", ""
		}
		if !strings.HasPrefix(a, "-") {
			continue
		}
		candidate, _, _ := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if fs.Lookup(candidate) != nil {
			continue
		}
		if owner := preset.Declaring(candidate); owner != "" {
			return candidate, owner
		}
	}
	return "", ""
}

// addPresetFlags puts the parameters of the named preset on the generate flag
// set, before anything is parsed.
func addPresetFlags(fs *flag.FlagSet, args []string, errOut io.Writer) int {
	id := presetNamed(args)
	if id == "" {
		return ExitOK
	}
	p, err := preset.Get(id)
	if err != nil {
		// Left alone here. The dispatch below reports it once, with the id the
		// caller actually wrote rather than the one this scan guessed at.
		return ExitOK
	}
	if clash := clashingParameter(fs, p); clash != "" {
		fmt.Fprintf(errOut, "tfg: the preset %s declares a parameter called %q and that is already a flag of generate. This is a fault in the build rather than in what you typed, and there is nothing you can do about it from here.\n", p.ID, clash)
		return ExitRuntime
	}
	registerPresetFlags(fs, p)
	return ExitOK
}

// explainUndefinedFlag answers the mistake the shared namespace invites.
//
// Parameters of a preset are flags, so --limit exists in one invocation and not
// in the next. Left to the flag package that arrives as "flag provided but not
// defined", which says nothing about where the flag went or how to get it back.
// It reports whether it answered, so the caller knows whether the complaint it
// held back still has to be let through.
func explainUndefinedFlag(fs *flag.FlagSet, args []string, errOut io.Writer) bool {
	name, owner := parameterWithoutItsPreset(fs, args)
	if name == "" {
		return false
	}
	fmt.Fprintf(errOut,
		"tfg: --%s is a parameter of the preset %s, so it only exists beside it. Add --preset %s, or drop --%s.\n",
		name, owner, owner, name)
	return true
}

// describingFlagsBeside is describingFlagsGiven minus what this preset reads.
//
// A preset lays out a whole set, so a flag describing one target has no target
// to describe - the same rule as beside a recipe. The exception is a flag the
// preset itself reads: --format is a global flag that size-boundaries supplies
// a default for, and refusing it would refuse the documented way to use it.
func describingFlagsBeside(p preset.Preset, given map[string]bool) []string {
	reads := map[string]bool{}
	for _, r := range p.Reads {
		reads[r] = true
	}
	var bad []string
	for _, name := range describingFlagsGiven(given) {
		if !reads[strings.TrimPrefix(name, "--")] {
			bad = append(bad, name)
		}
	}
	return bad
}

// targetsFromPreset expands a preset and hands the result to the recipe path.
//
// It becomes an ordinary recipe here and stays one, which is what makes the
// record honest: the manifest carries a recipe hash that "tfg preset eject"
// reproduces exactly, because both sides are the same bytes through the same
// canonical form.
func targetsFromPreset(fs *flag.FlagSet, g *generateOpts, given map[string]bool, opt *engine.Options, errOut io.Writer) ([]engine.Target, int) {
	p, err := preset.Get(g.presetID)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}
	if bad := describingFlagsBeside(p, given); len(bad) > 0 {
		fmt.Fprintf(errOut,
			"tfg: %s describe a single target and the preset %s lays out a whole set, where the sizes are the statement it makes. Run \"tfg preset eject %s\" to get the recipe and edit it.\n",
			strings.Join(bad, ", "), p.ID, p.ID)
		return nil, ExitUsage
	}

	expanded, err := expandPreset(g.presetID, givenPresetArgs(fs, p))
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}
	rec, err := recipe.Parse(expanded.Source, g.presetID)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}
	hash, err := recipe.Hash(expanded.Source)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, classify(err)
	}

	for _, note := range expanded.notes() {
		fmt.Fprintf(errOut, "note: %s\n", note)
	}
	opt.Preset = expanded.record()
	return targetsFromParsedRecipe(rec, hash, g, given, opt), ExitOK
}

func presetCmd(args []string, out, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "list":
			return presetList(args[1:], out, errOut)
		case "show":
			return presetShow(args[1:], out, errOut)
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
	*expandedPreset, int) {

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

	expanded, err := expandPreset(id, givenPresetArgs(fs, p))
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

func presetShow(args []string, out, errOut io.Writer) int {
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

	b, err := budgetOf(expanded)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	if asJSON {
		entry := presetEntryFor(expanded.Preset)
		entry.Budget = &b
		entry.Defaulted = expanded.Defaulted
		entry.Notes = expanded.notes()
		return renderJSON(entry, out, errOut)
	}
	describePreset(expanded, b, out)
	return ExitOK
}

func describePreset(e *expandedPreset, b budget, out io.Writer) {
	p := e.Preset
	fmt.Fprintf(out, "%s - %s\n%s\n", p.ID, p.Title, p.Question)

	if len(p.Parameters) > 0 {
		fmt.Fprint(out, "\nparameters:\n")
		for _, param := range p.Parameters {
			fmt.Fprintf(out, "  --%-12s %s\n", param.Name, allowedText(param))
			if param.Detail != "" {
				fmt.Fprintf(out, "  %-14s %s\n", "", param.Detail)
			}
		}
	}
	for _, name := range p.Reads {
		fmt.Fprintf(out, "  --%-12s the global flag, this preset gives it a default\n", name)
	}

	fmt.Fprintf(out, "\nbudget at these values:\n  %d target(s), %d file(s), %d B total, format %s\n",
		b.Targets, b.Files, b.Bytes, strings.Join(b.Formats, ", "))
	for _, note := range e.notes() {
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
	for _, note := range expanded.notes() {
		fmt.Fprintf(errOut, "note: %s\n", note)
	}
	if _, err := out.Write(expanded.Source); err != nil {
		fmt.Fprintf(errOut, "tfg: cannot write the recipe: %s\n", describeError(err))
		return ExitIO
	}
	return ExitOK
}
