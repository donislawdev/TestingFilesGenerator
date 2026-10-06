package guard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The inputs that would render, look fine, and be wrong - and the renderer
// has to refuse each one out loud rather than hand a feed a package that
// installs the wrong thing. Every refusal leaves nothing behind: the packages
// are written into a directory beside the destination and renamed onto it
// only when every one of them rendered, so a refusal or a Ctrl+C half way
// leaves no half of a package to be submitted by mistake.

func TestTheRendererRefusesInputsThatLookFineAndAreWrong(t *testing.T) {
	withoutArm := []byte(strings.Join(append(packagingSums[:2:2], packagingSums[3:]...), "\n") + "\n")
	otherInstaller := []byte(strings.ReplaceAll(string(fixtureSums()), "tfg-setup_0.4.0", "tfg-setup_0.3.0"))
	otherRelease := []byte(strings.ReplaceAll(string(fixtureSums()), "0.4.0", "0.3.0"))
	unreleased := []byte(strings.ReplaceAll(string(fixtureSums()), "0.4.0", "9.9.9"))

	// In a directory of its own, because the guard lists the destination's
	// parent before and after - and the parent of t.TempDir() is the system's
	// temporary directory, which other processes change while this runs.
	occupied := filepath.Join(t.TempDir(), "packages")
	if err := os.MkdirAll(occupied, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "keep.txt"), []byte("somebody's"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A destination inside the repository must not exist before this runs,
	// and whatever a broken renderer writes there is taken away again - under
	// a mutation that removes the refusal, the renderer writes into the real
	// tree, and the tree is not this guard's to leave changed.
	inside := filepath.Join(repoRoot(t), "packaging-guard-out")
	if _, err := os.Stat(inside); err == nil {
		t.Fatalf("%s already exists, so this guard cannot tell what the renderer wrote there", inside)
	}
	t.Cleanup(func() { _ = os.RemoveAll(inside) })

	for _, c := range []struct {
		what, tag string
		sums      []byte
		out       string
		says      string
	}{
		{"the checksum file of another release", packagingTag, otherRelease, "", "tfg-gui_0.3.0_windows_amd64.zip"},
		{"a release candidate", "v0.4.0-rc1", fixtureSums(), "", "release candidate"},
		{"a version without its v", "0.4.0", fixtureSums(), "", "not a release tag"},
		{"a checksum file missing one archive", packagingTag, withoutArm, "", "tfg_0.4.0_windows_arm64.zip"},
		{"a checksum file without the installer", packagingTag, withoutInstaller(), "", "tfg-setup_0.4.0_windows_amd64.msi, the Windows installer"},
		{"the installer of another release", packagingTag, otherInstaller, "", "It lists tfg-setup_0.3.0_windows_amd64.msi"},
		{"a version the changelog never released", "v9.9.9", unreleased, "", "CHANGELOG.md"},
		{"a destination inside the repository", packagingTag, fixtureSums(), inside, "inside the repository"},
		{"a destination that already holds something", packagingTag, fixtureSums(), occupied, "already holds"},
	} {
		out := c.out
		if out == "" {
			out = filepath.Join(t.TempDir(), "packages")
		}
		before := entriesOf(t, filepath.Dir(out))
		r := renderPackages(t, c.tag, c.sums, out)
		if r.code != 1 {
			t.Errorf("%s: the renderer exited %d, and a refusal exits 1:\n%s", c.what, r.code, r.said)
			continue
		}
		// A crash exits 1 too, and its traceback may well contain the file
		// name this looks for - so a refusal is only a refusal when the
		// renderer said it on purpose.
		if !strings.HasPrefix(r.said, "build_packages: ") || strings.Contains(r.said, "Traceback") {
			t.Errorf("%s: the renderer crashed instead of refusing, and a crash tells a person "+
				"nothing about what to fix:\n%s", c.what, r.said)
			continue
		}
		if !strings.Contains(r.said, c.says) {
			t.Errorf("%s: the refusal does not say %q, so a person reading it cannot tell what "+
				"to fix:\n%s", c.what, c.says, r.said)
		}
		if after := entriesOf(t, filepath.Dir(out)); after != before {
			t.Errorf("%s: the refusal left something behind beside the destination\nbefore: %s\n after: %s",
				c.what, before, after)
		}
	}

	// And the same inputs, put right, render - so every refusal above is about
	// its one input and not about something the fixture always gets wrong.
	if r := renderPackages(t, packagingTag, fixtureSums(), filepath.Join(t.TempDir(), "packages")); r.code != 0 {
		t.Errorf("the fixture itself is refused (exit %d), so the refusals above prove nothing:\n%s", r.code, r.said)
	}
}

// withoutInstaller is the fixture as a release published before the Windows
// installer existed has it - v0.4.0's own checksum file.
func withoutInstaller() []byte {
	var lines []string
	for _, line := range packagingSums {
		if !strings.HasSuffix(line, ".msi") {
			lines = append(lines, line)
		}
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// A release without the installer is refused for WinGet, whose window package
// installs it first, and still renders for Chocolatey, which needs none - the
// packages job in ci.yml renders the latest release, and v0.4.0 has none. Each
// feed rendered alone is, byte for byte, its part of a full render.
func TestAReleaseWithoutTheInstallerStillRendersForChocolatey(t *testing.T) {
	r := renderPackages(t, packagingTag, withoutInstaller(), filepath.Join(t.TempDir(), "packages"))
	if r.code != 1 || !strings.Contains(r.said, "pass --only chocolatey") {
		t.Errorf("without the installer, a render of both feeds exited %d and did not say what "+
			"to do instead:\n%s", r.code, r.said)
	}
	full := renderedPackages(t)
	for _, c := range []struct {
		feed string
		sums []byte
	}{
		{"chocolatey", withoutInstaller()},
		{"winget", fixtureSums()},
	} {
		alone := renderedFrom(t, c.sums, "--only", c.feed)
		want := 0
		for name, body := range full {
			if !strings.HasPrefix(name, c.feed+"/") {
				continue
			}
			want++
			if got, ok := alone[name]; !ok || got != body {
				t.Errorf("--only %s: %s is missing or differs from the full render", c.feed, name)
			}
		}
		if want == 0 || len(alone) != want {
			t.Errorf("--only %s wrote %d file(s), and the full render has %d for that feed",
				c.feed, len(alone), want)
		}
	}
}

// entriesOf lists a directory's names, or says it is absent - which is what a
// refusal must leave the parent of its destination as.
func entriesOf(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return "(absent)"
	}
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return strings.Join(names, ", ")
}

// A file the renderer cannot read, and a destination that leads into the
// repository by another name, are refused with a sentence rather than a Python
// traceback or a write into the tree. An outside review of #143 found both: a
// missing --sums printed an exception, and --out was checked by how it was
// spelled rather than by where it leads.
func TestTheRendererRefusesAFileItCannotReadAndAPathThatLeadsIntoTheTree(t *testing.T) {
	dir := t.TempDir()
	notUTF8 := filepath.Join(dir, "latin1.txt")
	huge := filepath.Join(dir, "huge.txt")
	aFile := filepath.Join(dir, "a-file-not-a-folder")
	for path, body := range map[string][]byte{
		notUTF8: {0xff, 0xfe, 0x41, 0x0a},
		huge:    bytes.Repeat([]byte("a"), 2<<20),
		aFile:   []byte("x"),
	} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// The link points at an EMPTY folder made for this guard inside the tree,
	// not at the tree itself, so no cleanup that followed it could reach
	// anything else. The link is removed before the temporary directory that
	// holds it - cleanups run last registered first.
	target := filepath.Join(repoRoot(t), "packaging-guard-link-target")
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("%s already exists, so this guard cannot tell what the renderer wrote there", target)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(target) })
	link := filepath.Join(t.TempDir(), "into-the-tree")
	if err := makeDirectoryLink(link, target); err != nil {
		t.Fatalf("making a link to %s: %v", target, err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })

	for _, c := range []struct{ what, sums, out, says string }{
		{"a checksum file that is not there", filepath.Join(dir, "missing.txt"),
			filepath.Join(t.TempDir(), "packages"), "cannot read the checksum file"},
		{"a checksum file that is not UTF-8", notUTF8,
			filepath.Join(t.TempDir(), "packages"), "is not UTF-8 text"},
		{"a file far too big to be a checksum file", huge,
			filepath.Join(t.TempDir(), "packages"), "is not a release's checksum file"},
		{"a destination that leads into the tree through a link", sumsFile(t, fixtureSums()),
			filepath.Join(link, "packages"), "inside the repository"},
		{"a destination under something that is a file", sumsFile(t, fixtureSums()),
			filepath.Join(aFile, "packages"), "cannot create a working folder"},
		// A device opens and reads like a file and has no size worth asking:
		// NUL answers nothing, /dev/zero answers forever.
		{"a checksum file that is a device", map[bool]string{true: "NUL", false: "/dev/zero"}[runtime.GOOS == "windows"],
			filepath.Join(t.TempDir(), "packages"), "is not a file"},
	} {
		r := renderFrom(t, packagingTag, c.sums, c.out)
		if r.code != 1 || !strings.HasPrefix(r.said, "build_packages: ") || strings.Contains(r.said, "Traceback") {
			t.Errorf("%s: exit %d, and a refusal is exit 1 with a sentence, not a crash:\n%s", c.what, r.code, r.said)
			continue
		}
		if !strings.Contains(r.said, c.says) {
			t.Errorf("%s: the refusal does not say %q:\n%s", c.what, c.says, r.said)
		}
	}
	if left := entriesOf(t, target); left != "" {
		t.Errorf("the renderer wrote into the tree through the link: %s", left)
	}
}

