package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A setting is described with the same words whether the box is empty or the
// value was refused.
//
// Two sentences describe every declared setting: Allowed under an empty field,
// and Allows after somebody typed something wrong. For four of the five kinds
// they agreed word for word. Size did not - "a size written the way any size
// is, such as 2mb or a plain byte count" against "a size such as 2mb, or a
// plain byte count" - so what a person read depended on whether they had
// already made a mistake. O64.
//
// Asked of the registry rather than of one property, because the point is that
// no kind may drift, and a size setting is the one that did.
func TestASettingIsDescribedTheSameWayEmptyOrRefused(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		for _, p := range d.Properties {
			if p.Kind != format.PropertySize {
				continue
			}
			// Allowed carries the default on the end, because it is read under
			// a field where the default is worth knowing. Allows never does -
			// somebody who typed a value is not choosing whether to.
			want := p.Allowed()
			if cut := strings.Index(want, ", default "); cut >= 0 {
				want = want[:cut]
			}
			got := p.Allows("this is not a size")
			if got != "it takes "+want {
				t.Errorf("%s.%s is described two ways.\n"+
					"  empty field:  %q\n"+
					"  after a typo: %q\n"+
					"Reason: both come from the declaration, so one of them is a second copy"+
					" of a sentence that already exists.\n"+
					"What to do: build them from the same words, the way sizePhrase does.",
					d.ID, p.Name, want, got)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a size setting, so this guard checked nothing")
	}
	t.Logf("%d size setting(s), each described the same way empty or refused", checked)
}

// The refusal about an empty output directory does not tell the reader to do
// the thing that has just been refused.
//
// It used to end "or leave it out to use the current one". That is true of a
// recipe file, and it is advice a person in the window CANNOT take: leaving it
// out is exactly what they did, and all three screens refuse it. A refusal that
// names an impossible way out is worse than one that names none, because the
// reader spends the attempt. O125.
func TestTheEmptyDirectoryRefusalDoesNotOfferAWayOutThatIsRefused(t *testing.T) {
	_, err := engine.Plan(
		[]engine.Target{{ID: "files", Format: "txt", Sizes: engine.Uniform(1, 1024)}},
		engine.Options{OutDir: ""},
	)
	if err == nil {
		t.Fatal("an empty output directory was accepted, so this guard has nothing to read")
	}
	said := err.Error()
	if !strings.Contains(said, "output directory is empty") {
		t.Fatalf("the run refused for another reason, so this guard read the wrong message: %q", said)
	}
	for _, offer := range []string{"leave it out", "leaving it out", "omit it"} {
		if strings.Contains(strings.ToLower(said), offer) {
			t.Errorf("the refusal says %q.\nFull message: %q\n"+
				"Reason: every surface refuses an empty directory, so that is not a way out"+
				" - in the window it is what the person just did.\n"+
				"What to do: say what to type, and nothing about not typing it.", offer, said)
		}
	}
}
