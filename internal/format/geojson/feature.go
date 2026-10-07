package geojson

import (
	"math"
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

const (
	// noteCap is the longest the closing note gets. Measured 2026-10-07: GDAL
	// reads a string of 9 999 999 characters and refuses one of 10 000 000
	// ("Too many characters in string"), and a feature of a million points is
	// 16 to 25 MB - so the remainder a closing note would have to absorb can be
	// larger than the reference reader takes. Above the cap the last bytes are
	// spaces after the feature, which GDAL read at 20 MB without a word.
	// docs/GEOJSON-2026-10-07.md section 5, the owner's decision.
	noteCap = 1 << 20
)

// records builds the features of the collection, one at a time, for
// core.FillRecords.
type records struct {
	s        settings
	g        grid
	next     int64
	e        emitter
	d        drawing
	tail     []byte
	shortest int64
}

// values is everything about a feature except its geometry and its note,
// drawn or chosen in one place so the smallest feature is measured with the
// same code that writes every other one.
type values struct {
	name, tagA, tagB, city string
	whole, cents, zip      int64
	active                 bool
}

func newRecords(s settings) *records {
	r := &records{s: s, g: gridFor(s.precision), e: emitter{indent: s.layout.indent}}
	r.shortest = r.measureShortest()
	return r
}

// Shortest is the smallest feature this builder can close the collection with.
func (r *records) Shortest() int64 { return r.shortest }

// Discard hands back the number the thrown away feature took with it, so the
// closing one carries it instead and the ids read 1..N. The kind follows the
// number, so the closing feature is the kind the thrown away one would have
// been.
func (r *records) Discard() { r.next-- }

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	dst = r.draw(dst, rng)
	dst = appendPhrase(dst, rng, 3+rng.IntN(5))
	dst = r.shut(dst)
	return append(dst, r.s.layout.between...)
}

// AppendExact writes the closing feature at exactly n bytes, the end of the
// collection included. The note takes the remainder up to noteCap, and spaces
// after the feature take the rest.
func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	mark := len(dst)
	dst = r.draw(dst, rng)
	at := len(dst)
	dst = r.shut(dst)
	r.tail = append(r.tail[:0], dst[at:]...)
	fill := n - int64(len(dst)-mark) - int64(len(r.s.layout.epilogue))
	note := min(fill, noteCap)
	dst = core.AppendFiller(dst[:at], words, note, nil)
	dst = append(dst, r.tail...)
	dst = appendSpaces(dst, fill-note)
	return append(dst, r.s.layout.epilogue...)
}

// draw writes the next feature up to the opening quote of its note.
func (r *records) draw(dst []byte, rng *rand.Rand) []byte {
	r.next++
	k := r.s.kinds[(r.next-1)%int64(len(r.s.kinds))]
	r.d.draw(rng, r.s, r.g, k)
	v := values{
		name: word(rng), whole: 100000 + rng.Int64N(899999), cents: rng.Int64N(100),
		active: rng.IntN(2) == 0, tagA: word(rng), tagB: word(rng), city: word(rng),
		zip: 10000 + rng.Int64N(90000),
	}
	return r.write(dst, r.next, k, v)
}

// measureShortest is the closing feature at its longest with an empty note:
// the widest feature number, the longest word everywhere a word goes, the
// widest coordinates and the largest kind the settings draw. It has to hold
// for every draw rather than for the lucky one.
//
// Measured on the bytes rather than worked out, like everything in this
// project that has to agree with bytes beside it - but measured at three and
// four points and carried forward, because every position of a kind is the
// same number of bytes at its widest, so a kind's length grows by the same
// amount with every point, and a million points measured for every file
// planned would be the slowest part of planning it. The guard compares the two.
//
// Carried forward kind by kind and only then compared. The largest kind at
// three points need not be the largest at a million - a position nested deeper
// costs more indentation - and the largest of seven straight lines is not a
// straight line.
func (r *records) measureShortest() int64 {
	var longest int64
	for _, k := range r.s.kinds {
		three, four := r.worstLength(k, minVertices), r.worstLength(k, minVertices+1)
		longest = max(longest, three+int64(r.s.vertices-minVertices)*(four-three))
	}
	return longest
}

