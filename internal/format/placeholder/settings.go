package placeholder

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/placeholder/glyphs"
)

// Setting names, public under rule 10. foreground rather than text_color: it
// pairs with background, it colours the guides as well as the words, and it
// keeps out of the colour and color spelling the manifest already has one of
// (colour_type).
const (
	SettingContent    = "content"
	SettingText       = "text"
	SettingBackground = "background"
	SettingForeground = "foreground"
	SettingGuides     = "guides"
)

// The two values of content. test is what every picture format drew before
// placeholders existed, to the byte (D11), and stays the default.
const (
	ContentTest        = "test"
	ContentPlaceholder = "placeholder"
)

// The tokens text understands. {index:04} is spelled as in a name template, so
// a file called hero_0007.png can say 0007 in its picture.
const (
	tokenIndex       = "{index}"
	tokenIndexPadded = "{index:04}"
	tokenWidth       = "{width}"
	tokenHeight      = "{height}"
)

// The defaults. The text is the size with a multiplication sign between, as
// people write it - U+00D7, spelled in bytes so this file stays ASCII.
const (
	DefaultText       = tokenWidth + " \xc3\x97 " + tokenHeight
	DefaultBackground = "cccccc"
	DefaultForeground = "555555"
)

// MaxText is how many characters text may have. One line on a picture, so a
// hundred is more than any layout uses - the ceiling exists because the value
// is typed by somebody and drawn on every file (rule 23).
const MaxText = 100

// shapeColour is what a colour has to look like, said once for both colours -
// the site translates it as one term.
const shapeColour = "a colour such as cccccc or ccc"

// Group is the heading the settings stand under on both surfaces.
const Group = "Placeholder"

// Properties declares the five settings. One declaration for every picture
// format, so the names and what they take cannot differ between them.
func Properties() []format.Property {
	return []format.Property{
		{
			Name: SettingContent, Kind: format.PropertyChoice,
			Choices: []string{ContentTest, ContentPlaceholder},
			Default: ContentTest,
			Detail:  "What the picture shows. test is a pattern for checking how a file is read. placeholder is a flat colour with words in the middle, for laying out a page before the real pictures exist.",
			Group:   Group,
		},
		{
			Name: SettingText, Kind: format.PropertyText,
			Shape:   fmt.Sprintf("one line of up to %d characters", MaxText),
			Default: DefaultText,
			Detail:  "The words in the middle of a placeholder. {width} and {height} become the size of the picture, {index} the number of the file and {index:04} the same number with four digits.",
			Group:   Group,
		},
		{
			Name: SettingBackground, Kind: format.PropertyText,
			Shape:   shapeColour,
			Default: DefaultBackground,
			Detail:  "The colour of a placeholder, such as cccccc or ccc. In a recipe write it without # or in quotes, because # starts a comment there.",
			Group:   Group,
		},
		{
			Name: SettingForeground, Kind: format.PropertyText,
			Shape:   shapeColour,
			Default: DefaultForeground,
			Detail:  "The colour of the words and the guides on a placeholder, written the same way as background.",
			Group:   Group,
		},
		{
			Name: SettingGuides, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Draws a frame along the edges of a placeholder, both diagonals and a circle touching its shorter sides, so you can see where a page crops or stretches the picture.",
			Group:   Group,
		},
	}
}

// Settings is what the five settings asked for, read and checked.
type Settings struct {
	Content string
	// Text is the template, tokens and all. Expand puts a file's numbers in.
	Text       string
	Background color.RGBA
	Foreground color.RGBA
	Guides     bool
}

// Placeholder reports whether the picture is a placeholder rather than the
// test pattern.
func (s Settings) Placeholder() bool { return s.Content == ContentPlaceholder }

// Read is the five settings of one request, each checked and with its default
// where it is left out. owner is the format that refuses a value.
//
// The declaration checks what it can say on its own - the two words content
// takes, true or false. The rest is checked here because it is more than a
// shape: whether the typeface has the letters, whether a token exists, whether
// a setting that changes nothing was set. The engine asks the declaration and
// then the plan, so a recipe gets both before any file exists.
func Read(owner string, props map[string]string) (Settings, error) {
	s := Settings{Content: ContentTest, Text: DefaultText}
	if v := props[SettingContent]; v != "" {
		if err := allowed(owner, SettingContent, v); err != nil {
			return Settings{}, err
		}
		s.Content = v
	}
	if !s.Placeholder() {
		return s, nothingToChange(owner, props)
	}
	if v := props[SettingText]; v != "" {
		if err := checkText(owner, v); err != nil {
			return Settings{}, err
		}
		s.Text = v
	}
	var err error
	if s.Background, err = colourOf(owner, SettingBackground, DefaultBackground, props); err != nil {
		return Settings{}, err
	}
	if s.Foreground, err = colourOf(owner, SettingForeground, DefaultForeground, props); err != nil {
		return Settings{}, err
	}
	if v := props[SettingGuides]; v != "" {
		if err := allowed(owner, SettingGuides, v); err != nil {
			return Settings{}, err
		}
		s.Guides = v == "true"
	}
	return s, nil
}

