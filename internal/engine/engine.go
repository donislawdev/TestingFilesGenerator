// Package engine plans a run, writes the files and backs verify and cleanup.
//
// It knows nothing about the command line or the window. That rule erodes one
// exception at a time, so a test enforces it instead of good intentions.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// Target is one group of files to produce.
type Target struct {
	ID         string
	Format     string
	Count      int
	Bytes      int64
	NameTmpl   string
	Label      bool
	Expected   string
	Properties map[string]string
}

// Options are the settings of one run.
type Options struct {
	OutDir  string
	Seed    int64
	DryRun  bool
	Command string

	// AvailableBytes reports the free space at a path. It is injected so a
	// test can describe a small disk without owning one - and so that a test
	// of the guard writes kilobytes rather than trying for a petabyte when
	// the guard is broken. Nil means ask the operating system.
	AvailableBytes func(path string) (int64, error)
}

func (o Options) availableBytes(path string) (int64, error) {
	if o.AvailableBytes != nil {
		return o.AvailableBytes(path)
	}
	return core.AvailableBytes(path)
}

// Result is what a run produced.
type Result struct {
	Manifest *manifest.Manifest
	// Failures counts files that could not be produced. A run with failures
	// keeps its good files and reports the partial outcome.
	Failures int
}

// PlannedFile is one file worked out before anything is written.
type PlannedFile struct {
	ID     string
	Target *Target
	Index  int
	Name   string
	Seed   uint64
	Desc   format.Descriptor
	Plan   format.Plan
}

// Plan works out every file of every target without touching the disk.
//
// Everything is planned before anything is written. A size a format cannot
// deliver is refused here, which is what makes the promise of "zero files on
// disk" true rather than nearly true.
func Plan(targets []Target, opt Options) ([]PlannedFile, error) {
	var out []PlannedFile
	seen := map[string]bool{}
	names := map[string]string{}

	for i := range targets {
		t := &targets[i]

		if t.ID == "" {
			return nil, &RecipeError{Detail: "a target has no id: every target needs a stable id, it anchors the seed and links to the manifest"}
		}
		if seen[t.ID] {
			return nil, &RecipeError{Detail: fmt.Sprintf("target id %q is used twice: ids identify targets, so a duplicate is an error rather than a silent overwrite", t.ID)}
		}
		seen[t.ID] = true

		if t.Count <= 0 {
			return nil, &RecipeError{Detail: fmt.Sprintf("target %q asks for %d files: ask for at least one", t.ID, t.Count)}
		}

		desc, err := format.Get(t.Format)
		if err != nil {
			return nil, err
		}

		if t.Bytes < desc.MinBytes {
			return nil, &format.BelowMinimumError{
				Format:    strings.ToUpper(desc.ID),
				Requested: t.Bytes,
				Minimum:   desc.MinBytes,
				Reason:    "that is the smallest valid file this format can produce",
				Hint:      fmt.Sprintf("Ask for %d B or more.", desc.MinBytes),
			}
		}

		targetSeed := core.TargetSeed(opt.Seed, t.ID)

		for idx := 0; idx < t.Count; idx++ {
			fileSeed := core.FileSeed(targetSeed, idx)

			p, err := desc.Generator.Plan(format.Request{
				Bytes:      t.Bytes,
				Seed:       fileSeed,
				Label:      t.Label,
				Properties: t.Properties,
			})
			if err != nil {
				return nil, err
			}

			name := renderName(t, desc, idx)
			// Two files heading for one name means one of them would be
			// destroyed by the other, and the manifest would still describe
			// both. A manifest that quietly lost a file looks complete and
			// reaches the test suite as a false truth.
			if owner, clash := names[name]; clash {
				return nil, &RecipeError{Detail: fmt.Sprintf(
					"targets %q and %q both produce a file named %s - give one of them a --name template containing {index:04}",
					owner, t.ID, name)}
			}
			names[name] = t.ID

			out = append(out, PlannedFile{
				ID:     fmt.Sprintf("f_%04d", len(out)+1),
				Target: t,
				Index:  idx,
				Name:   name,
				Seed:   fileSeed,
				Desc:   desc,
				Plan:   p,
			})
		}
	}
	return out, nil
}

// TotalBytes is what a plan will occupy on disk. Known before the first byte
// is written, which is what the free space guard and --dry-run stand on.
func TotalBytes(files []PlannedFile) int64 {
	var n int64
	for _, f := range files {
		n += f.Plan.Bytes
	}
	return n
}

