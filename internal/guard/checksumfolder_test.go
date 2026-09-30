package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// The two folder tools of the Tools tab - checksum-write and checksum-check,
// docs/NARZEDZIA-SUMY-2026-09-29.md §15. The format they write and read is
// somebody else's, so the guards that matter most ask somebody else: the
// sha256sum people check a folder with.

// folderOf writes files into a fresh folder, a slash in a name making the
// folders on the way.
func folderOf(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runTool runs a tool of tfg tool from its command line, the way a person or
// a script does, and hands back what it said.
func runTool(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = cli.Run(context.Background(), append([]string{"tool"}, args...), &o, &e)
	return code, o.String(), e.String()
}

// checkedJSON runs checksum-check with --json and reads what it said, from
// whichever stream it said it on - a failed check writes nothing to standard
// output.
func checkedJSON(t *testing.T, sums string) (int, checksum.Checked) {
	t.Helper()
	code, out, errOut := runTool(t, checksum.CheckID, sums, "--json")
	said := out
	if code != cli.ExitOK {
		said = errOut
	}
	var got checksum.Checked
	if err := json.Unmarshal([]byte(said), &got); err != nil {
		t.Fatalf("checksum-check ended with %d and did not print JSON: %v\n%s%s", code, err, out, errOut)
	}
	return code, got
}

// coreutilsMajor is the major version of the sha256sum on this machine, or
// nought when there is none. A carriage return in a name is escaped only from
// coreutils 9 (measured on 2026-09-30), so the guard below asks.
func coreutilsMajor(t *testing.T) (string, int) {
	t.Helper()
	program, err := exec.LookPath("sha256sum")
	if err != nil {
		return "", 0
	}
	out, err := exec.Command(program, "--version").Output()
	if err != nil {
		return "", 0
	}
	first, _, _ := strings.Cut(string(out), "\n")
	fields := strings.Fields(first)
	if len(fields) == 0 {
		return program, 0
	}
	major := 0
	for _, c := range fields[len(fields)-1] {
		if c < '0' || c > '9' {
			break
		}
		major = major*10 + int(c-'0')
	}
	return program, major
}

// TestTheChecksumFileIsTheOneSha256sumWrites writes a checksum file of a
// folder with every kind of name that is escaped, or that looks as if it
// might be, and holds it BYTE FOR BYTE to what sha256sum writes about the same
// files in the same order - then has sha256sum check it.
//
// Byte for byte rather than "sha256sum -c passes", because -c passes lines it
// cannot read over with a warning and still ends with success (measured, 8.32
// and 9.7), so a wrong escape could pass it. A name with a backslash or a line
// break exists only where the system allows one, which Windows does not. A
// carriage return is added where coreutils is 9 or later, the first to escape
// one.
func TestTheChecksumFileIsTheOneSha256sumWrites(t *testing.T) {
	files := map[string]string{
		"plain.txt": "abc", "with space.txt": "b", " leading space": "c",
		"zażółć.txt": "d", "sub/inner.txt": "e", "empty": "",
	}
	program, major := coreutilsMajor(t)
	if runtime.GOOS != "windows" {
		files["*star"] = "f"
		files[`back\slash`] = "g"
		files["new\nline"] = "h"
		if major >= 9 {
			files["carriage\rreturn"] = "i"
		}
	}
	dir := folderOf(t, files)
	if code, out, errOut := runTool(t, checksum.WriteID, dir); code != cli.ExitOK {
		t.Fatalf("checksum-write ended with %d: %s%s", code, out, errOut)
	}
	ours, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}

	if program == "" {
		if runtime.GOOS == "linux" && os.Getenv("CI") != "" {
			t.Fatal("no sha256sum on a Linux runner, so the checksum file was held to nothing")
		}
		t.Skip("SKIPPED: sha256sum is not installed here, so there is nothing to hold the file to")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	// -t, the text mode, which is what sha256sum writes on Linux by default and
	// what this tool writes everywhere. The one Git ships for Windows writes the
	// binary mode unless told, "hash *name" - measured on 2026-09-30. Both are
	// read by both, so the difference is the oracle's default, not a fault.
	write := exec.Command(program, append([]string{"-t", "--"}, names...)...)
	write.Dir = dir
	theirs, err := write.Output()
	if err != nil {
		t.Fatalf("sha256sum about the same files: %v", err)
	}
	if !bytes.Equal(ours, theirs) {
		t.Errorf("the checksum file is not what sha256sum %d writes about the same files in the same order.\nours:\n%q\ntheirs:\n%q", major, ours, theirs)
	}

	check := exec.Command(program, "-c", "--strict", "SHA256SUMS")
	check.Dir = dir
	if said, err := check.CombinedOutput(); err != nil {
		t.Errorf("sha256sum -c --strict does not pass the checksum file: %v\n%s", err, said)
	}
	t.Logf("%d names held to sha256sum %d", len(names), major)
}

// The lines a checksum file may hold, as the tools people use write them -
// taken from the measurement of 2026-09-30 rather than from memory. The
// checksums are of "abc", which every file of the folder below holds.
const (
	abcSHA256 = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	abcMD5    = "900150983cd24fb0d6963f7d28e17f72"
)

// TestTheCheckReadsTheLinesOtherToolsWrite hands checksum-check one file of
// every way a checksum line is written - GNU, the tagged form of --tag and
// shasum, a star, one space, capitals, a Windows line end, a byte order mark -
// among the lines a signed checksum file is full of, and holds each line to
// what it is: checked and matched, or named with its number and passed over.
func TestTheCheckReadsTheLinesOtherToolsWrite(t *testing.T) {
	dir := folderOf(t, map[string]string{"a.txt": "abc", "b.txt": "abc", "c.txt": "abc", "d.txt": "abc", "e.txt": "abc", "f.txt": "abc", "g.txt": "abc"})
	lines := []string{
		"\xef\xbb\xbf" + abcSHA256 + "  a.txt", // 1: a byte order mark, which sha256sum refuses
		"iQIzBAEBCAAdFiEE",                     // 2: a line of a signature's body
		"Hash: SHA256",                         // 3
		"",                                     // 4
		abcSHA256 + " *b.txt",                  // 5: the star of binary mode
		abcSHA256 + " c.txt",                   // 6: one space
		strings.ToUpper(abcSHA256) + "  d.txt", // 7: capitals
		abcSHA256 + "  e.txt\r",                // 8: a Windows line end
		"SHA256 (f.txt) = " + abcSHA256,        // 9: --tag and shasum --tag
		"MD5 (g.txt) = " + abcMD5,              // 10
		"# a comment",                          // 11
		abcSHA256[:63] + "  a.txt",             // 12: a digit lost
		strings.Repeat("a", 56) + "  a.txt",    // 13: sha224
		"SHA3-256 (a.txt) = " + abcSHA256,      // 14
		"=vW3x",                                // 15: the check line that ends a signature
	}
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, got := checkedJSON(t, sums)
	if code != cli.ExitOK {
		t.Errorf("every checksum line matches and the check ended with %d, %+v", code, got.Problems)
	}
	if got.Checked != 7 || got.Matched != 7 {
		t.Errorf("seven checksum lines were written and %d were checked, %d matched", got.Checked, got.Matched)
	}
	if want := []int{2, 3, 4, 11, 12, 15}; !equalInts(got.NotChecked.NotChecksums, want) {
		t.Errorf("the lines that are not checksums are %v and the check named %v", want, got.NotChecked.NotChecksums)
	}
	if want := []string{"line 13: sha224", "line 14: SHA3-256"}; strings.Join(got.NotChecked.Unknown, "|") != strings.Join(want, "|") {
		t.Errorf("the lines of other algorithms are %q and the check named %q", want, got.NotChecked.Unknown)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestTheCheckUndoesTheEscapingOfANameAndKeepsItInTheFolder reads the lines
// sha256sum writes for names with a backslash and a line break, and a path
// that climbs out, and holds each to where it points. The names cannot be
// files on Windows, so they are asked about while missing - which is the path
// the check reports either way, with the escaping undone.
func TestTheCheckUndoesTheEscapingOfANameAndKeepsItInTheFolder(t *testing.T) {
	dir := folderOf(t, map[string]string{"sub/abc.txt": "abc"})
	sums := filepath.Join(dir, "sub", "SHA256SUMS")
	body := `\` + abcSHA256 + `  back\\slash` + "\n" +
		`\` + abcSHA256 + `  new\nline` + "\n" +
		abcSHA256 + "  ../sub/abc.txt\n" +
		abcSHA256 + "  abc.txt\n" +
		abcSHA256 + "  abc.txt\n"
	if err := os.WriteFile(sums, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	code, got := checkedJSON(t, sums)
	if code != cli.ExitVerify {
		t.Errorf("two listed files are not there and one path climbs out, and the check ended with %d", code)
	}
	want := map[string]string{`back\slash`: "missing", "new\nline": "missing", "../sub/abc.txt": "outside"}
	if runtime.GOOS == "windows" {
		// Windows refuses a name with a line break in it as a name, which is
		// not the same answer as nothing being there, and the check says the
		// system's reason rather than guessing which it meant.
		want["new\nline"] = "unreadable"
	}
	for _, p := range got.Problems {
		if want[p.Path] != p.Kind {
			t.Errorf("%q came back %s, and it is %q", p.Path, p.Kind, want[p.Path])
		}
		delete(want, p.Path)
	}
	for path, kind := range want {
		t.Errorf("%q is %s and the check did not say so", path, kind)
	}
	if got.Matched != 2 || strings.Join(got.NotChecked.Twice, "|") != "abc.txt" {
		t.Errorf("abc.txt is listed twice, matched twice and named once, and the check said matched %d, twice %q", got.Matched, got.NotChecked.Twice)
	}
}

// FuzzChecksumFile hands the parser of checksum files whatever the fuzzer
// makes, and asks two things of every answer. Every line is filed under
// exactly one of checked, not a checksum and another algorithm - nothing lost,
// nothing twice. And a name written by SumsLine reads back as the same name,
// whatever it holds: an escape that loses a byte is a checksum file that
// checks a file nobody listed.
func FuzzChecksumFile(f *testing.F) {
	for _, seed := range []string{
		abcSHA256 + "  a.txt\n", `\` + abcSHA256 + `  back\\slash` + "\n", "SHA256 (x) = " + abcSHA256 + "\n",
		"\xef\xbb\xbf" + abcSHA256 + "  a\r\n", "", "\n\n", abcSHA256 + "  ", `\` + abcSHA256 + `  a\q`,
		"MD5 (a) = b) = " + abcMD5, strings.Repeat("x", 300000) + "\n" + abcSHA256 + "  z",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		parsed, err := checksum.ParseSums(strings.NewReader(input))
		if err != nil {
			t.Fatalf("reading from memory failed: %v", err)
		}
		filed := map[int]int{}
		for _, l := range parsed.Listed {
			filed[l.Line]++
		}
		for _, n := range parsed.NotSums {
			filed[n]++
		}
		lines := strings.Count(input, "\n")
		if input != "" && !strings.HasSuffix(input, "\n") {
			lines++
		}
		if len(parsed.Listed)+len(parsed.NotSums)+len(parsed.Unknown) != lines {
			t.Fatalf("%d lines came back as %d checked, %d not checksums and %d unknown", lines, len(parsed.Listed), len(parsed.NotSums), len(parsed.Unknown))
		}
		for n, times := range filed {
			if times != 1 || n < 1 || n > lines {
				t.Fatalf("line %d was filed %d times among %d lines", n, times, lines)
			}
		}

		// A name longer than a line may be is read past on purpose - no system
		// has one - so it cannot come back, and asking it to would be wrong.
		if input == "" || strings.ContainsRune(input, 0) || len(input) > 100000 {
			return
		}
		back, err := checksum.ParseSums(strings.NewReader(checksum.SumsLine(abcSHA256, input)))
		if err != nil || len(back.Listed) != 1 || back.Listed[0].Path != input || back.Listed[0].Sum != abcSHA256 {
			t.Fatalf("the name %q was written as %q and read back as %+v (%v)", input, checksum.SumsLine(abcSHA256, input), back, err)
		}
	})
}

// TestTheFolderToolsEndWithTheCodeOfWhatHappened holds checksum-write and
// checksum-check to the frozen table of exit codes, and to the rule that a
// failure writes nothing to standard output.
func TestTheFolderToolsEndWithTheCodeOfWhatHappened(t *testing.T) {
	good := folderOf(t, map[string]string{"a.txt": "abc"})
	if code, out, errOut := runTool(t, checksum.WriteID, good); code != cli.ExitOK {
		t.Fatalf("writing the checksum file of a folder of one file ended with %d: %s%s", code, out, errOut)
	}
	sums := filepath.Join(good, "SHA256SUMS")
	changed := folderOf(t, map[string]string{"a.txt": "abc"})
	if err := os.WriteFile(filepath.Join(changed, "SHA256SUMS"), []byte(strings.Repeat("0", 32)+"  a.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	notSums := writeTemp(t, "notes.txt", "hello\n")
	empty := t.TempDir()

	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"a folder written again over its checksum file", []string{checksum.WriteID, good}, cli.ExitIO},
		{"a file where a folder goes", []string{checksum.WriteID, sums}, cli.ExitUsage},
		{"a folder that is not there", []string{checksum.WriteID, good + "-gone"}, cli.ExitIO},
		{"an empty folder", []string{checksum.WriteID, empty}, cli.ExitUsage},
		{"an algorithm with no checksum file", []string{checksum.WriteID, empty, "--algorithm", "crc32"}, cli.ExitUsage},
		{"no folder", []string{checksum.WriteID}, cli.ExitUsage},
		{"a checksum file that holds", []string{checksum.CheckID, sums}, cli.ExitOK},
		{"a checksum file that does not", []string{checksum.CheckID, filepath.Join(changed, "SHA256SUMS")}, cli.ExitVerify},
		{"a file with no checksum line", []string{checksum.CheckID, notSums}, cli.ExitIO},
		{"a folder where the checksum file goes", []string{checksum.CheckID, good}, cli.ExitUsage},
		{"a checksum file that is not there", []string{checksum.CheckID, sums + "-gone"}, cli.ExitIO},
	} {
		code, out, errOut := runTool(t, c.args...)
		if code != c.want {
			t.Errorf("%s: ended with %d and the table says %d. It said: %s%s", c.name, code, c.want, out, errOut)
		}
		if code != cli.ExitOK && out != "" {
			t.Errorf("%s: failed and still wrote to standard output: %s", c.name, out)
		}
	}
}

// TestAChecksumFileIsNeverWrittenOver holds untouchable rule 7 for the one
// tool that writes: a checksum file already there stays as it is, byte for
// byte, and so does a half written one another run left - which is named in
// the answer rather than listed as a file of the folder.
func TestAChecksumFileIsNeverWrittenOver(t *testing.T) {
	dir := folderOf(t, map[string]string{"a.txt": "abc", "SHA256SUMS": "somebody's own\n"})
	if code, _, _ := runTool(t, checksum.WriteID, dir); code != cli.ExitIO {
		t.Errorf("a checksum file was there and writing another ended with %d, not %d", code, cli.ExitIO)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS")); string(got) != "somebody's own\n" {
		t.Errorf("the checksum file that was there now reads %q", got)
	}

	left := folderOf(t, map[string]string{"a.txt": "abc", "b.bin.tfg-writing": "half"})
	code, out, errOut := runTool(t, checksum.WriteID, left, "--json")
	if code != cli.ExitOK {
		t.Fatalf("a folder with a half written file ended with %d: %s%s", code, out, errOut)
	}
	var written checksum.Written
	if err := json.Unmarshal([]byte(out), &written); err != nil {
		t.Fatal(err)
	}
	if written.Files != 1 || strings.Join(written.LeftOut.Unfinished, "|") != "b.bin.tfg-writing" {
		t.Errorf("one file and one half written one, and the tool listed %d and left out %q", written.Files, written.LeftOut.Unfinished)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "b.bin.tfg-writing")); string(got) != "half" {
		t.Errorf("the half written file now reads %q", got)
	}
}

// TestAStoppedWriteLeavesNothing stops checksum-write in the middle of reading
// and asks the folder: no checksum file, and no half of one under the name it
// is written under first (G7).
func TestAStoppedWriteLeavesNothing(t *testing.T) {
	dir := folderOf(t, map[string]string{"big.bin": strings.Repeat("x", 8<<20), "small.txt": "abc"})
	d, err := tool.Get(checksum.WriteID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := false
	_, err = d.Start(ctx, tool.Request{Inputs: map[string]string{checksum.InputFolder: dir}}, func(done, _ int64) {
		if done > 0 && !stopped {
			stopped = true
			cancel()
		}
	})
	if !stopped {
		t.Fatal("the tool never said it had read anything, so this guard stopped nothing")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("stopped in the middle and the tool answered %v rather than that it was stopped", err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "big.bin" && e.Name() != "small.txt" {
			t.Errorf("a stopped run left %s in the folder", e.Name())
		}
	}
}

// TestTheToolsTabWritesAndChecksAFolder runs both folder tools from the
// screen, through the pickers a person uses: the folder picker for the folder
// and the file picker for the checksum file, each asked, each answer landing
// in its box. Then the screen has to say what the command line says.
func TestTheToolsTabWritesAndChecksAFolder(t *testing.T) {
	dir := folderOf(t, map[string]string{"a.txt": "abc", "sub/b.txt": "b"})
	host := newFakeHost(t)
	host.picked = dir
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	menu := chooserUnder(t, screen, text.FieldTool())

	write, _ := tool.Get(checksum.WriteID)
	menu.SetSelected(text.ToolQuestion(write.ID, write.Question))
	pressNamed(t, screen, text.ButtonChoose())
	if host.asked == 0 {
		t.Fatal("the browse button of checksum-write asked nobody for a folder")
	}
	if got := entryUnder(t, screen, text.SettingLabel(checksum.InputFolder)).Text; got != dir {
		t.Fatalf("%s was chosen and the box holds %q", dir, got)
	}
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	sums := filepath.Join(dir, "SHA256SUMS")
	if _, err := os.Stat(sums); err != nil {
		t.Fatalf("Run on checksum-write wrote no checksum file: %v\nThe screen says:\n%s", err, allText(screen))
	}
	if words := allText(screen); !strings.Contains(words, text.ToolNote(write.ID, "written", write.NoteSays("written"))) {
		t.Errorf("the screen does not say the file was written. It says:\n%s", words)
	}

	check, _ := tool.Get(checksum.CheckID)
	menu.SetSelected(text.ToolQuestion(check.ID, check.Question))
	host.pickedFile = sums
	pressNamed(t, screen, text.ButtonChoose())
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	if words := allText(screen); !strings.Contains(words, text.ToolListMatches("SHA256SUMS")) {
		t.Errorf("the folder is what its checksum file says and the screen does not say so. It says:\n%s", words)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	if words := allText(screen); !strings.Contains(words, text.ToolListDoesNotMatch("SHA256SUMS")) || !strings.Contains(words, "a.txt") {
		t.Errorf("a.txt changed and the screen does not say which file does not match. It says:\n%s", words)
	}
}

// TestTheToolsTabCutsALongListShort checks a checksum file listing more
// missing files than the screen draws, and holds the screen to drawing the
// first NoteItemsShown and saying how many more there are - a folder that
// moved lists every one of its files as missing, and a hundred thousand
// lines drawn on a canvas is a window that stops answering.
func TestTheToolsTabCutsALongListShort(t *testing.T) {
	dir := t.TempDir()
	var body strings.Builder
	listed := 25
	for i := 1; i <= listed; i++ {
		body.WriteString(fmt.Sprintf("%s  missing-%02d.txt\n", abcSHA256, i))
	}
	sums := filepath.Join(dir, "SHA256SUMS")
	if err := os.WriteFile(sums, []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	host := newFakeHost(t)
	host.pickedFile = sums
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	check, _ := tool.Get(checksum.CheckID)
	chooserUnder(t, screen, text.FieldTool()).SetSelected(text.ToolQuestion(check.ID, check.Question))
	pressNamed(t, screen, text.ButtonChoose())
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()

	words := allText(screen)
	shown := parts.NoteItemsShown
	if !strings.Contains(words, fmt.Sprintf("missing-%02d.txt", shown)) {
		t.Fatalf("the screen does not show the first %d missing files at all, so this guard read a screen that is not the result. It says:\n%s", shown, words)
	}
	if strings.Contains(words, fmt.Sprintf("missing-%02d.txt", shown+1)) {
		t.Errorf("the screen draws more than %d items of one note", shown)
	}
	if want := text.ToolMoreItems(listed-shown, check.ID); !strings.Contains(words, want) {
		t.Errorf("the screen cut the list short without saying %q. It says:\n%s", want, words)
	}
}

// TestAFolderToolNeverWaitsOnAPipeOrAJunction puts a thing that is not a file
// in a folder - a pipe where the system has them, a junction on Windows - and
// holds both tools to naming it without opening it. The failure being guarded
// is a run that never returns, so every run has a deadline.
func TestAFolderToolNeverWaitsOnAPipeOrAJunction(t *testing.T) {
	dir := folderOf(t, map[string]string{"a.txt": "abc", "target/inside.txt": "x"})
	name := notAFileIn(t, dir)
	within := func(args ...string) (int, string, string) {
		t.Helper()
		type ending struct {
			code     int
			out, err string
		}
		ended := make(chan ending, 1)
		go func() {
			code, out, errOut := runTool(t, args...)
			ended <- ending{code, out, errOut}
		}()
		select {
		case e := <-ended:
			return e.code, e.out, e.err
		case <-time.After(20 * time.Second):
			t.Fatalf("tfg tool %s has not returned after twenty seconds", strings.Join(args, " "))
			return 0, "", ""
		}
	}

	code, out, errOut := within(checksum.WriteID, dir, "--json")
	if code != cli.ExitOK {
		t.Fatalf("checksum-write ended with %d: %s%s", code, out, errOut)
	}
	var written checksum.Written
	if err := json.Unmarshal([]byte(out), &written); err != nil {
		t.Fatal(err)
	}
	if strings.Join(written.LeftOut.NotFiles, "|") != name {
		t.Errorf("%s is not a file and the tool left out %q", name, written.LeftOut.NotFiles)
	}

	sums := filepath.Join(dir, "SHA256SUMS")
	list := abcSHA256 + "  " + name + "\n" + abcSHA256 + "  a.txt\n"
	if err := os.WriteFile(sums+"2", []byte(list), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = within(checksum.CheckID, sums+"2", "--json")
	var checked checksum.Checked
	if err := json.Unmarshal([]byte(errOut), &checked); err != nil {
		t.Fatalf("checksum-check ended with %d and printed no JSON: %v", code, err)
	}
	// A pipe is opened without waiting and found not to be a file. A junction
	// is not opened at all: the boundary cannot follow one to see where it
	// leads, so it is refused as a way out of the folder (O265).
	wantKind := "not_a_file"
	if runtime.GOOS == "windows" {
		wantKind = "outside"
	}
	if code != cli.ExitVerify || len(checked.Problems) != 1 || checked.Problems[0].Kind != wantKind {
		t.Errorf("a checksum file lists %s and the check ended with %d, saying %+v", name, code, checked.Problems)
	}
}

// TestAFolderThatCannotBeListedIsRefusedWholeAndVerifyStillStopsAtIt holds the
// walk verify and checksum-write share to two answers about a directory that
// cannot be listed. checksum-write refuses the whole checksum file and names
// EVERY such directory, since a checksum file promises every file. Verify,
// which the walk learned to go past one for on 2026-09-30, still refuses on
// the first one with the code it gave before - reporting a clean run over a
// folder it could not see into would be the one answer it must never give.
//
// Windows has no mode that stops the owner of a folder listing it, and
// neither does root anywhere, so this runs where it can say something.
func TestAFolderThatCannotBeListedIsRefusedWholeAndVerifyStillStopsAtIt(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("SKIPPED: no way to make a folder unreadable to its own user here")
	}
	lock := func(dirs ...string) {
		for _, d := range dirs {
			if err := os.Chmod(d, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
		}
	}

	dir := folderOf(t, map[string]string{"a.txt": "abc", "first/x": "x", "second/y": "y"})
	lock(filepath.Join(dir, "first"), filepath.Join(dir, "second"))
	code, out, errOut := runTool(t, checksum.WriteID, dir)
	if code != cli.ExitIO || !strings.Contains(errOut, "first") || !strings.Contains(errOut, "second") {
		t.Errorf("two folders could not be listed and checksum-write ended with %d, saying:\n%s%s", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "SHA256SUMS")); err == nil {
		t.Error("checksum-write refused and still wrote a checksum file")
	}

	generatedDir, manifest := generated(t)
	locked := filepath.Join(generatedDir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	lock(locked)
	code, out, errOut = runVerify(manifest)
	if code != cli.ExitIO || !strings.Contains(errOut, "locked") {
		t.Errorf("verify could not list a folder of the run and ended with %d, saying:\n%s%s", code, out, errOut)
	}
}

// runVerify is tfg verify of one manifest.
func runVerify(manifest string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"verify", manifest}, &out, &errOut)
	return code, out.String(), errOut.String()
}

// notAFileIn makes one thing in dir that is not a file, and says its name: a
// pipe on Linux and macOS, a junction on Windows - which an ordinary user can
// make, and which the walk reports as neither a link nor a folder.
func notAFileIn(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		if err := exec.Command("mkfifo", filepath.Join(dir, "pipe")).Run(); err != nil {
			t.Fatalf("mkfifo: %v", err)
		}
		return "pipe"
	}
	out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(dir, "junction"), filepath.Join(dir, "target")).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, out)
	}
	return "junction"
}
