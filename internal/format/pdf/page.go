package pdf

// What a page says: its heading, the body text and the footer label.

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// pageContent is what page index says. A page lying wide is shorter, so it
// holds fewer lines - worked out from its own paper, never from the first
// page's, because a mixed document has pages of five heights.
func pageContent(m memo, index int) string {
	var b strings.Builder
	top := m.opts.geometry(index).height - 72

	b.WriteString("BT\n/F1 18 Tf\n")
	fmt.Fprintf(&b, "72 %d Td\n(%s) Tj\n", top, escapeString(fmt.Sprintf("Page %d of %d", index+1, m.opts.pages)))
	b.WriteString("ET\n")

	b.WriteString("BT\n/F1 11 Tf\n")
	rng := core.NewRand(core.FileSeed(m.seed, index))
	y := top - 36
	for line := 0; line < 24 && y > 108; line++ {
		fmt.Fprintf(&b, "1 0 0 1 72 %d Tm\n(%s) Tj\n", y, escapeString(bodyLine(rng)))
		y -= 16
	}
	b.WriteString("ET\n")

	// The label lives in the footer, where it does not sit on top of the
	// content a person is looking at.
	if m.label != "" {
		b.WriteString("BT\n/F1 8 Tf\n")
		fmt.Fprintf(&b, "72 48 Td\n(%s) Tj\n", escapeString(m.label))
		b.WriteString("ET\n")
	}
	return b.String()
}

// lineWidth is how many characters every line of body text comes to.
//
// Fixed, and that is the whole of the point. The words are still drawn from the
// seed, so two seeds give different text - only the length is settled, the same
// way every record based format here settles the length of its closing record.
//
// Why it has to be fixed was measured rather than argued. A line used to be
// eight to thirteen words of four to nine characters, so the smallest document
// one seed could produce was not the smallest another could:
//
//	the floor across 200 seeds   3090 B to 3499 B
//	--size 3300                  accepted for 6 seeds out of 10
//
// An error that appears and disappears when the seed changes is the one thing a
// tool built on "the same seed gives the same run" cannot have, and the engine
// says exactly that about a size drawn from a range. Nobody had applied it to
// the minimum itself.
//
// The first repair kept the ragged lines and took the theoretical worst case as
// the floor. That was honest and consistent, and it cost about 1400 B of range
// that no request could reach any more. This is the answer that costs the user
// nothing, and it pays for it by shifting every byte this format produces -
// a breaking change, taken deliberately while nothing depends on those bytes.
//
// Eighty four characters is close to what the old lines averaged, so a page
// still looks like a page. Justified rather than ragged, which if anything
// reads more like a real document than the random lengths did.
const lineWidth = 84

// bodyLine is one line of body text: drawn from the seed, always lineWidth
// characters long, and never ending in the middle of a word.
//
// core.AppendFiller does almost this and is not used, which is worth saying
// because sharing a primitive is usually the right answer here. That one cuts
// at the byte and makes the difference up with spaces, which is exactly right
// where it is used - inside a field of a CSV row or a JSON value, where a
// clipped word is padding nobody reads. This text is the visible content of a
// page. A document whose every line ends in "refere" reads as broken rather
// than as filler, and the fidelity bar for this format is that it looks like a
// document in Adobe.
//
// So: whole words while the next one still fits, then spaces. The line is the
// same length either way, which is the property the floor depends on.
func bodyLine(rng interface{ IntN(int) int }) string {
	var b strings.Builder
	b.Grow(lineWidth)

	// Started at a word the seed chose and walked in order, so two seeds read
	// differently without needing a draw per word.
	at := rng.IntN(len(words))
	for {
		w := words[at%len(words)]
		need := len(w)
		if b.Len() > 0 {
			need++ // the space in front of it
		}
		if b.Len()+need > lineWidth {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w)
		at++
	}
	// Justified to the fixed width. Every vocabulary here is ASCII, so a byte
	// and a character are the same thing and the count is the width.
	return b.String() + strings.Repeat(" ", lineWidth-b.Len())
}

var words = []string{
	"account", "amount", "balance", "branch", "client", "column", "contract",
	"customer", "delivery", "document", "invoice", "item", "ledger", "note",
	"order", "payment", "period", "product", "quantity", "receipt", "record",
	"reference", "region", "report", "sample", "service", "shipment",
	"statement", "summary", "supplier", "total", "transfer", "unit", "value",
}
