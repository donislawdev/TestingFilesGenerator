// Package format defines the generator interface and the registry each format
// announces itself in.
//
// A format declares where its padding channel sits and how much it holds. How
// many bytes are missing is worked out by core, not here.
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
	// Why is the reason, in the words the refusal uses.
	Why string
}

// Allows reports whether a pair of values satisfies the limit, and says what is
// wrong when it does not.
func (j JointLimit) Allows(of, by int64) (bad string) {
	if of*by <= j.Max {
		return ""
	}
	return fmt.Sprintf("together they come to %d %s and the limit is %d, because %s",
		of*by/j.per(), j.Unit, j.Max/j.per(), j.Why)
}

// Describe is the rule as one sentence, for the format list and for a window.
func (j JointLimit) Describe() string {
	return fmt.Sprintf("%s times %s cannot pass %d %s, because %s",
		j.Of, j.By, j.Max/j.per(), j.Unit, j.Why)
}

func (j JointLimit) per() int64 {
	if j.Per == 0 {
		return 1
	}
	return j.Per
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

	// Properties are the settings this format understands. Anything else in a
	// recipe is a typo, and a typo accepted in silence gives a file with
	// default settings and an hour spent wondering why the test passes when
	// it should not. An empty list means the format takes no properties.
	Properties []Property

	// JointLimits are the rules binding two settings that neither can state on
	// its own. Empty for every format that has none.
	JointLimits []JointLimit

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
func (e *NotAContainerError) What() string {
	return fmt.Sprintf("%s holds no other files, so it cannot take contains", e.Format)
}

func (e *NotAContainerError) Why() string {
	return "the formats that can are " + strings.Join(e.Containers, ", ")
}

func (e *NotAContainerError) Instead() string { return "Drop contains, or change the format" }

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

// AboutSetting is the property this refusal is about, so a form can put the
// message under the box it came from. Its sibling below has carried this since
// 2026-08-12 and this one did not, which made a mistyped key the one refusal
// about a declared setting that still landed at the foot of the form.
func (e *UnknownPropertyError) AboutSetting() string { return e.Key }

// Why this is refused, for a report that keeps the four parts of D6 apart.
func (e *UnknownPropertyError) Why() string {
	return "a format takes only the settings it declares, and one it does not know would be dropped on the way"
}

// Instead is what to do about it, named from the declaration.
func (e *UnknownPropertyError) Instead() string {
	if len(e.Known) == 0 {
		return "remove the line"
	}
	return "use one of: " + strings.Join(e.Known, ", ")
}

// What happened, without the list of names. Kept apart from Error so a report
// with four parts does not print the names twice - once in the sentence and
// again in what to do instead.
func (e *UnknownPropertyError) What() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("%s takes no properties, so %q is not one of them", e.Format, e.Key)
	}
	return fmt.Sprintf("%s does not have a property called %q", e.Format, e.Key)
}

// Error is the whole thing in one sentence, unchanged to the character - it is
// what the one-target path from the command line flags prints.
func (e *UnknownPropertyError) Error() string {
	if len(e.Known) == 0 {
		return e.What()
	}
	return e.What() + ". It takes: " + strings.Join(e.Known, ", ")
}

// PropertyValueError is a declared key given a value the declaration forbids.
//
// Separate from UnknownPropertyError because the mistake is different and so
// is the fix: one is a key that does not exist, the other a value out of
// range. Both are the caller's doing rather than the tool's, which is the
// point - this used to surface as a plain error and end with the exit code
// that means the program itself failed, so CI could not tell "you typed that
// wrong" from "this build has a bug".
type PropertyValueError struct {
	Format string
	Key    string
	Value  string
	Reason string
	// Remedy is what to do about it, built from the declaration. It is carried
	// here rather than worked out by whoever reports this, because a refusal in
	// this tool has four parts - what happened, why, what is allowed, what to do
	// instead (D6) - and the fourth had nowhere to come from until 2026-08-25.
	// A reader that wants the whole thing in one sentence still gets it from
	// Error, which leaves this out: it is the part a form puts under the box.
	//
	// Named Remedy rather than Instead because the accessor below has to be
	// called Instead - that is the name the other refusals in this package use
	// for the same part, and the reader that asks for all three asks by name.
	Remedy string
}

