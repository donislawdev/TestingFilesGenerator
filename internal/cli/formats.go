// Part of package cli. See cli.go.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

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
			fmt.Fprintf(errOut, "tfg: cannot render the list: %s\n", describeError(err))
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
