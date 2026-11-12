"""The map of a file, read back from its structure.

A generator marks the map of the file it writes - which bytes are structure,
content, padding and the end (internal/format/layout.go). This reads the same
map out of the finished file, from the format's own rules, so the guard
TestEveryFormatsMapIsTheOneItsStructureReadsBack can hold one against the
other. The same bargain as strict.py: a second implementation, in another
language, written to the specification rather than to what the generator
meant. Its own walkers rather than strict.py's, which end the process on the
first refusal - right for a verdict, and fatal to a batch of maps.

Coarse on purpose. Every role is one or a few regions, never one per record:
an archive's entries are content with their local headers, a picture's chunks
are content with their checksums. A damage aims at a role, and a role is all
the map has to answer.

Usage: python layout.py --batch
Reads one JSON request per line, {"format": ..., "path": ...}, and writes one
JSON answer per line in the order asked, {"path": ..., "regions": [[role, at,
length], ...]} or {"path": ..., "error": ...}.
"""

import json
import re
import struct
import sys

STRUCTURE, CONTENT, PADDING, END = "structure", "content", "padding", "end"


class NoMap(Exception):
    """The file does not have the shape its map is read from."""


def merged(regions):
    """Neighbouring regions of one role as one, and empty ones dropped."""
    out = []
    for role, at, length in regions:
        if length == 0:
            continue
        if out and out[-1][0] == role and out[-1][1] + out[-1][2] == at:
            out[-1][2] += length
        else:
            out.append([role, at, length])
    return out


def layout_text(data):
    """A text is its words. The label, the records and the padding note of
    the last record are all words a reader hands back."""
    return [[CONTENT, 0, len(data)]]


PNG_ROLES = {b"IHDR": STRUCTURE, b"IDAT": CONTENT, b"tfGp": PADDING, b"IEND": END}


