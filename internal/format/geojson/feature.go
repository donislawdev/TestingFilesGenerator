package geojson

import (
	"math"
	"slices"
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
	closing  []byte
	shortest int64

	// seen is the box every feature written so far lies in, for the bbox of
	// the collection, and before is what it was until the last feature. The
	// one feature built only to learn it does not fit is thrown away, and its
	// place must go with it - a box that takes in a feature the file does not
	// hold is the same lie as a number the file skips.
	seen, before extent
}

// values is everything about a feature except its geometry and its note,
// drawn or chosen in one place so the smallest feature is measured with the
// same code that writes every other one. bounds is the box of its geometry,
// set only when the feature carries a bbox.
type values struct {
	name, tagA, tagB, city string
	whole, cents, zip      int64
	active                 bool
	id                     int64
	idForm                 string
	bounds                 extent
}

func newRecords(s settings) *records {
	r := &records{s: s, g: gridFor(s.precision), e: emitter{indent: s.layout.indent}}
	r.shortest = r.measureShortest()
	return r
}

// Shortest is the smallest feature this builder can close the collection with.
func (r *records) Shortest() int64 { return r.shortest }

// Discard hands back the number the thrown away feature took with it, so the
// closing one carries it instead and the ids read 1..N, and the place it took
// in the collection's box. The kind follows the number, so the closing feature
// is the kind the thrown away one would have been.
func (r *records) Discard() {
	r.next--
	r.seen = r.before
}

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	r.before = r.seen
	dst = r.draw(dst, rng)
	dst = appendPhrase(dst, rng, 3+rng.IntN(5))
	dst = r.shut(dst)
	return append(dst, r.s.layout.between...)
}

// AppendExact writes the closing feature at exactly n bytes, the end of the
// collection included. The note takes the remainder up to noteCap, and spaces
// after the feature take the rest.
//
// The end of the collection is built after the closing feature is drawn,
// because its bbox takes that feature in as well.
func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	mark := len(dst)
	dst = r.draw(dst, rng)
	at := len(dst)
	dst = r.shut(dst)
	r.tail = append(r.tail[:0], dst[at:]...)
	r.closing = r.epilogue(r.closing[:0], r.seen)
	fill := n - int64(len(dst)-mark) - int64(len(r.closing))
	note := min(fill, noteCap)
	dst = core.AppendFiller(dst[:at], words, note, nil)
	dst = append(dst, r.tail...)
	dst = appendSpaces(dst, fill-note)
	return append(dst, r.closing...)
}

// epilogue is what closes the collection: the end of its features, its bbox
// when one was asked for and any feature has a place, and the end of the
// object.
func (r *records) epilogue(dst []byte, x extent) []byte {
	dst = append(dst, r.s.layout.after...)
	if r.s.bbox && x.set {
		e := &r.e
		e.b, e.depth, e.fresh = dst, 1, false
		e.key("bbox")
		e.box(x, r.g, r.s.dims())
		dst = e.b
	}
	return append(dst, r.s.layout.end...)
}

// draw writes the next feature up to the opening quote of its note.
func (r *records) draw(dst []byte, rng *rand.Rand) []byte {
	r.next++
	k := r.s.kindAt(r.next)
	r.d.draw(rng, r.s, r.g, k)
	v := values{
		name: word(rng), whole: 100000 + rng.Int64N(899999), cents: rng.Int64N(100),
		active: rng.IntN(2) == 0, tagA: word(rng), tagB: word(rng), city: word(rng),
		zip: 10000 + rng.Int64N(90000), id: r.next, idForm: r.s.idAt(r.next),
	}
	if r.s.bbox {
		v.bounds = r.d.extent()
		r.seen.grow(v.bounds, r.d.stride)
	}
	return r.write(dst, k, v)
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
//
// Its id is the widest number in the widest form the settings write, and both
// boxes hold the widest number at every axis. The box of the collection is
// counted whenever the settings draw any geometry - with some features
// unlocated the first one always has a place, so the box is always there.
func (r *records) worstLength(k kind, n int) int64 {
	s := r.s
	s.vertices = n
	r.d.worst(s, r.g, k)
	widest := values{name: longestWord, tagA: longestWord, tagB: longestWord, city: longestWord,
		whole: 999999, cents: 99, zip: 99999, id: math.MaxInt64, idForm: s.widestID()}
	if s.bbox {
		widest.bounds = r.d.extent()
	}
	buf := r.shut(r.write(nil, k, widest))
	// Into the buffer the closing feature reuses, rather than a new one for
	// each kind and each count measured: fourteen of those with mixed were
	// enough to take the generator past the allocation ceiling of the guard
	// that keeps a file out of memory (CI on #172).
	r.closing = r.epilogue(r.closing[:0], widestBox(r.s, r.g))
	return int64(len(buf) + len(r.closing))
}

// widestBox is a box with the widest number the grid writes at every axis, or
// none when the settings draw no geometry at all.
func widestBox(s settings, g grid) extent {
	if slices.Equal(s.kinds, []kind{kindNone}) {
		return extent{}
	}
	var x extent
	x.add([]int64{-g.lon, -g.lat, g.low})
	return x
}

// write is one feature through the opening quote of its note, from a drawing
// already made and values already chosen.
func (r *records) write(dst []byte, k kind, v values) []byte {
	e := &r.e
	e.b = append(dst, r.s.layout.start...)
	e.depth = featureDepth
	e.open('{')
	e.key("type")
	e.text("Feature")
	e.id(v.id, v.idForm)
	if v.bounds.set {
		e.key("bbox")
		e.box(v.bounds, r.g, r.s.dims())
	}
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

// id writes the id member in the form asked for: a number, the same number as
// text after an f, or nothing at all. Every form counts 1..N, so a test can
// still find a feature that went missing.
func (e *emitter) id(n int64, form string) {
	switch form {
	case Number:
		e.key("id")
		e.whole(n)
	case String:
		e.key("id")
		e.b = append(e.b, '"', 'f')
		e.whole(n)
		e.b = append(e.b, '"')
	}
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
