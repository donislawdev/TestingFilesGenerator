package otlp

import (
	"math"
	"strconv"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// The names of the settings, for the guards that set them.
const (
	Signal       = "signal"
	Formatting   = "formatting"
	Batch        = "batch"
	Start        = "start"
	Rate         = "rate"
	Depth        = "depth"
	StatusMix    = "status_mix"
	LevelMix     = "level_mix"
	TraceContext = "trace_context"
	MetricKind   = "metric_kind"
	Temporality  = "temporality"
	Exemplars    = "exemplars"
	Interval     = "interval"

	IDEncoding     = "id_encoding"
	EnumEncoding   = "enum_encoding"
	KeyCase        = "key_case"
	LargeNumbers   = "large_numbers"
	BOM            = textenc.SettingBOM
	TimeOrder      = "time_order"
	EndBeforeStart = "end_before_start"
	EmptyEnvelope  = "empty_envelope"
	Body           = "body"
	NonFinite      = "non_finite"
)

// The values of the pitfalls, the first of each pair the spelling of the
// specification.
const (
	Hex        = "hex"
	UpperHex   = "upper-hex"
	Base64     = "base64"
	Number     = "number"
	Name       = "name"
	Camel      = "camel"
	Snake      = "snake"
	AsText     = "text"
	InOrder    = "in-order"
	OutOfOrder = "out-of-order"
	AnyValue   = "any-value"
)

// The kinds of telemetry a file can hold. One kind to a file, which is what
// the specification of the OTLP file exporter asks.
const (
	Traces  = "traces"
	Logs    = "logs"
	Metrics = "metrics"
)

// The kinds of metric, under the names metric_kind gives them: one for each
// kind of data metrics.proto has, and all of them together.
const (
	Gauge                = "gauge"
	Sum                  = "sum"
	Histogram            = "histogram"
	ExponentialHistogram = "exponential-histogram"
	Summary              = "summary"
	Mixed                = "mixed"
)

// The aggregation temporalities, under the names temporality gives them.
const (
	Cumulative = "cumulative"
	Delta      = "delta"
)

// The blocks the settings that shape one kind of record are drawn in.
const (
	spansGroup    = "Spans"
	logsGroup     = "Log records"
	metricsGroup  = "Metrics"
	pitfallsGroup = "Pitfalls"
)

// The layouts, under the names json and geojson use for theirs.
const (
	Indented      = "indented"
	Minified      = "minified"
	RecordPerLine = "record-per-line"
)

const (
	// defaultBatch is the batch an OpenTelemetry SDK exports by default,
	// maxExportBatchSize in the specification of the batching span processor.
	defaultBatch = 512
	// maxBatch keeps the longest line a batch can make under lineCeiling, and
	// a guard measures that it does.
	maxBatch = 1000

	defaultRate = 1
	maxRate     = 1_000_000

	defaultDepth = 3
	maxDepth     = 100

	// defaultInterval is how often the periodic reader of an SDK collects,
	// exportIntervalMillis in metrics/sdk.md of the specification. A day at
	// the most.
	defaultInterval = 60
	maxInterval     = 86400

	// defaultStart is the moment the log format starts at, so two files of
	// one run tell the same time.
	defaultStart = "2026-08-01T12:00:00Z"
	startLayout  = "2006-01-02T15:04:05Z"
	firstYear    = 2002
	lastYear     = 2199

	// lastNanos is the last moment a count of nanoseconds in 64 bits with a
	// sign can name, in April 2262. From firstYear to there every time has
	// nineteen digits, which is what lets the length of a record be known
	// before it is drawn.
	lastNanos = math.MaxInt64

	// leastRecordBytes is a length no record is shorter than: its two times
	// alone, with their keys, are longer. It bounds how many records a file of
	// a given size can hold.
	leastRecordBytes = 64

	// LineCeiling is the longest line the file receiver of the Collector reads
	// with its default settings, without the line ending. One byte more and
	// the line is dropped without a word, measured on 2026-11-10.
	LineCeiling = 1 << 20
)

// settings is one file's worth of choices, read and checked.
type settings struct {
	signal       string
	layout       layout
	batch        int
	startText    string
	start        int64
	rate         int
	depth        int
	statusMix    string
	statuses     []int
	levelMix     string
	levels       []level
	traceContext bool

	metricChoices
	pitfallChoices
}

// metricChoices are what shapes a file of metrics.
type metricChoices struct {
	metricKind  string
	instruments []*instrument
	temporality string
	exemplars   bool
	// interval is how many seconds lie between two collections.
	interval int64
}

// pitfallChoices are the pitfalls. Each is off, or spelled the way the
// specification asks, unless chosen.
type pitfallChoices struct {
	idEncoding     string
	enumNames      bool
	snakeKeys      bool
	bareNumbers    bool
	bom            bool
	outOfOrder     bool
	endBeforeStart bool
	emptyEnvelope  bool
	anyBody        bool
	nonFinite      bool
}

// order is the place on the clock of the trace, log record or collection a
// file draws as the nth: the nth itself, or with time_order out-of-order its
// neighbour, so every second one is dated before the one in front of it.
func (s settings) order(n int64) int64 {
	if s.outOfOrder {
		return n ^ 1
	}
	return n
}

// prologue is what the file opens with before its first batch: a byte order
// mark, and a line holding an empty envelope.
func (s settings) prologue() []byte {
	var out []byte
	if s.bom {
		out = append(out, 0xEF, 0xBB, 0xBF)
	}
	if s.emptyEnvelope {
		e := &emitter{sp: s.spelling()}
		e.open('{')
		e.key(envelopes[s.signal].resources)
		e.b = append(e.b, '[', ']', '}', '\n')
		out = append(out, e.b...)
	}
	return out
}

// step is how far the clock moves from one trace, log record or collection to
// the next.
func (s settings) step() int64 {
	if s.signal == Metrics {
		return s.interval * int64(time.Second)
	}
	return int64(time.Second) / int64(s.rate)
}

// points is how many points a collection of these settings holds.
func (s settings) points() int {
	n := 0
	for _, inst := range s.instruments {
		n += len(inst.series)
	}
	return n
}

func parse(props map[string]string) (settings, error) {
	values := map[string]string{}
	for _, p := range properties() {
		v, err := valueOf(props, p)
		if err != nil {
			return settings{}, err
		}
		values[p.Name] = v
	}
	batch, _ := strconv.Atoi(values[Batch])
	rate, _ := strconv.Atoi(values[Rate])
	depth, _ := strconv.Atoi(values[Depth])
	interval, _ := strconv.ParseInt(values[Interval], 10, 64)
	s := settings{
		metricChoices: metricChoices{
			metricKind:  values[MetricKind],
			instruments: kinds[values[MetricKind]],
			temporality: values[Temporality],
			exemplars:   values[Exemplars] == "true",
			interval:    interval,
		},
		pitfallChoices: pitfallChoices{
			idEncoding:     values[IDEncoding],
			enumNames:      values[EnumEncoding] == Name,
			snakeKeys:      values[KeyCase] == Snake,
			bareNumbers:    values[LargeNumbers] == Number,
			bom:            values[BOM] == "true",
			outOfOrder:     values[TimeOrder] == OutOfOrder,
			endBeforeStart: values[EndBeforeStart] == "true",
			emptyEnvelope:  values[EmptyEnvelope] == "true",
			anyBody:        values[Body] == AnyValue,
			nonFinite:      values[NonFinite] == "true",
		},

		signal:       values[Signal],
		layout:       layouts[values[Formatting]],
		batch:        batch,
		startText:    values[Start],
		rate:         rate,
		depth:        depth,
		statusMix:    values[StatusMix],
		statuses:     statusSets[values[StatusMix]],
		levelMix:     values[LevelMix],
		levels:       levelSets[values[LevelMix]],
		traceContext: values[TraceContext] == "true",
	}
	start, err := readStart(s.startText)
	if err != nil {
		return settings{}, err
	}
	s.start = start
	if s.signal == Metrics {
		// A line of metrics is one collection, so the batch is as many
		// points as a collection holds rather than a number anybody chose.
		s.batch = s.points()
	}
	return s, misplaced(props, s)
}

func valueOf(props map[string]string, p format.Property) (string, error) {
	v, ok := props[p.Name]
	if !ok || v == "" {
		return p.Default, nil
	}
	if bad := p.Allows(v); !bad.IsZero() {
		return "", &format.PropertyValueError{Format: "otlp", Key: p.Name, Value: v, Reason: bad, Remedy: p.Instead()}
	}
	return v, nil
}

// readStart reads the moment the file starts at, as nanoseconds since 1970.
//
// One spelling only, the one `date -u +%FT%TZ` prints. Go's own parser takes a
// fraction of a second after the seconds even when the layout has none, so the
// moment is written back and compared rather than trusted.
func readStart(raw string) (int64, error) {
	at, err := time.Parse(startLayout, raw)
	if err != nil || at.Format(startLayout) != raw {
		return 0, &format.PropertyValueError{
			Format: "otlp", Key: Start, Value: raw,
			Reason: core.Says("otlp.ItTakesAMomentInUTC", "it takes a moment in UTC written as 2026-08-01T12:00:00Z"),
			Remedy: core.Says("otlp.WriteTheMomentThatWay", "Write the date, a T, the time to the second and a Z, with no fraction of a second."),
		}
	}
	if at.Year() < firstYear || at.Year() > lastYear {
		return 0, &format.PropertyValueError{
			Format: "otlp", Key: Start, Value: raw,
			Reason: core.Says("otlp.ItHasToLieInTheYears", "it has to lie in the years %d to %d", core.A("First", firstYear), core.A("Last", lastYear)),
			Remedy: core.Says("otlp.ChooseAStartInThoseYears", "Choose a start in those years, such as 2026-08-01T12:00:00Z."),
		}
	}
	return at.UnixNano(), nil
}

// endsInTime refuses a file whose last record would be dated past the last
// time this format writes.
//
// Reachable only by a file of hundreds of gigabytes at a slow rate, and worth
// a refusal all the same: past that moment a time grows a digit, the record
// that carries it is a byte longer than planned, and the file would come out
// the wrong size somewhere near its end.
func (s settings) endsInTime(bytes int64) error {
	if s.signal == Metrics {
		return s.collectsInTime(bytes)
	}
	ticks := bytes / leastRecordBytes
	if room := (lastNanos - s.start - int64(longestSpan)) / s.step(); ticks <= room {
		return nil
	}
	// The advice stands in the reason, the way the log format writes a
	// conflict: the command line prints the reason and nothing after it, and
	// no range declared for start could say what to do here.
	return &format.PropertyValueError{
		Format: "otlp", Key: Start, Value: s.startText,
		Reason: core.Says("otlp.AFileThisLargeEndsAfter",
			"a file of this size at a rate of %d would date its last records after the year 2262. Choose an earlier start, a higher rate or a smaller file",
			core.A("Rate", s.rate)),
	}
}

// collectsInTime is endsInTime for a file of metrics, which counts
// collections rather than records, and asks one thing more.
//
// The time a cumulative series has measured grows with every collection and
// is written to the microsecond. Past microCeiling a sum would need a seventh
// decimal to be read back as written, so a file that could reach it is
// refused - a file of hundreds of gigabytes at the least, and worth the
// refusal for the reason endsInTime gives.
func (s settings) collectsInTime(bytes int64) error {
	// One more than the file can hold, and one more for time_order
	// out-of-order, which dates a collection where the next would stand.
	collections := bytes/(leastRecordBytes*int64(s.points())) + 2
	if room := (lastNanos - s.start) / s.step(); collections > room {
		return &format.PropertyValueError{
			Format: "otlp", Key: Start, Value: s.startText,
			Reason: core.Says("otlp.AFileThisLargeCollectsAfter",
				"a file of this size collected every %d s would date its last collections after the year 2262. Choose an earlier start, a longer interval or a smaller file",
				core.A("Interval", s.interval)),
		}
	}
	for _, inst := range s.instruments {
		measures := inst.shape == shapeHistogram || inst.shape == shapeExponential || inst.shape == shapeSummary
		if measures && s.cumulative(inst) && collections > (microCeiling-1)/(mostMeasurements*longestDuration) {
			return &format.PropertyValueError{
				Format: "otlp", Key: MetricKind, Value: s.metricKind,
				Reason: core.Says("otlp.AFileThisLargeAddsUpMore",
					"a file of this size would add up more time in one series of %s than can be written to the microsecond. Choose a smaller file, or temporality delta where the kind has it",
					core.A("Metric", inst.name)),
			}
		}
	}
	return nil
}
