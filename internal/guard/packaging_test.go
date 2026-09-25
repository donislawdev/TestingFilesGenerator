package guard

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The WinGet and Chocolatey packages, and the renderer that fills them.
//
// These files are published under the project's name to feeds somebody else
// moderates, so a mistake costs a stranger's review time, and for the few
// lines that decide whether an install works at all, a person whose install
// does nothing. What is guarded here is what a reviewer cannot catch for us.
// What the packages DO on a real machine is not a text question - the job in
// ci.yml that installs them on a Windows runner answers that, and the
// measurement on a virtual machine before each submission
// (docs/PACKAGING-2026-09-25.md section 8).
//
// Every guard renders a real release: v0.4.0, with the checksums of its real
// archives, so the guards read the same shape a person packaging it reads.

const packagingTag = "v0.4.0"

var packagingSums = []string{
	"e744f4ff793407218fac9eec139dd49c03264ab636db2057ba2963d0515625ad  tfg-gui_0.4.0_windows_amd64.zip",
	"16fe56c7b3f2a13d22385a6b428ed6f0c9fb98c2103076b8c7c876401eeaaf79  tfg_0.4.0_windows_amd64.zip",
	"3f74f66e181bdef20785bccd603c3e5c5ad04cdff3b38338487a02b1682eccc6  tfg_0.4.0_windows_arm64.zip",
	// A line no package needs, because the real file has eight of them.
	"c7a63f918db43cf2841a89359d3ef272c861a89a408018f6453dea7f7aaebb96  tfg_0.4.0_linux_amd64.tar.gz",
}

// Where each package lands, in the layout the renderer writes. WinGet's
// follows winget-pkgs, so the three files can be copied across as they are.
const (
	windowWinget = "winget/manifests/d/DonislawDev/TestingFilesGenerator/0.4.0/DonislawDev.TestingFilesGenerator"
	cliWinget    = "winget/manifests/d/DonislawDev/TestingFilesGenerator/CLI/0.4.0/DonislawDev.TestingFilesGenerator.CLI"
	windowChoco  = "chocolatey/testing-files-generator/"
	cliChoco     = "chocolatey/testing-files-generator-cli/"
)

// rendering is one run of the renderer: where it was told to write, what it
// said, and how it exited.
type rendering struct {
	out  string
	said string
	code int
}

func packagingScript(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "build_packages.py")
}

// sumsFile writes a checksum file and returns its path.
func sumsFile(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "verify-SHA256SUMS.txt")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing the checksum file: %v", err)
	}
	return path
}

func fixtureSums() []byte {
	return []byte(strings.Join(packagingSums, "\n") + "\n")
}

// renderPackages runs the renderer the way a person does.
func renderPackages(t *testing.T, tag string, sums []byte, out string) rendering {
	t.Helper()
	python := pythonForGate(t)
	cmd := exec.Command(python, packagingScript(t),
		"--tag", tag, "--sums", sumsFile(t, sums), "--out", out)
	cmd.Dir = repoRoot(t)
	said, err := cmd.CombinedOutput()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("the renderer could not be started: %v", err)
	}
	return rendering{out: out, said: string(said), code: code}
}

// renderedPackages renders the fixture release and returns every file it
// wrote, by its path under the output directory.
func renderedPackages(t *testing.T) map[string]string {
	t.Helper()
	r := renderPackages(t, packagingTag, fixtureSums(), filepath.Join(t.TempDir(), "packages"))
	if r.code != 0 {
		t.Fatalf("the renderer refused a real release (exit %d):\n%s", r.code, r.said)
	}
	files := map[string]string{}
	err := filepath.WalkDir(r.out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		rel, _ := filepath.Rel(r.out, path)
		files[filepath.ToSlash(rel)] = string(body)
		return err
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("reading what the renderer wrote: %v (%d files)", err, len(files))
	}
	return files
}

// scriptLines drops blank lines and the comments of a PowerShell or YAML file,
// so a guard reads what runs rather than what explains it. The first version
// of a guard like this in the sibling project passed on a script whose only
// mention of the flag was the comment saying why it was there.
func scriptLines(text string) []string {
	var code []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			code = append(code, line)
		}
	}
	return code
}

func anyLine(lines []string, pattern string) bool {
	re := regexp.MustCompile(pattern)
	for _, line := range lines {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// packagingTemplates returns every template in packaging/, by its path under
// the repository.
func packagingTemplates(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	found := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, "packaging"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".in") {
			return err
		}
		body, err := os.ReadFile(path)
		rel, _ := filepath.Rel(root, path)
		found[filepath.ToSlash(rel)] = string(body)
		return err
	})
	if err != nil || len(found) == 0 {
		t.Fatalf("reading packaging/: %v (%d templates)", err, len(found))
	}
	return found
}

