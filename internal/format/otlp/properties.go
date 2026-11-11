package otlp

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// recorded is what the manifest says about the file: only what is true of
// this signal and this layout, so a test never asserts on a setting that left
// no trace in the bytes.
func (s settings) recorded() map[string]any {
	out := map[string]any{
		"encoding": "utf-8",
		Signal:     s.signal,
		Formatting: s.layout.name,
		Start:      s.startText,
		// Stated even though it is always false here, so a test can assert on
		// it without knowing which formats carry a label internally.
		format.PropertyLabelEmbedded: false,
	}
	s.recordPitfalls(out)
	switch s.signal {
	case Traces:
		out[Depth] = s.depth
		out[StatusMix] = s.statusMix
	case Logs:
		out[LevelMix] = s.levelMix
		out[TraceContext] = s.traceContext
	case Metrics:
		s.recordMetrics(out)
		return out
	}
	out[Rate] = s.rate
	if s.layout.lines {
		out[Batch] = s.batch
	}
	return out
}

// recordMetrics states what shapes a file of metrics: the kind and the
// interval, and the temporality and the exemplars where the kind has them.
func (s settings) recordMetrics(out map[string]any) {
	out[MetricKind] = s.metricKind
	out[Interval] = s.interval
	for _, inst := range s.instruments {
		if inst.shape.temporal() {
			out[Temporality] = s.temporality
		}
		if inst.exemplary {
			out[Exemplars] = s.exemplars
		}
	}
}

// recordPitfalls states every pitfall that can reach this file, chosen or not,
// so a test can assert that a file is spelled the way the specification asks.
// A pitfall that has nothing to change here is left out, by the same rule
// that refuses it when it is chosen.
func (s settings) recordPitfalls(out map[string]any) {
	values := map[string]any{
		IDEncoding: s.idEncoding, EnumEncoding: pick2(s.enumNames, Name, Number), KeyCase: pick2(s.snakeKeys, Snake, Camel),
		LargeNumbers: pick2(s.bareNumbers, Number, AsText), BOM: s.bom, TimeOrder: pick2(s.outOfOrder, OutOfOrder, InOrder),
		EndBeforeStart: s.endBeforeStart, EmptyEnvelope: s.emptyEnvelope, Body: pick2(s.anyBody, AnyValue, AsText), NonFinite: s.nonFinite,
	}
	for _, p := range pitfallProperties() {
		if why, _ := outOfPlace(p.Name, s); why.IsZero() {
			out[p.Name] = values[p.Name]
		}
	}
}

