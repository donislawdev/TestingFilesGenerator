package guard

import (
	"bytes"
	"encoding/binary"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// macOS is signed on a second machine, and every guard here is about a failure
// that is quiet rather than loud.
//
// The measurements these stand on were taken on 2026-08-28, on a real Developer
// ID certificate and two real notarisations:
//
//   - a bare Mach-O binary CANNOT be stapled. stapler exits 66 and says it is
//     "incapable of working with Document files", and Gatekeeper then rejects
//     the binary even though it is signed AND notarised. So the bundle is not
//     packaging taste, it is the only shape a ticket attaches to;
//   - a .dmg staples and does not help - the binary inside a stapled dmg is
//     still rejected, because the ticket describes the container;
//   - a copy of the binary taken out of the bundle is rejected with "invalid
//     resource directory", so the link beside the bundle has to be a link;
//   - the ticket survives tar.gz, which is why the archive names on the site
//     did not have to change.
//
// Each of those is a way to ship something that looks signed and is refused on
// the first machine that downloads it.

func macSigningScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "sign_macos.sh"))
	if err != nil {
		t.Skipf("no macOS signing script here: %v", err)
	}
	return string(body)
}

func bundleScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "make_app_bundle.sh"))
	if err != nil {
		t.Skipf("no bundle script here: %v", err)
	}
	return string(body)
}

// The release has to BUILD a bundle, or there is nothing to staple.
//
// This is the guard the whole macOS half rests on. Without a bundle the signing
// script still runs, still signs, still notarises - and produces archives that
// Gatekeeper turns away, because a ticket has nowhere to go on a bare binary.
func TestTheReleaseWrapsTheMacBinariesInBundles(t *testing.T) {
	workflow := withoutYamlComments(workflowText(t, "release.yml"))

	if !strings.Contains(workflow, "make_app_bundle.sh") {
		t.Fatal("the release workflow never calls make_app_bundle.sh, so the macOS " +
			"archives carry bare binaries - which cannot be stapled at all, and are " +
			"rejected by Gatekeeper even after they are signed and notarised")
	}
	// Both of them. One command line binary and one window binary reach macOS,
	// and wrapping one of two would publish a rejected program beside a working
	// one - which is the shape of failure that is hardest to notice.
	calls := strings.Count(workflow, "make_app_bundle.sh")
	if calls < 2 {
		t.Errorf("make_app_bundle.sh is called %d time(s) and both the command line "+
			"binary and the window binary need it", calls)
	}
	for _, id := range []string{"com.donislawdev.tfg", "com.donislawdev.tfg-gui"} {
		if !strings.Contains(workflow, id) {
			t.Errorf("no bundle identifier %q in the release workflow", id)
		}
	}
}

// A link beside the bundle has to be a LINK.
//
// ln makes a copy without complaining on a filesystem that cannot do symlinks,
// and a copy here is worse than nothing: it looks like the program, it is the
// right size, and Gatekeeper refuses it. Measured by accident - a build on
// Windows produced exactly this, and the archive doubled in size.
func TestTheLinkBesideTheBundleCannotBeACopy(t *testing.T) {
	script := bundleScript(t)
	if !strings.Contains(script, "-L ") {
		t.Error("make_app_bundle.sh never checks that what it made is a symlink, " +
			"so a filesystem without symlinks turns it into a copy of the binary - " +
			"and a copy outside the bundle is rejected by Gatekeeper")
	}
	if !strings.Contains(script, "ln -s") {
		t.Error("make_app_bundle.sh does not put a link beside the bundle, so a person " +
			"has to reach inside a .app to run a command line tool")
	}
}

