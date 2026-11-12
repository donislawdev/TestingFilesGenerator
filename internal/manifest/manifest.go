package manifest

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime"
	"slices"
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

	// saved is how long this document was when Save wrote it, and nought
	// before that or for a document read back from the disk. Unexported, so it
	// is not written and not read back.
	saved int64
}

// Tool records what produced these bytes. Without it a hash mismatch after an
// upgrade cannot be diagnosed.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// Go is the toolchain that built the binary which wrote this, as
	// runtime.Version reports it - "go1.26.7".
	//
	// It is here for the reason the whole of Tool is here. D11 promises the
	// same bytes for the same recipe within a major version, and the compiler
	// is one of the things that promise rests on: this project pins a
	// toolchain in go.mod and in both CI workflows precisely because the
	// standard library's flate, gzip, zip and png outputs are part of the
	// contract. Without this field a hash that moved after somebody rebuilt
	// with a newer Go looks identical to a hash that moved because the recipe
	// changed, and the manifest - the one artefact that outlives the run -
	// cannot tell the two apart.
	//
	// Added on 2026-08-27, review item S1, with the owner's yes. It does NOT
	// move manifest_version, and that is the contract rather than a shortcut:
	// docs/MANIFEST.md section 10 says the schema grows by ADDING fields and
	// that a consumer has to ignore the ones it does not recognise. A reader
	// written against 1.0 is unaffected by a key it never looks at.
	Go         string            `json:"go"`
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

	// Instructions is the name of the file written beside this manifest that
	// says, in words, what every file of the run is and why it is there.
	// Absent when the run wrote none - no file had a purpose, or writing them
	// failed and the run said so. verify and cleanup read it, so the file is
	// never taken for something nobody asked for.
	Instructions string `json:"instructions,omitempty"`
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
	// Question is the one sentence the preset answers, in the words the
	// preset list gives it. Carried so that the instructions written beside
	// the manifest can open with it without knowing about presets.
	Question string `json:"question,omitempty"`
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
	// ByTarget counts the files each target produced, keyed by target id.
	// The cheapest form of the question O97 is about - a person checking by
	// eye, or a script asserting a shape, does not have to walk the entries.
	ByTarget map[string]int `json:"by_target"`
}

// File is one entry.
type File struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
	// SendAs is the name to hand the file to the system under test under,
	// when the recipe asked for one that is not the name on disk - a name
	// that climbs out of a directory, say, which no file system stores.
	//
	// Absent when nobody asked, so every manifest written before this
	// existed is byte for byte what it was.
	//
	// A field of its own rather than a different value in Name, and that is a
	// decision about other people's code. Name has always been the last part
	// of Path, so a script that joins a directory and Name lands on the file.
	// Handing that script "../x" in Name would put a path traversal into the
	// consumer of a tool that exists to test for them. Nothing here reads this
	// field to find a file: verify and cleanup go by Path alone.
	SendAs string `json:"send_as,omitempty"`
	// Materialized says whether the file is on disk. A name the host system
	// refuses is still a test case, and its entry still exists.
	Materialized bool `json:"materialized"`

	// TargetID is the id of the target this file came from, as the recipe
	// spells it - "invoices" rather than "f_0007".
	//
	// ID above answers a different question. It is the handle a test refers to
	// one file by (MF2) and it is numbered across the whole run, so two targets
	// in one recipe produce f_0001 to f_0004 with nothing saying where the
	// break falls. Neither does anything else: format is shared the moment two
	// targets ask for one format, group is optional and free text, and the
	// name only carries the target while nobody supplies a template of their
	// own - which is exactly when somebody would want to ask.
	//
	// So a consumer counting "how many files did which target produce" had to
	// parse file names, which is the work a manifest exists to remove.
	// Observation O97, added on 2026-08-27 with the owner's yes.
	//
	// Always written, never omitted, because a target without an id does not
	// reach here - the engine refuses an empty one and refuses a duplicate, so
	// this value identifies the target within the run.
	TargetID string `json:"target_id"`

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

	// Purpose is what this file is and why it is in the set, in words, from
	// the recipe or the preset that produced it - the sentence the
	// instructions beside this manifest are written from. Left out when
	// nothing gave one, so every manifest written before it existed keeps its
	// bytes, and it does not move manifest_version for the reason Damage gives.
	Purpose string `json:"purpose,omitempty"`

	LabelEmbedded bool `json:"label_embedded"`

	// Damage is what was deliberately broken about this file, in the order it
	// was applied.
	//
	// Left out entirely for a file nothing damaged, rather than written as an
	// empty list, so a consumer can tell "not damaged" from "damaged with
	// nothing" - and so every manifest written before this existed stays byte
	// for byte what it was.
	//
	// The order is part of it. Two damages applied the other way round are
	// different bytes, and D11 promises those bytes do not move, so a record
	// that lost the order would describe a file that cannot be rebuilt from it.
	//
	// Adding the field does not move manifest_version, the same way the
	// toolchain field did not: a reader of schema 1.0 that does not know this
	// key ignores it, and no key it does know has changed meaning.
	Damage []Damage `json:"damage,omitempty"`

	// Notes are the things that must not be swallowed - a label that did not
	// fit, a fidelity level lowered on the fly, a file that failed.
	Notes []Note `json:"notes,omitempty"`
	// Failure is set for an entry the run could not produce. Its two keys sit
	// in the entry itself, beside the others, as they always have.
	Failure
}