// What happened, why the declaration forbids it, and what to do instead.
//
// Instead is the part Error leaves out on purpose - see the field - so this is
// the only way a report gets all four parts of D6 for the refusal a person hits
// most often, by typing a number.
func (e *PropertyValueError) What() string {
	return fmt.Sprintf("%s: %s cannot be %q", e.Format, e.Key, e.Value)
}

func (e *PropertyValueError) Why() string { return core.InTheWordsOf(e.Reason, e.Key) }

func (e *PropertyValueError) Instead() string { return e.Remedy }

func (e *PropertyValueError) Error() string {
	return e.InTheWordsOf(e.Key)
}

// InTheWordsOf is this refusal with the property named the way one surface
// names it - the declared key on the command line, the label above the box in
// a window. See core.SettingSlot.
//
// The name is a field here rather than a slot in a sentence, because this
// refusal is assembled rather than written out: the key already stands on its
// own in the format string. The reason a property gives can still hold slots,
// so a declaration that names itself twice needs no special case.
func (e *PropertyValueError) InTheWordsOf(name string) string {
	if name == "" {
		name = e.Key
	}
	return fmt.Sprintf("%s: %s cannot be %q - %s",
		e.Format, name, e.Value, core.InTheWordsOf(e.Reason, name))
}

// AboutSetting is the property this refusal is about, so a window can put the
// message under the box it came from.
//
// It was missing until 2026-08-12, and the gap is worth recording because it
// was invisible from either side. A window cannot place a message it is not
// told the subject of, so every refusal about a declared setting - width,
// height, pages, entries, a preset's own parameters - landed at the foot of the
// form however carefully the window was written. Two of the three interfaces
// beside this one had the method. This one is the one that fires most often,
// because it is the one a person hits by typing a number.
func (e *PropertyValueError) AboutSetting() string { return e.Key }

// Allows reports whether raw is a value this property accepts, and says what
// is wrong when it is not.
//
// The sentence is built from the declaration, so every format refuses in the
// same words and a new format gets the wording by declaring rather than by
// writing it again.
func (p Property) Allows(raw string) (bad string) {
	switch p.Kind {
	case PropertyInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return "it takes a whole number" + p.unitSuffix()
		}
		if p.Min != 0 || p.Max != 0 {
			if n < p.Min || n > p.Max {
				return fmt.Sprintf("it takes a whole number%s from %d to %d",
					p.unitSuffix(), p.Min, p.Max)
			}
		}
	case PropertyChoice:
		for _, c := range p.Choices {
			if strings.EqualFold(raw, c) {
				return ""
			}
		}
		return "it takes one of: " + strings.Join(p.Choices, ", ")
	case PropertyBool:
		switch strings.ToLower(raw) {
		case "true", "false":
		default:
			return "it takes true or false"
		}
	case PropertySize:
		if _, err := core.ParseSize(raw); err != nil {
			return "it takes a size written the way any size is, such as 2mb or a plain byte count"
		}
	}
	return ""
}

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
		what = "a size such as 2mb, or a plain byte count"
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
			bad = append(bad, &UnknownPropertyError{Format: d.ID, Key: k, Known: d.PropertyNames()})
			continue
		}
		// An empty value means "not stated", the same as leaving the key out,
		// because that is what an unset flag and an empty recipe entry both
		// look like by the time they arrive here.
		if raw := props[k]; raw != "" {
			if why := p.Allows(raw); why != "" {
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
func (p Property) Instead() string {
	if p.Default != "" {
		return fmt.Sprintf("write a value it takes, such as %s, or leave the line out", p.Default)
	}
	return "write a value it takes, or leave the line out and the format works it out"
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
	Reason    string
	Hint      string
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
	return fmt.Sprintf("%s cannot be smaller than %d B. Requested: %d B", e.Format, e.Minimum, e.Requested)
}

func (e *BelowMinimumError) Why() string { return e.Reason }

func (e *BelowMinimumError) Instead() string { return e.Hint }

func (e *BelowMinimumError) Error() string {
	return fmt.Sprintf("%s cannot be smaller than %d B - %s. Requested: %d B. %s",
		e.Format, e.Minimum, e.Reason, e.Requested, e.Hint)
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

// UnknownFormatError is a request for a format nobody registered.
type UnknownFormatError struct {
	ID    string
	Known []string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("unknown format %q. Known formats: %v", e.ID, e.Known)
}