func pick2(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

func properties() []format.Property {
	return append(append(fileProperties(), recordProperties()...), pitfallProperties()...)
}

// fileProperties are the settings every file has, whatever it holds.
func fileProperties() []format.Property {
	return []format.Property{
		{
			Name: Signal, Kind: format.PropertyChoice,
			Choices: []string{Logs, Metrics, Traces},
			Default: Traces,
			Detail:  "Which kind of telemetry the file holds. traces writes spans, logs writes log records and metrics writes the measurements of a service, collected every interval. A file holds one kind, as the OTLP file format asks.",
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
		// The byte order mark every text format of this tool declares, in the
		// words they all share. What the readers of OTLP do with it - the
		// Collector drops the first batch without a word - was measured with
		// the pitfalls (docs/OTLP-2026-11-10.md section 14).
		textenc.Axes(textenc.SettingBOM)[0],
		{
			Name: Batch, Kind: format.PropertyInt,
			Min: 1, Max: maxBatch,
			Default: strconv.Itoa(defaultBatch),
			Detail:  "How many spans or log records each line holds. 512 is the batch an OpenTelemetry SDK sends by default. Only record-per-line has batches, and beside another layout or beside signal metrics, where a line holds one collection, a batch is refused.",
		},
		{
			Name: Start, Kind: format.PropertyText,
			Shape:   "a moment in UTC such as 2026-08-01T12:00:00Z",
			Default: defaultStart,
			Detail:  "The moment the first span or log record is dated, and the moment a service writing metrics started. The later ones follow it at the rate or every interval. Change it when the system that receives the file refuses or hides data that is too old.",
		},
		{
			Name: Rate, Kind: format.PropertyInt,
			Min: 1, Max: maxRate, Unit: "entries per second",
			Default: strconv.Itoa(defaultRate),
			Detail:  "How many traces or log records begin each second. Metrics are collected every interval instead, and beside signal metrics a rate is refused.",
		},
	}
}

// recordProperties are the settings that shape one kind of record. Each says
// which kind, because beside the others it is refused.
func recordProperties() []format.Property {
	return append(spanAndLogProperties(), metricProperties()...)
}

// spanAndLogProperties shape spans and log records, in a block for each.
func spanAndLogProperties() []format.Property {
	return []format.Property{
		{
			Name: Depth, Kind: format.PropertyInt,
			Min: 1, Max: maxDepth,
			Default: strconv.Itoa(defaultDepth),
			Group:   spansGroup,
			Detail:  "How many spans each trace has, each one the child of the one before it. 1 leaves every span without a parent. It shapes spans, and beside signal logs or metrics it is refused.",
		},
		{
			Name: StatusMix, Kind: format.PropertyChoice,
			Choices: []string{"client-errors", "realistic", "server-errors", "success"},
			Default: "realistic",
			Group:   spansGroup,
			Detail:  "Which response codes the requests end with. A code of 500 or above marks its span as an error, as the HTTP conventions of OpenTelemetry ask, and every other code leaves the status unset. It shapes spans, and beside signal logs or metrics it is refused.",
		},
		{
			Name: LevelMix, Kind: format.PropertyChoice,
			Choices: []string{"debug", "errors", "quiet", "realistic"},
			Default: "realistic",
			Group:   logsGroup,
			Detail:  "Which severities the log records carry. Realistic is mostly INFO with some of the others. It shapes log records, and beside signal traces or metrics it is refused.",
		},
		{
			Name: TraceContext, Kind: format.PropertyBool,
			Default: "true",
			Group:   logsGroup,
			Detail:  "Whether every log record names the trace and the span it was written in. The file holds no spans, so those ids match nothing in it. It shapes log records, and beside signal traces or metrics it is refused.",
		},
	}
}

// metricProperties shape a file of metrics.
func metricProperties() []format.Property {
	return []format.Property{
		{
			Name: MetricKind, Kind: format.PropertyChoice,
			Choices: []string{ExponentialHistogram, Gauge, Histogram, Mixed, Sum, Summary},
			Default: Mixed,
			Group:   metricsGroup,
			Detail: "Which metrics the file holds. gauge writes jvm.cpu.recent_utilization and shop.orders.pending, sum writes jvm.cpu.time, jvm.class.loaded and jvm.memory.used, " +
				"histogram writes http.server.request.duration in the buckets the OpenTelemetry conventions name, exponential-histogram writes jvm.gc.duration in buckets that double every eight, " +
				"and summary writes http.server.requests as three percentiles, the way Micrometer writes a timer. mixed writes all of them. gauge and sum each hold whole numbers and fractions.",
		},
		{
			Name: Temporality, Kind: format.PropertyChoice,
			Choices: []string{Cumulative, Delta},
			Default: Cumulative,
			Group:   metricsGroup,
			Detail: "Whether a sum or a histogram counts everything since the service started or only what came since the collection before. cumulative is what an SDK sends by default. " +
				"delta leaves jvm.memory.used cumulative, as the delta setting of an SDK does. Beside metric_kind gauge or summary it is refused.",
		},
		{
			Name: Exemplars, Kind: format.PropertyBool,
			Default: "false",
			Group:   metricsGroup,
			Detail: "Whether the durations of requests name the trace and the span a measurement was taken in, one for each bucket a collection measured in. " +
				"The file holds no spans, so those ids match nothing in it. Only http.server.request.duration has them, and beside a metric_kind without it exemplars are refused.",
		},
		{
			Name: Interval, Kind: format.PropertyInt,
			Min: 1, Max: maxInterval, Unit: "seconds",
			Default: strconv.Itoa(defaultInterval),
			Group:   metricsGroup,
			Detail:  "How long a service waits between two collections of its metrics. 60 is what an SDK uses by default. Every point of a collection is dated at the same moment, and in record-per-line a line holds one collection.",
		},
	}
}
