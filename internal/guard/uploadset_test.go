package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A type cannot be allowed and denied at once.
//
// The two lists never saw each other, so "--allow pdf --deny pdf" laid out a
// set holding a PDF expecting accept and a PDF expecting reject for
// extension_rule. Both reach the manifest, so any suite running that set
// contradicts itself whatever the system under test does - and nothing said a
// word, which is untouchable rule 6 on the silence that is hardest to notice:
// the one where nothing fails.
//
// Found by review on 2026-09-22, and the whole set is refused rather than one
// half dropped, because dropping a half chooses for somebody which of the two
// they meant.
func TestAnExtensionCannotBeAllowedAndDeniedAtOnce(t *testing.T) {
	// The state this guard is about, asserted rather than assumed: the
	// declared defaults have to be buildable, or the refusal below would be
	// the preset refusing everything.
	if _, err := preset.Expand("upload-validation", preset.Args{}); err != nil {
		t.Fatalf("the preset refuses its own defaults, so nothing here is about an overlap: %v", err)
	}

	for _, id := range []string{"pdf", "targz"} {
		_, err := preset.Expand("upload-validation", preset.Args{"allow": id, "deny": id})
		if err == nil {
			t.Errorf("%s is allowed and denied at once and the set was built anyway - one of its "+
				"files expects accept and another expects reject, and both are %s", id, id)
			continue
		}
		if !strings.Contains(err.Error(), id) {
			t.Errorf("the refusal for %s allowed and denied does not name it: %s", id, err)
		}
	}
}

// Every file of the denied group is named with the extension the registry
// declares for it.
//
// A format's extension is not always a dot and its id. targz is written
// .tar.gz, so a denied file built from the id alone was called denied.targz -
// a name no upload form has a rule about, which is the one thing that group
// exists to test. Found on 2026-09-22 while checking the overlap above.
//
// An entry this build has no format for keeps the extension as typed, which is
// the point of taking those at all.
func TestADeniedFileIsNamedWithTheExtensionTheRegistryDeclares(t *testing.T) {
	const denied = "targz,zip,exe"

	expanded, err := preset.Expand("upload-validation", preset.Args{"deny": denied})
	if err != nil {
		t.Fatalf("the preset refused %q: %v", denied, err)
	}
	doc, err := recipe.Parse(expanded.Source, "upload-validation.yaml")
	if err != nil {
		t.Fatalf("the preset wrote a recipe this build cannot read: %v", err)
	}

	checked := 0
	for _, target := range doc.Targets {
		if target.Group != "denied-types" {
			continue
		}
		checked++
		want := "." + strings.TrimPrefix(target.ID, "denied_")
		if desc, err := format.Get(strings.TrimPrefix(target.ID, "denied_")); err == nil {
			want = desc.Extension
		}
		if !strings.HasSuffix(target.Name, want) {
			t.Errorf("the denied file %s is called %q and this build writes that format as %q",
				target.ID, target.Name, want)
		}
	}
	if checked != 3 {
		t.Fatalf("%d denied file(s) for a list of three, so this guard is not looking at what it thinks",
			checked)
	}
	// One of the three has a multi-part extension and one is outside the
	// registry, or the two halves of this guard are the same half twice.
	if desc, err := format.Get("targz"); err != nil || desc.Extension == ".targz" {
		t.Errorf("targz is written %q in this build, so it no longer stands for the case this "+
			"guard was written about - find another format whose extension is not a dot and its id",
			desc.Extension)
	}
}
