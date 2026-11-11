package oracle

import "encoding/json"

// otelcol is the OpenTelemetry Collector: its file receiver reads the file and
// its file exporter writes back what it understood.
//
// Reading alone would be a weak witness, measured 2026-11-10
// (docs/OTLP-2026-11-10.md section 3): the Collector takes an enum written as
// a name and keys in snake case, both of which the specification forbids, and
// it drops a line it cannot read without a word in its log. So the question
// asked here is the stronger one - does what comes back equal what went in,
// byte for byte and batch for batch. A file written the way the Collector
// writes one passes, a line it dropped is missing from the answer, and a
// field it read differently comes back spelled differently.
//
// The Collector is named through TFG_OTELCOL or found on the path as
// otelcol-contrib, the distribution that carries both components. It is a
// server rather than a command, so the script starts it on a directory of its
// own, waits for the batches and stops it. No port is opened, which is what
// lets several of these run side by side.
var otelcol = Checker{
	Name:   "the OpenTelemetry Collector",
	find:   inPath("python"),
	args:   func(p string) []string { return []string{"-c", otelcolScript, p} },
	accept: expectOK("the OpenTelemetry Collector"),
}

// OTLPPitfalls asks the Collector about a file written with pitfalls, told
// which ones: the same script, which then spells every line the way the
// specification asks before it compares, and expects the lines the Collector
// was measured to drop to be missing. Byte for byte would be the wrong
// question here - the Collector writes its own spelling of what it read.
func OTLPPitfalls(path string, pitfalls map[string]string) Result {
	told, err := json.Marshal(pitfalls)
	if err != nil {
		return Result{Available: true, Tool: otelcol.Name, Err: err}
	}
	c := otelcol
	c.args = func(p string) []string { return []string{"-c", otelcolScript, p, string(told)} }
	return c.Check(path)
}