// allowed asks the declaration of one setting, for a caller that did not come
// through the registry - a guard, or a format reading its own request.
func allowed(owner, name, v string) error {
	for _, p := range Properties() {
		if p.Name == name {
			if bad := p.Allows(v); !bad.IsZero() {
				return &format.PropertyValueError{Format: owner, Key: name, Value: v, Reason: bad, Remedy: p.Instead()}
			}
			return nil
		}
	}
	return core.Defect(fmt.Errorf("placeholder: %q is not one of its settings", name))
}

// nothingToChange refuses a placeholder setting on a test picture. It would be
// read, accepted and ignored, and somebody would wait for a colour that never
// comes - a setting that does nothing has to say so.
//
// guides is refused only when it asks for guides. A window sends every switch
// in the position it shows (parts.boolField), so guides=false arrives with
// every picture made there, and it asks for exactly what a test picture has.
func nothingToChange(owner string, props map[string]string) error {
	for _, name := range []string{SettingText, SettingBackground, SettingForeground, SettingGuides} {
		v, ok := props[name]
		if ok && !(name == SettingGuides && v == "false") {
			return &format.PropertyValueError{
				Format: owner, Key: name, Value: v,
				Reason: core.Says("placeholder.OnlyOnAPlaceholder", "it only changes a placeholder, and content is test"),
				Remedy: core.Says("placeholder.SetContentPlaceholder", "Set content to placeholder, or leave the line out"),
			}
		}
	}
	return nil
}

// colourOf reads one colour, or its default when it is left out.
//
// Present and empty is refused rather than read as left out, which is what
// every other setting does with it. A colour is the one value people write
// with a # in front, and in a recipe that is a comment: background: #ff0000
// arrives as nothing, and a grey picture with no word of why is exactly the
// silence rule 6 forbids. Measured on 2026-11-14 with a text setting, which
// took its default the same way. A window never sends an empty box.
func colourOf(owner, name, fallback string, props map[string]string) (color.RGBA, error) {
	v, ok := props[name]
	if !ok {
		c, _ := parseColour(fallback)
		return c, nil
	}
	if v == "" {
		return color.RGBA{}, &format.PropertyValueError{
			Format: owner, Key: name, Value: v,
			Reason: core.Says("placeholder.ColourIsEmpty", "it is empty, and in a recipe # starts a comment, so a colour written after it is not read"),
			Remedy: core.Says("placeholder.WriteTheColourWithoutHash", "Write the colour without the # or in quotes, such as %s: ff0000 or %s: \"#ff0000\"", core.A("Name", name), core.A("Name2", name)),
		}
	}
	c, good := parseColour(v)
	if !good {
		return color.RGBA{}, &format.PropertyValueError{
			Format: owner, Key: name, Value: v,
			Reason: core.Says("placeholder.NotAColour", "it has to be a colour written as six hex digits such as cccccc, or three such as ccc"),
			Remedy: core.Says("placeholder.WriteAColourSuchAs", "Write a colour such as %s, with or without a # in front", core.A("Default", fallback)),
		}
	}
	return c, nil
}

// parseColour reads rrggbb or rgb, with one # in front or none, in either
// case. Three digits mean each one twice, as on the web: ccc is cccccc.
func parseColour(v string) (color.RGBA, bool) {
	v = strings.TrimPrefix(v, "#")
	if len(v) == 3 {
		v = string([]byte{v[0], v[0], v[1], v[1], v[2], v[2]})
	}
	if len(v) != 6 {
		return color.RGBA{}, false
	}
	n, err := strconv.ParseUint(v, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}, true
}

// Hex is a colour as the manifest records it: six lower case digits, whatever
// spelling the recipe used.
func Hex(c color.RGBA) string { return fmt.Sprintf("%02x%02x%02x", c.R, c.G, c.B) }

