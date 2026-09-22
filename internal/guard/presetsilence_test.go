package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A preset asked for something that empties one of its groups says so.
//
// Untouchable rule 6, on the one path where it is easiest to break: nothing
// FAILS. The set expands, the run succeeds, the manifest is honest about every
// file it holds - and it holds five groups where the card describes six,
// because a parameter the caller set emptied one. A set missing a group looks
// exactly like a set that never had it, and somebody goes looking for files
// that were never going to be there.
//
// Written on 2026-09-22 with the three presets that made it reachable. Until
// then no parameter of any preset could empty anything.
func TestAGroupAPresetLeavesOutIsSaidOutLoud(t *testing.T) {
	cases := []struct {
		id    string
		args  preset.Args
		gone  string
		about string
	}{
		{
			id: "empty-and-minimal", args: preset.Args{"formats": "zip"}, gone: "empty",
			about: "no archive has a legal empty form, so a set of archives has no empty half",
		},
		{
			id: "upload-validation", args: preset.Args{"bulk": "0"}, gone: "bulk-upload",
			about: "nought files in the mass upload is a legal thing to ask for",
		},
		{
			id: "upload-validation", args: preset.Args{"allow": "jpg"}, gone: "extension-content-mismatch",
			about: "one allowed type leaves nothing to name a file wrongly after",
		},
		{
			id: "upload-validation", args: preset.Args{"far-over": "off"}, gone: "",
			about: "the file well past the limit was turned off",
		},
	}

	for _, c := range cases {
		t.Run(c.id+" "+c.about, func(t *testing.T) {
			full := groupsOf(t, c.id, preset.Args{})
			narrowed := groupsOf(t, c.id, c.args)

			// The state this guard is about, asserted rather than assumed.
			// A parameter that stopped emptying the group would leave this
			// guard green while saying nothing - O118, a dozen times over.
			if c.gone != "" {
				if !full[c.gone] {
					t.Fatalf("%s at its defaults has no group called %q, so this case is not about anything",
						c.id, c.gone)
				}
				if narrowed[c.gone] {
					t.Fatalf("%v was supposed to empty the group %q and did not", c.args, c.gone)
				}
			}

			// And the set says more than it does when nothing is missing. The
			// sentence itself is not matched: a guard that looks for words
			// pins the wording rather than the behaviour, and the wording is
			// meant to improve.
			said := notesOf(t, c.id, c.args)
			for _, always := range notesOf(t, c.id, preset.Args{}) {
				said = without(said, always)
			}
			if len(said) == 0 {
				t.Errorf("%s asked for %v leaves something out and says nothing it does not say at its defaults",
					c.id, c.args)
			}
		})
	}
}

// groupsOf is every group the set holds at these values.
func groupsOf(t *testing.T, id string, args preset.Args) map[string]bool {
	t.Helper()

	expanded, err := preset.Expand(id, args)
	if err != nil {
		t.Fatalf("%s refused %v: %v", id, args, err)
	}
	doc, err := recipe.Parse(expanded.Source, id+".yaml")
	if err != nil {
		t.Fatalf("%s wrote a recipe this build cannot read: %v", id, err)
	}
	out := map[string]bool{}
	for _, target := range doc.Targets {
		out[target.Group] = true
	}
	return out
}

func notesOf(t *testing.T, id string, args preset.Args) []string {
	t.Helper()

	expanded, err := preset.Expand(id, args)
	if err != nil {
		t.Fatalf("%s refused %v: %v", id, args, err)
	}
	return expanded.Notes()
}

func without(all []string, one string) []string {
	out := make([]string, 0, len(all))
	for _, said := range all {
		if said != one {
			out = append(out, said)
		}
	}
	return out
}

