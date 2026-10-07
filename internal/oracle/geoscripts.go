package oracle

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// gdalGeoJSON is GDAL's GeoJSON driver - the reader under QGIS, ogr2ogr and
// much of the GIS world, which is neither our code nor our language. On its own
// it is a weak witness, measured 2026-10-07 (docs/GEOJSON-2026-10-07.md section
// 3): it opened seven of nine broken files, three of them with a warning and
// four without a word. So a warning is a refusal here, and every feature is
// read for one to appear - the summary mode does not parse the geometry and
// says nothing about an outline left open. What it cannot see, a shape that
// crosses itself, the guard of the format asks shapely.
var gdalGeoJSON = Checker{
	Name: "ogrinfo",
	find: ogrinfo,
	args: func(p string) []string { return []string{"-ro", "-al", "-q", "-geom=NO", p} },
	accept: func(stdout, stderr string, code int) error {
		if s := strings.TrimSpace(stderr); s != "" {
			return fmt.Errorf("GDAL complained: %s", s)
		}
		if code != 0 {
			return fmt.Errorf("ogrinfo ended with %d and said nothing", code)
		}
		if GDALFeatures(stdout) == 0 {
			return fmt.Errorf("GDAL read no feature")
		}
		return nil
	},
}

// GDALFeatures is how many features the GDAL oracle read, counted from what
// ogrinfo printed - one header line per feature. A guard compares it with the
// features in the file, because a reader that stops early and says nothing
// would otherwise pass.
func GDALFeatures(output string) int { return strings.Count(output, "OGRFeature(") }

// ogrinfo looks where QGIS puts GDAL on Windows as well, because its installer
// does not put it on the path - measured 2026-10-07 with QGIS 4.2.2. Any copy
// will do for this question, so the last by name is taken.
func ogrinfo() (string, bool) {
	if p, err := exec.LookPath("ogrinfo"); err == nil {
		return p, true
	}
	if runtime.GOOS != "windows" {
		return "", false
	}
	found, _ := filepath.Glob(`C:\Program Files\QGIS *\bin\ogrinfo.exe`)
	if len(found) == 0 {
		return "", false
	}
	return found[len(found)-1], true
}

// ShapelyValid asks shapely, which is GEOS underneath, whether every geometry
// in a GeoJSON file is valid in the sense of the OGC simple features - closed
// rings, no outline crossing itself, no two parts of a multi polygon
// overlapping.
//
// GDAL reading the file does not ask this, measured 2026-10-07: an outline
// crossing itself opened without a word. It is a question about the shape
// rather than about the text, so it lives beside the reader rather than in it.
//
// unwrap moves longitudes so no step between neighbours is more than 180
// degrees before asking. That is how a shape crossing the antimeridian is meant
// to be read. Read flat, as GEOS reads coordinates, the same outline crosses
// itself - measured on the same day, and that is the trap the setting exists
// to hand a tester, not a defect for this question to find.
func ShapelyValid(path string, unwrap bool) Result {
	return Checker{
		Name: "shapely",
		find: inPath("python"),
		args: func(p string) []string {
			return []string{"-c", shapelyScript, p, strconv.FormatBool(unwrap)}
		},
		accept: expectOK("shapely"),
	}.Check(path)
}

const shapelyScript = `
import json, sys
try:
    from shapely.geometry import shape
    from shapely.validation import explain_validity
except ImportError:
    print("SKIP shapely is not installed")
    sys.exit(0)

path, unwrap = sys.argv[1], sys.argv[2] == "true"

def unwrapped(c):
    if isinstance(c[0], (int, float)):
        return c
    if isinstance(c[0][0], (int, float)):
        out = [list(c[0])]
        for p in c[1:]:
            q = list(p)
            while q[0] - out[-1][0] > 180:
                q[0] -= 360
            while q[0] - out[-1][0] < -180:
                q[0] += 360
            out.append(q)
        return out
    return [unwrapped(x) for x in c]

def geometry(g):
    if g["type"] == "GeometryCollection":
        return {"type": g["type"], "geometries": [geometry(m) for m in g["geometries"]]}
    return {"type": g["type"], "coordinates": unwrapped(g["coordinates"]) if unwrap else g["coordinates"]}

with open(path, encoding="utf-8") as f:
    doc = json.load(f)
for feature in doc["features"]:
    s = shape(geometry(feature["geometry"]))
    if not s.is_valid:
        print("FAIL feature %s %s: %s" % (feature["id"], feature["geometry"]["type"], explain_validity(s)))
        sys.exit(1)
print("OK %d features, every geometry valid" % len(doc["features"]))
`