// The bundle carries an icon, and it refuses to be built without one.
//
// O154, reported by the owner on 2026-08-28: a bundle with no CFBundleIconFile
// and nothing in Resources is drawn by the Finder and the Dock as a blank sheet
// of paper, which is what a program the system knows nothing about looks like.
// It was not a regression - until that day macOS got a bare binary and had no
// icon either - and it became visible the moment the program became a .app.
//
// Refusing rather than skipping, because the failure is silent on both ends: a
// bundle without an icon builds, signs, notarises and staples exactly like one
// with an icon, and nothing before a person's screen would say a word.
func TestTheMacBundleCarriesOurIcon(t *testing.T) {
	script := bundleScript(t)

	if !strings.Contains(script, "chickpea.icns") {
		t.Error("make_app_bundle.sh never copies an icon into the bundle, so macOS draws " +
			"the program as a blank page in the Finder and the Dock")
	}
	if !strings.Contains(script, "Contents/Resources/icon.icns") {
		t.Error("make_app_bundle.sh puts no icon.icns in Contents/Resources, which is " +
			"where CFBundleIconFile is looked up")
	}
	if !strings.Contains(script, "CFBundleIconFile") {
		t.Error("the Info.plist this script writes names no icon file, so an icon sitting " +
			"in Resources is never read")
	}
	// The refusal, and it is the half worth guarding. An icon that quietly is
	// not there produces a bundle that is wrong in the one way nothing later in
	// the release can see.
	if !strings.Contains(script, "no icon at") {
		t.Error("make_app_bundle.sh does not stop when the icon is missing, so a build " +
			"with no icon file produces a bundle that looks finished and shows a blank page")
	}
}

// And the icon it copies really carries every size macOS asks for.
//
// Read out of the bytes rather than trusted, because this file is written by a
// script that is not run in CI and cannot be regenerated here - Pillow is a
// build tool on one machine. So the committed file is the artefact, and what
// nobody would notice is a file that parses, opens in a viewer, and is missing
// the two sizes a screen without Retina asks for.
//
// Apple's own iconutil was asked whether it accepts what tools/appicon.py
// writes, on a real Mac on 2026-08-28, and handed back all ten entries at the
// pixel sizes below. This guard is the part of that answer that can be asked
// again on any machine, on every run.
func TestTheIconMacOSReadsCarriesEverySizeItIsAskedFor(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "gui", "icon", "chickpea.icns"))
	if err != nil {
		t.Skipf("no macOS icon here: %v", err)
	}
	if len(body) < 8 || string(body[:4]) != "icns" {
		t.Fatalf("the icon does not start with the four bytes that say what it is, so no "+
			"reader will take it: %q", body[:min(8, len(body))])
	}
	if declared := binary.BigEndian.Uint32(body[4:8]); int(declared) != len(body) {
		t.Errorf("the icon says it is %d bytes and it is %d, and a reader that trusts the "+
			"header stops early or runs off the end", declared, len(body))
	}
	// Apple's names for the sizes, with the pixels each one has to hold. Both
	// members of a pair are the same picture: macOS asks for a point size and a
	// scale, so 32 px answers two questions and has to be in the file twice.
	wanted := map[string]int{
		"icp4": 16, "ic11": 32, "icp5": 32, "ic12": 64, "ic07": 128,
		"ic13": 256, "ic08": 256, "ic14": 512, "ic09": 512, "ic10": 1024,
	}
	found := map[string]int{}
	for at := 8; at < len(body); {
		if at+8 > len(body) {
			t.Fatalf("a chunk header runs past the end of the file at byte %d", at)
		}
		name := string(body[at : at+4])
		size := int(binary.BigEndian.Uint32(body[at+4 : at+8]))
		if size < 8 || at+size > len(body) {
			t.Fatalf("the %q chunk says it is %d bytes, which does not fit in the file", name, size)
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(body[at+8 : at+size]))
		if err != nil {
			t.Fatalf("the %q chunk does not hold a picture: %v", name, err)
		}
		if format != "png" {
			t.Errorf("the %q chunk holds a %s and a bundle icon is read as PNG", name, format)
		}
		if config.Width != config.Height {
			t.Errorf("the %q chunk is %dx%d and an icon is square", name, config.Width, config.Height)
		}
		found[name] = config.Width
		at += size
	}
	for name, pixels := range wanted {
		got, is := found[name]
		if !is {
			t.Errorf("the icon has no %q entry, so macOS falls back to scaling another size "+
				"where it wanted %d px", name, pixels)
			continue
		}
		if got != pixels {
			t.Errorf("the %q entry is %d px and macOS reads it as %d", name, got, pixels)
		}
	}
	t.Logf("%d entries, every size macOS asks for, %d bytes", len(found), len(body))
}

