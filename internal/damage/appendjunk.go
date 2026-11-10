package damage

import (
	"encoding/binary"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// AppendJunk is the damage that writes bytes after the end of a file. A public
// name under untouchable rule 10. Its setting is bytes, the word zero-head
// already uses for how many bytes a damage touches.
const AppendJunk = "append-junk"

const (
	// smallestJunk is the fewest bytes worth appending, and it is a
	// measurement.
	//
	// Measured 2026-11-09 across 31 formats: one random byte is noticed by 21,
	// sixteen by 25, and from 64 up the set stays the same at every size tried
	// up to 1 MiB (docs/DAMAGE-ETAP-2-2026-11-09.md section 2.1). What the
	// bytes ARE matters as much: zeros are taken for padding by PDF, AVIF and
	// JPEG XL, so the bytes are random.
	smallestJunk = 64

	// largestJunk is where the setting stops. Chosen rather than measured: a
	// range with no top draws a field anything can be typed into, and 64 KiB
	// is far past any length a format reads its end markers from.
	largestJunk = 65536

	defaultJunk = smallestJunk

	// junkChunk is how much junk is made and written at a time, so 64 KiB of
	// it is never held at once. A multiple of eight, because the bytes come
	// from 64 bit draws and a chunk boundary inside one would change them.
	junkChunk = 8192
)

func init() {
	Register(Descriptor{
		ID: AppendJunk,
		Detail: "Writes random bytes after the end of the file and keeps exactly the size you asked for, " +
			"the way a file with something stuck to its end arrives.",
		Parameters: []format.Property{
			{
				Name: SettingBytes, Kind: format.PropertyInt,
				Min: smallestJunk, Max: largestJunk, Unit: "bytes",
				Default: strconv.Itoa(defaultJunk),
				Detail: "How many random bytes come after the end. " +
					"Below 64, some formats come out with damage no reader complains about.",
			},
		},
		Floor:    func(v Values) int64 { return int64(junkBytes(v)) },
		Length:   func(v Values, out int64) int64 { return out - int64(junkBytes(v)) },
		AddsTail: true,
		Policy:   appendPolicy,
		Open: func(v Values, f File, out io.Writer) (Stream, error) {
			return &junkTail{out: out, seed: f.Seed, n: junkBytes(v)}, nil
		},
	})
}

// appendPolicy is where bytes after the end of a file leave its fate to the
// reader.
//
// Measured on 2026-11-10 with every setting of every format, with the reason
// of each refusal read (docs/DAMAGE-ETAP-2-2026-11-09.md sections 12.2 and
// 12.3). Two kinds stand here. Bytes past the format's own end - after a
// PNG's IEND, a ZIP's last record, an RTF's closing brace - break a rule most
// readers do not enforce. Bytes the format allows - a TIFF or an icon is read
// through offsets, so bytes nothing points at are legal - break nothing.
// Everywhere else the bytes make something the file says about itself false:
// a length in its header, the encoding of its text, the boxes it is made of.
var appendPolicy = []PolicyCase{
	{Format: "png", Why: "A PNG ends at its IEND chunk and most viewers skip what follows it."},
	{Format: "gif", Why: "A GIF ends at its trailer and most viewers skip what follows it."},
	{Format: "jpg", Why: "A JPEG ends at its end of image marker, and cameras and editors often leave data after it."},
	{Format: "pdf", Why: "Readers look for the end of a PDF near the end of the file and skip what follows it."},
	{Format: "rtf", Why: "An RTF document ends at its closing brace and most readers skip what follows it."},
	{Format: "zip", Why: "Some ZIP readers skip bytes after the last record of an archive and others warn about them."},
	{Format: "docx", Why: "A Word document is a ZIP package, and some readers skip bytes after its last record while others refuse the file."},
	{Format: "xlsx", Why: "An Excel workbook is a ZIP package, and some readers skip bytes after its last record while others refuse the file."},
	{Format: "pptx", Why: "A PowerPoint presentation is a ZIP package, and some readers skip bytes after its last record while others refuse the file."},
	{Format: "targz", Why: "Some gzip readers skip bytes after the end of the stream with a warning and others refuse the archive."},
	{Format: "ico", Why: "An icon is read through the offsets in its directory, so bytes nothing points at are allowed."},
	{Format: "tiff", Why: "A TIFF is read through the offsets in its directories, so bytes nothing points at are allowed."},
	{Format: "eml", Why: "Whether bytes after the end of a message break it depends on how its body is encoded, and mail readers skip a part they cannot place."},
	{Format: "txt", When: map[string][]string{"encoding": {"utf-16be", "utf-16le"}},
		Why: "In UTF-16 most random bytes are valid characters, so the file can still be valid text."},
	{Format: "md", When: map[string][]string{"encoding": {"utf-16be", "utf-16le"}},
		Why: "In UTF-16 most random bytes are valid characters, so the document can still be valid text."},
}

// junkBytes is how many bytes this run appends, falling back for the reason
// headBytes gives.
func junkBytes(v Values) int {
	n, err := strconv.Atoi(v[SettingBytes])
	if err != nil || n < smallestJunk || n > largestJunk {
		return defaultJunk
	}
	return n
}

// junkTail passes the file through untouched and writes the junk once it has
// ended.
type junkTail struct {
	out  io.Writer
	seed uint64
	n    int
	// wrote records that the junk went out. Junk is never empty, so a junk
	// tail that wrote is one that changed the file.
	wrote bool
}

func (j *junkTail) Write(p []byte) (int, error) { return j.out.Write(p) }

func (j *junkTail) Touched() bool { return j.wrote }

// Finish writes the junk: the 64 bit draws of this damage's own source, eight
// bytes each, least significant first. Frozen from the first release by a
// golden hash (D11), because changing how the bytes are drawn changes every
// file this damage ever wrote.
func (j *junkTail) Finish() error {
	r := core.NewRand(j.seed)
	buf := make([]byte, min(j.n, junkChunk)+8)
	for left := j.n; left > 0; {
		size := min(left, junkChunk)
		for i := 0; i < size; i += 8 {
			binary.LittleEndian.PutUint64(buf[i:], r.Uint64())
		}
		if _, err := j.out.Write(buf[:size]); err != nil {
			return err
		}
		left -= size
		j.wrote = true
	}
	return nil
}
