package pdf

// The smallest document this format produces, and how a refusal explains it.

// labelCost and cleanHint keep the message about the minimum honest. The
// figure in the registry is the smallest document with no label, so a user
// who left the label on and hits the limit needs to be told why the number
// they were shown is not the number they got.
func labelCost(label bool) string {
	if label {
		return " carrying the self describing label"
	}
	return ""
}

func cleanHint(label bool) string {
	if label {
		return ", ask for fewer pages by setting pages to 1, or drop the label"
	}
	return " or ask for fewer pages by setting pages to 1"
}

// minimumBytes is the smallest document this generator can produce: one A4
// page with no label. Measured at start up rather than guessed.
func minimumBytes() int64 {
	m := memo{pages: 1, pageSize: pageSizes["a4"]}
	prefix, suffix := document(m)
	return int64(len(prefix) + len(suffix))
}
