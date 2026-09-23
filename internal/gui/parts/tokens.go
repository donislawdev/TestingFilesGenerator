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
	// GapTabs is the space between two words on the strip across the top,
	// box to box. The words keep TabInset inside their boxes as well, so the
	// ink of two neighbours stands three steps apart.
	GapTabs = space2
	// TabInset is the room a word on the strip keeps inside its own box, all
	// round the letters. It is what the fill under the pointer and the ring
	// round the keyboard have to draw in, so it is the smallest step that
	// leaves a ring readable as a ring rather than as an outline of the ink.
	TabInset = space2
	// GapUnderTabs is the space between the line under the strip and the title
	// of the screen it leads to. A section's step, because the strip and the
	// screen are two things and not one.
	GapUnderTabs = space5
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
	// RadiusMark rounds the square of a switch. A step under RadiusField,
	// because the square is 20 px and a 6 px corner on it reads as a disc.
	RadiusMark = 4
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
	// ringGap is the room between a control's face and the ring the keyboard
	// draws round it, on the controls that draw the ring themselves - a
	// button, the square of a switch. Read in the painter rather than assumed
	// (internal/painter/draw.go, drawOblong): a stroke runs down the middle of
	// its rectangle's edge, so a ring drawn ON the edge of the filled primary
	// button is primary over primary and cannot be seen. Standing this far
	// clear of the face the ring is one colour on every face. It is painted
	// outside the control's bounds, which the toolkit allows because it clips
	// nothing, so the control stays the size of its face.
	ringGap = 2
	// edgeWidth is the line round a control at rest that has one - a menu, a
	// plain button. One pixel, and it arrives fainter than its colour for the
	// reason ringWidth gives, which is why a resting edge and a state ring are
	// never the same thickness: the eye tells them apart by weight before it
	// reads the colour.
	edgeWidth = 1
	// TabIndicator is how thick the mark under the chosen word on the strip
	// is. The same thickness as a ring, for the same reason: one pixel is
	// anti-aliased to something fainter than the colour it was given.
	TabIndicator = 2
	// Hairline is a line that separates and says nothing else - the rule
	// under the strip across the top. One pixel, and drawn as a filled
	// rectangle rather than a stroke, so it is one pixel and not a blend.
	Hairline = 1
	// TipShadowDrop is how far below an explanation its shade shows.
	TipShadowDrop = 3
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
	// TextWidth is as wide as a box holding a short piece of text gets - a
	// batch's name, the template its files are named by, the manifest's file
	// name, an archive's password, a preset's list of sizes. Two number boxes
	// and the gap between them, so a text box ends where a pair of numbers
	// standing side by side would, and the column of controls keeps one
	// right edge for everything but a path. Until 2026-09-21 these boxes
	// took the whole row on the sentence that free text has no length to
	// promise, and the owner's report from the running window was the
	// obvious one: why are they so long. A path is the one value that can
	// be, so a path still takes the row.
	//
	// One column of the grid since the prototype of 2026-09-23, the same as a
	// number: in a form of GridColumns columns a name sized for two numbers
	// took two columns, and the owner's report from the running window was
	// boxes far wider than what they hold. A placeholder longer than this
	// still widens it (ShapedFor), and in the grid it fills its columns.
	TextWidth = NumericWidth
	// GridColumns is how many columns a section's fields are laid in. Four
	// gives 185 px a column in the form's 788 px (ColumnWidth less a panel's
	// inset on each side), which holds a number box with room for a short
	// menu. Chosen for the prototype of 2026-09-23, not measured against
	// five - five would leave a number box 4.8 px to spare.
	GridColumns = 4
	// GroupInset is the room inside the frame drawn round a group of
	// settings a section folds away - see NewInnerFolding.
	GroupInset = space3
	// GroupInsetY is the same room above and below, smaller because the
	// head of a fold keeps room of its own round its words.
	GroupInsetY = space1
	// BarButtonInsetX and BarButtonInsetY are the room inside a button that
	// stands in the bar at the foot, around its words. Bigger than the room
	// inside a field on purpose, prototype of 2026-09-23: measured in the
	// real window, Generate was 26 px tall - one of the smallest things on a
	// screen whose whole purpose is that one press. The first prototype gave
	// it 12 above and below and the owner's verdict from the running window
	// was "gigantic", so the height is a field's again and only the sides
	// keep more room: the button is told apart by its width and its colour.
	BarButtonInsetX = space4
	BarButtonInsetY = ControlInset
	// GapButtons is the space between two buttons side by side - in that bar
	// and in the head of a batch (ButtonRow). It was the toolkit's padding,
	// about 4 px, so Preview and Generate, and Duplicate and Remove, each read
	// as one control with two words in it.
	GapButtons = space3
	// GlyphButton is the side of the small square button that holds one
	// glyph - the mark beside a field's name that opens its explanation. The
	// glyph itself is the toolkit's inline icon, 20, and the square keeps two
	// pixels round it, which is what makes it a target and not a letter.
	GlyphButton = 24
	// SwatchSide is the side of the square of colour on the palette page. Off
	// the ladder of controls on purpose: this square is not a target and not a
	// mark beside a word, it is the thing being looked at, and at the size of
	// a switch's tick two neighbouring surfaces six L* apart read as one.
	SwatchSide = space6
	// markSide is the side of the square of a switch: the toolkit's inline
	// icon, the same 20 the glyph above is built round, so a switch and the
	// button that explains it are one size and stand in one 24 px box.
	markSide = 20
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
	// listShare is how much of the window an open list may cover: half of its
	// height, in whole rows, before it scrolls. NN/g puts the ceiling as a rule
	// rather than a number - the label and the context stay in view while the
	// list is open - and the other half of the window IS the context. Until
	// 2026-09-15 this was a count, eight rows, chosen on 2026-08-18 for the
	// smallest window this program opens at and then applied to every window:
	// measured with guirender that day, the list of twenty-four formats was
	// 224 px tall at 800x600, at 1100x1300 and at the owner's 1101x1025 alike,
	// so a third of the values showed however tall the window was (O203). Half
	// gives ten rows at 600 px, eighteen at 1025 and twenty-three at 1300, and
	// at 600 the list still ends above the buttons at the foot.
	listShare = 0.5
	// listOpensDownwardFrom is how many whole rows have to fit under a box
	// for its list to open downward, cut to that room and scrolling, rather
	// than upward into the room above. Decision of the owner, 2026-09-21,
	// from the running window: the format list on the preset screen opened
	// upward, over the question the preset asks, while the same list on the
	// other screens opened downward - one control behaving two ways for a
	// reason nobody could see. Upward is kept for the emergency alone, a box
	// standing just above the bar at the foot, where fewer rows than this
	// would fit. Five rows is enough to read a list and to see it scrolls.
	listOpensDownwardFrom = 5
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

// EdgeWidth is the line round a control at rest, for a guard measuring
// whether a fill stays inside it.
func EdgeWidth() float32 { return edgeWidth }

// RowGutter is the room in front of the first thing on a list row, for a
// guard asking whether the words start there or a column later.
func RowGutter() float32 { return rowGutter }
