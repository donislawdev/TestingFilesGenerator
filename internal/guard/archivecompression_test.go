package guard

import (
	stdzip "archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// A compressed archive still comes out the size that was ordered.
//
// This is the promise compression is most likely to break, and it breaks in a
// direction nothing else would notice. Every archive written before this was
// stored, so its length followed from the declared lengths of its parts and the
// plan could add them up. Compressed entries have a length nobody can predict -
// the project measured that on TIFF, where deflate moved the length by 48 B at
// 32x32 across seeds while uncompressed was flat - so the padding has to absorb
// whatever the compressor decides, at the moment it decides it.
//
// Asked at every level and across sizes, because the fault would sit in a band
// rather than at a point: a size where the space compression frees is larger
// than the padding can give back.
func TestACompressedArchiveStillHitsTheSizeToTheByte(t *testing.T) {
	levels := []string{archive.CompressFast, archive.CompressDefault, archive.CompressBest}
	checked := 0
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		for _, level := range levels {
			for _, size := range []int64{64 << 10, 256 << 10, 1 << 20} {
				plan, err := d.Generator.Plan(format.Request{
					Bytes: size, Seed: 7741, Label: true,
					Properties: map[string]string{archive.Compression: level},
				})
				if err != nil {
					t.Errorf("%s at %d B with compression %s was refused: %v", d.ID, size, level, err)
					continue
				}
				if plan.Bytes != size {
					t.Errorf("%s with compression %s planned %d B for an order of %d B",
						d.ID, level, plan.Bytes, size)
				}
				var buf bytes.Buffer
				if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
					t.Errorf("%s at %d B with compression %s could not be written: %v", d.ID, size, level, err)
					continue
				}
				if int64(buf.Len()) != size {
					t.Errorf("%s with compression %s wrote %d B where %d B was ordered - "+
						"the padding did not absorb what the compressor freed",
						d.ID, level, buf.Len(), size)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no compressed archive was measured, so this proved nothing")
	}
	t.Logf("%d compressed archives hit their ordered size", checked)
}

// And the setting has to actually squeeze something.
//
// Written because this project has just had the other kind of defect: a
// setting that reached the screen, reached the engine, and changed nothing a
// reader could see. A compression axis that quietly stored everything would
// pass the size guard above perfectly - the archive would still be the right
// length, because the padding would simply be larger.
//
// So this asks the archive itself rather than the plan: zip has to report an
// entry as deflated and smaller than it started, and a tar.gz has to inflate
// to more than the file holds.
func TestAskingForCompressionActuallyCompresses(t *testing.T) {
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		build := func(level string) []byte {
			t.Helper()
			plan, err := d.Generator.Plan(format.Request{
				Bytes: 1 << 20, Seed: 7741, Label: true,
				Properties: map[string]string{
					archive.Compression: level,
					archive.Entries:     "4",
					archive.EntrySize:   "32kb",
				},
			})
			if err != nil {
				t.Fatalf("%s with compression %s was refused: %v", d.ID, level, err)
			}
			var buf bytes.Buffer
			if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
				t.Fatalf("%s with compression %s could not be written: %v", d.ID, level, err)
			}
			return buf.Bytes()
		}

		squeezed := build(archive.CompressBest)

		switch d.ID {
		case "zip":
			r, err := stdzip.NewReader(bytes.NewReader(squeezed), int64(len(squeezed)))
			if err != nil {
				t.Fatalf("the compressed zip does not open: %v", err)
			}
			deflated := 0
			for _, f := range r.File {
				if f.Method == stdzip.Deflate && f.CompressedSize64 < f.UncompressedSize64 {
					deflated++
				}
			}
			if deflated == 0 {
				t.Errorf("no entry in the zip is deflated and smaller than it started, so asking for "+
					"%s stored everything and only grew the padding", archive.CompressBest)
			}
		case "targz":
			zr, err := gzip.NewReader(bytes.NewReader(squeezed))
			if err != nil {
				t.Fatalf("the compressed tar.gz does not open: %v", err)
			}
			inflated, err := io.Copy(io.Discard, zr)
			if err != nil {
				t.Fatalf("the compressed tar.gz does not inflate: %v", err)
			}
			if inflated <= int64(len(squeezed)) {
				t.Errorf("the tar inside is %d B and the file is %d B, so nothing was squeezed",
					inflated, len(squeezed))
			}
		default:
			t.Errorf("%s is a container and this guard has no way to tell whether it compressed, "+
				"so it would pass without looking", d.ID)
		}
	}
}