const otelcolScript = `
import json, os, shutil, subprocess, sys, tempfile, time

# Counted from here rather than from the moment the Collector starts: whoever
# runs this script stops it after a minute, and stops this process alone. A
# Collector still running then would be left behind, so everything below ends
# well inside that minute (review of #230).
STARTED = time.time()
BUDGET = 40
LINE_CEILING = 1 << 20
SIGNALS = {b'{"resourceSpans"': "traces", b'{"resourceLogs"': "logs", b'{"resourceMetrics"': "metrics",
           b'{"resource_spans"': "traces", b'{"resource_logs"': "logs", b'{"resource_metrics"': "metrics"}
BOM = b"\xef\xbb\xbf"

# The pitfalls the file was written with, when the guard says. Without them
# the question is the strong one below: the same bytes back. With them the
# Collector writes its own spelling of what it read, so each line is first
# spelled the way the specification asks and then compared, and a line the
# Collector was measured to drop is expected to be missing
# (docs/OTLP-2026-11-10.md section 13).
told = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}
ENUMS = {"SPAN_KIND_INTERNAL": 1, "SPAN_KIND_SERVER": 2, "STATUS_CODE_ERROR": 2,
         "SEVERITY_NUMBER_DEBUG": 5, "SEVERITY_NUMBER_INFO": 9, "SEVERITY_NUMBER_WARN": 13, "SEVERITY_NUMBER_ERROR": 17,
         "AGGREGATION_TEMPORALITY_DELTA": 1, "AGGREGATION_TEMPORALITY_CUMULATIVE": 2}
ENUM_KEYS = {"kind", "code", "severityNumber", "aggregationTemporality"}
WIDE_KEYS = {"startTimeUnixNano", "endTimeUnixNano", "timeUnixNano", "observedTimeUnixNano", "intValue", "asInt",
             "count", "bucketCounts", "zeroCount"}
ID_KEYS = {"traceId", "spanId", "parentSpanId"}
SENTINELS = {
    "traces": '{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{"name":"tfg.sentinel"},"spans":[{"traceId":"0102030405060708090a0b0c0d0e0f10",'
              '"spanId":"0102030405060708","name":"sentinel","kind":1,"startTimeUnixNano":"1","endTimeUnixNano":"2","status":{}}]}]}]}',
    "logs": '{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{"name":"tfg.sentinel"},"logRecords":[{"timeUnixNano":"1","body":{"stringValue":"sentinel"}}]}]}]}',
    "metrics": '{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{"name":"tfg.sentinel"},"metrics":[{"name":"sentinel",'
               '"gauge":{"dataPoints":[{"timeUnixNano":"1","asInt":"1"}]}}]}]}]}',
}

def camel(key):
    head, *rest = key.split("_")
    return head + "".join(p[:1].upper() + p[1:] for p in rest)

def canonical(value, key=None):
    # What the specification spells: keys in lower camel case, an enum as its
    # number, a number of 64 bits as text, ids as hex in lower case.
    if isinstance(value, dict):
        return {camel(k): canonical(v, camel(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [canonical(v, key) for v in value]
    if key in ENUM_KEYS and isinstance(value, str):
        return ENUMS.get(value, value)
    if key in WIDE_KEYS and type(value) is int:
        return str(value)
    if key in ID_KEYS and isinstance(value, str):
        return value.lower()
    return value

def spelled(line):
    return json.dumps(canonical(json.loads(line)), separators=(",", ":"), ensure_ascii=False).encode("utf-8")

named = os.environ.get("TFG_OTELCOL", "")
collector = named or shutil.which("otelcol-contrib")
if not collector:
    print("SKIP otelcol-contrib is not installed")
    sys.exit(0)
if not os.path.isfile(collector):
    print("FAIL TFG_OTELCOL names %s and no such file is there" % collector)
    sys.exit(1)

def squeezed(data):
    # The document with the whitespace between its tokens taken out and every
    # token left as it is written. Parsing and writing again would hide a
    # number the generator spelled differently from the Collector.
    out, in_text, escaped = bytearray(), False, False
    for b in data:
        if in_text:
            out.append(b)
            if escaped:
                escaped = False
            elif b == 0x5C:
                escaped = True
            elif b == 0x22:
                in_text = False
        elif b not in b" \t\r\n":
            out.append(b)
            in_text = b == 0x22
    return bytes(out)

def is_object(line):
    try:
        return isinstance(json.loads(line), dict)
    except (ValueError, RecursionError):
        return False

with open(sys.argv[1], "rb") as f:
    data = f.read()
bom = told.get("bom") == "true"
if bom:
    if not data.startswith(BOM):
        print("FAIL bom=true and the file does not open with a byte order mark")
        sys.exit(1)
    data = data[len(BOM):]
pieces = data.split(b"\n")
by_line = data.endswith(b"\n") and all(is_object(p) for p in pieces[:-1])
if by_line:
    wanted = pieces[:-1]
else:
    # One request. The file receiver reads lines, so it is handed the request
    # on one line, and told to read a line that long.
    try:
        json.loads(data)
    except RecursionError:
        print("FAIL the file is nested deeper than a JSON reader goes")
        sys.exit(1)
    except ValueError as err:
        print("FAIL the file is neither a batch on every line nor one JSON document: %s" % err)
        sys.exit(1)
    wanted = [squeezed(data)]

signal = next((name for start, name in SIGNALS.items() if wanted[0].startswith(start)), None)
if signal is None:
    print("FAIL the file does not open with resourceSpans, resourceLogs or resourceMetrics")
    sys.exit(1)

# What the file receiver is handed, and what it has to write back.
fed, expected, dropped = wanted, wanted, []
if told:
    fed = [BOM + wanted[0]] + wanted[1:] if bom else wanted
    expected = []
    for number, w in enumerate(wanted, start=1):
        doc = canonical(json.loads(w))
        why = None
        if not next(iter(doc.values())):
            why = "an empty envelope"
        elif told.get("id_encoding") == "base64":
            why = "ids in base64"
        elif bom and number == 1:
            why = "the line behind the byte order mark"
        if why:
            dropped.append("%d (%s)" % (number, why))
        else:
            expected.append(spelled(w))

work = tempfile.mkdtemp(prefix="tfg-otelcol-")
try:
    os.mkdir(os.path.join(work, "in"))
    with open(os.path.join(work, "in", "data.jsonl"), "wb") as f:
        f.write(b"\n".join(fed) + b"\n")
    if told:
        # A batch the Collector reads whatever the file holds: once it is back,
        # the file beside it has been read too, so a file that has to come
        # back empty does not wait for the whole budget.
        with open(os.path.join(work, "in", "zz-sentinel.jsonl"), "wb") as f:
            f.write(SENTINELS[signal].encode("utf-8") + b"\n")
    out = os.path.join(work, "out.jsonl")
    longest = max(len(w) for w in wanted)
    lines = ["receivers:", "  otlp_json_file:", "    include: ['%s']" % (work.replace(os.sep, "/") + "/in/*.jsonl"),
             "    start_at: beginning", "    include_file_name: false"]
    if not by_line and longest > LINE_CEILING:
        lines.append("    max_log_size: %d" % (longest + 1024))
    lines += ["exporters:", "  file:", "    path: '%s'" % out.replace(os.sep, "/"), "    flush_interval: 100ms",
              "service:", "  telemetry:", "    metrics:", "      level: none",
              "  pipelines:", "    %s:" % signal, "      receivers: [otlp_json_file]", "      exporters: [file]"]
    config = os.path.join(work, "config.yaml")
    with open(config, "w", encoding="utf-8", newline="\n") as f:
        f.write("\n".join(lines) + "\n")

    said = open(os.path.join(work, "collector.log"), "wb")
    process = subprocess.Popen([collector, "--config", config], stdout=said, stderr=said)
    def written():
        # Whole lines only. The exporter writes through a buffer, so the last
        # line of the file can be half of a batch for a moment.
        if not os.path.exists(out):
            return []
        with open(out, "rb") as f:
            return f.read().split(b"\n")[:-1]

    got, deadline = [], STARTED + BUDGET
    try:
        seen, still = 0, time.time()
        while time.time() < deadline and process.poll() is None:
            time.sleep(0.2)
            got = written()
            if told:
                # Done once the sentinel is back and nothing more has come for
                # a second and a half.
                if len(got) != seen:
                    seen, still = len(got), time.time()
                elif any(b"tfg.sentinel" in g for g in got) and time.time() - still > 1.5:
                    break
            elif len(got) >= len(wanted):
                # One more look, so a batch the Collector wrote twice shows.
                time.sleep(0.6)
                got = written()
                break
    finally:
        # Asked before it is stopped: once killed, every Collector has ended
        # with a code, and a file it dropped would read as one that never ran.
        ended = process.poll() is not None
        process.kill()
        process.wait()
        said.close()

    if ended and not got:
        with open(os.path.join(work, "collector.log"), "rb") as f:
            tail = f.read()[-600:].decode("utf-8", "replace")
        print("FAIL the Collector did not run: %s" % tail.strip())
        sys.exit(1)
    if told:
        if not any(b"tfg.sentinel" in g for g in got):
            print("FAIL the Collector wrote nothing back in %d s, not even the batch every reader takes" % BUDGET)
            sys.exit(1)
        got = [g for g in got if b"tfg.sentinel" not in g]
        if len(got) != len(expected):
            print("FAIL %d batches came back and %d were expected, with batches %s dropped as measured" % (
                len(got), len(expected), ", ".join(dropped) or "none"))
            sys.exit(1)
        for number, (a, b) in enumerate(zip(expected, got), start=1):
            if a != b:
                at = next((i for i in range(min(len(a), len(b))) if a[i] != b[i]), min(len(a), len(b)))
                print("FAIL batch %d came back different at byte %d once spelled the way the specification asks: ...%s against ...%s" % (
                    number, at, a[max(0, at - 40):at + 40].decode("utf-8", "replace"), b[max(0, at - 40):at + 40].decode("utf-8", "replace")))
                sys.exit(1)
        print("OK %d of %d batches of %s came back in the spelling of the specification, dropped as measured: %s" % (
            len(got), len(wanted), signal, ", ".join(dropped) or "none"))
        sys.exit(0)
    if len(got) != len(wanted):
        print("FAIL %d of %d batches came back - the Collector dropped the rest without a word, which is what it does "
              "with a line longer than %d bytes and with one it cannot read" % (len(got), len(wanted), LINE_CEILING))
        sys.exit(1)
    for number, (a, b) in enumerate(zip(wanted, got), start=1):
        if a != b:
            at = next((i for i in range(min(len(a), len(b))) if a[i] != b[i]), min(len(a), len(b)))
            print("FAIL batch %d came back different at byte %d: wrote ...%s and the Collector writes ...%s" % (
                number, at, a[max(0, at - 40):at + 40].decode("utf-8", "replace"), b[max(0, at - 40):at + 40].decode("utf-8", "replace")))
            sys.exit(1)
    print("OK %d batches of %s came back byte for byte, the longest %d bytes" % (len(wanted), signal, longest))
finally:
    shutil.rmtree(work, ignore_errors=True)
`
