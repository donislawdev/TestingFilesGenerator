package parts

// Every number that decides how the window LOOKS lives in this file and
// nowhere else. A view names a distance, a size or a width by its role and
// never by its value, so a change of look is a change in one place - and a
// number that is not here is a number nobody chose.
//
// Measured on 2026-09-11, before this file existed: 149 different gaps between
// neighbours across the stored screens, on 2113 pairs, 72 of them zero. The
// window had three gaps of its own (1, 9 and 14) and every stock container
// added the theme's padding of 6 on top, so what reached the screen was the
// sum of a step somebody chose and one nobody did. The scale below is the
// answer to that number, not to a taste.

// The scale of distances. Six steps, and every gap in the window is one of
// them. Unexported on purpose: a view asks for a role from the next block, and
// a distance that fits no role gets a role here rather than a bare step at the
// call site.
const (
	space1 = 4
	space2 = 8
	space3 = 12
	space4 = 16
	space5 = 24
	space6 = 32
)

// Distances, each a step of the scale, named for what they say. The steps
// have to be far enough apart to be read without counting: a field holds
// together at one step and two fields stand apart at twice it, which is the
// ratio TestAFieldHoldsTogetherMoreTightlyThanTwoFieldsDo asks for.
const (
	// GapInline separates things that share one line: a name and the mark
	// beside it, an icon and its word.
	GapInline = space1
	// GapTight is the space inside one field below its control - between the
	// control and the sentence under it, and before the line that says what
	// was refused. Those are one thing, so they sit close.
	GapTight = space1
	// GapLabel is the space between a field's name and its control. Wider than
	// GapTight because the name is drawn without room of its own around the
	// ink, where a toolkit label carried its padding.
	GapLabel = space2
	// GapField is the space between two fields in a section.
	GapField = space4
	// GapSection is the space between two panels. Bigger than the room inside
	// a panel, because a boundary drawn more weakly than the padding inside it
	// groups the wrong things - TestTheGapBetweenSectionsIsWiderThanTheGapBetweenFields.
	GapSection = space5
	// GapColumns is the space between two fields standing side by side.
	GapColumns = space4
	// Inset is the room a panel keeps between its edge and its content, and
	// the distance a title standing outside a panel is indented by, so the two
	// line up by construction rather than by two numbers agreeing.
	Inset = space4
	// InsetBar is the room the strip at the foot keeps around what stands on
	// it. A step less than a panel: a bar reads as a bar by running the whole
	// width, and the room a card keeps would make it read as a third panel.
	InsetBar = space3
	// ControlInset is the room INSIDE a box to type in and a button, between
	// their edge and their words. Off the scale on purpose: the scale is for
	// the distance between things, and this is what keeps a box 32 px tall,
	// which is what it has been since the palette was measured. The toolkit's
	// own value is 8.
	ControlInset = 6
	// ThemePadding is what the toolkit is told when a stock container asks how
	// far apart to put its children. The smallest step, so a container this
	// package did not replace lands ON the scale rather than beside it - zero
	// would make every such container glue its content together in silence.
	ThemePadding = space1
	// listEdgeGap is the space kept between an open list and the edge of the
	// window, so that a list filling the room still reads as sitting inside it.
	listEdgeGap = space2
	// revealMargin is how much of the form is kept above a control brought
	// into view, so it arrives looking like part of a form rather than pinned
	// to the top edge with its own label cut off above it.
	revealMargin = space5
)

// The ladder of type. Four sizes and nothing between them.
const (
	// TextTitle is the one line that says what a screen is for.
	TextTitle = 20
	// TextHeading names a section, between the screen and a field.
	TextHeading = 17
	// TextBody is a value, a field's name, a button and a sentence someone
	// reads at rest - the toolkit's own text size, kept.
	TextBody = 14
	// TextCaption is an explanation under a field. At 11 it was hard work.
	TextCaption = 12
)

// Corners. Chosen here rather than inherited: the toolkit's input radius was
// read in three places to keep our shapes in step with its boxes and never
// picked in any of them (GUI.md section 10.1).
const (
	// RadiusField rounds a box to type in, a menu and a button.
	RadiusField = 6
	// RadiusPanel rounds the surface a section stands on.
	RadiusPanel = 8
)

// Strokes.
const (
	// ringWidth is how thick the line round a control that holds the keyboard
	// or was refused is. Twice the toolkit's input border, so it reads as a
	// deliberate edge rather than as the box's own outline changing colour. One
	// pixel lands between pixels and is anti-aliased away to something fainter
	// than the number suggests - measured on the section surface on 2026-08-12,
	// where a one pixel stroke of a 29.4 L* colour came out at 22.3.
	ringWidth = 2
)

// Widths and heights that are not distances.
const (
	// ColumnWidth is as wide as the form is allowed to get, whatever the
	// window does. O72, measured on 2026-08-10 and again on 2026-08-11:
	// maximised to 3862 px, every box was 3848 to 3854 px of it - 99.7 per
	// cent - so the seed field holding "0" was nearly four thousand pixels
	// wide. 820 comes from the longest sentence the form actually holds, which
	// ends at 797 px. Prose is easiest at 45 to 75 characters a line and 820 px
	// is about 112, so the typography pass has room to tighten this. It cannot
	// widen it.
	ColumnWidth = 820
	// NumericWidth is as wide as a box holding a number gets. 140 px holds
	// eleven digits at the text size this window uses, which covers every
	// number any of these fields accepts - the ceiling on files is seven digits
	// and the largest size anybody types is eight.
	NumericWidth = 140
	// GlyphButton is the side of the small square button that holds one
	// glyph - the mark beside a field's name that opens its explanation. The
	// glyph itself is the toolkit's inline icon, 20, and the square keeps two
	// pixels round it, which is what makes it a target and not a letter.
	GlyphButton = 24
	// DetailWidth is how wide the longer explanation gets when it opens.
	// Narrower than the form on purpose: the column is 820 px because that is
	// what the form needs, and the same width for a paragraph of prose is about
	// 112 characters a line - well past the 45 to 75 that reads easily. A block
	// of text with nothing beside it has no reason to be as wide as a row of
	// fields.
	DetailWidth = 380
	// SlimHeight is how tall a progress track is drawn. The toolkit's bar is
	// as tall as the words "100%" and the padding around them, because it
	// writes the percentage inside itself - and the line under it already
	// ends with that number, so the second copy cost 23 px of a bar the owner
	// asked to make smaller.
	SlimHeight = 8
	// visibleRows is how many rows of an open list show before it scrolls.
	// Eight, decided by the owner on 2026-08-18. NN/g puts it as a rule rather
	// than a number - the label and the context stay in view while the list is
	// open - and eight is what leaves most of the form visible at the window
	// sizes this program opens at, including 800x600.
	visibleRows = 8
	// rowPadding is the room above and below a list row's contents. Ours rather
	// than the theme's, which is the entire point of that control: the theme's
	// inner padding is what a box to type in and a button are also built from,
	// so a list cannot be made denser through it without making every control
	// on the form denser too.
	rowPadding = space1
	// rowGutter is the room in front of a list row's mark, and rowGap the room
	// after it and after the icon. Both were 6 until the scale arrived and are
	// its second step now, so a row is 4 px wider than it was.
	rowGutter = space2
	rowGap    = space2
)