// The macOS signing script never changes directory.
//
// Found by the first real release, on 2026-08-28, and it stopped that release
// twice in two consecutive steps. Every path this script works with is derived
// from the directory it is handed, and sign_release.py hands it one relative to
// the home directory. Two commands ran inside a subshell that had cd'd
// somewhere else first, so the paths they were given stopped meaning anything:
//
//   - certificate_of cd'd into a temporary directory and then asked codesign
//     about the bundle. codesign answered "No such file or directory", no
//     certificate came out, and the script refused the release saying the
//     bundle was "signed by a DIFFERENT certificate, got none" - about a bundle
//     that was correctly signed by the pinned certificate, with a timestamp,
//     chaining to the Apple Root CA. Measured after the fact: the same command
//     with the path resolved gives exactly the pinned digest;
//   - the zip for notarisation cd'd into the unpacked bundle and wrote to a
//     path relative to where the script started, so the file was never created.
//     That one had not been reached yet and would have stopped the next step.
//
// Asked as "no cd at all" rather than "the paths are absolute", because that is
// the property that can be read off the file. If a directory change is ever
// genuinely needed here, make every path absolute first and this guard is the
// conversation about it.
//
// Why nothing caught this before: the rehearsal of 2026-08-28 ran the script
// by hand with an absolute directory, and sign_release.py passes a relative one.
// The command was proven, the call was not.
func TestTheMacSigningScriptDoesNotDependOnWhereItIsRunFrom(t *testing.T) {
	script := macSigningScript(t)

	if strings.Contains(script, "cd \"") {
		t.Error("sign_macos.sh changes directory somewhere. Every path in it comes from the " +
			"directory it is handed, and sign_release.py hands it a relative one - so a cd " +
			"silently changes what those paths mean.\n" +
			"What happened when this was last true: codesign reported no certificate for a " +
			"correctly signed bundle, and the release stopped saying the wrong thing about why.")
	}
}

// The pin is a digest with a date, and the script derives its selector from it.
//
// Same shape as the Windows pin and for the same reason: codesign selects by
// SHA-1, SHA-1 is not worth pinning anything to, and two written copies of one
// certificate identity is exactly the drift a pin exists to catch.
func TestThePinnedAppleCertificateIsADigestWithADate(t *testing.T) {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(legal.AppleDeveloperIDSHA256) {
		t.Errorf("the pinned Apple certificate is %q, which is not a sha256",
			legal.AppleDeveloperIDSHA256)
	}
	expires, err := time.Parse("2006-01-02", legal.AppleDeveloperIDExpires)
	if err != nil {
		t.Fatalf("the Apple certificate expiry %q is not a date: %v",
			legal.AppleDeveloperIDExpires, err)
	}
	t.Logf("the pinned Apple certificate expires %s", expires.Format("2006-01-02"))

	if !strings.Contains(signingScript(t), "AppleDeveloperIDSHA256") {
		t.Error("the signing script does not read the Apple pin from internal/legal, " +
			"so there are two written copies of one digest")
	}
}

