// Package opc builds Open Packaging Convention containers.
//
// DOCX, XLSX and PPTX are the same thing underneath: a ZIP holding XML parts
// plus a table saying what each part is and a set of relationship files saying
// how they refer to one another. Three formats share this rather than carrying
// three copies of the same container, the same padding arithmetic and the same
// measured limit.
//
// The limit is the part worth reading. A ZIP archive comment is the obvious
// place to put padding, it is where the ZIP generator puts it, and the format
// document said 64 KB of it was fine here too. Measured on 2026-08-19 against
// LibreOffice: the comment stops working at 513 bytes. Exactly 512 passes,
// 513 does not, the same for all three formats and whatever the file size, so
// it is a buffer in the reader rather than anything about the package. 7-Zip
// and Python's zipfile accept every one of the files it rejects, because they
// read the ZIP and not the package. Details in docs/MVP-FORMATS.md section 2.9.
//
// So the padding lives in an extra part instead, and the comment is kept for
// the last few bytes an extra part is too coarse to reach.
package opc

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

const (
	// CommentLimit is how much archive comment a package may carry.
	//
	// Measured, not taken from the ZIP specification, which allows 65 535.
	// The number that matters is what a reader accepts.
	CommentLimit = 512

	// FillerName is the part carrying padding. Its extension is declared in
	// the content types table like any other part, because the specification
	// says every part has a content type - and while the reader measured here
	// does not check that, a package that is only accidentally acceptable is
	// not the same as a correct one.
	FillerName      = "tfg/padding.bin"
	fillerExtension = "bin"

	// NoFiller means the package carries no padding part at all.
	NoFiller = -1

	// Written before each write of the padding part, so a large package costs
	// a buffer rather than its own size.
	writeChunk = 32 * 1024
)

// fixedTime is the timestamp on every part. The same value the ZIP generator
// uses, because two containers in one tool disagreeing about what "no time"
// means is the kind of difference nobody would predict.
var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// Part is one named piece of the package.
type Part struct {
	// Name is the path inside the package, such as word/document.xml.
	Name string
	// Body is the exact bytes of the part.
	Body []byte
	// Store asks for no compression. The XML parts are compressed, because
	// they are text and it is what every real writer does.
	Store bool
}

// Package is a container ready to be written.
type Package struct {
	Parts []Part
	// Comment is how many bytes of archive comment to write, at most
	// CommentLimit.
	Comment int
	// Filler is how many bytes the padding part carries, or NoFiller.
	Filler int64
	Seed   uint64
}

