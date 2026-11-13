package guard

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A PDF laid out for a cut comes to every size asked for, every offset in it
// lands, and the cut keeps the share of its objects.
//
// The padding in front of the objects moves every one of them, so the offset
// startxref carries grows with it and gains a digit at a power of ten, which
// the padding behind gives up (internal/format/pdf/cut.go). Each side is
// nothing or a comment of two bytes or more, so where the padding behind would
// come to one byte the front takes a byte more, and where that byte gains the
// digit the front takes a byte less instead. No other guard asks for the sizes
// where that happens. So this writes consecutive sizes cut at their last byte
// across the size where startxref gains its fifth digit, and at 1% and 50%
// where the padding behind runs out, holds each to its size, its offsets and
// the share, and asserts that it met the layouts that follow: all of the
// padding in front with four digits and with five, a few bytes left behind
// where the digit is gained, the share to the byte, and a byte more in front.
//
// A document of more than ten gigabytes is planned and not written: its
// objects stop where the cross reference table's ten digits do.
//
// Canary: the reader of offsets refuses a document whose table sends one
// object a byte off.
func TestAPDFLaidOutForACutComesToEverySizeAndItsOffsetsLand(t *testing.T) {
	desc, err := format.Get("pdf")
	if err != nil {
		t.Fatal(err)
	}
	met := map[string]bool{}
	sweep := func(from, to int64, cutAt func(size int64) int64) {
		for size := from; size < to; size++ {
			var below *format.BelowMinimumError
			if _, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true}); errors.As(err, &below) {
				// The one size in the gap just above the bare document.
				continue
			}
			data, regions := writePDF(t, desc, size, cutAt(size))
			layout, err := pdfLayoutOf(data, regions, size, cutAt(size))
			if err != nil {
				t.Errorf("%d B cut at %d: %v", size, cutAt(size), err)
				continue
			}
			met[layout] = true
		}
	}
	sweep(9_900, 10_400, func(size int64) int64 { return size - 1 })
	sweep(22_000, 24_600, func(size int64) int64 { return size - (size+99)/100 })
	smallest := desc.SmallestAccepted(format.Request{Label: true})
	sweep(smallest, smallest+600, func(size int64) int64 { return size / 2 })
	for _, layout := range []string{"all in front, startxref of 4 digits", "all in front, startxref of 5 digits",
		"a few bytes behind where startxref gains a digit", "the share", "a byte more in front"} {
		if !met[layout] {
			t.Errorf("no size met the layout %q - met %v", layout, met)
		}
	}

	pdfTooLargeForTheTable(t, desc)

	// Canary: one object a byte off.
	data, _ := writePDF(t, desc, 64<<10, 32<<10)
	at := bytes.Index(data, []byte(" 00000 n \n")) - 1
	broken := bytes.Clone(data)
	broken[at]++
	if err := pdfOffsetsLand(broken); err == nil {
		t.Errorf("canary: a table that sends an object a byte off passed")
	}
}

// writePDF writes a PDF of size bytes laid out for a cut after cutAt, whole,
// and the regions it marked.
func writePDF(t *testing.T, desc format.Descriptor, size, cutAt int64) ([]byte, []format.Region) {
	t.Helper()
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: cutAt})
	if err != nil {
		t.Fatalf("%d B: planning: %v", size, err)
	}
	var out bytes.Buffer
	rec := &format.MapRecorder{W: &out}
	if err := desc.Generator.Write(t.Context(), rec, p); err != nil {
		t.Fatalf("%d B: writing: %v", size, err)
	}
	return out.Bytes(), rec.Map(size, true)
}

