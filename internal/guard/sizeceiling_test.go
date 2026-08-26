package guard

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A format asked for a size it cannot describe has to say so, not produce it.
//
// Found on 2026-08-26 by an outside review, and the measurement that followed
// was worse than the report. WAV writes three four byte lengths - the RIFF
// size, the data chunk and the JUNK chunk - and nothing bounded them. A request
// for 8 GiB produced a file of exactly 8589934592 B whose RIFF field announced
// 4294967296 B, and every part of this tool agreed the file was fine: the size
// guard in the engine compares the writer's count against the plan and both
// said 8 GiB, the hash went into the manifest, and verify called it a match.
//
// That is rule 6 inverted. Silence is banned here, and this was louder than
// silence - the tool certified a corrupt file.
//
// The check is asked of the REGISTRY rather than of the formats the review
// happened to read, which is the whole point of writing it this way: a format
// added next year is covered on the day it is registered. Two answers are
// acceptable at these sizes - a refusal, or a plan whose bytes really do come
// back out of the writer intact. Anything else fails.
//
// The writer stops after the first bytes rather than producing gigabytes,
// because what is being read is the header. A run that generated them for real
// would cost the disk this project is careful about, and would measure nothing
// extra.
func TestAFormatRefusesASizeItCannotDescribe(t *testing.T) {
	sizes := []struct {
		name  string
		bytes int64
	}{
		// A control first. A check whose rows all say the same thing is
		// measuring itself rather than the code.
		{"1 MiB", 1 << 20},
		{"4 GiB minus one", 1<<32 - 1},
		{"4 GiB", 1 << 32},
		{"8 GiB", 1 << 33},
	}

	for _, d := range format.All() {
		for _, s := range sizes {
			req := format.Request{Bytes: s.bytes, Seed: 7, Label: true}
			if d.Container {
				req.Properties = map[string]string{"entries": "1"}
			}

			plan, err := d.Generator.Plan(req)
			if err != nil {
				var below *format.BelowMinimumError
				var above *format.AboveMaximumError
				if !errors.As(err, &below) && !errors.As(err, &above) {
					t.Errorf("%s at %s: refused with something that is not a refusal about size: %v",
						d.ID, s.name, err)
				}
				continue
			}

			head := &firstBytes{n: 64}
			werr := d.Generator.Write(context.Background(), head, plan)
			if werr != nil && !errors.Is(werr, errSeenEnough) {
				t.Errorf("%s at %s: the plan was accepted and the write failed: %v", d.ID, s.name, werr)
				continue
			}
			if why := headerDisagrees(d.ID, head.seen, s.bytes); why != "" {
				t.Errorf("%s at %s: %s", d.ID, s.name, why)
			}
		}
	}
}

// errSeenEnough stops a writer once the header has gone by.
var errSeenEnough = errors.New("guard: the header has been seen")

type firstBytes struct {
	seen []byte
	n    int
}

func (f *firstBytes) Write(p []byte) (int, error) {
	if len(f.seen) < f.n {
		room := f.n - len(f.seen)
		if room > len(p) {
			room = len(p)
		}
		f.seen = append(f.seen, p[:room]...)
	}
	if len(f.seen) >= f.n {
		return len(p), errSeenEnough
	}
	return len(p), nil
}

// headerDisagrees reads the length a format states about itself and compares it
// with the length that was asked for.
//
// Only the formats that carry a fixed width total are checked, and the ones
// that do not are named as such rather than left as a blank row - a column that
// can never speak reads exactly like a column that passed.
func headerDisagrees(id string, head []byte, want int64) string {
	switch id {
	case "wav":
		if len(head) < 8 || !bytes.HasPrefix(head, []byte("RIFF")) {
			return "no RIFF header in the first bytes"
		}
		// The field counts everything after itself, so the file is eight
		// bytes longer than the number in it.
		if stated := int64(binary.LittleEndian.Uint32(head[4:8])); stated+8 != want {
			return "the RIFF length field says " + strconv.FormatInt(stated+8, 10) + " B and the file is " + strconv.FormatInt(want, 10) + " B"
		}
	case "bmp":
		if len(head) < 6 || !bytes.HasPrefix(head, []byte("BM")) {
			return "no BM header in the first bytes"
		}
		if stated := int64(binary.LittleEndian.Uint32(head[2:6])); stated != want {
			return "the BMP length field says " + strconv.FormatInt(stated, 10) + " B and the file is " + strconv.FormatInt(want, 10) + " B"
		}
	}
	return ""
}

