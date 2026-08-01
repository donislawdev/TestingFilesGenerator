"""Structural checks written against the format specifications.

These exist because the real readers are tolerant on purpose. Measured:
ffprobe ignores a wrong size in the RIFF header, Pillow does not verify the
checksum of an ancillary PNG chunk, and a PDF reader rebuilds a cross
reference table whose offset is off by one. All three are genuine defects that
a tolerant reader hides.

So this is a second layer: an independent implementation, in another language,
written to the specification rather than to what a viewer will put up with. It
complements the tolerant readers rather than replacing them - a file has to
pass both.

Usage: python strict.py <format> <path>
Prints "OK <detail>" or "FAIL <reason>".
"""

import re
import struct
import sys
import zlib


def fail(msg):
    print("FAIL " + msg)
    sys.exit(1)


def ok(msg):
    print("OK " + msg)
    sys.exit(0)


def check_png(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        fail("the signature is not a PNG signature")

    pos, seen, chunks = 8, [], 0
    while pos < len(data):
        if pos + 8 > len(data):
            fail(f"a chunk header runs past the end of the file at offset {pos}")
        length = struct.unpack(">I", data[pos:pos + 4])[0]
        kind = data[pos + 4:pos + 8]
        body = data[pos + 8:pos + 8 + length]
        if len(body) != length:
            fail(f"chunk {kind!r} declares {length} B and the file holds {len(body)}")
        want = struct.unpack(">I", data[pos + 8 + length:pos + 12 + length])[0]
        got = zlib.crc32(kind + body) & 0xFFFFFFFF
        if want != got:
            fail(f"chunk {kind.decode('latin1')} has checksum {want:08x} and its bytes give {got:08x}")
        seen.append(kind)
        chunks += 1
        pos += 12 + length

    if seen[0] != b"IHDR":
        fail("the first chunk is not IHDR")
    if seen[-1] != b"IEND":
        fail("the last chunk is not IEND")
    if b"IDAT" not in seen:
        fail("there is no image data")
    ok(f"{chunks} chunks, every checksum matches")


def check_wav(data):
    if data[:4] != b"RIFF" or data[8:12] != b"WAVE":
        fail("this is not a RIFF WAVE file")

    declared = struct.unpack("<I", data[4:8])[0]
    actual = len(data) - 8
    if declared != actual:
        fail(f"the RIFF header declares {declared} B after itself and the file holds {actual}")

    pos, seen, total = 12, [], 0
    while pos + 8 <= len(data):
        kind = data[pos:pos + 4]
        length = struct.unpack("<I", data[pos + 4:pos + 8])[0]
        body_end = pos + 8 + length
        if body_end > len(data):
            fail(f"chunk {kind!r} declares {length} B and runs past the end")
        seen.append(kind.decode("latin1").strip())
        total += 1
        pos = body_end
        # A chunk of odd length is padded to an even boundary, except when it
        # is the last one in the file.
        if length % 2 and pos < len(data):
            if data[pos:pos + 1] != b"\x00":
                fail(f"chunk {kind!r} has an odd length and is not followed by an alignment byte")
            pos += 1

    if pos != len(data):
        fail(f"the chunks end at {pos} and the file is {len(data)} B")
    if "fmt" not in seen:
        fail("there is no format chunk")
    if "data" not in seen:
        fail("there is no data chunk")
    ok(f"{total} chunks: {', '.join(seen)}")


def check_pdf(data):
    if not data.startswith(b"%PDF-"):
        fail("the file does not start with a PDF header")
    if b"%%EOF" not in data[-64:]:
        fail("the end of file marker is not near the end")

    idx = data.rfind(b"startxref")
    if idx < 0:
        fail("there is no startxref keyword")
    tail = data[idx + len(b"startxref"):].strip().split()[0]
    try:
        offset = int(tail)
    except ValueError:
        fail(f"startxref is followed by {tail!r} rather than a number")

    if offset >= len(data):
        fail(f"startxref points at {offset} and the file is {len(data)} B")
    if data[offset:offset + 4] != b"xref":
        near = data[max(0, offset - 8):offset + 12]
        fail(f"startxref points at {offset}, where the file holds {near!r} rather than the xref table")

    # Every offset in the table has to land on the object it claims.
    body = data[offset:]
    lines = body.split(b"\n")
    if len(lines) < 2:
        fail("the xref table has no entries")
    try:
        first, count = lines[1].split()
        count = int(count)
    except Exception:
        fail(f"the xref header is {lines[1]!r}")

    checked = 0
    for i in range(1, count):
        entry = lines[1 + i + 1] if 1 + i + 1 < len(lines) else b""
        parts = entry.split()
        if len(parts) < 3 or parts[2] != b"n":
            continue
        obj_at = int(parts[0])
        expected = str(i).encode() + b" 0 obj"
        if data[obj_at:obj_at + len(expected)] != expected:
            fail(f"the xref table sends object {i} to offset {obj_at}, "
                 f"where the file holds {data[obj_at:obj_at + 16]!r}")
        checked += 1
    ok(f"{checked} object offsets all land on their object")


def check_zip(data):
    end = data.rfind(b"PK\x05\x06")
    if end < 0:
        fail("there is no end of central directory record")
    comment_len = struct.unpack("<H", data[end + 20:end + 22])[0]
    if end + 22 + comment_len != len(data):
        fail(f"the comment declares {comment_len} B and {len(data) - end - 22} follow the record")
    entries = struct.unpack("<H", data[end + 10:end + 12])[0]
    cd_at = struct.unpack("<I", data[end + 16:end + 20])[0]
    if data[cd_at:cd_at + 4] != b"PK\x01\x02":
        fail(f"the central directory should start at {cd_at} and does not")
    ok(f"{entries} entries, comment {comment_len} B")


def check_log(data):
    """Every line is a whole entry in the Apache combined format.

    A log is read line by line, so a line that is not a whole entry is a broken
    file however right its length is. And "the last line is truncated" is what
    a real log looks like caught mid rotation, so the defect reads as realism
    unless something checks.

    Written to the format rather than to our output. The octet pattern refuses
    a leading zero: real logs write 93, not 093, and a leading zero is read as
    octal by some address parsers - where 069 is not even valid octal. Our
    generator produced padded octets until 2026-08-01, and a checker written to
    match it would have blessed that instead of catching it.
    """
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        fail(f"not valid UTF-8: {exc}")

    if not text.endswith("\n"):
        fail("the file does not end with a newline, so the last entry is unterminated")

    octet = r"(?:0|[1-9][0-9]{0,2})"
    pattern = re.compile(
        r"^" + octet + r"\." + octet + r"\." + octet + r"\." + octet +
        r" - - \[[^\]]+\] \"(?:GET|POST|PUT|DELETE|HEAD|PATCH) /\S* HTTP/1\.[01]\""
        r" [1-5][0-9]{2} [0-9]+ \"[^\"]*\" \"[^\"]*\"$")

    entries = 0
    for number, line in enumerate(text.rstrip("\n").split("\n"), start=1):
        if line.startswith("# "):
            continue  # the label line, which says in words that it is not an entry
        if not pattern.match(line):
            fail(f"line {number} is not a whole entry: {line[:90]!r}")
        # An octet above 255 parses as a number and is not an address.
        address = line.split(" ", 1)[0]
        if any(int(part) > 255 for part in address.split(".")):
            fail(f"line {number} has an octet above 255: {address}")
        entries += 1

    if entries == 0:
        fail("the file holds no entries at all")
    ok(f"{entries} entries, all whole")


def check_csv(data):
    """Every row carries the same columns, written to RFC 4180 by hand.

    Not csv.reader. That module is the tolerant reader used as the reference
    tool beside this one, and it accepts a ragged row without a word - a row
    with a column missing is exactly the defect this has to catch, and the file
    would still be the right size and still parse.

    The field size ceiling below is the measurement of 2026-08-01 turned into a
    rule: the Python csv module at its default settings refuses a field above
    131 072 B. Padding pushed into one field instead of through rows would sail
    past every size and determinism guard and break the reader a tester has
    nearest.
    """
    FIELD_CEILING = 131072

    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        fail(f"not valid UTF-8: {exc}")

    if not text.endswith("\n"):
        fail("the file does not end with a newline, so the last row is unterminated")

    rows, field, row, quoted, i = [], [], [], False, 0
    while i < len(text):
        ch = text[i]
        if quoted:
            if ch == '"':
                if i + 1 < len(text) and text[i + 1] == '"':
                    field.append('"')
                    i += 2
                    continue
                quoted = False
            else:
                field.append(ch)
            i += 1
            continue
        if ch == '"':
            if field:
                fail(f"row {len(rows) + 1} opens a quote in the middle of a field")
            quoted = True
        elif ch == ",":
            row.append("".join(field))
            field = []
        elif ch == "\n":
            row.append("".join(field))
            field = []
            rows.append(row)
            row = []
        else:
            field.append(ch)
        i += 1

    if quoted:
        fail("a quoted field is never closed")
    if field or row:
        fail("the file ends in the middle of a row")
    if len(rows) < 2:
        fail(f"the table holds {len(rows)} row(s), so there is no data to check")

    columns = len(rows[0])
    if columns < 2:
        fail(f"the header declares {columns} column(s)")
    for number, r in enumerate(rows, start=1):
        if len(r) != columns:
            fail(f"row {number} has {len(r)} fields and the header declares {columns}")
        for value in r:
            if len(value.encode("utf-8")) > FIELD_CEILING:
                fail(f"row {number} has a field of {len(value.encode('utf-8'))} B - "
                     f"the default Python reader refuses anything above {FIELD_CEILING} B")

    ok(f"{len(rows) - 1} data rows, {columns} columns each")


def check_json(data):
    """The document is an array of records, each on its own line.

    Parsing is CPython's json here and V8's parser in the reference tool beside
    it, which are two implementations in two languages. What this adds is the
    shape: that the file is records rather than one enormous value, and that it
    is laid out the way the manifest says it is.

    The value types come from the format document, not from what the generator
    happens to emit. A generator that quietly stopped writing booleans would be
    the right size and would still parse.
    """
    import json

    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        fail(f"not valid UTF-8: {exc}")

    try:
        doc = json.loads(text)
    except ValueError as exc:
        fail(f"not valid JSON: {exc}")

    if not isinstance(doc, list):
        fail(f"the root is {type(doc).__name__} rather than an array")
    if not doc:
        fail("the array is empty")

    # The manifest states one record per line, so the line count has to match.
    lines = text.rstrip("\n").split("\n")
    if len(lines) != len(doc) + 2:
        fail(f"{len(doc)} records over {len(lines)} lines - the manifest says one record per line")

    kinds = set()
    keys = None
    for number, record in enumerate(doc, start=1):
        if not isinstance(record, dict):
            fail(f"record {number} is {type(record).__name__} rather than an object")
        if keys is None:
            keys = set(record)
        elif set(record) != keys:
            fail(f"record {number} has the keys {sorted(record)} and record 1 has {sorted(keys)}")
        for value in record.values():
            if value is None:
                kinds.add("null")
            elif isinstance(value, bool):
                kinds.add("bool")
            elif isinstance(value, (int, float)):
                kinds.add("number")
            elif isinstance(value, str):
                kinds.add("string")
            elif isinstance(value, list):
                kinds.add("array")
            elif isinstance(value, dict):
                kinds.add("object")

    wanted = {"null", "bool", "number", "string", "array", "object"}
    missing = wanted - kinds
    if missing:
        fail(f"no record carries a value of type {', '.join(sorted(missing))} - "
             "the point of a JSON fixture is that a parser meets every type")

    ok(f"{len(doc)} records, keys {sorted(keys)}")


def scan_xml(data):
    """Well formed, checked by hand against the XML specification.

    Deliberately not expat. That is the reference tool beside this one, so
    calling it here would run the same C parser twice and prove nothing the
    first run did not.

    What a hand scanner adds: tags balanced and correctly nested, a comment that
    never contains a double hyphen, and text where every ampersand starts a real
    entity reference. A raw ampersand is the classic way a generated document
    stops being well formed while staying exactly the right size.
    """
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        fail(f"not valid UTF-8: {exc}")

    if not text.startswith("<?xml "):
        fail("the document does not open with an XML declaration")

    entity = re.compile(r"&(?:amp|lt|gt|quot|apos|#[0-9]+|#x[0-9a-fA-F]+);")
    stack, elements, i, seen = [], 0, 0, {}

    while i < len(text):
        nxt = text.find("<", i)
        if nxt < 0:
            nxt = len(text)

        # Character data between two tags.
        chunk = text[i:nxt]
        for pos, ch in enumerate(chunk):
            if ch == "&" and not entity.match(chunk, pos):
                fail(f"a bare ampersand at offset {i + pos} - text has to escape it as &amp;")
        if chunk.strip() and not stack:
            fail(f"text outside the root element at offset {i}: {chunk.strip()[:40]!r}")
        if nxt == len(text):
            break

        if text.startswith("<!--", nxt):
            end = text.find("-->", nxt + 4)
            if end < 0:
                fail("a comment is never closed")
            if "--" in text[nxt + 4:end]:
                fail("a comment contains a double hyphen, which XML does not allow")
            i = end + 3
            continue

        if text.startswith("<?", nxt):
            end = text.find("?>", nxt + 2)
            if end < 0:
                fail("a processing instruction is never closed")
            i = end + 2
            continue

        end = text.find(">", nxt)
        if end < 0:
            fail(f"a tag opened at offset {nxt} is never closed")
        tag = text[nxt + 1:end]

        if tag.startswith("/"):
            name = tag[1:].strip()
            if not stack:
                fail(f"</{name}> closes an element that was never opened")
            opened = stack.pop()
            if opened != name:
                fail(f"</{name}> closes while <{opened}> is open")
        else:
            selfClosing = tag.endswith("/")
            body = tag[:-1] if selfClosing else tag
            name = body.split()[0] if body.split() else ""
            if not name:
                fail(f"an empty tag at offset {nxt}")
            # Every attribute value has to be quoted.
            for attr in re.finditer(r"(\w+)\s*=\s*(.?)", body[len(name):]):
                if attr.group(2) not in ('"', "'"):
                    fail(f"attribute {attr.group(1)} of <{name}> is not quoted")
            elements += 1
            seen[name] = seen.get(name, 0) + 1
            if not selfClosing:
                stack.append(name)
        i = end + 1

    if stack:
        fail(f"the document ends with {', '.join('<' + s + '>' for s in stack)} still open")
    return elements, seen


def check_xml(data):
    elements, _ = scan_xml(data)
    if elements < 2:
        fail(f"the document holds {elements} element(s), so there is nothing below the root")
    ok(f"{elements} elements, all balanced")


def check_svg(data):
    """An SVG that parses can still draw nothing.

    Inkscape stands beside this one and answers whether the file renders. What
    this adds is the shape of the document: the root really is an svg with a
    viewBox, and it holds drawable elements rather than only metadata. Written
    to the format, not to our output - the list below is what SVG calls a basic
    shape, and a generator that quietly stopped emitting them would be the right
    size and would still open.
    """
    elements, seen = scan_xml(data)

    text = data.decode("utf-8")
    if "<svg" not in text:
        fail("there is no svg root element")
    if 'xmlns="http://www.w3.org/2000/svg"' not in text:
        fail("the root carries no SVG namespace, so a renderer has no reason to draw it")
    if "viewBox=" not in text:
        fail("the root declares no viewBox")

    drawable = {"rect", "circle", "ellipse", "line", "polyline", "polygon", "path", "text"}
    shapes = sum(count for name, count in seen.items() if name in drawable)
    if shapes == 0:
        fail(f"the drawing holds {elements} element(s) and not one of them draws anything")
    ok(f"{shapes} drawable shapes out of {elements} elements")


def check_html(data):
    """Balanced, complete and with real blocks in the body.

    HTML is the weakest format in this project for checking, and that is a
    property of the format rather than of this machine. A parser is required to
    recover from almost anything, so the tolerant reader beside this one accepts
    documents nobody would want - "it parsed" is close to no information. This
    check is written to the specification instead: the void elements below are
    the HTML5 list, everything else has to close, and an ampersand has to start
    a real entity reference.
    """
    VOID = {"area", "base", "br", "col", "embed", "hr", "img", "input",
            "link", "meta", "param", "source", "track", "wbr"}
    BLOCKS = {"p", "h1", "h2", "h3", "ul", "ol", "table", "blockquote"}

    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        fail(f"not valid UTF-8: {exc}")

    if not text.lower().startswith("<!doctype html>"):
        fail("the document does not open with an HTML5 doctype")
    if not text.rstrip().endswith("</html>"):
        fail("the document does not end with a closing html tag")

    entity = re.compile(r"&(?:[a-zA-Z][a-zA-Z0-9]{1,31}|#[0-9]+|#x[0-9a-fA-F]+);")
    tag = re.compile(r"<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>")

    stack, blocks, i = [], 0, 0
    for m in tag.finditer(text):
        # Character data between the previous tag and this one.
        chunk = text[i:m.start()]
        for pos, ch in enumerate(chunk):
            if ch == "&" and not entity.match(chunk, pos):
                fail(f"a bare ampersand at offset {i + pos} - text has to escape it as &amp;")
        i = m.end()

        closing, name, rest = m.group(1) == "/", m.group(2).lower(), m.group(3)
        if name in VOID or rest.rstrip().endswith("/"):
            continue
        if closing:
            if not stack:
                fail(f"</{name}> closes an element that was never opened")
            opened = stack.pop()
            if opened != name:
                fail(f"</{name}> closes while <{opened}> is open")
        else:
            stack.append(name)
            if name in BLOCKS:
                blocks += 1

    if stack:
        fail(f"the document ends with {', '.join('<' + s + '>' for s in stack)} still open")
    if blocks == 0:
        fail("the body holds no block elements, so the page renders as nothing")
    ok(f"{blocks} blocks, all tags balanced")


CHECKS = {"png": check_png, "wav": check_wav, "pdf": check_pdf, "zip": check_zip,
          "log": check_log, "csv": check_csv, "json": check_json, "xml": check_xml,
          "svg": check_svg, "html": check_html}

if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] not in CHECKS:
        print("FAIL usage: strict.py <" + "|".join(CHECKS) + "> <path>")
        sys.exit(1)
    with open(sys.argv[2], "rb") as handle:
        CHECKS[sys.argv[1]](handle.read())
