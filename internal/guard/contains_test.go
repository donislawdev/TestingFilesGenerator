package guard

import (
	stdzip "archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// onlyArchive is the single .zip the run produced. Failing when there is not
// exactly one keeps a guard from quietly inspecting the wrong file.
func onlyArchive(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var found []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".zip") {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one archive in %s, found %d", dir, len(found))
	}
	return found[0]
}

// archiveMembers reads the archive back with the standard library reader
// rather than with anything this project wrote. A member list produced by our
// own writer would agree with itself whatever it did.
func archiveMembers(t *testing.T, path string) ([]string, []int64) {
	t.Helper()
	r, err := stdzip.OpenReader(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer r.Close()

	var names []string
	var sizes []int64
	for _, f := range r.File {
		// The generator's own padding entry is not a member somebody asked
		// for, and counting it would make every assertion below wrong.
		if strings.HasPrefix(f.Name, "tfg-") {
			continue
		}
		names = append(names, f.Name)
		sizes = append(sizes, int64(f.UncompressedSize64))
	}
	return names, sizes
}

func sizeText(n int64) string { return strconv.FormatInt(n, 10) }

// "an archive holds real files of other formats" is the feature docs/
// MVP-FORMATS.md 5.7 calls the key one, and the difference between this tool
// and an archive full of random bytes. Through a recipe it is contains.
//
// The guard that matters is not that an archive appears. It is that what comes
// out of it opens - checked here by size and by reading the members back, and
// by the oracle guard for the archive itself.

func containsRecipe(t *testing.T, dir, body string) string {
	t.Helper()
	return writeRecipe(t, dir, "version: 1\nseed: 7741\ntargets:\n"+body)
}

// The size of a container follows from what it holds, so a dry run reports a
// number without writing anything - that is why contains counts as a way of
// declaring a size at all (AR10, ARCHITECTURE.md section 9).
func TestAnArchiveDeclaringItsContentsNeedsNoSizeAndTheDryRunIsExact(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := containsRecipe(t, dir, `  - id: mixed
    format: zip
    contains:
      - format: pdf
        count: 3
        size: 8kb
      - format: png
        count: 2
        size: 4kb
output:
  dir: `+filepath.ToSlash(out)+`
`)

	// The summary is log, not data, so it goes to stderr - docs/CLI.md 2.
	code, _, dryReport := run(t, "generate", path, "--dry-run")
	if code != cli.ExitOK {
		t.Fatalf("dry run gave %d:\n%s", code, dryReport)
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		t.Errorf("the dry run wrote %d entries", len(entries))
	}

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}

	archive := onlyArchive(t, out)
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// The dry run has to have named the number the file actually has. An
	// estimate that reads like a measurement is the one thing ARCHITECTURE.md
	// section 9 forbids here.
	if !strings.Contains(dryReport, sizeText(info.Size())) {
		t.Errorf("the dry run did not predict the real size of %d B:\n%s", info.Size(), dryReport)
	}
}

// Both groups have to be in there, under names that do not collide, and the
// manifest has to say what is inside without anybody unpacking it.
func TestTheMembersOfAnArchiveAreRealFilesOfEveryFormatAsked(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := containsRecipe(t, dir, `  - id: mixed
    format: zip
    contains:
      - format: pdf
        count: 2
        size: 8kb
      - format: pdf
        count: 1
        size: 16kb
      - format: png
        count: 1
        size: 4kb
output:
  dir: `+filepath.ToSlash(out)+`
`)
	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}

	names, sizes := archiveMembers(t, onlyArchive(t, out))

	// Two groups of the same format must not both start at 0001 and overwrite
	// each other. Four members were asked for and four have to be there.
	if len(names) != 4 {
		t.Fatalf("expected 4 members, got %d: %v", len(names), names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("two members share the name %q, so one of them replaced the other", n)
		}
		seen[n] = true
	}

	var pdfs, pngs int
	for i, n := range names {
		switch {
		case strings.HasSuffix(n, ".pdf"):
			pdfs++
		case strings.HasSuffix(n, ".png"):
			pngs++
		}
		if sizes[i] == 0 {
			t.Errorf("member %q is empty, so it is not a real file of its format", n)
		}
	}
	if pdfs != 3 || pngs != 1 {
		t.Errorf("expected 3 PDFs and 1 PNG, got %d and %d: %v", pdfs, pngs, names)
	}

	// The sizes asked for are the sizes the members have. A container that
	// rounded them would break the one promise this tool makes.
	want := map[int64]int{8192: 2, 16384: 1, 4096: 1}
	got := map[int64]int{}
	for _, s := range sizes {
		got[s]++
	}
	for size, n := range want {
		if got[size] != n {
			t.Errorf("expected %d member(s) of %d B, got %d - the members are %v", n, size, got[size], sizes)
		}
	}

	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Files) != 1 {
		t.Fatalf("expected one file in the manifest, got %d", len(m.Files))
	}
	if m.Files[0].Properties["contains"] == nil {
		t.Error("the manifest does not say what the archive holds, so a test has to unpack it to find out")
	}
}

