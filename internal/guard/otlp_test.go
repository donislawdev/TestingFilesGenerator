package guard

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/otlp"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// otlpCases is every setting of the format at both ends of what it takes, and
// the settings that meet inside one record together. A batch of one and a
// trace of one span are the two places where "the first of" and "the last of"
// are the same record.
var otlpCases = []map[string]string{
	{},
	{otlp.Formatting: otlp.Minified},
	{otlp.Formatting: otlp.Indented},
	{otlp.Batch: "1"},
	{otlp.Batch: "2"},
	{otlp.Batch: "1000"},
	{otlp.Depth: "1"},
	{otlp.Depth: "2"},
	{otlp.Depth: "100"},
	{otlp.StatusMix: "success"},
	{otlp.StatusMix: "client-errors"},
	{otlp.StatusMix: "server-errors"},
	{otlp.Rate: "7"},
	{otlp.Rate: "1000000"},
	{otlp.Start: "2002-01-01T00:00:00Z"},
	{otlp.Start: "2199-12-31T23:59:59Z"},
	{otlp.Depth: "100", otlp.Batch: "1", otlp.StatusMix: "server-errors"},
	{otlp.Depth: "1", otlp.Formatting: otlp.Indented, otlp.Rate: "1000"},

	{otlp.Signal: otlp.Logs},
	{otlp.Signal: otlp.Logs, otlp.Formatting: otlp.Minified},
	{otlp.Signal: otlp.Logs, otlp.Formatting: otlp.Indented},
	{otlp.Signal: otlp.Logs, otlp.LevelMix: "quiet"},
	{otlp.Signal: otlp.Logs, otlp.LevelMix: "errors"},
	{otlp.Signal: otlp.Logs, otlp.LevelMix: "debug"},
	{otlp.Signal: otlp.Logs, otlp.TraceContext: "false"},
	{otlp.Signal: otlp.Logs, otlp.Batch: "3", otlp.Rate: "1000", otlp.LevelMix: "errors", otlp.TraceContext: "false"},
	{otlp.Signal: otlp.Logs, otlp.Batch: "1", otlp.Formatting: otlp.RecordPerLine, otlp.Start: "2199-12-31T23:59:59Z"},
}

// otlpSizes are the sizes every case is written at: the smallest file and the
// byte above it, where the padding goes from nothing to one byte, a size of a
// few dozen records, and one large enough to end a default batch and open a
// second.
func otlpSizes(smallest int64) []int64 { return []int64{smallest, smallest + 1, 20000, 300000} }

// The structural checker is told the settings and has to find the file made
// the way they say: laid out as ordered and byte for byte the text that layout
// makes of its own content, every batch but the last as large as ordered, the
// clock running from start at the rate, every trace as deep as ordered with
// each child inside its parent, the padding on the last record and on no
// other.
//
// It runs wherever python does, which is every machine this suite runs on.
// The Collector beside it runs in one job of CI only, and that is the reason
// this guard is not folded into the one below: a skip there would take this
// check with it.
func TestAnOTLPFileIsWhatItsSettingsSay(t *testing.T) {
	d := otlpFormat(t)
	dir := t.TempDir()
	var reqs []oracle.StrictRequest
	batched := 0
	for i, props := range otlpCases {
		smallest := d.SmallestAccepted(format.Request{Properties: props})
		for _, size := range otlpSizes(smallest) {
			path := filepath.Join(dir, strconv.Itoa(i)+"_"+strconv.FormatInt(size, 10)+".jsonl")
			body := writeOTLP(t, path, size, uint64(57+i), props)
			reqs = append(reqs, oracle.StrictRequest{Format: "otlp", Path: path, Settings: props})
			if bytes.Count(body, []byte("\n")) > 1 && props[otlp.Formatting] == "" {
				batched++
			}
		}
	}
	// One process for all of them: started once for each file, the checker
	// took a second a file and this guard two minutes.
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural check needs python")
	}
	if err != nil {
		t.Fatal(err)
	}
	for i, j := range got {
		if j.Verdict != oracle.StrictOK {
			t.Errorf("%v in %s: %s %s", reqs[i].Settings, filepath.Base(reqs[i].Path), j.Verdict, j.Reason)
		}
	}
	// The state this guard exists to check is a file of several batches, where
	// a record opens one and another ends one. Asserted rather than assumed.
	if len(got) != len(reqs) || batched < len(otlpCases)/2 {
		t.Fatalf("%d of %d files judged and %d of them held more than one batch - too few to say anything about a batch that ends",
			len(got), len(reqs), batched)
	}
}

