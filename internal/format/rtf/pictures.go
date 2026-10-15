package rtf

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// Pictures in the document: real files of a picture format, each in a \pict
// group in a paragraph of its own that starts with its name, after the links
// and before the blocks - where a page has them.
//
// Measured on 2026-10-15 by six readers on documents made by hand around files
// from this program (docs/RTF-2-2026-10-15.md section 1.1). In hex every one of
// them read every picture, LibreOffice, pandoc and Tika giving back the very
// bytes and RichEdit the same picture written again. Written binary, pandoc
// turns the line ends inside the picture into line feeds, so no PNG comes out
// whole - its signature holds one - and once the shift reaches the brace that
// closes the picture it refuses the document. The name in front is the one
// description all six returned: LibreOffice and pandoc kept nothing of the one
// a picture can carry inside its group.
//
// The settings are the ones a mail message and a page have, with the same
// names and the same meaning (package picture), and the group shape is the one
// LibreOffice writes: the picture's size in pixels and on the page, then the
// data, in hex lines of 128.

// picturesGroup is the block the picture settings sit in, on both surfaces.
const picturesGroup = "Pictures in the document"

// maxDocument is the largest document this format plans with pictures in it,
// for the reason a page has its own: far above anything a disk holds, and far
// enough below the int64 every size is counted in that adding the pictures up
// cannot wrap round.
const maxDocument = int64(1) << 60

// blips are the picture formats RTF has a name for, among those this program
// writes - the only ones a document can carry as the files they are.
var blips = map[string]string{"png": `\pngblip`, "jpg": `\jpegblip`}

// blipFormats is the list a refusal names, in the order the detail says it.
const blipFormats = "png, jpg"

const (
	// hexLine is how many hex digits stand on one line, as LibreOffice writes
	// them. Even, so a byte never splits across two lines.
	hexLine = 128
	// twipsPerPixel puts a picture on the page at 96 pixels to the inch, the
	// size a screen shows it at.
	twipsPerPixel = 15
	// columnTwips is the width of the text on the page RTF assumes when a
	// document names none: 12 240 twips of paper less two margins of 1800
	// (RTF 1.6). No picture is set wider, so none stands out past the text.
	columnTwips = 12240 - 2*1800
)

// pictureProperties are the settings of the pictures, and how their bytes are
// written.
func pictureProperties() []format.Property {
	return append(picture.Properties(picturesGroup, picture.Details{
		Count: "How many pictures the document shows, each a real file of its format in a paragraph of its own that starts with its name, such as inline_0001.png. " +
			"They sit above the blocks, at 96 pixels to the inch and never wider than 6 inches. " +
			"LibreOffice, pandoc and Tika return each file as it was written. RichEdit shows the same picture and writes it out again smaller.",
		Format: "The format of the pictures, png or jpg - the two picture formats RTF has a name for. Any other format is refused.",
		Size: "How big each picture is. Written in hex it takes twice as many bytes in the document, with a line end every 128 characters. " +
			"Written binary it takes its own size.",
	}), format.Property{
		Name: PictureData, Kind: format.PropertyChoice,
		Choices: []string{Hex, Binary},
		Default: Hex,
		Group:   picturesGroup,
		Detail: "How the bytes of each picture are written, which changes nothing while inline is 0. " +
			"hex writes them as text, twice as large, the way LibreOffice writes them. " +
			"binary writes them as they are, so the document is no longer text, and a program that counts braces or reads it as text loses its place. " +
			"pandoc changes the line ends inside a binary picture, so no png comes out whole, and it can lose the text after the picture or refuse the whole document. " +
			"LibreOffice, Tika and RichEdit read both.",
	})
}

// pictureLine is one picture, planned: the file, and its paragraph up to where
// the data starts and after it ends.
type pictureLine struct {
	desc        format.Descriptor
	plan        format.Plan
	open, close string
	binary      bool
}

// dataBytes is what the picture's data takes in the document.
func (p pictureLine) dataBytes() int64 {
	if p.binary {
		return p.plan.Bytes
	}
	return hexLength(p.plan.Bytes)
}

