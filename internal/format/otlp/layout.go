package otlp

import (
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
	Traces: {"resourceSpans", "scopeSpans", "spans"},
	Logs:   {"resourceLogs", "scopeLogs", "logRecords"},
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
	e.b = append(e.b, k...)
	e.b = append(e.b, '"', ':')
	if e.indent {
		e.b = append(e.b, ' ')
	}
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
// OTLP writes every number of 64 bits, a time among them.
func (e *emitter) quoted(v int64) {
	e.b = append(e.b, '"')
	e.whole(v)
	e.b = append(e.b, '"')
}

// nothing is an object with no member, written the same in every layout.
func (e *emitter) nothing() { e.b = append(e.b, '{', '}') }

// id is a trace id or a span id, two hex digits a byte. Lower case because
// that is what the Collector writes back, though the specification takes
// either.
func (e *emitter) id(id []byte) {
	e.b = append(e.b, '"')
	e.b = hex.AppendEncode(e.b, id)
	e.b = append(e.b, '"')
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
func opening(names envelope, indent bool) []byte {
	e := &emitter{indent: indent}
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
func closing(l layout) []byte {
	e := &emitter{indent: l.indent, depth: recordDepth}
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
