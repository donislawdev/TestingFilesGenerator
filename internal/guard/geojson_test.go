package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/geojson"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// geojsonCases is every value of every setting, one at a time, and the
// combinations measured on 2026-10-07 to be the ones where the drawing is at
// its tightest: three points across the antimeridian, a line of the most points
// a whole-degree grid has room for, and every pitfall at once in the layout
// that puts each number on its own line.
var geojsonCases = []map[string]string{
	{},
	{geojson.Geometry: geojson.Point},
	{geojson.Geometry: geojson.LineString},
	{geojson.Geometry: geojson.Polygon},
	{geojson.Geometry: geojson.MultiPoint},
	{geojson.Geometry: geojson.MultiLineString},
	{geojson.Geometry: geojson.MultiPolygon},
	{geojson.Geometry: geojson.GeometryCollection},
	{geojson.Formatting: geojson.Minified},
	{geojson.Formatting: geojson.Indented},
	{geojson.Precision: "0"},
	{geojson.Precision: "15"},
	{geojson.Altitude: "true"},
	{geojson.Vertices: "3"},
	{geojson.Vertices: "4"},
	{geojson.Vertices: "5"},
	{geojson.Winding: geojson.Reversed},
	{geojson.Antimeridian: "true"},
	{geojson.Antimeridian: "true", geojson.Winding: geojson.Reversed, geojson.Altitude: "true", geojson.Formatting: geojson.Indented},
	// Fifteen places and a height: the span of heights is then 9248 * 10^15
	// steps, past the largest int64, and the first build stopped every such run
	// with an internal error (found 2026-10-07 by the golden value probe, with
	// every case here green).
	{geojson.Precision: "15", geojson.Altitude: "true"},
	{geojson.Vertices: "3", geojson.Antimeridian: "true"},
	{geojson.Precision: "0", geojson.Vertices: "359", geojson.Geometry: geojson.LineString},
	{geojson.Precision: "0", geojson.Antimeridian: "true"},

	// The second set of settings (docs/GEOJSON-2-2026-10-07.md): holes, features
	// with no place, the type of the id and the bbox - each alone, then where
	// they were measured to be tightest.
	{geojson.Holes: "1"},
	{geojson.Holes: "3", geojson.Geometry: geojson.Polygon, geojson.Vertices: "6"},
	{geojson.Holes: "2", geojson.Winding: geojson.Reversed, geojson.Antimeridian: "true", geojson.Altitude: "true"},
	// The most holes a whole-degree grid has room for, in both parts of a
	// multi polygon. 118 of them were first allowed, and an outline of six
	// points across 357 degrees read as crossing the antimeridian.
	{geojson.Holes: "44", geojson.Precision: "0", geojson.Geometry: geojson.MultiPolygon},
	// Fifteen places with holes: a step is less than a double tells apart, and
	// holes first drawn a step apart touched and cut the polygon in two for
	// GEOS (measured 2026-10-07).
	{geojson.Holes: "4", geojson.Precision: "15", geojson.Altitude: "true"},
	{geojson.Unlocated: geojson.Some},
	{geojson.Unlocated: geojson.All, geojson.BBox: "true"},
	{geojson.IDs: geojson.String},
	{geojson.IDs: geojson.Mixed},
	{geojson.IDs: geojson.None},
	{geojson.BBox: "true"},
	{geojson.BBox: "true", geojson.Antimeridian: "true"},
	{geojson.BBox: "true", geojson.Formatting: geojson.Indented, geojson.Altitude: "true"},
	{geojson.BBox: "true", geojson.Formatting: geojson.Minified, geojson.Unlocated: geojson.Some, geojson.IDs: geojson.Mixed},
	{geojson.Holes: "2", geojson.Unlocated: geojson.Some, geojson.IDs: geojson.Mixed, geojson.BBox: "true",
		geojson.Antimeridian: "true", geojson.Formatting: geojson.Indented, geojson.Winding: geojson.Reversed},
}

