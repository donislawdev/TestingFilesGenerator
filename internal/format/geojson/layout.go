package geojson

import "strconv"

// layout is what stands around the features: what opens the collection, what
// goes before and after each feature, and what closes it.
//
// The features themselves go through emitter, which knows only whether to
// indent. Unlike the json format, whose records are flat enough to be held as
// literals, a feature here nests to a depth that depends on its geometry - a
// polygon's numbers sit two arrays deeper than a point's - so the whitespace is
// worked out per token rather than written down per field.
type layout struct {
	name     string
	indent   bool
	prologue string
	start    string
	between  string
	epilogue string
}

// featureDepth is how deep a feature sits: inside the collection and inside
// its features array. Indented, that is four spaces before its brace.
const featureDepth = 2

var layouts = map[string]layout{
	// One feature on each line, the shape a tester can split, count and diff
	// line by line.
	RecordPerLine: {
		name:     RecordPerLine,
		prologue: `{"type":"FeatureCollection","features":[` + "\n",
		between:  ",\n",
		epilogue: "\n]}\n",
	},
	// No whitespace anywhere and no newline at the end, the same promise the
	// json format makes for the same word.
	Minified: {
		name:     Minified,
		prologue: `{"type":"FeatureCollection","features":[`,
		between:  ",",
		epilogue: "]}",
	},
	// What JSON.stringify(x, null, 2) and json.dumps(x, indent=2) write: two
	// spaces a level and every value on its own line, every coordinate number
	// included. That is what makes it worth having - the same features are
	// several times larger, and a file a person opens in an editor often looks
	// exactly like this.
	Indented: {
		name:     Indented,
		indent:   true,
		prologue: "{\n  \"type\": \"FeatureCollection\",\n  \"features\": [\n",
		start:    "    ",
		between:  ",\n",
		epilogue: "\n  ]\n}\n",
	},
}

// emitter appends JSON tokens with the whitespace its layout asks for.
//
// It appends into one slice rather than writing to a stream, because a closing
// feature has to be measured before its note is written, and because one
// allocation per token over a file of millions of numbers is a multiple of the
// file in garbage.
type emitter struct {
	b      []byte
	indent bool
	depth  int
	// fresh is true right after a container opens, before its first element,
	// which is the one element not preceded by a comma.
	fresh bool
}

// indentation is enough spaces for the deepest token a feature has, with room
// to spare: a number inside a position inside a ring inside the polygon of a
// geometry collection sits nine levels down, eighteen spaces.
const indentation = "                                "

func (e *emitter) newline() {
	e.b = append(e.b, '\n')
	e.b = append(e.b, indentation[:2*e.depth]...)
}

// next is what comes before any element of a container - a member of an object
// or a value of an array.
func (e *emitter) next() {
	if !e.fresh {
		e.b = append(e.b, ',')
	}
	e.fresh = false
	if e.indent {
		e.newline()
	}
}

func (e *emitter) open(c byte) {
	e.b = append(e.b, c)
	e.depth++
	e.fresh = true
}

// close ends a container. Every container a feature opens has at least one
// element, so an empty one never needs its own spelling.
func (e *emitter) close(c byte) {
	e.depth--
	if e.indent {
		e.newline()
	}
	e.fresh = false
	e.b = append(e.b, c)
}

func (e *emitter) key(k string) {
	e.next()
	e.b = append(e.b, '"')
	e.b = append(e.b, k...)
	e.b = append(e.b, '"', ':')
	if e.indent {
		e.b = append(e.b, ' ')
	}
}

// text is a string value. Every string this format writes is a word from the
// vocabulary or a run of them, none of which needs escaping.
func (e *emitter) text(v string) {
	e.b = append(e.b, '"')
	e.b = append(e.b, v...)
	e.b = append(e.b, '"')
}

func (e *emitter) whole(v int64) { e.b = strconv.AppendInt(e.b, v, 10) }

// fixed writes v, a count of steps of 10^-places, as a decimal with exactly
// that many places, from whole numbers only.
//
// A coordinate is held as a count of grid steps rather than as a float, so
// what is written is the geometry itself rather than a rounding of it. A
// rounding can turn a simple outline into one that crosses itself, and a float
// formatted on one machine is not promised to match another (D11).
func (e *emitter) fixed(v int64, places int, scale int64) {
	if v < 0 {
		e.b = append(e.b, '-')
		v = -v
	}
	e.b = strconv.AppendInt(e.b, v/scale, 10)
	if places == 0 {
		return
	}
	e.b = append(e.b, '.')
	frac := v % scale
	// Leading zeros of the fraction: a value of 5 at three places is .005.
	for lead := scale / 10; lead > frac && lead > 1; lead /= 10 {
		e.b = append(e.b, '0')
	}
	e.b = strconv.AppendInt(e.b, frac, 10)
}
