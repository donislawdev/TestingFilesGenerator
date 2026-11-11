package oracle

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

const otelcolScript = `
import json, os, shutil, subprocess, sys, tempfile, time

LINE_CEILING = 1 << 20
SIGNALS = {b'{"resourceSpans"': "traces", b'{"resourceLogs"': "logs", b'{"resourceMetrics"': "metrics"}

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
    except ValueError:
        return False

with open(sys.argv[1], "rb") as f:
    data = f.read()
pieces = data.split(b"\n")
by_line = data.endswith(b"\n") and all(is_object(p) for p in pieces[:-1])
if by_line:
    wanted = pieces[:-1]
else:
    # One request. The file receiver reads lines, so it is handed the request
    # on one line, and told to read a line that long.
    try:
        json.loads(data)
    except ValueError as err:
        print("FAIL the file is neither a batch on every line nor one JSON document: %s" % err)
        sys.exit(1)
    wanted = [squeezed(data)]

signal = next((name for start, name in SIGNALS.items() if wanted[0].startswith(start)), None)
if signal is None:
    print("FAIL the file does not open with resourceSpans, resourceLogs or resourceMetrics")
    sys.exit(1)

work = tempfile.mkdtemp(prefix="tfg-otelcol-")
try:
    os.mkdir(os.path.join(work, "in"))
    with open(os.path.join(work, "in", "data.jsonl"), "wb") as f:
        f.write(b"\n".join(wanted) + b"\n")
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

    got, deadline = [], time.time() + 30
    try:
        while time.time() < deadline and process.poll() is None:
            time.sleep(0.2)
            got = written()
            if len(got) >= len(wanted):
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
