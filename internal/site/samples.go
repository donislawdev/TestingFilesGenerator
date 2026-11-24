// This file holds the sample files page: files a visitor downloads from the
// site itself, made by one recipe (web/samples/samples.yaml).
//
// The files are not in the repository. The workflow that publishes the site
// runs the recipe with the tfg of the same commit and checks every file
// against the SHA256SUMS rendered here, so the sums on the page are the sums
// of the bytes a visitor gets - a run of the recipe repeats them on any
// machine, which is the claim the page makes.

package site

import (
	"fmt"
	"strings"
)

// Sample is one file of the sample files page, as the manifest of the run
// that made it describes it.
type Sample struct {
	// Section is where on the page the file is listed: formats, corrupt,
	// tables or placeholders.
	Section string
	Name    string
	Format  string
	Bytes   int64
	SHA256  string
	// Outcome is what the manifest expects a system to do with the file.
	Outcome string
	// Damages names every damage done to the file, in the order it was done.
	Damages []string
}

// SamplesDir is where the files are published, beside their SHA256SUMS and
// the recipe that makes them.
const SamplesDir = "samples"

// SampleRow is one file as a page lists it.
//
// The outcome is the value the manifest writes, shown as code the way a
// damage is, so a reader can look for it in the manifest. What each value
// means is said once in the text of the page.
type SampleRow struct {
	Name    string
	URL     string
	Size    string
	SHA256  string
	Short   string
	Outcome string
	Damages []string
}

// SampleTable is one section of the page: its files, the column headings in
// the language being rendered, and whether it has a column for damages.
type SampleTable struct {
	Rows                               []SampleRow
	Damaged                            bool
	File, Size, SHA256, Expected, Harm string
}

// SampleTable is every file of one section, in the order the recipe makes
// them.
//
// A section with no file is an error rather than an empty table: a heading
// over nothing reads as a page that broke. So is an outcome the language has
// no sentence for, because the text of the page explains every one it shows.
func (v view) SampleTable(section string) (SampleTable, error) {
	t := SampleTable{Damaged: section == "corrupt"}
	for _, pair := range []struct {
		to  *string
		key string
	}{{&t.File, "colFile"}, {&t.Size, "colSize"}, {&t.SHA256, "colSha256"}, {&t.Expected, "colExpected"}, {&t.Harm, "colDamage"}} {
		w, err := v.Word(pair.key)
		if err != nil {
			return SampleTable{}, err
		}
		*pair.to = w
	}
	for _, s := range v.Facts.Samples {
		if s.Section != section {
			continue
		}
		if _, ok := v.Lang.Outcomes[s.Outcome]; !ok {
			return SampleTable{}, fmt.Errorf("the sample %s is expected to %q and the %s pages have no sentence for it", s.Name, s.Outcome, v.Lang.Code)
		}
		t.Rows = append(t.Rows, SampleRow{
			Name:    s.Name,
			URL:     "/" + SamplesDir + "/" + s.Name,
			Size:    grouped(s.Bytes) + " B",
			SHA256:  s.SHA256,
			Short:   s.SHA256[:16],
			Outcome: s.Outcome,
			Damages: s.Damages,
		})
	}
	if len(t.Rows) == 0 {
		return SampleTable{}, fmt.Errorf("the %s sample files page lists the section %q and no sample file is in it", v.Lang.Code, section)
	}
	return t, nil
}

// ToolRow is one tool as the tools page lists it.
type ToolRow struct {
	ID       string
	Question string
}

// ToolList is every tool the program has, in the order tfg tool list prints
// them, with the question each answers in the language being rendered.
func (v view) ToolList() ([]ToolRow, error) {
	out := make([]ToolRow, 0, len(v.Facts.Tools))
	for _, id := range v.Facts.Tools {
		q, ok := v.Lang.Tools[id]
		if !ok {
			return nil, fmt.Errorf("the tool %q has no question written in %s", id, v.Lang.Code)
		}
		out = append(out, ToolRow{ID: id, Question: q})
	}
	return out, nil
}

// SampleCount is how many files the sample files page offers.
func (f Facts) SampleCount() int { return len(f.Samples) }

// sampleSums is the SHA256SUMS of the sample files, in the shape sha256sum
// writes and tfg tool checksum-check reads.
func (s Site) sampleSums() []byte {
	var b strings.Builder
	for _, x := range s.Facts.Samples {
		b.WriteString(x.SHA256 + "  " + x.Name + "\n")
	}
	return []byte(b.String())
}
