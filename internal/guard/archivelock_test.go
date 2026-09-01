package guard

import (
	stdzip "archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// The methods this build writes, and the bytes each one adds to an entry.
//
// The numbers are measured rather than derived, twice over and from two
// directions - the size of the whole file and the compressed size field in the
// local header of what 7-Zip writes. Written up in docs/MVP-FORMATS.md section
// 2.16. Repeated here on purpose: a guard that reads its expectation out of the
// code it is guarding proves the code agrees with itself.
var lockOverhead = map[string]int64{
	archive.AES128: 20,
	archive.AES192: 24,
	archive.AES256: 28,
}

// A locked archive is one a real archiver opens with the password and refuses
// without it.
//
// This is the guard the whole feature rests on, and it is deliberately asked of
// somebody else's program. Our own code can be sure it encrypted something and
// be wrong about every detail that matters - the counter mode counting the
// wrong way, the key derived from the wrong material, the extra field saying a
// key length the data does not have. None of that shows up as an error here. It
// shows up as 7-Zip saying no.
//
// Three answers are needed, not one. A reader that accepts the file proves the
// bytes are well formed. A reader that REFUSES the wrong password proves the
// verifier and the authentication code are real rather than decorative - and a
// reader that accepts anything would have passed the first check while proving
// nothing at all.
func TestARealArchiverOpensALockedArchiveAndRefusesTheWrongPassword(t *testing.T) {
	bin, ok := oracle.SevenZip()
	if !ok {
		t.Skip("7-Zip is not installed here, so nothing can say whether the file is really locked")
	}

	const password = "Secret123"
	dir := t.TempDir()

	for _, method := range []string{archive.AES128, archive.AES192, archive.AES256} {
		t.Run(method, func(t *testing.T) {
			path := filepath.Join(dir, "locked-"+method+".zip")
			writeLocked(t, path, 40*1024, method, password, 7741)

			if out, err := runSevenZip(bin, path, password); err != nil {
				t.Fatalf("7-Zip refused an archive locked with %s and given the right password: %v\n%s",
					method, err, out)
			}

			// And the contents come back out as themselves.
			//
			// This half was missing until a mutation found it, and the miss is
			// worth writing down because it is a property of AE-2 rather than
			// an oversight. An AE-2 entry carries a CRC of zero and its
			// authentication code is computed over the CIPHERTEXT, so a reader
			// checks that nobody edited the encrypted bytes and never that the
			// plaintext is what was put in. Turn the keystream counter the
			// wrong way round and 7-Zip still says "Everything is Ok" - the
			// file is well formed, the password verifies, the code matches,
			// and what comes out is noise.
			//
			// So the archive is opened for real and compared against the same
			// archive built without a lock, which holds the same children from
			// the same seed. Both sides are extracted by 7-Zip, so this is not
			// our own code judging its own arithmetic.
			open := filepath.Join(dir, "open-"+method+".zip")
			writeArchive(t, open, 40*1024, nil, 7741)

			locked := extract(t, bin, path, "txt_0001.txt", password)
			plain := extract(t, bin, open, "txt_0001.txt", "")
			if string(locked) != string(plain) {
				t.Errorf("%s: the file inside came back out different from the same file in an unlocked "+
					"archive, so the contents are being scrambled rather than encrypted", method)
			}
			if len(plain) == 0 {
				t.Fatal("the unlocked archive gave nothing back, so the comparison above proved nothing")
			}
			if _, err := runSevenZip(bin, path, "NotThePassword"); err == nil {
				t.Errorf("7-Zip accepted %s with the WRONG password, so nothing here proves the archive is locked",
					method)
			}
			// And with none at all, which is what a system under test does
			// before it knows there is a password.
			if _, err := runSevenZip(bin, path, ""); err == nil {
				t.Errorf("7-Zip read %s with no password at all", method)
			}
		})
	}
}

// Locking does not cost the exact size, which is the promise the format makes.
//
// The reason it can be kept is the measurement: a stream cipher does not change
// the length of what it encrypts, so a lock adds a FIXED count per entry and
// the size of the archive stays an exact function of its input. If that were
// not true the planning phase could not state a size without building the
// archive, and this setting could not exist at all - which is the difference
// between it and compression.
//
// Sizes that cross the awkward places: just above the floor, odd, and large
// enough that the padding entry is doing the work rather than the comment.
func TestALockedArchiveStillHitsTheSizeToTheByte(t *testing.T) {
	dir := t.TempDir()
	for _, method := range []string{archive.AES128, archive.AES192, archive.AES256} {
		for _, size := range []int64{12 * 1024, 12*1024 + 1, 40 * 1024, 300*1024 + 7} {
			path := filepath.Join(dir, "size.zip")
			writeLocked(t, path, size, method, "Secret123", 11)

			st, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if st.Size() != size {
				t.Errorf("%s at %d B: the file on disk is %d B", method, size, st.Size())
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// The bytes a lock adds are the bytes it was measured to add.
//
// Asked of the declaration rather than of a file, because this is the number
// the planning phase adds to reach an exact size, and a plan that is wrong here
// produces an archive the engine deletes for missing its own size. The guard
// above would catch that - this one says which number moved.
func TestTheCostOfALockIsTheMeasuredCost(t *testing.T) {
	for method, want := range lockOverhead {
		got := archive.Lock{Method: method, Password: "x"}.EntryOverhead()
		if got != want {
			t.Errorf("%s adds %d B to an entry and the measurement is %d", method, got, want)
		}
	}
	if open := (archive.Lock{}).EntryOverhead(); open != 0 {
		t.Errorf("an archive that is not locked adds %d B", open)
	}
}

// Two settings say one thing, so the two that disagree are refused.
//
// The pair is forced by the window rather than chosen. A menu cannot be empty -
// it opens on its declared default and sends it - so encryption arrives as
// "none" on every run from a window whether or not anybody looked at it, while
// a box somebody types in arrives empty and is left out. "Password given,
// encryption none" therefore cannot be told apart from "password given,
// encryption not considered".
//
// Guessing between them is what the refusal exists to prevent, and the bad
// guess is not hypothetical: measured on 2026-09-01, 7-Zip takes -p on a tar,
// exits 0, says nothing and writes a plaintext archive. Somebody asking for a
// locked fixture gets an open one.
//
// Both halves have to be named. A refusal saying only "password" leaves
// somebody looking at a filled in password box wondering what is wrong with it.
func TestTheTwoHalvesOfALockHaveToAgree(t *testing.T) {
	d := descriptorFor(t, "zip")

	cases := []struct {
		name  string
		props map[string]string
		about string
	}{
		{
			name:  "a password with the lock switched off",
			props: map[string]string{archive.Password: "Secret123", archive.Encryption: archive.NoEncryption},
			about: archive.Encryption,
		},
		{
			name:  "a lock with no password to lock it",
			props: map[string]string{archive.Encryption: archive.AES256},
			about: archive.Password,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := d.Generator.Plan(format.Request{Bytes: 40 * 1024, Seed: 7741, Properties: c.props})
			if err == nil {
				t.Fatal("accepted, so an archive was about to be built on two settings that contradict each other")
			}
			var value *format.PropertyValueError
			if !errors.As(err, &value) {
				t.Fatalf("the refusal is %T, so it lands on a different exit code than a bad value: %v", err, err)
			}
			if value.AboutSetting() != c.about {
				t.Errorf("the refusal is about %q and the field to mark is %q", value.AboutSetting(), c.about)
			}
			// Both halves, by name, in one sentence.
			if whole := value.Error(); !mentionsBothHalves(whole) {
				t.Errorf("the refusal names one half and not the other, so it reads as a defect in the box "+
					"somebody filled in: %q", whole)
			}
		})
	}
}

// mentionsBothHalves says whether a sentence talks about the password AND about
// whether the archive is locked. Both spellings of the second, because the
// refusal about a missing password names the method rather than the key.
func mentionsBothHalves(s string) bool {
	saysPassword := strings.Contains(s, "password")
	saysLock := strings.Contains(s, "encryption") || strings.Contains(s, "locked") ||
		strings.Contains(s, archive.AES256)
	return saysPassword && saysLock
}

// A format that cannot lock says why, in words about the format.
//
// "targz does not have a property called password" is true and sends somebody
// looking for the build that has it. There is no such build and there never
// will be: neither tar nor gzip has any encryption in it. That is a fact about
// the file format, and the refusal is the only place a person will read it.
//
// The exit code is half the point. A key nobody declares is a typo, which is
// USAGE. A key this format cannot carry is a well formed request no format here
// can deliver, which is FORMAT - and a script branching on the code would
// otherwise be told that a deliberate limitation was a spelling mistake.
func TestAFormatThatCannotLockSaysSoInItsOwnWords(t *testing.T) {
	d := descriptorFor(t, "targz")

	for _, key := range []string{archive.Password, archive.Encryption} {
		bad := d.CheckEachProperty(map[string]string{key: "Secret123"})
		if len(bad) == 0 {
			t.Fatalf("targz accepted %q, and 7-Zip accepting -p on a tar and writing plaintext is exactly "+
				"the behaviour this is here to not copy", key)
		}
		var unsupported *format.UnsupportedSettingError
		if !errors.As(bad[0], &unsupported) {
			t.Errorf("%q on targz is refused as %T, which reads as a gap in this build rather than as a "+
				"fact about tar: %v", key, bad[0], bad[0])
			continue
		}
		if !strings.Contains(unsupported.Why(), "tar") || !strings.Contains(unsupported.Why(), "gzip") {
			t.Errorf("the reason for %q does not name the two formats it is about: %q", key, unsupported.Why())
		}
		if unsupported.AboutSetting() != key {
			t.Errorf("the refusal about %q says it is about %q, so a window marks the wrong box",
				key, unsupported.AboutSetting())
		}
	}
}

// The password goes into the manifest, in plain text, on purpose.
//
// A locked fixture whose password is written nowhere is a file no test can
// open, which makes the whole run worth nothing. This is the one place in the
// tool where writing a secret down is the correct behaviour rather than a leak,
// and it is a decision by the owner rather than an oversight - so it gets a
// guard, because the next person to read it will assume it is a mistake.
func TestTheManifestCarriesThePasswordSoATestCanOpenTheFile(t *testing.T) {
	d := descriptorFor(t, "zip")

	plan, err := d.Generator.Plan(format.Request{
		Bytes: 40 * 1024, Seed: 7741,
		Properties: map[string]string{archive.Password: "Secret123", archive.Encryption: archive.AES256},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Properties[archive.Password]; got != "Secret123" {
		t.Errorf("the manifest records the password as %v, so nothing can open the file it describes", got)
	}
	if got := plan.Properties[archive.Encryption]; got != archive.AES256 {
		t.Errorf("the manifest records the encryption as %v", got)
	}

	// And an open archive says nothing about either, rather than saying none.
	// A key that is there reads as a thing that happened.
	open, err := d.Generator.Plan(format.Request{Bytes: 40 * 1024, Seed: 7741})
	if err != nil {
		t.Fatal(err)
	}
	if _, there := open.Properties[archive.Password]; there {
		t.Error("an archive with no password still records one")
	}
	if _, there := open.Properties[archive.Encryption]; there {
		t.Error("an archive that is not locked still records an encryption")
	}
}

// The salt comes from the run, so one recipe is one file.
//
// Encryption is where determinism is easiest to lose and hardest to notice: a
// salt from crypto/rand gives a perfectly good archive that differs on every
// run, and every check except this one would stay green. Untouchable rule 3
// says the bytes hold within a major version, and a fixture nobody can
// reproduce is not a fixture.
//
// The other half matters too. A salt that ignores the seed would make every
// archive identical, which is deterministic and wrong - two runs asking for
// different seeds have to differ.
func TestALockedArchiveIsTheSameFileForTheSameSeed(t *testing.T) {
	dir := t.TempDir()

	first := filepath.Join(dir, "a.zip")
	same := filepath.Join(dir, "b.zip")
	other := filepath.Join(dir, "c.zip")
	writeLocked(t, first, 40*1024, archive.AES256, "Secret123", 4242)
	writeLocked(t, same, 40*1024, archive.AES256, "Secret123", 4242)
	writeLocked(t, other, 40*1024, archive.AES256, "Secret123", 9999)

	a, b, c := readAll(t, first), readAll(t, same), readAll(t, other)
	if string(a) != string(b) {
		t.Error("the same recipe and seed gave two different archives, so the salt is drawn rather than derived")
	}
	if string(a) == string(c) {
		t.Error("two different seeds gave the same archive, so the salt ignores the seed")
	}

	// The salt itself, not the file it is in.
	//
	// Comparing whole archives is too blunt to say anything about the salt: the
	// children are seeded from the run too, so two seeds give different
	// contents and therefore different files whatever the salt does. A mutation
	// that made the salt ignore the seed entirely left the comparison above
	// green. This reads the sixteen bytes at the start of the entry, which is
	// where a WinZip AES entry keeps its salt, and asks them directly.
	if first, other := saltOf(t, first), saltOf(t, other); string(first) == string(other) {
		t.Errorf("two seeds gave the same salt %x, so the salt is not derived from the run - "+
			"every archive this build writes shares it", first)
	}
}

// saltOf is the salt at the start of the first entry of a locked archive.
//
// DataOffset is what makes this readable without a second zip parser: the
// standard library says where the entry's bytes begin, and for an AES entry the
// salt is the first thing there.
func saltOf(t *testing.T, path string) []byte {
	t.Helper()
	r, err := stdzip.OpenReader(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = r.Close() }()
	if len(r.File) == 0 {
		t.Fatalf("%s holds nothing, so there is no salt to read", path)
	}
	at, err := r.File[0].DataOffset()
	if err != nil {
		t.Fatalf("finding the data of %s: %v", r.File[0].Name, err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	// Sixteen, because these archives are locked with AES-256 and its salt is
	// half the key.
	salt := make([]byte, 16)
	if _, err := f.ReadAt(salt, at); err != nil {
		t.Fatalf("reading the salt of %s: %v", path, err)
	}
	return salt
}

// writeLocked builds one locked archive on disk.
func writeLocked(t *testing.T, path string, size int64, method, password string, seed uint64) {
	t.Helper()
	writeArchive(t, path, size, map[string]string{
		archive.Password: password, archive.Encryption: method,
	}, seed)
}

// writeArchive builds one archive on disk, locked or not.
func writeArchive(t *testing.T, path string, size int64, props map[string]string, seed uint64) {
	t.Helper()
	d := descriptorFor(t, "zip")
	plan, err := d.Generator.Plan(format.Request{
		Bytes: size, Seed: seed, Label: true, Properties: props,
	})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := d.Generator.Write(context.Background(), f, plan)
	closeErr := f.Close()
	if writeErr != nil {
		t.Fatalf("writing %s: %v", path, writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}

// runSevenZip tests an archive, with a password or without one.
func runSevenZip(bin, path, password string) (string, error) {
	args := []string{"t", path}
	if password != "" {
		args = append(args, "-p"+password)
	} else {
		// Without this the archiver asks at the console and the test hangs.
		args = append(args, "-p")
	}
	// The binary is the one the oracle package found and the path is a file
	// this test just wrote, so neither comes from anything a person typed.
	//nolint:gosec // both arguments are ours
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	out, err := exec.Command(bin, args...).CombinedOutput()
	return string(out), err
}

func readAll(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// descriptorFor is one registered format, or a failure naming it.
func descriptorFor(t *testing.T, id string) format.Descriptor {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatalf("%s is not registered: %v", id, err)
	}
	return d
}

// extract pulls one member out of an archive, using the archiver rather than
// anything of ours. Reading it back with our own code would be the generator
// checking its own arithmetic.
func extract(t *testing.T, bin, path, member, password string) []byte {
	t.Helper()
	args := []string{"e", "-so", path, member}
	if password != "" {
		args = append(args, "-p"+password)
	} else {
		args = append(args, "-p")
	}
	// Both arguments are ours: the binary came from the oracle package and
	// the path is a file this test just wrote.
	//nolint:gosec // the command is ours
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(bin, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("extracting %s from %s: %v\n%s", member, filepath.Base(path), err, errOut.String())
	}
	return out.Bytes()
}