// The renderer writes these files and no others. The identifiers in them are
// public for good: a package renamed is a new package, and everybody who has
// the old one keeps it.
func TestThePackagesRenderFromAReleasesChecksums(t *testing.T) {
	files := renderedPackages(t)
	want := []string{
		windowChoco + "testing-files-generator.nuspec",
		windowChoco + "tools/chocolateybeforemodify.ps1",
		windowChoco + "tools/chocolateyinstall.ps1",
		windowChoco + "tools/chocolateyuninstall.ps1",
		cliChoco + "testing-files-generator-cli.nuspec",
		cliChoco + "tools/chocolateybeforemodify.ps1",
		cliChoco + "tools/chocolateyinstall.ps1",
		windowWinget + ".installer.yaml",
		windowWinget + ".locale.en-US.yaml",
		windowWinget + ".yaml",
		cliWinget + ".installer.yaml",
		cliWinget + ".locale.en-US.yaml",
		cliWinget + ".yaml",
	}
	var got []string
	for name := range files {
		got = append(got, name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the renderer wrote a different set of files than the two packages in two "+
			"feeds are made of\n got: %v\nwant: %v", got, want)
	}
}

// A version typed into a template is a version that stays behind when the
// next release is rendered, and a manifest with a stale version still parses.
// The one number allowed is the manifest schema, which is not a release.
func TestNoPackageSourceCarriesAVersionNumber(t *testing.T) {
	version := regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)
	for name, text := range packagingTemplates(t) {
		if found := version.FindString(text); found != "" {
			t.Errorf("%s carries the version %s. Versions come from the tag being packaged", name, found)
		}
	}
}

// Both WinGet packages keep the program beside the files it came with.
//
// Without ArchiveBinariesDependOnPath WinGet reaches the program through a
// symbolic link, and the window started that way looks for its software
// renderer next to the link, where it is not. With it, the package's folder
// goes on PATH. An alias would ask for the very link this avoids.
func TestEveryWingetPackageKeepsItsProgramBesideItsFiles(t *testing.T) {
	files := renderedPackages(t)
	for manifest, program := range map[string]string{
		windowWinget + ".installer.yaml": "tfg-gui.exe",
		cliWinget + ".installer.yaml":    "tfg.exe",
	} {
		code := scriptLines(files[manifest])
		if !anyLine(code, `^ArchiveBinariesDependOnPath: true$`) {
			t.Errorf("%s does not set ArchiveBinariesDependOnPath, so WinGet reaches the "+
				"program through a link and the window loses its renderer", manifest)
		}
		if !anyLine(code, `^- RelativeFilePath: `+regexp.QuoteMeta(program)+`$`) {
			t.Errorf("%s does not install %s, the program the archive holds", manifest, program)
		}
		if anyLine(code, `PortableCommandAlias`) {
			t.Errorf("%s names an alias, which is the link this package exists to avoid", manifest)
		}
	}
}

// The command line ships for both Windows architectures and the window for
// one, each with the checksum the release published for that archive.
func TestEachWingetPackageOffersTheArchitecturesTheReleaseBuilds(t *testing.T) {
	files := renderedPackages(t)
	for manifest, want := range map[string][]string{
		windowWinget + ".installer.yaml": {"x64 E744F4FF793407218FAC9EEC139DD49C03264AB636DB2057BA2963D0515625AD"},
		cliWinget + ".installer.yaml": {
			"x64 16FE56C7B3F2A13D22385A6B428ED6F0C9FB98C2103076B8C7C876401EEAAF79",
			"arm64 3F74F66E181BDEF20785BCCD603C3E5C5AD04CDFF3B38338487A02B1682ECCC6",
		},
	} {
		installer := regexp.MustCompile(`(?m)^- Architecture: (\S+)\n  InstallerUrl: \S+\n  InstallerSha256: (\S+)$`)
		var got []string
		for _, m := range installer.FindAllStringSubmatch(files[manifest], -1) {
			got = append(got, m[1]+" "+m[2])
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s offers %v, and the release built %v", manifest, got, want)
		}
	}
}

