package guard

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A file of the right size can still be the wrong file.
//
// The size guard walks ~120 sizes per format and says nothing about what is in
// them. The determinism guard compares two runs and says nothing either. The
// golden values notice a change but not what changed, and they would have to
// be re-measured for any deliberate edit - so they cannot be the guard that
// says "this is still a log".
//
// These are the structural guards for the text group, and mutation is what
// asked for them: truncating the closing entry of a log leaves the file at
// exactly the right size, so every guard above stayed green.

// combined is the Apache combined log format. Written here rather than shared
// with the generator, because a guard that reuses the code under test agrees
// with it whatever it does.
//
// The octet pattern forbids a leading zero on purpose. The first version of
// this guard demanded exactly three digits per octet, which is what the
// generator happened to produce - so it enforced our own defect instead of the
// format. Real logs write 93, not 093, and a leading zero is read as octal by
// some address parsers. A guard written to the output rather than to the
// specification agrees with whatever the code does, which is no guard at all.
const octet = `(?:0|[1-9]\d{0,2})`

var combined = regexp.MustCompile(
	`^` + octet + `\.` + octet + `\.` + octet + `\.` + octet +
		` - - \[[^\]]+\] "GET /\S* HTTP/1\.1" \d{3} \d+ "[^"]*" "[^"]*"$`)

// A log is read line by line, so a line that is not a whole entry is a broken
// file however right its length is. "The last line is truncated" is what a
// real log looks like caught mid rotation, which is why this cannot be left to
// somebody noticing.
func TestEveryLineOfALogIsAWholeEntry(t *testing.T) {
	// Sizes chosen to land the closing entry in different places: just above
	// the minimum, around the buffer boundary, and at awkward odd numbers.
	for _, size := range []int64{155, 156, 311, 512, 4096, 4097, 32769, 100000} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "log", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}

			text := string(body)
			if !strings.HasSuffix(text, "\n") {
				t.Errorf("the file does not end with a newline, so the last entry is unterminated")
			}
			lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

			entries := 0
			for i, line := range lines {
				if strings.HasPrefix(line, "# ") {
					continue // the label line, which says it is not an entry
				}
				if !combined.MatchString(line) {
					t.Errorf("line %d of %d is not a whole entry:\n  %q", i+1, len(lines), line)
					continue
				}
				entries++
			}
			if entries == 0 {
				t.Error("the file holds no entries at all, so this guard proved nothing")
			}
		})
	}
}

// Markdown is worth generating instead of text only because of the structure.
// A document that quietly stopped emitting blocks would be the right size,
// valid Markdown, and useless as a fixture for a renderer.
func TestAMarkdownDocumentCarriesRealStructure(t *testing.T) {
	// Large enough that every kind of block has come up. Below this the file
	// is legitimately mostly prose, which the size guard already covers.
	body := generateBytes(t, "md", 16384)
	text := string(body)
	lines := strings.Split(text, "\n")

	counts := map[string]int{}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "## "):
			counts["heading"]++
		case strings.HasPrefix(l, "- "):
			counts["bullet"]++
		case strings.HasPrefix(l, "> "):
			counts["quote"]++
		case strings.HasPrefix(l, "| "):
			counts["table"]++
		case strings.HasPrefix(l, "```"):
			counts["fence"]++
		}
	}
	for _, kind := range []string{"heading", "bullet", "quote", "table", "fence"} {
		if counts[kind] == 0 {
			t.Errorf("a 16 KiB document carries no %s - the structure is what makes this format worth generating", kind)
		}
	}

	// An unclosed fence is the failure the block-or-prose split exists to
	// prevent. It renders, so nothing else would notice.
	if counts["fence"]%2 != 0 {
		t.Errorf("%d fence markers - the document ends inside a code block", counts["fence"])
	}

	// Tables have to be rectangular. A row cut short would still look like a
	// table to a reader skimming the file.
	pipes := map[int]int{}
	for _, l := range lines {
		if strings.HasPrefix(l, "| ") {
			pipes[strings.Count(l, "|")]++
		}
	}
	if len(pipes) != 1 {
		t.Errorf("table rows have differing column counts %v - one of them is cut short", pipes)
	}
}

// generateBytes produces one file of a format and returns its bytes, through
// the same plan and write path a run uses.
func generateBytes(t *testing.T, formatID string, size int64) []byte {
	t.Helper()
	desc, err := format.Get(formatID)
	if err != nil {
		t.Fatalf("no such format %q: %v", formatID, err)
	}
	plan, err := desc.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
	if err != nil {
		t.Fatalf("planning %d B of %s: %v", size, formatID, err)
	}
	var out strings.Builder
	if err := desc.Generator.Write(context.Background(), &out, plan); err != nil {
		t.Fatalf("writing %s: %v", formatID, err)
	}
	return []byte(out.String())
}
