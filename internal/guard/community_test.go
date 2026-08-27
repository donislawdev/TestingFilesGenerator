package guard

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// ourRepository is how a link to this project has to read.
const ourRepository = "donislawdev/TestingFilesGenerator"

// githubProject matches a link to a repository, and only a link.
//
// Anchored on https:// on purpose. The workflows are full of module paths that
// begin github.com/ and are not links to anything - golangci-lint, staticcheck,
// every indirect dependency - and a rule that matched those would have to name
// exceptions for them for ever.
var githubProject = regexp.MustCompile(`https://github\.com/([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)`)

// Nothing that tells a person where to go points at a different project.
//
// These files were brought over from a sibling repository of the same author,
// and the shape of that mistake is specific: a link that still names the other
// project reads perfectly, and the worst of them - the private reporting link in
// the issue chooser - would send somebody's vulnerability report to a repository
// that is not this one.
//
// CODE_OF_CONDUCT.md is not read here, and that is deliberate rather than an
// oversight. It is the Contributor Covenant carried verbatim, and its closing
// attribution links to github.com/mozilla/diversity - somebody else's text
// pointing at somebody else's repository, which is correct and must stay.
func TestNothingThatDirectsAPersonPointsAtAnotherProject(t *testing.T) {
	root := repoRoot(t)

	var files []string
	err := filepath.WalkDir(filepath.Join(root, ".github"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking .github: %v", err)
	}
	files = append(files, filepath.Join(root, "SECURITY.md"))

	checked := 0
	for _, p := range files {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("reading %s: %v", p, err)
			continue
		}
		for _, m := range githubProject.FindAllStringSubmatch(string(body), -1) {
			checked++
			if m[1] == ourRepository {
				continue
			}
			t.Errorf("%s links to https://github.com/%s.\n"+
				"This project is %s. A link left pointing at another repository is the "+
				"specific way these files go wrong, and in the issue chooser it would "+
				"send a vulnerability report to somebody else.",
				filepath.Base(p), m[1], ourRepository)
		}
	}

	// A walk that matched nothing would report everything is fine.
	if checked == 0 {
		t.Error("no repository link was found in .github or SECURITY.md at all, " +
			"so this guard checked nothing")
	}
}

// The issue forms are shaped the way GitHub needs them.
//
// A form GitHub cannot read is not reported anywhere a person would see. It
// simply stops offering the template, and the first anybody knows is that
// reports arrive without the version in them.
func TestTheIssueFormsAreShapedTheWayGitHubNeeds(t *testing.T) {
	dir := filepath.Join(repoRoot(t), ".github", "ISSUE_TEMPLATE")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the issue templates: %v", err)
	}

	allowed := map[string]bool{
		"markdown": true, "input": true, "textarea": true,
		"dropdown": true, "checkboxes": true,
	}

	forms := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}

		if name == "config.yml" {
			var cfg struct {
				Blank bool `yaml:"blank_issues_enabled"`
			}
			if err := yaml.Unmarshal(body, &cfg); err != nil {
				t.Errorf("%s does not parse: %v", name, err)
			}
			if cfg.Blank {
				t.Errorf("%s leaves blank issues enabled, so a report can arrive with "+
					"none of the things that make it answerable", name)
			}
			continue
		}

		var form struct {
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Labels      []string `yaml:"labels"`
			Body        []struct {
				Type       string `yaml:"type"`
				ID         string `yaml:"id"`
				Attributes struct {
					Label string `yaml:"label"`
					Value string `yaml:"value"`
				} `yaml:"attributes"`
			} `yaml:"body"`
		}
		if err := yaml.Unmarshal(body, &form); err != nil {
			t.Errorf("%s does not parse: %v", name, err)
			continue
		}
		forms++

		if form.Name == "" || form.Description == "" {
			t.Errorf("%s has no name or no description, which are what the chooser "+
				"shows a person deciding where to report", name)
		}
		if len(form.Body) == 0 {
			t.Errorf("%s asks for nothing", name)
		}
		for _, label := range form.Labels {
			if strings.TrimSpace(label) == "" {
				t.Errorf("%s applies an empty label", name)
			}
		}

		seen := map[string]bool{}
		for i, item := range form.Body {
			if !allowed[item.Type] {
				t.Errorf("%s field %d has type %q, which GitHub does not accept",
					name, i, item.Type)
				continue
			}
			if item.Type == "markdown" {
				if item.Attributes.Value == "" {
					t.Errorf("%s field %d is a markdown block with nothing in it", name, i)
				}
				continue
			}
			if item.Attributes.Label == "" {
				t.Errorf("%s field %d has no label", name, i)
			}
			if item.ID == "" {
				t.Errorf("%s field %d has no id", name, i)
				continue
			}
			if seen[item.ID] {
				t.Errorf("%s uses the id %q twice, and GitHub refuses the whole form for that",
					name, item.ID)
			}
			seen[item.ID] = true
		}
	}

	if forms == 0 {
		t.Error("no issue form was read, so this guard checked nothing")
	}
}

// The dependency review asks for no write scope, and posts no comment.
//
// Owner decision, 2026-08-27: a failing gate says what the comment would, so the
// comment is not worth a write scope. The two go together - the action needs
// `pull-requests: write` to post, so leaving the comment on while dropping the
// scope would be a setting that silently does nothing.
func TestTheDependencyReviewAsksForNoWriteScope(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "dependency-review.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the dependency review workflow: %v", err)
	}
	text := withoutYamlComments(string(body))

	if strings.Contains(text, "write") {
		t.Error("the dependency review workflow asks for a write scope.\n" +
			"It needs contents: read and nothing more. A write scope kept for a " +
			"convenience is one a job added later inherits without anybody deciding.")
	}
	if strings.Contains(text, "comment-summary-in-pr") {
		t.Error("the dependency review posts a summary comment again.\n" +
			"That needs pull-requests: write, which this workflow deliberately does " +
			"not have, so the setting would do nothing while looking like it does.")
	}
}