// A run that fails part way leaves nothing behind - neither its working
// folder nor the folders it made to hold --out - and a destination it cannot
// look into is refused with a sentence. An outside review of #145 found all
// three: the parents made by makedirs outlived a refusal that said "Nothing was
// left behind", no case reached the handler for a write that fails after the
// working folder exists, and os.listdir on --out could still raise.
func TestTheRendererLeavesNothingBehindWhenItFailsPartWay(t *testing.T) {
	unreleased := []byte(strings.ReplaceAll(string(fixtureSums()), "0.4.0", "9.9.9"))
	nested := t.TempDir()
	long := t.TempDir()
	for _, c := range []struct {
		what, tag string
		sums      []byte
		out, base string
		says      string
	}{
		// The refusal comes from the changelog, after the parents are made.
		{"a refusal after new parent folders were made", "v9.9.9", unreleased,
			filepath.Join(nested, "new", "deeper", "packages"), nested, "CHANGELOG.md"},
		// Every file is written into the working folder first, and the name is
		// too long for any file system this runs on only at the last rename.
		{"a write that fails after the working folder exists", packagingTag, fixtureSums(),
			filepath.Join(long, strings.Repeat("x", 300)), long, "cannot write the packages"},
	} {
		r := renderPackages(t, c.tag, c.sums, c.out)
		if r.code != 1 || !strings.HasPrefix(r.said, "build_packages: ") || strings.Contains(r.said, "Traceback") {
			t.Errorf("%s: exit %d, and a refusal is exit 1 with a sentence, not a crash:\n%s", c.what, r.code, r.said)
			continue
		}
		if !strings.Contains(r.said, c.says) {
			t.Errorf("%s: the refusal does not say %q:\n%s", c.what, c.says, r.said)
		}
		if left := entriesOf(t, c.base); left != "" {
			t.Errorf("%s: the failed run left %s behind in %s", c.what, left, c.base)
		}
	}

	// A folder this account cannot list, asked of the system first rather than
	// assumed - an administrator can list some of these, and then the case says
	// so instead of passing on nothing.
	denied := map[string]string{"windows": `C:\System Volume Information`, "darwin": "/private/var/root"}[runtime.GOOS]
	if denied == "" {
		denied = "/root"
	}
	if _, err := os.ReadDir(denied); !os.IsPermission(err) {
		t.Logf("NOT ASKED: %s answered %v rather than a refusal to list it, so there is no folder here "+
			"this account cannot look into", denied, err)
		return
	}
	r := renderPackages(t, packagingTag, fixtureSums(), denied)
	if r.code != 1 || !strings.Contains(r.said, "cannot look inside --out") || strings.Contains(r.said, "Traceback") {
		t.Errorf("a destination this account cannot list: exit %d, and it has to be refused with a sentence:\n%s",
			r.code, r.said)
	}
}

