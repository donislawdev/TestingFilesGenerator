package otlp

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The kinds of span a file holds, as trace.proto numbers them. The encoding
// writes the number, never the name.
const (
	kindInternal = 1
	kindServer   = 2

	// statusError is the code of a span that failed. Left out, the status is
	// unset, which is what every other span carries.
	statusError = 2
)

// span is everything one span says, drawn or chosen in one place so the
// smallest file is measured with the same code that writes every other one.
type span struct {
	trace      traceID
	id, parent spanID
	// level is how deep the span sits in its trace. Nought is the request
	// itself, a span of the server kind, and every other one is work done
	// inside it.
	level      int
	start, end int64

	request request
	status  int
	agent   string
	client  int

	function string
}

// trace is where the next span belongs: the trace it is part of, the span it
// is the child of and how long the request at the top of it takes.
type trace struct {
	// begun is how many traces the file has started, which is what moves the
	// clock.
	begun  int64
	level  int
	id     traceID
	parent spanID
	start  int64
	length int64
}

// The two ids of the OTLP data model, sixteen bytes and eight.
type (
	traceID [16]byte
	spanID  [8]byte
)

// drawID fills an id from the generator, through the primitive every format
// draws its random bytes with, and keeps it from being all zeros - which the
// specification calls invalid and the Collector reads as no id at all.
func drawID(id []byte, rng *rand.Rand) {
	core.FillRandomBE(id, rng)
	for _, b := range id {
		if b != 0 {
			return
		}
	}
	id[len(id)-1] = 1
}

// drawSpan draws the next span of the file and moves the trace on.
//
// The order of the draws is part of the format: the same seed has to draw the
// same file for as long as the major version stands (D11), so a draw is never
// moved, only added after the last one of its span.
func (r *records) drawSpan(rng *rand.Rand) span {
	t := &r.st.trace
	if t.level == 0 {
		drawID(t.id[:], rng)
		t.start = r.s.start + t.begun*r.s.step()
		t.length = int64(shortestSpan) + rng.Int64N(int64(longestSpan-shortestSpan))
		t.begun++
	}
	// Every span starts later and ends earlier than its parent by the same
	// step, so a child always lies inside the span it belongs to. The request
	// is long enough for the deepest trace the settings allow.
	inset := int64(t.level) * t.length / int64(2*r.s.depth)
	sp := span{trace: t.id, parent: t.parent, level: t.level, start: t.start + inset, end: t.start + t.length - inset}
	drawID(sp.id[:], rng)
	if sp.level == 0 {
		sp.request = requests[rng.IntN(len(requests))]
		sp.status = r.s.statuses[rng.IntN(len(r.s.statuses))]
		sp.agent = agents[rng.IntN(len(agents))]
		sp.client = 1 + rng.IntN(clientHosts)
	} else {
		sp.function = functions[rng.IntN(len(functions))]
	}
	t.parent = sp.id
	t.level = (t.level + 1) % r.s.depth
	return sp
}

// widestSpans are the longest spans the settings can draw, one of each shape:
// the request and the work inside it. The longer of the two is what the
// smallest file leaves room for. None when the file holds no spans.
//
// Each is dated at the start of the file, because a time is nineteen digits
// from there on and a span dated nought would be measured eighteen short.
func (r *records) widestSpans() []span {
	if r.s.signal != Traces {
		return nil
	}
	widest := span{
		request: widestRequest(), status: widestStatus(r.s.statuses), agent: longest(agents), client: clientHosts,
		start: r.s.start, end: r.s.start,
	}
	if r.s.depth == 1 {
		return []span{widest}
	}
	return []span{widest, {level: 1, function: longest(functions), start: r.s.start, end: r.s.start}}
}

// widestRequest is the request that makes the longest span. Its method and
// its route are each written twice, in the name and in an attribute.
func widestRequest() request {
	var widest request
	weight := func(q request) int { return 2*len(q.method) + 2*len(q.route) + len(q.path) }
	for _, q := range requests {
		if weight(q) > weight(widest) {
			widest = q
		}
	}
	return widest
}

// widestStatus is a response code that makes the longest span: one a span
// records as an error, when the mix holds any, because that span carries an
// attribute and a status code more.
func widestStatus(statuses []int) int {
	widest := statuses[0]
	for _, s := range statuses {
		widest = max(widest, s)
	}
	return widest
}

// writeSpan writes one span, and its padding attribute when it is the closing
// one. at is where the padding begins, nought without one.
func (r *records) writeSpan(sp span, padded bool) (at int) {
	e := &r.e
	e.next()
	e.open('{')
	e.key("traceId")
	e.id(sp.trace[:])
	e.key("spanId")
	e.id(sp.id[:])
	if sp.level > 0 {
		e.key("parentSpanId")
		e.id(sp.parent[:])
	}
	e.key("name")
	if sp.level == 0 {
		e.text(sp.request.method + " " + sp.request.route)
		e.key("kind")
		e.whole(kindServer)
	} else {
		e.text(sp.function)
		e.key("kind")
		e.whole(kindInternal)
	}
	e.key("startTimeUnixNano")
	e.quoted(sp.start)
	e.key("endTimeUnixNano")
	e.quoted(sp.end)
	e.key("attributes")
	e.open('[')
	if sp.level == 0 {
		r.requestAttributes(sp)
	} else {
		e.textAttribute("code.function.name", sp.function)
	}
	if padded {
		at = e.padding()
	}
	e.close(']')
	e.key("status")
	if sp.failed() {
		e.open('{')
		e.key("code")
		e.whole(statusError)
		e.close('}')
	} else {
		// Always there, also with nothing in it: that is how the Collector
		// writes a span whose status is unset.
		e.nothing()
	}
	e.close('}')
	return at
}

// failed says whether a span is recorded as an error: a request the server
// answered with a code of 500 or above.
func (sp span) failed() bool { return sp.level == 0 && sp.status >= serverError }

// requestAttributes are what the HTTP conventions ask a server span to carry,
// under the names version 1.45.0 gives them.
func (r *records) requestAttributes(sp span) {
	e := &r.e
	e.textAttribute("http.request.method", sp.request.method)
	e.textAttribute("http.route", sp.request.route)
	e.textAttribute("url.path", sp.request.path)
	e.textAttribute("url.scheme", "https")
	e.wholeAttribute("http.response.status_code", int64(sp.status))
	e.textAttribute("server.address", serverAddress)
	e.textAttribute("client.address", clientNetwork+strconv.Itoa(sp.client))
	e.textAttribute("user_agent.original", sp.agent)
	if sp.failed() {
		// The conventions ask for the class of the error where a request
		// ended with one, and for a response code that class is the code.
		e.textAttribute("error.type", strconv.Itoa(sp.status))
	}
}
