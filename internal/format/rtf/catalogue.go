package rtf

// The parts elements=every adds - every place a document keeps text that a
// reader may show, leave out or show where it should not. Each sentence says
// where it is, so a test reading what an extractor returned knows which parts
// it took without knowing RTF. Measured on 2026-10-15: six readers, five
// different sets (docs/RTF-2026-10-15.md section 6.1 d).
//
// Two pieces, because RTF fixes where the first one goes: the document
// properties come before any text, and a page header and footer open the
// section they belong to. The rest are paragraphs after the label, each ending
// the way every block does - its paragraph closed and the properties reset - so
// whatever follows starts from the defaults.

// catalogueHead is what goes in front of the label: the properties of the
// document, and the header and footer of its pages.
const catalogueHead = `{\info{\title This is the title of the document}{\author tfg}{\keywords catalogue rtf}` +
	`{\doccomm This is the comment of the document}}` + "\r\n" +
	`{\header\pard This sentence is in the page header.\par}` + "\r\n" +
	`{\footer\pard This sentence is in the page footer.\par}` + "\r\n"

// catalogueBody is the paragraphs after the label. The link goes to a domain
// kept for examples (RFC 2606), and a field is never fetched by opening the
// document - a reader that follows it is doing so on a click.
const catalogueBody = `A paragraph with a footnote{\super\chftn}{\footnote\pard\plain{\super\chftn} This sentence is in a footnote.}.\par\pard` + "\r\n" +
	`A paragraph with a comment{\*\atnid tfg}{\*\atnauthor tfg}\chatn{\*\annotation\pard\plain This sentence is in a comment.}.\par\pard` + "\r\n" +
	`Before hidden text {\v This sentence is hidden text.}and after it.\par\pard` + "\r\n" +
	`A link {\field{\*\fldinst HYPERLINK "https://example.com/tfg/link"}{\fldrslt This sentence is the text of a link.}}\par\pard` + "\r\n" +
	`A bookmark {\*\bkmkstart tfg_bookmark}around these words{\*\bkmkend tfg_bookmark}.\par\pard` + "\r\n" +
	`A group {\*\tfgunknown This sentence is in a group every reader has to skip.}that ends here.\par\pard` + "\r\n" +
	`A group {\tfgword This sentence follows a word no reader knows, which is why it shows.} that ends here.\par\pard` + "\r\n" +
	`Before a line break\line after it, before a tab\tab after it.\par\pard` + "\r\n" +
	`This paragraph ends with a page break.\par\page\pard` + "\r\n" +
	`This paragraph ends a section.\par\sect\sectd\pard` + "\r\n" //nolint:misspell // \sect is the RTF control word of a section break, read by misspell as a backslash and "ect"