// ContentTypes builds [Content_Types].xml.
//
// The two defaults are what every package needs: one for the relationship
// files and one for anything else ending in xml. The third is ours, and it is
// declared whether or not a padding part is present, so that adding padding
// does not change the size of this part and take the arithmetic with it.
func ContentTypes(overrides [][2]string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Default Extension="` + fillerExtension + `" ContentType="application/octet-stream"/>`)
	for _, o := range overrides {
		b.WriteString(`<Override PartName="` + o[0] + `" ContentType="` + o[1] + `"/>`)
	}
	b.WriteString(`</Types>`)
	return []byte(b.String())
}

// Rels builds a relationship part from triples of id, type and target.
func Rels(entries [][3]string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for _, e := range entries {
		b.WriteString(`<Relationship Id="` + e[0] + `" Type="` + e[1] + `" Target="` + e[2] + `"/>`)
	}
	b.WriteString(`</Relationships>`)
	return []byte(b.String())
}

// Shape is what the parts cost, worked out by encoding them rather than by
// arithmetic over header sizes.
//
// Encoding twice rather than adding up field widths is deliberate. The cost of
// an entry depends on what the writer decides to emit - a data descriptor, an
// extra field - and a number derived from the specification would be right
// until the day the standard library changed its mind, at which point every
// file would be the wrong size and nothing here would say why.
type Shape struct {
	// Bare is the package with no comment and no padding part.
	Bare int64
	// FillerOverhead is what an empty padding part costs on top of Bare.
	FillerOverhead int64
}

// Measure works out the shape of a set of parts.
func Measure(parts []Part) (Shape, error) {
	bare, err := size(Package{Parts: parts, Filler: NoFiller})
	if err != nil {
		return Shape{}, err
	}
	empty, err := size(Package{Parts: parts, Filler: 0})
	if err != nil {
		return Shape{}, err
	}
	return Shape{Bare: bare, FillerOverhead: empty - bare}, nil
}

// Unreachable says a size cannot be produced, and names the two that can.
//
// The package can grow by one byte at a time through the comment up to
// CommentLimit, and from FillerOverhead upwards without a ceiling. When those
// two ranges meet - which they do for every part name in use here - every size
// above the floor is reachable and this is never returned. It exists because
// "the ranges meet" is a property of a name somebody could change.
type Unreachable struct {
	Below, Above int64
}

func (e *Unreachable) Error() string {
	return fmt.Sprintf("no package can be %d B to %d B", e.Below, e.Above)
}

// Zip32Ceiling is where ZIP stops describing itself in thirty two bits and
// archive/zip starts writing the zip64 records instead.
const Zip32Ceiling = 1<<32 - 1

// TooLarge is a package that would cross into zip64.
//
// Not because zip64 is wrong - archive/zip writes it correctly - but because
// Size cannot see it coming. It measures the package with the padding left out
// and adds the padding back afterwards, so while it is measuring, the entry
// that carries the bulk is nought bytes long, and nought bytes never triggers
// zip64. Measured on 2026-08-26 with tools/probes/zip64, the plan against what
// really came out of the writer for a docx:
//
//	64 MiB          agree
//	4 GiB - 1 MiB   agree
//	4 GiB + 1 MiB   the writer wrote 104 B more than the plan
//	5 GiB           the writer wrote 104 B more than the plan
//
// Nothing that works is taken away by refusing here, and that is measured
// rather than assumed: the engine compares what the writer produced against
// the plan and deletes any file that misses, so a package past this line could
// not succeed before either. What changes is that the person is told before
// four gigabytes are written and removed.
type TooLarge struct {
	Want    int64
	Ceiling int64
}

func (e *TooLarge) Error() string {
	return fmt.Sprintf("no package can be %d B, which is past the %d B a zip32 archive describes", e.Want, e.Ceiling)
}

// Settle decides how a package of this shape reaches an exact size.
func Settle(parts []Part, shape Shape, want int64) (Package, error) {
	if want > Zip32Ceiling {
		return Package{}, &TooLarge{Want: want, Ceiling: Zip32Ceiling}
	}
	p := Package{Parts: parts, Filler: NoFiller}
	delta := want - shape.Bare
	switch {
	case delta == 0:
		return p, nil
	case delta < 0:
		return Package{}, fmt.Errorf("opc: %d B is below the %d B floor", want, shape.Bare)
	case delta <= CommentLimit:
		p.Comment = int(delta)
		return p, nil
	case delta >= shape.FillerOverhead:
		p.Filler = delta - shape.FillerOverhead
		return p, nil
	}
	return Package{}, &Unreachable{
		Below: shape.Bare + CommentLimit,
		Above: shape.Bare + shape.FillerOverhead,
	}
}

// Size is what a settled package will weigh. Padding is counted rather than
// written, so asking about a package of many gigabytes costs nothing.
func Size(p Package) (int64, error) {
	payload := p.Filler
	if payload > 0 {
		p.Filler = 0
	} else {
		payload = 0
	}
	n, err := size(p)
	if err != nil {
		return 0, err
	}
	return n + payload, nil
}

func size(p Package) (int64, error) {
	c := &counter{}
	if err := write(context.Background(), c, p, false); err != nil {
		return 0, err
	}
	return c.n, nil
}

// Write emits the package.
func Write(ctx context.Context, w io.Writer, p Package) error {
	return write(ctx, w, p, true)
}

func write(ctx context.Context, w io.Writer, p Package, withPadding bool) error {
	zw := zip.NewWriter(w)
	if p.Comment > 0 {
		if p.Comment > CommentLimit {
			return fmt.Errorf("opc: a comment of %d B is past the %d B a reader accepts", p.Comment, CommentLimit)
		}
		if err := zw.SetComment(comment(p.Comment)); err != nil {
			return fmt.Errorf("opc: the archive comment was refused: %w", err)
		}
	}

	// One compressor and one buffer for every part, rather than one each.
	// Measured: a twelve part presentation allocated 303 objects writing a
	// 64 MiB file against a ceiling of 128, almost all of it here.
	press := &compressor{}
	for _, part := range p.Parts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := writePart(zw, part, press); err != nil {
			return err
		}
	}

	if p.Filler != NoFiller {
		// Stored rather than compressed, so a byte of padding is a byte of
		// file. Deflating it would make the size depend on how well random
		// bytes happen to compress, which is not a thing to promise.
		//
		// The checksum has to be known before the header is written, and the
		// padding is never held in memory, so it is generated twice: once to
		// check it and once to write it. Both passes are the same arithmetic
		// from the same seed, which is what makes them agree.
		head := &zip.FileHeader{Name: FillerName, Method: zip.Store, Modified: fixedTime}
		head.CRC32 = fillerChecksum(p.Seed, p.Filler)
		head.UncompressedSize64 = uint64(p.Filler)
		head.CompressedSize64 = uint64(p.Filler)
		entry, err := zw.CreateRaw(head)
		if err != nil {
			return err
		}
		if withPadding {
			if err := writeFiller(ctx, entry, p.Seed, p.Filler); err != nil {
				return err
			}
		}
	}

	return zw.Close()
}

// writePart writes one part with its sizes and checksum in the header.
//
// CreateRaw rather than CreateHeader, and this is measured rather than tidy.
// CreateHeader always sets the data descriptor flag, which puts the sizes and
// the checksum AFTER the data instead of in the header. That is legal ZIP, and
// 7-Zip, Explorer and Python all read it - but on 2026-08-19 LibreOffice's
// Writer and Impress import filters refused every package built that way while
// its Calc filter accepted them. One toolkit, three filters, two answers. The
// same parts repacked without the flag loaded in all three.
//
// So the bytes are compressed here, the checksum is taken here, and the header
// carries both.
func writePart(zw *zip.Writer, part Part, press *compressor) error {
	head := &zip.FileHeader{Name: part.Name, Modified: fixedTime}
	payload := part.Body
	if part.Store {
		head.Method = zip.Store
	} else {
		head.Method = zip.Deflate
		var err error
		payload, err = press.press(part.Body)
		if err != nil {
			return err
		}
	}
	head.CRC32 = crc32.ChecksumIEEE(part.Body)
	head.UncompressedSize64 = uint64(len(part.Body))
	head.CompressedSize64 = uint64(len(payload))

	entry, err := zw.CreateRaw(head)
	if err != nil {
		return err
	}
	_, err = entry.Write(payload)
	return err
}

// compressor keeps one deflate writer and one buffer for a whole package.
type compressor struct {
	buf bytes.Buffer
	fw  *flate.Writer
}

func (c *compressor) press(body []byte) ([]byte, error) {
	c.buf.Reset()
	if c.fw == nil {
		fw, err := flate.NewWriter(&c.buf, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
		c.fw = fw
	} else {
		c.fw.Reset(&c.buf)
	}
	if _, err := c.fw.Write(body); err != nil {
		return nil, err
	}
	if err := c.fw.Close(); err != nil {
		return nil, err
	}
	// Copied, because the buffer is reused for the next part.
	out := make([]byte, c.buf.Len())
	copy(out, c.buf.Bytes())
	return out, nil
}

// fillerChecksum is the first of the two passes over the padding.
func fillerChecksum(seed uint64, n int64) uint32 {
	sum := crc32.NewIEEE()
	if err := writeFiller(context.Background(), sum, seed, n); err != nil {
		// writeFiller only fails on a write error or a cancelled context, and
		// a hash never refuses a write.
		return 0
	}
	return sum.Sum32()
}

// comment is filler a person can read, because a reader that shows the archive
// comment shows this and a wall of random bytes reads as damage.
func comment(n int) string {
	const words = "tfg padding "
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(words)
	}
	return b.String()[:n]
}

func writeFiller(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n <= 0 {
		return nil
	}
	rng := core.NewRand(seed)
	size := int64(writeChunk)
	if n < size {
		size = n
	}
	buf := make([]byte, size)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		take := int64(len(buf))
		if remaining < take {
			take = remaining
		}
		chunk := buf[:take]
		for i := 0; i < len(chunk); i += 8 {
			var eight [8]byte
			binary.BigEndian.PutUint64(eight[:], rng.Uint64())
			copy(chunk[i:], eight[:])
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= take
	}
	return nil
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// Words is what the three Office formats fill their documents with. Plain
// ASCII on purpose, so nothing here ever needs escaping and the byte count of
// a word is its length.
var Words = []string{
	"report", "invoice", "summary", "quarter", "region", "customer",
	"balance", "revenue", "forecast", "attachment", "reference", "record",
	"delivery", "warehouse", "contract", "renewal", "discount", "shipment",
}

// LineWidth is how many bytes one run of filler text takes, whatever the seed
// chose to put in it.
//
// Fixed rather than natural, and the reason is the smallest file this format
// can produce. Words are of different lengths, so a line built from them is a
// different length for every seed - and then the floor of the format is a
// different number for every seed too. PDF hit exactly this on 2026-08-03:
// across 200 seeds its smallest document moved between 3090 B and 3499 B, and
// a size accepted for six seeds in ten was refused for the rest. An error that
// appears and disappears with the seed is the one thing this tool cannot have.
const LineWidth = 96

// Line builds a deterministic run of words, always LineWidth bytes long.
func Line(rng *rand.Rand, _ int) string {
	var b strings.Builder
	for b.Len() < LineWidth {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(Words[int(rng.UintN(uint(len(Words))))])
	}
	return Fixed(b.String(), LineWidth)
}

// LabelWidth is the room a label gets. Wider than any label core.Label can
// produce, so the text is never cut, and constant so the floor does not move
// with the number of digits in the size.
const LabelWidth = 64

// Fixed cuts or pads text to exactly n bytes.
//
// Padding is spaces, which XML keeps when the element asks it to and which
// nobody sees. The point is that the length of a part stops depending on what
// went into it.
func Fixed(text string, n int) string {
	if len(text) > n {
		return text[:n]
	}
	return text + strings.Repeat(" ", n-len(text))
}

// Text escapes what XML cannot carry raw.
//
// Nothing this tool puts into a document contains any of these today. It is
// here because the day somebody passes a label or a name through, a document
// that silently stops parsing is a worse answer than one that is correct.
func Text(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
