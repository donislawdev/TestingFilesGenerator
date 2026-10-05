package core

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Said is a sentence for a person, kept as its parts until a surface says it.
//
// The engine refuses in English and the command line prints that English as it
// always has (D9). A window speaks the language somebody chose, and until
// 2026-10-05 it had nothing to translate: every refusal reached it as finished
// English text, so a Polish window showed Polish buttons over English refusals.
// docs/OKNO-PO-POLSKU-2026-10-05.md is the analysis.
//
// So a refusal is kept as three things. An id, which a window looks its own
// words up by. The layout, which is the fmt layout the sentence was always
// printed with, copied character for character - the command line renders it
// with fmt and the same values, which is why its bytes cannot move. And the
// values, each with a name, because a translation is written with named fields
// rather than printf verbs: a translator who writes %s where the code hands a
// number puts "%!d(string=...)" on a screen and nothing says a word, while a
// named field cannot be the wrong type. That is the owner's decision of
// 2026-08-26, see sayf in internal/gui/text.
//
// The setting slots of setting.go stay in the layout as they are. Nothing here
// fills them, so each refusal fills them exactly where it did before.
type Said struct {
	id     string
	layout string
	// one is the layout said of exactly one, for a sentence whose noun has a
	// number. Empty for a sentence without one.
	one  string
	args []Arg
	// noStop is a sentence said without the full stop it ends with, for a
	// reader that puts its own after it - see WithoutFullStop.
	noStop bool
	// capital is a sentence that starts with a capital letter where it stands -
	// see Capitalized.
	capital bool
}

// Capitalized is this sentence starting with a capital letter, in every
// language - it stands where a new sentence begins.
func (s Said) Capitalized() Said {
	s.capital = true
	return s
}

// Shaped is a sentence's text with the full stop and the capital this
// sentence was asked for, for whichever language the text is in.
func (s Said) Shaped(text string) string {
	if s.noStop {
		text = strings.TrimSuffix(text, ".")
	}
	if s.capital && text != "" {
		r, size := utf8.DecodeRuneInString(text)
		text = string(unicode.ToUpper(r)) + text[size:]
	}
	return text
}

// WithoutFullStop is this sentence without the full stop at its end, in every
// language - the place it goes puts a stop of its own after it.
func (s Said) WithoutFullStop() Said {
	s.noStop = true
	return s
}

// Stopless is whether the full stop at the end is left off.
func (s Said) Stopless() bool { return s.noStop }

// Arg is one value of a sentence, with the name a translation calls it by.
type Arg struct {
	Name  string
	Value any
}

// A is one named value of a sentence.
func A(name string, value any) Arg { return Arg{Name: name, Value: value} }

// CountArg is the name of the value that chooses between the forms of a
// sentence said with SaysN.
const CountArg = "Count"

// Says is a sentence with one form.
//
// The id and the layout are literals at the call site, and a guard reads them
// from there to write the English catalogue a translator copies - a sentence
// assembled from pieces is a sentence nobody can translate.
func Says(id, layout string, args ...Arg) Said {
	return Said{id: id, layout: layout, args: settled(layout, args)}
}

// SaysN is a sentence whose noun follows a number: one layout for exactly one
// and one for every other count, the way English has always printed it here.
// The number is the value named Count, and a translation carries as many forms
// as its language has.
func SaysN(id, one, other string, args ...Arg) Said {
	return Said{id: id, layout: other, one: one, args: settled(other, args)}
}

// settled renders, at once, every value that could change before the sentence
// is read.
//
// fmt.Sprintf renders when it is called, and a Said renders when it is read. A
// list or a pointer changed in between would print something other than the
// sentence the command line printed before Said existed - so anything that is
// not a plain value is rendered now, with the verb the layout gives it, which
// is the text it would have had. Numbers, strings, sentences and errors are
// kept, because a window says those in its own words.
func settled(layout string, args []Arg) []Arg {
	verbs := directivesOf(layout)
	out := make([]Arg, len(args))
	for i, a := range args {
		out[i] = a
		if keeps(a.Value) || i >= len(verbs) {
			continue
		}
		out[i].Value = fmt.Sprintf(verbs[i], a.Value)
	}
	return out
}

// keeps is whether a value can be held as it is until the sentence is read.
func keeps(v any) bool {
	switch v.(type) {
	case nil, Said, error, Bytes, Term, Choice, Choices, Joined, Sentences, Conjoined, Lines, fmt.Formatter, interface{ Said() Said }:
		return true
	}
	switch reflect.ValueOf(v).Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// ID is the key a window looks this sentence up by.
func (s Said) ID() string { return s.id }

// Layout is the English layout the command line renders, the form for exactly
// one when the count is one.
func (s Said) Layout() string {
	if s.one != "" && s.count() == 1 {
		return s.one
	}
	return s.layout
}

// Layouts is both forms of a sentence with a number, and the one form twice of
// a sentence without one - for the guard that writes the English catalogue.
func (s Said) Layouts() (one, other string) {
	if s.one == "" {
		return s.layout, s.layout
	}
	return s.one, s.layout
}

// Plural is whether a count chooses the form of this sentence.
func (s Said) Plural() bool { return s.one != "" }

// Args is the values, in the order the layout takes them.
func (s Said) Args() []Arg { return s.args }

// IsZero is whether nothing was said.
func (s Said) IsZero() bool { return s.id == "" && s.layout == "" }

// count is the value named Count, or -1 when there is none.
func (s Said) count() int64 {
	for _, a := range s.args {
		if a.Name == CountArg {
			return asCount(a.Value)
		}
	}
	return -1
}

func asCount(v any) int64 {
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return r.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(r.Uint())
	}
	return -1
}

