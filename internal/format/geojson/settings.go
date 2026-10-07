package geojson

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Setting names. Public names, so each is spelled once.
const (
	Geometry     = "geometry"
	Formatting   = "formatting"
	Precision    = "precision"
	Altitude     = "altitude"
	Vertices     = "vertices"
	Winding      = "winding"
	Antimeridian = "antimeridian"
)

// The values of geometry, spelled as a recipe spells them: the RFC 7946 type
// names in lower case, and mixed for all seven in turn.
const (
	Mixed              = "mixed"
	Point              = "point"
	LineString         = "linestring"
	Polygon            = "polygon"
	MultiPoint         = "multipoint"
	MultiLineString    = "multilinestring"
	MultiPolygon       = "multipolygon"
	GeometryCollection = "geometrycollection"
)

// The values of winding.
const (
	RFC7946  = "rfc7946"
	Reversed = "reversed"
)

// The values of formatting - the same three names the json format takes, so a
// person who learnt them there meets the same words here, and a window keeps
// one translation of each.
const (
	Indented      = "indented"
	Minified      = "minified"
	RecordPerLine = "record-per-line"
)

const (
	defaultPrecision = 6
	maxPrecision     = 15

	defaultVertices = 8
	// Three is the fewest that makes an outline - a triangle - and a line of
	// three still bends.
	minVertices = 3
	// A million points in one ring is the size that breaks things: measured
	// 2026-10-07, one such ring is 16 MB of text, read by GDAL in one feature.
	maxVertices = 1_000_000
)

// settings is every choice a file is made with, read once in Plan.
type settings struct {
	geometry     string
	kinds        []kind
	layout       layout
	precision    int
	altitude     bool
	vertices     int
	reversed     bool
	antimeridian bool
}

// parse reads the settings a request carries.
//
// A value outside the declaration has already been refused by the registry,
// which checks every format in one place. The check stays here for the reason
// the json layout keeps its own: the function is callable directly, a guard is
// such a caller, and a generator that trusts its input is one registry change
// away from writing a file nobody ordered.
//
// Read by name rather than by position in the declaration, so reordering the
// list a person sees cannot hand one setting's value to another.
func parse(props map[string]string) (settings, error) {
	values := map[string]string{}
	for _, p := range properties() {
		v, err := valueOf(props, p)
		if err != nil {
			return settings{}, err
		}
		values[p.Name] = v
	}
	// Both have passed their declared range in valueOf, so neither can fail.
	precision, _ := strconv.Atoi(values[Precision])
	vertices, _ := strconv.Atoi(values[Vertices])
	return settings{
		geometry:     values[Geometry],
		kinds:        kindsOf(values[Geometry]),
		layout:       layouts[values[Formatting]],
		precision:    precision,
		altitude:     values[Altitude] == "true",
		vertices:     vertices,
		reversed:     values[Winding] == Reversed,
		antimeridian: values[Antimeridian] == "true",
	}, nil
}

// winding is the value of winding the file was made with.
func (s settings) winding() string {
	if s.reversed {
		return Reversed
	}
	return RFC7946
}

// valueOf is one setting's value, or its default, refused in the
// declaration's own words when the declaration does not allow it.
func valueOf(props map[string]string, p format.Property) (string, error) {
	v, ok := props[p.Name]
	if !ok || v == "" {
		return p.Default, nil
	}
	if bad := p.Allows(v); !bad.IsZero() {
		return "", &format.PropertyValueError{Format: "geojson", Key: p.Name, Value: v, Reason: bad, Remedy: p.Instead()}
	}
	return v, nil
}

// properties is the declaration, in the order parse reads it and a person
// meets it.
func properties() []format.Property {
	return []format.Property{
		{
			Name: Geometry, Kind: format.PropertyChoice,
			// In order, as every closed set is registered. The turns mixed
			// takes follow RFC 7946 instead - see everyKind.
			Choices: []string{GeometryCollection, LineString, Mixed, MultiLineString, MultiPoint, MultiPolygon, Point, Polygon},
			Default: Mixed,
			Detail:  "Which kind of geometry each feature carries. With mixed the seven kinds take turns, so a file of seven features or more holds every one of them.",
		},
		{
			Name: Formatting, Kind: format.PropertyChoice,
			Choices: []string{Indented, Minified, RecordPerLine},
			Default: RecordPerLine,
			Detail:  "How the document is laid out. Every reader accepts all three - minified puts the whole file on one line and ends without a newline, and indented puts every number on its own line, which makes the same features several times larger.",
		},
		{
			Name: Precision, Kind: format.PropertyInt,
			Min: 0, Max: maxPrecision, Unit: "decimal places",
			Default: strconv.Itoa(defaultPrecision),
			Detail:  "How many decimal places every coordinate is written with. Six is about ten centimetres on the ground, the figure RFC 7946 gives.",
		},
		{
			Name: Altitude, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Whether every position carries a third number, the height in metres from -400 to 8848. A program that reads two numbers a position has to ignore it or refuse the file.",
		},
		{
			Name: Vertices, Kind: format.PropertyInt,
			Min: minVertices, Max: maxVertices, Unit: "points",
			Default: strconv.Itoa(defaultVertices),
			Detail:  "How many points every line, outline and MultiPoint has. A feature larger than 1 MiB is followed by spaces that fill the last bytes, and at precision 3 or less fewer points fit, because each needs its own step of longitude.",
		},
		{
			Name: Winding, Kind: format.PropertyChoice,
			Choices: []string{Reversed, RFC7946},
			Default: RFC7946,
			Detail:  "Which way polygon outlines run - rfc7946 is counter-clockwise and reversed is clockwise. RFC 7946 tells readers to accept both, and they disagree about which side of the outline is inside. Points and lines do not change.",
		},
		{
			Name: Antimeridian, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Whether every line and outline crosses the 180th meridian without being cut in two. RFC 7946 asks for such shapes to be cut, and readers differ - read as flat coordinates, a crossing outline crosses itself and runs the other way round. A single point cannot cross and is placed beside it.",
		},
	}
}

// refuseCrowded is the refusal for more points than the globe has steps of
// longitude at the precision asked for.
//
// Not a JointLimit, because a JointLimit is a product of two settings and this
// is a count against a power of ten. Said in the same four parts all the same,
// with what to do inside the reason the way the picture formats say theirs,
// because the reason is the part the command line prints.
//
// Keyed by vertices alone, so a window puts it under the box that holds the
// number to change.
func refuseCrowded(s settings, most int64) error {
	return &format.PropertyValueError{
		Format: "geojson", Key: Vertices, Value: strconv.Itoa(s.vertices),
		Reason: core.Says("geojson.EachPointNeedsAStep",
			"each point needs its own step of longitude, and at precision %d the globe has room for %d - ask for that many or fewer, or for a higher precision",
			core.A("Precision", s.precision), core.A("Most", most)),
		// The same advice again on its own, for the reader that asks for the
		// parts by name - tfg validate --json puts it under fix, which was
		// empty without it (review of #171). The command line prints the
		// reason, so the advice stays there as well.
		Remedy: core.Says("geojson.AskForFewerPoints",
			"Ask for %d points or fewer, or for a higher precision.", core.A("Most", most)),
	}
}
