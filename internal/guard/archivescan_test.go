package guard

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/archivescan"
)

// The archive scanner, tfg tool archive-scan: what it finds in each kind of
// trap, what it ends with, that its own reader of a ZIP lists what archive/zip
// lists, and that the window says what the command line says.
// docs/SKANER-ARCHIWUM-2026-10-19.md.

// archiveFile writes bytes to a file of this name in a folder of the test's.
func archiveFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// scanArchive runs the tool on an archive and hands back what --json would print
// and the result both surfaces are drawn from.
func scanArchive(t *testing.T, path string, values map[string]string) (*archivescan.Report, tool.Result) {
	t.Helper()
	d, err := tool.Get(archivescan.ID)
	if err != nil {
		t.Fatal(err)
	}
	r, err := d.Start(context.Background(), tool.Request{Inputs: map[string]string{archivescan.InputArchive: path}, Values: values}, nil)
	if err != nil {
		t.Fatalf("%s was refused: %v", filepath.Base(path), err)
	}
	report, ok := r.Data.(*archivescan.Report)
	if !ok {
		t.Fatalf("the result of %s carries %T, not the report --json prints", filepath.Base(path), r.Data)
	}
	return report, r
}

// trap is one archive and what the scanner has to find in it.
type trap struct {
	what   string
	file   string
	data   []byte
	values map[string]string
	// found is every finding the report has to hold, and no other.
	found []string
	// names are entries one finding has to name.
	names map[string][]string
	safe  bool
	// kind is what the archive has to be read as, where that is the point:
	// a gzip of zeros begins as an empty TAR would, and is not one.
	kind string
}

