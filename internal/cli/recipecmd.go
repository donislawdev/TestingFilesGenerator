// Part of package cli. See cli.go.
package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// tooLargeToRead reports a recipe past the limit, checked on the directory
// entry rather than after loading.
//
// "Read it all, then say it was too big" is not a limit - the cost was already
// paid by then. One helper for all three commands that take a recipe, so none
// of them can be the one that forgets.
func tooLargeToRead(path string) error {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= recipe.MaxBytes {
		// A path that cannot be examined is not refused here. The read below
		// gives the better message for it.
		return nil
	}
	return &recipe.TooLargeError{Name: path, Bytes: info.Size()}
}

func loadRecipe(path string, errOut io.Writer) (*recipe.Recipe, string, int) {
	if err := tooLargeToRead(path); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return nil, "", ExitRecipe
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: cannot read the recipe %s: %s\n", path, describeError(err))
		return nil, "", ExitIO
	}
	rec, err := recipe.Parse(src, path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, "", classify(err)
	}
	hash, err := recipe.Hash(src)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return nil, "", classify(err)
	}
	return rec, hash, ExitOK
}

// validate runs the checks a run would run and writes nothing at all, so it
// suits a pre commit hook.
func validate(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the result as JSON, with every problem separately")
	usage := func(w io.Writer) {
		fmt.Fprint(w, "tfg validate - check a recipe and write nothing.\n\nUsage:\n  tfg validate <recipe.yaml>\n\nFlags:\n")
		fs.SetOutput(w)
		fs.PrintDefaults()
		fs.SetOutput(errOut)
	}
	fs.Usage = func() { usage(errOut) }
	if helpRequested(args) {
		usage(out)
		return ExitOK
	}
	leading, rest := splitLeadingPath(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	path, ok := onePath(leading, fs)
	if !ok {
		fmt.Fprintln(errOut, "tfg: validate takes one recipe file. Example: tfg validate recipe.yaml")
		return ExitUsage
	}
	if err := mustBeFile(path, "recipe.yaml", "validate"); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return ExitUsage
	}

	rec, hash, code := loadRecipeReporting(path, *asJSON, errOut)
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
		targets = append(targets, engineTarget(t, t.Label))
	}
	planned, err := engine.Plan(targets, planningOptions(rec))
	if err != nil {
		if *asJSON {
			writeJSON(errOut, validateReport{Recipe: path, Valid: false,
				Problems: []validateProblem{problemOf(err)}})
			return classify(err)
		}
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}

	if *asJSON {
		writeJSON(out, validateReport{
			Recipe: path, Valid: true, RecipeHash: hash,
			Targets: len(rec.Targets), Files: len(planned),
			TotalBytes: engine.TotalBytes(planned),
			Problems:   []validateProblem{},
		})
		return ExitOK
	}

	fmt.Fprintf(out, "%s is valid: %s, %s, %d B total\n%s\n",
		path, core.Count(len(rec.Targets), "target", "targets"), core.Count(len(planned), "file", "files"), engine.TotalBytes(planned), hash)
	return ExitOK
}

// planningOptions is what this command hands the planner.
//
// The manifest name is in it, and leaving it out was a hole this command exists
// to not have. Measured on 2026-08-25 with output.manifest set to a name
// holding a bar: validate called the recipe valid and generate refused it with
// code 3 a second later. A pre commit hook running this passed a recipe that
// could not run.
func planningOptions(rec *recipe.Recipe) engine.Options {
	return engine.Options{
		OutDir:       rec.Output.Dir,
		Seed:         rec.Seed,
		ManifestName: rec.Output.Manifest,
	}
}

// validateReport is what --json puts out. Every problem arrives separately
// rather than as one blob of prose, because RC7 already reports them all at
// once and a script should not have to split the message back apart.
type validateReport struct {
	Recipe     string            `json:"recipe"`
	Valid      bool              `json:"valid"`
	RecipeHash string            `json:"recipe_hash,omitempty"`
	Targets    int               `json:"targets,omitempty"`
	Files      int               `json:"files,omitempty"`
	TotalBytes int64             `json:"total_bytes,omitempty"`
	Problems   []validateProblem `json:"problems"`
}

// validateProblem carries the three parts every refusal in this tool has: what
// is wrong, why the rule exists, and what to do instead, plus where it is.
//
// The address is additive and stays omitted when there is none, because a
// problem about the document as a whole has no setting to name. A reader that
// only knows the three parts keeps working.
type validateProblem struct {
	What string `json:"what"`
	Why  string `json:"why,omitempty"`
	Fix  string `json:"fix,omitempty"`
	// At is the setting the problem is about, as a recipe key with a 1-based
	// index where a list is involved: targets[2].size. A script that groups a
	// report by field needs this rather than the sentence, which names a target
	// by its id and cannot be split back apart reliably.
	At string `json:"at,omitempty"`
}

