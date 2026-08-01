// Package manifest defines the manifest schema and writes entries while a run
// is still going.
//
// Writing at the end would lose every entry at the moment the disk fills up,
// which is the most common failure of this particular tool.
package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Version is the schema version. A consumer checks it before parsing and
// refuses when the major does not match.
const Version = "1.0"

// The manifest is what turns a folder of files into a test suite. It is the
// one place where the file, the intent behind it and the expected reaction
// meet.
//
// It is not a test report. It is written before anything runs and it does not
// know what actually happened.

// Manifest is the header of a run.
type Manifest struct {
	ManifestVersion string `json:"manifest_version"`
	GeneratedAt     string `json:"generated_at"`

	Tool Tool `json:"tool"`
	Run  Run  `json:"run"`

	Summary Summary `json:"summary"`
	Files   []File  `json:"files"`
}

// Tool records what produced these bytes. Without it a hash mismatch after an
// upgrade cannot be diagnosed.
type Tool struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Generators map[string]string `json:"generators"`
}

// Run records the settings that shaped this particular run.
type Run struct {
	ID       string   `json:"id"`
	Seed     int64    `json:"seed"`
	Command  string   `json:"command"`
	Platform Platform `json:"platform"`
	// Complete is false when the run stopped early. A manifest is written
	// even then, otherwise cleanup has nothing to work with and the leftovers
	// stay for good.
	Complete bool `json:"complete"`

	// RecipeHash identifies the recipe that shaped this run, taken from its
	// canonical form so that reformatting a file does not look like a change
	// of content. Absent when the run came from flags alone.
	RecipeHash string `json:"recipe_hash,omitempty"`

	// Overrides are the values a flag took away from the recipe.
	//
	// Without them the recipe hash stops describing the run, because two runs
	// of the same recipe would produce different files and nothing would say
	// why.
	Overrides map[string]Override `json:"overrides,omitempty"`
}

// Override records one value that came from the recipe and was replaced on the
// command line.
type Override struct {
	FromRecipe any `json:"from_recipe"`
	FromFlag   any `json:"from_flag"`
}

// Platform is where the run happened.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Summary is the cheapest assertion in a test suite and the first thing a
// person checks by eye.
type Summary struct {
	FileCount    int            `json:"file_count"`
	TotalBytes   int64          `json:"total_bytes"`
	Materialized int            `json:"materialized"`
	ByFormat     map[string]int `json:"by_format"`
	ByExpected   map[string]int `json:"by_expected"`
}

// File is one entry.
type File struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
	// Materialized says whether the file is on disk. A name the host system
	// refuses is still a test case, and its entry still exists.
	Materialized bool `json:"materialized"`

	Bytes    int64  `json:"bytes"`
	Format   string `json:"format"`
	Fidelity string `json:"fidelity"`

	Hashes Hashes `json:"hashes"`

	Seed        string         `json:"seed"`
	Generator   GeneratorRef   `json:"generator"`
	Determinism string         `json:"determinism"`
	Properties  map[string]any `json:"properties,omitempty"`

	Expected Expected `json:"expected"`

	LabelEmbedded bool `json:"label_embedded"`

	// Notes are the things that must not be swallowed - a label that did not
	// fit, a fidelity level lowered on the fly, a file that failed.
	Notes []Note `json:"notes,omitempty"`
	// Failed marks an entry the run could not produce. The run carries on and
	// ends with the partial exit code, because nine thousand good files are
	// worth keeping.
	Failed bool   `json:"failed,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Hashes identify the bytes.
type Hashes struct {
	SHA256 string `json:"sha256"`
}

// GeneratorRef is which generator produced the file, and which version of it.
type GeneratorRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Note is a visible event attached to one file.
type Note struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// Expected is what the system under test should do with this file.
//
// It never pretends to a certainty it does not have. Where the right reaction
// depends on the policy of the application, the outcome is unspecified with
// an explanation - a made up expectation produces false failures and destroys
// trust in the whole suite.
type Expected struct {
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// Outcomes a manifest entry can declare.
const (
	OutcomeAccept      = "accept"
	OutcomeReject      = "reject"
	OutcomeSanitize    = "sanitize"
	OutcomeUnspecified = "unspecified"
)

// New starts a manifest for a run.
func New(toolName, toolVersion, runID, command string, seed int64, os, arch string) *Manifest {
	return &Manifest{
		ManifestVersion: Version,
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Tool: Tool{
			Name:       toolName,
			Version:    toolVersion,
			Generators: map[string]string{},
		},
		Run: Run{
			ID:       runID,
			Seed:     seed,
			Command:  command,
			Platform: Platform{OS: os, Arch: arch},
		},
		Summary: Summary{
			ByFormat:   map[string]int{},
			ByExpected: map[string]int{},
		},
	}
}

// Add records one file and keeps the summary in step. Adding and counting in
// one place is what stops the two from drifting apart.
func (m *Manifest) Add(f File) {
	m.Files = append(m.Files, f)
	m.Summary.FileCount++
	m.Summary.ByFormat[f.Format]++
	if f.Expected.Outcome != "" {
		m.Summary.ByExpected[f.Expected.Outcome]++
	}
	if f.Materialized {
		m.Summary.Materialized++
		m.Summary.TotalBytes += f.Bytes
	}
	if _, ok := m.Tool.Generators[f.Format]; !ok && f.Generator.Version != "" {
		m.Tool.Generators[f.Format] = f.Generator.Version
	}
}

// Notes gathers every note in the run, so a caller can report them without
// walking the entries itself.
func (m *Manifest) Notes() []string {
	var out []string
	for _, f := range m.Files {
		for _, n := range f.Notes {
			out = append(out, fmt.Sprintf("%s: %s", f.Name, n.Detail))
		}
	}
	sort.Strings(out)
	return out
}

// Encode renders the manifest as JSON.
func (m *Manifest) Encode(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// Save writes the manifest to a file, replacing whatever was there.
func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := m.Encode(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
