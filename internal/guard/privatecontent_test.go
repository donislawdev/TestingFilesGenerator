package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Everything that goes through git is public and stays public. Not only the
// files: commit messages, the names of branches and tags, and every version of
// every file that ever existed. Deleting a file does not take it out of the
// history, and this project has its own case of that - the technical changelog
// left the repository and is still in every clone.
//
// Nothing watched any of it until 2026-08-04. The text guards read three files
// at the current commit, and the punctuation guard reads comments. A path from
// somebody's home directory in a commit message, or a private address in a
// branch name, would have gone out with the first push and stayed.
//
// Measured before this was written: 69 commits, 909 objects, 39040 lines of
// diff across the whole history, and not one hit. So this guard was armed
// green, which is the only time worth arming one - the alternative is a guard
// that starts red and gets an exception written for it.
//
// The one design rule here is easy to get wrong, and it is worth stating.
// This file is public too.
// A guard that watched for the owner's account name by writing that name down
// would publish the thing it protects. So every pattern below is structural -
// the shape of a home directory path, the shape of a private address - and
// never a literal taken from the machine this runs on.
var privatePatterns = []struct {
	name string
	rx   *regexp.Regexp
}{
	{"a path inside somebody's Windows profile", regexp.MustCompile(`(?i)[A-Za-z]:[\\/]{1,2}Users[\\/]`)},
	{"a path inside somebody's home directory", regexp.MustCompile(`/(?:home|Users)/([A-Za-z0-9._-]+)/`)},
	{"an address on somebody's private network", regexp.MustCompile(`\b(?:10\.(?:\d{1,3}\.){2}\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})\b`)},
	{"an e-mail address", regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)},
	// Built from pieces so this file does not contain the thing it looks for.
	// Written whole, the pattern would match its own source and the guard
	// would report itself.
	{"key material", regexp.MustCompile("-----" + "BEGIN ")},
	{"an AWS access key id", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"a GitHub token", regexp.MustCompile(`\bghp_[A-Za-z0-9]{36}\b`)},
	{"a Slack token", regexp.MustCompile(`\bxox[baprs]-`)},
}

// documentationNets are the addresses reserved for writing about networks,
// from RFC 5737. They are the right thing to put in an example and they are
// nobody's real network, so they are let past.
//
// This exists because the log generator draws its addresses from a range that
// overlaps the private ones. Nothing it produces is committed today, and if a
// fixture ever carries one the remedy is to use an address from here.
var documentationNets = regexp.MustCompile(`\b(?:192\.0\.2\.|198\.51\.100\.|203\.0\.113\.)`)

// homePlaceholders are the names an example uses when it means "your home
// directory". Refusing those would refuse ordinary help text.
var homePlaceholders = map[string]bool{
	"user": true, "you": true, "me": true, "name": true,
	"username": true, "someone": true, "example": true, "runner": true,
}

// privateFaults reports what in this text should not have been made public.
func privateFaults(text string) []string {
	var out []string
	for _, p := range privatePatterns {
		for _, m := range p.rx.FindAllStringSubmatch(text, -1) {
			switch {
			case p.name == "a path inside somebody's home directory" && homePlaceholders[strings.ToLower(m[1])]:
				continue
			case p.name == "an address on somebody's private network" && documentationNets.MatchString(m[0]):
				continue
			// A no-reply address names a service rather than a person, and
			// this project's own commit trailers carry one.
			case p.name == "an e-mail address" && strings.Contains(strings.ToLower(m[0]), "noreply"):
				continue
			// The address in Dependabot's own footer, published by GitHub on
			// every such commit in the world. It arrived on 2026-08-20 with the
			// first two update branches and it is the same class as the no-reply
			// above: a service, not a person, and not the owner's.
			//
			// This one had to be settled BEFORE those branches were merged. A
			// commit message cannot be edited once it is in the history without
			// rewriting it, so merging first would have left a guard that fails
			// on main for ever and a rule nobody could obey.
			case p.name == "an e-mail address" && strings.EqualFold(m[0], "support@github.com"):
				continue
			}
			out = append(out, p.name+": "+m[0])
		}
	}
	return out
}

// gitOutput asks git a question at the root of the repository. The root
// matters: run from the directory this test lives in, "git ls-files" answers
// about that directory alone and every path it returns is relative to it, so
// the scan quietly reads a handful of files and calls the repository clean.
func gitOutput(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git could not answer %v, so this cannot be read here: %v", args, err)
	}
	return string(out)
}

// The detector has to be able to find something, or every test below passes by
// looking at nothing. Each sample is built from pieces for the reason given at
// the top: written out whole they would be in this file, and this file is one
// of the things being scanned.
func TestTheScanForPrivateContentCanActuallyFindIt(t *testing.T) {
	sep := string(rune(92))
	sample := strings.Join([]string{
		"C:" + sep + "Users" + sep + "somebody" + sep + "notes",
		"/home/" + "bob/fixtures",
		// Split for the same reason as the rest, and this one was learned the
		// hard way: written whole they sat in this file as real matches and
		// the guard reported its own source. It catching its author is the
		// clearest evidence it works.
		"10.1." + "2.3",
		"192.168." + "0.4",
		"somebody" + "@" + "example.org",
		"-----" + "BEGIN " + "SOMETHING",
		"AKIA" + "0123456789ABCDEF",
		"ghp_" + strings.Repeat("a", 36),
		"xox" + "b-1",
	}, "\n")

	found := map[string]bool{}
	for _, f := range privateFaults(sample) {
		found[strings.SplitN(f, ":", 2)[0]] = true
	}
	for _, p := range privatePatterns {
		if !found[p.name] {
			t.Errorf("the scan does not find %s, so nothing below is checking for it", p.name)
		}
	}
}