// A refusal about a ceiling has to say ceiling, not floor.
//
// Three formats were using BelowMinimumError for a request that was too large,
// so a BMP asked for 4294967296 B was told "BMP cannot be smaller than
// 4294967295 B". That is not merely unhelpful - it states the opposite of what
// happened, and the number it offers as the way out is the ceiling the request
// had just passed. D6 asks a refusal for four parts, and the third of them, the
// value that is allowed, was pointing the wrong way.
func TestARefusalAboutTooLargeSaysLargerNotSmaller(t *testing.T) {
	// Every format that has a ceiling low enough to reach, asked for something
	// far above it. Read from the registry rather than listed here, so a
	// format that grows a ceiling later is covered without an edit.
	found := 0
	for _, d := range format.All() {
		req := format.Request{Bytes: 1 << 40, Seed: 7, Label: true}
		if d.Container {
			req.Properties = map[string]string{"entries": "1"}
		}
		_, err := d.Generator.Plan(req)
		if err == nil {
			continue
		}

		var above *format.AboveMaximumError
		if !errors.As(err, &above) {
			var below *format.BelowMinimumError
			if errors.As(err, &below) {
				t.Errorf("%s: a request for %d B was refused as being below a minimum of %d B - %q",
					d.ID, below.Requested, below.Minimum, below.Error())
			}
			continue
		}
		found++

		if above.Requested <= above.Maximum {
			t.Errorf("%s: refused a request of %d B against a ceiling of %d B, which is not above it",
				d.ID, above.Requested, above.Maximum)
		}
		// The four parts of D6. Each is asked for separately, because a
		// sentence that happens to be long is not the same as one that
		// answers all four.
		if above.Why() == "" {
			t.Errorf("%s: the refusal does not say why the ceiling exists", d.ID)
		}
		if above.Instead() == "" {
			t.Errorf("%s: the refusal does not say what to do instead", d.ID)
		}
		if !bytes.Contains([]byte(above.Error()), []byte("larger")) {
			t.Errorf("%s: the refusal reads %q, which does not say the size was too large", d.ID, above.Error())
		}
	}

	// Without this the test passes on a build where nothing refuses at all,
	// which is the shape this project keeps finding: a check that never
	// reaches the code it is about.
	if found == 0 {
		t.Fatal("no format refused a request for 1 TiB, so this proved nothing")
	}
}

// A container that would need the zip64 records refuses instead of planning a
// size it will not write.
//
// The arithmetic that works out an archive's size builds the whole structure
// with the contents left out and adds the planned sizes back, so while it is
// measuring, every entry is nought bytes long - and nought bytes never triggers
// zip64. Measured on 2026-08-26 with tools/probes/zip64, the plan against what
// really came out of the writer: zip drifts by 112 B past four gigabytes and an
// Office package by 104 B. TAR.GZ does not drift, which is why it carries no
// ceiling here - the report that raised this said it did.
//
// The failure this replaces was loud rather than silent: the engine compares
// the writer's count against the plan and deletes any file that misses. So
// nothing that worked is being taken away. What changes is that the person is
// told before four gigabytes are written and removed, instead of afterwards
// with a message about the generator disagreeing with its own plan.
func TestAContainerRefusesASizeItsArithmeticCannotSee(t *testing.T) {
	containers := 0
	for _, d := range format.All() {
		if !isZipArchive(d.ID) {
			continue
		}
		containers++

		req := format.Request{Bytes: 5 << 30, Seed: 7, Label: true}
		if d.Container {
			req.Properties = map[string]string{"entries": "1"}
		}
		_, err := d.Generator.Plan(req)
		if err == nil {
			t.Errorf("%s: a request for 5 GiB was planned, and the writer would not match it", d.ID)
			continue
		}
		var above *format.AboveMaximumError
		if !errors.As(err, &above) {
			t.Errorf("%s: refused 5 GiB with %T rather than a refusal about a ceiling: %v", d.ID, err, err)
			continue
		}
		if above.Maximum >= 5<<30 {
			t.Errorf("%s: names a ceiling of %d B, which is not below the 5 GiB asked for", d.ID, above.Maximum)
		}
	}
	if containers == 0 {
		t.Fatal("no ZIP based format was found, so this proved nothing")
	}

	// An archive that crosses the line without any single part crossing it.
	//
	// The case above is carried by one enormous entry, and a mutation showed
	// that it proves less than it looks: with the whole size in the padding
	// entry, the check on the padding catches it and the check on the total
	// never runs. Six hundred files of eight megabytes cross four gigabytes
	// together while no one of them is anywhere near it.
	spread := format.Request{
		Bytes: 5 << 30, Seed: 7, Label: true,
		Contains: []format.Content{{Format: "txt", Count: 600, Bytes: 8 << 20}},
	}
	zip, err := format.Get("zip")
	if err != nil {
		t.Fatalf("zip is not registered: %v", err)
	}
	if _, err := zip.Generator.Plan(spread); err == nil {
		t.Error("an archive of six hundred files past four gigabytes was planned")
	} else {
		var above *format.AboveMaximumError
		if !errors.As(err, &above) {
			t.Errorf("refused with %T rather than a refusal about a ceiling: %v", err, err)
		}
	}
}

// isZipArchive names the formats built on a ZIP archive, which is what decides
// whether the zip64 line applies at all.
//
// A list rather than a flag on the descriptor, because it is not a question a
// consumer of the registry ever asks - Container says whether a format holds
// other generated files, which is a different question: TAR.GZ is a container
// and is not a ZIP, and an Office package is a ZIP and is not a container.
//
// TAR.GZ being absent is measured rather than assumed. tools/probes/zip64 at
// 4 GiB + 1 MiB and at 5 GiB: zip drifts by 112 B, docx by 104 B, and targz by
// nothing at all. The review that raised this said TAR.GZ had the same defect,
// and it does not.
func isZipArchive(id string) bool {
	switch id {
	case "zip", "docx", "xlsx", "pptx":
		return true
	}
	return false
}
