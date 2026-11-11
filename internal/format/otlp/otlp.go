// Package otlp generates OpenTelemetry data in the JSON encoding of OTLP: a
// file of spans or of log records, laid out as the OTLP file exporter writes
// it or as one request for an OTLP endpoint.
//
// What makes this format worth its place is how quietly its readers disagree.
// Measured on 2026-11-10 (docs/OTLP-2026-11-10.md section 3): the file
// receiver of the OpenTelemetry Collector drops a line longer than 1 MiB
// without a word in its log, its HTTP receiver answers 200 to a file of
// several lines and keeps the first, and the JSON parser of the protobuf
// library reads a correct trace id as 24 bytes of something else. A file that
// is valid and exactly as large as asked is how a tester finds out which of
// those the system under test does.
//
// Every file is written the way the Collector writes one back, key for key
// and byte for byte, so the reference reader can be asked for more than "did
// you read it" - see section 5 of the same document.
package otlp

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const generatorVersion = "1"

func init() {
	format.Register(format.Descriptor{
		ID:        "otlp",
		Name:      "OpenTelemetry Protocol JSON",
		Extension: ".jsonl",
		// jsonlines.org names this type and says it is not registered yet. The
		// request body of OTLP over HTTP is application/json, and a file of
		// several lines is not one JSON document, so that type would be wrong
		// for the default layout.
		MediaType:   "application/jsonl",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The specification asks senders not to write an envelope with nothing
		// in it, so the smallest file is one batch holding one record. The
		// DEFAULT settings' minimum, the way json and geojson declare theirs.
		MinBytes: minimumBytes(defaultSettings()),

		// Whole records fill the file and the padding attribute of the last
		// one takes the remainder, which is always shorter than a record. One
		// long value would be the wrong channel twice over: a line longer than
		// 1 MiB is dropped by the file receiver, and a backend may cut a long
		// attribute short.
		Padding: format.PaddingChannel{
			Name:     "the " + paddingKey + " attribute of the last span or log record",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label never reaches the content. An attribute more on every
		// record changes what a receiver stores, so the file name and the
		// manifest carry it instead.
		Label:            format.LabelExternalOnly,
		Oracle:           "otelcol",
		Properties:       properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	seed uint64
	s    settings
}

func defaultSettings() settings {
	s, err := parse(nil)
	if err != nil {
		panic(fmt.Sprintf("otlp: the default settings do not parse: %v", err))
	}
	return s
}

// minimumBytes is one batch holding the longest closing record the settings
// can write, measured rather than written down.
func minimumBytes(s settings) int64 { return newRecords(s).Shortest() }

func (generator) Plan(r format.Request) (format.Plan, error) {
	s, err := parse(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	if min := minimumBytes(s); r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "OTLP",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    core.Says("otlp.AFileHoldsWholeRecords", "a file holds whole records, and the longest one these settings can write needs that much"),
			Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", min)),
		}
	}
	if err := s.endsInTime(r.Bytes); err != nil {
		return format.Plan{}, err
	}
	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties:  s.recorded(),
		Memo:        memo{seed: r.Seed, s: s},
	}, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("otlp: the plan was not produced by this generator"))
	}
	// No prologue of its own: what opens a batch is written with the first
	// record of that batch, because in the default layout every line opens
	// one.
	return core.FillRecords(ctx, w, core.NewRand(m.seed), p.Bytes, newRecords(m.s))
}