// And it has to leave ordinary text alone, or the rule above is satisfied by
// refusing everything and the guard becomes noise somebody switches off.
func TestTheScanForPrivateContentLeavesOrdinaryTextAlone(t *testing.T) {
	sep := string(rune(92))
	benign := strings.Join([]string{
		"C:" + sep + "ProgramData" + sep + "chocolatey" + sep + "bin",
		"/home/" + "user/fixtures",
		"192.0.2.1",
		"203.0.113.9",
		"noreply" + "@" + "anthropic.com",
		"invoice.txt",
	}, "\n")

	if faults := privateFaults(benign); len(faults) > 0 {
		t.Errorf("ordinary text was called private: %v", faults)
	}
}

// The files as they stand. The text guards read three of them, this one reads
// every file git tracks.
// quotedLicencesRemoved drops the fenced blocks of the notices file.
//
// Those blocks are other people's licence texts, reproduced word for word
// because the licences require it - "the above copyright notice shall be
// included in all copies" is the obligation, not a style choice. One of them
// carries the address of its copyright holder, published by him in his own
// repository.
//
// This guard exists to stop the OWNER's private content reaching a public
// repository. A copyright line somebody else published, that we are obliged to
// carry, is neither ours nor private, and the alternative to this exception is
// deleting text a licence requires.
//
// Narrow on purpose, and in two ways. It applies to one file, and inside it
// only to fenced blocks - the prose we wrote in that same file is still read,
// so a path or a token typed into it is still caught. Widening this is the
// cheap way to put the guard to sleep.
func quotedLicencesRemoved(name, body string) string {
	if filepath.Base(name) != "THIRD-PARTY-NOTICES.md" {
		return body
	}
	var kept []string
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func TestNoTrackedFileCarriesPrivateContent(t *testing.T) {
	// Tracked files and files that are not tracked yet but are not ignored
	// either, read from the working tree rather than from the last commit.
	// The point of this guard is to answer before the commit, not after it -
	// once a path is in the history the only remedy is rewriting history that
	// other people have already pulled.
	root := repoRoot(t)
	files := strings.Fields(gitOutput(t, "ls-files", "--cached", "--others", "--exclude-standard"))
	if len(files) == 0 {
		t.Skip("git lists nothing here, so there is nothing to read")
	}
	checked := 0
	var faults []string
	for _, f := range files {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		checked++
		for _, fault := range privateFaults(quotedLicencesRemoved(f, string(body))) {
			faults = append(faults, f+": "+fault)
		}
	}
	if checked == 0 {
		t.Fatal("no file was read, so this guard would pass without checking anything")
	}
	if len(faults) > 0 {
		t.Errorf("%d place(s) carry something that should not be public:\n  %s",
			len(faults), strings.Join(faults, "\n  "))
	}
}

// The messages. These are as public as the files and nothing has ever read
// them - a path or an address written into one goes out with the push and
// cannot be taken back without rewriting history everybody has already pulled.
func TestNoCommitMessageCarriesPrivateContent(t *testing.T) {
	// Subject and body. The author line is deliberately left out: an address
	// there is how git identifies a person and is the author's own choice,
	// while one in the text of a message is something that slipped in.
	log := gitOutput(t, "log", "--all", "--format=%H%n%s%n%b")
	if strings.TrimSpace(log) == "" {
		t.Skip("no history here, so there are no messages to read")
	}
	// Proof that the messages themselves were read, not only the hashes. A
	// format asking for less would leave this guard green while scanning forty
	// character hex strings and finding nothing in them, which is the shape of
	// a test that passes without reaching what it is aimed at.
	if !strings.Contains(log, " ") {
		t.Fatal("the history was read without the subjects and bodies, so this guard would pass while looking at hashes")
	}
	if faults := privateFaults(log); len(faults) > 0 {
		t.Errorf("%d place(s) in the history carry something that should not be public:\n  %s",
			len(faults), strings.Join(faults, "\n  "))
	}
}

// And the names of branches and tags, which travel with a pull request and are
// the easiest of the three to write without thinking.
func TestNoBranchOrTagNameCarriesPrivateContent(t *testing.T) {
	refs := gitOutput(t, "for-each-ref", "--format=%(refname)")
	if strings.TrimSpace(refs) == "" {
		t.Skip("no refs here")
	}
	// The same proof. A format that answers with something other than names
	// would leave nothing for the scan to look at and nothing to say so.
	if !strings.Contains(refs, "refs/") {
		t.Fatal("what was read does not look like a list of ref names, so this guard is scanning the wrong thing")
	}
	if faults := privateFaults(refs); len(faults) > 0 {
		t.Errorf("a branch or tag name carries something that should not be public:\n  %v", faults)
	}
}