// The encoding set holds every combination this build can write and none of the
// ones it cannot.
//
// Both halves, because either one alone is the guard that proves nothing. A set
// that simply expanded would pass a check for "no refused cell" by holding
// three files, and a check for "every cell" would demand two that XML refuses -
// the specification says a UTF-16 entity opens with a byte order mark, and the
// format enforces it.
//
// The registry is asked which is which rather than this file knowing. The rule
// belongs to XML today, and the preset was written so that the next text format
// bringing its own rule needs no line here and none there.
func TestTheEncodingSetHoldsEveryCombinationThisBuildCanWrite(t *testing.T) {
	expanded, err := preset.Expand("text-encoding", preset.Args{})
	if err != nil {
		t.Fatalf("the preset refused its own defaults: %v", err)
	}
	doc, err := recipe.Parse(expanded.Source, "text-encoding.yaml")
	if err != nil {
		t.Fatalf("the preset wrote a recipe this build cannot read: %v\n%s", err, expanded.Source)
	}

	held := map[string]bool{}
	for _, target := range doc.Targets {
		held[cellKey(target.Format, target.Properties)] = true
	}

	var refused, written int
	for _, desc := range format.All() {
		encoding, ok := settingNamed(desc, "encoding")
		if !ok {
			continue
		}
		for _, cell := range cellsOf(desc, encoding) {
			key := cellKey(desc.ID, cell)
			if formatRefuses(desc, cell) {
				refused++
				if held[key] {
					t.Errorf("the set holds %s, which this build refuses to write", key)
				}
				continue
			}
			written++
			if !held[key] {
				t.Errorf("this build writes %s and the set leaves it out", key)
			}
		}
	}

	if refused == 0 {
		t.Error("no combination of encoding and byte order mark is refused by any format in this " +
			"build, so the half of this guard about what the set leaves OUT is asserting nothing")
	}
	if written == 0 {
		t.Fatal("no combination is writable, so this guard is asserting nothing at all")
	}
	t.Logf("%d combination(s) written and in the set, %d refused and left out", written, refused)
}

// cellsOf is every combination of encoding and mark one format declares.
func cellsOf(desc format.Descriptor, encoding format.Property) []map[string]string {
	marks := []string{""}
	if _, ok := settingNamed(desc, "bom"); ok {
		marks = []string{"false", "true"}
	}
	var out []map[string]string
	for _, name := range encoding.Choices {
		for _, mark := range marks {
			cell := map[string]string{"encoding": name}
			if mark != "" {
				cell["bom"] = mark
			}
			out = append(out, cell)
		}
	}
	return out
}

// formatRefuses asks the format itself whether it will write this combination,
// at the size the format names as its smallest for it - so an answer that comes
// back is about the settings and not about the room they need.
func formatRefuses(desc format.Descriptor, cell map[string]string) bool {
	r := format.Request{Label: true, Properties: cell}
	r.Bytes = desc.SmallestAccepted(r)
	_, err := desc.Generator.Plan(r)
	return err != nil
}

func cellKey(id string, props map[string]string) string {
	mark := props["bom"]
	if mark == "" {
		mark = "false"
	}
	return id + " " + props["encoding"] + " bom=" + mark
}

// declaredSetting is one setting of a format as the format declares it.
func settingNamed(desc format.Descriptor, name string) (format.Property, bool) {
	for _, p := range desc.Properties {
		if p.Name == name {
			return p, true
		}
	}
	return format.Property{}, false
}

// A list of formats refused for a name this build does not have says what it
// does have.
//
// The person who typed heic has no other way to find out. The refusal for a
// single format has named the list since the registry was written, and a list
// parameter is where it would be dropped without anybody noticing: the value is
// split first, so the refusal is raised by the preset rather than by
// format.Get, and a preset that worded its own sentence would say "unknown
// format" and stop.
func TestAListOfFormatsRefusedNamesWhatThisBuildHas(t *testing.T) {
	lists := []struct{ id, param string }{
		{"empty-and-minimal", "formats"},
		{"upload-validation", "allow"},
	}
	if len(format.IDs()) == 0 {
		t.Fatal("this build registers no format, so there is no list for a refusal to name")
	}

	for _, list := range lists {
		_, err := preset.Expand(list.id, preset.Args{list.param: "heic"})
		if err == nil {
			t.Errorf("%s took %s=heic and this build has no such format", list.id, list.param)
			continue
		}
		for _, id := range format.IDs() {
			if !strings.Contains(err.Error(), id) {
				t.Errorf("%s refusing %s=heic does not name %s, which this build does have: %s",
					list.id, list.param, id, err)
				break
			}
		}
	}
}
