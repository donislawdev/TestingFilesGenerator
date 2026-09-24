// Package parts holds the pieces every window is built from.
//
// It exists because of what this interface is going to become rather than what
// it is today: many windows, many fields, many buttons, the same shapes over
// and over. A window that builds its own labelled entry is a window that will
// word its own error, size its own gap and validate its own value - and the
// third of those breaks G1 in the way nobody sees, because a form with its own
// rule is a second copy of rules the engine already owns.
//
// Two properties hold this package together.
//
// It knows nothing about windows. Nothing here opens, closes or navigates, so
// a part can be rendered on its own - which is what lets the golden images sit
// on parts rather than on whole screens. An image of a whole window changes
// with every layout change and stops being read after the third time. An image
// of one field in four states is stable and says something.
//
// It never reaches the toolkit's app package. Everything here builds a widget
// tree and nothing drives one, so this package compiles and tests with
// CGO_ENABLED=0, on a runner with no graphics and no C compiler.
package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Prose renders a block of text that a person reads rather than edits.
//
// Wrapping rather than truncating, and that is G9 rather than taste. An error
// in this tool has four parts - what happened, why, what is allowed, what to do
// instead - and a widget that shows one line forces a message that has one of
// the four. The rule in docs/GUI.md is a requirement on the layout, so it is
// answered here once instead of in every window that shows a sentence.
func Prose(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return inkTight(label)
}

// Heading is the name of one field, beside its control.
//
// Regular weight since 2026-09-14. Bold, at the same size as the value in the
// box beside it, every name on the form was as loud as every value, so a
// screen had twice as much heavy text as it had content and the eye had
// nothing to skip. One rank is bold on a screen now - the title of a section
// - and a name is read by where it stands: in the column of names, level with
// its box.
func Heading(text string) fyne.CanvasObject {
	return words(text, TextBody, false, ColorNameLabel)
}

// Subheading names a block inside a section - the list of what a preset finds,
// the table of files inside an archive - at the size of a field's name and
// the weight of a section's. The one bold thing at body size, so it is read as
// a heading of the things under it and not as the name of a box beside it.
func Subheading(text string) fyne.CanvasObject {
	return oneLine(text, theme.SizeNameText)
}

// Title is the one line that says what a screen is for.
//
// Bigger than a field's name rather than the same size in the same weight,
// which is what it was until 2026-08-11: the title of the screen and the label
// of every field were one style, so nothing led the eye and the first point of
// the UX section 7 checklist - squint, and see what stands out - had no answer.
func Title(text string) fyne.CanvasObject {
	return oneLine(text, theme.SizeNameSubHeadingText)
}

// oneLine is a bold line that stays one line: too long for its room, it ends
// in an ellipsis at the room's edge instead of running past it.
//
// A toolkit label since 2026-09-16 rather than canvas words, for one reason.
// A canvas text is one line however much room it has, and a title and a
// block's name are the two ranks with no column worked out for them - a
// name has room by construction, because the column of names is measured
// from every name the window can show, and a sentence wraps. The catalogue's
// long line state showed a title 60 px outside its panel (O214), which no
// screen shows today and a translation could. A label given
// TextTruncateEllipsis draws what fits and says that it cut.
//
// Two things keep this honest rather than a way to hide text. A guard asks
// every title and block name on every screen, at the largest first opening,
// whether it fits whole - so an ellipsis on a real screen is red in CI, and
// the ellipsis is the state of the control for a window somebody has made
// narrower than that, not for the words we ship. And the catalogue shows the
// cut state, so it is looked at rather than assumed.
//
// The size is asked for by the theme's name for it rather than as a number,
// because a label sizes itself from the theme, and this window's theme maps
// SizeNameSubHeadingText to TextTitle and SizeNameText to TextBody - see
// ours.Size for why the two heading names are the other way round from what
// they sound like. Ink tight like every other label here, so it stands on the
// scale.
func oneLine(text string, size fyne.ThemeSizeName) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.SizeName = size
	label.Wrapping = fyne.TextWrapOff
	label.Truncation = fyne.TextTruncateEllipsis
	return inkTight(label)
}

