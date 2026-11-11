package otlp

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// pitfallProperties are the spellings and the shapes the readers of OTLP
// disagree about, each off unless chosen. What every one of them does to the
// OpenTelemetry Collector 0.165.0 was measured before it was written down
// here (docs/OTLP-2026-11-10.md sections 3 and 13).
func pitfallProperties() []format.Property {
	return []format.Property{
		{
			Name: IDEncoding, Kind: format.PropertyChoice,
			Choices: []string{Base64, Hex, UpperHex},
			Default: Hex,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[IDEncoding],
		},
		{
			Name: EnumEncoding, Kind: format.PropertyChoice,
			Choices: []string{Name, Number},
			Default: Number,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[EnumEncoding],
		},
		{
			Name: KeyCase, Kind: format.PropertyChoice,
			Choices: []string{Camel, Snake},
			Default: Camel,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[KeyCase],
		},
		{
			Name: LargeNumbers, Kind: format.PropertyChoice,
			Choices: []string{Number, AsText},
			Default: AsText,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[LargeNumbers],
		},
		{
			Name: TimeOrder, Kind: format.PropertyChoice,
			Choices: []string{InOrder, OutOfOrder},
			Default: InOrder,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[TimeOrder],
		},
		{Name: EndBeforeStart, Kind: format.PropertyBool, Default: "false", Group: pitfallsGroup, Detail: pitfallDetail[EndBeforeStart]},
		{Name: EmptyEnvelope, Kind: format.PropertyBool, Default: "false", Group: pitfallsGroup, Detail: pitfallDetail[EmptyEnvelope]},
		{
			Name: Body, Kind: format.PropertyChoice,
			Choices: []string{AnyValue, AsText},
			Default: AsText,
			Group:   pitfallsGroup,
			Detail:  pitfallDetail[Body],
		},
		{Name: NonFinite, Kind: format.PropertyBool, Default: "false", Group: pitfallsGroup, Detail: pitfallDetail[NonFinite]},
	}
}

// pitfallDetail says what each pitfall writes and what the readers do with it.
// The OpenTelemetry Collector 0.165.0 was asked about every one of them in
// every kind of telemetry on 2026-11-11, through its file receiver, and the
// guard TestEveryOTLPPitfallMeetsItsReferenceTool asks again. Its HTTP
// receiver, the protobuf library and the Rust crate were asked about spans on
// 2026-11-10 (docs/OTLP-2026-11-10.md sections 3 and 11.2).
var pitfallDetail = map[string]string{
	IDEncoding: "How trace and span ids are written. hex is what the OTLP specification asks for. " +
		"upper-hex writes the same digits in capitals, which the specification allows, and the OpenTelemetry Collector writes them back in lower case. " +
		"base64 writes them the way the JSON mapping of protobuf writes bytes, which the specification forbids - the file receiver of the Collector drops every batch that holds an id without a word and its HTTP receiver answers 400. " +
		"A reader built on the protobuf library reads base64 ids right and hex ids as something else.",
	EnumEncoding: "How the kind of a span, the code of its status, the severity of a log record and the temporality of a sum or a histogram are written. number is what the OTLP specification asks for. " +
		"name writes names such as SPAN_KIND_SERVER, which the specification forbids. The OpenTelemetry Collector reads them anyway and writes numbers back, and the Rust crate opentelemetry-proto refuses a span written that way.",
	KeyCase: "How the keys of the file are written. camel is what the OTLP specification asks for, such as traceId. " +
		"snake writes the names the .proto files give the fields, such as trace_id, which the specification calls not valid. The OpenTelemetry Collector reads them and writes camel case back, and the Rust crate opentelemetry-proto refuses the file.",
	LargeNumbers: "How times and the other numbers of 64 bits are written. text is what the OTLP specification asks a writer for, such as \"1785585600000000000\". " +
		"number writes them as JSON numbers, which a reader is told to take as well, and the OpenTelemetry Collector does. " +
		"A reader that holds every JSON number as a double, as JavaScript does, gets every time to the nearest 256 nanoseconds.",
	TimeOrder: "Whether every trace, log record or collection is dated later than the one before it. out-of-order swaps the times of every two neighbours, so every second one is dated before the one in front of it. " +
		"The specification of the OTLP file promises no order, and the OpenTelemetry Collector reads the file and writes it back unchanged. What the system storing the data does with what comes late is the question this file asks.",
	EndBeforeStart: "Whether every span is dated with its end before its start, the two times changed places. The OpenTelemetry Collector reads such a span and writes it back unchanged. It shapes spans, and beside signal logs or metrics it is refused.",
	EmptyEnvelope: "Whether the file opens with a line holding a batch with nothing in it, which the OTLP specification asks senders not to write. " +
		"The file receiver of the OpenTelemetry Collector reads the line and writes nothing for it. Only record-per-line has a line to spare, and beside another layout it is refused.",
	Body: "What the body of a log record holds. text is a sentence. any-value gives the seven kinds of value OTLP has in turn - text, a boolean, the whole number 2^53 + 1, a fraction, a list, a map and bytes - " +
		"and the OpenTelemetry Collector reads them and writes them back unchanged. With large_numbers number, a reader that holds numbers as doubles reads that whole number one less. It shapes log records, and beside signal traces or metrics it is refused.",
	NonFinite: "Whether jvm.cpu.recent_utilization reports values that are not numbers: NaN in the second of every four collections, and Infinity and -Infinity in turn in the fourth, written as text the way the JSON mapping of protobuf writes them. " +
		"The OpenTelemetry Collector reads them and writes them back unchanged. Beside a metric_kind without that gauge it is refused.",
}
