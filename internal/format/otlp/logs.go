package otlp

import (
	"encoding/base64"

	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
)

// sampled is the one trace flag a log record carries here: the request it was
// written in was recorded.
const sampled = 1

// entry is everything one log record says.
type entry struct {
	at       int64
	level    level
	message  string
	function string
	line     int64

	trace traceID
	span  spanID
	// body is which kind of value the body holds, nought for text.
	body int
}

// drawEntry draws the next log record of the file. n is how many came before
// it, which is what moves the clock.
//
// The order of the draws is part of the format, for the reason drawSpan gives.
// The ids are drawn whether or not the file carries them, so turning the trace
// context off changes nothing else about a record.
func (r *records) drawEntry(rng *rand.Rand, n int64) entry {
	lv := r.s.levels[rng.IntN(len(r.s.levels))]
	en := entry{
		at:       r.s.start + r.s.order(n)*r.s.step(),
		level:    lv,
		message:  lv.messages[rng.IntN(len(lv.messages))],
		function: functions[rng.IntN(len(functions))],
		line:     firstLine + rng.Int64N(lastLine-firstLine+1),
	}
	if r.s.anyBody {
		en.body = int(n % bodyKinds)
	}
	drawID(en.trace[:], rng)
	drawID(en.span[:], rng)
	return en
}

// widestEntries are the longest log records the settings can draw, one for
// each severity of the mix with the longest thing it says. None when the file
// holds no log records.
func (r *records) widestEntries() []entry {
	if r.s.signal != Logs {
		return nil
	}
	var out []entry
	kinds := 1
	if r.s.anyBody {
		kinds = bodyKinds
	}
	for _, lv := range r.s.levels {
		for body := range kinds {
			out = append(out, entry{at: r.s.start, level: lv, message: longest(lv.messages), function: longest(functions), line: lastLine, body: body})
		}
	}
	return out
}

// writeEntry writes one log record, and its padding attribute when it is the
// closing one. at is where the padding begins, nought without one.
func (r *records) writeEntry(en entry, padded bool) (at int) {
	e := &r.e
	e.next()
	e.open('{')
	e.key("timeUnixNano")
	e.quoted(en.at)
	// The moment a collector saw the record. A record an SDK wrote is seen
	// the moment it happens, so the two are the same.
	e.key("observedTimeUnixNano")
	e.quoted(en.at)
	e.key("severityNumber")
	e.enum(en.level.number, en.level.enum)
	e.key("severityText")
	e.text(en.level.text)
	e.key("body")
	e.open('{')
	r.writeBody(en)
	e.close('}')
	e.key("attributes")
	e.open('[')
	e.textAttribute("code.function.name", en.function)
	e.wholeAttribute("code.line.number", en.line)
	if padded {
		at = e.padding()
	}
	e.close(']')
	if r.s.traceContext {
		// After the attributes, which is where logs.proto declares them and
		// where the Collector writes them.
		e.key("flags")
		e.whole(sampled)
		e.key("traceId")
		e.id(en.trace[:])
		e.key("spanId")
		e.id(en.span[:])
	}
	e.close('}')
	return at
}

// bodyKinds is how many kinds of value AnyValue has, which body any-value
// takes in turn.
const bodyKinds = 7

// beyondDoubles is the smallest whole number a float64 cannot hold, 2^53 + 1.
// A reader that turns every JSON number into a double reads it one less.
const beyondDoubles = 1<<53 + 1

// writeBody writes what a log record says as the kind of value its body
// holds: text, or with body any-value each of the seven kinds of AnyValue in
// turn - text, a boolean, a whole number past what a double holds, a
// fraction, a list, a map and bytes.
func (r *records) writeBody(en entry) {
	e := &r.e
	switch en.body {
	case 0:
		e.key("stringValue")
		e.text(en.message)
	case 1:
		e.key("boolValue")
		e.truth()
	case 2:
		e.key("intValue")
		e.quoted(beyondDoubles)
	case 3:
		e.key("doubleValue")
		e.decimal(en.line * 1000)
	case 4:
		e.key("arrayValue")
		e.open('{')
		e.key("values")
		e.open('[')
		e.next()
		e.open('{')
		e.key("stringValue")
		e.text(en.message)
		e.close('}')
		e.next()
		e.open('{')
		e.key("intValue")
		e.quoted(en.line)
		e.close('}')
		e.close(']')
		e.close('}')
	case 5:
		e.key("kvlistValue")
		e.open('{')
		e.key("values")
		e.open('[')
		e.textAttribute("message", en.message)
		e.wholeAttribute("code.line.number", en.line)
		e.close(']')
		e.close('}')
	default:
		e.key("bytesValue")
		e.b = append(e.b, '"')
		e.b = base64.StdEncoding.AppendEncode(e.b, []byte(en.message))
		e.b = append(e.b, '"')
	}
}