// The Collector reads every file and writes it back, and what comes back has
// to be what went in, byte for byte and batch for batch. Reading alone would
// say little: measured on 2026-11-10 the Collector takes spellings the
// specification forbids and drops a line it cannot read without a word
// (docs/OTLP-2026-11-10.md section 3).
//
// Named to match ReferenceTool, because that is the one job in CI with the
// Collector installed. Anywhere else it skips, and says so.
func TestEveryOTLPSettingSurvivesItsReferenceTool(t *testing.T) {
	d := otlpFormat(t)
	collector := mustOracle(t, d)
	dir := t.TempDir()
	for i, props := range otlpCases {
		smallest := d.SmallestAccepted(format.Request{Properties: props})
		// Three sizes rather than four: the Collector is started for every
		// file, and the size left out says nothing the largest does not.
		for _, size := range []int64{smallest, smallest + 1, 300000} {
			path := filepath.Join(dir, strconv.Itoa(i)+"_"+strconv.FormatInt(size, 10)+".jsonl")
			writeOTLP(t, path, size, uint64(57+i), props)
			res := collector.Check(path)
			if !res.Available {
				t.Skipf("%s is not installed, so no file was read by it - a skip, not a pass", res.Tool)
			}
			if res.Err != nil {
				t.Fatalf("%v at %d B: %v", props, size, res.Err)
			}
		}
	}
}

// The file receiver of the Collector drops a line longer than 1 MiB without a
// word in its log, measured on 2026-11-10: 1 048 576 B was read and 1 048 577 B
// was not. So no setting may make such a line, and the largest batch the
// format allows is what decides that.
//
// Asked two ways. The bound the format works out has to stay under the
// ceiling for the longest records either kind of telemetry can hold, and a
// real file at the largest batch has to stay under the bound - with at least
// one line holding a full batch, or the file never reached the state the
// bound is about.
func TestNoOTLPLineIsLongerThanTheFileReceiverReads(t *testing.T) {
	d := otlpFormat(t)
	for _, props := range []map[string]string{
		{otlp.Batch: "1000", otlp.Depth: "1", otlp.StatusMix: "server-errors"},
		{otlp.Batch: "1000", otlp.Depth: "100"},
		{otlp.Batch: "1000", otlp.Signal: otlp.Logs},
		{otlp.Batch: "1000", otlp.Signal: otlp.Logs, otlp.LevelMix: "errors"},
	} {
		bound, err := otlp.LongestLine(props)
		if err != nil {
			t.Fatal(err)
		}
		if bound > otlp.LineCeiling {
			t.Errorf("%v: a line can reach %d B and the file receiver reads %d B at most.\n"+
				"Reason: the receiver drops a longer line without a word, so the batches of it would go missing\n"+
				"from a file that is the right size. Lower the largest batch", props, bound, otlp.LineCeiling)
		}
		p, err := d.Generator.Plan(format.Request{Bytes: 3 << 20, Seed: 4, Properties: props})
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
			t.Fatal(err)
		}
		full := 0
		leaf := map[string]string{"": `{"traceId":`, otlp.Logs: `{"timeUnixNano":`}[props[otlp.Signal]]
		for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
			if int64(len(line)) > bound {
				t.Errorf("%v: a line of %d B, and the format says none is longer than %d B", props, len(line), bound)
			}
			if strings.Count(line, leaf) == 1000 {
				full++
			}
		}
		if full == 0 {
			t.Errorf("%v: no line of the file holds a full batch, so nothing was measured at the largest one", props)
		}
	}
}

