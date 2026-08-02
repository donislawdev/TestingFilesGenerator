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

	// commentPaddingLimit is how much of that we actually use.
	//
	// Measured on CI: p7zip 17.06 on macOS segfaults reading an archive whose
	// comment is filled to the maximum. 7-Zip 26.02 on Windows and p7zip
	// 23.01 on Linux read the same file without complaint, so the crash is a
	// defect in that build rather than in the archive - the structural check
	// agrees the file is well formed.
	//
	// It is still our problem. p7zip is what "7z" means on a lot of machines,
	// and a fixture that crashes the archiver a tester reaches for is a
	// fixture nobody will trust. Padding through a stored entry reaches every
	// size just as exactly, so the enormous comment bought nothing and now it
	// is gone.
	commentPaddingLimit = 4096

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
		Container:        true,
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

// groupsFor works out what the archive holds.
//
// There are two ways to say it. "contains" in a recipe is the general one and
// takes a list of groups of different formats. The entries, entry_format and
// entry_size properties are the flag sized one, reachable through --set, and
// they say the same thing for a single format.
//
// Both at once is refused rather than one of them being picked. Picking would
// produce an archive holding something other than what the recipe says, and
// the recipe is the thing somebody reads in a pull request. Same rule as a
// boundary declared beside a size.
func groupsFor(r format.Request) ([]format.Content, error) {
	var stated []string
	for _, key := range []string{"entries", "entry_format", "entry_size"} {
		if _, ok := r.Properties[key]; ok {
			stated = append(stated, key)
		}
	}

	// Not len() > 0. An empty contains says "an archive holding nothing",
	// which is a legitimate thing to ask for, and it is a different statement
	// from saying nothing at all.
	if r.Contains != nil {
		if len(stated) > 0 {
			return nil, &format.ContentsConflictError{Format: "zip", Keys: stated}
		}
		for _, g := range r.Contains {
			if g.Format == "zip" {
				return nil, &format.NestingUnsupportedError{Format: "zip"}
			}
		}
		return r.Contains, nil
	}

	if r.SizeFromContents {
		// Only reachable if a caller sets the flag without contents. Saying so
		// beats producing an empty archive and calling it the answer.
		return nil, fmt.Errorf("zip: the size was left to the contents and there are none")
	}

	entries, err := intProperty(r.Properties, "entries", defaultEntries, 0, maxEntries)
	if err != nil {
		return nil, err
	}
	// Sizes in properties use the same syntax as --size. Anything else would
	// mean entry_size=200kb failing while size=200kb works, which nobody
	// would predict.
	entrySize, err := sizeProperty(r.Properties, "entry_size", defaultEntrySize)
	if err != nil {
		return nil, err
	}
	entryFmt := defaultEntryFmt
	if v, ok := r.Properties["entry_format"]; ok && v != "" {
		entryFmt = v
	}
	if entryFmt == "zip" {
		return nil, &format.NestingUnsupportedError{Format: "zip"}
	}
	return []format.Content{{Format: entryFmt, Count: entries, Bytes: entrySize}}, nil
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	groups, err := groupsFor(r)
	if err != nil {
		return format.Plan{}, err
	}

	m := memo{seed: r.Seed}
	if m.children, err = planChildren(r, groups); err != nil {
		return format.Plan{}, err
	}

	target, bare, label, err := settleSize(&m, r)
	if err != nil {
		return format.Plan{}, err
	}

	p := describe(target, label, m, groups)
	if err := pad(&m, &p, r, target, bare, label, groups); err != nil {
		return format.Plan{}, err
	}

	p.Memo = m
	return p, nil
}

// planChildren plans every file the archive will hold.
//
// The children are real files of another format, each valid on its own. The
// registry sits on the same layer as this package, so reaching for it needs
// nothing from the engine.
//
// Members are numbered across the whole archive rather than per group, so the
// seed of a member does not move when a group above it changes count. That is
// untouchable rule 2 applied one level down.
func planChildren(r format.Request, groups []format.Content) ([]child, error) {
	var out []child
	index := 0
	// Numbering runs per format rather than per group, so two groups of the
	// same format do not both start at 0001 and collide inside the archive.
	numbered := map[string]int{}
	for _, g := range groups {
		desc, err := format.Get(g.Format)
		if err != nil {
			return nil, err
		}
		for i := 0; i < g.Count; i++ {
			childSeed := core.FileSeed(r.Seed, index)
			cp, err := desc.Generator.Plan(format.Request{
				Bytes: g.Bytes,
				Seed:  childSeed,
				Label: r.Label,
			})
			if err != nil {
				return nil, fmt.Errorf("zip: the %s file inside cannot be made: %w", g.Format, err)
			}
			numbered[g.Format]++
			out = append(out, child{
				name: fmt.Sprintf("%s_%04d%s", g.Format, numbered[g.Format], desc.Extension),
				desc: desc,
				plan: cp,
			})
			index++
		}
	}
	return out, nil
}

