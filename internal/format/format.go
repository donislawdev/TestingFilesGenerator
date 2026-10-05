package format

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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
// its ID3v2 tag, and BMP, ICO and TIFF in the gap before the image data. An
// interface built around "padding goes at the end" would have to be rewritten
// when the twelfth format arrives.
//
// TIFF used to be described here as padding "between its directories", which
// is what the documents said before anybody measured. It writes one directory,
// so there is no between - and the gap it does use is the one StripOffsets
// points past, the same shape as bfOffBits in BMP. Corrected 2026-08-29.
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

// PropertyKind is what sort of value a setting takes.
//
// It exists so something other than the generator can answer the question. A
// window has to know whether to draw a number field, a list or a switch, and
// "tfg formats png" has to be able to say what png accepts - neither of which
// is possible when the only description is a name.
type PropertyKind string

const (
	// PropertyInt is a whole number, bounded by Min and Max.
	PropertyInt PropertyKind = "int"
	// PropertyChoice is one of a closed set of names.
	PropertyChoice PropertyKind = "choice"
	// PropertyBool is true or false.
	PropertyBool PropertyKind = "bool"
	// PropertySize is a size written the way --size accepts it, so 2mb or a
	// plain byte count. Its own kind rather than text, because a window draws
	// it differently and because the same syntax failing here while working
	// for --size is the kind of difference nobody would predict.
	PropertySize PropertyKind = "size"
	// PropertyText is free text the format interprets itself.
	PropertyText PropertyKind = "text"
)

// Property is one setting a format understands, described well enough that
// something other than the format can act on it.
//
// Names alone used to be the whole of this, and the type and the range lived
// inside the generator that read them. That put the knowledge one import away
// from every consumer that needed it: tfg formats could not print it, a window
// had nothing to build a field from, and each format phrased the same refusal
// in its own words. Worse, a bad value surfaced as an ordinary error and came
// out with the exit code that means the tool itself broke.
//
// Two formats declare properties today and every format will eventually - a
// WAV its sample rate and channels, a ZIP its compression method, a JPG its
// quality. This is shaped for that rather than for the two.
type Property struct {
	Name string
	Kind PropertyKind

	// Min and Max bound an int. Both zero means unbounded.
	Min, Max int64
	// Unit is what an int counts, for the message and for a window's field.
	// Empty when the number counts itself, as a page count does.
	Unit string
	// Choices are the allowed values of a choice, lower case.
	Choices []string

	// Shape is what free text has to look like, in a few words, for a kind that
	// has no range and no closed set to describe itself with.
	//
	// It exists because "text" was the whole of what a text setting could say
	// about itself, and under a field that reads as no description at all -
	// seen on screen on 2026-08-05, where the spread of a boundary set was
	// announced as "text, default 1B,1kb,1mb". The value it wants is a list of
	// sizes separated by commas, the declaration knew that, and there was
	// nowhere to put it. Ignored by every other kind, which say what they take
	// from their own range or set.
	Shape string

	// Default is what the format uses when nothing says otherwise, written
	// the way a person would write it. Empty means the format works it out -
	// a picture size chosen to fit the requested bytes, for instance.
	Default string
	// Detail is one sentence for a person, and it is what tfg formats prints
	// and what a window shows beside the field.
	Detail string

	// Group names the block of settings this one opens or belongs to, for a
	// format that declares enough of them to need blocks. Empty for the
	// settings that come first and belong to no block.
	//
	// It exists because PDF went from two settings to twelve in one step, and
	// eight of them describe the document rather than its pages - a title, an
	// author, two dates. One column of twelve names read as a list nobody
	// sorted. The window draws the name as a heading above the first setting
	// of the block and tfg formats prints it the same way.
	//
	// A block is declared as consecutive settings. Register refuses a group
	// that comes back after another one, because both surfaces draw a heading
	// where a block starts and a split block would get two.
	Group string

	// Secret marks a value that is a credential rather than a description of
	// the file, and there is exactly one of them today: the password an
	// archive is locked with.
	//
	// It does NOT mean the value is hidden. A locked fixture whose password is
	// not written down is worth nothing, so the manifest records it on purpose
	// and says so in that property's own Detail. What this flag decides is
	// everything AROUND that one deliberate place: the recorded command line
	// does not repeat it, and the manifest that carries it is written for its
	// owner rather than for everyone on the machine.
	//
	// Declared here rather than known by the places that care, because a
	// second secret property added later would otherwise have to find them.
	Secret bool

	// Long marks free text whose value is long by nature - a checksum of 64 or
	// 128 digits - so a window gives its box the whole row, as it gives a path.
	//
	// It says something about the value rather than about pixels, and a
	// terminal has nothing to do with it. Every other box of text is as wide
	// as a short name (the owner's report of 2026-09-21), which is right for a
	// name and showed about twenty of the sixty four digits a pasted sha256
	// has (docs/NARZEDZIA-SUMY-2026-09-29.md §14, the owner's decision of
	// 2026-09-30). Ignored by every kind but text.
	Long bool
}

