package pdf

// The document information dictionary and the strings written into it.

import (
	"fmt"
	"strings"
)

func infoObject(m memo) string {
	title := m.label
	if title == "" {
		title = "Testing Files Generator"
	}
	// A fixed date, because a timestamp taken from the clock would make two
	// runs of the same recipe differ.
	return fmt.Sprintf("<</Title(%s)/Producer(Testing Files Generator)/CreationDate(D:20200101000000Z)>>",
		escapeString(title))
}

// escapeString protects the three characters that end or nest a PDF string.
// Without this a label containing a bracket would produce a file no reader
// can parse.
func escapeString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