// traps is an archive for each kind of trap, and the clean ones beside them.
func traps(t *testing.T) []trap {
	t.Helper()
	e, accent := string(rune(0xe9)), string(rune(0x301))
	clean := zipOf(t, zipPart{name: "a.txt", body: "hello"}, zipPart{name: "dir/b.txt", body: strings.Repeat("world", 100)})
	return []trap{
		{what: "a clean ZIP", file: "clean.zip", data: clean, safe: true},
		{what: "names Windows stores perfectly well, measured in engine/filename.go", file: "fine.zip", safe: true,
			data: zipOf(t, zipPart{name: "con.txt"}, zipPart{name: "nul.txt"}, zipPart{name: "aux"}, zipPart{name: "./a/./b.txt"})},
		{what: "entries leaving the folder", file: "leaves.zip", found: []string{"leaves_folder"},
			data: zipOf(t, zipPart{name: "../up.txt"}, zipPart{name: "/abs.txt"}, zipPart{name: "C:/drive.txt"},
				zipPart{name: "C:rel.txt"}, zipPart{name: "a/../../x"}, zipPart{name: `..\back.txt`}, zipPart{name: ".. /dots.txt"}, zipPart{name: "ok.txt"}),
			names: map[string][]string{"leaves_folder": {"../up.txt", "/abs.txt", "C:/drive.txt", "C:rel.txt", "a/../../x", `..\back.txt`, ".. /dots.txt"}}},
		{what: "a link in a ZIP", file: "link.zip", found: []string{"links"},
			data:  zipOf(t, zipPart{name: "link", body: "../../etc/passwd", mode: os.ModeSymlink | 0o777}),
			names: map[string][]string{"links": {"link"}}},
		{what: "entries landing on one file", file: "collide.zip", found: []string{"collides"},
			data: zipOf(t, zipPart{name: "Readme.txt"}, zipPart{name: "README.TXT"}, zipPart{name: "a"}, zipPart{name: "a/b"},
				zipPart{name: "caf" + e}, zipPart{name: "cafe" + accent}),
			names: map[string][]string{"collides": {"README.TXT", "a/b", "cafe" + accent}}},
		{what: "a name Windows strips and lands on another", file: "trail.zip", found: []string{"collides", "unwritable"},
			data: zipOf(t, zipPart{name: "trail"}, zipPart{name: "trail."})},
		{what: "names no system writes as they are", file: "names.zip", found: []string{"unwritable"},
			data:  zipOf(t, zipPart{name: "nul"}, zipPart{name: "q?.txt"}, zipPart{name: strings.Repeat("x", 256)}),
			names: map[string][]string{"unwritable": {"nul", "q?.txt"}}},
		{what: "a name at the entry other than the one listed", file: "twonames.zip", found: []string{"two_names"},
			data: handZip("good.txt", "evil.txt", false)},
		{what: "two entries sharing their bytes", file: "overlap.zip", found: []string{"overlapping", "two_names"},
			data: handZip("first", "first", true), names: map[string][]string{"overlapping": {"first", "second"}}},
		{what: "a ZIP built to explode, by its list", file: "bomb.zip", found: []string{"bomb"},
			data: zipOf(t, zipPart{raw: &zip.FileHeader{Name: "zeros", Method: zip.Deflate, UncompressedSize64: 2 << 30,
				CompressedSize64: uint64(len(deflated(t, "x")))}, body: deflated(t, "x")})},
		{what: "an entry holding more than its size", file: "longer.zip", found: []string{"longer_than_said"},
			data: zipOf(t, zipPart{raw: &zip.FileHeader{Name: "more", Method: zip.Deflate, UncompressedSize64: 10,
				CompressedSize64: uint64(len(deflated(t, strings.Repeat("y", 1000))))}, body: deflated(t, strings.Repeat("y", 1000))})},
		{what: "contents that do not match their checksum", file: "crc.zip", found: []string{"damaged"},
			data: zipOf(t, zipPart{raw: &zip.FileHeader{Name: "stored", Method: zip.Store, CRC32: 1, UncompressedSize64: 4, CompressedSize64: 4}, body: "abcd"})},
		{what: "entries that cannot be checked", file: "unchecked.zip", found: []string{"unchecked"},
			data: zipOf(t, zipPart{raw: &zip.FileHeader{Name: "locked", Method: zip.Store, Flags: 1, UncompressedSize64: 4, CompressedSize64: 4}, body: "abcd"},
				zipPart{raw: &zip.FileHeader{Name: "lzma", Method: 14, UncompressedSize64: 4, CompressedSize64: 4}, body: "abcd"}),
			names: map[string][]string{"unchecked": {"locked", "lzma"}}},
		{what: "an archive inside, reported and harmless", file: "nested.zip", found: []string{"nested"}, safe: true,
			data: zipOf(t, zipPart{name: "inner.zip", body: string(clean)})},
		{what: "a ZIP without the record at its end", file: "cut.zip", found: []string{"damaged"}, data: clean[:len(clean)-5]},
		{what: "a TAR.GZ with every kind of entry that is not a file", file: "bad.tar.gz", found: []string{"leaves_folder", "links", "special"},
			data: tarOf(t, true, tarPart{name: "../up.txt", body: "abc"}, tarPart{name: "lnk", typ: '2', link: "/etc/shadow"},
				tarPart{name: "hard", typ: '1', link: "../outside"}, tarPart{name: "fifo", typ: '6'}, tarPart{name: "dev", typ: '3'},
				tarPart{name: "suid", body: "x", mode: 0o4755}),
			names: map[string][]string{"links": {"lnk", "hard"}, "special": {"fifo", "dev", "suid"}}},
		{what: "a clean TAR", file: "clean.tar", data: tarOf(t, false, tarPart{name: "x.txt", body: "abc"}), safe: true},
		{what: "an empty TAR", file: "empty.tar", data: make([]byte, 1024), safe: true, kind: "tar"},
		{what: "a gzip of one file", file: "one.gz", data: gzipOf(t, []byte("hello\n")), safe: true, kind: "gzip"},
		{what: "a gzip of zeros built to explode", file: "bomb.gz", found: []string{"bomb", "stopped"}, data: zerosMembers(t, nil), kind: "gzip"},
		{what: "a TAR.GZ built to explode", file: "bomb.tar.gz", found: []string{"bomb", "stopped"}, data: tarGzBomb(t), kind: "tar.gz"},
		// Zeros after the end of a small TAR: no TAR program reads them, and
		// gunzip writes every one - only the count below the TAR sees them.
		{what: "a TAR.GZ exploding after the end of its TAR", file: "after.tar.gz", found: []string{"bomb", "stopped"}, kind: "tar.gz",
			data: zerosMembers(t, gzipOf(t, tarOf(t, false, tarPart{name: "a", body: "abc"})))},
		{what: "a TAR.GZ cut short", file: "cut.tar.gz", found: []string{"damaged"}, data: tarOf(t, true, tarPart{name: "a", body: strings.Repeat("z", 5000)})[:40]},
		{what: "more than the limit given", file: "limit.zip", data: clean, values: map[string]string{archivescan.SettingLimit: "100"},
			found: []string{"over_limit"}},
		{what: "within the limit given", file: "within.zip", data: clean, values: map[string]string{archivescan.SettingLimit: "1kb"}, safe: true},
	}
}

