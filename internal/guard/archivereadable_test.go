package guard

import (
	stdzip "archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// Every archive this tool writes opens in the standard library of the language
// it is written in.
//
// This guard exists because that was not true for a month and nothing said so.
// TAR.GZ padded through the gzip header comment, up to four kilobytes of it, and
// Go's compress/gzip reads that field into a fixed buffer: it takes 511 bytes
// and refuses 512. Measured on 2026-09-01 across 11 260 reachable sizes, 4 134
// of them produced an archive no Go program could open. O163.
//
// Three things made it invisible, and each one is a lesson this guard is the
// answer to.
//
// The reference tools all accepted the files. 7-Zip, GNU tar, bsdtar, Python
// and node take a comment of ten megabytes without a word, so the oracle was
// green on every one of them. An oracle says "would a real reader take this",
// and the answer was yes - for every reader anybody had thought to ask.
//
// The padding channel had been measured, carefully, and written up. The
// measurement asked five readers and Go was not among them, which is the same
// shape of miss as trusting a document instead of measuring: the question was
// right and the sample was short one entry.
//
// And the failure was silent in the worst way. compress/gzip says
// "gzip: invalid header", which reads like a corrupt file rather than like a
// field this reader will not take, so somebody meeting it would suspect the
// fixture rather than the tool that made it. Testers write tools in Go.
//
// Asked of every container and every size across the band where the padding
// channel changes shape, because the fault was in a band rather than at a
// point - the sizes just above where the header can no longer hold it all.
func TestEveryArchiveThisToolWritesCanBeReadByTheStandardLibrary(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		if !d.Container {
			continue
		}
		read, ok := standardReaderFor(d.ID)
		if !ok {
			t.Errorf("%s is a container and nothing here knows how to open it with the standard "+
				"library, so this guard skips it silently - which is how the fault it was "+
				"written for survived", d.ID)
			continue
		}
		// From the floor upwards, far enough past it that the header channel
		// fills and the run has to reach for a filler entry.
		for size := d.MinBytes; size <= d.MinBytes+20000; size += 13 {
			plan, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
			if err != nil {
				// Below the floor, or one of the sizes this format says it
				// cannot reach. Neither is the subject.
				continue
			}
			var buf bytes.Buffer
			if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
				t.Fatalf("%s at %d B could not be written: %v", d.ID, size, err)
			}
			checked++
			if err := read(buf.Bytes()); err != nil {
				t.Fatalf("%s at %d B cannot be opened by the standard library: %v\n"+
					"  every reference tool would take this file, so nothing else here would have said so",
					d.ID, size, err)
			}
		}
	}

	if checked == 0 {
		t.Fatal("no archive was read back, so this proved nothing")
	}
	t.Logf("%d archives opened with the standard library", checked)
}

// standardReaderFor is how the standard library opens one container, or false
// when this guard has no way to.
//
// A table rather than a switch inside the loop, so a container arriving without
// an entry is reported rather than skipped. A guard that quietly covers less
// than it says is the shape of the fault this file exists for.
func standardReaderFor(id string) (func([]byte) error, bool) {
	switch id {
	case "targz":
		return func(b []byte) error {
			r, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return err
			}
			return r.Close()
		}, true
	case "zip":
		return func(b []byte) error {
			_, err := stdzip.NewReader(bytes.NewReader(b), int64(len(b)))
			return err
		}, true
	}
	return nil, false
}