// Compression and a size that comes from the contents cannot both be had.
//
// The refusal is the honest answer rather than a limitation to apologise for.
// If the size comes from the contents, then the archive's length is whatever
// they compress to - and that is knowable only by compressing them, which is
// what the guard on planning forbids. Measured: our content compresses at about
// 50 MB/s, and that guard plans three gigabytes in milliseconds.
//
// Both halves have to be named. From "compression is not allowed" a reader
// cannot tell whether to drop the compression or give the archive a size.
func TestCompressionWithASizeFromTheContentsIsRefusedNamingBoth(t *testing.T) {
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		_, err := d.Generator.Plan(format.Request{
			Seed: 7741, SizeFromContents: true,
			Contains:   []format.Content{{Format: "txt", Count: 2, Bytes: 4096}},
			Properties: map[string]string{archive.Compression: archive.CompressBest},
		})
		if err == nil {
			t.Errorf("%s accepted compression with a size from the contents, so it planned a length "+
				"it could only have got by compressing", d.ID)
			continue
		}
		assertNamesBothHalves(t, d.ID, err, archive.Compression, "size")
	}
}

// And compression with a password cannot be had either, for a different reason.
//
// A locked entry goes through CreateRaw, which writes the compressed length
// into the header BEFORE the data. A compressed entry does not know its length
// until it has been compressed, so the entry would have to be held in memory
// whole - and not holding a file in memory is the other promise this project
// keeps. Refusing is the honest answer to a combination this design cannot
// stream.
func TestCompressionWithAPasswordIsRefusedNamingBoth(t *testing.T) {
	asked := 0
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		if !offers(d, archive.Password) {
			continue
		}
		asked++
		_, err := d.Generator.Plan(format.Request{
			Bytes: 1 << 20, Seed: 7741,
			Properties: map[string]string{
				archive.Compression: archive.CompressBest,
				archive.Password:    "hunter2",
				archive.Encryption:  archive.AES256,
			},
		})
		if err == nil {
			t.Errorf("%s accepted compression together with a password, which cannot be written "+
				"without holding an entry in memory", d.ID)
			continue
		}
		assertNamesBothHalves(t, d.ID, err, archive.Compression, archive.Password)
	}
	if asked == 0 {
		t.Fatal("no container offers a password, so this guard asked nothing")
	}
}

// An archive nobody asked to compress is stored, and that is what keeps every
// hash where it is.
//
// The default cannot be anything else. Every archive this tool has written is
// stored, so a compressing default would move the bytes of all of them without
// a single recipe changing - untouchable rule 3.
func TestAnArchiveNobodyAskedToCompressIsStored(t *testing.T) {
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		plain, err := d.Generator.Plan(format.Request{Bytes: 1 << 20, Seed: 7741, Label: true})
		if err != nil {
			t.Fatalf("%s at 1 MB was refused: %v", d.ID, err)
		}
		stated, err := d.Generator.Plan(format.Request{
			Bytes: 1 << 20, Seed: 7741, Label: true,
			Properties: map[string]string{archive.Compression: archive.CompressNone},
		})
		if err != nil {
			t.Fatalf("%s with compression none was refused: %v", d.ID, err)
		}

		var a, b bytes.Buffer
		if err := d.Generator.Write(context.Background(), &a, plain); err != nil {
			t.Fatalf("%s: %v", d.ID, err)
		}
		if err := d.Generator.Write(context.Background(), &b, stated); err != nil {
			t.Fatalf("%s: %v", d.ID, err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("%s: saying compression none gives different bytes from saying nothing, so the "+
				"default is not what it was", d.ID)
		}
	}
}

// assertNamesBothHalves holds a refusal to naming both settings it is about.
func assertNamesBothHalves(t *testing.T, id string, err error, halves ...string) {
	t.Helper()
	var refusal *format.PropertyValueError
	if !errors.As(err, &refusal) {
		t.Errorf("%s: the refusal is %T, so it does not carry the four things a refusal owes a reader: %v",
			id, err, err)
		return
	}
	said := refusal.Reason + " " + refusal.Remedy
	for _, half := range halves {
		if !strings.Contains(said, half) {
			t.Errorf("%s: the refusal never names %q, so a reader cannot tell which half to change:\n  %s",
				id, half, said)
		}
	}
}

// offers reports whether a format declares the named setting.
func offers(d format.Descriptor, name string) bool {
	for _, p := range d.Properties {
		if p.Name == name {
			return true
		}
	}
	return false
}
