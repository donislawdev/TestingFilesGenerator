package guard

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
	"testing"
)

// pngSignature opens every PNG file.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// pictureTextBudget is the room this guard reads from one picture: every text
// chunk of it together, inflated. A picture carrying more is refused rather
// than read in part, because the part not read is exactly where a word would
// leave unseen.
//
// One room for the picture rather than one per chunk, because a limit per
// chunk is a silence per chunk. The number is a ceiling on an accident, not on
// a file: measured 2026-09-16, no PNG in the tree carries a text chunk at all.
const pictureTextBudget = 1 << 20

// errPictureTextBudget is the refusal for a picture with more text than that.
var errPictureTextBudget = fmt.Errorf("carries more text than the %d bytes this guard reads from one picture", pictureTextBudget)

// readableText is what a file can carry that a person could have typed into
// it. For text that is the whole file. For a PNG it is the text chunks alone -
// tEXt, zTXt and iTXt - because the rest is compressed pixels, and compressed
// pixels are bytes in every arrangement, including the seven that spell an
// e-mail address.
//
// Measured on 2026-09-16: regenerating the stored screen of the format menu
// moved its pixels by twelve columns, and the private content guard read
// seven of the new bytes - a digit, the at sign, dots and two letters - as an
// address and refused the picture. A guard that reads pixels as
// prose is red at random - whenever a picture happens to compress into a
// pattern - and a guard that is red at random is a guard somebody switches
// off. The text chunks are where a PNG carries words, so that is what is read.
//
// Not "skip binary files": a PNG CAN carry a path or an address, in exactly
// these chunks, and an exporter that stamps the author into iTXt is not a
// hypothetical. The canary below proves the address is still found there.
//
// A picture is read whole or refused. Every way this reader could read less
// than the picture carries - a chunk longer than the file, a compressed text
// that will not inflate, more text than the budget, bytes after IEND, a text
// chunk without its separators - is an error rather than a skip, because a
// guard that read part of a file and said nothing would be green on the part
// it had not read. None of these states occurs in a PNG we ship (measured
// 2026-09-16 on every tracked one), and the encoder that writes ours is the
// standard library's, so a refusal here names a broken file, not a wall.
func readableText(body []byte) (string, error) {
	if !bytes.HasPrefix(body, pngSignature) {
		return string(body), nil
	}
	text := pictureText{room: pictureTextBudget}
	rest := body[len(pngSignature):]
	for len(rest) >= 12 {
		length := binary.BigEndian.Uint32(rest[:4])
		kind := string(rest[4:8])
		if uint64(len(rest)) < 12+uint64(length) {
			return "", fmt.Errorf("has a %s chunk of %d bytes that runs past the end of the file", kind, length)
		}
		data := rest[8 : 8+length]
		rest = rest[12+length:]
		switch kind {
		case "tEXt", "zTXt", "iTXt":
			pieces, err := textChunk(kind, data, text.room)
			if err == nil {
				err = text.take(pieces)
			}
			if err != nil {
				return "", err
			}
		case "IEND":
			if len(rest) > 0 {
				return "", fmt.Errorf("has %d bytes after IEND, where a picture ends", len(rest))
			}
			return strings.Join(text.out, "\n"), nil
		}
	}
	return "", errors.New("ends without IEND, so it is not a whole picture")
}

// pictureText is what a picture's text chunks say, within the room left.
type pictureText struct {
	out  []string
	room int
}

// take keeps the pieces of one chunk, or refuses the picture once they would
// not fit - the one place the budget is asked, so a chunk and a picture are
// held to the same number.
func (p *pictureText) take(pieces []string) error {
	for _, piece := range pieces {
		if len(piece) > p.room {
			return errPictureTextBudget
		}
		p.room -= len(piece)
		p.out = append(p.out, piece)
	}
	return nil
}

