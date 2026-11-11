package otlp

import (
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

	traceHigh, traceLow uint64
	span                uint64
}

// drawEntry draws the next log record of the file. n is how many came before
// it, which is what moves the clock.
//
// The order of the draws is part of the format, for the reason drawSpan gives.
// The ids are drawn whether or not the file carries them, so turning the trace
// context off changes nothing else about a record.
func (r *records) drawEntry(rng *rand.Rand, n int64) entry {
	lv := r.s.levels[rng.IntN(len(r.s.levels))]
	return entry{
		at:        r.s.start + n*r.s.step(),
		level:     lv,
		message:   lv.messages[rng.IntN(len(lv.messages))],
		function:  functions[rng.IntN(len(functions))],
		line:      firstLine + rng.Int64N(lastLine-firstLine+1),
		traceHigh: nonZero(rng.Uint64()), traceLow: rng.Uint64(),
		span: nonZero(rng.Uint64()),
	}
}

// widestEntries are the longest log records the settings can draw, one for
// each severity of the mix with the longest thing it says. None when the file
// holds no log records.
func (r *records) widestEntries() []entry {
	if r.s.signal != Logs {
		return nil
	}
	var out []entry
	for _, lv := range r.s.levels {
		out = append(out, entry{at: r.s.start, level: lv, message: longest(lv.messages), function: longest(functions), line: lastLine})
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
	e.whole(en.level.number)
	e.key("severityText")
	e.text(en.level.text)
	e.key("body")
	e.open('{')
	e.key("stringValue")
	e.text(en.message)
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
		e.id(en.traceHigh, en.traceLow)
		e.key("spanId")
		e.id(en.span)
	}
	e.close('}')
	return at
}
