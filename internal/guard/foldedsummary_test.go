package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The folded line says what was CHOSEN, not what the fields started on.
//
// A menu cannot be empty. It opens on the value its format declared as the
// default, so asking "does this field have a value" answers yes for every menu
// on the screen before anybody has touched one. The line built from that answer
// is not a summary of anything - it is the format's whole declaration, written
// out.
//
// It went unnoticed while formats declared one or two settings between them.
// log declared seven on 2026-08-31 and the line ran off the right edge of the
// window, which is how it was found - from a screenshot, not from a test.
//
// Both halves matter and they fail in opposite directions. A line that names an
// untouched default is the defect above. A line that stays quiet about a value
// somebody DID pick is worse: the section arrives folded, so that setting is
// off the screen with nothing to say it is there.
func TestTheFoldedLineNamesWhatWasChosenAndNotTheDefaults(t *testing.T) {
	label := settingLabelFor(t, "log", "entry_format")

	t.Run("an untouched default is not named", func(t *testing.T) {
		screen := window.NewGenerate(newFakeHost(t))
		body := screen.Object()
		chooserIn(t, screen.Fields(), recipe.KeyFormat).SetSelected("log")

		if said := shownText(body); strings.Contains(said, label) {
			t.Errorf("the screen names %q with nothing chosen, so the folded line is listing the format's\n"+
				"declaration rather than anybody's choices. log declares seven settings and the line then\n"+
				"runs off the edge of the window.", label)
		}
	})

	t.Run("a chosen value is named", func(t *testing.T) {
		screen := window.NewGenerate(newFakeHost(t))
		body := screen.Object()
		fields := screen.Fields()
		chooserIn(t, fields, recipe.KeyFormat).SetSelected("log")

		// Open it, choose, shut it again - which is the order a person does it
		// in, and the only one that means anything: the line is worked out when
		// the section closes, because at build time nobody had chosen yet.
		//
		// A property field on this screen registers under the key the format
		// declared, with nothing in front of it - the recipe screen is the one
		// that prefixes, because there the same key belongs to several batches.
		openFold(t, body, "", text.SettingsFor("log"))
		chooserIn(t, fields, "entry_format").SetSelected("syslog")
		foldTitled(t, body, "", text.SettingsFor("log")).OnTapped()

		said := shownText(body)
		if !strings.Contains(said, "syslog") {
			t.Errorf("syslog was chosen and nothing on the screen says so. The settings section arrives\n" +
				"folded, so a value nobody can see is one they cannot tell they set.")
		}
	})
}

// settingLabelFor is how the window words one declared setting, taken from the
// registry so this guard cannot drift from what the screen draws.
func settingLabelFor(t *testing.T, formatID, property string) string {
	t.Helper()
	d, err := format.Get(formatID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Properties {
		if p.Name == property {
			return text.SettingLabel(p.Name)
		}
	}
	t.Fatalf("%s declares no %s, so this guard would check nothing", formatID, property)
	return ""
}