// Three readers, three questions. GDAL is the reader a GIS tester has, and it
// has to read every feature with nothing on standard error. shapely has to
// call every geometry valid, which GDAL never asks. The structural checker is
// told the settings and has to find the file made the way they say - the
// decimal places, the winding, the crossing, the layout byte for byte. Each of
// them alone was measured letting through what another catches
// (docs/GEOJSON-2026-10-07.md section 3).
//
// Named to match ReferenceTool, because that is the one job in CI with GDAL and
// shapely installed. Anywhere else it skips, and says so.
func TestEveryGeoJSONSettingSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	gdal := mustOracle(t, d)
	dir := t.TempDir()
	for i, props := range geojsonCases {
		smallest := d.SmallestAccepted(format.Request{Properties: props})
		for _, size := range []int64{smallest, smallest + 1, 20000} {
			path := filepath.Join(dir, strconv.Itoa(i)+"_"+strconv.FormatInt(size, 10)+".geojson")
			features := writeGeoJSON(t, path, size, uint64(31+i), props)
			gdalReadsEveryFeature(t, gdal, path, props, features)
			if v := oracle.ShapelyValid(path, props[geojson.Antimeridian] == "true"); !v.Available {
				t.Skipf("%s is not installed, so no geometry was judged - a skip, not a pass", v.Tool)
			} else if v.Err != nil {
				t.Fatalf("%v at %d B: %v", props, size, v.Err)
			}
			if s := oracle.Strict("geojson", path, geojsonSettings(props)...); s.Err != nil {
				t.Fatalf("%v at %d B: %v", props, size, s.Err)
			} else if !s.Available {
				t.Skip("the structural check needs python")
			}
		}
	}
}

// The smallest closing feature is carried forward from three and four points
// rather than measured at a million, so planning a file of a million points
// does not draw seven features of a million points first. Carrying it forward
// is only right while every position of a kind costs the same at its widest -
// and the largest kind at three points is not the largest at a thousand, so
// the two have to agree kind by kind. This measures the whole feature and
// compares.
func TestTheSmallestGeoJSONFeatureCarriedForwardIsTheOneMeasuredWhole(t *testing.T) {
	checked := 0
	for _, geometry := range []string{geojson.Mixed, geojson.Point, geojson.LineString, geojson.Polygon,
		geojson.MultiPoint, geojson.MultiLineString, geojson.MultiPolygon, geojson.GeometryCollection} {
		for _, layout := range []string{geojson.RecordPerLine, geojson.Minified, geojson.Indented} {
			for _, extra := range []string{"3", "4", "5", "6", "9", "100", "5000"} {
				// The settings of the second set take turns at their own pace, so
				// every one of their values meets every geometry and layout.
				props := map[string]string{geojson.Geometry: geometry, geojson.Formatting: layout,
					geojson.Vertices: extra, geojson.Altitude: strconv.FormatBool(checked%2 == 0),
					geojson.Precision: strconv.Itoa(checked % 16),
					geojson.Holes:     []string{"0", "1", "7"}[checked%3],
					geojson.BBox:      strconv.FormatBool(checked/2%2 == 0),
					geojson.IDs:       []string{geojson.Number, geojson.String, geojson.Mixed, geojson.None}[checked%4],
					geojson.Unlocated: []string{geojson.None, geojson.Some, geojson.None, geojson.All, geojson.None}[checked%5]}
				carried, whole, err := geojson.ClosingFeatureBounds(props)
				if err != nil {
					t.Fatalf("%v: %v", props, err)
				}
				if carried != whole {
					t.Errorf("%v: the smallest closing feature carried forward is %d B and measured whole it is %d B",
						props, carried, whole)
				}
				checked++
			}
		}
	}
	t.Logf("%d combinations, each carried forward and measured whole", checked)
}

// GDAL refuses a string of ten million characters (measured 2026-10-07), and a
// feature of a hundred thousand points leaves more than a mebibyte for the note
// of a file one feature long. Above a mebibyte the remainder is spaces after
// the feature. The checker reports how many, and the file has to have reached
// that state - a guard of the cap that never makes a file past it would be
// green for a reason that has nothing to do with the cap.
func TestAGeoJSONNotePastAMebibyteBecomesSpacesAndSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	props := map[string]string{geojson.Geometry: geojson.Polygon, geojson.Vertices: "100000"}
	size := d.SmallestAccepted(format.Request{Properties: props}) + 5<<20/4
	path := filepath.Join(t.TempDir(), "capped.geojson")
	if n := writeGeoJSON(t, path, size, 5, props); n != 1 {
		t.Fatalf("the file holds %d features and this guard needs one, so its note owes the whole remainder", n)
	}
	s := oracle.Strict("geojson", path, geojsonSettings(props)...)
	if !s.Available {
		t.Skip("the structural check needs python")
	}
	if s.Err != nil {
		t.Fatal(s.Err)
	}
	m := regexp.MustCompile(`(\d+) trailing spaces`).FindStringSubmatch(s.Output)
	if m == nil || m[1] == "0" {
		t.Fatalf("the checker reports %q - no spaces after the feature, so the note was never capped", s.Output)
	}
	if res := mustOracle(t, d).Check(path); res.Available && res.Err != nil {
		t.Fatalf("GDAL refused the capped file: %v", res.Err)
	}
}

