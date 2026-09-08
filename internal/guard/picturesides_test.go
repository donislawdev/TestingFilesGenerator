package guard

import (
	"errors"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagedim"
)

// The backstop under the ten picture formats has to refuse what it says it
// refuses.
//
// Ten formats used to carry their own copy of this and every copy checked the
// range. Measured on 2026-09-08, none of those checks could be reached: the
// registry refuses a width outside the declared bounds first, in engine.go and
// in recipe/target.go, which are the only two production callers, and a run
// asking png for a width of 999999 comes back as exit 4 with the sentence built
// from the declaration. Nothing in this package reached a generator with a
// value outside its range either.
//
// That is the shape this project calls a defence nothing can redden, and it had
// two honest endings. It was kept rather than deleted, for the reason
// textenc.Parse gives about its own branches - the function is callable
// directly and a generator that trusts its input is one registry change away
// from encoding a picture nobody ordered - and keeping it puts the burden here:
// a backstop with no test is a comment. This is the test, so the ten copies
// became one copy that something presses.
//
// What it does NOT claim is that a person can reach these. A person cannot, and
// the guards on the declaration are what prove the sentence they do get.
func TestThePictureSideBackstopRefusesWhatTheDeclarationWould(t *testing.T) {
	const largest = 256

	for _, c := range []struct {
		name  string
		key   string
		value string
	}{
		{"above the largest side", imagedim.SettingWidth, "257"},
		{"below the smallest side", imagedim.SettingHeight, "0"},
		{"a negative side", imagedim.SettingWidth, "-1"},
		{"not a number at all", imagedim.SettingHeight, "wide"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := imagedim.Value("ico", c.key, map[string]string{c.key: c.value}, largest, 32)
			if err == nil {
				t.Fatalf("%s=%q was accepted and came back as %d", c.key, c.value, got)
			}

			// The structured refusal rather than a sentence, because whatever
			// reports it asks for the parts by name. Nine of the ten built a
			// string instead and the tenth did this.
			var refused *format.PropertyValueError
			if !errors.As(err, &refused) {
				t.Fatalf("refused %s=%q with %T, which nothing can take apart: %v", c.key, c.value, err, err)
			}
			if refused.Format != "ico" {
				t.Errorf("the refusal names %q rather than the format that asked", refused.Format)
			}
			if refused.Key != c.key {
				t.Errorf("the refusal names the setting %q rather than %q", refused.Key, c.key)
			}
			if refused.Value != c.value {
				t.Errorf("the refusal quotes %q rather than what was written, %q", refused.Value, c.value)
			}
		})
	}
}

// A side inside the range is not refused, and that is the control.
//
// Without it the test above passes for a function that refuses everything,
// which would be a backstop that has become a wall - and a wall here means no
// picture format can be given a size at all.
func TestASideInsideTheRangeIsTakenAsWritten(t *testing.T) {
	const largest = 256

	for _, value := range []string{"1", "32", "256"} {
		got, err := imagedim.Value("ico", imagedim.SettingWidth,
			map[string]string{imagedim.SettingWidth: value}, largest, 99)
		if err != nil {
			t.Fatalf("width=%q is inside the range and was refused: %v", value, err)
		}
		if want := atoi(value); got != want {
			t.Errorf("width=%q came back as %d", value, got)
		}
	}
}
