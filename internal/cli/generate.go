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
	formatID string
	sizeStr  string
	count    int
	outDir   string
	name     string
	id       string
	seed     int64
	expected string
	clean    bool
	dryRun   bool
	asJSON   bool
	props    propertyFlag
}

func generateFlagSet(errOut io.Writer, g *generateOpts) *flag.FlagSet {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(errOut)

	fs.StringVar(&g.formatID, "format", "", "format of the files to produce, for example txt")
	fs.StringVar(&g.sizeStr, "size", "", "exact size of each file. Units count in 1024s, so 10mb is 10485760 bytes. Also accepts a plain byte count")
	fs.IntVar(&g.count, "count", 1, "how many files to produce")
	fs.StringVar(&g.outDir, "out", ".", "directory to write into")
	fs.StringVar(&g.name, "name", "", "name template, for example invoice_{index:04}.txt")
	fs.StringVar(&g.id, "id", "files", "target id, the anchor the seeds are derived from")
	fs.Int64Var(&g.seed, "seed", 0, "run seed, the same seed gives the same bytes")
	fs.StringVar(&g.expected, "expected", "", "declared expectation: accept, reject, sanitize or unspecified")
	fs.BoolVar(&g.clean, "clean", false, "turn off the self describing label")
	fs.BoolVar(&g.dryRun, "dry-run", false, "count and show, write nothing at all")
	fs.BoolVar(&g.asJSON, "json", false, "write the manifest to standard output")

	// One repeatable flag rather than a separate flag per format property.
	// Twenty five formats with a dozen properties each would give a surface
	// nobody reads in --help, and this maps one to one onto the properties
	// block of a recipe, so both surfaces speak the same words.
	g.props = propertyFlag{}
	fs.Var(&g.props, "set", "format property, repeatable: --set width=1920 --set height=1080")

	fs.Usage = func() {
		fmt.Fprint(errOut, `tfg generate - produce files.

Usage:
  tfg generate <recipe.yaml> [flags]   settings come from the file
  tfg generate --format txt --size 1mb  settings come from the flags

Flags:
`)
		fs.PrintDefaults()
	}
	return fs
}

func generate(ctx context.Context, args []string, out, errOut io.Writer) int {
	var g generateOpts
	fs := generateFlagSet(errOut, &g)

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
		targets, code = targetsFromFlags(&g, errOut)
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
func targetsFromFlags(g *generateOpts, errOut io.Writer) ([]engine.Target, int) {
	if g.formatID == "" {
		fmt.Fprintln(errOut, "tfg: --format is required. Run \"tfg formats\" to see what this build supports, or name a recipe file instead.")
		return nil, ExitUsage
	}
	if g.sizeStr == "" {
		fmt.Fprintln(errOut, "tfg: --size is required. Every target declares its size, which is what lets --dry-run report exact numbers before anything is written.")
		return nil, ExitUsage
	}

	bytesWanted, err := core.ParseSize(g.sizeStr)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, ExitUsage
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

	return []engine.Target{{
		ID:         g.id,
		Format:     g.formatID,
		Sizes:      engine.Uniform(g.count, bytesWanted),
		NameTmpl:   g.name,
		Label:      !g.clean,
		Expected:   g.expected,
		Properties: g.props,
	}}, ExitOK
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

	if g.dryRun {
		fmt.Fprintln(errOut, "dry run - nothing was written.")
	}

	res, runErr := engine.Run(ctx, planned, opt)

	for _, n := range res.Manifest.Notes() {
		fmt.Fprintf(errOut, "note: %s\n", n)
	}

	if runErr != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(runErr))
		if !g.dryRun {
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