// Subtitle is the one quiet sentence under a screen's title, saying what the
// screen is for.
//
// It exists because every work screen had two names until 2026-09-15: the word
// on its tab and a title that said something else - "Single batch" over a
// screen headed "Generate files". One vocabulary now: the tab's word is the
// title, and what the title used to say is this sentence, in the colour of a
// hint rather than of a value, so it reads as an explanation of the word above
// it and not as a second heading competing with it.
//
// Wrapping words rather than canvas words, because a sentence wraps and a
// name does not - the same split words and Prose make. Its colour is the
// hint's, the same step the words on the strip stand at when they are not
// chosen, so the head of a screen has one quiet colour and not two. QuietText,
// because the toolkit draws a low importance label in the DISABLED colour, and
// this palette keeps that a step brighter than a hint on purpose - see
// ColorNameDisabled in theme.go.
func Subtitle(sentence string) fyne.CanvasObject {
	subtitle := NewQuietText(sentence)
	subtitle.Wrapping = fyne.TextWrapWord
	return subtitle
}

// Caption is a short quiet line over or under a thing, at the smallest rank
// of the scale: the name of a state in the catalogue, the count of bytes
// beside a size. The rank ByteCount has drawn since 2026-08-19, given a name
// on 2026-09-15 when a second thing needed it - a rank used twice without a
// name is GUI rule 7's definition of a missing style.
func Caption(text string) fyne.CanvasObject {
	caption := NewQuietText(text)
	caption.Wrapping = fyne.TextWrapWord
	caption.SizeName = theme.SizeNameCaptionText
	return caption
}

// Titled is what stands above a screen's sections: its name, and under it the
// sentence saying what it is for. The two are one step apart, the step a
// field's name keeps from its box, because they are one thing read together.
func Titled(name, sentence string) fyne.CanvasObject {
	return Column(GapLabel, Title(name), Subtitle(sentence))
}

// Section groups fields that answer one question, under a name.
//
// It draws its own surface rather than using widget.Card, and that is a
// correction rather than a preference. Measured on 2026-08-12: a card fills
// itself with ColorNameBackground - card.go line 44 - which is the name the
// page uses, so every section came out at exactly the page colour. Zero L*
// apart, with nothing but a shadow at its edge going darker than the page it
// sat on. The grouping this was introduced for did not exist, and no palette
// value could have supplied it, because the toolkit has no name for a panel.
//
// What replaces the card is three ordinary pieces rather than a widget of our
// own: a rectangle behind, padded content in front. That keeps a section a
// plain container, so anything that walks the tree already knows what it is -
// and a walk that does not know one type stops seeing every field below it,
// which is exactly what happened when cards arrived.
//
// The content is a Grid since the prototype of 2026-09-23: fields in
// GridColumns columns, each taking the fewest columns that hold its value,
// and anything else across the row.
func Section(title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(panelSurface(), Padded(Inset, Column(GapField, sectionTitle(title), Grid(content...))))
}

// FieldColumn stacks fields the way a section stacks them, for the boxes a screen
// refills at run time.
//
// The settings a format declares and the parameters a preset declares arrive
// after the screen is built, into a box of their own. Left as plain vertical
// boxes those fields sat at the toolkit's padding while every field around
// them sat at GapField - one rhythm above the box and another inside it, on
// the same screen.
func FieldColumn(children ...fyne.CanvasObject) *fyne.Container {
	return Column(GapField, children...)
}

// SettingsHeading is gone as of 2026-08-25. The block of fields a chosen
// format declares is a fold now, and a fold draws its own title at the rank
// this gave it - see NewInnerFolding. The reason it existed still holds and is
// worth keeping written down: until 2026-08-20 "settings for bmp" was a field
// label in bold, the same size and weight as the name of the box under it, so
// a heading and the thing it heads were drawn identically and the block had no
// top edge.

