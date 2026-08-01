// Package cli parses flags, keeps data and log on separate channels and maps
// every ending onto an exit code.
//
// The command line is not an advanced mode. It is the interface CI drives, so
// an ending CI cannot tell apart is a defect, not a detail.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
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
)

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
	case "recipe":
		return recipeCmd(args[1:], out, errOut)
	case "formats":
		return formats(args[1:], out, errOut)
	case "--version", "version":
		fmt.Fprintln(out, version.Version)
		return ExitOK
	case "--help", "-h", "help":
		usage(errOut)
		return ExitOK
	default:
		fmt.Fprintf(errOut, "tfg: unknown command %q.\n\n", args[0])
		usage(errOut)
		return ExitUsage
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `tfg - generate test files and know how the system under test should react.

Commands:
  generate    produce files, from a recipe or from flags
  validate    check a recipe and write nothing
  recipe fmt  print a recipe in its settled shape
  formats     list the formats this build supports
  version     print the tool version

Run "tfg <command> --help" for the flags of one command.
`)
}

func generate(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(errOut)

	var (
		formatID = fs.String("format", "", "format of the files to produce, for example txt")
		sizeStr  = fs.String("size", "", "exact size of each file. Units count in 1024s, so 10mb is 10485760 bytes. Also accepts a plain byte count")
		count    = fs.Int("count", 1, "how many files to produce")
		outDir   = fs.String("out", ".", "directory to write into")
		name     = fs.String("name", "", "name template, for example invoice_{index:04}.txt")
		id       = fs.String("id", "files", "target id, the anchor the seeds are derived from")
		seed     = fs.Int64("seed", 0, "run seed, the same seed gives the same bytes")
		expected = fs.String("expected", "", "declared expectation: accept, reject, sanitize or unspecified")
		clean    = fs.Bool("clean", false, "turn off the self describing label")
		dryRun   = fs.Bool("dry-run", false, "count and show, write nothing at all")
		asJSON   = fs.Bool("json", false, "write the manifest to standard output")
	)

	// One repeatable flag rather than a separate flag per format property.
	// Twenty five formats with a dozen properties each would give a surface
	// nobody reads in --help, and this maps one to one onto the properties
	// block of a recipe, so both surfaces speak the same words.
	props := propertyFlag{}
	fs.Var(&props, "set", "format property, repeatable: --set width=1920 --set height=1080")

	fs.Usage = func() {
		fmt.Fprint(errOut, `tfg generate - produce files.

Usage:
  tfg generate <recipe.yaml> [flags]   settings come from the file
  tfg generate --format txt --size 1mb  settings come from the flags

Flags:
`)
		fs.PrintDefaults()
	}

	// A recipe is named first, before any flag. Anywhere else and a value
	// such as "--seed 5" could not be told apart from a file name.
	recipePath, rest := splitRecipeArgument(args)

	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}

	// A flag that was never written must not beat the recipe. The tool sees
	// the default --count 1 and would otherwise wipe out count: 500 from the
	// file, which is the whole class of "my recipe stopped working after I
	// added a flag I did not touch".
	given := flagsGiven(fs)

	opt := engine.Options{
		OutDir:       *outDir,
		Seed:         *seed,
		DryRun:       *dryRun,
		Command:      "tfg " + strings.Join(args2(args), " "),
		ManifestName: defaultManifestName,
	}

	var targets []engine.Target

	if recipePath != "" {
		rec, hash, code := loadRecipe(recipePath, errOut)
		if code != ExitOK {
			return code
		}

		// Flags that describe one target have no target to describe when the
		// recipe names several. Saying so beats picking one silently.
		if bad := describingFlagsGiven(given); len(bad) > 0 {
			fmt.Fprintf(errOut,
				"tfg: %s describe a single target and this run comes from %s. Edit the recipe, or drop the recipe and pass the flags on their own.\n",
				strings.Join(bad, ", "), recipePath)
			return ExitUsage
		}

		opt.RecipeHash = hash
		opt.Overrides = map[string]manifest.Override{}
		opt.OutDir = rec.Output.Dir
		opt.ManifestName = rec.Output.Manifest
		opt.Seed = rec.Seed

		if given["out"] {
			opt.Overrides["out"] = manifest.Override{FromRecipe: rec.Output.Dir, FromFlag: *outDir}
			opt.OutDir = *outDir
		}
		if given["seed"] {
			opt.Overrides["seed"] = manifest.Override{FromRecipe: rec.Seed, FromFlag: *seed}
			opt.Seed = *seed
		}

		for _, t := range rec.Targets {
			label := t.Label
			if given["clean"] {
				label = !*clean
			}
			targets = append(targets, engine.Target{
				ID:             t.ID,
				Format:         t.Format,
				Sizes:          t.Sizes,
				NameTmpl:       t.Name,
				Label:          label,
				Expected:       t.Expected,
				ExpectedReason: t.ExpectedReason,
				Properties:     t.Properties,
			})
		}
		if given["clean"] {
			opt.Overrides["label"] = manifest.Override{FromRecipe: "per target", FromFlag: !*clean}
		}
		if len(opt.Overrides) == 0 {
			opt.Overrides = nil
		}
	} else {
		if *formatID == "" {
			fmt.Fprintln(errOut, "tfg: --format is required. Run \"tfg formats\" to see what this build supports, or name a recipe file instead.")
			return ExitUsage
		}
		if *sizeStr == "" {
			fmt.Fprintln(errOut, "tfg: --size is required. Every target declares its size, which is what lets --dry-run report exact numbers before anything is written.")
			return ExitUsage
		}

		bytesWanted, err := core.ParseSize(*sizeStr)
		if err != nil {
			fmt.Fprintf(errOut, "tfg: %v\n", err)
			return ExitUsage
		}

		// An expectation nobody recognises is a typo, and a typo accepted in
		// silence becomes an expectation no test will ever check.
		switch *expected {
		case "", manifest.OutcomeAccept, manifest.OutcomeReject,
			manifest.OutcomeSanitize, manifest.OutcomeUnspecified:
		default:
			fmt.Fprintf(errOut,
				"tfg: --expected %q is not a known outcome. Use accept, reject, sanitize or unspecified.\n", *expected)
			return ExitUsage
		}

		targets = []engine.Target{{
			ID:         *id,
			Format:     *formatID,
			Sizes:      engine.Uniform(*count, bytesWanted),
			NameTmpl:   *name,
			Label:      !*clean,
			Expected:   *expected,
			Properties: props,
		}}
	}

	planned, err := engine.Plan(targets, opt)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return classify(err)
	}

	total := engine.TotalBytes(planned)
	// Echo the exact byte count. The exact number is the point of this tool,
	// and it is what any other tool will show when the user goes to check the
	// file.
	fmt.Fprintf(errOut, "%d file(s) in %d target(s), %d B total\n",
		len(planned), len(targets), total)

	if *dryRun {
		fmt.Fprintln(errOut, "dry run - nothing was written.")
	}

	res, runErr := engine.Run(ctx, planned, opt)

	for _, n := range res.Manifest.Notes() {
		fmt.Fprintf(errOut, "note: %s\n", n)
	}

	if runErr != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", runErr)
		if !*dryRun {
			saveManifest(res, opt, errOut)
		}
		return classify(runErr)
	}

	if !*dryRun {
		if code := saveManifest(res, opt, errOut); code != ExitOK {
			return code
		}
	}

	// Data goes to standard output and only on success.
	if *asJSON {
		var buf bytes.Buffer
		if err := res.Manifest.Encode(&buf); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot render the manifest: %v\n", err)
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
const defaultManifestName = "manifest.json"

// splitRecipeArgument takes the recipe path off the front of the arguments.
//
// It has to be first. A path recognised anywhere in the list could not be told
// apart from the value of a flag, so "--seed 5" would turn 5 into a file name.
func splitRecipeArgument(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
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
	for _, name := range []string{"format", "size", "count", "name", "id", "set", "expected"} {
		if given[name] {
			bad = append(bad, "--"+name)
		}
	}
	return bad
}

func loadRecipe(path string, errOut io.Writer) (*recipe.Recipe, string, int) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: cannot read the recipe %s: %v\n", path, err)
		return nil, "", ExitIO
	}
	rec, err := recipe.Parse(src, path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return nil, "", classify(err)
	}
	hash, err := recipe.Hash(src)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return nil, "", classify(err)
	}
	return rec, hash, ExitOK
}

