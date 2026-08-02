package guard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The manifest is the only record of what a run wrote, so writing over it
// takes away the only thing able to remove those files again.
//
// Found by hand on 2026-08-03, not by any guard, and the shape is worth
// keeping in mind. The refusal to write over data files worked perfectly and
// the manifest was not part of it, so a second run into the same directory
// destroyed the record while dutifully protecting the bytes. On a run whose
// names collided it at least ended with an error. On a run whose names did
// not collide it ended with code 0 and said nothing at all, which is the
// silence rule broken in its purest form: nothing failed and data was lost.
//
// Measured before the fix: run one wrote two files and a manifest, run two
// wrote a manifest listing two different files, and cleanup then reported
// "manifest.json lists no files" for the first pair. Those two files could
// never be removed by this tool again.

// manifestOf is the path a run writes its manifest to.
func manifestOf(dir string) string {
	return filepath.Join(dir, engine.DefaultManifestName)
}

// seedDirectory performs one complete run so there is an earlier manifest to
// protect, and returns its bytes.
func seedDirectory(t *testing.T, dir string) []byte {
	t.Helper()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test", ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 2, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning the first run: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("the first run failed: %v", err)
	}
	if err := res.Manifest.Save(manifestOf(dir)); err != nil {
		t.Fatalf("saving the first manifest: %v", err)
	}
	body, err := os.ReadFile(manifestOf(dir))
	if err != nil {
		t.Fatalf("reading the first manifest: %v", err)
	}
	return body
}

// A second run is refused whether or not its file names collide. The names not
// colliding is the dangerous case, because nothing else stops it.
func TestASecondRunNeverWritesOverAnEarlierManifest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target engine.Target
	}{
		{"names collide", txtTarget("files", 2, 4096)},
		{"names do not collide", txtTarget("other", 2, 4096)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			before := seedDirectory(t, dir)

			opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test", ManifestName: engine.DefaultManifestName}
			planned, err := engine.Plan([]engine.Target{tc.target}, opt)
			if err != nil {
				t.Fatalf("planning the second run: %v", err)
			}

			res, runErr := engine.Run(context.Background(), planned, opt)
			if runErr == nil {
				t.Fatal("the second run went ahead and the earlier manifest was replaceable")
			}
			var collision *engine.CollisionError
			if !errors.As(runErr, &collision) {
				t.Errorf("refused with %T, expected a CollisionError so the caller answers with the right exit code", runErr)
			}
			if res.Started {
				t.Error("the run reported that it started, so the caller would write a manifest for it")
			}

			after, err := os.ReadFile(manifestOf(dir))
			if err != nil {
				t.Fatalf("reading the manifest back: %v", err)
			}
			if string(after) != string(before) {
				t.Error("the earlier manifest changed, so the files it described can no longer be cleaned up")
			}
		})
	}
}

// A dry run reaches the same verdict as the real run would.
//
// Both checks used to sit behind the dry run's early return, so --dry-run
// reported success for runs that refuse to start. The command exists to count
// and show before the disk is touched, which makes it the one place where a
// wrong answer costs the most.
func TestADryRunReachesTheSameVerdictAsTheRealRun(t *testing.T) {
	dir := t.TempDir()
	seedDirectory(t, dir)

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName, DryRun: true}
	planned, err := engine.Plan([]engine.Target{txtTarget("other", 2, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("the dry run promised a run that the real one refuses")
	}
	var collision *engine.CollisionError
	if !errors.As(runErr, &collision) {
		t.Errorf("the dry run refused with %T, expected the same CollisionError the real run gives", runErr)
	}
}

// The same for a run too big for the disk, and the dry run still writes
// nothing at all. Without the second half, "predict the real run" could be
// satisfied by simply performing it.
func TestADryRunSeesAFullDiskAndStillWritesNothing(t *testing.T) {
	dir := t.TempDir()

	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test", DryRun: true,
		ManifestName: engine.DefaultManifestName,
		// Injected rather than measured, so the guard does not need a full
		// disk to run and does not fill anybody's.
		AvailableBytes: func(string) (int64, error) { return 1024, nil },
	}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 4, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	var spaceErr *engine.SpaceError
	if !errors.As(runErr, &spaceErr) {
		t.Fatalf("the dry run refused with %T, expected a SpaceError", runErr)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the dry run left %d entries behind and it must write nothing at all", len(entries))
	}
}
