package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// pagesStep is the part of a workflow step this guard reads.
type pagesStep struct {
	ID              string `yaml:"id"`
	Uses            string `yaml:"uses"`
	ContinueOnError bool   `yaml:"continue-on-error"`
}

// pagesJob is the part of a workflow job this guard reads.
type pagesJob struct {
	Needs any         `yaml:"needs"`
	Steps []pagesStep `yaml:"steps"`
}

// pagesDeployProblems is what is wrong with how a workflow uploads and deploys
// the site: nothing when the deployment runs in a job of its own, after the
// job that uploads, and asks for the artifact more than once.
func pagesDeployProblems(body []byte) []string {
	var workflow struct {
		Jobs map[string]pagesJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		return []string{"the workflow does not parse: " + err.Error()}
	}
	uploads, deploys := map[string]bool{}, map[string][]pagesStep{}
	for name, job := range workflow.Jobs {
		for _, s := range job.Steps {
			switch {
			case strings.HasPrefix(s.Uses, "actions/upload-pages-artifact@"):
				uploads[name] = true
			case strings.HasPrefix(s.Uses, "actions/deploy-pages@"):
				deploys[name] = append(deploys[name], s)
			}
		}
	}
	if len(uploads) == 0 || len(deploys) == 0 {
		return []string{"no job uploads the site or none deploys it, so there is nothing here to check"}
	}
	var problems []string
	for name, steps := range deploys {
		if uploads[name] {
			problems = append(problems, "the job "+name+" uploads the site and deploys it, and in one job the "+
				"lookup by name misses the artifact uploaded a moment before")
		}
		needs := map[string]bool{}
		switch n := workflow.Jobs[name].Needs.(type) {
		case string:
			needs[n] = true
		case []any:
			for _, v := range n {
				if s, ok := v.(string); ok {
					needs[s] = true
				}
			}
		}
		for up := range uploads {
			if !needs[up] {
				problems = append(problems, "the job "+name+" does not wait for "+up+", which uploads what it deploys")
			}
		}
		if len(steps) < 2 {
			problems = append(problems, "the job "+name+" asks for the artifact once, and a lookup that misses it fails the whole run")
			continue
		}
		for _, s := range steps[:len(steps)-1] {
			if !s.ContinueOnError {
				problems = append(problems, "try "+s.ID+" of "+name+" ends the job when it fails, so the tries after it never run")
			}
		}
		if steps[len(steps)-1].ContinueOnError {
			problems = append(problems, "the last try of "+name+" may fail without failing the job, so a site that never went out would look published")
		}
	}
	return problems
}

// The site is deployed in a job of its own, after the upload, with more than
// one try.
//
// Measured on 2026-11-24: two runs in a row on main failed at deploy-pages
// with every file ready. It listed the sample-files artifact of the earlier
// job and not the github-pages one the same job had uploaded a moment before
// (actions/deploy-pages#451). Re-running the failed job cannot recover it,
// because the re-run uploads a second github-pages artifact.
//
// Its own canary: the job as it stood on main that day, uploading and
// deploying once in the same job, has to be reported.
func TestTheSiteIsDeployedInAJobAfterTheUploadWithMoreThanOneTry(t *testing.T) {
	canary := []byte(`jobs:
  publish:
    steps:
      - uses: actions/upload-pages-artifact@abc
      - id: deployment
        uses: actions/deploy-pages@abc
`)
	got := pagesDeployProblems(canary)
	if len(got) < 2 {
		t.Fatalf("the canary, one job that uploads and deploys once, gave %d problem(s) and has two: %v", len(got), got)
	}

	path := filepath.Join(repoRoot(t), ".github", "workflows", "pages.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the publishing workflow is not here: %v", err)
	}
	for _, p := range pagesDeployProblems(body) {
		t.Error(p)
	}
}
