// Package format defines the generator interface and the registry each format
// announces itself in.
//
// A format declares where its padding channel sits and how much it holds. How
// many bytes are missing is worked out by core, not here.
package format

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Fidelity is how close to the real thing a generated file gets. Every format
// aims for the highest level it can reach, and dropping a level is a
// decision to write down, not a shortcut because it was easier.
type Fidelity string

const (
	// FidelityFull means the file opens correctly in its native application.
	FidelityFull Fidelity = "full"
	// FidelityStructural means parsers accept it and the native application
	// may still complain.
	FidelityStructural Fidelity = "structural"
	// FidelityStub means correct magic bytes and a minimal skeleton.
	FidelityStub Fidelity = "stub"
)

// Determinism is how far the repeatability promise reaches for a format.
type Determinism string

const (
	// DeterminismByte means the same recipe and seed give the same bytes.
	// This is the default and it holds everywhere until the user says
	// otherwise.
	DeterminismByte Determinism = "byte"
	// DeterminismSize means the exact size and the declared properties hold
	// but the bytes depend on the machine, because a system encoder produced
	// them. It needs explicit consent and it is marked per file.
	DeterminismSize Determinism = "size"
)

// Placement says where in the stream a format tolerates arbitrary bytes.
//
// This is not decoration. Four Tier 1 formats pad at the front - MP3 inside
// its ID3v2 tag, BMP and ICO in the gap before the image data, TIFF between
// its directories. An interface built around "padding goes at the end" would
// have to be rewritten when the twelfth format arrives.
type Placement string

const (
	// PlacementEnd means the padding sits at or near the end of the stream,
	// so the writer can measure what came before and then emit it.
	PlacementEnd Placement = "end"
	// PlacementStart means the padding precedes the content, so its size has
	// to be known before the first byte is written.
	PlacementStart Placement = "start"
	// PlacementInside means the padding lives somewhere in the middle, at an
	// offset the format decides.
	PlacementInside Placement = "inside"
)

// LabelCarrier is where a format can carry the self describing label.
type LabelCarrier string

const (
	// LabelVisible means the label is burned into what a person sees.
	LabelVisible LabelCarrier = "visible"
	// LabelInternal means the label rides in metadata or a comment, out of
	// the way of the content.
	LabelInternal LabelCarrier = "internal"
	// LabelExternalOnly means the file never carries the label. Touching the
	// content would change the very data under test, which is the case for
	// CSV and JSON.
	LabelExternalOnly LabelCarrier = "external"
)

// OracleNone is the explicit statement that a format has no reference tool.
// An empty string is not accepted - saying nothing and saying none have to
// look different, otherwise a forgotten declaration passes as a decision.
const OracleNone = "none"

// PaddingChannel is the place a format tolerates arbitrary bytes.
type PaddingChannel struct {
	// Name is what this place is called, for people reading documentation.
	Name string
	// Where in the stream it sits.
	Where Placement
	// Capacity in bytes, or 0 when the channel has no limit of its own.
	// ZIP is the one Tier 1 format with a hard limit - 65 535 bytes of
	// archive comment, above which padding moves into the content.
	Capacity int64
}

// Descriptor is everything a format announces about itself. A format missing
// any of it fails the registry test rather than shipping half implemented.
type Descriptor struct {
	ID               string
	Extension        string
	Fidelity         Fidelity
	Determinism      Determinism
	MinBytes         int64
	Padding          PaddingChannel
	Label            LabelCarrier
	Oracle           string
	GeneratorVersion string
	Generator        Generator

	// Properties are the keys this format understands. Anything else in a
	// recipe is a typo, and a typo accepted in silence gives a file with
	// default settings and an hour spent wondering why the test passes when
	// it should not. An empty list means the format takes no properties.
	Properties []string

	// Container says this format holds other files, so a recipe may declare
	// contains for it.
	//
	// Declared rather than inferred. A format that quietly ignored contains
	// would produce an archive with nothing in it and report success, and
	// that is the silence rule broken in the worst way - the file looks right
	// and the test suite believes it.
	Container bool
}

// NotAContainerError is contains asked of a format that holds nothing.
type NotAContainerError struct {
	Format     string
	Containers []string
}

func (e *NotAContainerError) Error() string {
	return fmt.Sprintf(
		"%s holds no other files, so it cannot take contains - the formats that can are %s. Drop contains, or change the format",
		e.Format, strings.Join(e.Containers, ", "))
}

// ContentsConflictError is contains stated beside format properties saying the
// same thing. Picking one would build an archive holding something other than
// what the recipe says, and the recipe is what somebody reads in a review.
type ContentsConflictError struct {
	Format string
	Keys   []string
}

func (e *ContentsConflictError) Error() string {
	return fmt.Sprintf(
		"%s: contains and the %s propert%s both say what the archive holds. Keep contains and drop the properties, or the other way round",
		e.Format, strings.Join(e.Keys, ", "), plural(len(e.Keys)))
}

// NestingUnsupportedError is a container asked to hold its own format.
//
// A legitimate test case that needs a depth limit before it is allowed, and
// there is none yet. It says that rather than pretending the format is unknown.
type NestingUnsupportedError struct {
	Format string
}