// sectionTitle names a section, at the rank between the screen and a field.
//
// The size the card used to draw it at, kept deliberately: the scale is four
// ranks and this is the second, so moving off the toolkit's widget must not
// quietly move the type with it.
func sectionTitle(text string) fyne.CanvasObject {
	return words(text, TextHeading, true, theme.ColorNameForeground)
}

// panelSurface is what a section and the action bar stand on.
//
// The colours come from the palette directly rather than through the installed
// theme, and that is what "one look" means here: this program answers dark
// whatever the desktop says, so a surface asking the theme what variant is in
// force would be asking a question that has one answer. It is also the same
// function the guard measures, so the picture and the measurement cannot come
// apart.
// It has no border, and that is the point rather than a simplification. A
// border was drawn here and around every box to type in, at one pixel in a
// near enough colour - so the one mark this form uses to say "your value goes
// here" was also the mark it used to say "these things belong together", and a
// mark that means two things means neither. The surface still groups, by being
// a surface. The border belongs to the fields now.
func panelSurface() *canvas.Rectangle {
	rect := canvas.NewRectangle(PaletteColour(ColorNamePanel, theme.VariantDark))
	rect.CornerRadius = RadiusPanel
	// A line round the edge again, owner's decision of 2026-09-21 after the
	// running window: the fill alone did not say where a section ends.
	rect.StrokeColor = PaletteColour(theme.ColorNameSeparator, theme.VariantDark)
	rect.StrokeWidth = edgeWidth
	return rect
}

// floatingSurface is what anything drawn OVER the form stands on: the list a
// menu drops down, and the explanation behind a field's button.
//
// One function for both since 2026-09-21, and the second of them is why. The
// explanation stood on panelSurface until then, and it opens over a section -
// so a box the colour of the thing under it had no edge anywhere, and the
// owner's report from the running window was a sentence laid straight over
// the form, covering the row beneath. The list had already met the same
// question on 2026-08-12 and the palette answers it: the surface that floats
// is the lightest one, told from a panel by 13.6 L* with no border and no
// shadow (theme.go, ColorNameMenuBackground). The corner is a field's, not a
// panel's, because what floats is the size of a control and not of a section.
func floatingSurface() *canvas.Rectangle {
	rect := canvas.NewRectangle(PaletteColour(theme.ColorNameMenuBackground, theme.VariantDark))
	rect.CornerRadius = RadiusField
	return rect
}

// tipSurface is what an explanation stands on: the floating surface with a
// line round it, so it reads as a thing laid over the form and not as a
// patch of it. Owner's report from the running window, 2026-09-21: without
// the line it looked like a random rectangle.
func tipSurface() *canvas.Rectangle {
	rect := floatingSurface()
	rect.StrokeColor = PaletteColour(theme.ColorNameInputBorder, theme.VariantDark)
	rect.StrokeWidth = edgeWidth
	return rect
}

// tipShadow is the shade an explanation casts, offset downwards so it reads
// as depth rather than as a smudge - the same reason Refactoring UI gives
// for offsetting shadows. Drawn under tipSurface in a stack, so it shows
// only past the surface's lower edge.
func tipShadow() fyne.CanvasObject {
	rect := canvas.NewRectangle(PaletteColour(ColorNameTipShade, theme.VariantDark))
	rect.CornerRadius = RadiusField
	return container.New(shifted{dy: TipShadowDrop}, rect)
}

// shifted lays its one child at an offset from its own origin.
type shifted struct{ dx, dy float32 }

func (shifted) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (s shifted) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(s.dx, s.dy))
		o.Resize(size)
	}
}

