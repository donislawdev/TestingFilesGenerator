package guard

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// The same feature the neighbouring file guards for ZIP: an archive holds real
// files of other formats, and a container that declares its contents needs no
// size because the size follows from them (AR10).
//
// It is guarded twice rather than once because the two formats work the size
// out differently. ZIP measures its own structure. TAR.GZ cannot, so it adds up
// tar blocks and gzip framing by arithmetic, and the contains path is where
// that arithmetic has to agree with children whose sizes it did not choose.
//
// Written after coverage showed the path was untouched: settleSize sat at 31%
// and groupsFor at 60% for this format, and both numbers were the contains
// branch. The property test and the golden values both go through properties.

func onlyTarGz(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var found []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") {
			found = append(found, filepath.Join(dir, e.Name()))
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one archive in %s, found %d", dir, len(found))
	}
	return found[0]
}

// tarGzMembers reads the archive back with the standard library rather than
// with anything this project wrote, for the reason the ZIP version gives: a
// member list produced by our own writer would agree with itself whatever it
// did.
func tarGzMembers(t *testing.T, path string) ([]string, []int64) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("the archive is not readable gzip: %v", err)
	}
	defer zr.Close()

	var names []string
	var sizes []int64
	tr := tar.NewReader(zr)
	for {
		head, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the tar stream: %v", err)
		}
		// The padding entry is not a member anybody asked for.
		if strings.HasPrefix(head.Name, "tfg-") {
			continue
		}
		names = append(names, head.Name)
		sizes = append(sizes, head.Size)
	}
	return names, sizes
}

func TestATarGzDeclaringItsContentsNeedsNoSizeAndTheDryRunIsExact(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := containsRecipe(t, dir, `  - id: mixed
    format: targz
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

	archive := onlyTarGz(t, out)
	info, err := os.Stat(archive)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !strings.Contains(dryReport, sizeText(info.Size())) {
		t.Errorf("the dry run did not predict the real size of %d B:\n%s", info.Size(), dryReport)
	}
}

func TestTheMembersOfATarGzAreRealFilesOfEveryFormatAsked(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := containsRecipe(t, dir, `  - id: mixed
    format: targz
    contains:
      - format: pdf
        count: 2
        size: 8kb
      - format: png
        count: 1
        size: 4kb
output:
  dir: `+filepath.ToSlash(out)+`
`)

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}

	names, sizes := tarGzMembers(t, onlyTarGz(t, out))
	if len(names) != 3 {
		t.Fatalf("expected 3 members, found %d: %v", len(names), names)
	}

	// Names must not collide, or one member would have overwritten another and
	// the count above would be the only thing to notice.
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("two members share the name %q", n)
		}
		seen[n] = true
	}

	want := map[string]int64{
		"pdf_0001.pdf": 8192,
		"pdf_0002.pdf": 8192,
		"png_0001.png": 4096,
	}
	for i, n := range names {
		size, ok := want[n]
		if !ok {
			t.Errorf("unexpected member %q", n)
			continue
		}
		if sizes[i] != size {
			t.Errorf("member %s is %d B and the recipe asked for %d B", n, sizes[i], size)
		}
	}

	// A member that is a real file of its format rather than random bytes is
	// the whole claim, so one is read back and checked for its magic.
	verifyFirstBytes(t, onlyTarGz(t, out))
}

// verifyFirstBytes pulls one member out and looks at what it starts with. The
// reference tool guard checks the archive, and this checks that what came out
// of it is the format the recipe named.
func verifyFirstBytes(t *testing.T, archive string) {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer zr.Close()

	magic := map[string]string{".pdf": "%PDF-", ".png": "\x89PNG"}
	checked := 0
	tr := tar.NewReader(zr)
	for {
		head, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the tar stream: %v", err)
		}
		want, ok := magic[filepath.Ext(head.Name)]
		if !ok {
			continue
		}
		buf := make([]byte, len(want))
		if _, err := io.ReadFull(tr, buf); err != nil {
			t.Fatalf("reading %s: %v", head.Name, err)
		}
		if string(buf) != want {
			t.Errorf("%s does not start like its format: got %q", head.Name, buf)
		}
		checked++
	}
	if checked == 0 {
		t.Error("no member was checked for its magic - this guard would pass without checking anything")
	}
}