// Every archive a package downloads is one the release workflow builds.
//
// The renderer spells the archive names, and so does release.yml. This reads
// the name pattern out of the workflow and holds every address the packages
// download from to it, so renaming the archives in one place and not the
// other reddens here instead of in a feed's moderation queue.
func TestThePackagesDownloadWhatTheReleaseBuilds(t *testing.T) {
	release := withoutYamlComments(workflowText(t, "release.yml"))
	bases := regexp.MustCompile(`base="(tfg(?:-gui)?)_\$\{version\}_\$\{label\}_\$\{arch\}"`).
		FindAllStringSubmatch(release, -1)
	if len(bases) != 2 {
		t.Fatalf("release.yml names its archives %d way(s) this guard can read, and it reads "+
			"exactly two - the command line and the window. Read the base= lines again", len(bases))
	}
	built := regexp.MustCompile(`^(` + bases[0][1] + `|` + bases[1][1] + `)_0\.4\.0_windows_(amd64|arm64)\.zip$`)
	address := regexp.MustCompile(`https://github\.com/[^/\s']+/[^/\s']+/releases/download/v0\.4\.0/([^\s']+)`)
	seen := 0
	for name, text := range renderedPackages(t) {
		for _, m := range address.FindAllStringSubmatch(text, -1) {
			seen++
			if !built.MatchString(m[1]) {
				t.Errorf("%s downloads %s, which is not a name release.yml builds", name, m[1])
			}
		}
	}
	if seen < 5 {
		t.Errorf("found %d download address(es) in the packages, and they hold five - this "+
			"guard is not reading what it thinks it reads", seen)
	}
}

// A Chocolatey package that downloads has to check what it downloaded against
// the release's own checksum, or it installs whatever the address answers.
func TestTheChocolateyPackagesCheckWhatTheyDownload(t *testing.T) {
	files := renderedPackages(t)
	for script, sum := range map[string]string{
		windowChoco + "tools/chocolateyinstall.ps1": "e744f4ff793407218fac9eec139dd49c03264ab636db2057ba2963d0515625ad",
		cliChoco + "tools/chocolateyinstall.ps1":    "16fe56c7b3f2a13d22385a6b428ed6f0c9fb98c2103076b8c7c876401eeaaf79",
	} {
		code := scriptLines(files[script])
		if !anyLine(code, `^\s*-Checksum64 '`+sum+`' `) {
			t.Errorf("%s does not check the download against the release's checksum %s", script, sum)
		}
		if !anyLine(code, `^\s*-ChecksumType64 'sha256' `) {
			t.Errorf("%s does not say the checksum is sha256", script)
		}
	}
}

// Not raw.githubusercontent.com, not github.com/.../raw, not a branch.
//
// Chocolatey's moderation refuses the first two alike - the sibling project's
// 0.5.0 was held over it - and an icon on a branch keeps changing under a
// package that is already approved and out of reach.
func TestTheChocolateyIconIsAPinnedCdnAddress(t *testing.T) {
	files := renderedPackages(t)
	for _, nuspec := range []string{
		windowChoco + "testing-files-generator.nuspec",
		cliChoco + "testing-files-generator-cli.nuspec",
	} {
		found := regexp.MustCompile(`<iconUrl>(.*?)</iconUrl>`).FindStringSubmatch(files[nuspec])
		if found == nil {
			t.Errorf("%s carries no icon", nuspec)
			continue
		}
		if !strings.HasPrefix(found[1], "https://cdn.jsdelivr.net/gh/") {
			t.Errorf("%s serves its icon from %s, which is not a CDN moderation accepts", nuspec, found[1])
		}
		if !strings.Contains(found[1], "@"+packagingTag+"/") {
			t.Errorf("%s does not pin its icon to %s: %s", nuspec, packagingTag, found[1])
		}
	}
}

// The window's Chocolatey package starts it without holding the terminal, and
// its shortcut starts it where the person can write.
//
// The shim waits for the program unless a .gui file lies beside it. The
// shortcut's working directory is the directory the window offers its tfg-out
// folder under, and the package's own folder is one an ordinary account cannot
// write to - owner's decision of 2026-09-25, the user's profile. And the
// uninstall removes only a shortcut that points into the package.
func TestTheWindowPackageStartsWhereAPersonCanWrite(t *testing.T) {
	files := renderedPackages(t)
	install := scriptLines(files[windowChoco+"tools/chocolateyinstall.ps1"])
	if !anyLine(install, `New-Item -ItemType File -Path "\$exe\.gui"`) {
		t.Error("the window package makes no .gui file, so typing tfg-gui blocks the terminal " +
			"until the window closes")
	}
	if anyLine(scriptLines(files[cliChoco+"tools/chocolateyinstall.ps1"]), `\.gui`) {
		t.Error("the command line package makes a .gui file, so its shim would return before " +
			"the program finished and a script would read no exit code")
	}
	if !anyLine(install, `^\s*-WorkingDirectory '%USERPROFILE%' `) {
		t.Error("the Start menu shortcut does not start in the user's profile, so the window " +
			"offers to write into a folder the person cannot write to")
	}
	uninstall := scriptLines(files[windowChoco+"tools/chocolateyuninstall.ps1"])
	if !anyLine(uninstall, `^if \(\$target\.StartsWith\(\$toolsDir \+ '\\', `) {
		t.Error("the uninstall removes the Start menu shortcut without asking whether it points " +
			"into this package, so it can take a shortcut that is somebody else's")
	}
}

