// Package geojson generates GeoJSON documents (RFC 7946): a FeatureCollection
// of features, each with a geometry and a set of properties.
//
// What makes this format worth its place is what a map application does with
// a file that is valid and still hard: every one of the seven geometry types,
// a million points in one outline, coordinates with fifteen decimal places, a
// height on every position, an outline wound the other way round, a shape that
// crosses the 180th meridian without being cut. RFC 7946 tells readers to
// accept all of them and readers do not agree on what they mean.
// docs/GEOJSON-2026-10-07.md has the measurement this is built on.
package geojson

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const generatorVersion = "1"

// The padding goes where the json format puts it, for the reason given there:
// whole features fill the file, and the note of the last one takes the
// remainder, so a large file is a large amount of geometry rather than a small
// one with a long tail. The one difference is noteCap - see feature.go.

func init() {
	format.Register(format.Descriptor{
		ID:          "geojson",
		Name:        "GeoJSON",
		Extension:   ".geojson",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty FeatureCollection is legal GeoJSON, and nobody orders one
		// by naming a byte count - the minimum is the collection and one whole
		// feature. The DEFAULT settings' minimum, the way json declares its
		// default layout's. Every other combination answers for itself, through
		// the same refusal, and SmallestAccepted asks the generator.
		MinBytes: minimumBytes(defaultSettings()),

		Padding: format.PaddingChannel{
			Name:     "the note value of the last feature, then spaces after it once the note reaches 1 MiB",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label never reaches the content. An extra member changes the
		// structure under test, the reason json gives too. The file name and
		// the manifest carry it instead.
		Label:            format.LabelExternalOnly,
		Oracle:           "gdal-geojson",
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
		panic(fmt.Sprintf("geojson: the default settings do not parse: %v", err))
	}
	return s
}

// minimumBytes is what opens the collection and the largest closing feature
// the settings can draw, measured rather than written down.
func minimumBytes(s settings) int64 {
	return int64(len(s.layout.prologue)) + newRecords(s).Shortest()
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	s, err := parse(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	if err := refusal(s); err != nil {
		return format.Plan{}, err
	}
	if min := minimumBytes(s); r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "GeoJSON",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    core.Says("geojson.ACollectionHoldsWholeFeatures", "a collection holds whole features, and the largest one these settings can draw needs that much"),
			Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", min)),
		}
	}
	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":   "utf-8",
			"root":       "FeatureCollection",
			Geometry:     s.geometry,
			Formatting:   s.layout.name,
			Precision:    s.precision,
			Altitude:     s.altitude,
			Vertices:     s.vertices,
			Holes:        s.holes,
			Winding:      s.winding(),
			Antimeridian: s.antimeridian,
			Unlocated:    s.unlocated,
			IDs:          s.ids,
			BBox:         s.bbox,
			// Stated even though it is always false here, so a test can assert
			// on it without knowing which formats carry a label internally.
			format.PropertyLabelEmbedded: false,
		},
		Memo: memo{seed: r.Seed, s: s},
	}, nil
}

// refusal is why the globe has no room for what these settings draw, or nil.
// Holes ask for nothing when no outline is drawn, and nothing asks for room
// when every feature is unlocated.
func refusal(s settings) error {
	g := gridFor(s.precision)
	if most := mostPoints(s.kinds, g); int64(s.vertices) > most {
		return refuseCrowded(s, most)
	}
	if s.holes == 0 || !draws(s.kinds, pieceRing) {
		return nil
	}
	if most := mostHoles(g); s.vertices < minVerticesWithHoles || int64(s.holes) > most {
		return refuseHoles(s, most)
	}
	return nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("geojson: the plan was not produced by this generator"))
	}
	prologue := m.s.layout.prologue
	if err := core.WriteAll(w, []byte(prologue)); err != nil {
		return err
	}
	return core.FillRecords(ctx, w, core.NewRand(m.seed), p.Bytes-int64(len(prologue)), newRecords(m.s))
}
