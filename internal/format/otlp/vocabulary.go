package otlp

import "time"

// What every file says about where its records come from. One service and one
// scope: a batch a single process exported, which is what an OTLP file from an
// SDK is.
const (
	serviceName    = "shop"
	serviceVersion = "1.4.2"
	scopeName      = "shop.server"
	serverAddress  = "shop.example.com"

	// schemaURL is the version of the semantic conventions the attribute
	// names below were read from on 2026-11-10. Every one of them is marked
	// stable there.
	schemaURL = "https://opentelemetry.io/schemas/1.45.0"

	// clientNetwork is one of the three networks RFC 5737 sets aside for
	// documentation, so no address in a file belongs to anybody.
	clientNetwork = "203.0.113."
	clientHosts   = 254
)

const (
	// shortestSpan and longestSpan are how long a request takes. The longest
	// is counted in when a file is checked to end in time.
	shortestSpan = 5 * time.Millisecond
	longestSpan  = 500 * time.Millisecond

	// The lines a log record says it was written at, three digits wide at
	// most, so the widest is what the smallest file leaves room for.
	firstLine = 10
	lastLine  = 999
)

// request is one thing a client asks the service for: the method, the route
// that matched and the path that was sent.
//
// A route and a path rather than a path alone, because the HTTP conventions
// name a span after the route and forbid naming it after the path - a path
// carries an id, and a span name is meant to be the same for every order.
type request struct {
	method, route, path string
}

var requests = []request{
	{"GET", "/", "/"},
	{"GET", "/health", "/health"},
	{"GET", "/search", "/search"},
	{"POST", "/login", "/login"},
	{"GET", "/static/{file}", "/static/app.css"},
	{"GET", "/api/v1/orders", "/api/v1/orders"},
	{"POST", "/api/v1/orders", "/api/v1/orders"},
	{"GET", "/api/v1/orders/{id}", "/api/v1/orders/4711"},
	{"PUT", "/api/v1/orders/{id}", "/api/v1/orders/4711"},
	{"DELETE", "/api/v1/orders/{id}", "/api/v1/orders/1024"},
	{"GET", "/api/v1/customers/{id}", "/api/v1/customers/308"},
	{"GET", "/api/v1/invoices", "/api/v1/invoices"},
	{"GET", "/api/v1/invoices/{id}/pdf", "/api/v1/invoices/9001/pdf"},
}

// statusSets are the mixes of response codes, under the names the log format
// gives the same four. Repeats are the weighting.
var statusSets = map[string][]int{
	"realistic":     {200, 200, 200, 201, 204, 301, 302, 304, 400, 401, 403, 404, 409, 429, 500, 502, 503},
	"success":       {200, 200, 201, 204, 206},
	"client-errors": {400, 401, 403, 404, 409, 410, 422, 429},
	"server-errors": {500, 500, 502, 503, 504},
}

// serverError is the first response code the HTTP conventions tell a server
// span to record as an error. A code of the 4xx range leaves the status unset
// on a server span.
const serverError = 500

var agents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) Chrome/126.0.0.0 Safari/537.36",
	"curl/8.7.1",
	"python-requests/2.32.3",
	"Go-http-client/2.0",
}

// functions are what the service does inside a request, and where a log
// record says it was written.
var functions = []string{
	"shop.auth.authorize", "shop.cache.lookup", "shop.orders.load", "shop.orders.validate",
	"shop.stock.reserve", "shop.payments.charge", "shop.invoices.render", "shop.mail.send",
}

// level is one severity: the word a log record carries and the number the
// OTLP data model gives it.
type level struct {
	text   string
	number int64
	// messages are what a record of this severity says, so an ERROR never
	// reports that an order was accepted.
	messages []string
}

// The four severities, with the numbers logs.proto gives the first of each
// range.
var (
	levelDebug = level{"DEBUG", 5, []string{"cache miss", "loading order", "query finished", "stock checked"}}
	levelInfo  = level{"INFO", 9, []string{"order accepted", "invoice sent", "customer signed in", "report generated"}}
	levelWarn  = level{"WARN", 13, []string{"slow query", "retrying payment", "stock running low"}}
	levelError = level{"ERROR", 17, []string{"payment refused", "invoice could not be rendered", "upstream timed out"}}
)

// levelSets are the mixes of severities, under the names and in the shares the
// log format gives the same four. Repeats are the weighting.
var levelSets = map[string][]level{
	"realistic": {levelInfo, levelInfo, levelInfo, levelWarn, levelError, levelDebug},
	"quiet":     {levelInfo},
	"errors":    {levelError, levelError, levelError, levelWarn, levelWarn, levelInfo},
	"debug":     {levelDebug, levelDebug, levelDebug, levelInfo, levelInfo, levelWarn},
}

// words fill the padding attribute of the last record. Words and single
// spaces only - a JSON string would otherwise need escaping, and an escape
// costs a byte the size arithmetic did not budget for.
var words = []string{
	"account", "address", "amount", "balance", "batch", "buyer", "carrier", "contract",
	"credit", "customer", "delivery", "deposit", "discount", "invoice", "ledger", "order",
	"parcel", "payment", "receipt", "refund", "region", "reseller", "shipment", "supplier",
}

// longest is the longest of a list of texts, because the smallest file has to
// leave room for the widest draw rather than for the lucky one.
func longest(texts []string) string {
	out := ""
	for _, t := range texts {
		if len(t) > len(out) {
			out = t
		}
	}
	return out
}
