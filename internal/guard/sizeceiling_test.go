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
