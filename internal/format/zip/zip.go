// Package zip generates ZIP archives, including archives that pull in other
// generators.
//
// The first container. An archive here does not hold random bytes - it holds
// real generated files of other Tier 1 formats, each one valid on its own.
package zip

import (
	stdzip "archive/zip"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	generatorVersion = "1"

	// commentCapacity is the hard limit of the archive comment. The field
	// that carries its length is two bytes wide, so this is a property of the
	// format rather than a choice. ZIP is the only Tier 1 format whose
	// padding channel has a ceiling.
	commentCapacity = 65535

	// fillerName is the entry that carries padding the comment cannot hold.
	// Named plainly, because somebody opening the archive should see what it
	// is rather than wonder.
	fillerName = "tfg-padding.bin"

	defaultEntries   = 1
	defaultEntryFmt  = "txt"
	defaultEntrySize = 4096
	maxEntries       = 10000

	writeChunk = 32 * 1024
)

// A fixed timestamp on every entry. Taking one from the clock would make two
// runs of the same recipe differ, and relying on the zero value would be an
// unstated dependency on what the library does with it.
var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func init() {
	format.Register(format.Descriptor{
		ID:          "zip",
		Extension:   ".zip",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "archive comment, then a stored filler entry above its limit",
			Where:    format.PlacementEnd,
			Capacity: commentCapacity,
		},
		Label:            format.LabelInternal,
		Oracle:           "7z",
		Properties:       []string{"entries", "entry_format", "entry_size"},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type child struct {
	name string
	desc format.Descriptor
	plan format.Plan
}

type memo struct {
	children   []child
	comment    string
	fillerSize int64
	withFiller bool
	seed       uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	entries, err := intProperty(r.Properties, "entries", defaultEntries, 0, maxEntries)
	if err != nil {
		return format.Plan{}, err
	}
	// Sizes in properties use the same syntax as --size. Anything else would
	// mean entry_size=200kb failing while size=200kb works, which nobody
	// would predict.
	entrySize, err := sizeProperty(r.Properties, "entry_size", defaultEntrySize)
	if err != nil {
		return format.Plan{}, err
	}

	entryFmt := defaultEntryFmt
	if v, ok := r.Properties["entry_format"]; ok && v != "" {
		entryFmt = v
	}
	if entryFmt == "zip" {
		// An archive of archives is a legitimate test case, but it needs a
		// depth limit before it is allowed, and there is none yet.
		return format.Plan{}, fmt.Errorf("zip: entry_format cannot be zip yet - nesting needs a depth limit first")
	}

	desc, err := format.Get(entryFmt)
	if err != nil {
		return format.Plan{}, err
	}

	// The children are real files of another format, each valid on its own.
	// The registry sits on the same layer as this package, so reaching for it
	// needs nothing from the engine.
	m := memo{seed: r.Seed}
	for i := 0; i < entries; i++ {
		childSeed := core.FileSeed(r.Seed, i)
		cp, err := desc.Generator.Plan(format.Request{
			Bytes: entrySize,
			Seed:  childSeed,
			Label: r.Label,
		})
		if err != nil {
			return format.Plan{}, fmt.Errorf("zip: the %s file inside cannot be made: %w", entryFmt, err)
		}
		m.children = append(m.children, child{
			name: fmt.Sprintf("%s_%04d%s", entryFmt, i+1, desc.Extension),
			desc: desc,
			plan: cp,
		})
	}

	label := ""
	if r.Label {
		label = core.Label("zip", r.Bytes, r.Seed)
	}
	m.comment = label

	// Measure the archive with no padding at all. Every entry is stored
	// rather than compressed, so the size follows from the parts exactly and
	// nothing has to be guessed.
	bare, err := archiveSize(m)
	if err != nil {
		return format.Plan{}, err
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"entries":        entries,
			"entry_format":   entryFmt,
			"entry_size":     entrySize,
			"method":         "store",
			"label_embedded": label != "",
		},
	}

	switch {
	case r.Bytes < bare:
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason: fmt.Sprintf("an archive holding %d %s file(s) of %d B already needs that much",
				entries, strings.ToUpper(entryFmt), entrySize),
			Hint: fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files with --set entries=... --set entry_size=...", bare),
		}
	case r.Bytes == bare:
		// Nothing to pad.
	default:
		needed := r.Bytes - bare
		room := int64(commentCapacity - len(label))
		if needed <= room {
			m.comment = label + strings.Repeat(" ", int(needed))
		} else {
			// The comment is full, so the rest goes into a stored entry
			// whose bytes land in the archive one for one. This is the only
			// padding channel in Tier 1 with a ceiling, and this is what
			// happens above it.
			m.comment = label + strings.Repeat(" ", int(room))
			m.withFiller = true
			withFiller, err := archiveSize(m)
			if err != nil {
				return format.Plan{}, err
			}
			m.fillerSize = r.Bytes - withFiller
			if m.fillerSize < 0 {
				return format.Plan{}, &format.BelowMinimumError{
					Format:    "ZIP",
					Requested: r.Bytes,
					Minimum:   withFiller,
					Reason:    "the padding entry the archive needs at this size does not fit",
					Hint:      fmt.Sprintf("Ask for %d B or more.", withFiller),
				}
			}
			p.Properties["padding_entry"] = fillerName
		}
	}

	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("zip: the plan was not produced by this generator")
	}
	return build(ctx, w, m)
}

