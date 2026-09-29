package guard

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A block of settings is named where it starts, the same way on both outputs
// that print a declaration.
//
// PDF was the first format with enough settings to need blocks - thirteen on
// 2026-09-29, eight of them about the document rather than its pages. The
// window draws the name of a block above its first setting (the picture is
// generate-pdf-settings), and tfg formats has to put it in the same place, or
// the two surfaces describe one format in two shapes. The machine readable
// list carries it too, because a window built on that list could not draw a
// heading it is never told about.
func TestASettingsBlockIsNamedWhereItStartsOnBothOutputs(t *testing.T) {
	d, err := format.Get("pdf")
	if err != nil {
		t.Fatalf("pdf is not registered: %v", err)
	}
	starts := map[string]string{}
	for i, p := range d.Properties {
		if p.Group != "" && (i == 0 || d.Properties[i-1].Group != p.Group) {
			starts[p.Group] = p.Name
		}
	}
	if len(starts) == 0 {
		t.Fatal("pdf declares no block of settings, so this guard checks nothing - point it at a format that does")
	}

	code, stdout, errOut := run(t, "formats", "pdf")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for group, first := range starts {
		heading := "\n  " + group + ":\n"
		if n := strings.Count(stdout, heading); n != 1 {
			t.Errorf("tfg formats pdf names the block %q %d times, and it starts once:\n%s", group, n, stdout)
		}
		if !strings.Contains(stdout, heading+"  "+first+" ") {
			t.Errorf("the block %q is not named directly above %s, its first setting:\n%s", group, first, stdout)
		}
	}

	code, stdout, errOut = run(t, "formats", "pdf", "--json")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var list []struct {
		Properties []struct{ Name, Group string } `json:"properties"`
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil || len(list) != 1 {
		t.Fatalf("the machine readable list is not one format of JSON: %v\n%s", err, stdout)
	}
	printed := map[string]string{}
	for _, p := range list[0].Properties {
		printed[p.Name] = p.Group
	}
	for _, p := range d.Properties {
		if got := printed[p.Name]; got != p.Group {
			t.Errorf("the JSON puts %s in the block %q and the declaration in %q", p.Name, got, p.Group)
		}
	}
}

// A block declared in two places is refused when the format registers.
//
// Both surfaces draw a heading where a block starts, so a block that stops and
// starts again would be named twice with other settings between the two - the
// reader would take them for two blocks. Refused at start up rather than drawn,
// because it is a mistake in a declaration and every build would carry it.
func TestABlockOfSettingsDeclaredInTwoPlacesIsRefused(t *testing.T) {
	txt, err := format.Get("txt")
	if err != nil {
		t.Fatalf("txt is not registered: %v", err)
	}
	for i, props := range [][]format.Property{
		{{Name: "a", Group: "X"}, {Name: "b", Group: "Y"}, {Name: "c", Group: "X"}},
		// Settings with no block come first, so one after a block is the
		// same mistake.
		{{Name: "a"}, {Name: "b", Group: "X"}, {Name: "c"}},
	} {
		// A name of its own each time: if the first were wrongly accepted,
		// the second would panic for being registered twice and pass for
		// the wrong reason.
		id := "guard-split-block-" + string(rune('a'+i))
		func() {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(fmt.Sprint(r), "two places") {
					t.Errorf("a declaration with a block in two places was not refused for it (%v): %+v", r, props)
				}
			}()
			format.Register(format.Descriptor{ID: id, Generator: txt.Generator, Properties: props})
		}()
	}
}
