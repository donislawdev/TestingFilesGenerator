package otlp

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

// layout is how a file is laid out: whether it is cut into lines, one batch
// to a line, and whether every value stands on a line of its own.
type layout struct {
	name string
	// lines is true for the layout of the OTLP file exporter, where a batch
	// ends its line. The other two write the whole file as one batch.
	lines bool
	// indent puts every value on its own line, two spaces a level, the way
	// the examples in the opentelemetry-proto repository are written.
	indent bool
	// end is what follows the last brace of a batch.
	end string
}

var layouts = map[string]layout{
	// One batch on each line: JSON Lines, which the specification of the OTLP
	// file exporter names.
	RecordPerLine: {name: RecordPerLine, lines: true, end: "\n"},
	// No whitespace anywhere and no newline at the end, the same promise the
	// json format makes for the same word.
	Minified: {name: Minified},
	// What JSON.stringify(x, null, 2) writes, and what the OTLP examples look
	// like.
	Indented: {name: Indented, indent: true, end: "\n"},
}

// envelope is the names one kind of telemetry gives the three levels a record
// sits under: the resource, the scope and the records themselves.
type envelope struct {
	resources, scopes, records string
}

var envelopes = map[string]envelope{
	Traces:  {"resourceSpans", "scopeSpans", "spans"},
	Logs:    {"resourceLogs", "scopeLogs", "logRecords"},
	Metrics: {"resourceMetrics", "scopeMetrics", "metrics"},
}

const (
	// recordDepth is how deep a record sits: inside the document, the array
	// of resources, a resource, its array of scopes, a scope and its array of
	// records.
	recordDepth = 6

	// paddingKey names the attribute that takes the last bytes of a file. Its
	// own prefix, so it cannot be mistaken for an attribute the semantic
	// conventions define.
	paddingKey = "tfg.padding"
)

// emitter appends JSON tokens with the whitespace its layout asks for.
//
// It appends into one slice rather than writing to a stream, because the
// closing record has to be measured before its padding is written. The same
// shape as the emitter of the geojson format, which nests for the same reason.
type emitter struct {
	b      []byte
	indent bool
	depth  int
	// fresh is true right after a container opens, before its first element,
	// which is the one element not preceded by a comma.
	fresh bool
	sp    spelling
}

// spelling is how the tokens of a file are spelled where the pitfalls change
// them. The zero value is the spelling of the specification, the one the
// Collector writes back.
type spelling struct {
	// ids is upper-hex or base64 where the ids are not hex in lower case.
	ids string
	// names writes an enum as its name, which the specification forbids.
	names bool
	// snake writes every key in snake case, which the specification forbids.
	snake bool
	// bare writes a number of 64 bits as a JSON number rather than as text.
	bare bool
}

func (s settings) spelling() spelling {
	return spelling{ids: s.idEncoding, names: s.enumNames, snake: s.snakeKeys, bare: s.bareNumbers}
}

// indentation is enough spaces for the deepest token a record has: the value
// of an attribute sits ten levels down, twenty spaces.
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

// close ends a container that holds at least one element. An empty one is
// written by nothing.
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
	if e.sp.snake {
		e.b = appendSnake(e.b, k)
	} else {
		e.b = append(e.b, k...)
	}
	e.b = append(e.b, '"', ':')
	if e.indent {
		e.b = append(e.b, ' ')
	}
}

// appendSnake appends a key in snake case: resourceSpans as resource_spans,
// the name the .proto file gives the field and the specification calls not
// valid as a key.
func appendSnake(dst []byte, k string) []byte {
	for i := 0; i < len(k); i++ {
		if c := k[i]; 'A' <= c && c <= 'Z' {
			dst = append(dst, '_', c-'A'+'a')
			continue
		}
		dst = append(dst, k[i])
	}
	return dst
}

// enum is a value of an enum: its number, or its name where the pitfall asks.
func (e *emitter) enum(number int64, name string) {
	if e.sp.names {
		e.text(name)
		return
	}
	e.whole(number)
}

// text is a string value. Every string this format writes comes from its own
// vocabulary, none of which needs escaping.
func (e *emitter) text(v string) {
	e.b = append(e.b, '"')
	e.b = append(e.b, v...)
	e.b = append(e.b, '"')
}

func (e *emitter) whole(v int64) { e.b = strconv.AppendInt(e.b, v, 10) }

// quoted is a whole number written as text, which is how the JSON encoding of
// OTLP writes every number of 64 bits, a time among them - or as a JSON
// number where the pitfall asks, which a reader is told to take as well.
func (e *emitter) quoted(v int64) {
	if e.sp.bare {
		e.whole(v)
		return
	}
	e.numbered("", v)
}

// numbered is a string value made of a prefix and a whole number, appended
// straight into the file.
//
// Never through strconv.Itoa, which builds a new string for every number from
// 100 up. Measured on 2026-11-11: a client address and a response code built
// that way cost 37292 objects for a 64 MiB file of spans, against a budget of
// 128, and TestNoGeneratorHoldsTheWholeFileInMemory turned CI red on it.
func (e *emitter) numbered(prefix string, v int64) {
	e.b = append(e.b, '"')
	e.b = append(e.b, prefix...)
	e.whole(v)
	e.b = append(e.b, '"')
}

// nothing is an object with no member, written the same in every layout.
func (e *emitter) nothing() { e.b = append(e.b, '{', '}') }