// settleSize returns the size the archive is aiming at, the size it comes to
// with no padding, and the label it carries.
//
// The label carries the size, so a size that comes from the contents is not
// known until the archive has been measured. Measure bare first with no
// comment, then settle the label, then measure again.
func settleSize(m *memo, r format.Request) (target, bare int64, label string, err error) {
	if r.Label && !r.SizeFromContents {
		label = core.Label("zip", r.Bytes, r.Seed)
	}
	m.comment = label

	// Measure the archive with no padding at all. Every entry is stored rather
	// than compressed, so the size follows from the parts exactly and nothing
	// has to be guessed.
	if bare, err = archiveSize(*m); err != nil {
		return 0, 0, "", err
	}

	if !r.SizeFromContents {
		return r.Bytes, bare, label, nil
	}

	// The contents decide the size, and the label states the size, so the two
	// settle against each other: a longer number makes a longer file.
	//
	// It converges rather than oscillating, because the size only ever grows
	// and a longer number never shortens the label. Bounded anyway, and an
	// error rather than a guess if it somehow does not settle - a label stating
	// the wrong size is worse than the work of finding out.
	if r.Label {
		size := bare
		settled := false
		for i := 0; i < 4 && !settled; i++ {
			m.comment = core.Label("zip", size, r.Seed)
			measured, err := archiveSize(*m)
			if err != nil {
				return 0, 0, "", err
			}
			settled = measured == size
			size = measured
		}
		if !settled {
			return 0, 0, "", fmt.Errorf(
				"zip: the size of this archive and the size written in its label do not settle. Give an explicit size")
		}
		label = m.comment
		bare = size
	}
	return bare, bare, label, nil
}

// describe builds the plan and the properties that reach the manifest.
func describe(target int64, label string, m memo, groups []format.Content) format.Plan {
	p := format.Plan{
		Bytes:       target,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"entries":        len(m.children),
			"contains":       contentSummary(groups),
			"method":         "store",
			"label_embedded": label != "",
		},
	}
	// The single format shape keeps the keys it always had, so a test asserting
	// on entry_format does not break the day contains arrives.
	if len(groups) == 1 {
		p.Properties["entry_format"] = groups[0].Format
		p.Properties["entry_size"] = groups[0].Bytes
	}
	return p
}

// pad decides where the difference between the bare archive and the size that
// was asked for goes.
func pad(m *memo, p *format.Plan, r format.Request, target, bare int64, label string, groups []format.Content) error {
	switch {
	case target < bare:
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: target,
			Minimum:   bare,
			Reason:    fmt.Sprintf("an archive holding %s already needs that much", describeGroups(groups)),
			Hint:      fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files.", bare),
		}
	case target == bare:
		return nil // Nothing to pad.
	}

	needed := r.Bytes - bare
	room := int64(commentPaddingLimit - len(label))
	if needed <= room {
		m.comment = label + strings.Repeat(" ", int(needed))
		return nil
	}

	// Above what the comment takes, the rest goes into a stored entry whose
	// bytes land in the archive one for one. This is the only padding channel
	// in Tier 1 with a ceiling.
	m.comment = label + strings.Repeat(" ", int(room))
	m.withFiller = true
	withFiller, err := archiveSize(*m)
	if err != nil {
		return err
	}
	m.fillerSize = r.Bytes - withFiller
	if m.fillerSize < 0 {
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: r.Bytes,
			Minimum:   withFiller,
			Reason:    "the padding entry the archive needs at this size does not fit",
			Hint:      fmt.Sprintf("Ask for %d B or more.", withFiller),
		}
	}
	p.Properties["padding_entry"] = fillerName
	return nil
}

// contentSummary is what the archive holds, in the manifest, so a test can
// assert on it without unpacking the file.
func contentSummary(groups []format.Content) []map[string]any {
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		out = append(out, map[string]any{
			"format": g.Format,
			"count":  g.Count,
			"bytes":  g.Bytes,
		})
	}
	return out
}

// describeGroups is the same thing for a person reading an error.
func describeGroups(groups []format.Content) string {
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		parts = append(parts, fmt.Sprintf("%d %s file(s) of %d B", g.Count, strings.ToUpper(g.Format), g.Bytes))
	}
	return strings.Join(parts, " and ")
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