// String is the sentence in English, with the setting slots left in it -
// exactly the text fmt made of the layout and the values before Said existed.
//
// The %w of a wrapping layout is printed the way fmt.Errorf prints it, as the
// error's own text.
func (s Said) String() string {
	values := make([]any, len(s.args))
	for i, a := range s.args {
		values[i] = a.Value
	}
	if s.countUnsaid() {
		values = values[:len(values)-1]
	}
	return s.Shaped(fmt.Sprintf(withoutWrapVerb(s.Layout()), values...))
}

// countUnsaid is whether the number that chooses the form is not printed -
// "holds" against "hold". It is then the last value, and the layout takes one
// value fewer than the sentence carries.
func (s Said) countUnsaid() bool {
	n := len(s.args)
	return s.one != "" && n > 0 && s.args[n-1].Name == CountArg && len(directivesOf(s.Layout())) == n-1
}

// Format lets a sentence be a value of another, rendered with the verb the
// outer layout gives it.
func (s Said) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), s.String())
}

// Bytes is a number of bytes said the way a person reads it - 10.0 MB.
//
// A type rather than core.HumanBytes at the call site, so a window can write
// the decimal mark of its own language where the command line writes a dot.
type Bytes int64

// Format prints the readable size for %s and %v, and the number for anything
// else.
func (b Bytes) Format(f fmt.State, verb rune) {
	if verb == 's' || verb == 'v' {
		fmt.Fprintf(f, fmt.FormatString(f, verb), HumanBytes(int64(b)))
		return
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), int64(b))
}

// SettingField is the field a translation names the setting by, where the
// English layout has SettingSlot. Reserved: no value of a sentence is called
// this.
const SettingField = "Setting"

// NamedLayout is a layout with every value written as the field a translation
// uses - {{.Format}} where the English has %s - and the setting slot as
// {{.Setting}}. The article slot goes: it is English grammar, and every other
// language that needs an article writes its own.
//
// This is what a translator copies, and what the pseudo language disguises.
// It is never what the command line prints - that is the layout itself.
func NamedLayout(layout string, names []string) string {
	layout = strings.ReplaceAll(layout, ArticleSlot+" ", "")
	layout = strings.ReplaceAll(layout, ArticleSlot, "")
	layout = strings.ReplaceAll(layout, SettingSlot, "{{."+SettingField+"}}")
	var b strings.Builder
	value := 0
	for i := 0; i < len(layout); i++ {
		if layout[i] != '%' {
			b.WriteByte(layout[i])
			continue
		}
		end := directiveEnd(layout, i+1)
		if end >= len(layout) {
			b.WriteString(layout[i:])
			break
		}
		if layout[end] == '%' {
			b.WriteByte('%')
		} else if value < len(names) {
			b.WriteString("{{." + names[value] + "}}")
			value++
		}
		i = end
	}
	return b.String()
}

// Names is the name of every value, in order.
func (s Said) Names() []string {
	out := make([]string, len(s.args))
	for i, a := range s.args {
		out[i] = a.Name
	}
	return out
}

// Directives is how the layout prints each value, in order - "%q", "%d".
func (s Said) Directives() []string { return directivesOf(s.Layout()) }

// DirectivesOf is how a layout prints each value, in order, for the guard that
// reads every layout out of the code.
func DirectivesOf(layout string) []string { return parseDirectives(layout) }

// Term is a word a registry declares - a unit, the shape of a text - as a value
// of a sentence. The command line prints the word as the registry spells it,
// and a window looks it up under Key, the key its catalogue keeps the word by.
type Term struct {
	Key  string
	Text string
}

// Format prints the word, with the verb the layout gives it.
func (t Term) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), t.Text)
}

// LabelTerm is the name of a declared setting - "width" - which a window shows
// as the label above its box.
func LabelTerm(name string) Term { return Term{Key: LabelKey(name), Text: name} }

// LabelKey and JointKey are two more of the keys a window keeps words under -
// see UnitKey. FormatOwner is whose declaration a format's words are.
func LabelKey(name string) string          { return "Label." + name }
func JointKey(owner, of, by string) string { return "Joint." + owner + "." + of + "." + by }
func FormatOwner(id string) string         { return "format/" + id }

// UnitTerm is the word for what a number counts - "pixels".
func UnitTerm(unit string) Term { return Term{Key: UnitKey(unit), Text: unit} }

