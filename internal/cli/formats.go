// Part of package cli. See cli.go.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

type formatEntry struct {
	ID          string `json:"id"`
	Extension   string `json:"extension"`
	Fidelity    string `json:"fidelity"`
	Determinism string `json:"determinism"`
	// MinBytes is the structural floor of the format: its skeleton, with no
	// label and every setting at its default. Kept under the name it has
	// always had, because a key in machine output is a public name.
	MinBytes int64 `json:"min_bytes"`
	// SmallestAccepted is the smallest --size this build will actually take
	// for an ordinary run, with the label on. For three formats out of twelve
	// that is not MinBytes, and asking for MinBytes was refused.
	SmallestAccepted int64           `json:"smallest_accepted"`
	Padding          string          `json:"padding_channel"`
	PaddingCap       int64           `json:"padding_capacity,omitempty"`
	Label            string          `json:"label_carrier"`
	Properties       []propertyEntry `json:"properties,omitempty"`
	Oracle           string          `json:"oracle"`
	Version          string          `json:"generator_version"`
}

// propertyEntry is one setting as a script sees it. Everything a window would
// need to draw a field is here, which is the point of describing properties
// rather than only naming them.
type propertyEntry struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Min     int64    `json:"min,omitempty"`
	Max     int64    `json:"max,omitempty"`
	Unit    string   `json:"unit,omitempty"`
	Choices []string `json:"choices,omitempty"`
	Default string   `json:"default,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}

// smallestAccepted is the number this command prints as the minimum.
//
// It is what an ordinary run will take, so the label is on and every property
// is at its default - which is what somebody reading that column is about to
// type. The registry's own MinBytes is the structural floor with no label, and
// for pdf, wav and zip the two are not the same number: asking for the floor
// was refused, measured on 2026-08-03.
func smallestAccepted(d format.Descriptor) int64 {
	return d.SmallestAccepted(format.Request{Label: true})
}

func entryFor(d format.Descriptor) formatEntry {
	props := make([]propertyEntry, 0, len(d.Properties))
	for _, p := range d.Properties {
		props = append(props, propertyEntry{
			Name: p.Name, Kind: string(p.Kind), Min: p.Min, Max: p.Max,
			Unit: p.Unit, Choices: p.Choices, Default: p.Default, Detail: p.Detail,
		})
	}
	return formatEntry{
		ID: d.ID, Extension: d.Extension,
		Fidelity: string(d.Fidelity), Determinism: string(d.Determinism),
		MinBytes: d.MinBytes, SmallestAccepted: smallestAccepted(d),
		Padding: d.Padding.Name, PaddingCap: d.Padding.Capacity,
		Label: string(d.Label), Properties: props,
		Oracle: d.Oracle, Version: d.GeneratorVersion,
	}
}

// describeOne prints everything one format declares.
//
// "tfg formats pdf" is documented in docs/CLI.md and used to print the whole
// list and ignore the argument, ending with 0 - so there was no way to ask what
// a format accepts, and the silence looked like an answer.
func describeOne(d format.Descriptor, out io.Writer) {
	fmt.Fprintf(out, "%s - %s fidelity, %s deterministic, minimum %d B\n",
		d.ID, d.Fidelity, d.Determinism, smallestAccepted(d))
	fmt.Fprintf(out, "  extension  %s\n", d.Extension)
	fmt.Fprintf(out, "  padding    %s\n", d.Padding.Name)
	fmt.Fprintf(out, "  label      %s\n", d.Label)
	fmt.Fprintf(out, "  oracle     %s\n", d.Oracle)

	if len(d.Properties) == 0 {
		fmt.Fprint(out, "\nThis format takes no properties.\n")
		return
	}
	fmt.Fprint(out, "\nproperties, set with --set name=value:\n")
	for _, p := range d.Properties {
		fmt.Fprintf(out, "  %-14s %s\n", p.Name, allowedText(p))
		if p.Detail != "" {
			fmt.Fprintf(out, "  %-14s %s\n", "", p.Detail)
		}
	}
}

// allowedText says what a property accepts, in the same words the refusal uses.
func allowedText(p format.Property) string {
	var what string
	switch p.Kind {
	case format.PropertyInt:
		unit := ""
		if p.Unit != "" {
			unit = " of " + p.Unit
		}
		if p.Min != 0 || p.Max != 0 {
			what = fmt.Sprintf("whole number%s from %d to %d", unit, p.Min, p.Max)
		} else {
			what = "whole number" + unit
		}
	case format.PropertyChoice:
		what = "one of: " + strings.Join(p.Choices, ", ")
	case format.PropertyBool:
		what = "true or false"
	case format.PropertySize:
		what = "a size such as 2mb, or a plain byte count"
	default:
		what = "text"
	}
	if p.Default != "" {
		what += ", default " + p.Default
	}
	return what
}

func formats(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("formats", flag.ContinueOnError)
	fs.SetOutput(errOut)
	asJSON := fs.Bool("json", false, "write the list as JSON to standard output")
	usage := func(w io.Writer) {
		fmt.Fprint(w, `tfg formats - list the formats this build supports.

Says three things a request depends on and that nobody can guess: how faithful
the file will be, whether it repeats to the byte, and how small it can go.

Usage:
  tfg formats
  tfg formats --json

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
	leading, rest := splitLeadingPath(args)
	if err := fs.Parse(rest); err != nil {
		return ExitUsage
	}
	wanted := leading
	if wanted == "" && fs.NArg() == 1 {
		wanted = fs.Arg(0)
	}

	if wanted != "" {
		d, err := format.Get(wanted)
		if err != nil {
			// The registry already names the formats it does know, so the
			// message stays where it is rather than being written twice.
			fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
			return classify(err)
		}
		if *asJSON {
			return renderJSON([]formatEntry{entryFor(d)}, out, errOut)
		}
		describeOne(d, out)
		return ExitOK
	}

	if *asJSON {
		list := make([]formatEntry, 0, len(format.All()))
		for _, d := range format.All() {
			list = append(list, entryFor(d))
		}
		return renderJSON(list, out, errOut)
	}

	fmt.Fprintf(out, "%-8s %-10s %-12s %-10s %s\n", "FORMAT", "FIDELITY", "DETERMINISM", "MINIMUM", "PADDING CHANNEL")
	for _, d := range format.All() {
		fmt.Fprintf(out, "%-8s %-10s %-12s %-10d %s\n",
			d.ID, d.Fidelity, d.Determinism, smallestAccepted(d), d.Padding.Name)
	}
	fmt.Fprint(out, "\nRun \"tfg formats <id>\" for what one format accepts.\n")
	return ExitOK
}

func renderJSON(v any, out, errOut io.Writer) int {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(errOut, "tfg: cannot render the list: %s\n", describeError(err))
		return ExitRuntime
	}
	return ExitOK
}

// classify turns an error into an exit code.
//
// The mapping lives here and nowhere else, and it works on error types rather
// than on message text. Anything unrecognised becomes a runtime error, which
// is the honest answer for a failure the tool did not anticipate.
// describeError renders an error for a person, in English, whatever language
// the operating system speaks.
//
// Every message this tool prints is English. A wrapped operating system error
// breaks that with nothing noticing, because the system formats its messages in
// the language of the machine - "Incorrect function." reaches somebody on a
// Polish install as a Polish sentence. The guard that scans this binary for non
// English text cannot see it, since that text is not in the binary at all. It
// arrives at run time.
//
// So the system's sentence is swapped for ours and every layer of our own
// context above it is kept. The number it carried stays, because a number means
// the same thing in every language and it is what somebody puts into a search.