// pdfLayoutOf holds a PDF laid out for a cut to its size, its offsets and the
// share the cut keeps of its objects, and names the layout it took.
func pdfLayoutOf(data []byte, regions []format.Region, size, cutAt int64) (string, error) {
	if int64(len(data)) != size {
		return "", fmt.Errorf("came to %d B", len(data))
	}
	if err := pdfOffsetsLand(data); err != nil {
		return "", err
	}
	roles := []format.Role{format.RoleStructure, format.RolePadding, format.RoleContent, format.RoleEnd, format.RolePadding, format.RoleEnd}
	if len(regions) != len(roles) {
		return "", fmt.Errorf("marked %d regions", len(regions))
	}
	for i, r := range regions {
		if r.Role != roles[i] {
			return "", fmt.Errorf("region %d is %s where %s was due", i, r.Role, roles[i])
		}
	}
	head, front, content, back := regions[0].Length, regions[1].Length, regions[2].Length, regions[4].Length
	if front == 1 || back == 1 {
		return "", fmt.Errorf("a comment of one byte: %d in front, %d behind", front, back)
	}
	want := format.PaddingInFront(head, content, cutAt, size)
	kept, share := cutAt-head-front, content*cutAt/size
	switch {
	case front == 0:
		return "nothing in front", nil
	case front == want && kept == share:
		return "the share", nil
	case front == want+1 && kept == share-1:
		return "a byte more in front", nil
	case front == want-1 && kept == share+1:
		return "a byte less in front", nil
	case front < want && back == 0:
		return fmt.Sprintf("all in front, startxref of %d digits", len(strconv.FormatInt(regions[3].At, 10))), nil
	case front < want && back <= 3:
		return "a few bytes behind where startxref gains a digit", nil
	}
	return "", fmt.Errorf("%d B in front where the share wants %d, keeping %d B of the objects where the share is %d", front, want, kept, share)
}

// pdfOffsetsLand reads a PDF's cross reference table from the offset
// startxref gives and requires every object it lists to start where it says.
func pdfOffsetsLand(data []byte) error {
	if !bytes.HasSuffix(data, []byte("\n%%EOF\n")) {
		return errors.New("the file does not end with the end of file marker")
	}
	sx := bytes.LastIndex(data, []byte("startxref\n"))
	if sx < 0 {
		return errors.New("no startxref")
	}
	line := data[sx+len("startxref\n") : len(data)-len("%%EOF\n")-1]
	xref, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil || xref < 0 || xref >= int64(len(data)) || !bytes.HasPrefix(data[xref:], []byte("xref\n0 ")) {
		return fmt.Errorf("startxref gives %q, where no cross reference table starts", line)
	}
	table := data[xref+int64(len("xref\n0 ")):]
	end := bytes.IndexByte(table, '\n')
	count, err := strconv.Atoi(string(table[:max(end, 0)]))
	if err != nil {
		return fmt.Errorf("the table counts %q", table[:max(end, 0)])
	}
	entries := table[end+1+20:]
	for i := 1; i < count; i++ {
		if len(entries) < 20 {
			return fmt.Errorf("the table ends before object %d", i)
		}
		at, err := strconv.ParseInt(string(entries[:10]), 10, 64)
		obj := fmt.Sprintf("%d 0 obj\n", i)
		if err != nil || at < 0 || at >= int64(len(data)) || !bytes.HasPrefix(data[at:], []byte(obj)) {
			return fmt.Errorf("the table sends object %d to %q, where it does not start", i, entries[:10])
		}
		entries = entries[20:]
	}
	return nil
}

// pdfTooLargeForTheTable plans a PDF of 20 GiB cut at 90% and holds its
// objects to where an offset of ten digits reaches (ISO 32000-1 section
// 7.5.4): the padding in front stops there, short of the share. It writes
// nothing past the marks, which come before the first byte.
func pdfTooLargeForTheTable(t *testing.T, desc format.Descriptor) {
	t.Helper()
	const size, largestOffset = 20 << 30, 9_999_999_999
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: size / 10 * 9})
	if err != nil {
		t.Fatalf("planning %d B: %v", int64(size), err)
	}
	rec := &format.MapRecorder{W: failingWriter{}}
	if err := desc.Generator.Write(t.Context(), rec, p); err == nil {
		t.Fatalf("writing %d B went through a writer that refuses everything", int64(size))
	}
	regions := rec.Map(size, true)
	if len(regions) != 6 {
		t.Fatalf("%d B: marked %d regions", int64(size), len(regions))
	}
	front, objects := regions[1].Length, regions[2]
	if front == 0 || objects.At+objects.Length != largestOffset {
		t.Errorf("%d B cut at 90%%: %d B in front and the objects end at %d, where the table's ten digits stop them at %d",
			int64(size), front, objects.At+objects.Length, int64(largestOffset))
	}
}
