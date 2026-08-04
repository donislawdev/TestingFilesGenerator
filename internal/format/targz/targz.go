// Package targz generates gzip compressed tar archives, including archives
// that pull in other generators.
//
// The second container, and the second Tier 1 format whose padding channel has
// a ceiling. An archive here does not hold random bytes - it holds real
// generated files of other Tier 1 formats, each one valid on its own.
package targz

import (
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

	// commentCapacity is the largest gzip header comment any reader here
	// accepts. RFC 1952 gives the field no length at all, and the document
	// carried that as "no limit" until it was measured on 2026-08-04.
	//
	// Measured, by bisection, on two builds of the same archiver:
	//
	//	7-Zip 26.02 on Windows   accepts 65 535, refuses 65 536
	//	p7zip 23.01 on Linux     accepts 65 535, refuses 65 536
	//
	// The refusal is "Is not archive" on the whole file rather than a warning,
	// so a fixture past that line is not degraded, it is unreadable. GNU gzip
	// 1.12, GNU tar 1.35, bsdtar 3.8.4, Python and node take ten megabytes
	// without a word, and bsdtar draws its own line at 1 048 566 - one mebibyte
	// less the fixed ten byte header, so that one caps the whole header rather
	// than this field.
	commentCapacity = 65535

	// commentPaddingLimit is how much of that we actually use.
	//
	// Not the measured ceiling, and the reason is local rather than borrowed.
	// The second stage below pads through a tar entry, and a tar entry is
	// aligned to 512 bytes, so it delivers bulk and never the last few bytes.
	// The comment is what reaches an exact size, and a few kilobytes of it is
	// more than enough for that - the gap it has to close is under one block.
	//
	// Which leaves no reason to sit on the edge of a two byte length field, and
	// one reason not to: that edge is where a known bad build of p7zip crashed
	// on a ZIP comment filled to its maximum. That build is not installed on
	// any machine here, so this is caution about something unmeasured rather
	// than a measurement, and it costs nothing.
	commentPaddingLimit = 4096

	// fillerName is the entry that carries padding the comment cannot hold.
	// Named plainly, because somebody opening the archive should see what it
	// is rather than wonder.
	fillerName = "tfg-padding.bin"

	// tarBlock is the tar block size. Every header is one block and every
	// entry's content is padded up to a whole number of them, which is why the
	// second padding stage cannot reach an exact byte on its own.
	tarBlock = 512

	// storeBlock is the largest run of bytes gzip emits as one stored block at
	// compression level zero. It decides the framing overhead, so it decides
	// the arithmetic in size.go.
	storeBlock = 65535

	// gzipFraming is the fixed ten byte header plus the eight byte trailer.
	gzipFraming = 18

	// storeBlockCost is what each stored block costs on top of its content.
	storeBlockCost = 5

	defaultEntries  = 1
	defaultEntryFmt = "txt"
	maxEntries      = 10000

	// defaultEntrySizeText is the default size of a file inside, written the
	// way somebody would write it. The number below is derived from it rather
	// than written a second time, because the declaration is what tfg formats
	// prints and nothing makes a printed default agree with the one the code
	// uses. They already disagree in ZIP - it prints 8kb and uses 4096.
	defaultEntrySizeText = "4kb"

	writeChunk = 32 * 1024
)

// defaultEntrySize is defaultEntrySizeText in bytes. Package variables are
// initialised before init runs, so the registration below can rely on it.
var defaultEntrySize = mustSize(defaultEntrySizeText)

func mustSize(s string) int64 {
	n, err := core.ParseSize(s)
	if err != nil {
		panic(fmt.Sprintf("targz: the default entry size %q is not a size this build can parse: %v", s, err))
	}
	return n
}

