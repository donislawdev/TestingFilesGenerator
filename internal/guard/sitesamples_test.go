package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/site"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/tool/all"
)

// The sample files page offers files to download from the site itself. They
// are made by web/samples/samples.yaml when the site is published
// (.github/workflows/pages.yml) and are not in the repository. What the page
// says about each one - name, size, SHA-256 and what a system should do with
// it - is read here from the manifest of the same recipe, run in this
// process. The bytes are deterministic, so the two runs agree, and the
// workflow checks that they did against the SHA256SUMS the site renders.

// sampleSections are the parts of the page, in the order it shows them. A
// file of the recipe's own groups is listed under that group, and the files
// the tabular-import preset adds under tables.
var sampleSections = []string{"formats", "corrupt", "tables", "placeholders"}

// sampleFilesRecipe is where the recipe lives, and the path it is published at.
const sampleFilesRecipe = "web/samples/samples.yaml"

var sampleRun struct {
	once    sync.Once
	samples []site.Sample
	err     error
}

// samplesFromTheRecipe runs the recipe once per process, since several site
// guards render the page and the answer cannot change between them.
func samplesFromTheRecipe(t *testing.T) []site.Sample {
	t.Helper()
	recipe := filepath.Join(repoRoot(t), filepath.FromSlash(sampleFilesRecipe))
	sampleRun.once.Do(func() { sampleRun.samples, sampleRun.err = runSampleRecipe(recipe) })
	if sampleRun.err != nil {
		t.Fatal(sampleRun.err)
	}
	return sampleRun.samples
}

// runSampleRecipe makes the sample files in a folder that is removed after,
// and reads what the manifest says about each.
func runSampleRecipe(recipe string) ([]site.Sample, error) {
	dir, err := os.MkdirTemp("", "tfg-samples-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"generate", recipe, "--out", dir}, &out, &errOut); code != cli.ExitOK {
		return nil, fmt.Errorf("the sample recipe %s ended with code %d: %s", sampleFilesRecipe, code, errOut.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("the sample recipe wrote no manifest: %w", err)
	}
	var m struct {
		Files []struct {
			Name   string `json:"name"`
			Format string `json:"format"`
			Bytes  int64  `json:"bytes"`
			Group  string `json:"group"`
			Hashes struct {
				SHA256 string `json:"sha256"`
			} `json:"hashes"`
			Expected struct {
				Outcome string `json:"outcome"`
			} `json:"expected"`
			Damage []struct {
				Type string `json:"type"`
			} `json:"damage"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("reading the manifest of the sample recipe: %w", err)
	}
	samples := make([]site.Sample, 0, len(m.Files))
	for _, f := range m.Files {
		section := f.Group
		if !slices.Contains(sampleSections, section) {
			section = "tables"
		}
		var damages []string
		for _, d := range f.Damage {
			damages = append(damages, d.Type)
		}
		if len(f.Hashes.SHA256) != 64 {
			return nil, fmt.Errorf("the manifest gives %s no SHA-256, so the page has none to show", f.Name)
		}
		samples = append(samples, site.Sample{
			Section: section, Name: f.Name, Format: f.Format, Bytes: f.Bytes,
			SHA256: f.Hashes.SHA256, Outcome: f.Expected.Outcome, Damages: damages,
		})
	}
	return samples, nil
}

// toolsTheProgramHas is every tool in the order tfg tool list prints them.
func toolsTheProgramHas() []string {
	var out []string
	for _, d := range tool.All() {
		out = append(out, d.ID)
	}
	return out
}

// sampleGaps says what the page would be missing with these samples: a
// format with no file of its own, a section with nothing in it, and a file
// under corrupt that is not damaged.
func sampleGaps(samples []site.Sample, formats []string) []string {
	var gaps []string
	shown := map[string]int{}
	per := map[string]int{}
	for _, s := range samples {
		per[s.Section]++
		if s.Section == "formats" {
			shown[s.Format]++
		}
		if s.Section == "corrupt" && len(s.Damages) == 0 {
			gaps = append(gaps, s.Name+" is listed as corrupt and is not damaged")
		}
	}
	for _, id := range formats {
		if shown[id] != 1 {
			gaps = append(gaps, fmt.Sprintf("the format %s has %d file(s) of its own, and has to have one", id, shown[id]))
		}
	}
	for _, section := range sampleSections {
		if per[section] == 0 {
			gaps = append(gaps, "the section "+section+" has no file")
		}
	}
	return gaps
}

func TestTheSampleFilesPageOffersAFileOfEveryFormat(t *testing.T) {
	var formats []string
	for _, d := range format.All() {
		formats = append(formats, d.ID)
	}
	if len(formats) == 0 {
		t.Fatal("no format is registered, so this guard would pass having checked nothing")
	}
	// Canary: a set missing its first format, with a corrupt file nobody
	// damaged, has to be caught on both counts.
	planted := []site.Sample{{Section: "corrupt", Name: "x.pdf", Format: "pdf"}}
	for _, id := range formats[1:] {
		planted = append(planted, site.Sample{Section: "formats", Format: id})
	}
	if gaps := sampleGaps(planted, formats); len(gaps) < 4 {
		t.Fatalf("canary: a planted set missing %s and two sections, with an undamaged corrupt file, "+
			"gave only %v", formats[0], gaps)
	}

	for _, gap := range sampleGaps(samplesFromTheRecipe(t), formats) {
		t.Errorf("%s: %s.\nWhat to do: give %s a target for it, then rerun with TFG_WRITE_SITE=1.",
			sampleFilesRecipe, gap, sampleFilesRecipe)
	}

	// The files are made when the site is published, from the recipe, and
	// checked there against the sums on the page. A workflow that stopped
	// doing either would publish a page offering files that are not there.
	pages := withoutYamlComments(workflowText(t, "pages.yml"))
	for _, step := range []string{"generate " + sampleFilesRecipe, "tool checksum-check web/public/" + site.SamplesDir + "/SHA256SUMS"} {
		if !strings.Contains(pages, step) {
			t.Errorf("pages.yml no longer runs %q, so the sample files page offers files the published "+
				"site may not have, or not with the sums it shows", step)
		}
	}
}