// JointLimit is a rule binding two settings that neither of them can state
// alone.
//
// A Property bounds one value. PNG needs more than that: each side of a picture
// may go up to twenty thousand pixels and the two multiplied may not pass forty
// megapixels, because the picture is held in memory while it is encoded. That
// rule had nowhere to live, so it lived in the generator and in a sentence of
// prose - and "tfg formats png" offered a pair it then refused.
//
// A sentence is enough for a person and not for anything else. AR9 has the
// registry as the one place a consumer asks what a format accepts, and a window
// drawing two number fields from Min and Max would offer twenty thousand in
// both and produce a request the run rejects. Declaring it is what lets the
// refusal, the printed description and a future field all come from one place.
//
// Kept to a product of two because that is the shape every case in Tier 1 has -
// pixels, samples times channels, pages times page size. A general expression
// would be a language to learn, and this is a line to read.
type JointLimit struct {
	// Of and By are the two settings multiplied together.
	Of, By string
	// Max is the largest their product may be, counted in the same units the
	// settings themselves use.
	Max int64
	// Unit is what the product is reported in, and Per is how many of the
	// settings' own units make one of it. Pixels are counted in millions when
	// spoken about, so Unit is "megapixels" and Per is a million - "400
	// megapixels and the limit is 40" is a sentence somebody can act on and
	// "400000000 and the limit is 40000000" is not. Per of nought means one.
	Unit string
	Per  int64
	// Base is what the product itself counts, for the sentence that has to be
	// exact: "pixels", "cells".
	//
	// It exists because the readable form above cannot always be used. Rounding
	// two counts that differ can land them on one number, and then the refusal
	// says "they come to 40 megapixels and the limit is 40" - measured on
	// 2026-09-22 for a picture of 20000x2001, which is 40 020 000 pixels
	// against a limit of 40 000 000. A person reading that has been told
	// nothing. See Allows.
	Base string
	// Why is the reason, in the words the refusal uses.
	Why string
}

// Allows reports whether a pair of values satisfies the limit, and says what is
// wrong when it does not.
//
// Both values are expected to have passed their own Property first, and this
// does not check that again. of*by is not guarded against overflow because the
// declarations that reach it cap each side at twenty thousand, so the product
// is nine orders of magnitude below the range of the type. A guard here would
// be a branch nothing could ever reach, and an unreachable branch reads as a
// protection somebody is relying on.
//
// The format is whose limit it is, so a window finds the reason in its own
// words under the key it keeps it by.
func (j JointLimit) Allows(format string, of, by int64) (bad core.Said) {
	got := of * by
	if got <= j.Max {
		return core.Said{}
	}
	asked, allowed := j.readably(got)
	return core.Says("format.JointTooMuch", "together they come to %s and the limit is %s, because %s",
		core.A("Asked", asked), core.A("Limit", allowed),
		core.A("Why", core.Term{Key: core.JointKey(core.FormatOwner(format), j.Of, j.By), Text: j.Why}))
}