// Run writes a planned set of files.
//
// Each file is written under a temporary name and only then renamed, so the
// output directory never holds an incomplete file. That invariant covers the
// process ending - Ctrl+C, kill, a CI timeout. It does not cover power loss,
// because that would need a flush per file and ten thousand of those is a
// real cost.
//
// A manifest is returned even when the run is cut short, otherwise cleanup
// has nothing to work with.
func Run(ctx context.Context, files []PlannedFile, opt Options) (*Result, error) {
	m := manifest.New(
		"testing-files-generator", version.Version,
		runID(opt.Seed), opt.Command, opt.Seed,
		runtime.GOOS, runtime.GOARCH,
	)
	res := &Result{Manifest: m}

	if opt.DryRun {
		// Nothing is written. Every entry is what would have been produced,
		// which is the same planning path the real run uses rather than a
		// separate one that can drift away from it.
		for _, f := range files {
			m.Add(entryFor(f, "", false, nil))
		}
		m.Run.Complete = true
		return res, nil
	}

	// Free space is checked before the first byte. Finding out at file five
	// thousand of ten thousand leaves a half written set and a full disk on a
	// machine somebody works on.
	needed := TotalBytes(files)
	if available, err := opt.availableBytes(opt.OutDir); err == nil {
		if available < needed {
			return res, &SpaceError{Needed: needed, Available: available, Path: opt.OutDir}
		}
	}
	// A failure to read the free space is not a reason to refuse. It is
	// reported by the caller and the run goes ahead, because a disk we cannot
	// measure is not the same as a disk that is full.

	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return res, fmt.Errorf("cannot create the output directory %s: %w", opt.OutDir, err)
	}

	// Nothing is written over. This tool runs in directories that belong to
	// the user, so destroying their work is the one failure that cannot be
	// undone by running again.
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(opt.OutDir, f.Name)); err == nil {
			return res, &CollisionError{Path: filepath.Join(opt.OutDir, f.Name)}
		}
	}

	for _, f := range files {
		select {
		case <-ctx.Done():
			// Stop starting new files. What is already finished stays, and
			// the manifest describes exactly that.
			m.Run.Complete = false
			return res, ctx.Err()
		default:
		}

		sum, err := writeOne(ctx, f, opt.OutDir)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				m.Run.Complete = false
				return res, err
			}
			// One file failing does not end the run. Nine thousand good
			// files are worth keeping, and the entry says what went wrong.
			res.Failures++
			m.Add(entryFor(f, "", false, err))
			continue
		}
		m.Add(entryFor(f, sum, true, nil))
	}

	m.Run.Complete = true
	return res, nil
}

func writeOne(ctx context.Context, f PlannedFile, outDir string) (string, error) {
	final := filepath.Join(outDir, f.Name)
	tmp := final + ".tfg-partial"

	fh, err := os.Create(tmp)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(fh, h)}

	writeErr := f.Desc.Generator.Write(ctx, counter, f.Plan)
	closeErr := fh.Close()

	if writeErr != nil {
		os.Remove(tmp)
		return "", writeErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return "", closeErr
	}

	// The size is the promise. A generator that missed it by a byte is a bug
	// worth catching here rather than in someone's test suite, so the file
	// never reaches its final name.
	if counter.n != f.Plan.Bytes {
		os.Remove(tmp)
		return "", fmt.Errorf("generator for %s produced %d B where the plan said %d B",
			f.Desc.ID, counter.n, f.Plan.Bytes)
	}

	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func entryFor(f PlannedFile, sha string, materialized bool, failure error) manifest.File {
	var notes []manifest.Note
	for _, n := range f.Plan.Notes {
		notes = append(notes, manifest.Note{Code: n.Code, Detail: n.Detail})
	}

	label, _ := f.Plan.Properties["label_embedded"].(bool)

	e := manifest.File{
		ID:            f.ID,
		Path:          filepath.ToSlash(f.Name),
		Name:          f.Name,
		Materialized:  materialized,
		Bytes:         f.Plan.Bytes,
		Format:        f.Desc.ID,
		Fidelity:      string(f.Desc.Fidelity),
		Hashes:        manifest.Hashes{SHA256: sha},
		Seed:          core.SeedLabel(f.Seed),
		Generator:     manifest.GeneratorRef{Name: f.Desc.ID, Version: f.Desc.GeneratorVersion},
		Determinism:   string(f.Plan.Determinism),
		Properties:    f.Plan.Properties,
		LabelEmbedded: label,
		Notes:         notes,
		Expected:      expectationFor(f),
	}

	if failure != nil {
		e.Failed = true
		e.Error = failure.Error()
		e.Materialized = false
		e.Notes = append(e.Notes, manifest.Note{
			Code:   "generation_failed",
			Detail: "This file was not produced. The run carried on and ended with the partial exit code.",
		})
	}
	return e
}

func expectationFor(f PlannedFile) manifest.Expected {
	switch f.Target.Expected {
	case "":
		return manifest.Expected{
			Outcome:    manifest.OutcomeUnspecified,
			Detail:     "No expectation was declared for this file.",
			Confidence: "policy_dependent",
		}
	case manifest.OutcomeAccept, manifest.OutcomeReject,
		manifest.OutcomeSanitize, manifest.OutcomeUnspecified:
		return manifest.Expected{Outcome: f.Target.Expected, Confidence: "certain"}
	default:
		return manifest.Expected{
			Outcome:    manifest.OutcomeUnspecified,
			Detail:     fmt.Sprintf("Unrecognised expectation %q was declared.", f.Target.Expected),
			Confidence: "policy_dependent",
		}
	}
}

func renderName(t *Target, d format.Descriptor, index int) string {
	tmpl := t.NameTmpl
	if tmpl == "" {
		tmpl = t.ID + "_{index:04}" + d.Extension
	}
	return strings.ReplaceAll(tmpl, "{index:04}", fmt.Sprintf("%04d", index+1))
}

func runID(seed int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("run:%d", seed)))
	return "run_" + hex.EncodeToString(h[:5])
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