// A setting that can do nothing in the file ordered is refused rather than
// ignored: one that shapes spans beside logs, one that shapes log records
// beside traces, and a batch beside a layout that has no batches. The refusal
// names both settings, because naming one of a pair leaves the reader to guess
// which of the two to change.
//
// Only a real choice is refused. A window sends every setting it draws at its
// default, so the same keys at their defaults have to pass - the defect the
// log format had on 2026-08-31, when four of its shapes could not be reached
// from the window at all.
func TestOTLPRefusesASettingThatHasNothingToShape(t *testing.T) {
	d := otlpFormat(t)
	for _, c := range []struct {
		props map[string]string
		key   string
	}{
		{map[string]string{otlp.Signal: otlp.Logs, otlp.Depth: "5"}, "depth and signal"},
		{map[string]string{otlp.Signal: otlp.Logs, otlp.StatusMix: "success"}, "status_mix and signal"},
		{map[string]string{otlp.LevelMix: "errors"}, "level_mix and signal"},
		{map[string]string{otlp.TraceContext: "false"}, "trace_context and signal"},
		{map[string]string{otlp.Formatting: otlp.Minified, otlp.Batch: "10"}, "batch and formatting"},
		{map[string]string{otlp.Formatting: otlp.Indented, otlp.Batch: "10"}, "batch and formatting"},
	} {
		_, err := d.Generator.Plan(format.Request{Bytes: 100000, Seed: 1, Properties: c.props})
		var refused *format.PropertyValueError
		if !errors.As(err, &refused) {
			t.Errorf("%v was planned, and one of those settings does nothing beside the other: %v", c.props, err)
			continue
		}
		if refused.Key != c.key {
			t.Errorf("%v was refused over %q, not over %q", c.props, refused.Key, c.key)
		}
	}

	everyDefault := map[string]string{}
	for _, p := range d.Properties {
		everyDefault[p.Name] = p.Default
	}
	for _, signal := range []string{otlp.Traces, otlp.Logs} {
		everyDefault[otlp.Signal] = signal
		for _, layout := range []string{otlp.RecordPerLine, otlp.Minified, otlp.Indented} {
			everyDefault[otlp.Formatting] = layout
			if _, err := d.Generator.Plan(format.Request{Bytes: 100000, Seed: 1, Properties: everyDefault}); err != nil {
				t.Errorf("signal %s laid out %s with every other setting sent at its default was refused: %v.\n"+
					"Reason: a window sends every setting it draws, so a default that arrived is not a choice",
					signal, layout, err)
			}
		}
	}
}

// start takes one spelling of a moment, in the years where a time in
// nanoseconds has nineteen digits, and a file may not run past the last moment
// such a count can name. Each of them is refused with the setting named,
// rather than written as a time one digit longer than planned - which would
// make the file the wrong size somewhere near its end.
func TestOTLPRefusesAStartItCannotWrite(t *testing.T) {
	d := otlpFormat(t)
	for _, start := range []string{
		"2026-08-01", "2026-08-01T12:00:00", "2026-08-01T12:00:00+02:00", "2026-08-01T12:00:00.5Z",
		"2026-02-30T12:00:00Z", "2026-08-01 12:00:00Z", "yesterday",
		"2001-12-31T23:59:59Z", "2200-01-01T00:00:00Z", "1970-01-01T00:00:00Z",
	} {
		_, err := d.Generator.Plan(format.Request{Bytes: 100000, Seed: 1, Properties: map[string]string{otlp.Start: start}})
		var refused *format.PropertyValueError
		if !errors.As(err, &refused) || refused.Key != otlp.Start {
			t.Errorf("start=%q was not refused over start: %v", start, err)
		}
	}

	// Hundreds of gigabytes at one record a second, from the last year a file
	// may start in. Planned only: nothing is written.
	late := map[string]string{otlp.Start: "2199-12-31T23:59:59Z", otlp.Signal: otlp.Logs}
	_, err := d.Generator.Plan(format.Request{Bytes: 400 << 30, Seed: 1, Properties: late})
	var refused *format.PropertyValueError
	if !errors.As(err, &refused) || refused.Key != otlp.Start {
		t.Errorf("a file of 400 GiB from %s at one record a second was not refused over start: %v", late[otlp.Start], err)
	}
	late[otlp.Rate] = "1000000"
	if _, err := d.Generator.Plan(format.Request{Bytes: 400 << 30, Seed: 1, Properties: late}); err != nil {
		t.Errorf("the same file at a million records a second ends in time and was refused: %v", err)
	}
}

