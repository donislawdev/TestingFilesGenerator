package otlp

// What a file of metrics measures: the instruments a Java service exports
// beside the requests it serves. Every name, unit and attribute but two is
// marked stable in version 1.45.0 of the semantic conventions, read on
// 2026-11-11 from docs/http/http-metrics.md and docs/runtime/jvm-metrics.md.
// The conventions name no stable gauge of whole numbers, so the queue of
// orders is the service's own, under its own prefix. The summary is what
// Micrometer, the metrics library of Spring, writes for a timer with
// percentiles, because no convention defines one.

// shape is the kind of data a metric carries, as metrics.proto names the
// member that holds it.
type shape int

const (
	shapeGauge shape = iota
	shapeSum
	shapeHistogram
	shapeExponential
	shapeSummary
)

// member is the name of the object that holds the data points of each shape.
var member = [...]string{
	shapeGauge:       "gauge",
	shapeSum:         "sum",
	shapeHistogram:   "histogram",
	shapeExponential: "exponentialHistogram",
	shapeSummary:     "summary",
}

// temporal says whether a shape states its aggregation temporality. A gauge
// and a summary have none.
func (sh shape) temporal() bool {
	return sh == shapeSum || sh == shapeHistogram || sh == shapeExponential
}

// The aggregation temporalities as metrics.proto numbers them.
const (
	temporalityDelta      = 1
	temporalityCumulative = 2
)

// temporalityNames are the names metrics.proto gives the two, for a file that
// spells an enum by its name.
var temporalityNames = map[int64]string{
	temporalityDelta:      "AGGREGATION_TEMPORALITY_DELTA",
	temporalityCumulative: "AGGREGATION_TEMPORALITY_CUMULATIVE",
}

// values is what one instrument measures, which decides how its points are
// drawn.
type values int

const (
	cpuUtilization values = iota
	ordersPending
	cpuTime
	classesLoaded
	memoryUsed
	requestDuration
	gcDuration
	requestTimer
)

// instrument is one metric a file holds in every collection.
type instrument struct {
	name, description, unit string
	shape                   shape
	values                  values
	// whole says the points carry a whole number, asInt, rather than asDouble.
	// For a gauge and a sum only.
	whole bool
	// monotonic is a sum that only grows.
	monotonic bool
	// updown is an UpDownCounter, which stays cumulative when delta is asked:
	// the delta preference of an SDK keeps every UpDownCounter cumulative,
	// sdk_exporters/otlp.md of the specification, version 1.62.0.
	updown bool
	// exemplary is measured inside a request, which is what lets a point of
	// it name the trace it was measured in. The jvm instruments are read by a
	// callback when the collection runs, outside every span.
	exemplary bool
	series    []series
}

// series is one set of attributes an instrument reports a point for, in the
// order the conventions list them.
type series []attribute

// attribute is one attribute of a series: text, or a whole number.
type attribute struct {
	key, text string
	number    int64
}

// whole says the attribute holds a number. Every number this vocabulary uses
// is above nought.
func (a attribute) whole() bool { return a.number != 0 }

// httpSeries are the requests the duration of every request is reported for:
// the routes of the trace vocabulary with the code each usually ends in, and
// three that end otherwise. The order of the attributes is the one the table
// of http.server.request.duration gives.
var httpSeries = func() []series {
	type ended struct {
		request request
		status  int64
	}
	var all []ended
	for _, q := range requests {
		code := int64(200)
		switch q.method {
		case "POST":
			code = 201
			if q.route == "/login" {
				code = 200
			}
		case "DELETE":
			code = 204
		}
		all = append(all, ended{q, code})
	}
	all = append(all,
		ended{request{"POST", "/login", "/login"}, 401},
		ended{request{"GET", "/api/v1/orders/{id}", "/api/v1/orders/4711"}, 404},
		ended{request{"POST", "/api/v1/orders", "/api/v1/orders"}, 503},
	)
	out := make([]series, 0, len(all))
	for _, x := range all {
		s := series{{key: "http.request.method", text: x.request.method}, {key: "url.scheme", text: "https"}}
		if x.status >= serverError {
			s = append(s, attribute{key: "error.type", text: statusText(x.status)})
		}
		s = append(s, attribute{key: "http.response.status_code", number: x.status}, attribute{key: "http.route", text: x.request.route})
		out = append(out, s)
	}
	return out
}()

