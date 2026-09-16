package guard

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"io"
	"strings"
	"testing"
)

// pngSignature opens every PNG file.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

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
func readableText(body []byte) string {
	if !bytes.HasPrefix(body, pngSignature) {
		return string(body)
	}
	var out []string
	rest := body[len(pngSignature):]
	for len(rest) >= 12 {
		length := binary.BigEndian.Uint32(rest[:4])
		kind := string(rest[4:8])
		if uint64(len(rest)) < 12+uint64(length) {
			break
		}
		data := rest[8 : 8+length]
		switch kind {
		case "tEXt":
			out = append(out, string(bytes.ReplaceAll(data, []byte{0}, []byte{' '})))
		case "zTXt":
			// keyword, a nought, a method byte, then compressed text.
			if i := bytes.IndexByte(data, 0); i >= 0 && i+2 <= len(data) {
				out = append(out, string(data[:i]), inflated(data[i+2:]))
			}
		case "iTXt":
			// keyword, nought, compression flag, method, language, nought,
			// translated keyword, nought, text.
			if i := bytes.IndexByte(data, 0); i >= 0 && i+3 <= len(data) {
				compressed := data[i+1] == 1
				tail := data[i+3:]
				parts := bytes.SplitN(tail, []byte{0}, 3)
				if len(parts) == 3 {
					text := string(parts[2])
					if compressed {
						text = inflated(parts[2])
					}
					out = append(out, string(data[:i]), string(parts[0]), string(parts[1]), text)
				}
			}
		}
		rest = rest[12+length:]
	}
	return strings.Join(out, "\n")
}

// inflated is zlib data as text, or nothing when it will not inflate.
func inflated(data []byte) string {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil {
		return ""
	}
	return string(b)
}

// pngWithText is the smallest PNG that carries one text chunk - built here so
// the canary does not depend on any file in the tree.
func pngWithText(kind string, data []byte) []byte {
	chunk := func(kind string, data []byte) []byte {
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
	var out bytes.Buffer
	out.Write(pngSignature)
	// A 1x1 greyscale header, 8 bits, no interlace.
	out.Write(chunk("IHDR", []byte{0, 0, 0, 1, 0, 0, 0, 1, 8, 0, 0, 0, 0}))
	out.Write(chunk(kind, data))
	out.Write(chunk("IEND", nil))
	return out.Bytes()
}

// The exception above has a red path, and this is it: an address written into
// a PNG's text chunk is still found. Without this the narrowing to text
// chunks could quietly become "pictures are never read".
func TestAnAddressInAPictureTextChunkIsStillFound(t *testing.T) {
	address := strings.Join([]string{"someone", "example.org"}, "@")
	for name, png := range map[string][]byte{
		"tEXt": pngWithText("tEXt", append([]byte("Author\x00exported by "), address...)),
		"iTXt": pngWithText("iTXt", append([]byte("Comment\x00\x00\x00en\x00\x00"), address...)),
	} {
		faults := privateFaults(readableText(png))
		if len(faults) == 0 {
			t.Errorf("an address in a %s chunk was not found, so a picture can carry one out unread", name)
		}
	}
	// And the same bytes with no text chunk say nothing - which is the whole
	// reason the chunks are read rather than the file.
	plain := pngWithText("tIME", []byte{7, 234, 9, 16, 12, 0, 0})
	if faults := privateFaults(readableText(plain)); len(faults) != 0 {
		t.Errorf("a picture with no text chunk reports %v", faults)
	}
}