// UnitKey and ChoiceKey are the keys a window's catalogue keeps a unit and a
// value of a list under. Here rather than in the window, so the sentence that
// carries the word and the catalogue that translates it cannot come to spell
// the key two ways.
func UnitKey(unit string) string        { return "Unit." + unit }
func ChoiceKey(of, value string) string { return "Choice." + of + "." + value }

// KindKey is the key of the word for what a tool works on - file, folder.
func KindKey(kind string) string { return "Kind." + kind }

// Choice is a value of a closed list, as a value of a sentence.
//
// The command line prints the value as the layout says - "accept", quoted
// where the layout quotes. A window shows a list's values under names in its
// own language, and a sentence naming one has to call it what the list calls
// it, without the quotes of a value somebody typed (the owner's decision of
// 2026-10-05). Of is the setting whose list it is, by its recipe key.
type Choice struct {
	Of    string
	Value string
}

// Format prints the value, with the verb the layout gives it.
func (c Choice) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), c.Value)
}

// Joined is names in a sentence joined the way English joins them - "a, b and
// c", "a or b" - which a window joins with its own words.
type Joined struct {
	Items []string
	// And joins the last two with "and", and false with "or".
	And bool
}

// Format prints the names joined, with the verb the layout gives the whole.
func (j Joined) Format(f fmt.State, verb rune) {
	word := "or"
	if j.And {
		word = "and"
	}
	joined := ""
	switch len(j.Items) {
	case 0:
	case 1:
		joined = j.Items[0]
	default:
		joined = strings.Join(j.Items[:len(j.Items)-1], ", ") + " " + word + " " + j.Items[len(j.Items)-1]
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), joined)
}

// Conjoined is several sentences joined with "and" - "2 PDF files of 10 B and
// 1 TXT file of 4 B" - which a window joins the way its language does.
type Conjoined []Said

// Format prints the sentences in English, each pair joined with "and".
func (c Conjoined) Format(f fmt.State, verb rune) {
	parts := make([]string, len(c))
	for i, s := range c {
		parts[i] = s.String()
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), strings.Join(parts, " and "))
}

// Sentences is several sentences said as a list, one after another with a
// comma between them.
type Sentences []Said

// Format prints the sentences in English, joined with commas.
func (l Sentences) Format(f fmt.State, verb rune) {
	parts := make([]string, len(l))
	for i, s := range l {
		parts[i] = s.String()
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), strings.Join(parts, ", "))
}

// Lines is several sentences said one under another, each on a line of its own
// and indented by two spaces after the first.
type Lines []Said

// Format prints the sentences in English, one a line.
func (l Lines) Format(f fmt.State, verb rune) {
	parts := make([]string, len(l))
	for i, s := range l {
		parts[i] = s.String()
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), strings.Join(parts, "\n  "))
}

// Choices is several values of one closed list, said as a list - "comma, tab,
// semicolon" - for a sentence that names what a list offers.
type Choices struct {
	Of     string
	Values []string
}

// Format prints the values joined the way the command line always joined
// them.
func (c Choices) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), strings.Join(c.Values, ", "))
}

// directivesOf is every value directive of a layout, in the order the values
// are taken - "%q", "%d", "% x". A literal %% takes no value and is left out.
//
// Kept once per layout: a note is said for every file of a run, from a handful
// of layouts, and parsing one again for each of a million files would be work
// done a million times to learn one answer.
func directivesOf(layout string) []string {
	if kept, ok := parsedLayouts.Load(layout); ok {
		return kept.([]string)
	}
	verbs := parseDirectives(layout)
	parsedLayouts.Store(layout, verbs)
	return verbs
}

var parsedLayouts sync.Map

// parseDirectives reads a layout the way fmt does: a per cent sign, then flags,
// a width, a precision, then one verb letter. Widths taken from a value (*) and
// explicit positions ([2]) are refused by the guard over every layout, so a
// directive here is always one value.
func parseDirectives(layout string) []string {
	var out []string
	for i := 0; i < len(layout); i++ {
		if layout[i] != '%' {
			continue
		}
		end := directiveEnd(layout, i+1)
		if end >= len(layout) {
			break
		}
		if layout[end] != '%' {
			out = append(out, layout[i:end+1])
		}
		i = end
	}
	return out
}

// directiveEnd is where the verb of the directive starting after a per cent
// sign stands.
func directiveEnd(layout string, from int) int {
	j := from
	for j < len(layout) && strings.ContainsRune("+-# 0123456789.", rune(layout[j])) {
		j++
	}
	return j
}

// withoutWrapVerb is a layout with %w spelled %v, which is what fmt.Errorf
// prints for it and what fmt.Sprintf would otherwise print as %!w(...).
func withoutWrapVerb(layout string) string {
	if !strings.Contains(layout, "%w") {
		return layout
	}
	var b strings.Builder
	for i := 0; i < len(layout); i++ {
		b.WriteByte(layout[i])
		if layout[i] != '%' {
			continue
		}
		end := directiveEnd(layout, i+1)
		if end >= len(layout) {
			break
		}
		b.WriteString(layout[i+1 : end])
		if layout[end] == 'w' {
			b.WriteByte('v')
		} else {
			b.WriteByte(layout[end])
		}
		i = end
	}
	return b.String()
}