// timerSeries are the same requests as Micrometer tags them: every tag is
// text, and they come sorted by name.
var timerSeries = func() []series {
	out := make([]series, 0, len(httpSeries))
	for _, s := range httpSeries {
		var method, route string
		var code int64
		for _, a := range s {
			switch a.key {
			case "http.request.method":
				method = a.text
			case "http.route":
				route = a.text
			case "http.response.status_code":
				code = a.number
			}
		}
		outcome := "SUCCESS"
		switch {
		case code >= serverError:
			outcome = "SERVER_ERROR"
		case code >= 400:
			outcome = "CLIENT_ERROR"
		}
		out = append(out, series{
			{key: "exception", text: "none"}, {key: "method", text: method}, {key: "outcome", text: outcome},
			{key: "status", text: statusText(code)}, {key: "uri", text: route},
		})
	}
	return out
}()

// statusText is a response code as text, built once while the vocabulary is.
func statusText(code int64) string {
	return string([]byte{byte('0' + code/100), byte('0' + code/10%10), byte('0' + code%10)})
}

var gcSeries = []series{
	{{key: "jvm.gc.action", text: "end of minor GC"}, {key: "jvm.gc.name", text: "G1 Young Generation"}},
	{{key: "jvm.gc.action", text: "end of major GC"}, {key: "jvm.gc.name", text: "G1 Old Generation"}},
}

// memoryPools are the pools jvm.memory.used reports, each with the range its
// use moves in, in bytes.
var memoryPools = []struct {
	series series
	low    int64
	high   int64
}{
	{series{{key: "jvm.memory.pool.name", text: "G1 Eden Space"}, {key: "jvm.memory.type", text: "heap"}}, 16 << 20, 256 << 20},
	{series{{key: "jvm.memory.pool.name", text: "G1 Old Gen"}, {key: "jvm.memory.type", text: "heap"}}, 64 << 20, 1 << 30},
	{series{{key: "jvm.memory.pool.name", text: "Metaspace"}, {key: "jvm.memory.type", text: "non_heap"}}, 48 << 20, 80 << 20},
}

var (
	instCPUUtilization = &instrument{
		name: "jvm.cpu.recent_utilization", description: "Recent CPU utilization for the process as reported by the JVM.", unit: "1",
		shape: shapeGauge, values: cpuUtilization, series: []series{nil},
	}
	instOrdersPending = &instrument{
		name: "shop.orders.pending", description: "Orders accepted and not yet shipped.", unit: "{order}",
		shape: shapeGauge, values: ordersPending, whole: true, series: []series{nil},
	}
	instCPUTime = &instrument{
		name: "jvm.cpu.time", description: "CPU time used by the process as reported by the JVM.", unit: "s",
		shape: shapeSum, values: cpuTime, monotonic: true, series: []series{nil},
	}
	instClassesLoaded = &instrument{
		name: "jvm.class.loaded", description: "Number of classes loaded since JVM start.", unit: "{class}",
		shape: shapeSum, values: classesLoaded, whole: true, monotonic: true, series: []series{nil},
	}
	instMemoryUsed = &instrument{
		name: "jvm.memory.used", description: "Measure of memory used.", unit: "By",
		shape: shapeSum, values: memoryUsed, whole: true, updown: true,
		series: []series{memoryPools[0].series, memoryPools[1].series, memoryPools[2].series},
	}
	instRequestDuration = &instrument{
		name: "http.server.request.duration", description: "Duration of HTTP server requests.", unit: "s",
		shape: shapeHistogram, values: requestDuration, exemplary: true, series: httpSeries,
	}
	instGCDuration = &instrument{
		name: "jvm.gc.duration", description: "Duration of JVM garbage collection actions.", unit: "s",
		shape: shapeExponential, values: gcDuration, series: gcSeries,
	}
	instRequestTimer = &instrument{
		name: "http.server.requests", description: "Duration of HTTP server request handling.", unit: "s",
		shape: shapeSummary, values: requestTimer, series: timerSeries,
	}
)

