package guard

import (
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
