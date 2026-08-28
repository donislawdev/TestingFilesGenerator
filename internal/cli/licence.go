package cli

import (
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// printCarried writes what the running binary contains that somebody else
// wrote, under two headings: code, and bytes that are not code.
//
// The values come from internal/legal, which joins the build's own record of
// itself with the reviewed licences. The words around them are written here,
// because the command line is English by rule and the window is not - the same
// split the site uses, where a number comes from the program and the noun
// after it comes from the language.
//
// Nothing is printed when a build carries no record of itself. That happens
// when a binary is assembled by something other than the go tool, and printing
// an empty heading would suggest this program ships nothing.
func printCarried(out io.Writer, items []legal.Item) {
	if len(items) == 0 {
		return
	}
	printGroup(out, "\nThird party code compiled into this binary:\n\n", items, false)
	printGroup(out, "\nFiles compiled in that are not code:\n\n", items, true)
	fmt.Fprint(out, "\nThe full text of each licence is in THIRD-PARTY-NOTICES.md.\n")
}

// printGroup writes one heading and the items belonging under it, and writes
// nothing at all when none belong there. The command line binary has no
// embedded files, so its answer ends after the first group rather than
// carrying a heading with nothing beneath it.
func printGroup(out io.Writer, heading string, items []legal.Item, embedded bool) {
	written := 0
	for _, item := range items {
		if item.Embedded != embedded {
			continue
		}
		if written == 0 {
			fmt.Fprint(out, heading)
		}
		written++
		fmt.Fprintf(out, "  %s\n", item.Line())
	}
}