// Bullets is a list of short statements, drawn as a list.
//
// It replaces a run of labels each starting with a dash typed into the string.
// That was a list in the way a paragraph is a list: the marker was text, so it
// sat on the same baseline as the words and wrapped with them, and each item
// was a separate label carrying a label's full spacing - which put more air
// between the items than between the list and the things around it.
//
// The marker is its own column, so a wrapped item hangs under its own text
// rather than under the marker. These items wrap: one of them is a sentence
// about MB against MiB that runs past the width of this card.
//
// At the size and colour of ordinary text since the prototype of 2026-09-23.
// It was drawn as a caption - the smallest rank, in the hint's colour - and
// measured in the real window that made the most useful words on two screens
// the faintest: what a preset typically finds, and how to use the program,
// under a licence drawn in full white. The marker stands level with the FIRST
// line of its item rather than in the middle of all of them (O235).
func Bullets(items []string) fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(items))
	for _, item := range items {
		marker := words(bulletMarker, TextBody, false, ColorNameLabel)
		rows = append(rows, container.New(hanging{}, marker, Prose(item)))
	}
	// Tight, because these items are one list rather than a run of separate
	// statements. Measured on 2026-08-20 at the toolkit's padding: 35 px
	// between items carrying 12 px of text, which is nearly three times the
	// type and reads as loose beside the form next to it.
	return Column(GapTight, rows...)
}

// bulletMarker is what stands in front of one item.
//
// Not a hyphen. D17 keeps the flat hyphen for prose because an en dash is the
// thing being banned, and this is not prose - it is the marker of a list, where
// a hyphen reads as a word that lost its other half.
const bulletMarker = "•"

// Row puts fields side by side, for the ones that are read together.
//
// Size and how many are one thought, and so are the id and the name template.
// Stacked, each took a full width it did not need and pushed the next one off
// the screen.
func Row(fields ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(columns{gap: GapColumns}, fields...)
}

// columns shares a row out in equal columns with one gap from the scale
// between them. The toolkit's grid does the same with the theme's padding,
// which is the smallest step - and two fields side by side are two things,
// not one thing and its caption.
type columns struct{ gap float32 }

func (c columns) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	shown := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		size.Width = fyne.Max(size.Width, min.Width)
		size.Height = fyne.Max(size.Height, min.Height)
		shown++
	}
	if shown > 0 {
		size.Width = size.Width*float32(shown) + c.gap*float32(shown-1)
	}
	return size
}

func (c columns) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	shown := 0
	for _, o := range objects {
		if o.Visible() {
			shown++
		}
	}
	if shown == 0 {
		return
	}
	width := (size.Width - c.gap*float32(shown-1)) / float32(shown)
	x := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		o.Resize(fyne.NewSize(width, size.Height))
		o.Move(fyne.NewPos(x, 0))
		x += width + c.gap
	}
}

// BesideFields puts something that is not a field into a row of them without
// letting the row decide how big it is.
//
// Row shares the width out in equal columns, which is what fields want and what
// anything else gets whether it wants it or not. Measured on 2026-08-28, from
// the owner reading the batch screen: the Remove button ending a row of an
// archive's contents came out 198x63 px for a word needing 63x32, so it was
// drawn as a panel with a word in the middle of it rather than as a button. The
// Duplicate button at the head of the same batch, which is in no row, is 79x35.
//
// Two layouts and each answers one half. The box across gives it the width it
// asks for instead of the column's. The spacers above and below hold it level
// with the middle of the controls beside it. Until 2026-09-16 there was one
// spacer, above, pushing it down onto the line the controls stood on - a cell
// was then a name with a control under it, so anything at the top of that
// column lined up with the names and read as a heading of its own. The names
// stand over the table once now (Table.Header), a cell is its control,
// and a button three pixels taller than a box beside it is centred on it.
func BesideFields(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(layout.NewSpacer(), container.NewHBox(o), layout.NewSpacer())
}

