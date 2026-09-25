package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	withoutArm := []byte(strings.Join(packagingSums[:2], "\n") + "\n" + packagingSums[3] + "\n")
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
         "HASH": "a #b", "LINES": "one\ntwo"}
for label, text, name in [
    ("unknown placeholder", "x {{NOT_A_KEY}} y", "chocolatey/tools/a.ps1"),
    ("quote in a script", "Write-Host '{{QUOTE}}'", "chocolatey/tools/a.ps1"),
    ("markup in the nuspec", "<t>{{MARKUP}}</t>", "chocolatey/package.nuspec"),
    ("colon in YAML", "Short: {{COLON}}", "winget/locale.en-US.yaml"),
    ("comment in YAML", "Short: {{HASH}}", "winget/locale.en-US.yaml"),
    ("several lines inside a line", "x {{LINES}} y", "winget/locale.en-US.yaml"),
    ("clean", "Short: {{OK}}", "winget/locale.en-US.yaml"),
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
		"colon in YAML", "comment in YAML", "several lines inside a line", "unused value"} {
		if answers[label] != "REFUSED" {
			t.Errorf("%s: the renderer answered %q, and it has to refuse:\n%s", label, answers[label], said)
		}
	}
	if answers["clean"] != "RENDERED" {
		t.Errorf("the clean case was not rendered (%q), so the probe cannot tell a refusal from "+
			"a failure:\n%s", answers["clean"], said)
	}
}