// TestTheArchiveScanFindsEveryTrapItNames scans an archive for each kind of
// trap and holds the report to exactly what is in it, and the conclusion to
// whether any of that makes it unsafe.
func TestTheArchiveScanFindsEveryTrapItNames(t *testing.T) {
	for _, c := range traps(t) {
		report, r := scanArchive(t, archiveFile(t, c.file, c.data), c.values)
		found := make([]string, 0, len(report.Found))
		for k := range report.Found {
			found = append(found, k)
		}
		sort.Strings(found)
		want := append([]string{}, c.found...)
		sort.Strings(want)
		if !slices.Equal(found, want) {
			t.Errorf("%s (%s): found %v and holds %v", c.what, c.file, found, want)
		}
		for key, names := range c.names {
			for _, name := range names {
				if !slices.Contains(report.Found[key], name) {
					t.Errorf("%s (%s): %s names %q and not %q", c.what, c.file, key, report.Found[key], name)
				}
			}
		}
		if c.kind != "" && report.Kind != c.kind {
			t.Errorf("%s (%s): read as %s, and it is %s", c.what, c.file, report.Kind, c.kind)
		}
		if report.Safe != c.safe || (r.Concluded == "safe") != c.safe {
			t.Errorf("%s (%s): safe is %v in the report and the conclusion is %q, and it is %v", c.what, c.file, report.Safe, r.Concluded, c.safe)
		}
	}
}