// CPython's json reads NaN and Infinity, which no JSON grammar has, and
// writes them back, so one of them in a value nothing else looked at passed
// the comparison of the layout and the file was called sound. A document
// nested past what the decoder walks ended the checker with an exception,
// and so did a rate of nought handed to it. All three were found by the
// review of #230 and measured on the checker as it stood: OK, a traceback, a
// traceback.
//
// Each has to be a refusal with a reason. A crash also turns a guard red, and
// a guard asking whether a damaged file is refused must not count the
// checker falling over as a refusal.
func TestTheOTLPCheckRefusesWhatAJSONReaderLetsThrough(t *testing.T) {
	dir := t.TempDir()
	good := writeOTLP(t, filepath.Join(dir, "good.jsonl"), 4096, 9, map[string]string{otlp.Batch: "1"})
	service := []byte(`{"stringValue":"shop"}`)
	if bytes.Count(good, service) == 0 {
		t.Fatal("the file names no service, so there is nowhere to put a number that is not JSON")
	}
	write := func(name string, body []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	deep := append(bytes.Repeat([]byte("["), 200000), bytes.Repeat([]byte("]"), 200000)...)
	batch := map[string]string{otlp.Batch: "1"}
	// because is what the refusal has to say. Refused is not enough: the
	// checker has later checks, and a file let through by the first one can
	// be refused by another for a different reason. Measured on 2026-11-11 -
	// with NaN read quietly, the mutation run on #230 found the file still
	// refused, by a later check, and reported the entry as a hole.
	cases := []struct {
		what    string
		because string
		req     oracle.StrictRequest
	}{
		{"NaN where the name of the service stands", "NaN is not a JSON number", oracle.StrictRequest{Format: "otlp", Settings: batch,
			Path: write("nan.jsonl", bytes.Replace(good, service, []byte(`{"doubleValue":NaN}`), 1))}},
		{"Infinity in the same place", "Infinity is not a JSON number", oracle.StrictRequest{Format: "otlp", Settings: batch,
			Path: write("infinity.jsonl", bytes.Replace(good, service, []byte(`{"doubleValue":Infinity}`), 1))}},
		{"a document nested 200000 deep", "nested deeper than a JSON reader goes", oracle.StrictRequest{Format: "otlp", Settings: batch, Path: write("deep.jsonl", append(deep, '\n'))}},
		{"a rate of nought", "rate='0'", oracle.StrictRequest{Format: "otlp", Path: filepath.Join(dir, "good.jsonl"),
			Settings: map[string]string{otlp.Batch: "1", otlp.Rate: "0"}}},
		{"a depth that is a word", "depth='deep'", oracle.StrictRequest{Format: "otlp", Path: filepath.Join(dir, "good.jsonl"),
			Settings: map[string]string{otlp.Batch: "1", otlp.Depth: "deep"}}},
	}
	reqs := []oracle.StrictRequest{{Format: "otlp", Path: filepath.Join(dir, "good.jsonl"), Settings: batch}}
	for _, c := range cases {
		reqs = append(reqs, c.req)
	}
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural check needs python")
	}
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Verdict != oracle.StrictOK {
		t.Fatalf("the file every case is made from is itself refused, so no refusal below says anything: %s", got[0].Reason)
	}
	for i, c := range cases {
		if j := got[i+1]; j.Verdict != oracle.StrictFail {
			t.Errorf("%s: the checker answered %s (%s), and it has to refuse with a reason", c.what, j.Verdict, j.Reason)
		} else if !strings.Contains(j.Reason, c.because) {
			t.Errorf("%s: refused, but for another reason - %q, where it has to say %q", c.what, j.Reason, c.because)
		}
	}
}

func otlpFormat(t *testing.T) format.Descriptor {
	t.Helper()
	d, err := format.Get("otlp")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// writeOTLP writes one file of exactly size bytes and returns what it holds.
func writeOTLP(t *testing.T, path string, size int64, seed uint64, props map[string]string) []byte {
	t.Helper()
	d := otlpFormat(t)
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