// Subject is the pair of settings this limit binds, named for a refusal.
func (j JointLimit) Subject() core.Said {
	return core.Says("format.TwoSettings", "%s and %s", core.A("Of", core.LabelTerm(j.Of)), core.A("By", core.LabelTerm(j.By)))
}

// readably is the pair of counts as a person reads them, and it never puts two
// different counts on one number.
//
// The rounded form is offered first, because that is the one worth reading:
// "400 megapixels and the limit is 40 megapixels" is a sentence somebody can
// act on. It is stood down when both counts round to the same text, which is
// not a corner case - it is what a request just over the limit looks like.
// Measured on 2026-09-22 (O232): 20000x2001 is 40 020 000 pixels, the limit is
// 40 000 000, and the sentence read "they come to 40 megapixels and the limit
// is 40". Two identical numbers, and no way to tell how far over it was.
//
// Both halves carry the unit now. Only the first one did, so the limit was a
// bare number taking its noun from four words earlier.
//
// Adding decimal places was the other way out and it does not work: 40 000 001
// against 40 000 000 collides at every fixed number of places.
func (j JointLimit) readably(got int64) (asked, allowed core.Said) {
	if j.per() > 1 && got/j.per() != j.Max/j.per() {
		return core.Says("format.CountOf", "%d %s", core.A("Number", got/j.per()), core.A("Unit", core.UnitTerm(j.Unit))),
			core.Says("format.CountOf", "%d %s", core.A("Number", j.Max/j.per()), core.A("Unit", core.UnitTerm(j.Unit)))
	}
	return core.Says("format.ExactlyOf", "%s %s", core.A("Number", core.Exactly(got)), core.A("Unit", core.UnitTerm(j.Base))),
		core.Says("format.ExactlyOf", "%s %s", core.A("Number", core.Exactly(j.Max)), core.A("Unit", core.UnitTerm(j.Base)))
}

// Describe is the rule as one sentence, for the format list and for a window.
func (j JointLimit) Describe() string {
	return fmt.Sprintf("%s times %s cannot pass %d %s, because %s",
		j.Of, j.By, j.Most(), j.Unit, j.Why)
}

// Most is the ceiling counted in Unit - 40 for forty million pixels in
// megapixels - which is the number a sentence about this limit says. Here
// rather than worked out by whoever words the sentence, so the window saying it
// in another language divides the same way.
func (j JointLimit) Most() int64 { return j.Max / j.per() }

func (j JointLimit) per() int64 {
	if j.Per == 0 {
		return 1
	}
	return j.Per
}

