package guard

import (
	"archive/tar"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A tar.gz laid out for a cut comes to every size asked for and reads as the
// archive it is, whatever frame its end of archive marker takes.
//
// The last bytes behind its files land in the deflate frame around the marker
// (internal/format/targz/cut.go): stored blocks for steps of five bytes, and
// one to three empty blocks of fixed codes in front of a block of the marker
// for the rest. Which frame an archive takes follows its size, and no other
// guard asks for enough sizes in a row to meet every one - a frame that adds a
// byte too few passed the share and the map guards (2026-11-13). So this
// writes consecutive sizes, cut half way, holds each to its size and to
// archive/tar, and asserts that it met the marker in plain stored blocks and
// behind one, two, three, and one and then two empty blocks.
//
// Canary: the reader of the frame counts the empty blocks of a stream built by
// hand, which compress/flate inflates to the bytes it holds.
func TestATarGzLaidOutForACutComesToEverySizeAndReads(t *testing.T) {
	desc, err := format.Get("targz")
	if err != nil {
		t.Fatal(err)
	}
	met := map[string]bool{}
	for size := int64(64 << 10); size < 64<<10+64; size++ {
		data := writeWhole(t, desc, size)
		if int64(len(data)) != size {
			t.Errorf("%d B: laid out for a cut, the archive came to %d B", size, len(data))
			continue
		}
		if err := readsEveryEntry(data); err != nil {
			t.Errorf("%d B: %v", size, err)
			continue
		}
		runs, err := emptyBlockRuns(data[gzipStreamStart(data):])
		if err != nil {
			t.Errorf("%d B: %v", size, err)
			continue
		}
		met[fmt.Sprint(runs)] = true
	}
	for _, frame := range []string{"[]", "[1]", "[2]", "[3]", "[1 2]"} {
		if !met[frame] {
			t.Errorf("no size met the marker framed behind empty blocks %s - met %v", frame, met)
		}
	}

	// Canary: two bytes stored, one empty block and a stored byte, two empty
	// blocks and the last stored byte.
	stream := []byte{0x00, 2, 0, 0xfd, 0xff, 'a', 'b', 0x02, 0x00, 1, 0, 0xfe, 0xff, 'c', 0x02, 0x08, 0x10, 1, 0, 0xfe, 0xff, 'd'}
	inflated, err := io.ReadAll(flate.NewReader(bytes.NewReader(stream)))
	if err != nil || string(inflated) != "abcd" {
		t.Errorf("canary: the stream built by hand inflates to %q, %v", inflated, err)
	}
	if runs, err := emptyBlockRuns(stream); err != nil || !slices.Equal(runs, []int{1, 2}) {
		t.Errorf("canary: the stream built by hand read as runs %v, %v", runs, err)
	}
}

// writeWhole writes a tar.gz of size bytes laid out for a cut half way, whole.
func writeWhole(t *testing.T, desc format.Descriptor, size int64) []byte {
	t.Helper()
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: size / 2})
	if err != nil {
		t.Fatalf("%d B: planning: %v", size, err)
	}
	var out bytes.Buffer
	if err := desc.Generator.Write(t.Context(), &out, p); err != nil {
		t.Fatalf("%d B: writing: %v", size, err)
	}
	return out.Bytes()
}

// readsEveryEntry reads every entry of a tar.gz to its end, through
// compress/gzip, which checks the trailer, and archive/tar.
func readsEveryEntry(data []byte) error {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return fmt.Errorf("%s: %w", h.Name, err)
		}
		files++
	}
	if _, err := io.Copy(io.Discard, zr); err != nil {
		return fmt.Errorf("after the tar: %w", err)
	}
	if files < 2 {
		return fmt.Errorf("%d entries, where a file and padding beside it were due", files)
	}
	return nil
}

// gzipStreamStart is where the deflate stream of a gzip member starts: after
// the fixed header, the extra field and the comment (RFC 1952 section 2.3) -
// the only fields this tool writes.
func gzipStreamStart(data []byte) int {
	at := 10
	if data[3]&4 != 0 {
		at += 2 + int(binary.LittleEndian.Uint16(data[10:12]))
	}
	if data[3]&16 != 0 {
		at += bytes.IndexByte(data[at:], 0) + 1
	}
	return at
}

// emptyBlockRuns reads a deflate stream of stored blocks and empty blocks of
// fixed codes, and says how many empty blocks stand together in front of each
// stored block that has any. Any other block is an error.
func emptyBlockRuns(s []byte) ([]int, error) {
	var runs []int
	run, bit := 0, 0
	for bit/8 < len(s) {
		final, kind := bitsAt(s, bit, 1), bitsAt(s, bit+1, 2)
		switch {
		case kind == 0:
			at := (bit + 3 + 7) / 8
			if at+4 > len(s) {
				return nil, fmt.Errorf("the stored block at bit %d runs off the end", bit)
			}
			if run > 0 {
				runs = append(runs, run)
				run = 0
			}
			bit = (at + 4 + int(binary.LittleEndian.Uint16(s[at:]))) * 8
		case kind == 1 && bitsAt(s, bit+3, 7) == 0:
			run++
			bit += 10
		default:
			return nil, fmt.Errorf("the block at bit %d is neither stored nor an empty block of fixed codes", bit)
		}
		if final == 1 {
			return runs, nil
		}
	}
	return nil, errors.New("the stream has no final block")
}

// bitsAt is n bits of s from bit at, the least significant first.
func bitsAt(s []byte, at, n int) int {
	v := 0
	for i := range n {
		b := at + i
		if b/8 < len(s) && s[b/8]>>(b%8)&1 == 1 {
			v |= 1 << i
		}
	}
	return v
}