// problemOf turns a refusal from below the recipe reader into a report entry.
//
// A refusal the reader produced arrives already in three parts, because the
// reader built it that way. One from underneath - a format refusing a size, a
// preset refusing a set, the engine refusing a name - arrived as one sentence,
// so a script reading this had to take the sentence apart to group by reason,
// and the sentence is the one thing here written for a person.
//
// The three parts are asked for by name rather than by type, the same way the
// address is. Not everything answers, and one that does not still reports as it
// always did: the whole sentence in what, and nothing in the other two.
func problemOf(err error) validateProblem {
	entry := validateProblem{What: err.Error(), At: addressOf(err)}
	var parted interface {
		What() string
		Why() string
		Instead() string
	}
	if errors.As(err, &parted) {
		entry.What, entry.Why, entry.Fix = parted.What(), parted.Why(), parted.Instead()
	}
	return entry
}

// addressOf is where a refusal from below the recipe reader happened.
//
// Everything that knows the setting it is about answers the same interface the
// window asks, so this does not need to know the type.
//
// It used to drop an address that did not name a target, because a refusal
// from the engine or a format named the setting - "size" - without the entry
// of the list it happened in, and "at": "size" in a report about a recipe with
// twenty targets points at all of them while looking like something to act on.
// That filter went on 2026-08-25, in the same breath as the reason for it: the
// engine gives every refusal about a target the position of that target now,
// so nothing arriving here is half an address. A guard asks that of every
// address this reports rather than leaving it to be true by accident.
func addressOf(err error) string {
	var about interface{ AboutSetting() string }
	if !errors.As(err, &about) {
		return ""
	}
	return about.AboutSetting()
}

// loadRecipeReporting is loadRecipe with the option of a machine readable
// refusal. A recipe with five problems has to arrive as five entries, not as
// one string a script would have to take apart.
func loadRecipeReporting(path string, asJSON bool, errOut io.Writer) (*recipe.Recipe, string, int) {
	if !asJSON {
		return loadRecipe(path, errOut)
	}
	if err := tooLargeToRead(path); err != nil {
		writeJSON(errOut, validateReport{Recipe: path, Valid: false,
			Problems: []validateProblem{{What: err.Error()}}})
		return nil, "", ExitRecipe
	}
	src, err := os.ReadFile(path)
	if err != nil {
		writeJSON(errOut, validateReport{Recipe: path, Valid: false,
			Problems: []validateProblem{{What: fmt.Sprintf("cannot read the recipe: %v", err)}}})
		return nil, "", ExitIO
	}
	rec, err := recipe.Parse(src, path)
	if err != nil {
		report := validateReport{Recipe: path, Valid: false, Problems: []validateProblem{}}
		var invalid *recipe.ValidationError
		if errors.As(err, &invalid) {
			for _, p := range invalid.Problems {
				report.Problems = append(report.Problems, validateProblem{What: p.What, Why: p.Why, Fix: p.Fix, At: p.At})
			}
		} else {
			report.Problems = append(report.Problems, validateProblem{What: err.Error()})
		}
		writeJSON(errOut, report)
		return nil, "", classify(err)
	}
	hash, err := recipe.Hash(src)
	if err != nil {
		writeJSON(errOut, validateReport{Recipe: path, Valid: false,
			Problems: []validateProblem{{What: err.Error()}}})
		return nil, "", classify(err)
	}
	return rec, hash, ExitOK
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
	check := fs.Bool("check", false, "print nothing and end with code 3 when the layout is not settled. It says nothing about whether the recipe is valid - use tfg validate for that")
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg recipe fmt - print a recipe in its settled shape, comments kept.

This settles the layout of a file. It does not check that the recipe makes
sense - a file with a key nobody recognises or a format that does not exist
still has a settled shape, and this will print it. Run "tfg validate" for
that, and run both if you are checking recipes before a commit.

Usage:
  tfg recipe fmt <recipe.yaml>

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
	leading, rest := splitLeadingPath(args[1:])
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	path, ok := onePath(leading, fs)
	if !ok {
		fmt.Fprintln(errOut, "tfg: recipe fmt takes one recipe file. Example: tfg recipe fmt recipe.yaml")
		return ExitUsage
	}
	if err := mustBeFile(path, "recipe.yaml", "recipe fmt"); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return ExitUsage
	}

	if err := tooLargeToRead(path); err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", err)
		return ExitRecipe
	}
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: cannot read the recipe %s: %s\n", path, describeError(err))
		return ExitIO
	}

	canon, err := recipe.Canonical(src, path)
	if err != nil {
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
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
		if err := core.ReplaceFile(path, canon); err != nil {
			fmt.Fprintf(errOut, "tfg: cannot write %s: %s\n", path, describeError(err))
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
