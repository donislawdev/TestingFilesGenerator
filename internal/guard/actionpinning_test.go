package guard

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// Every action a workflow runs is named by commit, never by a tag.
//
// A tag is a pointer somebody else can move. `actions/checkout@v7` means "run
// whatever that account calls v7 at the moment the job starts", and the account
// is free to repoint it - which is the supply chain question this repository
// answers nowhere else. A release of this tool is built by these workflows, so
// an action that changed under us would sign and publish binaries nobody here
// reviewed.
//
// Measured 2026-08-27: a registry rule flagged 32 such uses across the three
// workflows, and it was the only finding in that scan carrying high confidence.
// All 32 now carry a commit.
//
// The tag is not lost - it moves into a comment beside the commit, so a reader
// can still tell v7.0.1 from v5.0.0 without resolving anything. Dependabot
// updates both halves together, which is why the comment is allowed to be the
// only place a version appears.
//
// This walks every file in the directory rather than a list of names, so a
// workflow added next month is covered without anybody remembering to come
// back here.
//
// An action of this repository - `uses: ./.github/actions/...` - has no tag
// and no commit, and needs neither: it is code in this tree, and it changes
// only through a commit that is reviewed here. It is not a door, though. A
// composite action runs steps with `uses:` of their own, so every action.yml
// under .github/actions is read by the same rule, and a local reference is let
// through only when this guard read the file it points at. Measured on #226,
// 2026-11-09: the first local action turned this guard red for both workflows
// that run it, while its own steps were read by nothing.
func TestEveryActionIsPinnedToACommitRatherThanATag(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workflows are not here: %v", err)
	}

	// Asking the predicate about inputs the tree does not currently contain.
	//
	// Without this the guard would be unfalsifiable: every action IS pinned
	// today, so a change that weakened the rule would find nothing to let
	// through and stay green. These cases keep a hold on the rule itself, and
	// they do not go away when the tree is clean.
	for _, c := range []struct {
		uses   string
		pinned bool
		why    string
	}{
		{"actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1", true, "a full commit"},
		{"actions/checkout@v7", false, "a moving major tag"},
		{"actions/checkout@v7.0.1", false, "an exact version is still a tag somebody can move"},
		{"actions/checkout@main", false, "a branch"},
		{"actions/checkout@3d3c42e", false, "an abbreviated commit, which is not unique for ever"},
		{"actions/checkout", false, "no ref at all means the default branch"},
		{"actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90bZ", false, "the right length but not hexadecimal"},
	} {
		if pinnedToACommit(c.uses) != c.pinned {
			t.Errorf("pinnedToACommit(%q) should be %v, because it is %s", c.uses, c.pinned, c.why)
		}
	}
	for _, c := range []struct {
		uses  string
		local bool
		why   string
	}{
		{"./.github/actions/release-archives", true, "an action kept in this repository"},
		{"./.github/workflows/build.yml", true, "a workflow of this repository called by another"},
		{".github/actions/release-archives", false, "a path without ./, which GitHub reads as OWNER/NAME"},
		{"actions/checkout@v7", false, "an action of another account"},
	} {
		if inThisRepository(c.uses) != c.local {
			t.Errorf("inThisRepository(%q) should be %v, because it is %s", c.uses, c.local, c.why)
		}
	}

	files := yamlFilesIn(entries, dir)
	files = append(files, actionFiles(t, filepath.Join(root, ".github", "actions"))...)

	// The files read, by their path from the root with forward slashes - the
	// way a local `uses:` names them.
	read := map[string]bool{}
	type reference struct{ where, uses string }
	var local []reference
	seen := 0
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatalf("%s is not under %s: %v", file, root, err)
		}
		rel = filepath.ToSlash(rel)
		read[rel] = true
		// Comments are removed first, because the version now lives in one and
		// "# v7.0.1" would otherwise read as a reference.
		for i, line := range strings.Split(withoutYamlComments(string(body)), "\n") {
			_, after, found := strings.Cut(strings.TrimSpace(line), "uses:")
			if !found {
				continue
			}
			uses := strings.TrimSpace(after)
			if uses == "" {
				continue
			}
			seen++
			if inThisRepository(uses) {
				local = append(local, reference{fmt.Sprintf("%s:%d", rel, i+1), uses})
				continue
			}
			if pinnedToACommit(uses) {
				continue
			}
			t.Errorf("%s:%d runs %s, which names a tag rather than a commit.\n"+
				"Whoever owns that action can repoint the tag, and this repository builds "+
				"its releases here. Resolve the tag to its commit and keep the version in a "+
				"comment beside it:\n"+
				"    uses: OWNER/NAME@<40 hex characters>  # v1.2.3",
				rel, i+1, uses)
		}
	}

	// A local reference is let through because what it points at is read
	// here too. One this walk did not read would be let through unread.
	for _, r := range local {
		target := path.Clean(strings.TrimPrefix(r.uses, "./"))
		if read[target] || read[path.Join(target, "action.yml")] || read[path.Join(target, "action.yaml")] {
			continue
		}
		t.Errorf("%s runs %s, an action of this repository, and this guard did not read it.\n"+
			"A local action is let through only because the steps it runs are held to the same rule. "+
			"Keep it as .github/actions/<name>/action.yml, where this guard reads it, or teach the "+
			"guard where it lives.", r.where, r.uses)
	}

	// A walk that matched nothing would report a clean tree, which is the way
	// this guard is most likely to break: a rename of the directory, a change
	// of suffix, a parser that stops seeing "uses:".
	if seen == 0 {
		t.Errorf("no action reference was found in %s, so this guard checked nothing.\n"+
			"Either the workflows moved or the way this reads them stopped working.", dir)
	}
}

// yamlFilesIn are the .yml and .yaml files directly in a directory, of the
// entries read from it. workflowFiles in buildfloor_test.go takes .yml alone.
func yamlFilesIn(entries []os.DirEntry, dir string) []string {
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files
}

// actionFiles are the action.yml files of the actions this repository keeps
// for its workflows, or none when it keeps none.
func actionFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir
			}
			return err
		}
		if !d.IsDir() && (d.Name() == "action.yml" || d.Name() == "action.yaml") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the actions in %s: %v", dir, err)
	}
	return files
}

// inThisRepository reports whether a `uses:` value names an action or a
// workflow of this repository, which GitHub knows by the leading "./".
func inThisRepository(uses string) bool {
	return strings.HasPrefix(uses, "./")
}

// pinnedToACommit reports whether a `uses:` value names a commit.
//
// Forty hexadecimal characters and nothing else. An abbreviated hash is refused
// on purpose: git shortens for people, and a prefix that is unique today can
// stop being unique in a repository that keeps growing.
func pinnedToACommit(uses string) bool {
	_, ref, found := strings.Cut(uses, "@")
	if !found || len(ref) != 40 {
		return false
	}
	for _, r := range ref {
		digit := r >= '0' && r <= '9'
		lower := r >= 'a' && r <= 'f'
		if !digit && !lower {
			return false
		}
	}
	return true
}
