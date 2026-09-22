package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The table preset takes the counts the spreadsheet itself takes.
//
// It has to declare them rather than read them: a preset is registered at init
// and the format registry has not necessarily finished filling by then, which
// is why preset.Global reads the formats when it is called instead. So the
// range is written down, and a written down number goes stale green. This is
// the thing that reddens instead.
//
// What it protects: a field on a screen and a refusal on the command line, both
// built from the declaration. A preset offering "1 to 200000 rows" beside a
// build whose sheet stops at 50000 would take the value, draw the field, refuse
// the run - and blame the sheet for a range this preset had promised.
func TestTheTabularPresetTakesTheRangesTheSheetDeclares(t *testing.T) {
	sheet, err := format.Get("xlsx")
	if err != nil {
		t.Fatalf("this build has no spreadsheet, so the preset built on one cannot be checked: %v", err)
	}
	p, err := preset.Get("tabular-import")
	if err != nil {
		t.Fatalf("this build has no tabular-import preset: %v", err)
	}

	checked := 0
	for _, param := range p.Parameters {
		declared, ok := settingNamed(sheet, param.Name)
		if !ok {
			t.Errorf("the preset declares a parameter called %q and the spreadsheet has no such "+
				"setting, so nothing says what its range should be", param.Name)
			continue
		}
		if param.Min != declared.Min || param.Max != declared.Max {
			t.Errorf("the preset takes %s from %d to %d and the spreadsheet takes %d to %d - "+
				"the field and the refusal are built from the first and the run is judged by the second",
				param.Name, param.Min, param.Max, declared.Min, declared.Max)
		}
		if param.Unit != declared.Unit {
			t.Errorf("the preset counts %s in %q and the spreadsheet counts it in %q",
				param.Name, param.Unit, declared.Unit)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no parameter was compared - this guard would pass against any range ever written")
	}
	t.Logf("%d parameter(s) with the range the spreadsheet declares", checked)
}

// Every dialect the table format declares is in the set.
//
// The group is built by walking what CSV declares rather than by a list written
// in the preset, and this is what holds that true from the other side: a fifth
// dialect setting has to arrive in the set without anybody adding it, and one
// that stopped arriving has to redden.
//
// One file per value, minus the one the base already stands for. The base is a
// file of its own holding every setting at its default, so the count is exact
// rather than a lower bound: a set where one axis quietly produced nothing
// would still hold files for the others and still look like a set.
//
// What it does NOT check is which of those files expect accept and which say
// the answer is the reader's policy. That line is drawn in the preset, by a
// standard rather than by the registry - RFC 4180 asks for CRLF and for
// quoting, and no registry knows that.
func TestEveryDialectTheTableDeclaresIsInTheSet(t *testing.T) {
	csv, err := format.Get("csv")
	if err != nil {
		t.Fatalf("this build has no csv, so the dialect half of this set cannot be checked: %v", err)
	}
	expanded, err := preset.Expand("tabular-import", preset.Args{})
	if err != nil {
		t.Fatalf("the preset refused its own defaults: %v", err)
	}
	doc, err := recipe.Parse(expanded.Source, "tabular-import.yaml")
	if err != nil {
		t.Fatalf("the preset wrote a recipe this build cannot read: %v\n%s", err, expanded.Source)
	}

	dialects := targetsInGroup(doc, "csv-dialects")
	if len(dialects) == 0 {
		t.Fatal("the set holds no dialect group at all, so this guard is asserting nothing")
	}

	axes := 0
	wanted := 1 // the base, which stands for every setting at its default
	for _, p := range csv.Properties {
		if p.Kind != format.PropertyChoice && p.Kind != format.PropertyBool {
			continue
		}
		axes++
		wanted += len(valuesOfSetting(p)) - 1
		assertAxisIsVaried(t, dialects, p)
	}
	if axes == 0 {
		t.Fatal("csv declares no setting that names a shape, so there is no dialect to vary")
	}
	if len(dialects) != wanted {
		t.Errorf("the dialect group holds %d files and %d setting(s) with their values come to %d",
			len(dialects), axes, wanted)
	}
	t.Logf("%d dialect file(s) across %d setting(s)", len(dialects), axes)
}

// assertAxisIsVaried checks that every value of one setting appears somewhere in
// the group, the default included - it is the base that carries that one.
func assertAxisIsVaried(t *testing.T, dialects []recipe.Target, p format.Property) {
	t.Helper()

	seen := map[string]bool{}
	for _, target := range dialects {
		seen[target.Properties[p.Name]] = true
	}
	for _, value := range valuesOfSetting(p) {
		if !seen[value] {
			t.Errorf("csv takes %s=%s and no file of the dialect group is written that way",
				p.Name, value)
		}
	}
}

func valuesOfSetting(p format.Property) []string {
	if p.Kind == format.PropertyBool {
		return []string{"false", "true"}
	}
	return p.Choices
}

func targetsInGroup(doc *recipe.Recipe, group string) []recipe.Target {
	var out []recipe.Target
	for _, target := range doc.Targets {
		if target.Group == group {
			out = append(out, target)
		}
	}
	return out
}
