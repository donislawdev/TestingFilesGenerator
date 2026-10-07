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
	Holes        = "holes"
	Winding      = "winding"
	Antimeridian = "antimeridian"
	Unlocated    = "unlocated"
	IDs          = "ids"
	BBox         = "bbox"
)

// The values of unlocated: how many features have no geometry. None is shared
// with ids, which has no features without an id the same way.
const (
	None = "none"
	Some = "some"
	All  = "all"
)

// The values of ids - the two types RFC 7946 allows, both of them in turn, and
// no member at all. Mixed is the word geometry takes for the same idea.
const (
	Number = "number"
	String = "string"
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

	// An outline keeps its middle free for holes only with two points above
	// it and two below - see pierced.
	minVerticesWithHoles = 6
	// A hundred thousand holes is half a million positions in one polygon,
	// a few megabytes - the kind of land cover outline that slows a reader
	// down, at a size a test still writes in seconds.
	maxHoles = 100_000

	// unlocatedEvery is how often a feature has no geometry with unlocated
	// set to some. Five shares no factor with the seven kinds mixed takes in
	// turn, so each kind loses a feature now and then rather than one kind
	// every time, and the first feature always has a place.
	unlocatedEvery = 5
)

// settings is every choice a file is made with, read once in Plan.
type settings struct {
	geometry     string
	kinds        []kind
	layout       layout
	precision    int
	altitude     bool
	vertices     int
	holes        int
	reversed     bool
	antimeridian bool
	unlocated    string
	ids          string
	bbox         bool
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
	// All three have passed their declared range in valueOf, so none can fail.
	precision, _ := strconv.Atoi(values[Precision])
	vertices, _ := strconv.Atoi(values[Vertices])
	holes, _ := strconv.Atoi(values[Holes])
	return settings{
		geometry:     values[Geometry],
		kinds:        kindsOf(values[Geometry], values[Unlocated]),
		layout:       layouts[values[Formatting]],
		precision:    precision,
		altitude:     values[Altitude] == "true",
		vertices:     vertices,
		holes:        holes,
		reversed:     values[Winding] == Reversed,
		antimeridian: values[Antimeridian] == "true",
		unlocated:    values[Unlocated],
		ids:          values[IDs],
		bbox:         values[BBox] == "true",
	}, nil
}

// kindAt is the kind feature number n carries. The kind follows the number,
// so a feature thrown away and drawn again is the kind it would have been,
// and so is a feature with no geometry - the kinds after it keep their turns.
func (s settings) kindAt(n int64) kind {
	if s.unlocated == Some && n%unlocatedEvery == 0 {
		return kindNone
	}
	return s.kinds[(n-1)%int64(len(s.kinds))]
}

// idAt is how feature number n writes its id. With mixed the odd numbers are
// numbers and the even ones text.
func (s settings) idAt(n int64) string {
	if s.ids == Mixed {
		if n%2 == 0 {
			return String
		}
		return Number
	}
	return s.ids
}

// dims is how many axes every position has.
func (s settings) dims() int {
	if s.altitude {
		return 3
	}
	return 2
}

// widestID is the id form that takes the most bytes for the same number, for
// the smallest closing feature, which has to hold for every feature.
func (s settings) widestID() string {
	if s.ids == Mixed {
		return String
	}
	return s.ids
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
// meets it: what is drawn first, then the pitfalls and the members around it.
func properties() []format.Property {
	return append(drawingProperties(), pitfallProperties()...)
}

// drawingProperties are the settings of what a feature is drawn as and how
// the document is laid out.
func drawingProperties() []format.Property {
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
			Name: Holes, Kind: format.PropertyInt,
			Min: 0, Max: maxHoles, Unit: "holes",
			Default: "0",
			Detail:  "How many holes every polygon has inside its outline. Each hole has four points and runs the other way round to its outline, as RFC 7946 asks. Holes need 6 or more vertices, fewer of them fit at precision 3 or less, and points and lines do not change.",
		},
	}
}

// pitfallProperties are the settings readers disagree about: which way an
// outline runs, a shape across the antimeridian, a feature with no place, the
// type of an id and the bbox.
func pitfallProperties() []format.Property {
	return []format.Property{
		{
			Name: Winding, Kind: format.PropertyChoice,
			Choices: []string{Reversed, RFC7946},
			Default: RFC7946,
			Detail:  "Which way polygon outlines run - rfc7946 is counter-clockwise and reversed is clockwise, and holes always run the other way. RFC 7946 tells readers to accept both, and they disagree about which side of the outline is inside. Points and lines do not change.",
		},
		{
			Name: Antimeridian, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Whether every line and outline crosses the 180th meridian without being cut in two. RFC 7946 asks for such shapes to be cut, and readers differ - read as flat coordinates, a crossing outline crosses itself and runs the other way round. A single point cannot cross and is placed beside it.",
		},
		{
			Name: Unlocated, Kind: format.PropertyChoice,
			Choices: []string{All, None, Some},
			Default: None,
			Detail:  "How many features have no place - RFC 7946 writes their geometry as null. some takes the place of every fifth feature, from the fifth on, and all takes it from every one, so the settings that shape a geometry change nothing.",
		},
		{
			Name: IDs, Kind: format.PropertyChoice,
			Choices: []string{Mixed, None, Number, String},
			Default: Number,
			Detail:  "Which type every feature id has. RFC 7946 allows a number or a string, mixed gives odd features a number and even ones a string such as f2, and none leaves the id out. Readers that turn the id into a key may keep only one of the two types.",
		},
		{
			Name: BBox, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Whether the collection and every feature with a place carry a bbox, the box their coordinates lie in. The collection's bbox follows its features, because its extent is known only after the last one, and a box across the antimeridian has a west edge greater than its east edge, as RFC 7946 asks.",
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

// refuseHoles is the refusal for holes an outline cannot hold: too few points
// to keep its middle free, or more holes than the globe has room for at the
// precision asked for. Keyed by holes, the number the person added last and
// the one the reason talks about. The advice is in the reason as well as on
// its own, for the reason refuseCrowded gives.
func refuseHoles(s settings, most int64) error {
	err := &format.PropertyValueError{Format: "geojson", Key: Holes, Value: strconv.Itoa(s.holes)}
	if s.vertices < minVerticesWithHoles {
		err.Reason = core.Says("geojson.HolesNeedSixPoints",
			"an outline keeps its middle free for holes only with two points above it and two below - ask for %d vertices or more, or for no holes",
			core.A("Least", minVerticesWithHoles))
		err.Remedy = core.Says("geojson.AskForMorePointsOrNoHoles",
			"Ask for %d vertices or more, or for no holes.", core.A("Least", minVerticesWithHoles))
		return err
	}
	err.Reason = core.Says("geojson.EachHoleNeedsSteps",
		"each hole needs four steps of longitude and an outline with holes spans at most half the globe, so at precision %d it has room for %d - ask for that many or fewer, or for a higher precision",
		core.A("Precision", s.precision), core.A("Most", most))
	err.Remedy = core.Says("geojson.AskForFewerHoles",
		"Ask for %d holes or fewer, or for a higher precision.", core.A("Most", most))
	return err
}