// kinds are the instruments each metric_kind writes in every collection, in
// that order.
var kinds = map[string][]*instrument{
	Gauge:                {instCPUUtilization, instOrdersPending},
	Sum:                  {instCPUTime, instClassesLoaded, instMemoryUsed},
	Histogram:            {instRequestDuration},
	ExponentialHistogram: {instGCDuration},
	Summary:              {instRequestTimer},
	Mixed: {instCPUUtilization, instOrdersPending, instCPUTime, instClassesLoaded, instMemoryUsed,
		instRequestDuration, instGCDuration, instRequestTimer},
}

// requestBounds are the bucket boundaries the conventions ask
// http.server.request.duration to be recorded with, in millionths of a
// second.
var requestBounds = []micros{5000, 10000, 25000, 50000, 75000, 100000, 250000, 500000, 750000,
	1_000_000, 2_500_000, 5_000_000, 7_500_000, 10_000_000}

// The exponential histogram of a GC duration, at scale 3: eight buckets to
// every doubling. A duration lies between a millisecond and two seconds, which
// the 88 buckets from index -80 to 7 hold - fewer than the 160 an SDK keeps by
// default, so an SDK would not scale down further.
const (
	exponentialScale = 3
	firstBucket      = -80
	bucketCount      = 88
)

// bucketTops is where each exponential bucket ends, from index -80 up: the
// largest whole number of microseconds not above 2^((i+1)/8) seconds. A
// duration is a whole number of microseconds, so lying at or below the value
// in this table is lying at or below the bound itself. Written out rather
// than computed, because computing it takes a logarithm in floating point. A
// guard checks each entry against the eighth power of the bound in exact
// arithmetic.
var bucketTops = [bucketCount]micros{
	1064, 1161, 1266, 1381, 1506, 1642, 1791, 1953,
	2129, 2322, 2532, 2762, 3012, 3284, 3582, 3906,
	4259, 4645, 5065, 5524, 6024, 6569, 7164, 7812,
	8519, 9290, 10131, 11048, 12048, 13139, 14328, 15625,
	17039, 18581, 20263, 22097, 24097, 26278, 28656, 31250,
	34078, 37162, 40526, 44194, 48194, 52556, 57312, 62500,
	68156, 74325, 81052, 88388, 96388, 105112, 114625, 125000,
	136313, 148650, 162104, 176776, 192776, 210224, 229251, 250000,
	272626, 297301, 324209, 353553, 385552, 420448, 458502, 500000,
	545253, 594603, 648419, 707106, 771105, 840896, 917004, 1000000,
	1090507, 1189207, 1296839, 1414213, 1542210, 1681792, 1834008, 2000000,
}

// BucketTops is the table above with the index of its first bucket and the
// scale, for the guard that checks it.
func BucketTops() (scale, first int, tops []int64) {
	return exponentialScale, firstBucket, bucketTops[:]
}

// band is a range durations are drawn from, in microseconds. Repeats in a
// list are the weighting.
type band struct{ low, high micros }

var (
	// requestBands: most requests are quick, some slow, few very slow - from a
	// millisecond to two seconds, the range the exponential table holds.
	requestBands = []band{{1000, 20000}, {1000, 20000}, {1000, 20000}, {1000, 20000},
		{20000, 150000}, {20000, 150000}, {20000, 150000}, {150000, 800000}, {150000, 800000}, {800000, 2_000_000}}
	// gcBands by action: a young collection pauses briefly and an old one
	// longer.
	gcBands = [][]band{
		{{1000, 8000}, {1000, 8000}, {8000, 50000}},
		{{50000, 300000}, {50000, 300000}, {300000, 2_000_000}},
	}
)

// longestDuration is the longest duration any band draws.
const longestDuration micros = 2_000_000

// timerQuantiles are the percentiles the summary reports, in millionths.
var timerQuantiles = []micros{500000, 950000, 990000}

// mostMeasurements is how many measurements a series can take between two
// collections. Few, so a closing point stays short and every count of the file
// grows by a bounded step.
const mostMeasurements = 3