// Failure marks an entry the run could not produce, and says why. The run
// carries on and ends with the partial exit code, because nine thousand good
// files are worth keeping.
//
// A type of its own since the entry gained the name a file is sent under and
// went past what one type should hold. These two are the pair that is always
// set together and read together, and written inside File without a name they
// keep their place in the document: the same two keys, last in the entry.
type Failure struct {
	Failed bool   `json:"failed,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Damage is one thing broken about a file, with the settings it was given.
//
// The settings are recorded as they were resolved rather than as they were
// written, defaults included, because a consumer asking "what was done to this
// file" should not have to know what the default was in the build that wrote
// it.
type Damage struct {
	Type     string            `json:"type"`
	Settings map[string]string `json:"settings,omitempty"`
	// Cut is where a damage that cuts the end off the file fell. Left out for
	// every other damage, so the entries written before it existed keep their
	// bytes, and manifest_version stays for the reason File.Damage gives.
	Cut *Cut `json:"cut,omitempty"`
}

// Cut is where a cut fell in the file it was made in, and how much of the
// content it left.
//
// Asked because the cut does not always fall where the setting suggests. In a
// format that keeps its padding after the content, a large file is nearly all
// padding, so the cut falls there and the content stays whole whatever share
// was asked for (docs/TRUNCATE-2026-11-11.md). A test that means "half of the
// picture is gone" can read here whether it is.
type Cut struct {
	// In is what the first byte cut away was for: structure, content,
	// padding or end.
	In string `json:"in"`
	// ContentBytes is how long the content of the whole file was. Absent when
	// the generator did not know when it stopped, rather than a guess.
	ContentBytes *int64 `json:"content_bytes,omitempty"`
	// ContentKeptBytes is how much of the content is still in the file.
	ContentKeptBytes int64 `json:"content_kept_bytes"`
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
	// PolicyOptions are the answers a policy dependent outcome leaves open,
	// for a consumer that resolves it by its own policy (MF5).
	//
	// Absent from every file that has none, so each manifest written before
	// it existed is byte for byte what it was, and the schema stays 1.0 by
	// the rule MANIFEST.md section 10 states: it grows by adding.
	PolicyOptions []string `json:"policy_options,omitempty"`
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
			Name:    toolName,
			Version: toolVersion,
			// Read here rather than passed in, unlike the operating system and
			// the architecture below it. Those are injected so a guard can
			// describe a machine it is not running on. This one has exactly
			// one right answer at run time - the toolchain that built the
			// binary executing this line - and a caller able to pass a
			// different one could only ever pass a wrong one.
			Go:         runtime.Version(),
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
			ByTarget:   map[string]int{},
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
	m.count(f)
}

// count folds one file into the summary. Apart from Add so that a Projection
// counts a file exactly as Add does without keeping it - still one place that
// counts.
func (m *Manifest) count(f File) {
	m.Summary.FileCount++
	m.Summary.ByFormat[f.Format]++
	if f.TargetID != "" {
		m.Summary.ByTarget[f.TargetID]++
	}
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

// noteExamples is how many file names a grouped note shows before it stops
// listing and starts counting.
//
// Three rather than one, because one name reads as if one file were special
// when the note is about a whole target, and three shows the shape of the
// naming without becoming a list.
const noteExamples = 3

// Notes gathers every note in the run, so a caller can report them without
// walking the entries itself.
//
// GROUPED BY WHAT THE NOTE SAYS, not one line per file, and that is the whole
// point of it. Measured on 2026-09-06: a run of 25 000 one byte text files
// emitted 25 001 "note:" lines on stderr, one per file, every one of them the
// same sentence about the label not fitting. The advice was per file for a
// decision that is per target.
//
// The line that mattered was buried under them. A run whose manifest would
// be too big for this build to read back said so first, in a "note:" line
// typographically identical to the 25 000 that followed it - and that one was
// the only thing standing between somebody and a directory neither verify nor
// cleanup could read. That note went on 2026-11-04 with the ceiling it was
// about (read.go), and the grouping stays for every note that is left.
//
// The same reasoning was already applied to the progress bar, which is
// throttled to 10 Hz and silent when stderr is not a terminal, on the grounds
// that "thousands of redrawn lines in a CI log are worse than no bar"
// (cli/progress.go). Notes had not had it applied to them.
//
// The stored notes do not change. This is a view for a reader - every entry
// keeps its own note in the manifest, where a machine reads it and nothing
// scrolls.
//
// Names are not sorted here and the count is kept rather than the names, so a
// million entry run no longer sorts a million strings to print sixteen lines.
func (m *Manifest) Notes() []string {
	groups := m.NoteGroups()
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Line(core.Says("manifest.NoteText", "%s", core.A("Text", g.Detail))).String())
	}
	return out
}

// NoteGroup is one sentence of the notes and the files that carry it, for a
// reader that words the line in its own language - see Line.
type NoteGroup struct {
	Detail string
	Count  int
	First  []string
}

// NoteGroups is the notes grouped by their sentence, in the order Notes prints
// them.
func (m *Manifest) NoteGroups() []NoteGroup {
	groups := noteGroups{byDetail: map[string]*noteGroup{}}
	for _, f := range m.Files {
		for _, n := range f.Notes {
			groups.add(n.Detail, f.Name)
		}
	}
	sort.Strings(groups.order)

	out := make([]NoteGroup, 0, len(groups.order))
	for _, detail := range groups.order {
		g := groups.byDetail[detail]
		out = append(out, NoteGroup{Detail: detail, Count: g.count, First: g.first})
	}
	return out
}

// noteGroup is one sentence and the files that carry it.
//
// The names are kept only up to noteExamples and the rest is a count, so a
// million entry run holds a handful of strings rather than a million.
type noteGroup struct {
	count int
	first []string
}

// noteGroups collects them, keeping the order the details were first seen in.
type noteGroups struct {
	byDetail map[string]*noteGroup
	order    []string
}

// add records one note against the file that carries it.
//
// The first few names are taken in the order the manifest lists them rather
// than sorted, which is what makes this cheap AND deterministic: the manifest's
// own order is fixed, guarded by auditorder_test.go, so the same recipe names
// the same few files every time.
func (n *noteGroups) add(detail, name string) {
	g := n.byDetail[detail]
	if g == nil {
		g = &noteGroup{}
		n.byDetail[detail] = g
		n.order = append(n.order, detail)
	}
	g.count++
	if len(g.first) < noteExamples {
		g.first = append(g.first, name)
	}
}

// AsHeld is the group's note said as the manifest holds it, for a window
// given a note the run did not keep as a sentence. There is none today, and
// one would reach a window in English rather than not at all.
func (g NoteGroup) AsHeld() core.Said {
	return core.Says("manifest.NoteText", "%s", core.A("Text", g.Detail))
}

// line renders one group for a person to read.
//
// A group of one keeps the shape it always had - the file name in front - so
// the common case of one file with something to say about it does not get worse
// to make the large case better.
//
// The note is handed in as a sentence, so the command line gives the English
// the manifest holds and a window gives the same note in its own language.
func (g NoteGroup) Line(note core.Said) core.Said {
	if g.Count == 1 {
		return core.Says("manifest.NoteOne", "%s: %s", core.A("File", core.Shown(g.First[0])), core.A("Note", note))
	}
	named := strings.Join(core.ShownEach(g.First), ", ")
	files := core.SaysN("manifest.Files", "%d file", "%d files", core.A("Count", g.Count))
	if hidden := g.Count - len(g.First); hidden > 0 {
		return core.Says("manifest.NoteMany", "%s: %s Named: %s. %s not named here.",
			core.A("Files", files), core.A("Note", note), core.A("Named", named),
			core.A("Hidden", core.SaysN("manifest.Files", "%d file", "%d files", core.A("Count", hidden))))
	}
	return core.Says("manifest.NoteSeveral", "%s: %s Named: %s.", core.A("Files", files), core.A("Note", note), core.A("Named", named))
}

// SchemaError is a manifest this build cannot read.
type SchemaError struct {
	Path   string
	Detail core.Said
}

func (e *SchemaError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *SchemaError) Said() core.Said {
	return core.Says("manifest.Unreadable", "%s cannot be read as a manifest: %s", core.A("Path", e.Path), core.A("Detail", e.Detail))
}

// MaxBytes is the largest manifest Load will hold whole.
//
// Load's alone since 2026-11-04. verify and cleanup read through ReadClaims,
// which takes a document a piece at a time under MaxValueBytes on one value and
// has no ceiling on the document - a run past this one wrote a manifest they
// then refused, so its files were files this tool could never remove
// (read.go). Load holds every entry, so for it the length of the document is
// still the memory it takes, and what is below is still its reason.
//
// A manifest arrives from outside the same way a recipe does - it travels with
// a fixture set, it turns up in a pull request, it is handed to "verify" by
// path - so its size is chosen by somebody else. The recipe has had a ceiling
// since 2026-08-02 for exactly that reason and the manifest had none, which was
// an asymmetry rather than a decision.
//
// Sixteen megabytes rather than the recipe's one. A manifest is generated
// rather than typed and it grows with the run: measured on 2026-10-26, an entry
// costs from about 840 B (txt) to about 1580 B (eml), so this holds a run of
// roughly ten to twenty thousand files - above the largest preset in
// docs/PRESETS.md and around the ten thousand this tool is designed around.
//
// What it does not do, stated as plainly as the recipe's: it does not defend
// against a manifest that fits and is still expensive.
const MaxBytes = 16 << 20

// TooLargeError is returned for a manifest past MaxBytes.
type TooLargeError struct {
	Path  string
	Bytes int64
}

func (e *TooLargeError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *TooLargeError) Said() core.Said {
	return core.Says("manifest.TooLarge",
		"%s is %d B and the limit is %d B. A manifest is read into memory to be compared against a directory, so an unbounded one is a way to exhaust it. Check that this is a manifest this tool wrote, or split the run it describes",
		core.A("Path", e.Path), core.A("Bytes", e.Bytes), core.A("Limit", int64(MaxBytes)))
}

// Load reads a manifest written by an earlier run.
//
// The major of manifest_version is checked before anything is believed. A
// manifest from a future major describes fields this build does not know, and
// acting on the half of it we recognise is how "verify" ends up calling a
// directory sound on the strength of the part it could read.
//
// One larger than MaxBytes is refused before a byte of it is read, because
// Load holds the whole document and every entry of it, and the file being read
// is not necessarily one of ours. verify and cleanup read through ReadClaims,
// which holds three fields of the claimed entries and has no ceiling on the
// document - Load is for a caller that needs all of it, and a manifest past
// sixteen megabytes is one such a caller should not be holding.
//
// The same reader as ReadClaims underneath (read.go), so the two refuse the
// same documents in the same words.
func Load(path string) (*Manifest, error) {
	raw, err := readAtMost(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	// Nothing long to stop: the document is in memory already and has a
	// ceiling, so a context here would be one nobody could use.
	err = decode(context.Background(), bytes.NewReader(raw), path, &m, func(f File) {
		m.Files = append(m.Files, f)
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// settled is what is asked of a manifest once all of it has been read, in the
// order the answers come in: the version, then the first entry that leaves the
// directory, then the instructions. A fault of the document as a document came
// before all three and was answered while it was read.
//
// The version first, before anything is believed. A manifest from a future
// major describes fields this build does not know, and acting on the half of it
// we recognise is how "verify" ends up calling a directory sound on the
// strength of the part it could read - and an entry of it that seems to leave
// the directory is an entry this build does not know how to read.
func settled(path string, m *Manifest, escaped error) error {
	if m.ManifestVersion == "" {
		return &SchemaError{Path: path, Detail: core.Says("manifest.ItCarriesNoManifestVersionSo", "it carries no manifest_version, so it is not a manifest this tool wrote")}
	}
	got, want := major(m.ManifestVersion), major(Version)
	if got != want {
		return &SchemaError{Path: path, Detail: core.Says("manifest.ItIsSchemaVersionAndThis", "it is schema version %s and this build reads %s. Use the version of tfg that wrote it", core.A("ManifestVersion", m.ManifestVersion), core.A("Version", Version))}
	}
	if escaped != nil {
		return escaped
	}
	// The instructions are a name beside the manifest and nothing else.
	// cleanup --with-manifest removes the file this names, so a manifest
	// carrying a path here would be a manifest that removes something the run
	// never wrote.
	if name := m.Run.Instructions; name != "" && !isInstructionsName(name) {
		return &SchemaError{Path: path, Detail: core.Says("manifest.RunInstructionsIsAndItCan", "run.instructions is %q, and it can only be the name of a file beside the manifest ending in %s. "+
			"This tool removes that file with the manifest, so it will not act on one that names anything else. "+
			"Use the manifest the run actually wrote, or remove the key", core.A("Name", name), core.A("InstructionsSuffix", instructionsSuffix))}
	}
	return nil
}

// entryEscapes refuses entry i of a manifest when its path points outside the
// directory the manifest describes, and is nil when it stays inside.
//
// This is the door. Every consumer of a manifest comes through the one reader,
// so the check sits there rather than beside each use - "verify" and "cleanup"
// then cannot drift into disagreeing about it, and a consumer written later
// gets it without knowing it exists.
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
// Every entry is checked, not only the claimed ones. An entry that no command
// reads today is one a command reads tomorrow. i counts every entry from one,
// so the number is the one a person finds by counting the list.
func entryEscapes(path string, i int, p string) error {
	problem := core.ContainmentProblem(p)
	if problem == "" {
		return nil
	}
	return &SchemaError{Path: path, Detail: core.Says("manifest.EntryHasThePathWhichLands", "entry %d has the path %q, which lands outside the directory the manifest describes - %s. "+
		"This tool never reads or removes anything outside that directory, so a manifest that asks it to is one it will not act on. "+
		"Use the manifest the run actually wrote, or correct the path to one inside the directory", core.A("I", i), core.A("Path", p), core.A("Problem", problem))}
}

// major is the part before the first dot. A minor bump adds fields and stays
// readable, which is the whole point of splitting the two.
func major(v string) string {
	if i := strings.IndexByte(v, '.'); i >= 0 {
		return v[:i]
	}
	return v
}

// Save writes the manifest under a name nothing holds, and never writes over a
// manifest that is already there.
//
// Two failures shaped this, and neither is hypothetical.
//
// The first is a second run into the same directory. The engine refuses that in
// its preflight, but a check followed by a write is not the same as claiming
// the name: two runs started together both found the directory empty, both
// wrote, and one manifest replaced the other. Measured on 2026-08-03 with two
// runs of eight files under different ids - both ended with exit code 0,
// sixteen files were on the disk, one manifest described eight of them, and the
// other eight could never be removed by this tool again.
//
// The second is the process ending part way through the write. Every generated
// file goes through a temporary name for exactly this reason, while the
// manifest - the one file that can remove all the others - was written in
// place. A truncated manifest does not parse, so the record of a finished run
// would be lost to a Ctrl+C landing in the wrong second.
//
// So the manifest is written whole under a temporary name and given its own by
// core.Publish, which refuses a name somebody holds rather than replacing it.
//
// A run reserves the name before its first file and saves through that
// Reservation. This is for every other caller - the guards, and anything that
// writes a manifest it did not reserve - and it reserves and saves in one go.
func (m *Manifest) Save(path string) error {
	r, err := Claim(path)
	if err != nil {
		return err
	}
	return r.Save(m)
}

// secretProperties are the property names whose value is a credential rather
// than a description of a file.
//
// A copy of what the format registry declares, and deliberately a copy. This
// package records what a run produced and knows nothing about formats - asking
// the registry here would tie the record to it, and the registry can be empty
// in a process that never registered anything, which would answer "no secrets"
// quietly. So the fact is written twice and a guard compares the two, which is
// the same arrangement the licence notices and the registry already have.
var secretProperties = []string{"password"}

// SecretProperties is what this package treats as a credential.
//
// Exported for the guard that compares it with what the format registry
// declares, which is the mechanism that makes the copy above safe. A second
// secret property added to a format and not added here would otherwise be
// recorded in a file everybody on the machine can read, and nothing would say
// so.
func SecretProperties() []string { return slices.Clone(secretProperties) }

// mode is how the manifest file is written.
//
// 0644 for the ordinary case, on purpose and against the usual advice:
// .golangci.yml turns gosec's file permission rules off here because this tool
// exists to produce files somebody else's CI will read, and 0600 on a fixture
// would be a defect in the product rather than a fix.
//
// A manifest carrying a password is the one file that argument does not cover.
// The password is written there deliberately - a locked fixture nobody can open
// checks nothing - but deliberate is not the same as readable by every account
// on the machine.
//
// Windows has no permission bits and Go maps only the owner write bit onto its
// read only attribute, so this changes nothing there. It is the systems where
// other people have accounts that it is for.
func (m *Manifest) mode() os.FileMode {
	for _, f := range m.Files {
		if holdsACredential(f.Properties) {
			return 0o600
		}
	}
	return 0o666
}

// holdsACredential says whether these properties carry a value that is a
// credential rather than a description of the file.
//
// An empty value is not one. That is how a window says "no password": a box
// nobody typed in arrives as an empty string, and a manifest tightened for a
// password that is not there would be a mode nobody asked for.
//
// Split out of mode above rather than written inside it, because the two loops
// and the test nest three deep together and the shape guard counts how many
// functions sit that deep.
func holdsACredential(props map[string]any) bool {
	for _, name := range secretProperties {
		if value, ok := props[name]; ok && value != "" && value != nil {
			return true
		}
	}
	return false
}

// readAtMost reads a manifest and refuses one that is over the ceiling.
//
// One rule in one place, and it used to be two. A look at the directory entry
// ran before the read and the read had no ceiling of its own, which is the
// pattern this project keeps finding: an entry is a look, not a limit. The
// file can grow between the two, a named pipe reports a size of zero and then
// hands over as much as it likes, and os.ReadFile takes the entry as a
// starting size and reads to the end whatever that entry said.
//
// The pair also could not be proven. An ordinary oversized file is refused by
// either check alone, so removing either one left the guard green - the
// mutation runner said so on 2026-08-26, NOT CAUGHT, about a healthy guard.
// Two checks of one rule are two places to be wrong and nowhere to be caught.
//
// One byte past the ceiling is read on purpose: reading exactly MaxBytes
// cannot tell a manifest of that size from a longer one cut off at it. The
// size in the refusal is still the real one, asked of the open file after the
// read rather than of the path before it.
func readAtMost(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	raw, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) <= MaxBytes {
		return raw, nil
	}
	// Asked again, and of the open file rather than the path, so a file that
	// grew is reported at the size it grew to. A pipe answers zero and the
	// count of what was read stands in - one past the ceiling, which is all
	// anybody can say about something that has no size to ask for.
	bytes := int64(len(raw))
	if info, statErr := f.Stat(); statErr == nil && info.Size() > bytes {
		bytes = info.Size()
	}
	return nil, &TooLargeError{Path: path, Bytes: bytes}
}