// Divider is a line between two things standing side by side.
//
// The rail at the left of the action bar carries Donate and, on the batch
// screen, Add a batch. Side by side in the same style they read as a pair, and
// they are the two least related buttons in the window: one adds to the form in
// front of you, the other hands an address to your browser. Reported in the
// design audit of 2026-08-20 and answered here rather than by moving the
// button, because a guard already keeps Add a batch reachable without
// scrolling and putting it under the last batch would break exactly that.
//
// ROOM rather than a line, since 2026-08-24. The line was reported from the
// screen as something the eye keeps landing on, and it was doing a job that
// distance does on its own - the guard behind this has always asked for a gap
// wider than the one between two buttons that DO belong together, and never for
// anything drawn. So the protection stays exactly as strong and the mark goes.
//
// The width is unchanged, which is the point: nothing moved, one rectangle
// stopped being painted.
func Divider() fyne.CanvasObject {
	return container.New(dividerLayout{})
}

// dividerLayout is the room a boundary takes. It held a one pixel line until
// 2026-08-24 and now holds nothing, so the name is about what it separates.
type dividerLayout struct{}

func (dividerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(GapSection, 0)
}

func (dividerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// Nothing to lay out since the line went, and the loop stays because the
	// layout is what owns the width either way.
	for _, o := range objects {
		o.Resize(fyne.NewSize(1, size.Height))
		o.Move(fyne.NewPos(GapSection/2, 0))
	}
}

// ActionBar is the strip that stays put while the form scrolls under it.
//
// On a surface of its own rather than floating, and that is not decoration:
// pinned over a transparent background the scrolling text ran underneath the
// buttons and through their labels. It uses the same surface as a section, so
// the fix for an invisible card fixed this at the same time - the bar had been
// drawing itself in the page colour too, which is to say not drawing itself.
//
// The surface runs the whole width and what stands on it does not, which is
// the one place those two differ on purpose. A bar pinned across the foot is
// what makes it read as a bar, and the buttons and the run's own messages line
// up with the form above them instead of starting where the window happens to
// begin. Until 2026-08-12 they did the latter: the form stopped at 822 px and
// a refusal about it ran to 1099.
//
// The rail stands in that column too, on its left edge, since 2026-09-24. It
// stood at the left edge of the bar from the owner's decision of 2026-08-19 -
// what it holds is what the run is not about, Donate and adding a batch, and
// lining it up with the form seemed to buy nothing. Looked at again in the
// review of 2026-09-24 (UI-006), the bar had four left edges - Donate at the
// window's, Add a batch beside it, the buttons centred and the line under them
// on the form's - and the owner reversed the decision knowing it: one edge for
// everything that is not centred. Pass nil on a screen that has none.
func ActionBar(rail fyne.CanvasObject, content ...fyne.CanvasObject) fyne.CanvasObject {
	// The padding goes inside the column as well as around the bar, and that is
	// what puts the bar's own words on the same left edge as the form's.
	//
	// Measured on 2026-08-20: the outer padding here is cancelled by the
	// centring. A section is 820 px wide with its padding INSIDE it, so its
	// fields start 6 px in from its edge - while this column is centred within
	// what the padding left over, which puts it back at the section's edge
	// rather than at its content. The status line and every field name on the
	// screen above it were 6 px apart, which is the distance that reads as a
	// mistake rather than as an indent.
	inner := Column(GapLabel, content...)
	if rail != nil && len(content) > 0 {
		// Laid over the column rather than beside it. Sharing the row, the rail
		// would take width from one side only and the buttons the column
		// centres would sit off centre by half of it.
		//
		// The vertical box is what keeps the rail one row tall. Handed straight
		// to a stack it would be resized to the whole bar, and a Donate button
		// as tall as the bar is what the first attempt drew.
		//
		// Hung out to the left by the room a bar button keeps round its words.
		// The rail opens with Donate, a quiet button - words, with a surface
		// only under the pointer - so what stands on the edge has to be its
		// words and not its invisible box, the rule inkTight keeps for a label.
		// Measured on 2026-09-24 with the box on the edge: the heart stood
		// 16 px right of the line under it.
		inner = container.New(railOver{centred: content[0]}, inner,
			container.NewVBox(container.New(hungOut{by: BarButtonInsetX}, rail)))
	}
	column := container.New(readableWidth{}, Indented(inner))
	return container.NewStack(panelSurface(), Padded(InsetBar, column))
}

