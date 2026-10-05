package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// The Tools tab and tfg tool - docs/NARZEDZIA-SUMY-2026-09-29.md.

// knownChecksums are published answers: RFC 1321 for md5, FIPS 180-2 appendix
// examples for the SHA family ("abc"), and the check value every CRC-32/IEEE
// catalogue gives for "123456789". Written down rather than worked out, so a
// wrong table in the tool and a wrong table here cannot agree by sharing code.
var knownChecksums = []struct {
	content, algorithm, sum string
}{
	{"abc", "md5", "900150983cd24fb0d6963f7d28e17f72"},
	{"abc", "sha1", "a9993e364706816aba3e25717850c26c9cd0d89d"},
	{"abc", "sha256", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	{"abc", "sha512", "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
		"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	{"123456789", "crc32", "cbf43926"},
	{"", "sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	{"", "md5", "d41d8cd98f00b204e9800998ecf8427e"},
}

// publishedAnswer is the checksum knownChecksums gives for a content, so a
// guard that needs one reads it from the table rather than typing it again.
func publishedAnswer(t *testing.T, content, algorithm string) string {
	t.Helper()
	for _, k := range knownChecksums {
		if k.content == content && k.algorithm == algorithm {
			return k.sum
		}
	}
	t.Fatalf("knownChecksums has no %s of %q", algorithm, content)
	return ""
}

// checksumOf runs the checksum tool the way both surfaces do, through Start.
func checksumOf(t *testing.T, path string, values map[string]string) (tool.Result, error) {
	t.Helper()
	d, err := tool.Get(checksum.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d.Start(context.Background(), tool.Request{
		Inputs: map[string]string{checksum.InputFile: path}, Values: values,
	}, nil)
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestTheChecksumsAreTheKnownAnswers holds every algorithm to a published
// answer - a wrong algorithm behind a right name, or a checksum written out in
// the wrong byte order, turns this red.
func TestTheChecksumsAreTheKnownAnswers(t *testing.T) {
	for _, k := range knownChecksums {
		path := writeTemp(t, "known.bin", k.content)
		r, err := checksumOf(t, path, map[string]string{checksum.SettingAlgorithm: k.algorithm})
		if err != nil {
			t.Fatalf("%s of %q: %v", k.algorithm, k.content, err)
		}
		got := r.Data.(*checksum.Checksums).Checksums[k.algorithm]
		if got != k.sum {
			t.Errorf("%s of %q is %s and the published answer is %s", k.algorithm, k.content, got, k.sum)
		}
	}
	// Every algorithm the tool offers has an answer above, so one added
	// tomorrow is not offered unchecked.
	for _, name := range checksum.Names() {
		found := false
		for _, k := range knownChecksums {
			found = found || k.algorithm == name
		}
		if !found {
			t.Errorf("the tool offers %s and no published answer holds it", name)
		}
	}
}

// TestTheChecksumsAgreeWithTheSystemsOwnTools asks md5sum, sha1sum, sha256sum
// and sha512sum about the same file - the programs people check a download
// with, so a checksum from here that they would not reproduce is a defect
// whatever the published vectors say.
//
// A tool missing from this machine is said, not skipped in silence. On a Linux
// runner of the CI all four are part of the system, so there a missing one is
// a failure: a guard that checked nothing there would look like one that
// checked everything.
func TestTheChecksumsAgreeWithTheSystemsOwnTools(t *testing.T) {
	content := strings.Repeat("the quick brown fox jumps over the lazy dog\n", 5000)
	path := writeTemp(t, "sample.txt", content)
	r, err := checksumOf(t, path, map[string]string{checksum.SettingAlgorithm: "all"})
	if err != nil {
		t.Fatal(err)
	}
	ours := r.Data.(*checksum.Checksums).Checksums

	checked := 0
	for _, name := range []string{"md5", "sha1", "sha256", "sha512"} {
		program, err := exec.LookPath(name + "sum")
		if err != nil {
			t.Logf("SKIPPED: %ssum is not installed here", name)
			continue
		}
		out, err := exec.Command(program, path).Output()
		if err != nil {
			t.Fatalf("%ssum: %v", name, err)
		}
		theirs, _, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
		theirs = strings.TrimPrefix(theirs, `\`)
		if theirs != ours[name] {
			t.Errorf("%s: this tool says %s and %ssum says %s", name, ours[name], name, theirs)
		}
		checked++
	}
	if checked == 0 && runtime.GOOS == "linux" && os.Getenv("CI") != "" {
		t.Fatal("no md5sum, sha1sum, sha256sum or sha512sum on a Linux runner, so nothing was compared")
	}
	t.Logf("%d algorithm(s) compared with the system's own program", checked)
}

// TestTheWindowSaysAVerdictAsTheCommandLineDoes holds the English of the two
// wordings of one verdict to one sentence. The window words it from its
// catalogue so it can say it in another language, and the command line from
// tool.Verdict.Said - two compositions, which drift unless compared (D1).
func TestTheWindowSaysAVerdictAsTheCommandLineDoes(t *testing.T) {
	match := tool.Verdict{Outcome: tool.Match, About: "sha256", Got: "abc123", Wanted: "abc123"}
	if got, want := text.ToolMatches(match.About, match.Got), match.Said(); got != want {
		t.Errorf("a match reads %q in the window and %q on the command line", got, want)
	}
	miss := tool.Verdict{Outcome: tool.Mismatch, About: "md5", Got: "aa", Wanted: "bb"}
	if got, want := text.ToolDoesNotMatch(miss.About, miss.Got, miss.Wanted), miss.Said(); got != want {
		t.Errorf("a mismatch reads %q in the window and %q on the command line", got, want)
	}
	if said := (tool.Verdict{Outcome: tool.Unasked}).Said(); said != "" {
		t.Errorf("nothing was compared and the command line says %q", said)
	}
	// A list of files compared with a checksum file, which is said about the
	// list rather than about one value (checksum-check).
	listMatch := tool.Verdict{Outcome: tool.Match, About: "SHA256SUMS", Listed: true}
	if got, want := text.ToolListMatches(listMatch.About), listMatch.Said(); got != want {
		t.Errorf("a checksum file that holds reads %q in the window and %q on the command line", got, want)
	}
	listMiss := tool.Verdict{Outcome: tool.Mismatch, About: "SHA256SUMS", Listed: true}
	if got, want := text.ToolListDoesNotMatch(listMiss.About), listMiss.Said(); got != want {
		t.Errorf("a checksum file that does not hold reads %q in the window and %q on the command line", got, want)
	}
}

// TestTheToolsScreenOffersEveryToolWithEveryBox walks the registry, not a list:
// every tool is in the menu, and choosing it draws a box named for everything
// it works on and every setting it takes. A tool registered tomorrow is held
// on the day it arrives.
func TestTheToolsScreenOffersEveryToolWithEveryBox(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	menu := chooserUnder(t, screen, text.FieldTool())
	all := tool.All()
	if len(all) == 0 {
		t.Fatal("the registry holds no tool, so this guard would pass against an empty screen")
	}
	for _, d := range all {
		question := text.ToolQuestion(d.ID, d.Question)
		menu.SetSelected(question)
		if menu.Selected != question {
			t.Errorf("the menu does not offer %s (%q)", d.ID, question)
			continue
		}
		words := allText(screen)
		for _, in := range d.Inputs {
			if !strings.Contains(words, text.SettingLabel(in.Name)) {
				t.Errorf("%s works on a %s and the screen has no box named %q", d.ID, in.Name, text.SettingLabel(in.Name))
			}
		}
		for _, p := range d.Settings {
			if !strings.Contains(words, text.SettingLabel(p.Name)) {
				t.Errorf("%s takes %s and the screen has no box named %q", d.ID, p.Name, text.SettingLabel(p.Name))
			}
		}
	}
}

// TestTheWindowRunsAToolAndSaysWhatTheCommandLineSays runs the checksum tool
// from the screen and from tfg tool on one file, and compares the answers - the
// run from the window through the boxes a person fills and the button a person
// presses.
func TestTheWindowRunsAToolAndSaysWhatTheCommandLineSays(t *testing.T) {
	path := writeTemp(t, "both.bin", strings.Repeat("x", 100000))
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"tool", checksum.ID, path, "--json"}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("tfg tool checksum ended with %d: %s", code, errOut.String())
	}
	var said checksum.Checksums
	if err := json.Unmarshal(out.Bytes(), &said); err != nil {
		t.Fatal(err)
	}
	want := said.Checksums["sha256"]

	host := newFakeHost(t)
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())
	fillField(t, screen, text.SettingLabel(checksum.InputFile), path)
	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	if words := allText(screen); !strings.Contains(words, want) {
		t.Errorf("the command line says the sha256 is %s and the screen does not show it. It shows:\n%s", want, words)
	}
}

// TestToolsEndWithTheCodeOfWhatHappened holds the exit codes of tfg tool to
// the frozen table: a mismatch is VERIFY, a mistake in the request USAGE, a
// path that cannot be read IO.
func TestToolsEndWithTheCodeOfWhatHappened(t *testing.T) {
	file := writeTemp(t, "abc.txt", "abc")
	sha := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	wrong := strings.Repeat("0", 64)
	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"the checksum given", []string{file, "--expected", sha}, cli.ExitOK},
		{"the checksum given in capitals", []string{file, "--expected", strings.ToUpper(sha)}, cli.ExitOK},
		{"a different checksum", []string{file, "--expected", wrong}, cli.ExitVerify},
		{"an algorithm spelled otherwise", []string{file, "--algorithm", "SHA256"}, cli.ExitUsage},
		{"a checksum no algorithm has", []string{file, "--expected", "abcd"}, cli.ExitUsage},
		{"no file", nil, cli.ExitUsage},
		{"a directory", []string{filepath.Dir(file)}, cli.ExitUsage},
		{"a file that is not there", []string{file + ".missing"}, cli.ExitIO},
		{"two files", []string{file, file}, cli.ExitUsage},
	} {
		var out, errOut bytes.Buffer
		code := cli.Run(context.Background(), append([]string{"tool", checksum.ID}, c.args...), &out, &errOut)
		if code != c.want {
			t.Errorf("%s: ended with %d and the table says %d. It said: %s%s", c.name, code, c.want, out.String(), errOut.String())
		}
		if code != cli.ExitOK && out.Len() > 0 {
			t.Errorf("%s: failed and still wrote to standard output: %s", c.name, out.String())
		}
	}
}

// TestALoneDashIsAFileNameAndNotAHang holds "tfg tool checksum -" to what
// "tfg verify -" does: a path, read like any other, and an answer. Until
// 2026-09-30 the flags were read again and again around a "-" nothing ever
// took, and the command never returned (a review of #157, measured before the
// fix: killed by timeout, code 124). The failure being guarded is a hang, so
// every run has a deadline and says so.
func TestALoneDashIsAFileNameAndNotAHang(t *testing.T) {
	t.Chdir(t.TempDir())
	runs := func(args ...string) (int, string) {
		t.Helper()
		type ending struct {
			code int
			out  string
		}
		ended := make(chan ending, 1)
		go func() {
			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), append([]string{"tool", checksum.ID}, args...), &out, &errOut)
			ended <- ending{code, out.String()}
		}()
		select {
		case e := <-ended:
			return e.code, e.out
		case <-time.After(10 * time.Second):
			t.Fatalf("tfg tool checksum %s has not returned after ten seconds", strings.Join(args, " "))
			return 0, ""
		}
	}

	if code, _ := runs("-"); code != cli.ExitIO {
		t.Errorf("nothing is called - here and the tool ended with %d, not %d as for any path that is not there",
			code, cli.ExitIO)
	}
	if err := os.WriteFile("-", []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := publishedAnswer(t, "abc", "md5")
	for _, args := range [][]string{{"-", "--algorithm", "md5"}, {"--algorithm", "md5", "-"}} {
		code, out := runs(args...)
		if code != cli.ExitOK || !strings.Contains(out, want) {
			t.Errorf("a file is called - and tfg tool checksum %s ended with %d, saying:\n%s",
				strings.Join(args, " "), code, out)
		}
	}
}

// TestTheToolsScreenTakesAChosenFileAndCopiesTheChecksum asks the two things
// only the Tools tab asks of a window: the file picker, whose answer has to
// land in the box, and the clipboard, which has to get the checksum the row
// shows. The stand in window recorded both from the day they arrived and
// nothing read either until 2026-09-30.
func TestTheToolsScreenTakesAChosenFileAndCopiesTheChecksum(t *testing.T) {
	path := writeTemp(t, "picked.txt", "abc")
	host := newFakeHost(t)
	host.pickedFile = path
	window.Open(host)
	screen := selectTab(t, host.content, text.TabTools())

	pressNamed(t, screen, text.ButtonChoose())
	if host.askedFile == 0 {
		t.Fatal("the browse button of the Tools tab asked nobody for a file")
	}
	if got := entryUnder(t, screen, text.SettingLabel(checksum.InputFile)).Text; got != path {
		t.Fatalf("%s was chosen and the box holds %q", path, got)
	}

	pressNamed(t, screen, text.ButtonRunTool())
	host.waitForWork()
	pressNamed(t, screen, text.ButtonCopy())
	if want := publishedAnswer(t, "abc", "sha256"); host.copied != want {
		t.Errorf("Copy put %q on the clipboard and the sha256 of the file is %s", host.copied, want)
	}
}
