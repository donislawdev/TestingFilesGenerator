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
  generate    produce files
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
		fmt.Fprint(errOut, "tfg generate - produce files.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	if *formatID == "" {
		fmt.Fprintln(errOut, "tfg: --format is required. Run \"tfg formats\" to see what this build supports.")
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

	target := engine.Target{
		ID:         *id,
		Format:     *formatID,
		Count:      *count,
		Bytes:      bytesWanted,
		NameTmpl:   *name,
		Label:      !*clean,
		Expected:   *expected,
		Properties: props,
	}
	opt := engine.Options{
		OutDir:  *outDir,
		Seed:    *seed,
		DryRun:  *dryRun,
		Command: "tfg " + strings.Join(args2(args), " "),
	}

	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %v\n", err)
		return classify(err)
	}

	total := engine.TotalBytes(planned)
	// Echo what was asked for next to the exact byte count. The exact number
	// is the point of this tool, and it is what any other tool will show when
	// the user goes to check the file.
	fmt.Fprintf(errOut, "%d file(s) of %s = %d B each, %d B total\n",
		len(planned), *sizeStr, bytesWanted, total)

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

func saveManifest(res *engine.Result, opt engine.Options, errOut io.Writer) int {
	path := filepath.Join(opt.OutDir, "manifest.json")
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