// hungOut lays its one child that far to the left of where it stands, the part
// hanging out taking no room - so a thing whose edge is invisible stands with
// what IS visible on the edge.
type hungOut struct{ by float32 }

func (h hungOut) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.Size{}
	for _, o := range objects {
		size = size.Max(o.MinSize().Subtract(fyne.NewSize(h.by, 0)))
	}
	return size
}

func (h hungOut) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(-h.by, 0))
		o.Resize(size.Add(fyne.NewSize(h.by, 0)))
	}
}

// railOver is the rail laid over the column, and a bar that cannot be made
// narrower than the width at which the two stop meeting.
//
// A stack until 2026-09-23, and a stack's minimum is the larger of its two
// children rather than what they need side by side - so the window could be
// narrowed until the centred buttons slid under the rail. Measured with
// guirender on the batch screen: "Add a batch" covered Preview from about
// 495 px on main and from about 588 px once the buttons grew their gaps on
// #126, and Preview was gone altogether at 495. Found while checking an
// outside review of #126 (docs/REVIEW-126-2026-09-23.md). GUI rule 21 asks
// exactly this of the smallest window.
//
// The buttons are centred in the bar, so the room they leave either side is
// half of what is left over. That half has to hold the rail and a gap, which
// makes the smallest bar the buttons plus twice the rail and the gap.
type railOver struct{ centred fyne.CanvasObject }

func (r railOver) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.Size{}
	for _, o := range objects {
		size = size.Max(o.MinSize())
	}
	if len(objects) > 1 {
		need := r.centred.MinSize().Width + 2*(objects[1].MinSize().Width+GapColumns)
		size.Width = fyne.Max(size.Width, need)
	}
	return size
}

func (railOver) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

// Screen stacks sections under a head - a Title, or a Titled pair.
//
// Windows compose sections rather than laying themselves out in one function.
// That is not tidiness: the shape gate caps a function at eighty lines of
// logic and window layout is long by nature, so a window written as one
// function would arrive as an argument for raising the cap. The cap is a
// ratchet and only goes down, so the composition has to come first.
func Screen(head fyne.CanvasObject, sections ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(readableWidth{},
		Stacked(append([]fyne.CanvasObject{Indented(head)}, sections...)...))
}

// Indented puts something that stands outside a panel on the same left edge as
// the fields inside one.
//
// A screen title sits on the column, and a panel puts its own padding between
// its edge and its content - so the two were 6 px apart. Not enough to read as
// an indent and too much to read as alignment, which is the worst of the three
// possible distances. Measured on 2026-08-20: the title's words started at 97
// and every field name under it at 103.
func Indented(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(indent{}, o)
}

// indent is the layout behind Indented. Horizontal only: the vertical scale
// says how far apart these things stand. It is the same token a panel keeps
// between its edge and its content, so the two line up by construction.
type indent struct{}

func (indent) by() float32 { return Inset }

func (i indent) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, o := range objects {
		min := o.MinSize()
		size.Width = fyne.Max(size.Width, min.Width+i.by()*2)
		size.Height = fyne.Max(size.Height, min.Height)
	}
	return size
}

func (i indent) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(fyne.Max(0, size.Width-i.by()*2), size.Height))
		o.Move(fyne.NewPos(i.by(), 0))
	}
}

// Numeric sizes a control to what it holds rather than to the column it is in.
//
// Only for boxes taking a number. A name, a template and an id are short text
// and take TextWidth (see Text). A path is the one thing whose length nobody
// can predict, so a path still takes the column.
//
// It uses a layout of ours rather than the toolkit's grid wrap, and the reason
// is the edge that marks a refused box: a stack sized by the slot draws its
// line round the whole half column while the box in it is 140 px, which is a
// mark that points at the gap beside the field as much as at the field. Naming
// the layout is what lets WithRing put the line INSIDE this rather than around
// it. Seen on a render on 2026-08-12, which is the only way that kind of thing
// is ever seen.
func Numeric(control fyne.CanvasObject) fyne.CanvasObject {
	return Sized(NumericWidth, control)
}

