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
}

// Request is what the caller wants from a generator.
type Request struct {
	// Bytes is the exact size of the file, to the byte.
	Bytes int64
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
