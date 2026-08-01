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


CHECKS = {"png": check_png, "wav": check_wav, "pdf": check_pdf, "zip": check_zip}

if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] not in CHECKS:
        print("FAIL usage: strict.py <" + "|".join(CHECKS) + "> <path>")
        sys.exit(1)
    with open(sys.argv[2], "rb") as handle:
        CHECKS[sys.argv[1]](handle.read())