// Text sizes a box for a short piece of text - a name, a template, a file
// name - to TextWidth rather than to the column it is in. A path is not
// this case and takes the row, see the token.
func Text(control fyne.CanvasObject) fyne.CanvasObject {
	return Sized(TextWidth, control)
}

// Sized draws a control at a width worked out by the caller.
//
// Numeric is the common case and this is the one behind it, for the fields
// whose width comes from what they have to show rather than from a constant -
// see ShapedFor.
func Sized(width float32, control fyne.CanvasObject) fyne.CanvasObject {
	return container.New(fixedWidth{width}, control)
}

// fixedWidth gives its one child a width decided here and the height it asks
// for.
type fixedWidth struct{ width float32 }

func (f fixedWidth) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	size := objects[0].MinSize()
	size.Width = f.width
	return size
}

func (f fixedWidth) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	// At least the width asked for, and the whole of the room when the parent
	// offers more: since the prototype of 2026-09-23 a field stands in a
	// column of a Grid and fills it, so the right edge of a form is one line.
	// The width is a floor now, not a size. Clamping it DOWN to the room was
	// tried on 2026-08-25 and taken straight back out: the parents that offer
	// nought are the ones part way through being built, where clamping
	// collapsed every declared setting to a box of nought or minus three
	// pixels.
	objects[0].Resize(fyne.NewSize(fyne.Max(f.width, size.Width), size.Height))
	objects[0].Move(fyne.NewPos(0, 0))
}

// readableWidth gives its one child the lesser of the space offered and
// ColumnWidth, centred. A VBox stretches its children to whatever it is
// given, which is the whole window, and that is the entire defect.
//
// Centred rather than left aligned, changed on 2026-08-12. Held at the left it
// traded one kind of waste for another: at 1100 px the form ended at 822 and
// left 278 px of nothing down the right hand side, and maximised it left three
// thousand. Space split either side reads as a margin, and the same space all
// on one side reads as a column that failed to fill the window.
type readableWidth struct{}

func (readableWidth) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	size := objects[0].MinSize()
	// The height is the child's and is never capped. Only the width is a
	// choice - a form too tall scrolls, a form too wide cannot be read.
	size.Width = fyne.Min(size.Width, ColumnWidth)
	return size
}

func (readableWidth) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	width := fyne.Min(size.Width, ColumnWidth)
	objects[0].Resize(fyne.NewSize(width, size.Height))
	objects[0].Move(fyne.NewPos((size.Width-width)/2, 0))
}

// Column stacks its children with one fixed gap, whatever the theme's padding
// is.
//
// A layout of our own rather than a spacer between the children of a vertical
// box, and that is what makes the three steps above mean anything: a vertical
// box adds its own padding on top of whatever is put between its children, so
// a spacer can only ever make a gap BIGGER. The tightest step this form needs
// is smaller than that padding.
//
// It skips what is hidden, which is the behaviour WithRoomForARun and the
// error area under every field are built on - a hidden widget costs no height.
func Column(gap float32, children ...fyne.CanvasObject) *fyne.Container {
	return container.New(column{gap: gap}, children...)
}

// column is the layout behind Column.
type column struct{ gap float32 }

func (c column) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	shown := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		size.Width = fyne.Max(size.Width, min.Width)
		size.Height += min.Height
		shown++
	}
	if shown > 1 {
		size.Height += c.gap * float32(shown-1)
	}
	return size
}

func (c column) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		height := o.MinSize().Height
		o.Resize(fyne.NewSize(size.Width, height))
		o.Move(fyne.NewPos(0, y))
		y += height + c.gap
	}
}