// validate runs the checks a run would run and writes nothing at all, so it
// suits a pre commit hook.
func validate(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprint(errOut, "tfg validate - check a recipe and write nothing.\n\nUsage:\n  tfg validate <recipe.yaml>\n")
	}
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(errOut, "tfg: validate takes one recipe file. Example: tfg validate recipe.yaml")
		return ExitUsage
	}

	path := fs.Arg(0)
	rec, hash, code := loadRecipe(path, errOut)
	if code != ExitOK {
		return code
	}

	// The schema and the semantics both passed. Planning is what proves the
	// rest: a size below the minimum of its format, a format nobody
	// registered, a property that does not exist, two files heading for one
	// name. Refusing those here rather than at generate time is the point of
	// this command.
	var targets []engine.Target
	for _, t := range rec.Targets {
		targets = append(targets, engine.Target{
			ID: t.ID, Format: t.Format, Sizes: t.Sizes,
			NameTmpl: t.Name, Label: t.Label, Expected: t.Expected,
			ExpectedReason: t.ExpectedReason, Properties: t.Properties,
		})
	}
	planned, err := engine.Plan(targets, engine.Options{OutDir: rec.Output.Dir, Seed: rec.Seed})
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return classify(err)
	}

	fmt.Fprintf(out, "%s is valid: %d target(s), %d file(s), %d B total\n%s\n",
		path, len(rec.Targets), len(planned), engine.TotalBytes(planned), hash)
	return ExitOK
}

