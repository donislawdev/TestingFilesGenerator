package guard

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The order of a release's downloads is decided by ONE thing, and it is not
// anything this project controls at publish time.
//
// Measured on 2026-08-28 against a real draft release, four hypotheses tested
// and three of them killed:
//
//   - upload order: no. Uploaded zzz, then mmm, then aaa, one at a time. The
//     list came back aaa, mmm, zzz.
//   - the asset id: no. The ids ran in upload order, the list did not.
//   - the asset label: no. A file named aaa carrying the label zzz still sorted
//     first, and a file named zzz labelled aaa sorted last.
//   - the file name, alphabetically, IGNORING letter case: yes. A file called
//     Zzz sorted after aaa, and probka_Windows sorted after probka_linux.
//
// So the only lever is the name, and that is what these guards are about. The
// four files a person checks a download WITH have to sort after the things they
// check, or they land in the middle of the programs - which is where they were:
// SHA256SUMS.txt sorted above every archive, and the three statement files fell
// between the window archives and the command line ones, because a dot sorts
// before an underscore.

// auxPrefix is what sign_release.py calls AUX_PREFIX, read from the script so
// there is one written copy rather than two that drift apart.
func auxPrefix(t *testing.T) string {
	t.Helper()
	found := regexp.MustCompile(`AUX_PREFIX = "([^"]+)"`).FindStringSubmatch(signingScript(t))
	if found == nil {
		t.Fatal("sign_release.py no longer names a prefix for the files a person " +
			"checks a download with, so nothing keeps them out of the middle of the list")
	}
	return found[1]
}

// The prefix has to sort AFTER the archive names, or it is decoration.
//
// This asks about the property rather than about the spelling: any prefix that
// sorts after both archive shapes passes, and the word "verify" is not required
// by anything here. What is required is that it works.
func TestTheFilesYouCheckADownloadWithSortToTheEnd(t *testing.T) {
	prefix := auxPrefix(t)

	// The two shapes an archive name takes today. Both are compared, because a
	// prefix that beats one and loses to the other would split the list in a way
	// that is harder to notice than the problem it replaced.
	for _, archive := range []string{
		"tfg_0.2.0_windows_amd64.zip",
		"tfg-gui_0.2.0_windows_amd64.zip",
	} {
		if !sortsAfter(prefix, archive) {
			t.Errorf("a file named %q sorts BEFORE %q, so it lands above or inside "+
				"the list of programs instead of under it", prefix+"whatever", archive)
		}
	}

	// And the whole list, sorted the way GitHub sorts it, has to put every
	// program above every check file. This is the question the guard above asks
	// in pieces, asked once in the shape a person actually sees.
	names := []string{
		"tfg-gui_0.2.0_windows_amd64.zip",
		"tfg-gui_0.2.0_linux_amd64.tar.gz",
		"tfg-gui_0.2.0_macos_arm64.tar.gz",
		"tfg_0.2.0_windows_amd64.zip",
		"tfg_0.2.0_linux_amd64.tar.gz",
		"tfg_0.2.0_macos_arm64.tar.gz",
		prefix + "SHA256SUMS.txt",
		prefix + "tfg_0.2.0.spdx.json",
		prefix + "tfg_0.2.0.provenance.sigstore.json",
		prefix + "tfg_0.2.0.sbom.sigstore.json",
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	seenCheckFile := false
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			seenCheckFile = true
			continue
		}
		if seenCheckFile {
			t.Errorf("sorted the way GitHub sorts them, %q comes after a file a person "+
				"checks downloads with, so the list is programs and checks interleaved: %v",
				name, names)
			break
		}
	}
}

// Every place that MAKES one of those four files has to use the prefix.
//
// Three different files create them - the build workflow makes the bill of
// materials, the signing script writes the checksums and renames the build's
// provenance, and the attestation workflow names the statement it publishes.
// A prefix applied in two places out of three splits the list rather than
// ordering it, and that is harder to spot than no prefix at all.
func TestEveryFileYouCheckADownloadWithCarriesThePrefix(t *testing.T) {
	prefix := auxPrefix(t)

	// Asked as a NEGATIVE, and the first version of this guard is the reason.
	// Asking whether the prefix appears anywhere in the file passed a mutation
	// that took it off the one name being created, because these files mention
	// the prefixed names elsewhere too. So this looks for the thing that must
	// not be there: a bill of materials or a statement named without it.
	bare := regexp.MustCompile(`["/]tfg_[^"'\s]*\.(spdx|sigstore)\.json`)

	for _, where := range []struct{ what, text string }{
		{"the build workflow, making the bill of materials",
			withoutYamlComments(workflowText(t, "release.yml"))},
		{"the signing script, writing the checksums and renaming the provenance",
			signingScript(t)},
		{"the attestation workflow, naming the statement it publishes",
			withoutYamlComments(workflowText(t, "attest-release.yml"))},
	} {
		if !strings.Contains(where.text, prefix) {
			t.Errorf("%s never uses the %q prefix, so the file it creates lands in the "+
				"middle of the download list", where.what, prefix)
		}
		if found := bare.FindString(where.text); found != "" {
			t.Errorf("%s names %s without the %q prefix, so that file sorts into the "+
				"middle of the programs instead of under them", where.what, found, prefix)
		}
	}

	// And the one that reads them back has to expect it, or it looks for a file
	// that is not there and calls the release incomplete.
	if !strings.Contains(withoutYamlComments(workflowText(t, "verify-release.yml")), prefix) {
		t.Error("the published release check does not expect the prefix, so it will " +
			"report the checksums file missing on a release that has it")
	}
}

// sortsAfter answers the question GitHub answers: which name comes later,
// ignoring letter case. Measured rather than assumed - see the note at the top.
func sortsAfter(later, earlier string) bool {
	return strings.ToLower(later) > strings.ToLower(earlier)
}