// checkText refuses a text that is too long, names a token that does not
// exist, or has a character the typeface cannot draw.
func checkText(owner, v string) error {
	refuse := func(reason, remedy core.Said) error {
		return &format.PropertyValueError{Format: owner, Key: SettingText, Value: v, Reason: reason, Remedy: remedy}
	}
	if n := utf8.RuneCountInString(v); n > MaxText {
		return refuse(core.Says("placeholder.TextTooLong", "it has %d characters and a placeholder takes up to %d", core.A("Count", n), core.A("Most", MaxText)),
			core.Says("placeholder.ShortenTheText", "Write a shorter text, one line that fits across the picture"))
	}
	literal, unknown := withoutTokens(v)
	if unknown != "" {
		return refuse(core.Says("placeholder.UnknownToken", "it has %s, which is not a token this build understands", core.A("Token", unknown)),
			core.Says("placeholder.TheTokensAre", "The tokens are {width}, {height}, {index} and {index:04}"))
	}
	if missing := glyphs.Inter().Missing(literal); len(missing) > 0 {
		return refuse(core.Says("placeholder.NoLetterFor", "the typeface has no letter for %s", core.A("Characters", runeList(missing))),
			core.Says("placeholder.UseLettersItHas", "Use the Latin, Greek or Cyrillic alphabet, digits and common punctuation, all on one line"))
	}
	return nil
}

// withoutTokens is the text with every token taken out, and the first brace
// that does not open a token, from that brace to the one closing it or to the
// end. A brace left in would be drawn as written, and somebody would get a
// picture saying {idx} rather than the number they meant - the same reason a
// name template refuses one.
func withoutTokens(v string) (literal, unknown string) {
	for _, t := range []string{tokenIndexPadded, tokenIndex, tokenWidth, tokenHeight} {
		v = strings.ReplaceAll(v, t, "")
	}
	open := strings.IndexAny(v, "{}")
	if open < 0 {
		return v, ""
	}
	end := strings.IndexByte(v[open:], '}')
	if end < 0 || v[open] == '}' {
		return v, v[open:min(len(v), open+20)]
	}
	return v, v[open : open+end+1]
}

// runeList names characters for a refusal: each one quoted, with its code
// point, because the ones the typeface lacks are often ones nobody can see -
// a tab, a zero width space. The first five, and an ellipsis for the rest,
// which needs no word in any language.
func runeList(rs []rune) string {
	const most = 5
	parts := make([]string, 0, most+1)
	for i, r := range rs {
		if i == most {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, fmt.Sprintf("%q (U+%04X)", r, r))
	}
	return strings.Join(parts, ", ")
}

// Expand is the text of one file: number is the file's number in its target,
// counted from one as its name counts it.
func (s Settings) Expand(number, width, height int) string {
	return strings.NewReplacer(
		tokenIndexPadded, fmt.Sprintf("%04d", number),
		tokenIndex, strconv.Itoa(number),
		tokenWidth, strconv.Itoa(width),
		tokenHeight, strconv.Itoa(height),
	).Replace(s.Text)
}

// Look is what one file of these settings shows.
func (s Settings) Look(text, label string) Look {
	return Look{Text: text, Label: label, Background: s.Background, Foreground: s.Foreground, Guides: s.Guides}
}

// Describe adds the settings to what reaches the manifest. content always, so
// a test reads the same key from every picture. The rest only for a
// placeholder, where they describe what is in it: the text as drawn, the
// colours as six digits, and whether the text and the label fitted.
func (s Settings) Describe(props map[string]any, text string, drawn Drawn) {
	props[SettingContent] = s.Content
	if !s.Placeholder() {
		return
	}
	props[SettingText] = text
	props[SettingBackground] = Hex(s.Background)
	props[SettingForeground] = Hex(s.Foreground)
	props[SettingGuides] = s.Guides
	props["text_drawn"] = drawn.Text
}

// Claim is what a plan says about a placeholder of width by height: whether
// it carries the label, and a note for the text and for the label each that
// did not fit. Neither is squeezed below what can be read, and neither is left
// out in silence (rule 6). One place for every picture format, so the words
// cannot come apart between them.
func Claim(drawn Drawn, label string, width, height int, notes []format.Note) (bool, []format.Note) {
	if !drawn.Text {
		notes = append(notes, format.Note{
			Code: "text_omitted",
			Detail: core.Says("placeholder.TextOmitted", "The picture is %d by %d px and its text needs more room, so this placeholder carries no text.",
				core.A("Width", width), core.A("Height", height)),
		})
	}
	if label != "" && !drawn.Label {
		notes = append(notes, format.Note{
			Code: "label_omitted",
			Detail: core.Says("placeholder.LabelOmitted", "The picture is %d by %d px and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.",
				core.A("Width", width), core.A("Height", height)),
		})
	}
	return drawn.Label, notes
}
