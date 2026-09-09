// Part of package cli. See cli.go.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
)

// damageEntry is what "tfg damage --json" returns.
//
// The parameters ride in the same propertyEntry the format and the preset lists
// use, under the same key the preset list gives them. That is not tidiness: a
// damage parameter IS a format.Property, so a script that already knows how to
// draw a field from "tfg preset show --json" draws this one with no new code.
type damageEntry struct {
	ID     string `json:"id"`
	Detail string `json:"detail"`
	// SmallestFileBytes is the smallest file this damage can be given, with
	// its settings left at their defaults.
	//
	// At the defaults on purpose, because that is the number somebody reading
	// this column is about to ask a run for. The floor is a function of the
	// settings - zero-head's floor IS its bytes value - so a person who raises
	// the setting raises this, and the refusal names the new number rather
	// than this one. tfg formats made the other choice once and printed a
	// floor that runs refused: measured 2026-08-03, pdf announced 3265 and
	// took 3286.
	SmallestFileBytes int64           `json:"smallest_file_bytes"`
	Parameters        []propertyEntry `json:"parameters,omitempty"`
}

// damageEntryFor is one damage as a script sees it.
func damageEntryFor(d damage.Descriptor) damageEntry {
	params := make([]propertyEntry, 0, len(d.Parameters))
	for _, p := range d.Parameters {
		params = append(params, propertyEntry{
			Name: p.Name, Kind: string(p.Kind), Min: p.Min, Max: p.Max,
			Unit: p.Unit, Choices: p.Choices, Default: p.Default, Detail: p.Detail,
		})
	}
	return damageEntry{
		ID: d.ID, Detail: d.Detail,
		SmallestFileBytes: defaultFloor(d), Parameters: params,
	}
}

// defaultFloor is the smallest file this damage takes when nobody states a
// setting.
//
// Asked of the declaration rather than worked out here, so a damage whose floor
// is arithmetic on two settings answers for itself.
func defaultFloor(d damage.Descriptor) int64 {
	return d.Floor(d.Defaults())
}

// damageExample is the flag a person would type to use this damage, with its
// first setting at the declared default.
//
// Built from the declaration rather than written out, because the colon and
// comma syntax is the one thing about this flag nobody guesses, and an example
// naming a setting that no longer exists teaches the wrong thing twice.
func damageExample(d damage.Descriptor) string {
	out := "--damage " + d.ID
	if len(d.Parameters) > 0 {
		p := d.Parameters[0]
		value := p.Default
		if value == "" {
			value = "<value>"
		}
		out += ":" + p.Name + "=" + value
	}
	return out
}

// describeOneDamage prints everything one damage declares.
//
// The sentence comes first and the settings after it, which is the other way
// round from a format: a format id says what the file will be and a damage id
// does not, so the sentence is the answer rather than the footnote.
func describeOneDamage(d damage.Descriptor, out io.Writer) {
	fmt.Fprintf(out, "%s - smallest file %s with the settings below\n",
		d.ID, core.ExactBytes(defaultFloor(d)))
	fmt.Fprintf(out, "  %s\n", d.Detail)

	if len(d.Parameters) == 0 {
		fmt.Fprint(out, "\nThis damage takes no settings.\n")
		return
	}
	fmt.Fprintf(out, "\nsettings, written after a colon: %s\n", damageExample(d))
	for _, p := range d.Parameters {
		fmt.Fprintf(out, "  %-14s %s\n", p.Name, p.Allowed())
		if p.Detail != "" {
			fmt.Fprintf(out, "  %-14s %s\n", "", p.Detail)
		}
	}
}

