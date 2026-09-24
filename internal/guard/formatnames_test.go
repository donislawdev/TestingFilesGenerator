package guard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A format's name is shown by every command that lists formats, and shown
// where a reader looks for it.
//
// The name went into the registry on 2026-09-24 so that the window and the
// command line say one thing (D1): somebody who meets "jxl" in either can learn
// it is JPEG XL. A name the registry carries and a surface drops is the window
// knowing something the command line does not, which is what putting it in the
// registry rather than beside the window's list was meant to prevent.
//
// The table is checked by COLUMN rather than by the name appearing somewhere
// in its row. "ZIP" or "YAML" appearing anywhere would pass a containment
// check whether or not the column held them.
func TestEveryFormatIsNamedWhereverTheCommandLineListsIt(t *testing.T) {
	all := format.All()
	if len(all) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	code, table, errOut := run(t, "formats")
	if code != cli.ExitOK {
		t.Fatalf("tfg formats: exit %d: %s", code, errOut)
	}
	lines := strings.Split(table, "\n")
	column := strings.Index(lines[0], "NAME")
	if column < 0 {
		t.Fatalf("the table has no NAME column:\n%s", lines[0])
	}
	rows := map[string]string{}
	for _, line := range lines[1:] {
		if id, _, found := strings.Cut(line, " "); found {
			rows[id] = line
		}
	}
	for _, d := range all {
		row, found := rows[d.ID]
		switch {
		case !found:
			t.Errorf("tfg formats has no row for %s", d.ID)
		case len(row) < column || !strings.HasPrefix(row[column:], d.Name+" "):
			t.Errorf("the row for %s does not carry %q under NAME:\n%s\n%s", d.ID, d.Name, lines[0], row)
		}
	}

	code, listed, errOut := run(t, "formats", "--json")
	if code != cli.ExitOK {
		t.Fatalf("tfg formats --json: exit %d: %s", code, errOut)
	}
	var entries []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(listed), &entries); err != nil {
		t.Fatalf("tfg formats --json is not a list: %v", err)
	}
	named := map[string]string{}
	for _, e := range entries {
		named[e.ID] = e.Name
	}
	for _, d := range all {
		if named[d.ID] != d.Name {
			t.Errorf("tfg formats --json names %s %q, the registry %q", d.ID, named[d.ID], d.Name)
		}
		code, one, errOut := run(t, "formats", d.ID)
		if code != cli.ExitOK {
			t.Fatalf("tfg formats %s: exit %d: %s", d.ID, code, errOut)
		}
		if !strings.Contains(one, "\n  name       "+d.Name+"\n") {
			t.Errorf("tfg formats %s does not give the name %q on a line of its own:\n%s", d.ID, d.Name, one)
		}
	}
}
