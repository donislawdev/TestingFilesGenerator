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
// Every preset since 2026-09-24, and a refusal's words as well as a source's
// bytes. Until then only size-boundaries was pinned, and the change that day
// was to how the other four work their sets out: asking the format for its
// smallest size once rather than for every file, and planning a file the set
// holds twelve times once (docs/GUI-MEMORY-2026-09-23.md section 4j). Neither
// may move a byte, so the gate came first and was measured on the tree before
// the change. The cases are the ones that reach what changed - other formats
// in allow, which ask for other floors, a limit small enough that a refusal
// names the floor, and a spread narrow enough to reach it.
//
// What to do when it goes red: decide, rather than update. The sum moving is a
// breaking change under D11 - a major, a Breaking entry in the changelog, and
// the owner's decision, because untouchable rule 12 says the assistant does not
// raise the version. A refactor that moved it is a refactor to undo.
func TestEjectingAPresetGivesTheBytesItAlwaysGave(t *testing.T) {
	// bytes and sum are the whole document, refused the whole refusal. The
	// first row was measured 2026-09-08, the rest on 2026-09-24 before the
	// change they guard.
	pinned := []struct {
		id      string
		args    preset.Args
		bytes   int
		sum     string
		refused string
	}{
		{id: "size-boundaries", args: preset.Args{"format": "pdf", "limit": "10mb"}, bytes: 1298, sum: "2733cf63db40465fb97e26790d668d65ea01f5e94927a44ddf0869399beee2bb"},
		{id: "size-boundaries", args: preset.Args{"format": "png", "limit": "1mb"}, refused: "the preset size-boundaries cannot build this set - under_1mb would be 0 B, and a file cannot be smaller than nothing. Raise the limit above 1048650 B, narrow the spread, or choose a format with a smaller minimum. The limit asked for was 1048576 B."},
		{id: "size-boundaries", args: preset.Args{"format": "jpg", "limit": "10mb"}, bytes: 1298, sum: "29c7e0a133fb97fdf9d19fb40d0d96ad97c4a1fef67556a9dc734b520f7d9b09"},
		{id: "size-boundaries", args: preset.Args{"format": "jpg", "limit": "2kb", "spread": "1kb"}, bytes: 640, sum: "1346f4a7ae514fe2d15de426b910e308442f66fa97b485932d8fa1f5df56b910"},
		{id: "size-boundaries", args: preset.Args{"format": "jpg", "limit": "300", "spread": "100"}, refused: "the preset size-boundaries cannot build this set - under_100 would be 200 B and the smallest JPG this build makes is 602 B. Raise the limit above 702 B, narrow the spread, or choose a format with a smaller minimum. The limit asked for was 300 B."},
		{id: "upload-validation", args: preset.Args{}, bytes: 4116, sum: "a75039d859ee25d5ea5fd463ac2a774aacb1c45d013cf7ef3b80d46e0688fb78"},
		{id: "upload-validation", args: preset.Args{"limit": "5mb"}, bytes: 4108, sum: "4b7e716e2e199837b3c2bef228921e0087e894c49a8cf868a246c0ac636b9a35"},
		{id: "upload-validation", args: preset.Args{"limit": "3kb"}, refused: "the preset upload-validation cannot build this set - allowed_pdf would be 1536 B and the smallest PDF this build makes is 3415 B. Raise the limit to 6830 B or more, or take pdf out of the allowed types. The limit asked for was 3072 B."},
		{id: "upload-validation", args: preset.Args{"allow": "docx,gif"}, bytes: 3816, sum: "33975195adf794dd9f6e96dbec8a9775bfbda3c4b7dfd3524475f70c39496059"},
		{id: "upload-validation", args: preset.Args{"allow": "xlsx,ico,wav"}, bytes: 4125, sum: "c46aba3ad1aa6bf3ac2442bde058b9802b4d4f72375fd2f7cbffc6fab1620d6f"},
		{id: "upload-validation", args: preset.Args{"bulk": "3", "far-over": "off"}, bytes: 3933, sum: "98bc51d5edb94a80fb764a03915eccc6997b70fa8eac153578320b3128cb6d95"},
		{id: "upload-validation", args: preset.Args{"deny": "exe,js"}, bytes: 3767, sum: "55d9946e7233761716849f52517cd58f7a38c8748f4cdae8620afade0681621c"},
		{id: "tabular-import", args: preset.Args{}, bytes: 3369, sum: "fad20b41756327a6e85da93715fb09b20db394fb23e4b3ce22bdaf6e11b20726"},
		{id: "tabular-import", args: preset.Args{"rows": "100"}, bytes: 3367, sum: "ee28ae5421d4717fb24ee6dfbef53f7a54e3a00105e5015f884d59768181d552"},
		{id: "tabular-import", args: preset.Args{"columns": "5"}, bytes: 3367, sum: "580d526546720b851ac7d834b97c163f5ff50b29caeff5b27fcc1ba00e5bb952"},
		{id: "text-encoding", args: preset.Args{}, bytes: 4570, sum: "de27b9dc6c646baebaa0b16019ba3ce15d0f1d145941d376b47263de242998e6"},
		{id: "text-encoding", args: preset.Args{"sample": "8kb"}, bytes: 4570, sum: "f87c73864e5f517abb08b50393cd9a1681a90a30560c1d14dfddcf31e8037479"},
		{id: "empty-and-minimal", args: preset.Args{}, bytes: 3811, sum: "80641962ac9dfb303f812fd78e0a0d1080f714094d159d7b10448291debb9279"},
		{id: "empty-and-minimal", args: preset.Args{"formats": "jpg,png,txt"}, bytes: 743, sum: "4fd23e4b06a2e27ede987ab48a2cc302cc2accb9f948c7d5d25f675681f31f69"},
		// The preset of unusual file names, measured 2026-09-25 on its first
		// build: the default, and a format whose extension is a byte longer,
		// since the names about length are made to a length with it.
		{id: "filename-handling", args: preset.Args{}, bytes: 10324, sum: "fc051b2285c0efed30bc5e19920e6f08e25fc8f95e3b1be0ed8d228a25310e43"},
		{id: "filename-handling", args: preset.Args{"format": "docx"}, bytes: 10418, sum: "65fcae1471cd3b0111cae3dba14cd7d3d39beb6f998dc665b3dbaec898aade86"},
	}

	for _, want := range pinned {
		expanded, err := preset.Expand(want.id, want.args)
		if want.refused != "" {
			if err == nil || err.Error() != want.refused {
				t.Errorf("%s at %v was refused with\n  %q\nuntil now, and now gives\n  %v", want.id, want.args, want.refused, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s refused %v: %v", want.id, want.args, err)
			continue
		}
		sum := sha256.Sum256(expanded.Source)
		got := hex.EncodeToString(sum[:])
		if len(expanded.Source) == want.bytes && got == want.sum {
			continue
		}
		t.Errorf("ejecting %s at %v gives %d B and %s, and it has given %d B and %s until now.\n"+
			"Every manifest written from this preset carries a hash of these bytes, so this is a "+
			"breaking change under D11 rather than a number to update here.\n%s",
			want.id, want.args, len(expanded.Source), got, want.bytes, want.sum, expanded.Source)
	}
}