// The script refuses before it signs, and refuses after it signs.
//
// Before: no profile, and a certificate that is not the pinned one. After:
// notarisation that did not come back Accepted, a staple that did not take, and
// a bundle Gatekeeper still rejects. The last three matter because every one of
// them leaves a file that looks finished.
func TestTheMacSigningScriptRefusesOnBothSidesOfSigning(t *testing.T) {
	script := macSigningScript(t)

	for _, want := range []struct{ needle, why string }{
		{"store-credentials",
			"a missing notarytool profile has to say how to create one"},
		{"hashes to the pin",
			"a certificate that is not the pinned one has to stop the run"},
		{"status: Accepted",
			"notarisation that came back anything else has to stop the run"},
		{"stapler validate",
			"a staple that did not take looks exactly like one that did"},
		{"spctl",
			"the last question is whether Gatekeeper actually accepts it, and " +
				"without asking, the script hands back an archive that fails on " +
				"the first machine that downloads it"},
	} {
		if !strings.Contains(script, want.needle) {
			t.Errorf("sign_macos.sh does not mention %q: %s", want.needle, want.why)
		}
	}

	// The hardened runtime, without which Apple refuses to notarise, and the
	// timestamp, without which the signature dies with the certificate.
	if !strings.Contains(script, "--options runtime") {
		t.Error("sign_macos.sh does not sign with the hardened runtime, and Apple " +
			"refuses to notarise anything signed without it")
	}
	if !strings.Contains(script, "--timestamp") {
		t.Error("sign_macos.sh signs without a timestamp, so those signatures die " +
			"when the certificate expires instead of outliving it")
	}
}

// A release is signed on two machines or it is not published.
//
// Stopping after the Windows half would leave three signed archives and two that
// Gatekeeper turns away, on one page, looking alike. So the absence of a Mac is
// a refusal at the very start rather than a warning in the middle.
func TestTheReleaseWillNotBeSignedOnOneMachineOnly(t *testing.T) {
	script := signingScript(t)

	// The call, not the name. "sign_macos" appears in a def and in prose, so
	// asking whether the name is anywhere would pass a script that defines the
	// step and never runs it.
	if !strings.Contains(script, "sign_macos(args.tag") {
		t.Error("sign_release.py never calls the macOS signing step, so those two " +
			"archives go out unsigned beside signed Windows ones")
	}
	// And the refusal, in its own words. Without it the script would sign the
	// Windows half and publish the macOS half unsigned, which is the one
	// outcome a release page cannot explain.
	if !strings.Contains(script, "no Mac to sign the macOS archives on") {
		t.Error("sign_release.py does not refuse when there is no Mac, so it would " +
			"publish three signed archives and two Gatekeeper turns away")
	}
	// The address of the Mac has to come from outside the file. Not checked
	// here that no address is written down anywhere - privatecontent_test.go
	// already asks that of every tracked file AND of the git history, and asks
	// it better: it names the private ranges instead of matching four numbers
	// with dots, which is why it does not mistake the certificate OID
	// 1.3.6.1.5.5.7.3.3 for an address. This one did, before it was deleted.
	if !strings.Contains(script, "TFG_MACOS_HOST") {
		t.Error("sign_release.py offers no way to name the Mac from outside the file, " +
			"so the address would have to be written into a public repository")
	}
}

// The release notes cannot go on saying macOS is unsigned.
//
// This is the same class as the guard over the site: prose nobody checks goes
// stale the moment the thing it describes changes, and a note telling people to
// right click their way past Gatekeeper is worse than useless once the binary
// opens normally.
func TestTheReleaseNotesDoNotCallTheMacBinariesUnsigned(t *testing.T) {
	workflow := workflowText(t, "release.yml")
	notes := workflow
	if at := strings.Index(workflow, "## Signing"); at >= 0 {
		notes = workflow[at:]
	}

	if strings.Contains(notes, "macOS and Linux binaries are not signed") {
		t.Error("the release notes still say the macOS binaries are unsigned, and " +
			"they are signed, notarised and stapled")
	}
	if strings.Contains(notes, "macOS** refuses to open it on the first try") {
		t.Error("the release notes still tell people to right click past Gatekeeper, " +
			"which is advice for a binary this release no longer publishes")
	}
	// And the build's own statement stops describing the macOS archives the
	// moment a person signs them, because signing moves the bytes.
	if strings.Contains(notes, "Where a **macOS or Linux** archive came from") {
		t.Error("the release notes still offer the build's provenance for macOS " +
			"archives, and signing changed those bytes, so that check now fails " +
			"for anybody who runs it")
	}
}