// TestTheArchiveScanReadsTheListArchiveZipReads holds the scanner's own
// reader of a ZIP to archive/zip, a reader nobody here wrote: as many entries,
// as many bytes once unpacked - with ZIP64, a comment after the end record,
// entries whose sizes follow their contents, and names outside ASCII.
func TestTheArchiveScanReadsTheListArchiveZipReads(t *testing.T) {
	var many []zipPart
	for i := range 70000 {
		many = append(many, zipPart{name: fmt.Sprintf("d/%06d", i)})
	}
	var commented bytes.Buffer
	w := zip.NewWriter(&commented)
	for _, name := range []string{"zolty-" + string(rune(0x17c)) + ".txt", "dir/", "dir/b.bin"} {
		out, err := w.Create(name)
		if err == nil && !strings.HasSuffix(name, "/") {
			_, err = out.Write(bytes.Repeat([]byte(name), 300))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.SetComment(strings.Repeat("PK comment ", 100)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"many.zip": zipOf(t, many...), "commented.zip": commented.Bytes()} {
		r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		var unpacked uint64
		for _, f := range r.File {
			unpacked += f.UncompressedSize64
		}
		report, _ := scanArchive(t, archiveFile(t, name, data), nil)
		if report.Entries != len(r.File) || report.Unpacked != unpacked || !report.Safe {
			t.Errorf("%s: the scan read %d entries and %d bytes unpacked, safe %v, and archive/zip reads %d and %d",
				name, report.Entries, report.Unpacked, report.Safe, len(r.File), unpacked)
		}
	}
}

// TestTheArchiveScanEndsWithTheCodeOfWhatHappened holds tfg tool archive-scan
// to the frozen table: safe is OK, a trap is VERIFY with the whole report on
// the error channel, a mistake in the request USAGE, a file that is not there
// IO, a stop INTERRUPTED.
func TestTheArchiveScanEndsWithTheCodeOfWhatHappened(t *testing.T) {
	clean := archiveFile(t, "clean.zip", zipOf(t, zipPart{name: "a.txt", body: "a"}))
	leaves := archiveFile(t, "leaves.zip", zipOf(t, zipPart{name: "../a.txt"}))
	sevenZip := archiveFile(t, "x.7z", append([]byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C}, make([]byte, 32)...))
	text := archiveFile(t, "notes.txt", []byte("not an archive"))
	stopped, stop := context.WithCancel(context.Background())
	stop()
	for _, c := range []struct {
		name string
		ctx  context.Context
		args []string
		want int
	}{
		{"a clean archive", context.Background(), []string{clean}, cli.ExitOK},
		{"an archive with a trap", context.Background(), []string{leaves}, cli.ExitVerify},
		{"more than the limit", context.Background(), []string{clean, "--unpacked_limit", "0"}, cli.ExitVerify},
		{"a limit that is not a size", context.Background(), []string{clean, "--unpacked_limit", "10xb"}, cli.ExitUsage},
		{"a file that is not an archive", context.Background(), []string{text}, cli.ExitUsage},
		{"an archive this tool does not read", context.Background(), []string{sevenZip}, cli.ExitUsage},
		{"a directory", context.Background(), []string{filepath.Dir(clean)}, cli.ExitUsage},
		{"no file", context.Background(), nil, cli.ExitUsage},
		{"two files", context.Background(), []string{clean, clean}, cli.ExitUsage},
		{"a file that is not there", context.Background(), []string{clean + ".missing"}, cli.ExitIO},
		{"a stop", stopped, []string{clean}, cli.ExitInterrupted},
	} {
		var out, errOut bytes.Buffer
		code := cli.Run(c.ctx, append([]string{"tool", archivescan.ID}, c.args...), &out, &errOut)
		if code != c.want {
			t.Errorf("%s: ended with %d and the table says %d. It said: %s%s", c.name, code, c.want, out.String(), errOut.String())
		}
		if code != cli.ExitOK && out.Len() > 0 {
			t.Errorf("%s: failed and still wrote to standard output: %s", c.name, out.String())
		}
	}
}

// TestTheToolsTabLooksIntoAnArchive runs the scanner from the screen, with a
// limit typed into its box, and holds what the screen says to what the
// command line says: the finding, the entry it names and the conclusion.
func TestTheToolsTabLooksIntoAnArchive(t *testing.T) {
	path := archiveFile(t, "leaves.zip", zipOf(t, zipPart{name: "../escape.txt", body: "a"}))
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"tool", archivescan.ID, path, "--unpacked_limit", "0"}, &out, &errOut); code != cli.ExitVerify {
		t.Fatalf("the command line ended with %d: %s", code, errOut.String())
	}
	d, err := tool.Get(archivescan.ID)
	if err != nil {
		t.Fatal(err)
	}
	var said []string
	for _, c := range d.Conclusions {
		if c.Fails {
			said = append(said, c.Says)
		}
	}
	said = append(said, d.NoteSays("leaves_folder"), d.NoteSays("over_limit"), "../escape.txt")
	host := newFakeHost(t)
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	chooseTool(t, screen, archivescan.ID)
	fillField(t, screen, text.SettingLabel(archivescan.InputArchive), path)
	fillField(t, screen, text.SettingLabel(archivescan.SettingLimit), "0")
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	words := allText(screen)
	for _, s := range said {
		if s == "" || !strings.Contains(errOut.String(), s) || !strings.Contains(words, s) {
			t.Errorf("%q is not on both: the command line said\n%s\nand the screen\n%s", s, errOut.String(), words)
		}
	}
}

// FuzzArchiveScan hands the scanner whatever bytes the fuzzer makes, as an
// archive somebody was sent. It must answer - refuse or conclude - without
// failing in itself, and a conclusion has to agree with the report.
func FuzzArchiveScan(f *testing.F) {
	t := &testing.T{}
	f.Add(zipOf(t, zipPart{name: "a.txt", body: "abc"}, zipPart{name: "../b"}))
	f.Add(tarOf(t, true, tarPart{name: "a", body: "abc"}, tarPart{name: "l", typ: '2', link: "/x"}))
	f.Add(tarOf(t, false, tarPart{name: "a", body: "abc"}))
	f.Add(handZip("first", "first", true))
	d, err := tool.Get(archivescan.ID)
	if err != nil {
		f.Fatal(err)
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		path := filepath.Join(dir, "fuzzed")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		r, err := d.Start(context.Background(), tool.Request{Inputs: map[string]string{archivescan.InputArchive: path}}, nil)
		if err != nil {
			return
		}
		report, ok := r.Data.(*archivescan.Report)
		if !ok || report.Safe != (r.Concluded == "safe") || r.Concluded == "" {
			t.Fatalf("a scan concluded %q with a report saying safe is %v", r.Concluded, ok && report.Safe)
		}
	})
}
