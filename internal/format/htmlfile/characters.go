package htmlfile

// The paragraphs a page carries when characters is international, measured on
// four readers on 2026-10-14 (docs/HTML-2-2026-10-14.md section 11.4).
//
// Polish, Russian and Japanese are written as they are, so the line can be
// read here. The Arabic is escaped, because text written right to left turns
// the line around in an editor, and so are the characters nobody can see: the
// joiner inside the emoji sequence and the accent that makes a second code
// point of the e. Written as they are, a change to either would look like no
// change at all.
//
// Each script is a paragraph of its own with its own lang, the Arabic one with
// dir="rtl", because that is how a page says which language a passage is in -
// and the attribute is what a reader keys its fonts and its direction on.
const (
	arabic = "\u0647\u0630\u0627 \u0646\u0635 \u0628\u0627\u0644\u0644\u063A\u0629 \u0627\u0644\u0639\u0631\u0628\u064A\u0629."

	international = `<p lang="pl">Zażółć gęślą jaźń.</p>` + "\n" +
		`<p lang="ru">Это текст на русском языке.</p>` + "\n" +
		`<p lang="ja">これは日本語のテキストです。</p>` + "\n" +
		`<p lang="ar" dir="rtl">` + arabic + `</p>` + "\n" +
		// The entities are the ones a converter most often gets wrong: the
		// five that have to be escaped, a non-breaking space, two named
		// symbols, and a number written in decimal and in hex, one of them past
		// the basic plane.
		`<p>Emoji ` + "\U0001F600" + ` and a sequence ` + "\U0001F469\u200D\U0001F4BB" + `, ` +
		`a letter in two code points e` + "\u0301" + `, ` +
		`and entities &amp; &lt; &gt; &quot; &nbsp; &copy; &euro; &#8364; &#x1F600;.</p>` + "\n"
)

// internationalBlocks is what the setting adds, nothing for ascii, in the
// spelling of the page.
func (s settings) internationalBlocks() string {
	if !s.international {
		return ""
	}
	return s.spelling()[scripts]
}