// No package script ends the program. Owner's decision of 2026-09-25: a run in
// progress may be halfway through a set of files, and cutting it leaves files
// with no manifest to say what they are.
func TestNoPackageScriptEndsTheProgram(t *testing.T) {
	kill := regexp.MustCompile(`(?i)\b(stop-process|taskkill|kill)\b|\.Kill\(`)
	checked := 0
	for name, text := range packagingTemplates(t) {
		if !strings.HasSuffix(name, ".ps1.in") {
			continue
		}
		checked++
		for _, line := range scriptLines(text) {
			if kill.MatchString(line) {
				t.Errorf("%s ends a process: %s", name, strings.TrimSpace(line))
			}
		}
	}
	if checked < 4 {
		t.Errorf("read %d package script(s), and there are four", checked)
	}
}

// Every package script is ASCII. Chocolatey runs them with Windows PowerShell
// 5.1, which reads a file without a byte order mark in the machine's ANSI code
// page, so anything else arrives changed.
func TestEveryPackageScriptIsASCII(t *testing.T) {
	for name, text := range renderedPackages(t) {
		if !strings.HasSuffix(name, ".ps1") {
			continue
		}
		for number, line := range strings.Split(text, "\n") {
			for _, r := range line {
				if r > 127 {
					t.Errorf("%s line %d is not ASCII: %q", name, number+1, line)
					break
				}
			}
		}
	}
}

// What a person reads in the feeds follows the punctuation rule (D17): a flat
// hyphen, and no semicolons. The guard over the program's own text reads Go
// files only, so this one reads the packages.
func TestThePackageTextFollowsThePunctuationRule(t *testing.T) {
	forbidden := []rune{';', rune(0x2013), rune(0x2014)}
	read := 0
	for name, text := range renderedPackages(t) {
		var shown []string
		switch {
		case strings.HasSuffix(name, ".locale.en-US.yaml"), strings.HasSuffix(name, ".nuspec"):
			shown = strings.Split(text, "\n")
		case strings.HasSuffix(name, ".ps1"):
			for _, line := range scriptLines(text) {
				if regexp.MustCompile(`^\s*Write-(Host|Warning) `).MatchString(line) {
					shown = append(shown, line)
				}
			}
		}
		read += len(shown)
		for _, line := range shown {
			if strings.ContainsAny(line, string(forbidden)) {
				t.Errorf("%s shows a person %q, which breaks the punctuation rule", name, strings.TrimSpace(line))
			}
		}
	}
	if read == 0 {
		t.Error("read no line a person sees - this guard is reading nothing")
	}
}

// The two values the renderer holds a copy of agree with the Go originals.
func TestThePackagesNameTheProductAndLicenceTheProgramDoes(t *testing.T) {
	script := readRepoFile(t, ".github/scripts/build_packages.py")
	for _, pair := range []struct{ what, ours, theirs, file string }{
		{"the product name", `(?m)^APP_NAME = "([^"]+)"$`, `(?m)^\s+Name:\s+"([^"]+)",$`, "internal/gui/run_cgo.go"},
		{"the licence", `(?m)^LICENCE = "([^"]+)"$`, `(?m)^const ourLicence = "([^"]+)"$`, "internal/legal/spdx.go"},
	} {
		ours := regexp.MustCompile(pair.ours).FindStringSubmatch(script)
		theirs := regexp.MustCompile(pair.theirs).FindStringSubmatch(readRepoFile(t, pair.file))
		if ours == nil || theirs == nil {
			t.Errorf("could not read %s from the renderer and %s", pair.what, pair.file)
			continue
		}
		if ours[1] != theirs[1] {
			t.Errorf("the packages give %s as %q and %s as %q", pair.what, ours[1], pair.file, theirs[1])
		}
	}
}

// The package sources are in git. Chocolatey's moderation asks packageSourceUrl
// to point at them, the job in ci.yml renders them on a fresh clone, and the
// point of a package source is that somebody else can see what the package
// does to their machine. A missing file shows up on somebody else's clone.
func TestThePackageSourcesAreTrackedByGit(t *testing.T) {
	tracked := map[string]bool{}
	for _, name := range strings.Fields(gitOutput(t, "ls-files", "packaging", ".github/scripts/build_packages.py")) {
		tracked[name] = true
	}
	want := []string{".github/scripts/build_packages.py", "packaging/README.md"}
	for name := range packagingTemplates(t) {
		want = append(want, name)
	}
	for _, name := range want {
		if !tracked[name] {
			t.Errorf("%s is not tracked by git, so a fresh clone and the moderators both find nothing", name)
		}
	}
}
