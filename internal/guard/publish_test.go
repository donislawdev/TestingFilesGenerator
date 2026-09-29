package guard

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Every finished file gets its name through core.Publish, which never replaces
// a name somebody holds. The window it closes is O252, measured on 2026-09-29
// with a second writer spinning on the same names: a rename lost that writer's
// file 2544 times in 3000 on NTFS and around 1900 in 1950 on ext4, tmpfs and
// overlay. docs/O252-ANALIZA-2026-09-29.md has the table.

// The three answers the primitive gives, on an ordinary disk.
func TestPublishNeverReplacesANameSomebodyHolds(t *testing.T) {
	cases := []struct {
		what   string
		theirs []byte // nil: the name is free
	}{
		{"a free name", nil},
		{"a name holding somebody's file", []byte("THEIRS")},
		{"a name holding an empty file, the shape a claim used to have", []byte{}},
	}
	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			dir := t.TempDir()
			tmp, final := filepath.Join(dir, "f.txt.tfg-writing"), filepath.Join(dir, "f.txt")
			writeOrFail(t, tmp, []byte("OURS"))
			if c.theirs != nil {
				writeOrFail(t, final, c.theirs)
			}
			err := core.Publish(tmp, final)
			assertPublished(t, err, tmp, final, c.theirs)
		})
	}
}

// The fallbacks refuse a taken name the same way the call does.
//
// They are reached only on a filesystem that does not know the call - some
// network shares, macOS on a FAT stick - and no runner has one. A fallback
// nothing can reach is a defence nothing can turn red, so the chain is walked
// here through core.PublishThrough, with the calls it would fall back from
// answering "unsupported".
func TestPublishFallbacksRefuseATakenNameToo(t *testing.T) {
	unsupported := func(string, string) error { return errors.ErrUnsupported }
	chains := []struct {
		what string
		link func(string, string) error
	}{
		{"through a hard link", os.Link},
		{"through a look and a rename, the last resort", unsupported},
	}
	for _, chain := range chains {
		for _, theirs := range [][]byte{nil, []byte("THEIRS")} {
			name := chain.what + ", free name"
			if theirs != nil {
				name = chain.what + ", taken name"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				tmp, final := filepath.Join(dir, "f.txt.tfg-writing"), filepath.Join(dir, "f.txt")
				writeOrFail(t, tmp, []byte("OURS"))
				if theirs != nil {
					writeOrFail(t, final, theirs)
				}
				err := core.PublishThrough(tmp, final, unsupported, chain.link)
				assertPublished(t, err, tmp, final, theirs)
			})
		}
	}

	// And a failure that is not "unsupported" is the answer, not a reason to
	// try the next way. A permission refused and then walked round by a
	// rename would be the tool deciding it knows better than the system.
	t.Run("a real failure stops the chain", func(t *testing.T) {
		dir := t.TempDir()
		tmp, final := filepath.Join(dir, "f.txt.tfg-writing"), filepath.Join(dir, "f.txt")
		writeOrFail(t, tmp, []byte("OURS"))
		linked := false
		err := core.PublishThrough(tmp, final,
			func(string, string) error { return fs.ErrPermission },
			func(string, string) error { linked = true; return nil })
		if !errors.Is(err, fs.ErrPermission) {
			t.Errorf("a refused permission came back as %v", err)
		}
		if linked {
			t.Error("the chain went on to a hard link after a failure that was not \"unsupported\"")
		}
		if _, err := os.Lstat(final); err == nil {
			t.Error("the file got its name after the system refused it")
		}
	})
}

// A second writer spinning on the same names never loses its file.
//
// The race itself rather than its shape, because the shape is what the two
// guards above ask and a race is what O252 was about. The other writer takes a
// name with an exclusive create whenever it finds the name free, which is what
// another program - or a person - does. It is a goroutine rather than a
// process, and the window does not care: it lies between two calls to the
// system.
//
// Asserted both ways. The other writer has to have taken names at all, or this
// passes against a race nobody entered - measured, it takes nearly every one.
func TestAWriterSpinningOnTheNameNeverLosesItsFile(t *testing.T) {
	const names = 300
	dir := t.TempDir()
	nameOf := func(i int64) string { return filepath.Join(dir, fmt.Sprintf("f%04d.txt", i)) }

	var current atomic.Int64
	current.Store(-1)
	took := make([]atomic.Bool, names)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			i := current.Load()
			if i < 0 {
				continue
			}
			f, err := os.OpenFile(nameOf(i), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				continue
			}
			_, _ = f.WriteString("THEIRS")
			_ = f.Close()
			took[i].Store(true)
		}
	}()

	for i := int64(0); i < names; i++ {
		tmp := nameOf(i) + ".tfg-writing"
		writeOrFail(t, tmp, []byte("OURS"))
		current.Store(i)
		if err := core.Publish(tmp, nameOf(i)); err != nil {
			_ = os.Remove(tmp)
		}
	}
	close(stop)
	wg.Wait()

	taken, lost := 0, 0
	for i := int64(0); i < names; i++ {
		if !took[i].Load() {
			continue
		}
		taken++
		if got, _ := os.ReadFile(nameOf(i)); string(got) != "THEIRS" {
			lost++
		}
	}
	if taken == 0 {
		t.Fatal("the other writer took no name at all, so nothing here was raced")
	}
	if lost != 0 {
		t.Errorf("the other writer took %d names and lost its file under %d of them", taken, lost)
	}
	t.Logf("the other writer took %d of %d names and kept every file", taken, names)
}

