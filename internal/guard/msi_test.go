package guard

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The Windows installer, and the script that builds it.
//
// What the installer DOES on a machine is asked by the job in ci.yml that
// installs it on a Windows runner, and before that on a virtual machine
// (docs/PACKAGING-2026-09-25.md section 13). These guards hold what that job
// cannot see coming: the lines each of those measurements rests on, a value
// that must never change, and the refusals of the script. Every refusal comes
// before WiX is asked for, so these run on every system, WiX or not.

// installerUpgradeCode is the product's identity in Windows Installer, for
// good. Every machine that has the program finds the version it has through
// it, so a new one would leave the old install beside the new one on every
// one of them. Written here a second time on purpose: this is the copy that
// does not move when the script does.
const installerUpgradeCode = "7DB637B7-3BEB-4FBD-BE84-60822854AA2F"

func msiScript(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "build_msi.py")
}

// runMSI runs build_msi.py with a temporary folder of its own, so a guard can
// see what a run leaves behind. withoutWix also takes WiX out of its reach -
// an empty PATH and a home with no .dotnet in it - so a run that gets past
// every check on the archives stops at the same place on every system.
func runMSI(t *testing.T, temp string, withoutWix bool, args ...string) rendering {
	t.Helper()
	python := pythonForGate(t)
	env := append(os.Environ(), "TMP="+temp, "TEMP="+temp, "TMPDIR="+temp)
	if withoutWix {
		nowhere := t.TempDir()
		env = append(env, "PATH="+nowhere, "HOME="+nowhere, "USERPROFILE="+nowhere)
	}
	// The interpreter is the one found on PATH, the script is a file of this
	// repository, and every argument is a value this guard chose.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(python, append([]string{msiScript(t)}, args...)...)
	cmd.Dir = repoRoot(t)
	cmd.Env = env
	said, err := cmd.CombinedOutput()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("build_msi.py could not be started: %v", err)
	}
	return rendering{said: string(said), code: code}
}

// wxsElement is one element of the installer source: its name, its namespace,
// its attributes and the element it sits in. Comments are not elements, so a
// word in the explanation of a line never counts as the line.
type wxsElement struct {
	name, space string
	attrs       map[string]string
	parent      int
}

func installerElements(t *testing.T, source string) []wxsElement {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(source))
	var found []wxsElement
	open := []int{-1}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("the installer source is not XML: %v", err)
		}
		switch el := tok.(type) {
		case xml.StartElement:
			attrs := map[string]string{}
			for _, a := range el.Attr {
				key := a.Name.Local
				if a.Name.Space != "" {
					key = a.Name.Space + ":" + a.Name.Local
				}
				attrs[key] = a.Value
			}
			found = append(found, wxsElement{el.Name.Local, el.Name.Space, attrs, open[len(open)-1]})
			open = append(open, len(found)-1)
		case xml.EndElement:
			open = open[:len(open)-1]
		}
	}
	return found
}

// renderedInstaller is the installer source of the fixture release, rendered
// the way the build renders it.
func renderedInstaller(t *testing.T) []wxsElement {
	t.Helper()
	r := runMSI(t, t.TempDir(), false, "--tag", packagingTag, "--source-only")
	if r.code != 0 {
		t.Fatalf("build_msi.py refused to render %s (exit %d):\n%s", packagingTag, r.code, r.said)
	}
	return installerElements(t, r.said)
}