// Stacked puts panels one under another with GapSection between them.
func Stacked(panels ...fyne.CanvasObject) fyne.CanvasObject {
	return Column(GapSection, panels...)
}

// WithRoomForARun keeps the height a run's own messages need, whether or not
// there is a run.
//
// A hidden widget takes no room in this toolkit, so the progress bar and the
// status line cost nothing at rest and their full height the moment a run
// starts. The bar at the foot of the form grew from 48 px to 116 px on the
// press of Generate, and the form above it lost exactly that much - so the
// field under the pointer moved out from under it, at the one moment somebody
// is looking at the buttons rather than at the form. Reported by the owner as
// the bar expanding oddly, then measured from the stored screens (O101).
//
// The height is measured from real widgets rather than written down as a
// number, because a number would be a copy of the theme's arithmetic and would
// drift the first time a font or a padding changed.
//
// The bar and ONE line, which is the tallest the ordinary path gets rather
// than the tallest it can get. A preview says one line and no bar, a run says
// one line and the bar, so one line and the bar covers both jumps the owner
// reported and costs about 60 px rather than the 87 px that keeping two lines
// costs. The room matters here: these screens are already taller than the
// window they open in (O102).
//
// A finished run can say more than that, because it prints one line per note
// and a manifest's notes are not bounded.
//
// That used to mean the height grew with the message, and it was written down
// here as a deliberate choice: a message nobody can read is worse than a bar
// that moved. Measured on 2026-08-20, the cost of that choice was 19 px of form
// per extra line - two lines took 849 px down to 830, three to 811 - at the one
// moment somebody is looking at the buttons rather than at the form. The owner
// reversed the choice that day.
//
// So the reserve is now a ceiling as well as a floor, and the message scrolls
// inside it. That keeps the original reason intact rather than trading it away:
// nothing is clipped and nothing becomes unreadable, it is reached by scrolling
// instead of by pushing the form. The height no longer depends on what a run
// has to say, which is the only way this can hold for a number of notes that
// comes from somebody else's preset.
func WithRoomForARun(content fyne.CanvasObject) fyne.CanvasObject {
	// "Ag" is this project's measuring sample - an ascender and a descender,
	// so the line is as tall as a line ever gets. It is never drawn.
	//
	// Measured against the control the screen really puts here, which is the
	// whole reason this sample exists: a reserve is only right while it matches
	// what a run actually draws.
	//
	// It was the toolkit's bar in a wrapper that forced it to 8 px until
	// 2026-08-20, and the answer was right by coincidence - the wrapper and our
	// own track name the same constant. The mutation runner is what said so:
	// taking the wrapper off the bar on the screen broke nothing, because our
	// track had stopped needing it.
	sample := container.NewVBox(NewProgress(), widget.NewLabel("Ag"))

	// Scrolled rather than clipped, and vertical only - a status line that
	// scrolled sideways would hide the start of its own sentence.
	inside := container.NewVScroll(content)
	return container.New(&reserving{height: sample.MinSize().Height}, inside)
}

// reserving is a layout that never reports less height than it was asked to
// keep, and lays its content out at the top of it.
//
// It does not need a matching ceiling, and one was tried and taken out again on
// 2026-08-20. What holds the height down is the scroll above: a scroll asks for
// almost nothing, so the larger of the two is always the reserve. Adding the
// ceiling here as well changed no measurement and could not be broken on
// purpose - the guard stayed green with it removed - and defensive code that
// cannot be broken is not a safeguard, it is a second explanation of the same
// thing for the next person to reconcile.
type reserving struct{ height float32 }

func (r *reserving) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, r.height)
	for _, o := range objects {
		needs := o.MinSize()
		if needs.Width > size.Width {
			size.Width = needs.Width
		}
		if needs.Height > size.Height {
			size.Height = needs.Height
		}
	}
	return size
}

func (r *reserving) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		// Given the whole box rather than its own minimum, so the scroll inside
		// fills the reserve and knows how much of the message it can show.
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}
