package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The README's table of presets names every preset the program registers,
// with the question that preset answers, and names nothing else.
//
// Until 2026-09-29 the section named no preset at all and sent the reader to
// tfg preset list, because a list typed by hand goes stale and nothing
// compared one with the registry. The owner asked for the list on the first
// page a visitor reads - a generator of single files loses to the ones in a
// browser, and the presets are what those do not have. This is the comparison
// that makes the list safe to keep: a seventh preset turns it red until the
// table names it, and a preset renamed or removed turns it red until its row
// goes. The question is compared word for word, because it is the one sentence
// the program itself prints for each preset.
func TestTheReadmeListsEveryPresetItShips(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("reading the README: %v", err)
	}
	const heading = "## 🧪 Presets"
	text := string(body)
	start := strings.Index(text, heading)
	if start < 0 {
		t.Fatalf("the README has no %q heading, so this guard has nothing to read", heading)
	}
	end := strings.Index(text[start+len(heading):], "\n## ")
	if end < 0 {
		t.Fatal("the presets section runs to the end of the README, which means the heading after it moved")
	}
	section := text[start : start+len(heading)+end]

	rows := map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		if len(cells) != 2 {
			t.Errorf("a row of the presets table has %d cells, and it has two - the preset and "+
				"its question:\n%s", len(cells), line)
			continue
		}
		id := strings.Trim(cells[0], "`")
		if _, twice := rows[id]; twice {
			t.Errorf("the presets table has two rows for %s, and the second would hide what the "+
				"first says from this guard", id)
		}
		rows[id] = strings.TrimSpace(cells[1])
	}
	if len(rows) == 0 {
		t.Fatal("the presets section has no table rows - this guard would pass against any README ever written")
	}

	registered := preset.All()
	if len(registered) == 0 {
		t.Fatal("no preset is registered - this guard would pass without checking anything")
	}
	known := map[string]bool{}
	for _, p := range registered {
		known[p.ID] = true
		question, ok := rows[p.ID]
		if !ok {
			t.Errorf("%s is registered and the table under %q does not name it, so the first page "+
				"a visitor reads offers fewer presets than the binary ships", p.ID, heading)
			continue
		}
		if question != p.Question {
			t.Errorf("the table says %s answers %q, and the preset says it answers %q",
				p.ID, question, p.Question)
		}
	}
	for id := range rows {
		if !known[id] {
			t.Errorf("the table under %q names %s, and no preset of that name is registered", heading, id)
		}
	}
}