// A fixed timestamp on every entry. Taking one from the clock would make two
// runs of the same recipe differ, and relying on the zero value would be an
// unstated dependency on what the library does with it.
var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func init() {
	format.Register(format.Descriptor{
		ID:          "targz",
		Extension:   ".tar.gz",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:  "gzip header comment, then a stored filler entry above its limit",
			Where: format.PlacementStart,
			// The comment precedes everything, so how long it is has to be
			// settled before the first byte goes out. That is the whole reason
			// the size is worked out by arithmetic rather than by measuring
			// what came before.
			Capacity: commentCapacity,
		},
		Label:  format.LabelInternal,
		Oracle: "7z",
		Properties: []format.Property{
			{
				Name: "entries", Kind: format.PropertyInt,
				Min: 0, Max: maxEntries,
				Default: strconv.Itoa(defaultEntries),
				Detail:  "How many files the archive holds. Use contains instead when the files are not all alike.",
			},
			{
				Name: "entry_format", Kind: format.PropertyText,
				// Not a choice, for the same reason as in ZIP: the allowed
				// values are whatever this build registered, and a list frozen
				// here would drift the moment a format is added.
				Default: defaultEntryFmt,
				Detail:  "The format of the files inside. Run tfg formats to see what this build supports.",
			},
			{
				Name: "entry_size", Kind: format.PropertySize,
				Default: defaultEntrySizeText,
				Detail:  "How big each file inside is.",
			},
		},
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
// Two ways to say it, exactly as in ZIP. "contains" in a recipe is the general
// one and takes groups of different formats. The entries, entry_format and
// entry_size properties are the flag sized one, reachable through --set.
//
// Both at once is refused rather than one of them being picked, because
// picking would build an archive holding something other than what the recipe
// says, and the recipe is what somebody reads in a pull request.
func groupsFor(r format.Request) ([]format.Content, error) {
	var stated []string
	for _, key := range []string{"entries", "entry_format", "entry_size"} {
		if _, ok := r.Properties[key]; ok {
			stated = append(stated, key)
		}
	}

	// Not len() > 0. An empty contains says "an archive holding nothing", which
	// is a legitimate request and a different statement from saying nothing.
	if r.Contains != nil {
		if len(stated) > 0 {
			return nil, &format.ContentsConflictError{Format: "targz", Keys: stated}
		}
		for _, g := range r.Contains {
			if g.Format == "targz" {
				return nil, &format.NestingUnsupportedError{Format: "targz"}
			}
		}
		return r.Contains, nil
	}

	if r.SizeFromContents {
		return nil, fmt.Errorf("targz: the size was left to the contents and there are none")
	}

	entries, err := intProperty(r.Properties, "entries", defaultEntries, 0, maxEntries)
	if err != nil {
		return nil, err
	}
	entrySize, err := sizeProperty(r.Properties, "entry_size", defaultEntrySize)
	if err != nil {
		return nil, err
	}
	entryFmt := defaultEntryFmt
	if v, ok := r.Properties["entry_format"]; ok && v != "" {
		entryFmt = v
	}
	if entryFmt == "targz" {
		return nil, &format.NestingUnsupportedError{Format: "targz"}
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

	target, label, err := settleSize(&m, r)
	if err != nil {
		return format.Plan{}, err
	}

	p := describe(target, label, m, groups)
	if err := pad(&m, &p, target, label, groups); err != nil {
		return format.Plan{}, err
	}

	p.Memo = m
	return p, nil
}

// planChildren plans every file the archive will hold.
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
			cp, err := desc.Generator.Plan(format.Request{
				Bytes: g.Bytes,
				Seed:  core.FileSeed(r.Seed, index),
				Label: r.Label,
			})
			if err != nil {
				return nil, fmt.Errorf("targz: the %s file inside cannot be made: %w", g.Format, err)
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

// settleSize returns the size the archive aims at and the label it carries.
//
// The label states the size, so a size that comes from the contents is not
// known until the archive has been measured, and measuring it changes the
// label. The two settle against each other the same way they do in ZIP.
func settleSize(m *memo, r format.Request) (target int64, label string, err error) {
	if r.Label && !r.SizeFromContents {
		label = core.Label("targz", r.Bytes, r.Seed)
	}
	m.comment = label

	if !r.SizeFromContents {
		return r.Bytes, label, nil
	}

	bare := archiveSize(*m)
	if !r.Label {
		return bare, label, nil
	}

	// It converges rather than oscillating, because the size only ever grows
	// and a longer number never shortens the label. Bounded anyway, and an
	// error rather than a guess if it somehow does not settle - a label stating
	// the wrong size is worse than the work of finding out.
	size := bare
	for i := 0; i < 4; i++ {
		m.comment = core.Label("targz", size, r.Seed)
		measured := archiveSize(*m)
		if measured == size {
			return size, m.comment, nil
		}
		size = measured
	}
	return 0, "", fmt.Errorf(
		"targz: the size of this archive and the size written in its label do not settle. Give an explicit size")
}

// describe builds the plan and the properties that reach the manifest.
func describe(target int64, label string, m memo, groups []format.Content) format.Plan {
	p := format.Plan{
		Bytes:       target,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"entries":  len(m.children),
			"contains": contentSummary(groups),
			// Stored rather than deflated, which is what makes the size exact
			// in one pass. Stated here so a test can assert on it rather than
			// infer it from how well the file compresses.
			"compression":    "none",
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
		return fmt.Errorf("targz: the plan was not produced by this generator")
	}
	return build(ctx, w, m)
}

// sizeProperty reads a byte count written the way --size accepts it.
func sizeProperty(props map[string]string, key string, fallback int64) (int64, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := core.ParseSize(raw)
	if err != nil {
		return 0, fmt.Errorf("targz: %s: %w", key, err)
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
		return 0, fmt.Errorf("targz: %s must be a whole number, got %q", key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("targz: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return n, nil
}

// minimumBytes is the structural floor of the format: an archive holding
// nothing, with no label and no padding. The end of archive marker is two
// empty blocks, and gzip frames that into 1052 bytes.
//
// An archive holding nothing is a request somebody can actually make, since
// entries goes down to nought, so this is a size the format really accepts
// rather than a number that describes an unreachable skeleton.
//
// It asks no other format anything, and that is on purpose. ZIP works its
// minimum out from a default TXT entry, which only works while txt happens to
// be registered first - its own comment calls that true by accident. The
// import list of format/all is sorted, and targz sorts before txt, so the same
// approach here would panic at startup. Not worked around by ordering imports,
// because gofmt sorts them back.
func minimumBytes() int64 {
	return archiveSize(memo{})
}
