package guard

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// pagesStep is the part of a workflow step this guard reads.
type pagesStep struct {
	ID              string `yaml:"id"`
	If              string `yaml:"if"`
	Uses            string `yaml:"uses"`
	ContinueOnError bool   `yaml:"continue-on-error"`
}

// pagesJob is the part of a workflow job this guard reads.
type pagesJob struct {
	Needs       any         `yaml:"needs"`
	Permissions any         `yaml:"permissions"`
	Steps       []pagesStep `yaml:"steps"`
}

// pagesDeployProblems is what is wrong with how a workflow uploads and deploys
// the site: nothing when the deployment runs in a job of its own, after the
// job that uploads, and asks for the artifact more than once, each try after
// the first only when the one before it failed, and the job that only uploads
// holds no write access to Pages.
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
	for name := range uploads {
		if _, deploys := deploys[name]; !deploys && writesPages(workflow.Jobs[name].Permissions) {
			problems = append(problems, "the job "+name+" only uploads the site and still holds pages: write or "+
				"id-token: write, which only the deploy job needs")
		}
	}
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
		for i, s := range steps[1:] {
			problems = append(problems, retryConditionProblems(name, steps[i], s)...)
		}
		if steps[len(steps)-1].ContinueOnError {
			problems = append(problems, "the last try of "+name+" may fail without failing the job, so a site that never went out would look published")
		}
	}
	return problems
}

// writesPages tells whether a job's permissions let it publish to Pages or
// ask for an OIDC token.
func writesPages(permissions any) bool {
	switch p := permissions.(type) {
	case string:
		return p == "write-all"
	case map[string]any:
		return p["pages"] == "write" || p["id-token"] == "write"
	}
	return false
}

// retryConditionProblems is what is wrong with the condition of a try that
// should run only when the try before it failed.
//
// It has to ask for the outcome. A step with continue-on-error that failed has
// the conclusion success, so a try that waited for the conclusion failure of
// the one before would never run, and the job would end green with nothing
// deployed.
func retryConditionProblems(job string, before, try pagesStep) []string {
	asks := "steps." + before.ID + ".outcome == 'failure'"
	var problems []string
	if before.ID == "" || !strings.Contains(try.If, asks) {
		problems = append(problems, "try "+try.ID+" of "+job+" does not wait for "+asks+
			", so it runs when it should not or never runs at all")
	}
	if strings.Contains(try.If, ".conclusion") {
		problems = append(problems, "try "+try.ID+" of "+job+" reads a conclusion, and a failed step with "+
			"continue-on-error concludes as success, so this try never runs")
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
// Two canaries: the job as it stood on main that day, uploading and deploying
// once in the same job, and a deploy job split off from an upload job that kept
// pages: write, with a retry that waits for the conclusion of the try before it
// rather than its outcome and so would never run. Every problem has to be
// reported.
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

	// The second canary: a retry that waits for the conclusion of the try
	// before it, which is success for a failed step with continue-on-error,
	// and an upload job that kept the write access only the deploy job needs.
	conclusion := []byte(`jobs:
  publish:
    permissions:
      contents: read
      pages: write
      id-token: write
    steps:
      - uses: actions/upload-pages-artifact@abc
  deploy:
    needs: publish
    steps:
      - id: first
        uses: actions/deploy-pages@abc
        continue-on-error: true
      - id: second
        if: steps.first.conclusion == 'failure'
        uses: actions/deploy-pages@abc
`)
	if got := pagesDeployProblems(conclusion); len(got) != 3 {
		t.Fatalf("the canary of a retry that reads the conclusion, after an upload job that holds pages: write, "+
			"gave %d problem(s) and has three: %v", len(got), got)
	}

	for _, p := range pagesDeployProblems([]byte(workflowText(t, "pages.yml"))) {
		t.Error(p)
	}
}