// bytes is what the paragraph takes in the document.
func (p pictureLine) bytes() int64 {
	return int64(len(p.open)) + p.dataBytes() + int64(len(p.close))
}

// hexLength is how many bytes n bytes take in hex lines of hexLine, each line
// after a line end of its own.
func hexLength(n int64) int64 {
	digits := 2 * n
	return digits + 2*((digits+hexLine-1)/hexLine)
}

// planPictures plans the pictures, each with the seed of its place (RC4), so
// asking for one more changes none of the others. The format is asked whether
// RTF can carry it even when none are asked for: a value the setting cannot
// take is a mistake either way, and a page and a message refuse it the same
// way.
func planPictures(ctx context.Context, r format.Request, s settings) ([]pictureLine, int64, error) {
	desc, blip, err := pictureFormat(s.pictures.Format)
	if err != nil {
		return nil, 0, err
	}
	out := make([]pictureLine, 0, s.pictures.Count)
	var sum int64
	for i := 0; i < s.pictures.Count; i++ {
		cp, err := format.PlanWith(ctx, desc.Generator, format.Request{Bytes: s.pictures.Size, Seed: core.FileSeed(r.Seed, i), Label: r.Label})
		if err != nil {
			return nil, 0, format.StopOr(ctx, core.Refuse(core.Says("rtf.ThePictureCannotBe", "rtf: the %s picture cannot be made: %w",
				core.A("Format", desc.ID), core.A("Err", err))))
		}
		if cp.Bytes > maxDocument {
			return nil, 0, documentTooLarge(cp.Bytes)
		}
		open, err := pictureOpen(desc, blip, cp, i+1, s.binary)
		if err != nil {
			return nil, 0, err
		}
		line := pictureLine{desc: desc, plan: cp, open: open, close: "}" + endBlock, binary: s.binary}
		if sum += line.bytes(); sum > maxDocument {
			return nil, 0, documentTooLarge(sum)
		}
		out = append(out, line)
	}
	return out, sum, nil
}

// pictureFormat is the descriptor of the pictures' format and the word RTF
// names it by, or the refusal of a format RTF cannot carry.
func pictureFormat(id string) (format.Descriptor, string, error) {
	desc, _, err := picture.Of(id)
	if err != nil {
		return format.Descriptor{}, "", err
	}
	blip, ok := blips[desc.ID]
	if !ok {
		return format.Descriptor{}, "", &format.PropertyValueError{
			Format: "rtf", Key: picture.Format, Value: id,
			Reason: core.Says("rtf.APictureInTheDocumentIs", "a picture in an RTF document has to be one of the two picture formats RTF has a name for"),
			Remedy: core.Says("rtf.UseOneOfThePictureFormats", "Use one of: %s.", core.A("Formats", blipFormats)),
		}
	}
	return desc, blip, nil
}

// pictureOpen is a picture's paragraph up to its data: the name the picture
// has in a mail message and in a page, inline_0001.png, so a tester finds the
// same picture by the same name in all three, then the group with the size in
// pixels and on the page, and with binary the count of bytes that follow.
func pictureOpen(desc format.Descriptor, blip string, cp format.Plan, n int, binary bool) (string, error) {
	w, h, err := pixels(desc, cp)
	if err != nil {
		return "", err
	}
	gw, gh := w*twipsPerPixel, h*twipsPerPixel
	if gw > columnTwips {
		gw, gh = columnTwips, h*columnTwips/w
	}
	var b strings.Builder
	fmt.Fprintf(&b, "inline_%04d%s ", n, desc.Extension)
	b.WriteString(`{\pict` + blip)
	fmt.Fprintf(&b, `\picw%d\pich%d\picwgoal%d\pichgoal%d`, w, h, gw, gh)
	if binary {
		b.WriteString(`\bin` + strconv.FormatInt(cp.Bytes, 10) + " ")
	}
	return b.String(), nil
}