// A line needs a step of longitude for every point. At a whole degree the
// globe has 360 of them, less one at each side, so a line takes 359 points
// and an outline, which goes out along the bottom and back along the top, 716.
// Points use no vertices and are never refused for them.
func TestGeoJSONRefusesMorePointsThanTheGlobeHasRoomFor(t *testing.T) {
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		geometry string
		most     int
	}{{geojson.LineString, 359}, {geojson.Mixed, 359}, {geojson.Polygon, 716}, {geojson.MultiPolygon, 716}} {
		for _, n := range []int{c.most, c.most + 1} {
			props := map[string]string{geojson.Geometry: c.geometry, geojson.Precision: "0", geojson.Vertices: strconv.Itoa(n)}
			_, err := d.Generator.Plan(format.Request{Bytes: 1 << 30, Properties: props})
			var refused *format.PropertyValueError
			switch {
			case n == c.most && err != nil:
				t.Errorf("%s with %d points at precision 0 was refused, and it fits: %v", c.geometry, n, err)
			case n > c.most && !(errors.As(err, &refused) && refused.Key == geojson.Vertices):
				t.Errorf("%s with %d points at precision 0 was not refused for its points: %v", c.geometry, n, err)
			}
		}
	}
	points := map[string]string{geojson.Geometry: geojson.Point, geojson.Precision: "0", geojson.Vertices: "1000000"}
	if _, err := d.Generator.Plan(format.Request{Bytes: 1 << 20, Properties: points}); err != nil {
		t.Errorf("points use no vertices, and a million of them at precision 0 was refused: %v", err)
	}
}

// A hole takes four steps of longitude and an outline with holes spans at most
// half the globe, so a whole-degree grid has room for 44 and three places for
// 44 999 - the setting's description says fewer fit at precision 3 or less,
// and four places hold the hundred thousand it allows. Holes keep the middle
// of an outline free only with two points above it and two below, so six
// vertices. A geometry with no outline, or no geometry at all, asks for no room
// and is never refused for holes.
func TestGeoJSONRefusesHolesTheOutlineHasNoRoomFor(t *testing.T) {
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		props   map[string]string
		refused bool
	}{
		{map[string]string{geojson.Geometry: geojson.Polygon, geojson.Precision: "0", geojson.Holes: "44"}, false},
		{map[string]string{geojson.Geometry: geojson.Polygon, geojson.Precision: "0", geojson.Holes: "45"}, true},
		{map[string]string{geojson.Geometry: geojson.GeometryCollection, geojson.Precision: "3", geojson.Holes: "44999"}, false},
		{map[string]string{geojson.Geometry: geojson.GeometryCollection, geojson.Precision: "3", geojson.Holes: "45000"}, true},
		{map[string]string{geojson.Precision: "4", geojson.Holes: "100000"}, false},
		{map[string]string{geojson.Vertices: "6", geojson.Holes: "1"}, false},
		{map[string]string{geojson.Vertices: "5", geojson.Holes: "1"}, true},
		{map[string]string{geojson.Geometry: geojson.LineString, geojson.Vertices: "3", geojson.Precision: "0", geojson.Holes: "100000"}, false},
		{map[string]string{geojson.Unlocated: geojson.All, geojson.Vertices: "1000000", geojson.Precision: "0", geojson.Holes: "100000"}, false},
	} {
		_, err := d.Generator.Plan(format.Request{Bytes: 1 << 40, Properties: c.props})
		var refused *format.PropertyValueError
		switch {
		case !c.refused && err != nil:
			t.Errorf("%v was refused, and it fits: %v", c.props, err)
		case c.refused && !(errors.As(err, &refused) && refused.Key == geojson.Holes):
			t.Errorf("%v was not refused for its holes: %v", c.props, err)
		}
	}
}

