// Part of package cli. See cli.go.
package cli

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

type generateOpts struct {
	formatID       string
	sizeStr        string
	sizeRange      string
	boundary       string
	count          int
	outDir         string
	name           string
	id             string
	seed           int64
	expected       string
	expectedReason string
	clean          bool
	dryRun         bool
	asJSON         bool
	props          propertyFlag
}

func generateFlagSet(errOut io.Writer, g *generateOpts) (*flag.FlagSet, func(io.Writer)) {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(errOut)

	fs.StringVar(&g.formatID, "format", "", "format of the files to produce, for example txt")
	fs.StringVar(&g.sizeStr, "size", "", "exact size of each file. Units count in 1024s, so 10mb is 10485760 bytes. Also accepts a plain byte count")
	fs.StringVar(&g.sizeRange, "size-range", "", "a size drawn from a range for each file, as 1kb-8kb. The draw comes from the seed, so the same seed gives the same sizes")
	fs.StringVar(&g.boundary, "boundary", "", "three files around a limit: one byte under it, the limit itself, one byte over")
	fs.IntVar(&g.count, "count", 1, "how many files to produce")
	fs.StringVar(&g.outDir, "out", ".", "directory to write into")
	fs.StringVar(&g.name, "name", "", "name template, for example invoice_{index:04}.txt")
	fs.StringVar(&g.id, "id", "files", "target id, the anchor the seeds are derived from")
	fs.Int64Var(&g.seed, "seed", 0, "run seed, the same seed gives the same bytes")
	fs.StringVar(&g.expected, "expected", "", "declared expectation: accept, reject, sanitize or unspecified")
	fs.StringVar(&g.expectedReason, "expected-reason", "",
		"why that outcome is expected, from the closed list. Run with an unknown value to see it")
	fs.BoolVar(&g.clean, "clean", false, "turn off the self describing label")
	fs.BoolVar(&g.dryRun, "dry-run", false, "count and show, write nothing at all")
	fs.BoolVar(&g.asJSON, "json", false, "write the manifest to standard output")

	// One repeatable flag rather than a separate flag per format property.
	// Twenty five formats with a dozen properties each would give a surface
	// nobody reads in --help, and this maps one to one onto the properties
	// block of a recipe, so both surfaces speak the same words.
	g.props = propertyFlag{}
	fs.Var(&g.props, "set", "format property, repeatable: --set width=1920 --set height=1080")

	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg generate - produce files.

Usage:
  tfg generate <recipe.yaml> [flags]   settings come from the file
  tfg generate --format txt --size 1mb  settings come from the flags

Flags:
`)
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(errOut)
	}
	fs.Usage = func() { usage(errOut) }
	return fs, usage
}

func generate(ctx context.Context, args []string, out, errOut io.Writer) int {
	var g generateOpts
	fs, usage := generateFlagSet(errOut, &g)
	if helpRequested(args) {
		usage(out)
		return ExitOK
	}

	// A recipe is named first, before any flag. Anywhere else and a value
	// such as "--seed 5" could not be told apart from a file name.
	recipePath, rest := splitLeadingPath(args)

	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}

	// A flag that was never written must not beat the recipe. The tool sees
	// the default --count 1 and would otherwise wipe out count: 500 from the
	// file, which is the whole class of "my recipe stopped working after I
	// added a flag I did not touch".
	given := flagsGiven(fs)

	opt := engine.Options{
		OutDir:       g.outDir,
		Seed:         g.seed,
		DryRun:       g.dryRun,
		Command:      "tfg " + strings.Join(args2(args), " "),
		ManifestName: defaultManifestName,
	}

	var (
		targets []engine.Target
		code    int
	)
	if recipePath != "" {
		targets, code = targetsFromRecipe(recipePath, &g, given, &opt, errOut)
	} else {
		targets, code = targetsFromFlags(&g, given, errOut)
	}
	if code != ExitOK {
		return code
	}

	return produce(ctx, targets, opt, &g, out, errOut)
}

// targetsFromRecipe reads the recipe and settles what the flags override.
func targetsFromRecipe(path string, g *generateOpts, given map[string]bool, opt *engine.Options, errOut io.Writer) ([]engine.Target, int) {
	rec, hash, code := loadRecipe(path, errOut)
	if code != ExitOK {
		return nil, code
	}

	// Flags that describe one target have no target to describe when the
	// recipe names several. Saying so beats picking one silently.
	if bad := describingFlagsGiven(given); len(bad) > 0 {
		fmt.Fprintf(errOut,
			"tfg: %s describe a single target and this run comes from %s. Edit the recipe, or drop the recipe and pass the flags on their own.\n",
			strings.Join(bad, ", "), path)
		return nil, ExitUsage
	}

	opt.RecipeHash = hash
	opt.Overrides = map[string]manifest.Override{}
	opt.OutDir = rec.Output.Dir
	opt.ManifestName = rec.Output.Manifest
	opt.Seed = rec.Seed

	if given["out"] {
		opt.Overrides["out"] = manifest.Override{FromRecipe: rec.Output.Dir, FromFlag: g.outDir}
		opt.OutDir = g.outDir
	}
	if given["seed"] {
		opt.Overrides["seed"] = manifest.Override{FromRecipe: rec.Seed, FromFlag: g.seed}
		opt.Seed = g.seed
	}

	var targets []engine.Target
	for _, t := range rec.Targets {
		label := t.Label
		if given["clean"] {
			label = !g.clean
		}
		targets = append(targets, engine.Target{
			ID:               t.ID,
			Format:           t.Format,
			Sizes:            t.Sizes,
			Contains:         contentsOf(t),
			SizeFromContents: t.SizeFromContents,
			SizeIsRange:      t.SizeIsRange,
			SizeMin:          t.SizeMin,
			SizeMax:          t.SizeMax,
			BoundaryLimit:    t.BoundaryLimit,
			NameTmpl:         t.Name,
			Label:            label,
			Expected:         t.Expected,
			ExpectedReason:   t.ExpectedReason,
			Properties:       t.Properties,
		})
	}
	if given["clean"] {
		opt.Overrides["label"] = manifest.Override{FromRecipe: "per target", FromFlag: !g.clean}
	}
	if len(opt.Overrides) == 0 {
		opt.Overrides = nil
	}
	return targets, ExitOK
}

// targetsFromFlags builds the single target of a run with no recipe.
func targetsFromFlags(g *generateOpts, given map[string]bool, errOut io.Writer) ([]engine.Target, int) {
	if g.formatID == "" {
		fmt.Fprintln(errOut, "tfg: --format is required. Run \"tfg formats\" to see what this build supports, or name a recipe file instead.")
		return nil, ExitUsage
	}
	// One of the three, and exactly one. Two of them together is two answers to
	// one question, and picking one silently is how a command stops meaning
	// what it says - the same rule the recipe applies to the same three keys.
	stated := 0
	for _, s := range []string{g.sizeStr, g.sizeRange, g.boundary} {
		if s != "" {
			stated++
		}
	}
	switch {
	case stated == 0:
		fmt.Fprintln(errOut, "tfg: one of --size, --size-range or --boundary is required. Every target declares its size, which is what lets --dry-run report exact numbers before anything is written.")
		return nil, ExitUsage
	case stated > 1:
		fmt.Fprintln(errOut, "tfg: --size, --size-range and --boundary each decide how big the files are, so only one of them can be given. Keep --size for identical files, --size-range for a different size each, or --boundary to test a limit.")
		return nil, ExitUsage
	}

	// Reported with the number the caller wrote. It used to fall through to the
	// planner, which builds an empty list from anything below one and then says
	// "asks for 0 files" - a sentence about a number nobody typed.
	if given["count"] && g.count < 1 {
		fmt.Fprintf(errOut,
			"tfg: --count %d asks for fewer than one file. A target that produces nothing is almost always a mistake rather than an intention. Ask for at least one, or leave the flag out to get a single file.\n",
			g.count)
		return nil, ExitRecipe
	}

	// Asked before the list is built, because building it is the failure. A
	// count past the ceiling used to reach make([]int64) and panic with a stack
	// trace under the exit code that means a mistyped flag.
	if int64(g.count) > core.MaxFilesPerRun {
		fmt.Fprintf(errOut, "tfg: --count %d cannot be planned - %s\n",
			g.count, core.ErrTooManyFiles)
		return nil, ExitRecipe
	}

	// A boundary set is exactly three files, so a count beside it is a number
	// that would be thrown away. Saying so beats producing three files for
	// somebody who asked for fifty.
	if g.boundary != "" && given["count"] {
		fmt.Fprintln(errOut, "tfg: --boundary and --count say different things about how many files to produce. A boundary set is exactly three: one byte under the limit, the limit itself, one byte over. Drop --count, or use --size with --count for identical files.")
		return nil, ExitUsage
	}

	sizes, rangeLow, rangeHigh, boundaryLimit, code := sizesFromFlags(g, errOut)
	if code != ExitOK {
		return nil, code
	}

	// An expectation nobody recognises is a typo, and a typo accepted in
	// silence becomes an expectation no test will ever check.
	switch g.expected {
	case "", manifest.OutcomeAccept, manifest.OutcomeReject,
		manifest.OutcomeSanitize, manifest.OutcomeUnspecified:
	default:
		fmt.Fprintf(errOut,
			"tfg: --expected %q is not a known outcome. Use accept, reject, sanitize or unspecified.\n", g.expected)
		return nil, ExitUsage
	}

	// The list is closed so that a report can group by reason, and a typo would
	// make a category of one. Same list the recipe uses rather than a second
	// copy, because two copies is how the two surfaces drift apart.
	if g.expectedReason != "" && !recipe.KnownReason(g.expectedReason) {
		fmt.Fprintf(errOut,
			"tfg: --expected-reason %q is not on the list. The list is closed so that a report can group by reason. Use one of: %s.\n",
			g.expectedReason, strings.Join(recipe.Reasons(), ", "))
		return nil, ExitUsage
	}
	if g.expectedReason != "" && g.expected == "" {
		fmt.Fprintln(errOut,
			"tfg: --expected-reason says why an outcome is expected and no outcome was given. Add --expected accept, reject, sanitize or unspecified, or drop the reason.")
		return nil, ExitUsage
	}

	return []engine.Target{{
		ID:             g.id,
		Format:         g.formatID,
		Sizes:          sizes,
		SizeIsRange:    g.sizeRange != "",
		SizeMin:        rangeLow,
		SizeMax:        rangeHigh,
		BoundaryLimit:  boundaryLimit,
		NameTmpl:       g.name,
		Label:          !g.clean,
		Expected:       g.expected,
		ExpectedReason: g.expectedReason,
		Properties:     g.props,
	}}, ExitOK
}

// sizesFromFlags turns whichever of the three size flags was given into the
// list of sizes, and for a range into the ends the engine draws between.
//
// The maths for both a range and a boundary lives in core, so this and the
// recipe reader cannot drift into disagreeing about what 1kb-8kb or a limit
// means. A range leaves the list carrying only the count, exactly as the
// recipe does, because the draw needs the seed of the run.
func sizesFromFlags(g *generateOpts, errOut io.Writer) (sizes []int64, low, high, boundaryLimit int64, code int) {
	switch {
	case g.sizeRange != "":
		lo, hi, err := core.ParseSizeRange(g.sizeRange)
		if err != nil {
			fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
			return nil, 0, 0, 0, ExitUsage
		}
		return make([]int64, max(g.count, 0)), lo, hi, 0, ExitOK

	case g.boundary != "":
		limit, err := core.ParseBoundary(g.boundary)
		if err != nil {
			fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
			return nil, 0, 0, 0, ExitUsage
		}
		if limit < 1 {
			fmt.Fprintf(errOut,
				"tfg: --boundary %d B is too small. The set needs a size one byte below the limit and there is nothing below zero. Use a limit of at least 1 B.\n", limit)
			return nil, 0, 0, 0, ExitUsage
		}
		sizes, err := core.BoundarySizes(limit)
		if err != nil {
			fmt.Fprintf(errOut, "tfg: --boundary %d B is too large - %s\n", limit, err)
			return nil, 0, 0, 0, ExitRecipe
		}
		return sizes, 0, 0, limit, ExitOK

	default:
		bytesWanted, err := core.ParseSize(g.sizeStr)
		if err != nil {
			fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
			return nil, 0, 0, 0, ExitUsage
		}
		return engine.Uniform(g.count, bytesWanted), 0, 0, 0, ExitOK
	}
}

// produce plans the run, writes it and reports what happened.
func produce(ctx context.Context, targets []engine.Target, opt engine.Options, g *generateOpts, out, errOut io.Writer) int {
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	// Echo the exact byte count. The exact number is the point of this tool,
	// and it is what any other tool will show when the user goes to check the
	// file.
	fmt.Fprintf(errOut, "%d file(s) in %d target(s), %d B total\n",
		len(planned), len(targets), engine.TotalBytes(planned))

	echoBoundaries(targets, planned, errOut)

	if g.dryRun {
		fmt.Fprintln(errOut, "dry run - nothing was written.")
	}

	// Nil when the error channel is not a terminal, and then the engine is
	// told nothing is listening rather than being handed a callback that
	// throws its work away.
	bar := newProgressBar(errOut)
	if bar != nil && !g.dryRun {
		opt.OnProgress = bar.report
	}

	res, runErr := engine.Run(ctx, planned, opt)

	// Taken back before anything else is written, or the summary lands on top
	// of a half drawn bar.
	if bar != nil {
		bar.clear()
	}

	for _, n := range res.Manifest.Notes() {
		fmt.Fprintf(errOut, "note: %s\n", n)
	}

	if runErr != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(runErr))
		// A run that was refused before it wrote anything gets no manifest.
		// Writing one would replace the record of whatever was already in the
		// directory, and that record is the only thing cleanup can work from.
		if !g.dryRun && res.Started {
			saveManifest(res, opt, errOut)
		}
		return classify(runErr)
	}

	if !g.dryRun {
		if code := saveManifest(res, opt, errOut); code != ExitOK {
			return code
		}
	}

	// Data goes to standard output and only on success.
	if g.asJSON {
		var buf bytes.Buffer
		if err := res.Manifest.Encode(&buf); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot render the manifest: %s\n", describeError(err))
			return ExitRuntime
		}
		out.Write(buf.Bytes())
	}

	if res.Failures > 0 {
		fmt.Fprintf(errOut, "tfg: %d file(s) could not be produced. The manifest says which ones.\n", res.Failures)
		return ExitPartial
	}
	return ExitOK
}

// defaultManifestName is where the manifest lands when nothing says otherwise.

// echoBoundaries spells out a boundary set, because a boundary set exists to
// sit either side of a limit that belongs to somebody else, and the exact
// numbers are the whole of it.
//
// Reported by hand: a set built for a 15 MB limit had all three files refused
// by the service it was aimed at. Nothing was broken. Sizes here count in
// 1024s, so the set sat around 15728640, and the service meant 15000000 - so
// every file was over the limit and the set tested nothing at all.
func echoBoundaries(targets []engine.Target, planned []engine.PlannedFile, errOut io.Writer) {
	for i := range targets {
		t := &targets[i]
		if t.BoundaryLimit <= 0 {
			continue
		}
		fmt.Fprintf(errOut, "boundary %q around %d B:\n", t.ID, t.BoundaryLimit)
		for _, f := range planned {
			if f.Target == t {
				fmt.Fprintf(errOut, "  %-26s %d B\n", f.Name, f.Plan.Bytes)
			}
		}

	}
}

func contentsOf(t recipe.Target) []format.Content {
	if t.Contains == nil {
		return nil
	}
	out := make([]format.Content, 0, len(t.Contains))
	for _, c := range t.Contains {
		out = append(out, format.Content{Format: c.Format, Count: c.Count, Bytes: c.Bytes})
	}
	return out
}

func saveManifest(res *engine.Result, opt engine.Options, errOut io.Writer) int {
	name := opt.ManifestName
	if name == "" {
		name = defaultManifestName
	}
	path := filepath.Join(opt.OutDir, name)
	if err := res.Manifest.Save(path); err != nil {
		fmt.Fprintf(errOut, "tfg: cannot write the manifest to %s: %s\n", path, describeError(err))
		return ExitIO
	}
	fmt.Fprintf(errOut, "manifest: %s\n", path)
	return ExitOK
}

// formatEntry is what "tfg formats --json" returns. It carries the three
// things a user cannot guess and that decide whether their request makes
// sense at all - how faithful the file will be, whether it repeats to the
// byte, and how small it can go.