// recipeCmd groups the operations that work on a recipe file itself rather
// than on the files it describes.
func recipeCmd(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] != "fmt" {
		fmt.Fprintln(errOut, "tfg: recipe takes one operation. Example: tfg recipe fmt recipe.yaml")
		return ExitUsage
	}

	fs := flag.NewFlagSet("recipe fmt", flag.ContinueOnError)
	fs.SetOutput(errOut)
	write := fs.Bool("w", false, "write the result back to the file instead of printing it")
	check := fs.Bool("check", false, "print nothing and end with code 3 when the file is not in its settled shape")
	fs.Usage = func() {
		fmt.Fprint(errOut, `tfg recipe fmt - print a recipe in its settled shape, comments kept.

Usage:
  tfg recipe fmt <recipe.yaml>

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args[1:]); err != nil {
		return ExitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(errOut, "tfg: recipe fmt takes one recipe file. Example: tfg recipe fmt recipe.yaml")
		return ExitUsage
	}

	path := fs.Arg(0)
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: cannot read the recipe %s: %v\n", path, err)
		return ExitIO
	}

	canon, err := recipe.Canonical(src, path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return classify(err)
	}

	// Reading the file and finding it already settled is the answer a hook
	// wants, and it costs nothing to say so.
	settled := bytes.Equal(src, canon)

	if *check {
		if settled {
			return ExitOK
		}
		fmt.Fprintf(errOut, "tfg: %s is not in its settled shape. Run \"tfg recipe fmt -w %s\".\n", path, path)
		return ExitRecipe
	}

	if *write {
		if settled {
			fmt.Fprintf(errOut, "%s was already settled and was not touched.\n", path)
			return ExitOK
		}
		if err := os.WriteFile(path, canon, 0o644); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot write %s: %v\n", path, err)
			return ExitIO
		}
		fmt.Fprintf(errOut, "%s rewritten.\n", path)
		return ExitOK
	}

	// Printing by default rather than rewriting. A command that edits a file
	// somebody wrote, without being asked to, is the wrong default in a tool
	// that already refuses to write over anything.
	out.Write(canon)
	return ExitOK
}

func saveManifest(res *engine.Result, opt engine.Options, errOut io.Writer) int {
	name := opt.ManifestName
	if name == "" {
		name = defaultManifestName
	}
	path := filepath.Join(opt.OutDir, name)
	if err := res.Manifest.Save(path); err != nil {
		fmt.Fprintf(errOut, "tfg: cannot write the manifest to %s: %v\n", path, err)
		return ExitIO
	}
	fmt.Fprintf(errOut, "manifest: %s\n", path)
	return ExitOK
}

// formatEntry is what "tfg formats --json" returns. It carries the three
// things a user cannot guess and that decide whether their request makes
// sense at all - how faithful the file will be, whether it repeats to the
// byte, and how small it can go.
type formatEntry struct {
	ID          string   `json:"id"`
	Extension   string   `json:"extension"`
	Fidelity    string   `json:"fidelity"`
	Determinism string   `json:"determinism"`
	MinBytes    int64    `json:"min_bytes"`
	Padding     string   `json:"padding_channel"`
	PaddingCap  int64    `json:"padding_capacity,omitempty"`
	Label       string   `json:"label_carrier"`
	Properties  []string `json:"properties,omitempty"`
	Oracle      string   `json:"oracle"`
	Version     string   `json:"generator_version"`
}

func formats(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("formats", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the list as JSON to standard output")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	if *asJSON {
		list := make([]formatEntry, 0, len(format.All()))
		for _, d := range format.All() {
			list = append(list, formatEntry{
				ID: d.ID, Extension: d.Extension,
				Fidelity: string(d.Fidelity), Determinism: string(d.Determinism),
				MinBytes: d.MinBytes, Padding: d.Padding.Name, PaddingCap: d.Padding.Capacity,
				Label: string(d.Label), Properties: d.Properties,
				Oracle: d.Oracle, Version: d.GeneratorVersion,
			})
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(list); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot render the list: %v\n", err)
			return ExitRuntime
		}
		return ExitOK
	}

	fmt.Fprintf(out, "%-8s %-10s %-12s %-10s %s\n", "FORMAT", "FIDELITY", "DETERMINISM", "MINIMUM", "PADDING CHANNEL")
	for _, d := range format.All() {
		fmt.Fprintf(out, "%-8s %-10s %-12s %-10d %s\n",
			d.ID, d.Fidelity, d.Determinism, d.MinBytes, d.Padding.Name)
	}
	return ExitOK
}

// classify turns an error into an exit code.
//
// The mapping lives here and nowhere else, and it works on error types rather
// than on message text. Anything unrecognised becomes a runtime error, which
// is the honest answer for a failure the tool did not anticipate.
func classify(err error) int {
	var recipeErr *engine.RecipeError
	if errors.As(err, &recipeErr) {
		return ExitRecipe
	}
	var invalid *recipe.ValidationError
	if errors.As(err, &invalid) {
		return ExitRecipe
	}
	var syntax *recipe.SyntaxError
	if errors.As(err, &syntax) {
		return ExitRecipe
	}
	var spaceErr *engine.SpaceError
	if errors.As(err, &spaceErr) {
		return ExitSpace
	}
	var collision *engine.CollisionError
	if errors.As(err, &collision) {
		return ExitIO
	}
	var belowMin *format.BelowMinimumError
	if errors.As(err, &belowMin) {
		return ExitFormat
	}
	var unknown *format.UnknownFormatError
	if errors.As(err, &unknown) {
		return ExitFormat
	}
	var badProp *format.UnknownPropertyError
	if errors.As(err, &badProp) {
		return ExitUsage
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return ExitIO
	}
	return ExitRuntime
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

func args2(args []string) []string {
	out := []string{"generate"}
	return append(out, args...)
}