// textChunk is the pieces of a tEXt, zTXt or iTXt chunk - its keyword, its
// language and translated keyword where it has them, and its text - or a
// refusal for a chunk without the separators its kind requires.
func textChunk(kind string, data []byte, room int) ([]string, error) {
	i := bytes.IndexByte(data, 0)
	switch {
	case kind == "tEXt":
		return []string{string(bytes.ReplaceAll(data, []byte{0}, []byte{' '}))}, nil
	case i < 0:
		return nil, fmt.Errorf("has a %s chunk without the nought after its keyword", kind)
	case kind == "zTXt":
		// keyword, a nought, a method byte, then compressed text.
		if i+2 > len(data) {
			return nil, errors.New("has a zTXt chunk that ends before its text")
		}
		return withText([]string{string(data[:i])}, data[i+2:], true, room)
	}
	// iTXt: keyword, nought, compression flag, method, language, nought,
	// translated keyword, nought, text.
	if i+3 > len(data) {
		return nil, errors.New("has an iTXt chunk that ends before its text")
	}
	parts := bytes.SplitN(data[i+3:], []byte{0}, 3)
	if len(parts) < 3 {
		return nil, errors.New("has an iTXt chunk without the noughts around its language")
	}
	return withText([]string{string(data[:i]), string(parts[0]), string(parts[1])}, parts[2], data[i+1] == 1, room)
}

// withText is the pieces of a chunk with its text after them - inflated when
// it is compressed, into the room left once the pieces before it are counted,
// so that the room the inflating read is held to is the room the budget will
// ask about.
func withText(pieces []string, text []byte, compressed bool, room int) ([]string, error) {
	if !compressed {
		return append(pieces, string(text)), nil
	}
	for _, piece := range pieces {
		room -= len(piece)
	}
	inflatedText, err := inflated(text, room)
	return append(pieces, inflatedText), err
}

// inflated is zlib data as text, or a refusal for data that will not inflate.
//
// Both halves were "" until 2026-09-16, and each was a silence: a stream with
// a wrong checksum was read to its end and thrown away, and a stream longer
// than a megabyte was cut there with the rest unread. The read stops one byte
// past the room rather than at it, so a stream longer than the room comes back
// longer than the room and the budget refuses it, while one exactly the room
// long is read whole - and memory is bounded by the room either way.
func inflated(data []byte, room int) (string, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	var b []byte
	if err == nil {
		defer r.Close()
		b, err = io.ReadAll(io.LimitReader(r, int64(room)+1))
	}
	if err != nil {
		return "", fmt.Errorf("has compressed text that will not inflate: %w", err)
	}
	return string(b), nil
}

// rawChunk is one chunk of a PNG built for a canary.
type rawChunk struct {
	kind string
	data []byte
}

// pngWith is the smallest PNG carrying the given chunks between its header
// and its IEND - built here so no canary depends on any file in the tree.
func pngWith(chunks ...rawChunk) []byte {
	var out bytes.Buffer
	out.Write(pngSignature)
	// A 1x1 greyscale header, 8 bits, no interlace.
	out.Write(chunkBytes("IHDR", []byte{0, 0, 0, 1, 0, 0, 0, 1, 8, 0, 0, 0, 0}))
	for _, c := range chunks {
		out.Write(chunkBytes(c.kind, c.data))
	}
	out.Write(chunkBytes("IEND", nil))
	return out.Bytes()
}

// pngWithText is a PNG carrying one chunk.
func pngWithText(kind string, data []byte) []byte {
	return pngWith(rawChunk{kind, data})
}

// chunkBytes is one chunk as a PNG writes it: length, kind, data, CRC.
func chunkBytes(kind string, data []byte) []byte {
	var b bytes.Buffer
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(data)))
	b.Write(n[:])
	b.WriteString(kind)
	b.Write(data)
	crc := crc32.NewIEEE()
	crc.Write([]byte(kind))
	crc.Write(data)
	binary.BigEndian.PutUint32(n[:], crc.Sum32())
	b.Write(n[:])
	return b.Bytes()
}

// zTXtChunk is a zTXt chunk's data: the keyword, its nought, the method and
// the text deflated.
func zTXtChunk(keyword string, text []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	w.Write(text)
	w.Close()
	return append([]byte(keyword+"\x00\x00"), b.Bytes()...)
}

