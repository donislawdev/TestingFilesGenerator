package otlp

import (
	"math"
	"strconv"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
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
)

// The kinds of telemetry a file can hold. One kind to a file, which is what
// the specification of the OTLP file exporter asks.
const (
	Traces = "traces"
	Logs   = "logs"
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
}

// step is how far the clock moves from one trace or log record to the next.
func (s settings) step() int64 { return int64(time.Second) / int64(s.rate) }

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
	s := settings{
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

// misplaced refuses a setting somebody chose that can do nothing in this
// file: one that shapes spans beside logs, one that shapes log records beside
// traces, and a batch beside a layout that has none.
//
// Only a real choice can be out of place. A window sends every setting it
// draws, at its default, so a key that arrived says nothing - the same lesson
// the log format wrote down on 2026-08-31.
func misplaced(props map[string]string, s settings) error {
	for _, p := range properties() {
		v, chosen := props[p.Name]
		if !chosen || v == "" || v == p.Default {
			continue
		}
		if why, other := outOfPlace(p.Name, s); !why.IsZero() {
			return conflict(p.Name, other, v, why)
		}
	}
	return nil
}

// outOfPlace says why a setting does nothing beside the rest of s, and which
// other setting it disagrees with. Nothing when it has work to do.
func outOfPlace(name string, s settings) (why core.Said, other string) {
	chosen := core.Choice{Of: Signal, Value: s.signal}
	switch name {
	case Depth, StatusMix:
		if s.signal != Traces {
			return core.Says("otlp.ItShapesSpansAndSignal", "it shapes spans, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
	case LevelMix, TraceContext:
		if s.signal != Logs {
			return core.Says("otlp.ItShapesLogRecordsAndSignal", "it shapes log records, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
	case Batch:
		if !s.layout.lines {
			return core.Says("otlp.OnlyOneLayoutHasBatches", "only %s cuts the file into batches, and %s writes one request",
				core.A("Lines", core.Choice{Of: Formatting, Value: RecordPerLine}),
				core.A("Layout", core.Choice{Of: Formatting, Value: s.layout.name})), Formatting
		}
	}
	return core.Said{}, ""
}

// conflict names both settings, because naming one of a pair leaves the reader
// to guess which of the two to change.
func conflict(one, other, val string, why core.Said) error {
	return &format.PropertyValueError{
		Format: "otlp", Key: one + " and " + other, Value: val,
		Subject: core.Says("format.TwoSettings", "%s and %s", core.A("Of", core.LabelTerm(one)), core.A("By", core.LabelTerm(other))),
		Reason:  core.Says("otlp.DropOneOfTheTwo", "%s. Drop one of the two, or change the other", core.A("Why", why)),
	}
}

// recorded is what the manifest says about the file: only what is true of
// this signal and this layout, so a test never asserts on a setting that left
// no trace in the bytes.
func (s settings) recorded() map[string]any {
	out := map[string]any{
		"encoding": "utf-8",
		Signal:     s.signal,
		Formatting: s.layout.name,
		Start:      s.startText,
		Rate:       s.rate,
		// Stated even though it is always false here, so a test can assert on
		// it without knowing which formats carry a label internally.
		format.PropertyLabelEmbedded: false,
	}
	if s.layout.lines {
		out[Batch] = s.batch
	}
	if s.signal == Traces {
		out[Depth] = s.depth
		out[StatusMix] = s.statusMix
	} else {
		out[LevelMix] = s.levelMix
		out[TraceContext] = s.traceContext
	}
	return out
}

func properties() []format.Property {
	return append(fileProperties(), recordProperties()...)
}

// fileProperties are the settings every file has, whatever it holds.
func fileProperties() []format.Property {
	return []format.Property{
		{
			Name: Signal, Kind: format.PropertyChoice,
			Choices: []string{Logs, Traces},
			Default: Traces,
			Detail:  "Which kind of telemetry the file holds. traces writes spans and logs writes log records. A file holds one kind, as the OTLP file format asks.",
		},
		{
			Name: Formatting, Kind: format.PropertyChoice,
			Choices: []string{Indented, Minified, RecordPerLine},
			Default: RecordPerLine,
			// A file of one request is one JSON document, and .jsonl would
			// tell a reader that picks its parser by the name to read it line
			// by line. docs/ROZSZERZENIE-UKLADU-2026-11-11.md.
			Extensions: map[string]string{Indented: ".json", Minified: ".json"},
			Detail: "How the file is laid out. record-per-line puts one batch on each line, which is what the OTLP file exporter writes and the file receiver of the OpenTelemetry Collector reads, and the file name ends in .jsonl. " +
				"minified and indented make the whole file one request, the body to post to an OTLP endpoint, and the file name ends in .json. " +
				"Posted as a body, a file of several lines loses every line after the first in the Collector, and its file receiver reads nothing from an indented file.",
		},
		{
			Name: Batch, Kind: format.PropertyInt,
			Min: 1, Max: maxBatch,
			Default: strconv.Itoa(defaultBatch),
			Detail:  "How many spans or log records each line holds. 512 is the batch an OpenTelemetry SDK sends by default. Only record-per-line has batches, and beside another layout a batch is refused.",
		},
		{
			Name: Start, Kind: format.PropertyText,
			Shape:   "a moment in UTC such as 2026-08-01T12:00:00Z",
			Default: defaultStart,
			Detail:  "The moment the first span or log record is dated. The later ones follow it at the rate. Change it when the system that receives the file refuses or hides data that is too old.",
		},
		{
			Name: Rate, Kind: format.PropertyInt,
			Min: 1, Max: maxRate, Unit: "entries per second",
			Default: strconv.Itoa(defaultRate),
			Detail:  "How many traces or log records begin each second.",
		},
	}
}

// recordProperties are the settings that shape one kind of record. Each says
// which kind, because beside the other it is refused.
func recordProperties() []format.Property {
	return []format.Property{
		{
			Name: Depth, Kind: format.PropertyInt,
			Min: 1, Max: maxDepth,
			Default: strconv.Itoa(defaultDepth),
			Detail:  "How many spans each trace has, each one the child of the one before it. 1 leaves every span without a parent. It shapes spans, and beside signal logs it is refused.",
		},
		{
			Name: StatusMix, Kind: format.PropertyChoice,
			Choices: []string{"client-errors", "realistic", "server-errors", "success"},
			Default: "realistic",
			Detail:  "Which response codes the requests end with. A code of 500 or above marks its span as an error, as the HTTP conventions of OpenTelemetry ask, and every other code leaves the status unset. It shapes spans, and beside signal logs it is refused.",
		},
		{
			Name: LevelMix, Kind: format.PropertyChoice,
			Choices: []string{"debug", "errors", "quiet", "realistic"},
			Default: "realistic",
			Detail:  "Which severities the log records carry. Realistic is mostly INFO with some of the others. It shapes log records, and beside signal traces it is refused.",
		},
		{
			Name: TraceContext, Kind: format.PropertyBool,
			Default: "true",
			Detail:  "Whether every log record names the trace and the span it was written in. The file holds no spans, so those ids match nothing in it. It shapes log records, and beside signal traces it is refused.",
		},
	}
}