// A size stated beside contains wins, and the padding channel closes the
// difference. ARCHITECTURE.md section 9, last row of the table.
func TestASizeStatedBesideContentsWinsAndPaddingClosesTheGap(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := containsRecipe(t, dir, `  - id: padded
    format: zip
    size: 128kb
    contains:
      - format: pdf
        count: 2
        size: 8kb
output:
  dir: `+filepath.ToSlash(out)+`
`)
	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}
	info, err := os.Stat(onlyArchive(t, out))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 128*1024 {
		t.Errorf("the archive is %d B and the recipe asked for %d B - a stated size has to win over the contents",
			info.Size(), 128*1024)
	}

	// The members are still in there. Padding to a size must not mean the
	// contents were dropped for room.
	names, _ := archiveMembers(t, onlyArchive(t, out))
	pdfs := 0
	for _, n := range names {
		if strings.HasSuffix(n, ".pdf") {
			pdfs++
		}
	}
	if pdfs != 2 {
		t.Errorf("expected 2 PDFs inside the padded archive, got %d: %v", pdfs, names)
	}
}

// Every refusal, with the code from the frozen table. A new ending landing in
// RUNTIME would tell CI to file a bug report against us for something the user
// wrote.
func TestContentsThatCannotBeHonouredAreRefusedWithTheRightCode(t *testing.T) {
	cases := []struct {
		name string
		body string
		code int
		says string
	}{
		{
			name: "a format that holds nothing",
			body: "  - id: a\n    format: txt\n    contains: [{format: pdf, count: 1, size: 4kb}]\n",
			code: cli.ExitFormat,
			says: "holds no other files",
		},
		{
			name: "contents stated twice",
			body: "  - id: a\n    format: zip\n    contains: [{format: pdf, count: 1, size: 4kb}]\n    properties: {entries: 2}\n",
			code: cli.ExitRecipe,
			says: "both say what the archive holds",
		},
		{
			name: "a group with no size",
			body: "  - id: a\n    format: zip\n    contains: [{format: pdf, count: 2}]\n",
			code: cli.ExitRecipe,
			says: "has no size",
		},
		{
			name: "contents larger than the size stated",
			body: "  - id: a\n    format: zip\n    size: 1kb\n    contains: [{format: pdf, count: 3, size: 8kb}]\n",
			code: cli.ExitFormat,
			says: "already needs that much",
		},
		{
			name: "an archive inside an archive",
			body: "  - id: a\n    format: zip\n    contains: [{format: zip, count: 1, size: 4kb}]\n",
			code: cli.ExitFormat,
			says: "depth limit",
		},
		{
			name: "a key nobody can honour",
			body: "  - id: a\n    format: zip\n    contains: [{format: pdf, count: 1, size: 4kb, compression: high}]\n",
			code: cli.ExitRecipe,
			says: "compression",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := containsRecipe(t, t.TempDir(), c.body)
			code, stdout, errOut := run(t, "validate", path)
			if code != c.code {
				t.Errorf("exit %d, expected %d:\n%s", code, c.code, errOut)
			}
			if stdout != "" {
				t.Errorf("a refused run wrote to stdout:\n%s", stdout)
			}
			if !strings.Contains(errOut, c.says) {
				t.Errorf("the message does not say %q:\n%s", c.says, errOut)
			}
		})
	}
}

// Degenerate but legal, and both of these are things somebody really writes.
// An archive holding nothing is a documented case (docs/CLI.md section 7), and
// a group contributing nothing is what a count of zero means.
func TestAnArchiveHoldingNothingIsLegal(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"an empty contains list", "  - id: a\n    format: zip\n    contains: []\n"},
		{"a group of no files", "  - id: a\n    format: zip\n    contains: [{format: pdf, count: 0, size: 4kb}]\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := containsRecipe(t, dir, c.body+"output:\n  dir: "+filepath.ToSlash(out)+"\n")

			if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
				t.Fatalf("generate gave %d:\n%s", code, errOut)
			}
			names, _ := archiveMembers(t, onlyArchive(t, out))
			for _, n := range names {
				// The padding entry is the generator's own and may be there.
				if !strings.HasPrefix(n, "tfg-") {
					t.Errorf("an archive asked to hold nothing holds %q", n)
				}
			}
		})
	}
}