// The exception above has a red path, and this is it: an address written into
// a PNG's text chunk is still found - plain, and behind zlib. Without this the
// narrowing to text chunks could quietly become "pictures are never read".
func TestAnAddressInAPictureTextChunkIsStillFound(t *testing.T) {
	address := strings.Join([]string{"someone", "example.org"}, "@")
	for name, png := range map[string][]byte{
		"tEXt": pngWithText("tEXt", append([]byte("Author\x00exported by "), address...)),
		"zTXt": pngWithText("zTXt", zTXtChunk("Author", []byte("exported by "+address))),
		"iTXt": pngWithText("iTXt", append([]byte("Comment\x00\x00\x00en\x00\x00"), address...)),
	} {
		text, err := readableText(png)
		if err != nil {
			t.Fatalf("a picture with an address in a %s chunk is refused (%v) rather than read", name, err)
		}
		if faults := privateFaults(text); len(faults) == 0 {
			t.Errorf("an address in a %s chunk was not found, so a picture can carry one out unread", name)
		}
	}
	// And the same bytes with no text chunk say nothing - which is the whole
	// reason the chunks are read rather than the file.
	plain := pngWithText("tIME", []byte{7, 234, 9, 16, 12, 0, 0})
	text, err := readableText(plain)
	if err != nil {
		t.Fatalf("a picture with no text chunk is refused: %v", err)
	}
	if faults := privateFaults(text); len(faults) != 0 {
		t.Errorf("a picture with no text chunk reports %v", faults)
	}
}

// A picture is read whole or refused, and this is the refusal's red path: each
// way the reader could read less than the picture carries. The first pair is
// the budget with one byte deciding - the picture exactly the room long is
// read, because a refusal that refused everything would pass the rest alone.
func TestAPictureThisGuardCannotReadWholeIsRefused(t *testing.T) {
	keyword := "Comment"
	fill := func(n int) []byte { return bytes.Repeat([]byte{'a'}, n) }
	// The room, less the keyword that is read with the text.
	room := pictureTextBudget - len(keyword)

	whole := pngWithText("zTXt", zTXtChunk(keyword, fill(room)))
	text, err := readableText(whole)
	if err != nil || !strings.HasSuffix(text, "aaa") || len(text) != pictureTextBudget+1 {
		t.Errorf("a picture with exactly the room of text is refused (%v) or read short (%d bytes)"+
			" - the budget is a ceiling, not a wall", err, len(text))
	}

	unfinished := zTXtChunk(keyword, []byte("exported by somebody"))
	unfinished[len(unfinished)-1] ^= 0xff // the checksum's last byte.
	cut := pngWithText("tEXt", []byte("Author\x00exported by somebody"))
	cut = cut[:len(cut)-20] // into the text chunk, past the IEND.
	noEnd := pngWithText("tEXt", []byte("Author\x00exported by somebody"))
	noEnd = noEnd[:len(noEnd)-12] // the IEND chunk exactly.

	for _, tc := range []struct {
		name, says string
		png        []byte
	}{
		{"a text chunk one byte past the room", "more text than",
			pngWithText("zTXt", zTXtChunk(keyword, fill(room+1)))},
		{"two text chunks that fit alone and not together", "more text than",
			pngWith(rawChunk{"zTXt", zTXtChunk(keyword, fill(room/2))}, rawChunk{"zTXt", zTXtChunk(keyword, fill(room/2))})},
		{"compressed text that will not inflate", "will not inflate", pngWithText("zTXt", unfinished)},
		{"a chunk that runs past the end of the file", "past the end", cut},
		{"a picture that ends before its IEND", "without IEND", noEnd},
		{"bytes after IEND", "after IEND", append(pngWithText("tIME", []byte{7, 234, 9, 16, 12, 0, 0}), "and words"...)},
		{"a text chunk without the nought after its keyword", "nought", pngWithText("zTXt", []byte(keyword))},
	} {
		_, err := readableText(tc.png)
		if err == nil || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%s is read rather than refused (%v), so what the reader did not reach could leave unread", tc.name, err)
		}
	}
}
