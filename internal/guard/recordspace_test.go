package guard

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// A disk with room for a run's files and not for its record is refused before
// the first byte.
//
// The free space was held against the files alone until 2026-10-26, so such a
// disk took every file and then refused the manifest - and a file no manifest
// lists is a file no command of this tool can remove, because the manifest is
// the only authority over that (untouchable rule 7, R08 in
// docs/STABILITY-REVIEW-2026-10-26.md). The free space is handed in, so the
// guard asks at the byte rather than filling a disk.
func TestARunWithRoomForItsFilesButNotItsRecordIsRefused(t *testing.T) {
	plan := func(t *testing.T) ([]engine.PlannedFile, engine.Options) {
		opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test"}
		planned, err := engine.Plan([]engine.Target{txtTarget("t", 3, 1000)}, opt)
		if err != nil {
			t.Fatalf("planning: %v", err)
		}
		return planned, opt
	}
	planned, opt := plan(t)
	record, err := engine.PredictRecord(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("forecasting: %v", err)
	}
	files := engine.TotalBytes(planned)
	if record.Manifest <= 0 {
		t.Fatalf("the record was forecast at %d B, so there is nothing for the space to fall short of", record.Manifest)
	}

	// One byte short of the files and their record.
	opt.AvailableBytes = func(string) (int64, error) { return files + record.Bytes() - 1, nil }
	_, err = engine.Run(context.Background(), planned, opt)
	var noRoom *engine.SpaceError
	if !errors.As(err, &noRoom) {
		t.Fatalf("room for the files and all but one byte of the record gave %v, not the refusal about space", err)
	}
	if noRoom.Needed != files+record.Bytes() {
		t.Errorf("the refusal says %d B are needed, and the files and their record are %d B", noRoom.Needed, files+record.Bytes())
	}
	if entries, _ := os.ReadDir(opt.OutDir); len(entries) != 0 {
		t.Errorf("a refused run left %d entries in the directory", len(entries))
	}

	// Exactly enough is enough - the other side of the same line.
	planned, opt = plan(t)
	opt.AvailableBytes = func(string) (int64, error) { return files + record.Bytes(), nil }
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Errorf("room for exactly the files and their record was refused: %v", err)
	}
}
