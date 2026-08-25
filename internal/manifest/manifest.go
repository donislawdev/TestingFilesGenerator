package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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

	// Preset names the question this run was built to answer, when it came
	// from one. Absent for a run driven by a recipe file or by flags.
	Preset *Preset `json:"preset,omitempty"`
}

// Preset records which named question produced this run, and on what numbers.
//
// The id alone would not be enough, and the reason is the untouchable rule
// about a manifest not claiming certainty it does not have. Some parameters
// describe somebody else's system - the size limit of an upload form is theirs,
// not ours - and when one is left out we stand a number of our own in its
// place. A set built around a limit we invented carries expectations that read
// exactly like a set built around the real one.
//
// So Defaulted names the values nobody gave us. The run says the same thing out
// loud while it runs, and that sentence scrolls away. This is the part that
// stays with the files.
type Preset struct {
	ID string `json:"id"`
	// Parameters are the settled values, ours and theirs together, written the
	// way somebody would type them.
	Parameters map[string]string `json:"parameters,omitempty"`
	// Defaulted are the parameter names that were not given and stood in from
	// the declaration. Sorted, so two runs of one preset compare byte for byte.
	Defaulted []string `json:"defaulted,omitempty"`
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

	// Group is the class of case this file belongs to, from the recipe or from
	// the preset that produced it. Left out when nothing named one, rather
	// than written as an empty string - a consumer reading it can then tell
	// "no class" from "a class called nothing".
	Group string `json:"group,omitempty"`

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
		// An empty list rather than nothing. A run interrupted before its first
		// file finished used to render "files": null, while every other empty
		// collection beside it rendered as {} - so a reader looping over the
		// entries met a value it had no reason to expect. The document shows a
		// list and this is what makes that true at every size, including nought.
		Files: []File{},
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

// SchemaError is a manifest this build cannot read.
type SchemaError struct {
	Path   string
	Detail string
}

func (e *SchemaError) Error() string {
	return fmt.Sprintf("%s cannot be read as a manifest: %s", e.Path, e.Detail)
}

// MaxBytes is the largest manifest this build will read.
//
// A manifest arrives from outside the same way a recipe does - it travels with
// a fixture set, it turns up in a pull request, it is handed to "verify" by
// path - so its size is chosen by somebody else. The recipe has had a ceiling
// since 2026-08-02 for exactly that reason and the manifest had none, which was
// an asymmetry rather than a decision.
//
// Sixteen megabytes rather than the recipe's one. A manifest is generated
// rather than typed and it grows with the run: measured on 2026-08-03, an entry
// costs about 700 B, so this holds a run of roughly twenty thousand files -
// twice the largest preset in docs/PRESETS.md and well above the ten thousand
// this tool is designed around.
//
// What it does not do, stated as plainly as the recipe's: it does not defend
// against a manifest that fits and is still expensive.
const MaxBytes = 16 << 20

// TooLargeError is returned for a manifest past MaxBytes.
type TooLargeError struct {
	Path  string
	Bytes int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf(
		"%s is %d B and the limit is %d B. A manifest is read into memory to be compared against a directory, so an unbounded one is a way to exhaust it. Check that this is a manifest this tool wrote, or split the run it describes",
		e.Path, e.Bytes, MaxBytes)
}

// Load reads a manifest written by an earlier run.
//
// The major of manifest_version is checked before anything is believed. A
// manifest from a future major describes fields this build does not know, and
// acting on the half of it we recognise is how "verify" ends up calling a
// directory sound on the strength of the part it could read.
//
// One larger than MaxBytes is refused before a byte of it is read, because a
// manifest is held in memory to be compared against a directory and the file
// being read is not necessarily one of ours.
func Load(path string) (*Manifest, error) {
	// Asked on the directory entry rather than after reading. "Read it all,
	// then say it was too big" is not a limit - the cost was already paid.
	if info, statErr := os.Stat(path); statErr == nil && info.Size() > MaxBytes {
		return nil, &TooLargeError{Path: path, Bytes: info.Size()}
	}
	// And again on the bytes, because the entry is a look and not a limit. The
	// file can grow between the two, and a named pipe or a device reports a
	// size of zero and then hands over as much as it likes - a manifest is
	// described as arriving from outside, with somebody else's fixture set, so
	// that belongs to the model rather than to the imagination.
	//
	// One byte over the ceiling is read on purpose. Reading exactly MaxBytes
	// cannot tell a file of that size from a longer one cut off at it.
	raw, err := readAtMost(path, MaxBytes+1)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > MaxBytes {
		return nil, &TooLargeError{Path: path, Bytes: int64(len(raw))}
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, &SchemaError{Path: path, Detail: err.Error()}
	}
	if m.ManifestVersion == "" {
		return nil, &SchemaError{Path: path, Detail: "it carries no manifest_version, so it is not a manifest this tool wrote"}
	}
	got, want := major(m.ManifestVersion), major(Version)
	if got != want {
		return nil, &SchemaError{Path: path, Detail: fmt.Sprintf(
			"it is schema version %s and this build reads %s. Use the version of tfg that wrote it",
			m.ManifestVersion, Version)}
	}
	if err := checkPaths(path, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkPaths refuses a manifest whose entries point outside the directory they
// describe.
//
// This is the door. Every consumer of a manifest comes through Load, so the
// check sits here rather than beside each use - "verify" and "cleanup" then
// cannot drift into disagreeing about it, and a consumer written later gets it
// without knowing it exists.
//
// Measured on 2026-08-03, before this existed: an entry with the path
// "../VICTIM.txt" and cleanup --yes --force removed a file one directory above
// the output directory, reported "1 file(s) removed from <outdir>", and ended
// with exit code 0. Untouchable rule 7 says the list is the whole authority
// over what may be deleted - and nothing was asking whether the list stayed
// inside the directory it was handed.
//
// The whole document is refused rather than the one entry. A manifest is a
// record of one run, and an entry pointing somewhere it could never have
// written means the file was edited or produced by something else. Acting on
// the rest of it would be trusting the half we happen to like.
//
// Every entry is checked, not only the ones Claimed returns. An entry that no
// command reads today is one a command reads tomorrow.
func checkPaths(path string, m *Manifest) error {
	for i, f := range m.Files {
		problem := core.ContainmentProblem(f.Path)
		if problem == "" {
			continue
		}
		return &SchemaError{Path: path, Detail: fmt.Sprintf(
			"entry %d has the path %q, which lands outside the directory the manifest describes - %s. "+
				"This tool never reads or removes anything outside that directory, so a manifest that asks it to is one it will not act on. "+
				"Use the manifest the run actually wrote, or correct the path to one inside the directory",
			i+1, f.Path, problem)}
	}
	return nil
}

// major is the part before the first dot. A minor bump adds fields and stays
// readable, which is the whole point of splitting the two.
func major(v string) string {
	if i := strings.IndexByte(v, '.'); i >= 0 {
		return v[:i]
	}
	return v
}

// Save writes the manifest, claiming the name before it writes and never
// writing over a manifest that is already there.
//
// Two failures shaped this, and neither is hypothetical.
//
// The first is a second run into the same directory. The engine refuses that in
// its preflight, but a check followed by a write is not the same as claiming
// the name: two runs started together both found the directory empty, both
// wrote, and one manifest replaced the other. Measured on 2026-08-03 with two
// runs of eight files under different ids - both ended with exit code 0,
// sixteen files were on the disk, one manifest described eight of them, and the
// other eight could never be removed by this tool again. O_EXCL closes that,
// because creating the name and finding out whether it existed become one
// operation the operating system settles.
//
// The second is the process ending part way through the write. Every generated
// file goes through a temporary name and a rename for exactly this reason,
// while the manifest - the one file that can remove all the others - was
// written in place. A truncated manifest does not parse, so the record of a
// finished run would be lost to a Ctrl+C landing in the wrong second.
//
// So the name is claimed first, the content is written beside it, and the
// rename puts it in place in one step.
// Claim takes the manifest name before a run writes anything.
//
// Claiming at save time was already better than checking in advance - two runs
// could no longer both write - but it happened after the last file, so the
// second run wrote its whole set and only then found out it had nowhere to
// record them. Measured on 2026-08-03: two runs started together under
// different ids ended 0 and 5, with sixteen files on the disk and eight of them
// in nobody's manifest. That turned a silent loss into a loud partial run,
// which was the improvement, and this is the rest of it.
//
// The claim is an empty file under the final name. Save renames over it, so the
// window between them belongs to this run and nobody else can take the name.
func Claim(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return claimName(path)
}

// Release gives the name back, for a run that claimed it and then could not
// start. Without it a refused run would leave an empty manifest behind and the
// next run into that directory would be refused for a file nobody wrote.
func Release(path string) error {
	if info, err := os.Stat(path); err != nil || info.Size() != 0 {
		// Somebody filled it in, so it is not ours to remove.
		return nil
	}
	return os.Remove(path)
}

func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// A run that got this far claimed the name before it wrote a byte, and the
	// claim is an empty file. Anything with content in it is somebody's
	// manifest and is never written over - that is the whole point of the
	// claim, and it is why "it exists" is not enough to go on here.
	switch info, err := os.Stat(path); {
	case errors.Is(err, fs.ErrNotExist):
		// Nothing there. A caller that writes a manifest without claiming
		// first - the guards do - claims it now.
		if err := claimName(path); err != nil {
			return err
		}
	case err != nil:
		// Something is there and it cannot be looked at - a permission, a
		// path whose parent is a file, a name the host will not take. Read as
		// "nothing there" until 2026-08-25, which sent the run on to claim a
		// name it had no answer about, and the claim then failed in words
		// about the wrong thing.
		return err
	case info.Size() != 0:
		return &os.PathError{Op: "save", Path: path, Err: fs.ErrExist}
	}

	tmp := path + ".tfg-writing"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := m.Encode(f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	// On the device before the rename, because the rename is what turns this
	// into the manifest and a rename can reach the disk before the bytes do.
	// What survives that is an empty file under the name of the only record
	// able to remove a run's files - the loss this whole function is shaped
	// against, reached by pulling the plug rather than by killing the process.
	//
	// One call per run, so the cost argument that keeps generated files
	// unsynced does not reach here. That one is written on engine.Run and it is
	// about ten thousand flushes, not one. docs/CODE-REVIEW-2026-08-23.md
	// section 3.4, owner's call on 2026-08-25.
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Over our own claim, which is why this rename is allowed to replace
	// something. Nobody else can be holding that name.
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// claimName creates the file only if nobody else holds the name.
//
// O_EXCL is the way to ask that question, because creating the file and finding
// out whether it existed are then one operation nobody can get between.
//
// It is not reliable everywhere, and that was measured rather than read.
// On Windows, Go opens with the flag that stops a create from following a link,
// and CREATE_NEW then reports ERROR_ALREADY_EXISTS for a file that is not there
// whenever any part of the path is a reparse point - a symbolic link or a
// junction. Measured on 2026-08-03 against a directory made three ways:
//
//	plain directory      O_EXCL succeeds
//	symbolic link        O_EXCL says "The file exists" and nothing is there
//	junction             the same
//
// It is Go rather than the system: the same create through the same link
// succeeds from .NET. A directory reached through a link is an ordinary setup -
// a redirected workspace, a mounted scratch disk - so taking the answer at face
// value made "tfg generate --out" fail with exit code 5 for those users. That
// was introduced on 2026-08-03 and found the same day by the guard that
// generates into a linked directory.
//
// So a refusal is believed only when something really is there. Where O_EXCL
// works this is exactly O_EXCL. Where it lies, this falls back to what the tool
// did before, which leaves the same narrow window two runs starting together
// could meet - see O43.
func claimName(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err == nil {
		return f.Close()
	}
	if _, statErr := os.Stat(path); statErr == nil {
		// Something is genuinely there. This is the refusal that matters.
		return err
	}
	f, err = os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// readAtMost reads a file and stops at limit bytes.
//
// os.ReadFile has no ceiling: it asks the entry how big the file is, uses that
// as a starting size and then keeps reading until the end, whatever the entry
// said. For anything that is not an ordinary file, or an ordinary file somebody
// is still writing, that is unbounded.
func readAtMost(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}
