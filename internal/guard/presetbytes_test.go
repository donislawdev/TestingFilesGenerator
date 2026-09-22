package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// What a preset ejects is bytes somebody else's manifest remembers.
//
// The manifest carries a recipe_hash, computed from the source a run consumed,
// and a preset's source is what eject prints - the same bytes, which is the
// whole of PR5. So a tidy-up that moved a space would tell everyone holding a
// record from an earlier run that their recipe had changed, and nothing in this
// tree would have said a word.
//
// PRESET-FEASIBILITY-2026-09-08.md section 5 asked for a measured gate on
// exactly one refactor: pulling the set a declared limit produces out of
// size-boundaries so that upload-validation could use it too. The measurement
// was taken by hand on 2026-09-08 and again either side of that move on
// 2026-09-22, both times 1298 B and this sum. This is that measurement kept.
//
// What to do when it goes red: decide, rather than update. The sum moving is a
// breaking change under D11 - a major, a Breaking entry in the changelog, and
// the owner's decision, because untouchable rule 12 says the assistant does not
// raise the version. A refactor that moved it is a refactor to undo.
func TestEjectingAPresetGivesTheBytesItAlwaysGave(t *testing.T) {
	pinned := []struct {
		id   string
		args preset.Args
		// bytes and sum are the whole document, measured 2026-09-08 and
		// unchanged since.
		bytes int
		sum   string
	}{
		{
			id:    "size-boundaries",
			args:  preset.Args{"limit": "10mb", "format": "pdf"},
			bytes: 1298,
			sum:   "2733cf63db40465fb97e26790d668d65ea01f5e94927a44ddf0869399beee2bb",
		},
	}

	for _, want := range pinned {
		expanded, err := preset.Expand(want.id, want.args)
		if err != nil {
			t.Errorf("%s refused %v: %v", want.id, want.args, err)
			continue
		}
		sum := sha256.Sum256(expanded.Source)
		got := hex.EncodeToString(sum[:])
		if len(expanded.Source) == want.bytes && got == want.sum {
			continue
		}
		t.Errorf("ejecting %s at %v gives %d B and %s, and it has given %d B and %s since "+
			"2026-09-08.\n"+
			"Every manifest written from this preset carries a hash of these bytes, so this is a "+
			"breaking change under D11 rather than a number to update here.\n%s",
			want.id, want.args, len(expanded.Source), got, want.bytes, want.sum, expanded.Source)
	}
}