def layout_png(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise NoMap("no PNG signature")
    out, pos = [[STRUCTURE, 0, 8]], 8
    while pos < len(data):
        if pos + 8 > len(data):
            raise NoMap(f"a chunk header runs past the end at {pos}")
        length = struct.unpack(">I", data[pos:pos + 4])[0]
        kind = data[pos + 4:pos + 8]
        if pos + 12 + length > len(data):
            raise NoMap(f"chunk {kind!r} at {pos} declares {length} B and runs past the end")
        out.append([PNG_ROLES.get(kind, STRUCTURE), pos, 12 + length])
        pos += 12 + length
    return out


RIFF_ROLES = {b"data": CONTENT, b"JUNK": PADDING}


def riff_chunks(data, start, roles):
    """The chunks of a RIFF list from start, each as a region, an alignment
    byte after an odd length counted to its chunk unless it is the last one
    (the walk strict.check_wav makes)."""
    out, pos = [], start
    while pos + 8 <= len(data):
        kind = data[pos:pos + 4]
        length = struct.unpack("<I", data[pos + 4:pos + 8])[0]
        end = pos + 8 + length
        if length % 2 and end < len(data):
            end += 1
        out.append([roles.get(kind, STRUCTURE), pos, end - pos])
        pos = end
    if pos != len(data):
        raise NoMap(f"the chunks end at {pos} and the file is {len(data)} B")
    return out


def layout_wav(data):
    if data[:4] != b"RIFF" or data[8:12] != b"WAVE":
        raise NoMap("no RIFF WAVE header")
    return [[STRUCTURE, 0, 12]] + riff_chunks(data, 12, RIFF_ROLES)


def layout_webp(data):
    """The container's header, the picture's chunk, any other chunk as padding,
    and whatever lies past the length the container declares as padding."""
    if data[:4] != b"RIFF" or data[8:12] != b"WEBP":
        raise NoMap("no RIFF WEBP header")
    declared = 8 + struct.unpack("<I", data[4:8])[0]
    if declared > len(data):
        raise NoMap(f"the container declares {declared} B and the file is {len(data)} B")
    roles = {b"VP8L": CONTENT, b"VP8 ": CONTENT}
    out = [[STRUCTURE, 0, 12]]
    out += [[r if r != STRUCTURE else PADDING, at, n] for r, at, n in riff_chunks(data[:declared], 12, roles)]
    out.append([PADDING, declared, len(data) - declared])
    return out


def iso_boxes(data, roles):
    """The top level boxes of an ISO base media file, each by the role its name
    has in roles, structure when it has none."""
    out, pos = [], 0
    while pos + 8 <= len(data):
        size = struct.unpack(">I", data[pos:pos + 4])[0]
        kind = data[pos + 4:pos + 8]
        if size == 1:
            size = struct.unpack(">Q", data[pos + 8:pos + 16])[0]
        elif size == 0:
            size = len(data) - pos
        if size < 8 or pos + size > len(data):
            raise NoMap(f"box {kind!r} at {pos} declares {size} B")
        out.append([roles.get(kind, STRUCTURE), pos, size])
        pos += size
    if pos != len(data):
        raise NoMap(f"the boxes end at {pos} and the file is {len(data)} B")
    return out


def layout_avif(data):
    return iso_boxes(data, {b"mdat": CONTENT, b"free": PADDING, b"skip": PADDING})


def layout_jxl(data):
    return iso_boxes(data, {b"jxlc": CONTENT, b"jxlp": CONTENT, b"free": PADDING, b"skip": PADDING})


def ebml_id(data, at):
    """An element ID at at, with its marker bits kept, and its length."""
    first = data[at]
    width = 1
    while width <= 4 and not first & (0x80 >> (width - 1)):
        width += 1
    if width > 4:
        raise NoMap(f"no element ID at {at}")
    return int.from_bytes(data[at:at + width], "big"), width


def ebml_size(data, at):
    """An element's data size at at, and the length of the field."""
    first = data[at]
    width = 1
    while width <= 8 and not first & (0x80 >> (width - 1)):
        width += 1
    if width > 8:
        raise NoMap(f"no size at {at}")
    value = first & (0xFF >> width)
    for b in data[at + 1:at + width]:
        value = value << 8 | b
    return value, width


WEBM_ROLES = {0x1F43B675: CONTENT, 0xEC: PADDING, 0x1C53BB6B: END}


def layout_webm(data):
    """The EBML header and the segment's headers, the seek head, the info and
    the tracks as structure, the clusters as content, a Void as padding and the
    Cues as the end."""
    out, pos = [], 0
    while pos < len(data):
        ident, w = ebml_id(data, pos)
        size, s = ebml_size(data, pos + w)
        if ident != 0x18538067:
            out.append([STRUCTURE, pos, w + s + size])
            pos += w + s + size
            continue
        out.append([STRUCTURE, pos, w + s])
        inner, stop = pos + w + s, pos + w + s + size
        while inner < stop:
            child, cw = ebml_id(data, inner)
            csize, cs = ebml_size(data, inner + cw)
            out.append([WEBM_ROLES.get(child, STRUCTURE), inner, cw + cs + csize])
            inner += cw + cs + csize
        pos = stop
    if pos != len(data):
        raise NoMap(f"the elements end at {pos} and the file is {len(data)} B")
    return out


def layout_mp4(data):
    return iso_boxes(data, {b"mdat": CONTENT, b"free": PADDING, b"skip": PADDING})


def layout_pdf(data):
    """The header lines, the objects up to the cross reference table startxref
    points at, the table with the trailer, any comment after the trailer as
    padding, and startxref with the closing marker - the end in two parts."""
    if not data.startswith(b"%PDF-"):
        raise NoMap("no PDF header")
    first = re.search(rb"\d+ 0 obj\n", data)
    sx = data.rfind(b"startxref\n")
    if first is None or sx < 0:
        raise NoMap("no object or no startxref")
    xref = int(data[sx + 10:data.index(b"\n", sx + 10)])
    trailer = data.find(b"trailer\n", xref)
    if data[xref:xref + 5] != b"xref\n" or trailer < 0:
        raise NoMap(f"startxref points at {xref}, where no cross reference table starts")
    closed = data.index(b">>\n", trailer) + 3
    return [[STRUCTURE, 0, first.start()], [CONTENT, first.start(), xref - first.start()],
            [END, xref, closed - xref], [PADDING, closed, sx - closed], [END, sx, len(data) - sx]]


ZIP_FILLERS = (b"tfg-padding.bin", b"tfg/padding.bin")


def zip_entries(data):
    """The end of central directory record, where the directory starts, and
    each entry the directory lists as (local header offset, name)."""
    eocd = -1
    for at in range(len(data) - 22, -1, -1):
        if data[at:at + 4] == b"PK\x05\x06" and at + 22 + struct.unpack("<H", data[at + 20:at + 22])[0] == len(data):
            eocd = at
            break
    if eocd < 0:
        raise NoMap("no end of central directory record")
    count, size, directory = struct.unpack("<HII", data[eocd + 10:eocd + 20])
    if directory == 0xFFFFFFFF or count == 0xFFFF:
        locator = eocd - 20
        if data[locator:locator + 4] != b"PK\x06\x07":
            raise NoMap("a ZIP64 directory with no locator")
        record = struct.unpack("<Q", data[locator + 8:locator + 16])[0]
        count, size, directory = struct.unpack("<QQQ", data[record + 32:record + 56])
    entries, pos = [], directory
    for _ in range(count):
        if data[pos:pos + 4] != b"PK\x01\x02":
            raise NoMap(f"no central directory entry at {pos}")
        n, extra, note = struct.unpack("<HHH", data[pos + 28:pos + 34])
        offset = struct.unpack("<I", data[pos + 42:pos + 46])[0]
        name = data[pos + 46:pos + 46 + n]
        if offset == 0xFFFFFFFF:
            # The ZIP64 extra field holds the sizes it replaces first, then
            # the offset.
            field = data[pos + 46 + n:pos + 46 + n + extra]
            sizes = struct.unpack("<II", data[pos + 20:pos + 28])
            skip = 8 * sum(1 for s in sizes if s == 0xFFFFFFFF)
            i = 0
            while i + 4 <= len(field):
                kind, length = struct.unpack("<HH", field[i:i + 4])
                if kind == 1:
                    offset = struct.unpack("<Q", field[i + 4 + skip:i + 12 + skip])[0]
                i += 4 + length
        entries.append((offset, name))
        pos += 46 + n + extra + note
    return eocd, directory, sorted(entries)


def layout_zip(data):
    """The entries of directories before the first file as structure, the
    files' entries as content, the filler entry as padding, the central
    directory with its end record as the end, and the archive comment as
    padding. Read from the central directory, not from the order of writing."""
    eocd, directory, entries = zip_entries(data)
    out = []
    files_seen = False
    for i, (offset, name) in enumerate(entries):
        end = entries[i + 1][0] if i + 1 < len(entries) else directory
        if name in ZIP_FILLERS:
            role = PADDING
        elif name.endswith(b"/") and not files_seen:
            role = STRUCTURE
        else:
            role, files_seen = CONTENT, True
        out.append([role, offset, end - offset])
    if entries and entries[0][0] != 0:
        raise NoMap(f"the first entry starts at {entries[0][0]}")
    out.append([END, directory, eocd + 22 - directory])
    out.append([PADDING, eocd + 22, len(data) - eocd - 22])
    return out


def gzip_header_length(data):
    """The fixed part of a gzip header, the extra field, and everything the
    flags add after it, as regions, and where the stream starts."""
    if data[:3] != b"\x1f\x8b\x08":
        raise NoMap("no gzip header")
    flags, pos = data[3], 10
    out = [[STRUCTURE, 0, 10]]
    if flags & 4:
        xlen = struct.unpack("<H", data[10:12])[0]
        out.append([PADDING, 10, 2 + xlen])
        pos = 12 + xlen
    start = pos
    for flag in (8, 16):
        if flags & flag:
            pos = data.index(b"\x00", pos) + 1
    if flags & 2:
        pos += 2
    out.append([STRUCTURE, start, pos - start])
    return out, pos


def stored_blocks(data, pos):
    """The stored deflate blocks from pos, as (file offset of the block,
    offset of its data in the stream, length), and where they end."""
    blocks, t = [], 0
    while True:
        header = data[pos]
        if data[pos:pos + 2] == b"\x03\x00":
            # An empty last block of fixed codes - BFINAL, BTYPE 01 and the
            # end of block code - which is how compress/flate closes a stream.
            return blocks, pos + 2
        if header & 0b110:
            raise NoMap(f"the block at {pos} is not stored")
        n = struct.unpack("<H", data[pos + 1:pos + 3])[0]
        blocks.append((pos, t, n))
        t += n
        pos += 5 + n
        if header & 1:
            return blocks, pos


def layout_targz(data, settings):
    """The gzip header, then - in a stored stream - the tar's directories,
    files, filler entry and end of archive marker where their bytes land in the
    file, and the end marker with the closing block and the trailer as the end.
    A compressed stream is content from its first byte to the trailer."""
    out, pos = gzip_header_length(data)
    if settings.get("compression", "none") != "none":
        return out + [[CONTENT, pos, len(data) - 8 - pos], [END, len(data) - 8, 8]]
    blocks, stop = stored_blocks(data, pos)
    tar = b"".join(data[at + 5:at + 5 + n] for at, _, n in blocks)

    def where(t):
        for at, start, n in blocks:
            if t == start:
                return at
            if start < t < start + n:
                return at + 5 + t - start
        raise NoMap(f"the tar's byte {t} is in no block")

    t, files_seen, regions = 0, False, []
    while t + 512 <= len(tar):
        head = tar[t:t + 512]
        if head == bytes(512):
            regions.append((END, t))
            break
        name = head[:100].rstrip(b"\x00")
        size = int(head[124:136].rstrip(b"\x00 ") or b"0", 8)
        kind = head[156:157]
        if name == b"tfg-padding.bin":
            role = PADDING
        elif kind == b"5" and not files_seen:
            role = STRUCTURE
        else:
            role, files_seen = CONTENT, True
        regions.append((role, t))
        t += 512 + (size + 511) // 512 * 512
    for i, (role, start) in enumerate(regions):
        end = where(regions[i + 1][1]) if i + 1 < len(regions) else len(data)
        out.append([role, where(start), end - where(start)])
    return out


def layout_bmp(data):
    """The two headers, then whatever lies before the offset the file header
    gives for the pixels, then the pixels the information header counts."""
    if data[:2] != b"BM":
        raise NoMap("no BM signature")
    pixels_at = struct.unpack("<I", data[10:14])[0]
    head = 14 + struct.unpack("<I", data[14:18])[0]
    pixels = struct.unpack("<I", data[34:38])[0]
    if pixels_at + pixels != len(data):
        raise NoMap(f"the pixels run from {pixels_at} for {pixels} B and the file is {len(data)} B")
    return [[STRUCTURE, 0, head], [PADDING, head, pixels_at - head], [CONTENT, pixels_at, pixels]]


def sub_blocks_end(data, pos):
    """Where a chain of GIF sub blocks from pos ends, its terminator counted."""
    while True:
        if pos >= len(data):
            raise NoMap("a chain of sub blocks runs off the end")
        size = data[pos]
        pos += 1 + size
        if size == 0:
            return pos


def layout_gif(data):
    """The signature and the screen descriptor, then everything up to the
    trailer - the palette, the frames and their extensions - as content, a
    comment as padding, and the trailer as the end."""
    if data[:6] not in (b"GIF87a", b"GIF89a"):
        raise NoMap("no GIF signature")
    out, pos = [[STRUCTURE, 0, 13]], 13
    packed = data[10]
    if packed & 0x80:
        table = 3 * (1 << ((packed & 0x07) + 1))
        out.append([CONTENT, pos, table])
        pos += table
    while pos < len(data):
        start, marker = pos, data[pos]
        if marker == 0x3B:
            out.append([END, pos, 1])
            pos += 1
            continue
        if marker == 0x21:
            label = data[pos + 1]
            pos = sub_blocks_end(data, pos + 2)
            out.append([PADDING if label == 0xFE else CONTENT, start, pos - start])
            continue
        if marker == 0x2C:
            local = data[pos + 9]
            pos += 10
            if local & 0x80:
                pos += 3 * (1 << ((local & 0x07) + 1))
            pos = sub_blocks_end(data, pos + 1)
            out.append([CONTENT, start, pos - start])
            continue
        raise NoMap(f"the byte {marker:#04x} at {pos} is not a block marker")
    return out


def layout_jpg(data):
    """The start marker, the comment segments, then every segment and the scan
    up to the end marker as content, fill bytes in front of a marker as
    padding, and the end marker.

    Fill bytes are X'FF' bytes, any number of which may stand in front of any
    marker (ITU T.81 B.1.1.2): every FF of a run but the last, which starts the
    marker itself."""
    if data[:2] != b"\xff\xd8":
        raise NoMap("no start of image marker")
    out, pos = [[STRUCTURE, 0, 2]], 2
    while pos < len(data):
        if data[pos] != 0xFF:
            raise NoMap(f"no marker at {pos}")
        run = pos
        while run + 1 < len(data) and data[run + 1] == 0xFF:
            run += 1
        if run > pos:
            out.append([PADDING, pos, run - pos])
            pos = run
            continue
        kind = data[pos + 1]
        if kind == 0xD9:
            out.append([END, pos, 2])
            pos += 2
            continue
        length = struct.unpack(">H", data[pos + 2:pos + 4])[0]
        end = pos + 2 + length
        if kind == 0xDA:
            # The scan runs to the next marker that is not a stuffed byte or
            # a restart marker.
            while end + 1 < len(data) and not (data[end] == 0xFF and data[end + 1] not in (0x00, *range(0xD0, 0xD8))):
                end += 1
        out.append([PADDING if kind == 0xFE else CONTENT, pos, end - pos])
        pos = end
    return out


def layout_ico(data):
    """The directory, then whatever lies before the offset its one entry gives,
    then the picture it counts, then whatever lies behind it - space no entry
    points at, as the space in front is."""
    if data[:4] != b"\x00\x00\x01\x00" or struct.unpack("<H", data[4:6])[0] != 1:
        raise NoMap("not an icon directory of one entry")
    head = 6 + 16
    size, at = struct.unpack("<II", data[14:22])
    if at < head or at + size > len(data):
        raise NoMap(f"the picture runs from {at} for {size} B and the file is {len(data)} B")
    out = [[STRUCTURE, 0, head], [PADDING, head, at - head], [CONTENT, at, size]]
    if at + size < len(data):
        out.append([PADDING, at + size, len(data) - at - size])
    return out


def tiff_values(data, kind, count, value_at):
    """The values of one directory entry, inline or where it points."""
    width = {3: 2, 4: 4}[kind]
    form = {3: "<H", 4: "<I"}[kind]
    at = value_at if width * count <= 4 else struct.unpack("<I", data[value_at:value_at + 4])[0]
    return [struct.unpack(form, data[at + width * i:at + width * (i + 1)])[0] for i in range(count)]


def layout_tiff(data):
    """The header, the strips the directory points at, and the directory - the
    end, as the generator writes it last. What lies between the header and the
    first strip is not anything the directory names."""
    if data[:4] != b"II*\x00":
        raise NoMap("no little-endian TIFF header")
    ifd_at = struct.unpack("<I", data[4:8])[0]
    count = struct.unpack("<H", data[ifd_at:ifd_at + 2])[0]
    tags = {}
    for i in range(count):
        at = ifd_at + 2 + 12 * i
        tag, kind, n = struct.unpack("<HHI", data[at:at + 8])
        if tag in (273, 279):
            tags[tag] = tiff_values(data, kind, n, at + 8)
    if 273 not in tags or 279 not in tags:
        raise NoMap("the directory names no strips")
    first = min(tags[273])
    last = max(o + n for o, n in zip(tags[273], tags[279]))
    if last != ifd_at:
        raise NoMap(f"the strips end at {last} and the directory starts at {ifd_at}")
    return [[STRUCTURE, 0, 8], [PADDING, 8, first - 8], [CONTENT, first, last - first],
            [END, ifd_at, len(data) - ifd_at]]


TEXT = ("csv", "eml", "geojson", "html", "json", "log", "md", "otlp", "rtf",
        "svg", "toml", "txt", "xml", "yaml")

LAYOUTS = {"png": layout_png, "wav": layout_wav, "bmp": layout_bmp, "tiff": layout_tiff, "ico": layout_ico, "gif": layout_gif, "jpg": layout_jpg, "webp": layout_webp, "avif": layout_avif, "jxl": layout_jxl, "mp4": layout_mp4, "webm": layout_webm, "pdf": layout_pdf, "zip": layout_zip, "docx": layout_zip, "xlsx": layout_zip, "pptx": layout_zip}
LAYOUTS.update({kind: layout_text for kind in TEXT})
LAYOUTS["targz"] = layout_targz

# The readers that take the settings the file was made with as well as its
# bytes - only where the map's shape follows a setting the file does not say.
TAKES_SETTINGS = {"targz"}


def answer(line):
    try:
        ask = json.loads(line)
        kind, path = ask["format"], ask["path"]
        settings = ask.get("settings") or {}
    except (ValueError, KeyError, TypeError) as exc:
        return {"path": "", "error": f"the request {line[:80]!r} is not readable: {exc}"}
    if kind not in LAYOUTS:
        return {"path": path, "error": f"there is no map for {kind!r}"}
    try:
        with open(path, "rb") as handle:
            data = handle.read()
        read = LAYOUTS[kind]
        regions = read(data, settings) if kind in TAKES_SETTINGS else read(data)
        return {"path": path, "regions": merged(regions)}
    except NoMap as exc:
        return {"path": path, "error": f"{kind}: {exc}"}
    except Exception as exc:  # noqa: BLE001 - a fault in a reader, named in the answer
        return {"path": path, "error": f"{kind}: the reader stopped on {type(exc).__name__}: {exc}"}


def main():
    if sys.argv[1:] != ["--batch"]:
        print(__doc__, file=sys.stderr)
        return 2
    for line in sys.stdin:
        if line.strip():
            print(json.dumps(answer(line)), flush=False)
    return 0


if __name__ == "__main__":
    sys.exit(main())