func (e *NestingUnsupportedError) Error() string {
	return fmt.Sprintf(
		"%s cannot hold %s yet - an archive inside an archive needs a depth limit first. Hold a different format, or build the inner archive as its own target",
		e.Format, e.Format)
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// Containers lists the formats that accept contains, for a message that tells
// somebody what to write instead.
func Containers() []string {
	mu.RLock()
	defer mu.RUnlock()
	var out []string
	for id, d := range registry {
		if d.Container {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// UnknownPropertyError is a property key no format recognises.
type UnknownPropertyError struct {
	Format string
	Key    string
	Known  []string
}

func (e *UnknownPropertyError) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("%s takes no properties, so %q is not one of them", e.Format, e.Key)
	}
	return fmt.Sprintf("%s does not have a property called %q. It takes: %s",
		e.Format, e.Key, strings.Join(e.Known, ", "))
}

// CheckProperties refuses any key the format does not declare.
func (d Descriptor) CheckProperties(props map[string]string) error {
	known := make(map[string]bool, len(d.Properties))
	for _, k := range d.Properties {
		known[k] = true
	}
	// Sorted, so the same recipe always reports the same key first.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !known[k] {
			return &UnknownPropertyError{Format: d.ID, Key: k, Known: d.Properties}
		}
	}
	return nil
}

// Content is one group of files a container holds.
//
// A group rather than a file, because "50 PDFs of 2 MB and 200 JPGs of 500 kB"
// is what somebody writes, and expanding that into 250 entries in the recipe
// would make the recipe unreadable and the diff useless.
type Content struct {
	// Format is the id of the format for these members, from the same
	// registry as any other format. A container holds real files.
	Format string
	// Count is how many members this group contributes.
	Count int
	// Bytes is the exact size of each member.
	Bytes int64
}

// Request is what the caller wants from a generator.
type Request struct {
	// Bytes is the exact size of the file, to the byte.
	//
	// It is meaningless when SizeFromContents is set - read that first.
	Bytes int64
	// SizeFromContents says the caller did not name a size and the generator
	// works it out from Contains, reporting it back in Plan.Bytes.
	//
	// A separate flag rather than a zero in Bytes, because zero is a real
	// size: a TXT file of nought bytes is legal and has a minimum of nought.
	// A sentinel that collides with a legal value is how a guard ends up
	// testing the wrong thing.
	SizeFromContents bool
	// Contains is what a container holds. Empty for every other format, and
	// a format that is not a container never receives it - the engine refuses
	// that before planning starts.
	Contains []Content
	// Seed determines the content. The same seed gives the same bytes.
	Seed uint64
	// Label asks for the self describing label. On by default, turned off
	// with --clean.
	Label bool
	// Properties are format specific settings, straight from the recipe.
	Properties map[string]string
}

// Note is something that happened and has to stay visible.
//
// Silence is banned. A file that was skipped, a name the filesystem refused,
// a fidelity level lowered on the fly - every one of those has to show up in
// the manifest and in the output. A manifest that quietly dropped ten files
// looks complete and reaches the test suite as a false truth.
type Note struct {
	// Code is the machine readable reason, for the manifest.
	Code string
	// Detail is one English sentence, for a person.
	Detail string
}

// Plan is the answer to "what exactly will be produced", worked out without
// touching the disk.
//
// Splitting this from the writing is what makes --dry-run a matter of
// skipping the second half rather than a separate path that can drift away
// from the real one. It also means a size a format cannot deliver is refused
// before the first file exists.
type Plan struct {
	// Bytes is the exact size the file will have.
	Bytes int64
	// Exact says whether Bytes is measured or estimated. It is false only
	// for a container whose size comes from its contents through a
	// compressing method - and there --dry-run says so out loud rather than
	// showing a number that looks like all the others.
	Exact bool
	// Properties are the declared facts about the file, carried into the
	// manifest so a test can assert on them.
	Properties map[string]any
	// Determinism is the level this particular file reached.
	Determinism Determinism
	// Notes are things that must not be swallowed.
	Notes []Note
	// Memo is the generator's own scratch space, carried from planning to
	// writing. Nothing outside the generator reads it.
	Memo any
}

// Generator turns a request into bytes.
type Generator interface {
	// Plan works out what will be produced. It never touches the disk and it
	// never writes a byte.
	Plan(Request) (Plan, error)
	// Write emits exactly Plan.Bytes bytes.
	//
	// The context is carried into the generator rather than only wrapping
	// the loop above it. Without that, interrupting a run in the middle of a
	// two gigabyte file waits for that file to finish, and "stop starting new
	// files" means nothing when there is one enormous file.
	Write(ctx context.Context, w io.Writer, p Plan) error
}

// BelowMinimumError is refusing a size a format cannot deliver.
//
// It carries four things on purpose: which format, what its minimum is, why
// that minimum exists, and what to do instead. Rounding up quietly is the one
// thing never on the table - in a batch of ten thousand files a warning is
// lost and the user silently receives data they did not order.
type BelowMinimumError struct {
	Format    string
	Requested int64
	Minimum   int64
	Reason    string
	Hint      string
}

func (e *BelowMinimumError) Error() string {
	return fmt.Sprintf("%s cannot be smaller than %d B - %s. Requested: %d B. %s",
		e.Format, e.Minimum, e.Reason, e.Requested, e.Hint)
}

// UnknownFormatError is a request for a format nobody registered.
type UnknownFormatError struct {
	ID    string
	Known []string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("unknown format %q. Known formats: %v", e.ID, e.Known)
}