// build writes the archive. The same path measures it during planning, with
// the output thrown away, so what was measured and what is written cannot
// drift apart.
func build(ctx context.Context, w io.Writer, m memo) error {
	zw := stdzip.NewWriter(w)
	if err := zw.SetComment(m.comment); err != nil {
		return fmt.Errorf("zip: the archive comment was refused: %w", err)
	}

	for _, c := range m.children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entry, err := zw.CreateHeader(&stdzip.FileHeader{
			Name:     c.name,
			Method:   stdzip.Store,
			Modified: fixedTime,
		})
		if err != nil {
			return err
		}
		if err := c.desc.Generator.Write(ctx, entry, c.plan); err != nil {
			return fmt.Errorf("zip: the %s file inside could not be written: %w", c.desc.ID, err)
		}
	}

	if m.withFiller {
		entry, err := zw.CreateHeader(&stdzip.FileHeader{
			Name:     fillerName,
			Method:   stdzip.Store,
			Modified: fixedTime,
		})
		if err != nil {
			return err
		}
		if err := writeFiller(ctx, entry, m.seed, m.fillerSize); err != nil {
			return err
		}
	}

	return zw.Close()
}

// writeFiller emits the padding entry without holding it in memory.
func writeFiller(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, writeChunk)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		chunk := buf[:size]
		for i := range chunk {
			chunk[i] = byte(rng.UintN(256))
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= size
	}
	return nil
}

// archiveSize measures the archive by building it and counting, throwing the
// bytes away. Everything inside is stored rather than compressed, so this is
// fast and the number is exact.
func archiveSize(m memo) (int64, error) {
	c := &counter{}
	if err := build(context.Background(), c, m); err != nil {
		return 0, err
	}
	return c.n, nil
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// sizeProperty reads a byte count written the way --size accepts it.
func sizeProperty(props map[string]string, key string, fallback int64) (int64, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := core.ParseSize(raw)
	if err != nil {
		return 0, fmt.Errorf("zip: %s: %w", key, err)
	}
	return n, nil
}

func intProperty(props map[string]string, key string, fallback, min, max int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("zip: %s must be a whole number, got %q", key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("zip: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return n, nil
}

// minimumBytes is the smallest archive this generator can produce: one empty
// entry, no label, no padding.
func minimumBytes() int64 {
	desc, err := format.Get(defaultEntryFmt)
	if err != nil {
		// The default entry format is not registered yet, which happens only
		// if registration order changes. Refusing every size is safer than
		// declaring a minimum that was never measured.
		return 1 << 62
	}
	cp, err := desc.Generator.Plan(format.Request{Bytes: 0, Seed: 0, Label: false})
	if err != nil {
		return 1 << 62
	}
	n, err := archiveSize(memo{children: []child{{
		name: "txt_0001.txt", desc: desc, plan: cp,
	}}})
	if err != nil {
		return 1 << 62
	}
	return n
}