// named returns the elements of one name, and one whose attribute has a value.
func named(els []wxsElement, name string) []wxsElement {
	var out []wxsElement
	for _, e := range els {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

func withAttr(els []wxsElement, name, attr, value string) []wxsElement {
	var out []wxsElement
	for _, e := range named(els, name) {
		if e.attrs[attr] == value {
			out = append(out, e)
		}
	}
	return out
}

// The lines the measurements rest on, read from the rendered source.
//
// Each was measured on Windows Server 2025 before it was written, and each
// one changed would still build a working installer - which is why it needs
// a guard rather than a build. What breaks is an upgrade, an uninstall, or
// another account's Start menu, on somebody else's machine.
func TestTheInstallerIsTheShapeThatWasMeasured(t *testing.T) {
	els := renderedInstaller(t)
	if len(els) < 10 {
		t.Fatalf("read %d element(s) from the installer source, so this guard is not reading it", len(els))
	}

	// No extension, no custom action, nothing that ends a program. The
	// owner's decision of 2026-09-25: a run in progress may be half way
	// through a set of files. An extension also puts code of its own into the
	// package, and then the package is not only our files.
	for _, e := range els {
		if e.space != "http://wixtoolset.org/schemas/v4/wxs" {
			t.Errorf("<%s> is from %q, an extension of WiX - the installer carries none", e.name, e.space)
		}
		for key := range e.attrs {
			if strings.HasPrefix(key, "xmlns:") {
				t.Errorf("<%s> brings in the namespace %s - the installer uses no extension", e.name, key)
			}
		}
		switch e.name {
		case "CustomAction", "CloseApplication", "InstallExecuteSequence", "UI", "UIRef":
			t.Errorf("the installer has a <%s>. It runs no action of its own and ends nothing that "+
				"is running, and its only dialogs are Windows Installer's", e.name)
		}
	}

	pkg := named(els, "Package")
	if len(pkg) != 1 {
		t.Fatalf("the source has %d <Package>, and it is one package", len(pkg))
	}
	if got := pkg[0].attrs["UpgradeCode"]; got != installerUpgradeCode {
		t.Errorf("the UpgradeCode is %q. It is %s for good - with another one every machine "+
			"keeps the old install beside the new one", got, installerUpgradeCode)
	}
	if got := pkg[0].attrs["Scope"]; got != "perMachine" {
		t.Errorf("the package installs %q. It installs for the machine, so the command line is "+
			"on PATH for a build agent and the window in every account's Start menu", got)
	}

	upgrade := named(els, "MajorUpgrade")
	if len(upgrade) != 1 || upgrade[0].attrs["Schedule"] != "afterInstallExecute" ||
		upgrade[0].attrs["AllowSameVersionUpgrades"] != "yes" {
		t.Errorf("the major upgrade is %v. It removes the old version after the new files are "+
			"in place (afterInstallExecute), and a rebuild of the same version replaces the first "+
			"build instead of installing beside it (AllowSameVersionUpgrades)", upgrade)
	}

	// Measured: with the Restart Manager on, an upgrade while tfg ran failed
	// after thirty seconds and closed the program anyway. It must be in the
	// package, because the old package's properties decide the upgrade.
	manager := withAttr(els, "Property", "Id", "MSIRESTARTMANAGERCONTROL")
	if len(manager) != 1 || manager[0].attrs["Value"] != "Disable" {
		t.Errorf("the package sets MSIRESTARTMANAGERCONTROL as %v. It turns the Restart Manager "+
			"off with Disable, or an upgrade while tfg runs fails and ends the run", manager)
	}

	env := named(els, "Environment")
	if len(env) != 1 {
		t.Fatalf("the source has %d <Environment>, and it sets one PATH entry", len(env))
	}
	for attr, want := range map[string]string{
		"Name": "PATH", "System": "yes", "Part": "last", "Value": "[INSTALLFOLDER]", "Permanent": "no",
	} {
		if got := env[0].attrs[attr]; got != want {
			t.Errorf("the PATH entry has %s=%q, want %q: the folder goes on the machine's PATH, "+
				"after everything already there, and comes off again at uninstall", attr, got, want)
		}
	}

	shortcuts := named(els, "Shortcut")
	if len(shortcuts) != 1 {
		t.Fatalf("the source has %d shortcut(s). The window has one, and the command line none", len(shortcuts))
	}
	if got := shortcuts[0].attrs["Target"]; got != "[INSTALLFOLDER]tfg-gui.exe" {
		t.Errorf("the shortcut starts %q, and it starts the window", got)
	}
	if got := shortcuts[0].attrs["WorkingDirectory"]; got != "INSTALLFOLDER" {
		t.Errorf("the shortcut starts in %q. It starts in the install folder, which the window "+
			"recognises as its own and sends its offer to the home folder instead. A profile "+
			"variable is expanded at install time, in the installing account", got)
	}

	folder := withAttr(els, "Directory", "Id", "INSTALLFOLDER")
	if len(folder) != 1 || folder[0].parent < 0 ||
		els[folder[0].parent].attrs["Id"] != "ProgramFiles64Folder" {
		t.Error("the install folder is not directly under ProgramFiles64Folder")
	}
}

// What a person reads from the installer follows the punctuation rule (D17): a
// flat hyphen, and no semicolons. The refusal to go back to an older version
// is the one sentence a person sees from it, and the shortcut's description
// is its tooltip.
func TestTheInstallerTextFollowsThePunctuationRule(t *testing.T) {
	forbidden := string([]rune{';', rune(0x2013), rune(0x2014)})
	read := 0
	for _, e := range renderedInstaller(t) {
		for _, attr := range []string{"DowngradeErrorMessage", "Description", "Name"} {
			text, ok := e.attrs[attr]
			if !ok || (attr == "Name" && e.name != "Package" && e.name != "Shortcut") {
				continue
			}
			read++
			if strings.ContainsAny(text, forbidden) {
				t.Errorf("<%s %s> shows a person %q, which breaks the punctuation rule", e.name, attr, text)
			}
		}
	}
	if read < 4 {
		t.Errorf("read %d line(s) a person sees, and the installer shows four", read)
	}
}

// A release candidate gets no installer, and a version Windows Installer
// cannot hold is refused rather than cut.
//
// A tag with a hyphen is a candidate - the rule release.yml marks a
// pre-release by. Windows Installer reads only the three numbers, so a
// candidate's installer would carry the release's own version and the release
// could not replace it.
func TestTheInstallerIsBuiltForAReleaseOnly(t *testing.T) {
	for _, c := range []struct{ tag, says string }{
		{"v0.5.0-rc1", "is a release candidate"},
		{"v0.5.0-beta.2", "is a release candidate"},
		{"0.5.0", "is not a release tag"},
		{"v256.0.0", "does not fit an installer version"},
		{"v0.256.0", "does not fit an installer version"},
		{"v0.0.65536", "does not fit an installer version"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			r := runMSI(t, t.TempDir(), false, "--tag", c.tag, "--source-only")
			if r.code != 1 || !strings.Contains(r.said, c.says) {
				t.Errorf("build_msi.py --tag %s exited %d and said:\n%s\nwant exit 1 and %q", c.tag, r.code, r.said, c.says)
			}
		})
	}
	// And the largest version that fits is not refused, so the cases above
	// are refused for what they are.
	if r := runMSI(t, t.TempDir(), false, "--tag", "v255.255.65535", "--source-only"); r.code != 0 {
		t.Errorf("the largest version an installer holds is refused (exit %d):\n%s", r.code, r.said)
	}
}

// writeZipOf writes a zip archive holding the given files.
func writeZipOf(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	w := zip.NewWriter(f)
	var names []string
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		part, err := w.Create(name)
		if err == nil {
			_, err = part.Write([]byte(files[name]))
		}
		if err != nil {
			t.Fatalf("writing %s into %s: %v", name, path, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing %s: %v", path, err)
	}
}

// The installer is built from both signed archives, or not at all - and a run
// that refuses leaves nothing behind, neither in the folder it was to write
// into nor in its own working folder.
//
// Each case differs from a set of archives that gets through in one thing,
// and that set is asked first: it reaches the step that asks for WiX, which
// this guard keeps out of reach. So each refusal below is the refusal of what
// the case changed, not of something the fixture got wrong.
func TestTheInstallerIsBuiltFromBothArchivesOrNotAtAll(t *testing.T) {
	const tag, version = "v9.9.9", "9.9.9"
	cli := "tfg_" + version + "_windows_amd64.zip"
	window := "tfg-gui_" + version + "_windows_amd64.zip"
	installer := "tfg-setup_" + version + "_windows_amd64.msi"
	good := func() map[string]map[string]string {
		return map[string]map[string]string{
			cli:    {"tfg.exe": "the command line", "LICENSE": "the licence", "README.md": "read me"},
			window: {"tfg-gui.exe": "the window", "opengl/opengl32.dll": "a renderer", "LICENSE": "the licence", "README.md": "read me"},
		}
	}

	type archives = map[string]map[string]string
	for _, c := range []struct {
		what   string
		change func(t *testing.T, a archives, dir, out string)
		says   string
	}{
		{"nothing wrong with the archives", func(*testing.T, archives, string, string) {},
			"dotnet tool install --global wix --version 5.0.2"},
		{"a document differs between the two archives", func(_ *testing.T, a archives, _, _ string) {
			a[window]["LICENSE"] = "another licence"
		}, "both hold LICENSE, and not the same bytes"},
		{"the command line archive is missing", func(_ *testing.T, a archives, _, _ string) {
			delete(a, cli)
		}, "holds no " + cli},
		{"an archive holds no program", func(_ *testing.T, a archives, _, _ string) {
			delete(a[cli], "tfg.exe")
		}, "hold no tfg.exe at the top"},
		// Deep enough to leave the run's working folder too: unpacked as
		// written, it would land beside the folders of this case, where the
		// guard looks for it below.
		{"a name inside an archive leads out of the folder", func(_ *testing.T, a archives, _, _ string) {
			a[cli]["../../../escaped.txt"] = "out"
		}, "a name that leads out of the folder"},
		{"an archive is not a zip", func(t *testing.T, a archives, dir, _ string) {
			delete(a, cli)
			if err := os.WriteFile(filepath.Join(dir, cli), []byte("not a zip"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "is not a zip archive"},
		{"the installer is already there", func(t *testing.T, _ archives, _, out string) {
			if err := os.WriteFile(filepath.Join(out, installer), []byte("an earlier one"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "is already there. Nothing is overwritten"},
	} {
		t.Run(c.what, func(t *testing.T) {
			base := t.TempDir()
			dir, out, temp := filepath.Join(base, "archives"), filepath.Join(base, "out"), filepath.Join(base, "temp")
			for _, d := range []string{dir, out, temp} {
				if err := os.Mkdir(d, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			set := good()
			c.change(t, set, dir, out)
			for name, files := range set {
				writeZipOf(t, filepath.Join(dir, name), files)
			}
			before := entriesOf(t, out)

			r := runMSI(t, temp, true, "--tag", tag, "--archives", dir, "--out-dir", out)
			if r.code != 1 || !strings.Contains(r.said, c.says) {
				t.Errorf("exit %d, and build_msi.py said:\n%s\nwant exit 1 and %q", r.code, r.said, c.says)
			}
			if strings.Contains(r.said, "github.com/wixtoolset") {
				t.Errorf("the refusal gives an address to download WiX from. It gives the command " +
					"that installs the pinned version, and nothing to click")
			}
			if after := entriesOf(t, out); after != before {
				t.Errorf("--out-dir held %q before and %q after a run that refused", before, after)
			}
			if left := entriesOf(t, temp); left != "" {
				t.Errorf("a run that refused left %q in its working folder", left)
			}
			if _, err := os.Stat(filepath.Join(base, "escaped.txt")); err == nil {
				t.Error("a name inside an archive wrote a file outside the folder it was unpacked into")
			}
		})
	}
}
