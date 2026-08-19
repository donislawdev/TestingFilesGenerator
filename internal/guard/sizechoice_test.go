package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Every one of the three ways of stating a size says that only one may be used.
//
// They stand side by side as equals and the engine refuses a batch that fills
// in more than one, but the rule was written under the FIRST of them only. So
// somebody reading the third had nothing to learn it from: the two beside the
// first described themselves and said nothing about excluding each other
// (O114).
//
// It walks the three hints rather than the screen, because what is being
// guarded is that the sentence exists on all three of them - a screen level
// check would pass on a screen that happened to show only one.
func TestEveryWayOfStatingASizeSaysOnlyOneMayBeUsed(t *testing.T) {
	for name, hint := range map[string]string{
		text.FieldSize:      text.HintSizeExact,
		text.FieldSizeRange: text.HintSizeRange,
		text.FieldBoundary:  text.HintBoundary,
	} {
		if !strings.Contains(hint, strings.TrimSpace(text.OneSizeSettingOnly)) {
			t.Errorf("the line under %q is %q and does not say that only one of the three may be "+
				"filled in.\nSomebody reading this one has nowhere else to learn it from, and the "+
				"engine refuses the batch if they get it wrong.", name, hint)
		}
	}
}
