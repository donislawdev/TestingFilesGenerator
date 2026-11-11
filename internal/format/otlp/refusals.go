package otlp

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

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
		if s.signal == Metrics {
			return core.Says("otlp.ALineOfMetricsIsOneCollection", "a line of %s holds one collection, so there is no batch to choose", core.A("Signal", chosen)), Signal
		}
	case Rate:
		if s.signal == Metrics {
			return core.Says("otlp.MetricsComeEveryInterval", "it sets how often a trace or a log record begins, and %s collects once every interval", core.A("Signal", chosen)), Signal
		}
	case MetricKind, Temporality, Exemplars, Interval:
		if s.signal != Metrics {
			return core.Says("otlp.ItShapesMetricsAndSignal", "it shapes metrics, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
		return s.outOfKind(name)
	default:
		return s.outOfPitfall(name)
	}
	return core.Said{}, ""
}

// outOfPitfall says why a pitfall does nothing beside the rest of s: one
// that shapes a kind of record the file does not hold, an empty batch where
// there are no lines, ids where nothing names one, an enum where there is none.
func (s settings) outOfPitfall(name string) (why core.Said, other string) {
	chosen := core.Choice{Of: Signal, Value: s.signal}
	switch name {
	case EndBeforeStart:
		if s.signal != Traces {
			return core.Says("otlp.ItShapesSpansAndSignal", "it shapes spans, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
	case Body:
		if s.signal != Logs {
			return core.Says("otlp.ItShapesLogRecordsAndSignal", "it shapes log records, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
	case NonFinite:
		if s.signal != Metrics {
			return core.Says("otlp.ItShapesMetricsAndSignal", "it shapes metrics, and signal %s writes none", core.A("Signal", chosen)), Signal
		}
		return s.outOfKind(name)
	case EmptyEnvelope:
		if !s.layout.lines {
			return core.Says("otlp.OnlyOneLayoutHasLinesToAdd", "only %s has a line to spare for an empty batch, and %s writes one request",
				core.A("Lines", core.Choice{Of: Formatting, Value: RecordPerLine}),
				core.A("Layout", core.Choice{Of: Formatting, Value: s.layout.name})), Formatting
		}
	case IDEncoding:
		return s.withoutIDs()
	case EnumEncoding:
		if s.signal == Metrics {
			return s.outOfKind(name)
		}
	}
	return core.Said{}, ""
}

// withoutIDs says why an id_encoding does nothing: log records that name no
// trace, or metrics that name no measurement.
func (s settings) withoutIDs() (why core.Said, other string) {
	switch {
	case s.signal == Logs && !s.traceContext:
		return core.Says("otlp.TheseLogRecordsNameNoTrace", "it spells the ids of traces and spans, and %s writes none",
			core.A("Off", core.Choice{Of: TraceContext, Value: "false"})), TraceContext
	case s.signal == Metrics && !s.exemplars:
		return core.Says("otlp.TheseMetricsNameNoTrace", "it spells the ids of traces and spans, and metrics name them only with %s",
			core.A("On", core.Choice{Of: Exemplars, Value: "true"})), Exemplars
	}
	return core.Said{}, ""
}

// outOfKind says why a setting of metrics does nothing beside the kind of
// metric chosen: a temporality where no sum and no histogram is written, and
// exemplars where no instrument measures inside a request.
func (s settings) outOfKind(name string) (why core.Said, other string) {
	kind := core.Choice{Of: MetricKind, Value: s.metricKind}
	temporal, exemplary, fractions := false, false, false
	for _, inst := range s.instruments {
		temporal = temporal || inst.shape.temporal()
		exemplary = exemplary || inst.exemplary
		fractions = fractions || inst.values == cpuUtilization
	}
	switch {
	case name == Temporality && !temporal:
		return core.Says("otlp.OnlySumsAndHistogramsAddUp", "only a sum or a histogram adds up over time, and %s writes neither", core.A("Kind", kind)), MetricKind
	case name == EnumEncoding && !temporal:
		return core.Says("otlp.TheseMetricsHaveNoEnum", "the one enum of metrics is the temporality of a sum or a histogram, and %s writes neither", core.A("Kind", kind)), MetricKind
	case name == Exemplars && !exemplary:
		return core.Says("otlp.OnlyRequestDurationsHaveExemplars", "only the durations of requests, a histogram, name the traces they were measured in, and %s writes none", core.A("Kind", kind)), MetricKind
	case name == NonFinite && !fractions:
		return core.Says("otlp.OnlyTheUtilizationGoesNaN", "only jvm.cpu.recent_utilization, a gauge, reports a value that is not a number, and %s writes no gauge", core.A("Kind", kind)), MetricKind
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