// ClosingFeatureBounds is the smallest closing feature worked out two ways, for
// the guard that keeps planning honest: carried forward from three and four
// points the way Plan does it, and measured whole at the points asked for.
func ClosingFeatureBounds(props map[string]string) (carried, whole int64, err error) {
	s, err := parse(props)
	if err != nil {
		return 0, 0, err
	}
	r := newRecords(s)
	for _, k := range s.kinds {
		whole = max(whole, r.worstLength(k, s.vertices))
	}
	return r.Shortest(), whole, nil
}

// worstLength is the length of the longest closing feature of kind k with n
// points, the end of the collection included.
func (r *records) worstLength(k kind, n int) int64 {
	s := r.s
	s.vertices = n
	r.d.worst(s, r.g, k)
	widest := values{name: longestWord, tagA: longestWord, tagB: longestWord, city: longestWord,
		whole: 999999, cents: 99, zip: 99999}
	buf := r.shut(r.write(nil, math.MaxInt64, k, widest))
	return int64(len(buf) + len(s.layout.epilogue))
}

// write is one feature through the opening quote of its note, from a drawing
// already made and values already chosen.
func (r *records) write(dst []byte, id int64, k kind, v values) []byte {
	e := &r.e
	e.b = append(dst, r.s.layout.start...)
	e.depth = featureDepth
	e.open('{')
	e.key("type")
	e.text("Feature")
	e.key("id")
	e.whole(id)
	e.key("geometry")
	r.geometry(k)
	e.key("properties")
	r.properties(v)
	e.key("note")
	e.b = append(e.b, '"')
	return e.b
}

// properties writes every value type JSON has - text, number, true or false,
// null, an array and an object - the same set the json format carries, so an
// import that turns properties into columns meets each of them. The note comes
// last and is left open.
func (r *records) properties(v values) {
	e := &r.e
	e.open('{')
	e.key("name")
	e.text(v.name)
	e.key("amount")
	e.whole(v.whole)
	e.b = append(e.b, '.', byte('0'+v.cents/10), byte('0'+v.cents%10))
	e.key("active")
	e.b = strconv.AppendBool(e.b, v.active)
	e.key("retired")
	e.b = append(e.b, "null"...)
	e.key("tags")
	e.open('[')
	e.next()
	e.text(v.tagA)
	e.next()
	e.text(v.tagB)
	e.close(']')
	e.key("address")
	e.open('{')
	e.key("city")
	e.text(v.city)
	e.key("zip")
	e.b = append(e.b, '"')
	e.whole(v.zip)
	e.b = append(e.b, '"')
	e.close('}')
}

// shut closes the note, the properties and the feature.
func (r *records) shut(dst []byte) []byte {
	r.e.b = append(dst, '"')
	r.e.close('}')
	r.e.close('}')
	return r.e.b
}

// appendPhrase writes a readable note of n words. Words and single spaces
// only - a JSON string would otherwise need escaping, and an escape costs a
// byte the size arithmetic did not budget for.
func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, word(rng)...)
	}
	return dst
}

// appendSpaces writes n spaces, in pieces, because n can be tens of megabytes.
func appendSpaces(dst []byte, n int64) []byte {
	for ; n > 0; n -= int64(len(indentation)) {
		dst = append(dst, indentation[:min(n, int64(len(indentation)))]...)
	}
	return dst
}

func word(rng *rand.Rand) string { return words[rng.IntN(len(words))] }

// longestWord is the widest draw, because the minimum has to hold for every
// draw rather than for the lucky one.
var longestWord = func() string {
	longest := ""
	for _, w := range words {
		if len(w) > len(longest) {
			longest = w
		}
	}
	return longest
}()

// words is the vocabulary for names, tags, cities and notes: places and things
// a map of a service area would mark.
var words = []string{
	"airfield", "anchorage", "bakery", "bridge", "canal", "carpark", "checkpoint",
	"clinic", "crossing", "dairy", "depot", "dock", "factory", "farm", "ferry",
	"field", "garage", "harbour", "hospital", "hostel", "junction", "kiosk",
	"landing", "library", "lighthouse", "market", "mill", "museum", "office",
	"orchard", "outpost", "pharmacy", "pier", "plant", "quarry", "reservoir",
	"school", "shelter", "siding", "station", "storage", "substation", "terminal",
	"tower", "tunnel", "viaduct", "warehouse", "wharf", "workshop", "yard",
}
