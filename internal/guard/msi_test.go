package guard

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
		if errors.Is(err, io.EOF) {
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

// The name the installer is signed with is the name it carries.
//
// sign_release.py signs the installer with what --product-name prints, and
// Windows shows that as the program's name when it asks an administrator to
// let the installer run. Asked of the effect: the printed name is the Name of
// the rendered package, so a template that renames the product renames both.
func TestTheInstallerIsSignedWithTheNameItCarries(t *testing.T) {
	pkg := named(renderedInstaller(t), "Package")
	if len(pkg) != 1 || pkg[0].attrs["Name"] == "" {
		t.Fatalf("read %d <Package> from the installer source, and this guard reads the name of one", len(pkg))
	}
	r := runMSI(t, t.TempDir(), false, "--tag", packagingTag, "--product-name")
	if r.code != 0 {
		t.Fatalf("build_msi.py --product-name exited %d:\n%s", r.code, r.said)
	}
	if got := strings.TrimRight(r.said, "\r\n"); got != pkg[0].attrs["Name"] {
		t.Errorf("build_msi.py --product-name says %q and the package is named %q. The signature "+
			"names the product the installer carries, and the signing script takes every word printed", got, pkg[0].attrs["Name"])
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
		}, "holds LICENSE, one file on Windows, and not the same bytes"},
		// One file to Windows, where the installer puts it. Compared as
		// written, the second name overwrote the first on a Windows disk
		// and nothing was said (outside review of #147).
		{"a document differs and its name only in letter case", func(_ *testing.T, a archives, _, _ string) {
			delete(a[cli], "LICENSE")
			a[cli]["License"] = "another licence"
		}, "holds License, one file on Windows, and not the same bytes"},
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

// pythonDef is one top-level function of a script, cut at the next top-level
// def - by what the text says, never by line numbers.
func pythonDef(t *testing.T, code, name string) string {
	t.Helper()
	start := strings.Index(code, "\ndef "+name+"(")
	if start < 0 {
		t.Fatalf("the script has no function %s", name)
	}
	rest := code[start+1:]
	if end := strings.Index(rest, "\ndef "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// The release signs the installer the way it signs the programs, asks whether
// it can build one before the card signs anything, and does not call a draft
// complete without it.
//
// The installer is what an administrator runs with the highest rights the
// machine has. And a check after the card would leave signed programs on a
// draft that can never get its installer from that run.
func TestTheSigningSignsTheInstallerAndExpectsItOnTheDraft(t *testing.T) {
	script := activePython(signingScript(t))
	build := pythonDef(t, script, "build_installer")
	for what, want := range map[string]string{
		"it timestamps the installer's signature, or it dies with the certificate": `"/tr", TIMESTAMP_URL, "/td", "sha256", "/v", path]`,
		"it reads the installer's certificate back out of the file":                `signer = certificate_of(path)`,
		"it holds that certificate to the pin":                                     `if signer != pin:`,
		"it runs build_msi.py as the tag has it":                                   `installer_script(tree)`,
		"it asks the tag for the name the installer carries":                       `name = product_name(tag, tree)`,
		"it signs the installer with that name":                                    `"/d", name,`,
	} {
		if !strings.Contains(build, want) {
			t.Errorf("%s: build_installer does not contain %q", what, want)
		}
	}
	if !strings.Contains(pythonDef(t, script, "check_installer"), "product_name(tag, tree)") {
		t.Error("check_installer does not ask for the name the installer is signed with, so a tag " +
			"that cannot give one is found only after the card has signed the programs")
	}

	main := pythonDef(t, script, "main")
	check, card := strings.Index(main, "check_installer(args.tag, tree)"), strings.Index(main, "signing_thumbprint(pin)")
	if check < 0 || card < 0 || check > card {
		t.Errorf("main asks whether the installer can be built at %d and reaches the card at %d. "+
			"It asks first, so a machine without WiX stops before anything is signed", check, card)
	}

	draft := pythonDef(t, script, "confirm_draft")
	for what, want := range map[string]string{
		"a release's draft is not complete without exactly one installer": `if not is_candidate(tag) and len(installers) != 1:`,
		"a candidate's draft carries none":                                `if is_candidate(tag) and installers:`,
	} {
		if !statementIn(draft, regexp.QuoteMeta(want)) {
			t.Errorf("%s: no line of confirm_draft begins with %s", what, want)
		}
	}
}

// The installer is built from the tree of the tag, not from the checkout.
//
// Nothing in the signing script knows which commit the checkout stands on, and
// a template changed on main after the tag would otherwise ship in a release
// that never carried it. Asked of the real mechanism: the tree of HEAD is
// exported the way a tag's is, and it has to hold exactly what the commit
// holds - a copy of the working tree would bring along whatever lies in it
// untracked - with build_msi.py taken from inside it.
func TestTheInstallerIsBuiltFromTheTaggedTreeNotTheCheckout(t *testing.T) {
	probe := `
import os, sys
sys.path.insert(0, sys.argv[1])
import sign_release as sr

base = sys.argv[2]
tree = os.path.join(base, "tree")
sr.export_tree("HEAD", tree)
count = sum(len(files) for _, _, files in os.walk(tree))
print("files: %d" % count)
print("script: " + os.path.relpath(sr.installer_script(tree), base).replace(os.sep, "/"))
print("name: " + sr.product_name("v0.5.0", tree))
for tag in ("v0.5.0", "v0.5.0-rc1", "v1.0.0-beta.2"):
    print("candidate %s: %s" % (tag, sr.is_candidate(tag)))
try:
    sr.export_tree("refs/tags/no-such-tag", os.path.join(base, "none"))
    print("missing tag: EXPORTED")
except SystemExit as refusal:
    print("missing tag: " + ("REFUSED" if "git fetch --tags" in str(refusal) else str(refusal)))
kept = os.path.join(base, "kept")
try:
    with sr.tagged_tree("HEAD", kept):
        print("inside: %s" % os.path.isfile(os.path.join(kept, "go.mod")))
        raise SystemExit("a refusal inside")
except SystemExit:
    pass
print("left after a refusal: %s" % os.path.exists(kept))
`
	dir := t.TempDir()
	file := filepath.Join(dir, "probe.py")
	if err := os.WriteFile(file, []byte(probe), 0o600); err != nil {
		t.Fatal(err)
	}
	// The interpreter is the one found on PATH, the script is the probe this
	// guard just wrote, and every argument is a path it chose.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(pythonForGate(t), file, filepath.Join(repoRoot(t), ".github", "scripts"), dir)
	cmd.Dir = repoRoot(t)
	said, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the probe failed: %v\n%s", err, said)
	}
	committed := len(strings.Fields(gitOutput(t, "ls-tree", "-r", "--name-only", "HEAD")))
	pkg := named(renderedInstaller(t), "Package")
	if len(pkg) != 1 {
		t.Fatalf("read %d <Package> from the installer source, and this guard reads the name of one", len(pkg))
	}
	for _, want := range []string{
		"files: " + strconv.Itoa(committed),
		"script: tree/.github/scripts/build_msi.py",
		"name: " + pkg[0].attrs["Name"],
		"candidate v0.5.0: False",
		"candidate v0.5.0-rc1: True",
		"candidate v1.0.0-beta.2: True",
		"missing tag: REFUSED",
		"inside: True",
		"left after a refusal: False",
	} {
		if !strings.Contains(string(said), want+"\n") && !strings.Contains(string(said), want+"\r\n") {
			t.Errorf("the probe did not say %q:\n%s", want, said)
		}
	}
}

// One rule for a release candidate, in every place that asks it: a hyphen in
// the tag. release.yml marks such a release a pre-release, build_msi.py builds
// it no installer, and the signing script neither builds one nor expects one.
// Two rules would disagree about a tag like v1.0.0-beta, and the one that
// says "release" would put an installer on a candidate's page.
func TestEveryPlaceAsksTheSameQuestionOfACandidate(t *testing.T) {
	if !strings.Contains(withoutYamlComments(workflowText(t, "release.yml")), "*-*) flags+=(--prerelease) ;;") {
		t.Error("release.yml no longer marks a tag with a hyphen as a pre-release in the words this guard reads")
	}
	if !statementIn(pythonDef(t, activePython(signingScript(t)), "is_candidate"), `return "-" in tag$`) {
		t.Error("sign_release.py does not call a tag with a hyphen a candidate")
	}
	if !statementIn(activePython(readRepoFile(t, ".github/scripts/build_msi.py")), `if "-" in tag:$`) {
		t.Error("build_msi.py does not refuse a tag with a hyphen as a candidate")
	}
	if !strings.Contains(releaseStepRun(t, "assets"), `*-*) test "${#installers[@]}" = "0"`) {
		t.Error("verify-release.yml does not expect no installer on a tag with a hyphen")
	}
}

// releaseStepRun is the script of one step of verify-release.yml, found by its
// id, with its comment lines taken out.
func releaseStepRun(t *testing.T, id string) string {
	t.Helper()
	var found []string
	for _, job := range readReleaseWorkflow(t).Jobs {
		for _, step := range job.Steps {
			if step.ID == id {
				found = append(found, strings.Join(scriptLines(step.Run), "\n"))
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("verify-release.yml has %d step(s) with the id %s, and this reads exactly one", len(found), id)
	}
	return found[0]
}

// The last phase asks the page a person downloads from for the installer: one
// for a release, under the name build_msi.py gives it, and signed by our
// certificate with a timestamp. The two phases before it are run by the
// people who made the release - this one is the check that looks at what was
// published.
func TestTheReleaseCheckAsksForTheInstaller(t *testing.T) {
	name := regexp.MustCompile(`(?m)^NAME = "([^"]+)"$`).FindStringSubmatch(readRepoFile(t, ".github/scripts/build_msi.py"))
	if name == nil || !strings.Contains(name[1], "{version}") {
		t.Fatal("build_msi.py names no installer with a {version} in it, so there is nothing to hold the check to")
	}
	published := strings.Replace(name[1], "{version}", "${TAG#v}", 1)
	if !strings.Contains(releaseStepRun(t, "assets"), `test -f "`+published+`"`) {
		t.Errorf("the asset count does not ask for %s, the name build_msi.py gives the installer - "+
			"a release would pass it with the installer missing or misnamed", published)
	}
	signature := releaseStepRun(t, "installer_signature")
	for what, want := range map[string]string{
		"it reads the installer's signature":           "Get-AuthenticodeSignature -LiteralPath $msi.FullName",
		"it refuses an installer with no timestamp":    "if (-not $sig.TimeStamperCertificate)",
		"it refuses one signed by another certificate": "if ($actual -ne $pinned)",
	} {
		if !strings.Contains(signature, want) {
			t.Errorf("%s: the installer's signature step does not contain %q", what, want)
		}
	}
}

// What the installer does on a machine is asked by one job in ci.yml, and the
// guards above only hold its source - so the job is held here, the way the
// import table's is: a job that stopped building the installer, stopped
// installing it or stopped failing on a failed check would leave every guard
// green and the installer asked by nobody.
//
// The WiX version is read out of build_msi.py rather than typed here, because
// the script refuses every other version and a job installing another one
// would stop at that refusal instead of at the install.
func TestTheInstallerIsInstalledOnARunner(t *testing.T) {
	jobs := ciJobs(workflowText(t, "ci.yml"))
	if len(jobs) < 5 {
		t.Fatalf("only %d job(s) were read out of ci.yml, so this guard is not reading the file it thinks it is", len(jobs))
	}
	var builders []string
	for job, block := range jobs {
		if strings.Contains(withoutYamlComments(block), "python .github/scripts/build_msi.py") {
			builders = append(builders, job)
		}
	}
	if len(builders) != 1 {
		t.Fatalf("%d job(s) in ci.yml build the installer, and exactly one has to: %v", len(builders), builders)
	}
	job := builders[0]
	block := withoutYamlComments(jobs[job])

	wix := regexp.MustCompile(`(?m)^WIX_VERSION = "([^"]+)"$`).FindStringSubmatch(readRepoFile(t, ".github/scripts/build_msi.py"))
	if wix == nil {
		t.Fatal("build_msi.py names no WIX_VERSION, so there is nothing to hold the job to")
	}
	for what, want := range map[string]string{
		"it runs on Windows, the only system that installs it":               "runs-on: windows-latest",
		"it installs the WiX version build_msi.py builds with":               "dotnet tool install --global wix --version " + wix[1],
		"it installs silently, as a deployment does":                         "'/qn'",
		"it asks the command line through PATH, from a new process":          "cmd.exe /c 'tfg version'",
		"it asks where the shortcut starts the window":                       "$link.WorkingDirectory",
		"it asks an upgrade while a tfg run is in progress":                  "$held.HasExited",
		"it uninstalls and asks what is left":                                "Msi '/x'",
		"it fails the step when a check failed, rather than printing FAILED": `throw "$failed check(s) failed`,
	} {
		if !strings.Contains(block, want) {
			t.Errorf("%s: job %q does not contain %q", what, job, want)
		}
	}
}