// makeDirectoryLink makes link lead to target: a junction on Windows, which
// needs no privilege where a symbolic link does, and a symbolic link elsewhere.
func makeDirectoryLink(link, target string) error {
	if runtime.GOOS != "windows" {
		return os.Symlink(target, link)
	}
	// Both paths are ones this guard just chose, under its own temporary
	// directory and the repository - nothing a person typed.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("making a junction %s to %s: %w (%s)", link, target, err, out)
	}
	return nil
}

// sha256sum writes '<hash>  <name>' in text mode and '<hash> *<name>' in
// binary mode, and a file saved on Windows may carry a byte order mark and
// CRLF. The star is not part of the name, and none of it may reach an address
// or a checksum the feed compares with.
func TestEverySpellingOfAChecksumFileRendersTheSamePackages(t *testing.T) {
	var lines []string
	for _, line := range packagingSums {
		hash, name, _ := strings.Cut(line, "  ")
		lines = append(lines, strings.ToUpper(hash)+" *"+name)
	}
	bom := []byte{0xEF, 0xBB, 0xBF}
	sums := append(bom, []byte(strings.Join(lines, "\r\n")+"\r\n")...)

	r := renderPackages(t, packagingTag, sums, filepath.Join(t.TempDir(), "packages"))
	if r.code != 0 {
		t.Fatalf("a checksum file written in binary mode on Windows is refused (exit %d):\n%s", r.code, r.said)
	}
	plain := renderedPackages(t)
	for name, want := range plain {
		got, err := os.ReadFile(filepath.Join(r.out, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s was not written from the other spelling: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s differs between two spellings of the same checksums", name)
		}
	}
}

// The checks the renderer makes on every value it puts into a file, asked
// directly. Each case would render, and each would hand the feed a file that
// means something else - so each has to be refused, and the one clean case
// has to render, or the probe cannot tell the two apart.
func TestTheRendererRefusesAValueThatWouldBreakItsFile(t *testing.T) {
	probe := `
import sys
sys.path.insert(0, sys.argv[1])
import build_packages as bp

table = {"OK": "fine", "QUOTE": "it's", "MARKUP": "a & b", "COLON": "key: value",
         "HASH": "a #b", "LINES": "one\ntwo", "DQUOTE": 'say "so"', "LT": "a < b",
         "WIXVAR": "$(PayloadDir)"}
for label, text, name in [
    ("unknown placeholder", "x {{NOT_A_KEY}} y", "chocolatey/tools/a.ps1"),
    ("quote in a script", "Write-Host '{{QUOTE}}'", "chocolatey/tools/a.ps1"),
    ("markup in the nuspec", "<t>{{MARKUP}}</t>", "chocolatey/package.nuspec"),
    ("colon in YAML", "Short: {{COLON}}", "winget/locale.en-US.yaml"),
    ("comment in YAML", "Short: {{HASH}}", "winget/locale.en-US.yaml"),
    ("several lines inside a line", "x {{LINES}} y", "winget/locale.en-US.yaml"),
    ("clean", "Short: {{OK}}", "winget/locale.en-US.yaml"),
    ("double quote in the installer", 'Name="{{DQUOTE}}"', "tfg-setup.wxs"),
    ("bracket in the installer", 'Name="{{LT}}"', "tfg-setup.wxs"),
    ("ampersand in the installer", 'Name="{{MARKUP}}"', "tfg-setup.wxs"),
    ("WiX variable in the installer", 'Name="{{WIXVAR}}"', "tfg-setup.wxs"),
    ("clean installer", 'Name="{{OK}}"', "tfg-setup.wxs"),
]:
    try:
        bp.render(text, table, name)
        print(label + ": RENDERED")
    except SystemExit:
        print(label + ": REFUSED")

orig = bp.values
bp.values = lambda *a: dict(orig(*a), NOBODY_USES_THIS="x")
try:
    bp.build(sys.argv[2], sys.argv[3], sys.argv[4])
    print("unused value: RENDERED")
except SystemExit as refusal:
    print("unused value: REFUSED" if "NOBODY_USES_THIS" in str(refusal) else "unused value: " + str(refusal))
`
	dir := t.TempDir()
	script := filepath.Join(dir, "probe.py")
	if err := os.WriteFile(script, []byte(probe), 0o600); err != nil {
		t.Fatal(err)
	}
	// The interpreter is the one found on PATH, the script is the probe this
	// guard just wrote, and every argument is a value it chose.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(pythonForGate(t), script, filepath.Dir(packagingScript(t)), packagingTag,
		sumsFile(t, fixtureSums()), filepath.Join(dir, "packages"))
	said, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the probe failed: %v\n%s", err, said)
	}
	answers := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^(.+): (RENDERED|REFUSED)\r?$`).FindAllStringSubmatch(string(said), -1) {
		answers[m[1]] = m[2]
	}
	for _, label := range []string{"unknown placeholder", "quote in a script", "markup in the nuspec",
		"colon in YAML", "comment in YAML", "several lines inside a line", "unused value",
		"double quote in the installer", "bracket in the installer", "ampersand in the installer",
		"WiX variable in the installer"} {
		if answers[label] != "REFUSED" {
			t.Errorf("%s: the renderer answered %q, and it has to refuse:\n%s", label, answers[label], said)
		}
	}
	for _, clean := range []string{"clean", "clean installer"} {
		if answers[clean] != "RENDERED" {
			t.Errorf("the %s case was not rendered (%q), so the probe cannot tell a refusal from "+
				"a failure:\n%s", clean, answers[clean], said)
		}
	}
}