// A bbox across the antimeridian has its west edge greater than its east edge,
// and one whose shapes reach all the way round is -180 to 180 (RFC 7946
// sections 5.2 and 5.3). The reference tools read every case of the second set
// above with a bbox, and this asks that the two states they exist for were
// reached - the checker says which shape the collection's box has, because a
// guard that never makes a crossing box is green for no reason.
func TestAGeoJSONBoxAcrossTheAntimeridianOrRoundTheGlobeSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	gdal := mustOracle(t, d)
	for _, c := range []struct {
		props map[string]string
		shape string
	}{
		{map[string]string{geojson.BBox: "true", geojson.Antimeridian: "true"}, "crossing"},
		// Whole degrees and lines of 300 points: each line spans 299 degrees
		// across 180, so a few of them together reach round the globe.
		{map[string]string{geojson.BBox: "true", geojson.Antimeridian: "true", geojson.Precision: "0",
			geojson.Vertices: "300", geojson.Geometry: geojson.LineString, geojson.Formatting: geojson.Indented}, "round"},
	} {
		path := filepath.Join(t.TempDir(), c.shape+".geojson")
		features := writeGeoJSON(t, path, 40000, 7, c.props)
		s := oracle.Strict("geojson", path, geojsonSettings(c.props)...)
		if !s.Available {
			t.Skip("the structural check needs python")
		}
		if s.Err != nil {
			t.Fatalf("%v: %v", c.props, s.Err)
		}
		if !strings.HasSuffix(strings.TrimSpace(s.Output), "collection box "+c.shape) {
			t.Fatalf("%v: the checker reports %q, and this case exists to make a box %s", c.props, s.Output, c.shape)
		}
		gdalReadsEveryFeature(t, gdal, path, c.props, features)
		if v := oracle.ShapelyValid(path, true); !v.Available {
			t.Skipf("%s is not installed, so no geometry was judged - a skip, not a pass", v.Tool)
		} else if v.Err != nil {
			t.Fatalf("%v: %v", c.props, v.Err)
		}
	}
}

// gdalMixedIDs is the one complaint GDAL is allowed. With ids=mixed it gives
// every feature whose id is text the same number, then says so - measured
// 2026-10-07 with GDAL 3.13.3, and that loss is the pitfall the setting hands
// a tester. Allowed, not required: another version may stay quiet.
var gdalMixedIDs = regexp.MustCompile(`^Warning 1: Several features with id = -?\d+ have been found\. Altering it to be unique\.`)

// gdalReadsEveryFeature asks GDAL to read every feature with nothing on
// standard error but the complaint the settings call for, and to read as many
// as the file holds.
func gdalReadsEveryFeature(t *testing.T, gdal oracle.Checker, path string, props map[string]string, features int) {
	t.Helper()
	res := gdal.Check(path)
	if !res.Available {
		t.Skipf("%s is not installed, so no file was read by GDAL - a skip, not a pass", res.Tool)
	}
	if res.Err != nil && !(props[geojson.IDs] == geojson.Mixed && onlyMixedIDComplaint(res.Err.Error())) {
		t.Fatalf("%v: %v", props, res.Err)
	}
	if got := oracle.GDALFeatures(res.Output); got != features {
		t.Fatalf("%v: GDAL read %d features and the file holds %d", props, got, features)
	}
}

// onlyMixedIDComplaint is whether every line GDAL complained with is the one
// about ids, and nothing else came with it.
func onlyMixedIDComplaint(complaint string) bool {
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(complaint, "GDAL complained: ")), "\n")
	for _, line := range lines {
		if !gdalMixedIDs.MatchString(strings.TrimSpace(line)) {
			return false
		}
	}
	return true
}

func mustOracle(t *testing.T, d format.Descriptor) oracle.Checker {
	t.Helper()
	c, ok := oracle.For(d.Oracle)
	if !ok {
		t.Fatalf("%s declares the oracle %q and nothing implements it", d.ID, d.Oracle)
	}
	return c
}

// writeGeoJSON writes one file and says how many features it holds, read back
// with the standard library rather than counted by the generator.
func writeGeoJSON(t *testing.T, path string, size int64, seed uint64, props map[string]string) int {
	t.Helper()
	d, err := format.Get("geojson")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("%v at %d B is not JSON: %v", props, size, err)
	}
	return len(doc.Features)
}

// geojsonSettings is a case as the words the structural checker is told.
func geojsonSettings(props map[string]string) []string {
	var out []string
	for k, v := range props {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
