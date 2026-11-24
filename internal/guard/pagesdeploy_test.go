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
		Permissions any                 `yaml:"permissions"`
		Jobs        map[string]pagesJob `yaml:"jobs"`
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
		// A job without permissions of its own has the workflow's.
		held := workflow.Jobs[name].Permissions
		if held == nil {
			held = workflow.Permissions
		}
		if _, deploys := deploys[name]; !deploys && writesPages(held) {
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
		for i := 1; i < len(steps); i++ {
			problems = append(problems, retryConditionProblems(name, steps[:i], steps[i])...)
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
// should run exactly when every try before it failed.
//
// The whole condition is compared, not a part of it: one that also says
// "&& false" never runs, and one that also says "|| true" runs after a try
// that worked. It has to ask for the outcome. A step with continue-on-error
// that failed has the conclusion success, so a try that waited for the
// conclusion failure of the one before would never run, and the job would end
// green with nothing deployed.
func retryConditionProblems(job string, before []pagesStep, try pagesStep) []string {
	asks := make([]string, len(before))
	for i, b := range before {
		asks[i] = "steps." + b.ID + ".outcome == 'failure'"
	}
	want := strings.Join(asks, " && ")
	said := strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(try.If), "${{"), "}}")), " ")
	var problems []string
	if said != want {
		problems = append(problems, "try "+try.ID+" of "+job+" runs on "+said+" rather than exactly "+want+
			", so it runs when it should not or never runs at all")
	}
	if strings.Contains(try.If, ".conclusion") {
		problems = append(problems, "try "+try.ID+" of "+job+" reads a conclusion, and a failed step with "+
			"continue-on-error concludes as success, so this try never runs")
	}
	return problems
}

// pagesCanary is a workflow that uploads in one job and deploys twice in the
// next, with the permissions and the condition of the second try given.
func pagesCanary(workflowPermissions, uploadPermissions, retryIf string) []byte {
	return []byte(workflowPermissions + `jobs:
  publish:
` + uploadPermissions + `    steps:
      - uses: actions/upload-pages-artifact@abc
  deploy:
    needs: publish
    steps:
      - id: first
        uses: actions/deploy-pages@abc
        continue-on-error: true
      - id: second
        if: ` + retryIf + `
        uses: actions/deploy-pages@abc
`)
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
// Its canaries: the job as it stood on main that day, uploading and deploying
// once in the same job, retries whose condition is not exactly "every try
// before me failed", and an upload job holding pages: write, its own or the
// workflow's. Every problem has to be reported, and a correct layout none.
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

	// The other canaries: retries that read the conclusion or say more than
	// they should, and an upload job that holds the write access only the
	// deploy job needs, by its own permissions or by the workflow's. A correct
	// layout and an empty override of the workflow's permissions pass.
	const retry, write, wide = "steps.first.outcome == 'failure'", "    permissions:\n      pages: write\n      id-token: write\n", "permissions:\n  pages: write\n"
	for _, c := range []struct {
		why                  string
		workflow, upload, at string
		problems             int
	}{
		{"a correct layout", "", "", retry, 0},
		{"a retry reading the conclusion after an upload job with pages: write", "", write, "steps.first.conclusion == 'failure'", 3},
		{"a retry that never runs", "", "", retry + " && false", 1},
		{"a retry that runs after a try that worked", "", "", retry + " || true", 1},
		{"an upload job inheriting pages: write", wide, "", retry, 1},
		{"an upload job overriding the workflow with nothing", wide, "    permissions: {}\n", retry, 0},
	} {
		if got := pagesDeployProblems(pagesCanary(c.workflow, c.upload, c.at)); len(got) != c.problems {
			t.Fatalf("the canary of %s gave %d problem(s) and has %d: %v", c.why, len(got), c.problems, got)
		}
	}

	for _, p := range pagesDeployProblems([]byte(workflowText(t, "pages.yml"))) {
		t.Error(p)
	}
}
