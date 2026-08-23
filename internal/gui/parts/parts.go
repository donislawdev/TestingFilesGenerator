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
	return label
}

// Heading is the name of one field, above its control.
func Heading(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

// Title is the one line that says what a screen is for.
//
// Bigger than a field's name rather than the same size in the same weight,
// which is what it was until 2026-08-11: the title of the screen and the label
// of every field were one style, so nothing led the eye and the first point of
// the UX section 7 checklist - squint, and see what stands out - had no answer.
func Title(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.SizeName = theme.SizeNameSubHeadingText
	return label
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
func Section(title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	body := make([]fyne.CanvasObject, 0, len(content)+1)
	body = append(body, sectionTitle(title))
	body = append(body, content...)
	return container.NewStack(panelSurface(), container.NewPadded(Column(GapField, body...)))
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

// SettingsHeading names the block of fields a chosen format declares.
//
// A rank of its own rather than a field's name in bold, which is what it was
// until 2026-08-20: "settings for bmp" sat in the same size and the same
// weight as the label of the box under it, so a heading and the thing it heads
// were drawn identically and the block had no top edge. It is the same rank a
// section title uses, because that is what it is - a group of fields under a
// name, inside a panel that already has one.
func SettingsHeading(title string) fyne.CanvasObject {
	return sectionTitle(title)
}

// sectionTitle names a section, at the rank between the screen and a field.
//
// The size the card used to draw it at, kept deliberately: the scale is four
// ranks and this is the second, so moving off the toolkit's widget must not
// quietly move the type with it.
func sectionTitle(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.SizeName = theme.SizeNameHeadingText
	return label
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
	rect.CornerRadius = Theme().Size(theme.SizeNameCardRadius)
	return rect
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
func Bullets(items []string) fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(items))
	for _, item := range items {
		marker := widget.NewLabel(bulletMarker)
		marker.Importance = widget.LowImportance
		marker.SizeName = theme.SizeNameCaptionText
		rows = append(rows, container.NewBorder(nil, nil, marker, nil, Note(item)))
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
	return container.NewGridWithColumns(len(fields), fields...)
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
// A separator from the palette at the width of a stroke, so it reads as a
// boundary rather than as a third control.
func Divider() fyne.CanvasObject {
	line := canvas.NewRectangle(PaletteColour(theme.ColorNameSeparator, theme.VariantDark))
	line.SetMinSize(fyne.NewSize(1, 0))
	return container.New(dividerLayout{}, line)
}

// dividerLayout keeps the line one pixel wide and gives it room on either side.
type dividerLayout struct{}

func (dividerLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(1+theme.Padding()*4, 0)
}

func (dividerLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(1, size.Height))
		o.Move(fyne.NewPos(theme.Padding()*2, 0))
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
// The rail is the exception, on the owner's decision of 2026-08-19: it stands
// at the left edge of the bar rather than in that column. What it holds is
// what the run is not about - Donate, and adding a batch - so lining it up
// with the form bought nothing and spent 78 px of margin saying so. Pass nil
// on a screen that has none.
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
	column := container.New(readableWidth{}, Indented(container.NewVBox(content...)))
	standing := fyne.CanvasObject(column)
	if rail != nil {
		// Laid over the column rather than beside it. Sharing the row, the rail
		// would take width from one side only and the buttons the column
		// centres would sit off centre by half of it.
		//
		// The vertical box is what keeps the rail one row tall. Handed straight
		// to a stack it would be resized to the whole bar, and a Donate button
		// as tall as the bar is what the first attempt drew.
		standing = container.NewStack(column, container.NewVBox(rail))
	}
	return container.NewStack(panelSurface(), container.NewPadded(standing))
}

// SlimHeight is how tall a progress track is drawn.
//
// The toolkit's progress bar is as tall as the words "100%" and the padding
// around them, because it writes the percentage inside itself. The line
// directly under it already ends with that same percentage - see text.Progress
// - so the number stood on the screen twice and the second copy cost 23 px of
// a bar the owner asked to make smaller. Measured from the stored tree on
// 2026-08-19: the bar was 31 px and the line under it another 31.
const SlimHeight = 8

// Screen stacks sections with a heading on top.
//
// Windows compose sections rather than laying themselves out in one function.
// That is not tidiness: the shape gate caps a function at eighty lines of
// logic and window layout is long by nature, so a window written as one
// function would arrive as an argument for raising the cap. The cap is a
// ratchet and only goes down, so the composition has to come first.
func Screen(heading string, sections ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(readableWidth{},
		Stacked(append([]fyne.CanvasObject{Indented(Title(heading))}, sections...)...))
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
// above already says how far apart these things stand.
//
// It asks the installed theme at layout time rather than holding a number, and
// that is not caution. A panel gets its inset from container.NewPadded, which
// reads the installed theme - so an indent taken from our own palette object
// agrees with it only while the two are the same. They are not always: a test
// canvas that has not installed our theme pads by 4 where we pad by 6, and the
// first version of this was 2 px out under exactly that canvas. Alignment is a
// relationship between two things, so it has to be read from the same place
// both of them read it from.
type indent struct{}

func (indent) by() float32 { return theme.Padding() }

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

// ColumnWidth is as wide as this form is allowed to get, whatever the window
// does. O72, measured on 2026-08-10 and again on 2026-08-11: maximised to
// 3862 px, every box was 3848 to 3854 px of it - 99.7 per cent - so the seed
// field holding "0" was nearly four thousand pixels wide. UX6 puts it as a
// question rather than a rule: run your eye along a row to the right edge, and
// if you got lost the row is too long.
//
// 820 comes from the longest sentence the form actually holds, which ends at
// 797 px - the hint under the self describing label - so nothing rewraps and
// this change only stops the stretching. It is not a claim about the ideal
// measure: prose is easiest at 45 to 75 characters a line and 820 px is about
// 112, so the typography pass has room to tighten this. It cannot widen it.
const ColumnWidth = 820

// NumericWidth is as wide as a box holding a number gets.
//
// A box is a promise about what goes in it, and one that runs half the window
// while holding "0" promises something the field cannot take. Measured on
// 2026-08-12 before this existed: the seed and the count were 397 and 399 px
// wide for a single digit, because a column split in two hands each half to
// whatever is in it.
//
// 140 px holds eleven digits at the text size this window uses, which covers
// every number any of these fields accepts - the ceiling on files is seven
// digits and the largest size anybody types is eight.
const NumericWidth = 140

// Numeric sizes a control to what it holds rather than to the column it is in.
//
// Only for boxes taking a number. A path, a name template and an id are all
// things whose length nobody can predict, so those still take the column.
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
	objects[0].Resize(fyne.NewSize(f.width, size.Height))
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

// The vertical scale. Three steps, and the ratio between them is the point
// rather than the individual numbers.
//
// Measured on 2026-08-20, before this existed: every gap in the form came out
// of the theme's one padding value, so the distance from a label to its own
// control and the distance from the end of one field to the start of the next
// were 20 px and 23 px. The form said "these belong together" and "this group
// has ended" with the same space, which is the whole of what spacing is for.
// A picture of it reads as a wall of text, and that was the first finding of
// the design audit.
//
// The steps have to be far enough apart to be read without counting. The pairs
// above were fifteen per cent apart, which the eye does not resolve.
const (
	// GapTight is the space inside one field, between its name, its control
	// and the sentence under it. Those are one thing, so they sit close.
	GapTight = 1
	// GapField is the space between two fields in a section.
	GapField = 9
	// GapSection is the space between two panels.
	//
	// Reported from use on 2026-08-18, looking at the recipe screen: with a
	// panel per batch stacked one under another, the gap the toolkit left was
	// small enough that two panels read as one long one with a line across it.
	// The edge of a panel is what says where a batch begins, and it was doing
	// that job at the same strength as the gap between two fields inside it.
	//
	// Applied wherever panels are stacked rather than inside Section, so a
	// panel used on its own carries no stray space under it.
	GapSection = 14
)

// SectionGap is the older name for GapSection, kept because that is the name
// the recipe screen and the guards were written against.
const SectionGap = GapSection

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