// A file somebody puts under a planned name while the run writes it is left as
// it is, and that one file fails in its own words.
//
// Through the engine rather than the primitive, because the primitive being
// right says nothing about the run calling it - a writer that went back to
// os.Rename would leave every guard above green. The file is put there from
// the progress report, which the run makes while that same file is still being
// written: after the preflight has looked, before the file gets its name.
func TestAFileTakenDuringTheRunIsNotWrittenOver(t *testing.T) {
	dir := t.TempDir()
	var planted string
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName,
	}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4<<20)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	final := filepath.Join(dir, planned[0].Name)
	opt.OnProgress = func(engine.Progress) {
		if planted != "" {
			return
		}
		planted = final
		if err := os.WriteFile(final, []byte("THEIRS"), 0o644); err != nil {
			t.Errorf("putting a file under the planned name: %v", err)
		}
	}

	res, _ := engine.Run(context.Background(), planned, opt)
	if planted == "" {
		t.Fatal("the run reported no progress while writing, so nothing was put in its way")
	}
	if got, err := os.ReadFile(final); err != nil || string(got) != "THEIRS" {
		t.Errorf("the file put under %s during the run was written over: %q, %v", planned[0].Name, got, err)
	}
	if res == nil || res.Failures != 1 {
		t.Errorf("the run should report the one file it could not name, and reported %v", res)
	}
}

// A reservation swapped for a link while the run goes is not written through.
//
// The reservation is closed as soon as it is made, so that a run which never
// saves leaves nothing open (a Windows directory with an open file in it
// cannot be removed). The save opens it again, and between the two somebody
// can put a hard link to their own file under its name - which needs no
// privilege on Windows. The save has to ask the file it opened, not the name.
func TestAReservationSwappedForALinkIsNotWrittenThrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	victim := filepath.Join(t.TempDir(), "notes.txt")
	writeOrFail(t, victim, []byte("ORIGINAL"))

	r, err := manifest.Claim(path)
	if err != nil {
		t.Fatalf("reserving: %v", err)
	}
	reservation := manifest.ReservationPath(path)
	if err := os.Remove(reservation); err != nil {
		t.Fatalf("taking the reservation away: %v", err)
	}
	if err := os.Link(victim, reservation); err != nil {
		t.Fatalf("putting a hard link under the reservation's name: %v", err)
	}

	m := manifest.New("testing-files-generator", "0.0.0-test", "run_x", "tfg generate", 1, "windows", "amd64")
	if err := r.Save(m); err == nil {
		t.Error("the manifest was saved through a reservation somebody swapped for a link")
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != "ORIGINAL" {
		t.Errorf("the manifest was written into a file outside the directory: %q, %v", got, err)
	}
	if _, err := os.Lstat(path); err == nil {
		t.Error("a manifest got its name after the save refused")
	}
}

// A reservation left by a killed run stops the next run, a dry run included,
// and says what it is.
//
// A real run meets it twice - in the preflight and again when it reserves the
// name - but a dry run stops before reserving anything, so for a preview the
// preflight is the whole answer. A preview that says a run would succeed in a
// directory where the run will be refused answers a question nobody asked.
func TestALeftReservationStopsEvenADryRun(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("making the directory: %v", err)
	}
	left := manifest.ReservationPath(filepath.Join(out, engine.DefaultManifestName))
	writeOrFail(t, left, nil)

	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--dry-run", "--out", out)
	if code == 0 {
		t.Fatal("a dry run said yes in a directory where the run itself will be refused")
	}
	if !strings.Contains(errOut, "another run is already writing") || !strings.Contains(errOut, filepath.Base(left)) {
		t.Errorf("the refusal does not say a run is going or was killed, and name the file to remove:\n%s", errOut)
	}
}

func writeOrFail(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing %s: %v", filepath.Base(path), err)
	}
}

// assertPublished holds one publish to what it should have done: ours under
// the name when it was free, and theirs untouched with ours kept aside when it
// was not.
func assertPublished(t *testing.T, err error, tmp, final string, theirs []byte) {
	t.Helper()
	got, readErr := os.ReadFile(final)
	if readErr != nil {
		t.Fatalf("nothing under the final name afterwards: %v", readErr)
	}
	if theirs == nil {
		if err != nil {
			t.Fatalf("a free name was refused: %v", err)
		}
		if string(got) != "OURS" {
			t.Errorf("the free name holds %q rather than what was written", got)
		}
		if _, err := os.Lstat(tmp); err == nil {
			t.Error("the temporary name is still there after the file got its own")
		}
		return
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("a taken name did not come back as fs.ErrExist: %v", err)
	}
	if string(got) != string(theirs) {
		t.Errorf("the file under a taken name was written over: %q", got)
	}
	if _, err := os.Lstat(tmp); err != nil {
		t.Errorf("the refused file is gone from its temporary name, which is the caller's to remove: %v", err)
	}
}
