package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/site"
)

// Every preset has a page of its own on the site since 2026-09-29, and what a
// page says about its preset is asked of the program here rather than written
// down beside it: the settings from the registry, what the set costs from
// tfg preset show, and how many files the system under test should take or
// turn away from a dry run of the set itself. The reasoning is in
// docs/PRESET-PAGES-2026-09-29.md.

// presetAnswers is asked once per test binary. Every site guard renders the
// site, and asking six presets for a dry run each time would repeat the same
// second of work for the same answer.
var presetAnswers struct {
	once  sync.Once
	facts []site.PresetFacts
	err   error
}

// presetFactsFromTheProgram is every registered preset as the site shows it.
func presetFactsFromTheProgram(t *testing.T) []site.PresetFacts {
	t.Helper()
	presetAnswers.once.Do(func() {
		presetAnswers.facts, presetAnswers.err = askEveryPreset()
	})
	if presetAnswers.err != nil {
		t.Fatalf("asking the program about its presets: %v", presetAnswers.err)
	}
	return presetAnswers.facts
}

func askEveryPreset() ([]site.PresetFacts, error) {
	// A dry run writes nothing, and it is still given a directory of its own
	// to not write into, removed afterwards, in case that ever changes.
	scratch, err := os.MkdirTemp("", "tfg-site-presets-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()

	out := make([]site.PresetFacts, 0, len(preset.All()))
	for _, p := range preset.All() {
		facts := site.PresetFacts{ID: p.ID}
		for _, param := range p.Parameters {
			facts.Settings = append(facts.Settings, site.Setting{
				Property:    siteProperty(param),
				Default:     param.Default,
				Placeholder: p.SaidWhenDefaulted[param.Name] != "",
			})
		}
		for _, name := range p.Reads {
			facts.Reads = append(facts.Reads, site.Read{Name: name, Default: p.ReadDefaults[name]})
		}
		if facts.Budget, err = budgetOf(p.ID); err != nil {
			return nil, err
		}
		if facts.Outcomes, err = outcomesOf(p.ID, filepath.Join(scratch, p.ID)); err != nil {
			return nil, err
		}
		out = append(out, facts)
	}
	return out, nil
}

// siteProperty is one setting flattened for the site, the same way for a
// format and for a preset - they are one type in the program, so the site
// describes them with one function.
func siteProperty(p format.Property) site.Property {
	return site.Property{
		Name:    p.Name,
		Kind:    string(p.Kind),
		Min:     p.Min,
		Max:     p.Max,
		Unit:    p.Unit,
		Choices: append([]string(nil), p.Choices...),
		Shape:   p.Shape,
	}
}

// runQuietly runs one command in this process and hands back what it printed.
func runQuietly(args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	if code := cli.Run(context.Background(), args, &stdout, &stderr); code != cli.ExitOK {
		return nil, fmt.Errorf("tfg %v ended with %d:\n%s", args, code, stderr.String())
	}
	return stdout.Bytes(), nil
}

// budgetOf is what tfg preset show says a preset costs at its defaults.
func budgetOf(id string) (site.Budget, error) {
	printed, err := runQuietly("preset", "show", id, "--json")
	if err != nil {
		return site.Budget{}, err
	}
	var shown struct {
		Budget *struct {
			Targets    int      `json:"targets"`
			Files      int      `json:"files"`
			TotalBytes int64    `json:"total_bytes"`
			Formats    []string `json:"formats"`
		} `json:"budget"`
	}
	if err := json.Unmarshal(printed, &shown); err != nil {
		return site.Budget{}, fmt.Errorf("reading what tfg preset show %s printed: %w", id, err)
	}
	// Refused rather than read as zero. A page announcing a set of no files
	// because a field was renamed is the quiet kind of wrong this file is for.
	if shown.Budget == nil || shown.Budget.Files == 0 {
		return site.Budget{}, fmt.Errorf("tfg preset show %s --json printed no budget, so its page would say the set is empty", id)
	}
	b := shown.Budget
	return site.Budget{Targets: b.Targets, Files: b.Files, Bytes: b.TotalBytes, Formats: b.Formats}, nil
}

// outcomesOf counts the reactions a dry run of the preset declares, by name.
func outcomesOf(id, dir string) ([]site.Outcome, error) {
	printed, err := runQuietly("generate", "--preset", id, "--dry-run", "--json", "--out", dir)
	if err != nil {
		return nil, err
	}
	var run struct {
		Files []struct {
			Expected struct {
				Outcome string `json:"outcome"`
			} `json:"expected"`
		} `json:"files"`
	}
	if err := json.Unmarshal(printed, &run); err != nil {
		return nil, fmt.Errorf("reading the dry run of %s: %w", id, err)
	}
	counts := map[string]int{}
	for _, f := range run.Files {
		counts[f.Expected.Outcome]++
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("a dry run of %s declared no files, so its page would have no reactions to show", id)
	}
	out := make([]site.Outcome, 0, len(counts))
	for name, n := range counts {
		out = append(out, site.Outcome{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// TestEveryPresetIsDescribedInEveryLanguage asks for the words of every
// preset page, in both directions.
//
// The render already stops on a word that is missing, so this is about the
// two things it cannot see. The English words are copies of the registry, and
// a copy nobody compares is the defect this whole site exists to prevent: the
// questions were copies for a month before this and nothing held them. And the
// number of sentences - a Polish page with one catch fewer renders without a
// complaint and says less than the English one.
func TestEveryPresetIsDescribedInEveryLanguage(t *testing.T) {
	langs := languagesOnDisk(t)
	sawEnglish := false
	for _, lang := range langs {
		for _, p := range preset.All() {
			text, ok := lang.Presets[p.ID]
			if !ok {
				t.Errorf("the preset %q has no words in %s", p.ID, lang.Code)
				continue
			}
			describesItsPreset(t, lang, p, text)
		}
		for id := range lang.Presets {
			if _, err := preset.Get(id); err != nil {
				t.Errorf("%s describes a preset %q that the program does not register", lang.Code, id)
			}
		}
		sawEnglish = sawEnglish || lang.Code == "en"
	}
	if !sawEnglish {
		t.Fatal("no English language file was read, so nothing was compared with the registry")
	}
}

func describesItsPreset(t *testing.T, lang site.Language, p preset.Preset, text site.PresetText) {
	t.Helper()
	if text.Question == "" || text.Title == "" || text.PageTitle == "" || text.Description == "" {
		t.Errorf("the preset %q is missing a question, title, page title or description in %s", p.ID, lang.Code)
	}
	if len(text.Catches) != len(p.Catches) {
		t.Errorf("the registry says %d things %q catches and %s says %d", len(p.Catches), p.ID, lang.Code, len(text.Catches))
	}
	declared := map[string]bool{}
	for _, param := range p.Parameters {
		declared[param.Name] = true
		if _, ok := text.Details[param.Name]; !ok {
			t.Errorf("--%s of %q has no sentence in %s", param.Name, p.ID, lang.Code)
		}
		if param.Shape != "" {
			said, ok := lang.Terms[param.Shape]
			if !ok {
				t.Errorf("--%s of %q takes %q and %s has no words for that", param.Name, p.ID, param.Shape, lang.Code)
			}
			if ok && lang.Code == "en" && said != param.Shape {
				t.Errorf("the registry says --%s of %q takes %q and the English page says %q", param.Name, p.ID, param.Shape, said)
			}
		}
	}
	for name := range text.Details {
		if !declared[name] {
			t.Errorf("%s describes a setting --%s that %q does not declare", lang.Code, name, p.ID)
		}
	}
	if lang.Code != "en" {
		return
	}
	if text.Question != p.Question || text.Title != p.Title {
		t.Errorf("the registry calls %q %q and asks %q, and the English page says %q and %q",
			p.ID, p.Title, p.Question, text.Title, text.Question)
	}
	if !slices.Equal(text.Catches, p.Catches) {
		t.Errorf("the English page lists what %q catches differently from the registry:\n  page:     %q\n  registry: %q",
			p.ID, text.Catches, p.Catches)
	}
	for _, param := range p.Parameters {
		if said := text.Details[param.Name]; said != param.Detail {
			t.Errorf("the registry describes --%s of %q as %q and the English page says %q", param.Name, p.ID, param.Detail, said)
		}
	}
}

// TestEveryReactionHasItsMeaningInEveryLanguage asks for the words of every
// outcome a manifest can declare, not only the ones today's presets happen to
// produce. A preset that starts declaring sanitize would otherwise stop the
// render on the day it lands, in a change about something else.
func TestEveryReactionHasItsMeaningInEveryLanguage(t *testing.T) {
	outcomes := []string{manifest.OutcomeAccept, manifest.OutcomeReject, manifest.OutcomeSanitize, manifest.OutcomeUnspecified}
	for _, lang := range languagesOnDisk(t) {
		for _, o := range outcomes {
			if lang.Outcomes[o] == "" {
				t.Errorf("the outcome %q has no meaning in %s", o, lang.Code)
			}
		}
		for o := range lang.Outcomes {
			if !slices.Contains(outcomes, o) {
				t.Errorf("%s explains an outcome %q that no manifest declares", lang.Code, o)
			}
		}
	}
}