// pixels are the picture's width and height, as its plan records them. A plan
// of png or jpg without them is this program's mistake: the group has to say
// how large the picture is.
func pixels(desc format.Descriptor, cp format.Plan) (int, int, error) {
	w, wok := cp.Properties["width"].(int)
	h, hok := cp.Properties["height"].(int)
	if !wok || !hok || w <= 0 || h <= 0 {
		return 0, 0, core.Defect(fmt.Errorf("rtf: the %s picture's plan gives no width and height: %v", desc.ID, cp.Properties))
	}
	return w, h, nil
}

// documentTooLarge refuses pictures that add up past maxDocument.
func documentTooLarge(asked int64) error {
	return &format.AboveMaximumError{
		Format:    "RTF",
		Requested: asked,
		Maximum:   maxDocument,
		Reason:    core.Says("rtf.ADocumentThisLargeCannotBe", "a document this large cannot be counted to the byte in this build"),
		Hint:      core.Says("rtf.AskForSmallerPictures", "Ask for fewer or smaller pictures."),
	}
}

// describePictures adds the pictures to what reaches the manifest, in the
// words a mail message and a page use for theirs: how many bytes they are as
// files, and as they stand in the document.
func describePictures(props map[string]any, s picture.Settings, lines []pictureLine) {
	var raw, encoded int64
	for _, p := range lines {
		raw += p.plan.Bytes
		encoded += p.dataBytes()
	}
	picture.Describe(props, s, len(lines), raw, encoded)
}

// writePictures writes every picture's paragraph, each file made by its own
// generator as it is written, so a picture of a hundred megabytes never sits in
// memory.
func writePictures(ctx context.Context, w io.Writer, lines []pictureLine) error {
	for _, p := range lines {
		if err := core.WriteAll(w, []byte(p.open)); err != nil {
			return err
		}
		if err := writeData(ctx, w, p); err != nil {
			return err
		}
		if err := core.WriteAll(w, []byte(p.close)); err != nil {
			return err
		}
	}
	return nil
}

// writeData writes one picture's bytes, as they are or in hex lines. The size
// of the whole document rests on each picture coming out as its plan says, so
// one that does not is a defect, not a document of the wrong size.
func writeData(ctx context.Context, w io.Writer, p pictureLine) error {
	out := &dataWriter{w: w, binary: p.binary}
	if err := p.desc.Generator.Write(ctx, out, p.plan); err != nil {
		return err
	}
	if err := out.flush(); err != nil {
		return err
	}
	if out.n != p.plan.Bytes {
		return core.Defect(fmt.Errorf("rtf: the %s picture came out as %d B where its plan gives %d B", p.desc.ID, out.n, p.plan.Bytes))
	}
	return nil
}

// dataWriter writes a picture's bytes in hex lines of hexLine, each after a
// line end of its own, or as they are when binary - and counts the bytes it was
// given either way.
type dataWriter struct {
	w      io.Writer
	binary bool
	buf    []byte
	col    int
	n      int64
}

func (h *dataWriter) Write(p []byte) (int, error) {
	h.n += int64(len(p))
	if h.binary {
		if err := core.WriteAll(h.w, p); err != nil {
			return 0, err
		}
		return len(p), nil
	}
	for rest := p; len(rest) > 0; {
		rest = h.appendLine(rest)
		if err := h.flushFull(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// appendLine adds as much of p in hex as the line holds, opening the line with
// its line end, and returns the rest.
func (h *dataWriter) appendLine(p []byte) []byte {
	if h.col == 0 {
		h.buf = append(h.buf, '\r', '\n')
	}
	k := min(len(p), (hexLine-h.col)/2)
	h.buf = hex.AppendEncode(h.buf, p[:k])
	h.col = (h.col + 2*k) % hexLine
	return p[k:]
}

// flushFull writes the hex gathered once it is a chunk.
func (h *dataWriter) flushFull() error {
	if len(h.buf) < writeChunk {
		return nil
	}
	return h.flush()
}

// flush writes the hex gathered so far.
func (h *dataWriter) flush() error {
	err := core.WriteAll(h.w, h.buf)
	h.buf = h.buf[:0]
	return err
}