// A script a workflow runs DIRECTLY has to be executable in the repository.
//
// Measured on 2026-08-28, on a real Unix filesystem: a file without the bit,
// invoked as ./script, exits 126 with "permission denied". Windows has no such
// bit and git bash ignores it, so a script can look perfectly fine here, pass
// every test here, and fail on the first runner that tries to run it.
//
// Found on this project's own work: make_app_bundle.sh went in as 100644 and
// would have stopped the FIRST release build, at the step that wraps the macOS
// binaries. Nothing else would have caught it - the guards read the script as
// text, and text does not have permissions.
func TestEveryScriptAWorkflowRunsDirectlyIsExecutable(t *testing.T) {
	root := repoRoot(t)
	entries, err := os.ReadDir(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Skipf("no workflows here: %v", err)
	}

	// Called directly means the line names the script with nothing in front of
	// it. A script handed to an interpreter - python x.py, bash x.sh - does not
	// need the bit, and demanding it there would be a rule this project does
	// not have.
	direct := regexp.MustCompile(`(?m)^\s*(\.github/scripts/[A-Za-z0-9_.-]+)`)
	wanted := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		body := withoutYamlComments(workflowText(t, entry.Name()))
		for _, m := range direct.FindAllStringSubmatch(body, -1) {
			wanted[m[1]] = true
		}
	}
	if len(wanted) == 0 {
		t.Skip("no workflow runs a script directly, so there is nothing to ask about")
	}

	// The mode git records, not the mode this filesystem reports. On Windows
	// the filesystem has no answer, and the repository is what the runner
	// clones.
	modes := map[string]string{}
	for _, line := range strings.Split(gitOutput(t, "ls-files", "-s", ".github/scripts"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			modes[fields[3]] = fields[0]
		}
	}

	for path := range wanted {
		mode, known := modes[path]
		if !known {
			t.Errorf("a workflow runs %s directly and git does not track it, so the "+
				"runner will not find it at all", path)
			continue
		}
		if mode != "100755" {
			t.Errorf("a workflow runs %s directly and git records it as %s, so the "+
				"runner gets permission denied and exits 126. It needs 100755 - "+
				"git update-index --chmod=+x %s", path, mode, path)
		}
	}
}

// The published release is checked on a Mac too, not only on Windows.
//
// Phase D exists to ask the questions a person downloading the release would
// ask, and until macOS was signed there was nothing to ask on that side. Now
// there is, and a checksum cannot answer it: it says the bytes did not move and
// says nothing about who signed them or whether a ticket is attached.
func TestThePublishedReleaseIsCheckedOnAMacAsWell(t *testing.T) {
	workflow := withoutYamlComments(workflowText(t, "verify-release.yml"))

	if !strings.Contains(workflow, "macos-latest") {
		t.Fatal("nothing in the release check runs on a Mac, so the macOS signatures " +
			"and tickets are never read by anything - and no other system can read them")
	}
	for _, want := range []struct{ needle, why string }{
		{"stapler validate",
			"the ticket has to be read out of the published file, and this is the " +
				"one question that does not depend on Gatekeeper being switched on"},
		{"AppleDeveloperIDSHA256",
			"the certificate that signed the published bundle has to be compared " +
				"with the pinned one, read from the source rather than copied"},
		{"assessments enabled",
			"a runner with Gatekeeper switched off answers accepted to everything, " +
				"so asking it without checking first is a test that cannot fail"},
	} {
		if !strings.Contains(workflow, want.needle) {
			t.Errorf("the release check never mentions %q: %s", want.needle, want.why)
		}
	}
}