// settingsColumn is the names of what a damage takes, for the list.
//
// A word rather than an empty cell when there are none, because a blank column
// reads as "not filled in yet" and this is an answer.
func settingsColumn(d damage.Descriptor) string {
	names := d.ParameterNames()
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func listDamages(out io.Writer) {
	fmt.Fprintf(out, "%-14s %-14s %s\n", "DAMAGE", "SMALLEST FILE", "SETTINGS")
	for _, d := range damage.All() {
		fmt.Fprintf(out, "%-14s %-14s %s\n",
			d.ID, core.ExactBytes(defaultFloor(d)), settingsColumn(d))
		// The sentence under the row rather than in a column of its own. A
		// damage id is not self describing the way a format id is - nobody
		// reads "zero-head" and knows what comes out - so a list of names
		// would say no more than the --damage flag help already says.
		fmt.Fprintf(out, "    %s\n", d.Detail)
	}
	fmt.Fprint(out, "\nRun \"tfg damage <id>\" for what one damage takes.\n")
	fmt.Fprint(out, "The smallest file is what the default settings need.\n")
}

func damageUsage(w io.Writer, fs *flag.FlagSet, errOut io.Writer) {
	fmt.Fprint(w, `tfg damage - list the ways this build can break a file on purpose.

A damaged file comes out exactly the size you asked for and is one a reader
refuses, so the manifest records it as expected to be rejected. Use them with
"tfg generate --damage <id>" or with the damage key of a recipe.

Usage:
  tfg damage
  tfg damage --json
  tfg damage <id>

Flags:
`)
	fs.SetOutput(w)
	fs.PrintDefaults()
	fs.SetOutput(errOut)
}

// everyDamageEntry is the whole registry as a script sees it.
func everyDamageEntry() []damageEntry {
	all := damage.All()
	list := make([]damageEntry, 0, len(all))
	for _, d := range all {
		list = append(list, damageEntryFor(d))
	}
	return list
}

// damageCmd answers what this build can break and what each one takes.
//
// It exists because the window drew the settings of a chosen damage from the
// declaration and the command line named the damages and stopped there: to
// learn that zero-head takes bytes, and that bytes runs from 4 to 4096, the
// only way was to type a wrong value and read the refusal. The parity guard
// does not see that gap, because it counts recipe keys, formats and presets and
// a damage parameter is none of the three. Written up in
// docs/CORRUPTION-ARCHITECTURE-2026-09-08.md section 14.5.
func damageCmd(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("damage", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the list as JSON to standard output")
	fs.Usage = func() { damageUsage(errOut, fs, errOut) }
	if helpRequested(args) {
		damageUsage(out, fs, errOut)
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
		if *asJSON {
			return renderJSON(everyDamageEntry(), out, errOut)
		}
		listDamages(out)
		return ExitOK
	}

	d, err := damage.Get(wanted)
	if err != nil {
		// The registry already names the damages it knows, so the message is
		// written once rather than here as well.
		fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
		return classify(err)
	}
	if *asJSON {
		return renderJSON([]damageEntry{damageEntryFor(d)}, out, errOut)
	}
	describeOneDamage(d, out)
	return ExitOK
}

// atMostOneName takes the single id a listing command was asked about.
//
// Two names are refused rather than answered about the first. Silence about an
// argument somebody typed is the shape untouchable rule 6 forbids: measured on
// 2026-09-09, "tfg formats png svg" describes png, ignores svg and ends with
// zero, so a script asking about the wrong thing gets a confident answer about
// something else. This command does not repeat that.
func atMostOneName(leading string, fs *flag.FlagSet, errOut io.Writer) (string, bool) {
	extra := fs.Args()
	switch {
	case leading == "" && len(extra) == 0:
		return "", true
	case leading == "" && len(extra) == 1:
		return extra[0], true
	case leading != "" && len(extra) == 0:
		return leading, true
	}
	names := append([]string{}, extra...)
	if leading != "" {
		names = append([]string{leading}, names...)
	}
	fmt.Fprintf(errOut, "tfg: asked about %s at once, and this describes one at a time. "+
		"Run it again with a single name, or with none for the whole list.\n", strings.Join(names, ", "))
	return "", false
}