// truth is the one boolean this format writes. A false one is left out, which
// is how the Collector writes it.
func (e *emitter) truth() { e.b = append(e.b, "true"...) }

// micros is how many millionths of a unit a decimal value is. Every fraction
// this format writes is counted that way and never held as a float64.
//
// Two reasons. math.Log and its kin run assembly on amd64 and Go elsewhere,
// and a result one unit of the last place apart would put a duration in the
// next bucket on one machine only, which D11 forbids. And a whole count of
// millionths below 2^33 * 10^6 is written exactly as the Collector writes the
// float64 it reads back: it has six decimals at most, and below 2^33 two
// float64 values lie closer than a millionth, so no shorter decimal names the
// same one - measured with tools/probes/otlp/metrics-canonical.py, 2026-11-11.
type micros = int64

// microCeiling is the first count of millionths that rule no longer covers.
const microCeiling micros = (1 << 33) * 1_000_000

// decimal writes a count of millionths as the shortest decimal that names it:
// the whole part, then the fraction without the zeros it ends in, and no point
// at all for a whole number - 100, 0.5, 0.000125.
func (e *emitter) decimal(v micros) {
	e.whole(v / 1_000_000)
	frac := v % 1_000_000
	if frac == 0 {
		return
	}
	var digits [6]byte
	for i := 5; i >= 0; i-- {
		digits[i] = byte('0' + frac%10)
		frac /= 10
	}
	n := 6
	for digits[n-1] == '0' {
		n--
	}
	e.b = append(e.b, '.')
	e.b = append(e.b, digits[:n]...)
}

// id is a trace id or a span id, two hex digits a byte. Lower case because
// that is what the Collector writes back, though the specification takes
// either.
func (e *emitter) id(id []byte) {
	e.b = append(e.b, '"')
	switch e.sp.ids {
	case UpperHex:
		at := len(e.b)
		e.b = hex.AppendEncode(e.b, id)
		capitals(e.b[at:])
	case Base64:
		// What the JSON mapping of protobuf writes for bytes, and what the
		// OTLP specification says the ids are not.
		e.b = base64.StdEncoding.AppendEncode(e.b, id)
	default:
		e.b = hex.AppendEncode(e.b, id)
	}
	e.b = append(e.b, '"')
}

// capitals turns the letters of hex digits into capitals, in place.
func capitals(digits []byte) {
	for i, c := range digits {
		if 'a' <= c && c <= 'f' {
			digits[i] = c - 'a' + 'A'
		}
	}
}

// attribute opens one attribute through the brace of its value.
func (e *emitter) attribute(key string) {
	e.next()
	e.open('{')
	e.key("key")
	e.text(key)
	e.key("value")
	e.open('{')
}

// shut closes the value of an attribute and the attribute.
func (e *emitter) shut() {
	e.close('}')
	e.close('}')
}

func (e *emitter) textAttribute(key, value string) {
	e.attribute(key)
	e.key("stringValue")
	e.text(value)
	e.shut()
}

// wholeAttribute is an attribute holding a whole number, which the encoding
// writes as text like every other number of 64 bits.
func (e *emitter) wholeAttribute(key string, value int64) {
	e.attribute(key)
	e.key("intValue")
	e.quoted(value)
	e.shut()
}

// numberedAttribute is an attribute holding text that ends in a whole number,
// written without building the text first.
func (e *emitter) numberedAttribute(key, prefix string, value int64) {
	e.attribute(key)
	e.key("stringValue")
	e.numbered(prefix, value)
	e.shut()
}

// padding writes the padding attribute with nothing in it and returns where
// its value begins, so the closing record can be measured first and filled
// afterwards.
func (e *emitter) padding() (at int) {
	e.attribute(paddingKey)
	e.key("stringValue")
	e.b = append(e.b, '"')
	at = len(e.b)
	e.b = append(e.b, '"')
	e.shut()
	return at
}

// opening is what starts a batch of this kind of telemetry, through the
// bracket that opens its records: the resource the records come from and the
// scope that wrote them.
func opening(names envelope, indent bool, sp spelling) []byte {
	e := &emitter{indent: indent, sp: sp}
	e.open('{')
	e.key(names.resources)
	e.open('[')
	e.next()
	e.open('{')
	e.key("resource")
	e.open('{')
	e.key("attributes")
	e.open('[')
	e.textAttribute("service.name", serviceName)
	e.textAttribute("service.version", serviceVersion)
	e.close(']')
	e.close('}')
	e.key(names.scopes)
	e.open('[')
	e.next()
	e.open('{')
	e.key("scope")
	e.open('{')
	e.key("name")
	e.text(scopeName)
	e.key("version")
	e.text(serviceVersion)
	e.close('}')
	e.key(names.records)
	e.open('[')
	return e.b
}

// closing is what ends a batch, from the bracket that closes its records. The
// schema address follows the records it describes at both levels, which is
// where the Collector writes it.
func closing(l layout, sp spelling) []byte {
	e := &emitter{indent: l.indent, depth: recordDepth, sp: sp}
	e.close(']')
	e.key("schemaUrl")
	e.text(schemaURL)
	e.close('}')
	e.close(']')
	e.key("schemaUrl")
	e.text(schemaURL)
	e.close('}')
	e.close(']')
	e.close('}')
	return append(e.b, l.end...)
}