// Descriptor is everything a format announces about itself. A format missing
// any of it fails the registry test rather than shipping half implemented.
type Descriptor struct {
	ID string

	// Name is what the format is called where it is known by a name - JPEG XL
	// for jxl, Portable Network Graphics for png - so that somebody who does
	// not recognise the identifier can still find the format. A proper name,
	// in English and never translated, which is why it can live here rather
	// than in a language file. It is shown beside the identifier and never
	// replaces it: the identifier is what a recipe and a manifest carry.
	//
	// Nothing written into a file, a manifest or a recipe reads it, so adding
	// or rewording one changes no byte a run produces (D11). The one place it
	// is a contract is the "name" key of "tfg formats --json", added on
	// 2026-09-24 as a widening. Recorded in docs/FORMAT-NAMES-2026-09-24.md.
	Name string

	Extension        string
	Fidelity         Fidelity
	Determinism      Determinism
	MinBytes         int64
	Padding          PaddingChannel
	Label            LabelCarrier
	Oracle           string
	GeneratorVersion string
	Generator        Generator

	// Properties are the settings this format understands. Anything else in a
	// recipe is a typo, and a typo accepted in silence gives a file with
	// default settings and an hour spent wondering why the test passes when
	// it should not. An empty list means the format takes no properties.
	Properties []Property

	// JointLimits are the rules binding two settings that neither can state on
	// its own. Empty for every format that has none.
	JointLimits []JointLimit

	// Unsupported are settings this format deliberately cannot take, each with
	// the reason. A key here is refused with that reason rather than with the
	// generic "no such property", which reads as a gap in this build.
	Unsupported []UnsupportedSetting

	// AllocCeiling is how many objects this format may allocate producing one
	// file, when the flat ceiling every other one meets does not describe it.
	// Zero means the flat one applies, which is the case for all but one.
	//
	// It exists because a borrowed encoder allocates on its own account. The
	// hand written generators here sit between 3 and 128 objects a file, and
	// gav1d, the AVIF encoder, sits at about a hundred - but gen2brain/jxl
	// allocates per block, about 618 000 of them for one 640x480 picture. A
	// single ceiling has to fit the heaviest format, so one that fits that one
	// would say nothing about the other twenty three.
	//
	// What the ceiling stands in for is untouched by this: the guard also asks
	// each format whether its allocation GROWS with the size of the file
	// asked for, and that question is the real one. Every format answers it,
	// this one included. Owner's decision, 2026-08-31.
	//
	// A ratchet, like the coverage threshold and the code shape ceilings: it
	// goes down when work makes it lowerable, never up to turn a run green.
	AllocCeiling int64

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

// What happened, what can do it instead, and what to do about it.
func (e *NotAContainerError) What() string { return e.what().String() }

func (e *NotAContainerError) what() core.Said {
	return core.Says("format.NotAContainer", "%s holds no other files, so it cannot take contains", core.A("Format", e.Format))
}

func (e *NotAContainerError) Why() string { return e.why().String() }

func (e *NotAContainerError) why() core.Said {
	return core.Says("format.NotAContainerWhy", "the formats that can are %s", core.A("Containers", strings.Join(e.Containers, ", ")))
}

func (e *NotAContainerError) Instead() string { return e.instead().String() }

func (e *NotAContainerError) instead() core.Said {
	return core.Says("format.NotAContainerFix", "Drop contains, or change the format")
}

// Parts is what happened, why and what to do instead, for a reader that lays
// them out apart and in its own language.
func (e *NotAContainerError) Parts() (what, why, instead core.Said) {
	return e.what(), e.why(), e.instead()
}

func (e *NotAContainerError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAContainerError) Said() core.Said {
	return core.Says("format.NotAContainerWhole", "%s - %s. %s", core.A("What", e.what()), core.A("Why", e.why()), core.A("Fix", e.instead()))
}

// ContentsConflictError is contains stated beside format properties saying the
// same thing. Picking one would build an archive holding something other than
// what the recipe says, and the recipe is what somebody reads in a review.
type ContentsConflictError struct {
	Format string
	Keys   []string
}

func (e *ContentsConflictError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *ContentsConflictError) Said() core.Said {
	if len(e.Keys) == 1 {
		return core.Says("format.ContentsConflictOne",
			"%s: contains and the %s property both say what the archive holds. Keep contains and drop the properties, or the other way round",
			core.A("Format", e.Format), core.A("Key", e.Keys[0]))
	}
	return core.Says("format.ContentsConflict",
		"%s: contains and the %s properties both say what the archive holds. Keep contains and drop the properties, or the other way round",
		core.A("Format", e.Format), core.A("Keys", strings.Join(e.Keys, ", ")))
}

// NestingUnsupportedError is a container asked to hold its own format.
//
// A legitimate test case that needs a depth limit before it is allowed, and
// there is none yet. It says that rather than pretending the format is unknown.
type NestingUnsupportedError struct {
	Format string
}

func (e *NestingUnsupportedError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *NestingUnsupportedError) Said() core.Said {
	return core.Says("format.NestingUnsupported",
		"%s cannot hold %s yet - an archive inside an archive needs a depth limit first. Hold a different format, or build the inner archive as its own target",
		core.A("Format", e.Format), core.A("Inner", e.Format))
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

// Allows reports whether raw is a value this property accepts, and says what
// is wrong when it is not.
//
// The sentence is built from the declaration, so every format refuses in the
// same words and a new format gets the wording by declaring rather than by
// writing it again.
func (p Property) Allows(raw string) (bad core.Said) {
	switch p.Kind {
	case PropertyInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return p.takesWholeNumber()
		}
		if p.Min != 0 || p.Max != 0 {
			if n < p.Min || n > p.Max {
				return p.takesRange()
			}
		}
	case PropertyChoice:
		// Spelled the way it is declared, not merely close to it. This used to
		// fold with EqualFold, and the folding was invisible here and decisive
		// three layers down: a value the declaration does not contain reached
		// the generator, and each generator did something different with it.
		// Measured 2026-09-02 across the eight formats with a closed set, four
		// answers - a refusal in the generator's own words, a fold that
		// understood it, a swallow that made the DEFAULT file and reported
		// success, and a misread that demanded a password to lock an archive
		// with "NONE". O168. One place to say no is the whole point of
		// declaring the set here.
		for _, c := range p.Choices {
			if raw == c {
				return core.Said{}
			}
		}
		return core.Says("format.TakesOneOf", "it takes one of: %s", core.A("Values", strings.Join(p.Choices, ", ")))
	case PropertyBool:
		// Exact for the same reason, and it costs a user nothing: a recipe
		// writing `header: True` arrives as "true" already, because the reader
		// puts a YAML boolean through FormatBool. Only a hand quoted "TRUE"
		// changes, and it changes into a refusal that names the setting.
		switch raw {
		case "true", "false":
		default:
			return core.Says("format.TakesTrueOrFalse", "it takes true or false")
		}
	case PropertySize:
		if _, err := core.ParseSize(raw); err != nil {
			return core.Says("format.TakesSize", "it takes a size such as 2mb, or a plain byte count")
		}
	case PropertyText:
		// Named rather than left out. Free text has no shape to check, so
		// anything is allowed - and a kind added later reddens the linter
		// instead of arriving here and being accepted without a word.
	}
	return core.Said{}
}

// takesWholeNumber and takesRange are what a whole number setting takes, with
// what it counts when the declaration says.
func (p Property) takesWholeNumber() core.Said {
	if p.Unit == "" {
		return core.Says("format.TakesWholeNumber", "it takes a whole number")
	}
	return core.Says("format.TakesWholeNumberOf", "it takes a whole number of %s", core.A("Unit", core.UnitTerm(p.Unit)))
}

func (p Property) takesRange() core.Said {
	if p.Unit == "" {
		return core.Says("format.TakesRange", "it takes a whole number from %d to %d", core.A("Least", p.Min), core.A("Most", p.Max))
	}
	return core.Says("format.TakesRangeOf", "it takes a whole number of %s from %d to %d",
		core.A("Unit", core.UnitTerm(p.Unit)), core.A("Least", p.Min), core.A("Most", p.Max))
}

// sizePhrase is what a size setting takes, written once.
//
// Allows and Allowed both have to say this, and until 2026-08-26 they said it
// differently - "a size written the way any size is, such as 2mb or a plain
// byte count" against "a size such as 2mb, or a plain byte count". The other
// four kinds agree word for word, so size was the one place where the sentence
// a person reads depended on whether they had already typed something. O64.
//
// The shorter half won: "written the way any size is" is a sentence about
// sizes rather than about this setting, and somebody who has just been refused
// needs the shape, not the provenance.
const sizePhrase = "a size such as 2mb, or a plain byte count"

func (p Property) unitSuffix() string {
	if p.Unit == "" {
		return ""
	}
	return " of " + p.Unit
}

// Allowed says what this property accepts, as one phrase for a person.
//
// It lives here rather than beside a consumer because there are two of them and
// they cannot import each other. "tfg formats png" prints it in a list, and the
// window shows it under the field it drew from this same declaration - so a
// second copy would be two surfaces coming to describe one format differently,
// which is D1 in the place nobody thinks to compare.
//
// Near neighbour of Allows above, and deliberately not folded into it. Allows
// answers somebody who has already typed a wrong value and names the flag that
// would have taken a right one. This answers somebody looking at an empty field.
// The two phrasings agree today for every kind but size, and unifying them would
// change a message the command line already prints - a decision for the owner
// rather than a tidy up. Recorded as O64.
func (p Property) Allowed() string {
	var what string
	switch p.Kind {
	case PropertyInt:
		if p.Min != 0 || p.Max != 0 {
			what = fmt.Sprintf("whole number%s from %d to %d", p.unitSuffix(), p.Min, p.Max)
		} else {
			what = "whole number" + p.unitSuffix()
		}
	case PropertyChoice:
		what = "one of: " + strings.Join(p.Choices, ", ")
	case PropertyBool:
		what = "true or false"
	case PropertySize:
		what = sizePhrase
	default:
		// A text setting describes itself with Shape or not at all. Saying
		// "text" under a field is a word where a description should be.
		what = p.Shape
	}
	if p.Default != "" && what == "" {
		return "default " + p.Default
	}
	if p.Default != "" {
		what += ", default " + p.Default
	}
	return what
}

// SmallestAccepted is the smallest size this format will actually produce for
// a request shaped like r.
//
// MinBytes beside it is the structural floor: the skeleton of the format with
// no label and nothing else. That is a real thing to know and it is not what a
// person reading a column headed MINIMUM takes it to mean, because the label is
// on unless it is turned off and some formats size themselves from their
// settings. Measured on 2026-08-03, asking for exactly what the tool printed:
//
//	pdf   printed 3265   refused it, said 3286
//	wav   printed 44     refused it, said 98
//	zip   printed 156    refused it, said 4285
//
// The generator is asked rather than a second number being declared beside the
// first. A declaration would be one more thing to keep in step, and the
// generator already works this out - it has to, in order to refuse - and
// carries it in the refusal. So the answer here and the answer somebody gets
// when they ask for one byte less cannot disagree.
// It asks repeatedly rather than once, and that is not caution - it is
// arithmetic. The minimum a format reports depends on the size being asked
// for, because the self describing label states the byte count and a longer
// number is a longer label. So the answer is a fixed point: keep asking until
// a size is accepted, moving to whatever the refusal names next.
//
// Measured on 2026-08-03, which is how this was found rather than reasoned
// about. Asking once at nought gave pdf 3334 while 3286 was accepted, and wav
// 96 while 96 was refused and 98 was not. One question gives a number that is
// wrong in either direction.
//
// A format may also refuse a band above a size it accepts - PNG takes 73 and
// refuses 74 through 84, because the smallest chunk that could make up the
// difference costs twelve on its own - so this steps by one when a refusal
// names nothing higher, rather than assuming the answer only ever grows.
func (d Descriptor) SmallestAccepted(r Request) int64 {
	r.SizeFromContents = false

	// Sixty four rounds is far more than any format needs: each round either
	// settles or jumps to a number the format itself named. It is here so that
	// a generator whose refusals ever cycle costs a wrong number rather than a
	// run that never ends.
	size := int64(0)
	for round := 0; round < 64; round++ {
		r.Bytes = size
		if _, err := d.Generator.Plan(r); err == nil {
			return size
		} else {
			var below *BelowMinimumError
			if !errors.As(err, &below) {
				// Refused for a reason that is not about size - a property this
				// request cannot have, say. The structural floor is the honest
				// answer left.
				return d.MinBytes
			}
			if below.Minimum > size {
				size = below.Minimum
				continue
			}
			// The refusal names nothing above where we already are, so this is
			// a size inside a band the format cannot reach. Step past it.
			size++
		}
	}
	return d.MinBytes
}

// PropertyNames is the declared keys, in the order the format listed them.
func (d Descriptor) PropertyNames() []string {
	out := make([]string, 0, len(d.Properties))
	for _, p := range d.Properties {
		out = append(out, p.Name)
	}
	return out
}

// CheckEachProperty is every problem with what was stated, in a stable order.
//
// All of them rather than the first, because a recipe is refused with every
// problem it has - RC7, on the grounds that fixing a file one error per run is
// the cheapest way to make somebody stop using the tool. The recipe reader asks
// this one so it can put each refusal on the box it belongs to. The engine asks
// CheckProperties below, which stops at the first, because by then the recipe
// has already been past this and what is left is the one-target path from the
// command line flags.
func (d Descriptor) CheckEachProperty(props map[string]string) []error {
	known := make(map[string]Property, len(d.Properties))
	for _, p := range d.Properties {
		known[p.Name] = p
	}
	// Sorted, so the same recipe always reports the same key first.
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var bad []error
	for _, k := range keys {
		p, ok := known[k]
		if !ok {
			if cannot, declared := d.cannotCarry(k); declared {
				bad = append(bad, cannot)
				continue
			}
			bad = append(bad, &UnknownPropertyError{Format: d.ID, Key: k, Known: d.PropertyNames()})
			continue
		}
		// An empty value means "not stated", the same as leaving the key out,
		// because that is what an unset flag and an empty recipe entry both
		// look like by the time they arrive here.
		if raw := props[k]; raw != "" {
			if why := p.Allows(raw); !why.IsZero() {
				bad = append(bad, &PropertyValueError{
					Format: d.ID, Key: k, Value: raw, Reason: why, Remedy: p.Instead(),
				})
			}
		}
	}
	return bad
}

// Instead is what to do about a value this property will not take, built from
// the declaration rather than written per format.
//
// A declared default is a fact and is offered as one. Where there is none, the
// format works the value out for itself - which is what the window says in the
// same case, and what every one of the ten properties with no default does
// today. The wording stops at "works it out" rather than naming the size,
// because a property added tomorrow may work it out from something else and
// the sentence has to stay true without anybody checking it.
func (p Property) Instead() core.Said {
	if p.Default != "" {
		return core.Says("format.WriteAValueSuchAs", "write a value it takes, such as %s, or leave the line out", core.A("Default", p.Default))
	}
	return core.Says("format.WriteAValue", "write a value it takes, or leave the line out and the format works it out")
}

// CheckProperties refuses any key the format does not declare, and any value
// the declaration does not allow, stopping at the first.
func (d Descriptor) CheckProperties(props map[string]string) error {
	if bad := d.CheckEachProperty(props); len(bad) > 0 {
		return bad[0]
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
	// Detail is one sentence for a person. The manifest carries its English,
	// and a window says it in its own language.
	Detail core.Said
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
	// manifest so a test can assert on them. Keys a reader outside this
	// program relies on are spelled once, below.
	Properties map[string]any
	// Determinism is the level this particular file reached.
	Determinism Determinism
	// Notes are things that must not be swallowed.
	Notes []Note
	// Memo is the generator's own scratch space, carried from planning to
	// writing. Nothing outside the generator reads it.
	Memo any
}

// PropertyLabelEmbedded is the key a generator sets to say whether the label
// it was asked for actually reached the file.
//
// It is written down once because twenty one places spell it and a typo in any
// of them is silent: the engine reads it with a type assertion that yields
// false rather than an error, so a misspelled key means "this file carries no
// label" in the manifest of a file that carries one.
//
// The spelling itself does not move. It reaches the manifest, which makes it a
// public name under untouchable rule 10 - somebody's test asserts on it - so
// what is centralised here is where it is written, not what it says.
const PropertyLabelEmbedded = "label_embedded"

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
	Reason    core.Said
	Hint      core.Said
}

// What happened, why the minimum exists, and what to do instead.
//
// The same three accessors UnknownPropertyError has carried since the recipe
// reader started reporting four parts, on the refusal every format can produce.
// Error stays exactly as it was and is still assembled by hand, because the
// order it reads best in is not the order the parts join in - the size asked
// for belongs beside the minimum rather than after the reason. A guard asks
// that the sentence still carries the why and the fix, so the two cannot drift.
func (e *BelowMinimumError) What() string {
	return core.Says("format.BelowMinimumWhat", "%s cannot be smaller than %d B. Requested: %d B",
		core.A("Format", e.Format), core.A("Minimum", e.Minimum), core.A("Requested", e.Requested)).String()
}

func (e *BelowMinimumError) Why() string { return e.Reason.String() }

func (e *BelowMinimumError) Instead() string { return e.Hint.String() }

func (e *BelowMinimumError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *BelowMinimumError) Said() core.Said {
	return core.Says("format.BelowMinimum", "%s cannot be smaller than %d B - %s. Requested: %d B. %s",
		core.A("Format", e.Format), core.A("Minimum", e.Minimum), core.A("Why", e.Reason),
		core.A("Requested", e.Requested), core.A("Fix", e.Hint))
}

// SettingSize is the recipe key this refusal is about.
//
// Spelled as the recipe key rather than as either surface's label, for the
// reason engine.RecipeError gives about its own: the keys are the vocabulary
// both surfaces already share, and a third naming is a third thing to keep in
// step.
const SettingSize = core.SettingSize

// AboutSetting lets a window put this message beside the box that caused it.
//
// The one refusal every format can produce, and until 2026-08-12 the only one
// of the four fields on that row with nowhere to land - so a message about the
// size appeared at the foot of the form, 900 px under the box, while a message
// about the count appeared under the count. It answers the same question the
// engine and the preset package answer, and none of the three had to know
// about the others: the window asks an interface rather than switching on a
// type, which is what made this a method rather than a case.
func (e *BelowMinimumError) AboutSetting() string { return SettingSize }

// AboveMaximumError is refusing a size a format is too small to describe.
//
// The mirror of BelowMinimumError, and it exists because three formats were
// using that one for this. A BMP asked for 4294967296 B was told "BMP cannot
// be smaller than 4294967295 B", which is not merely unhelpful - it is the
// opposite of what happened, and the number it offers as a way out is the
// ceiling the request had just passed. ICO and PNG said the same shape of
// thing. Found on 2026-08-26 by asking every registered format what it does
// with a size that does not fit in a thirty two bit field.
//
// It carries the same four parts as every other refusal here - what happened,
// why the ceiling exists, and what to do instead - and answers AboutSetting
// with the same key, so a window marks the same box for either end of the
// range and neither surface had to learn a new type.
type AboveMaximumError struct {
	Format    string
	Requested int64
	Maximum   int64
	Reason    core.Said
	Hint      core.Said
}

func (e *AboveMaximumError) What() string {
	return core.Says("format.AboveMaximumWhat", "%s cannot be larger than %d B. Requested: %d B",
		core.A("Format", e.Format), core.A("Maximum", e.Maximum), core.A("Requested", e.Requested)).String()
}

func (e *AboveMaximumError) Why() string { return e.Reason.String() }

func (e *AboveMaximumError) Instead() string { return e.Hint.String() }

func (e *AboveMaximumError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *AboveMaximumError) Said() core.Said {
	return core.Says("format.AboveMaximum", "%s cannot be larger than %d B - %s. Requested: %d B. %s",
		core.A("Format", e.Format), core.A("Maximum", e.Maximum), core.A("Why", e.Reason),
		core.A("Requested", e.Requested), core.A("Fix", e.Hint))
}

func (e *AboveMaximumError) AboutSetting() string { return SettingSize }

// UnknownFormatError is a request for a format nobody registered.
type UnknownFormatError struct {
	ID    string
	Known []string
}

func (e *UnknownFormatError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *UnknownFormatError) Said() core.Said {
	return core.Says("format.Unknown", "unknown format %q. Known formats: %v", core.A("Format", e.ID), core.A("Known", e.Known))
}
